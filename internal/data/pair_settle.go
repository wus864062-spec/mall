package data

import (
	"sort"
	"time"

	"mall/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

func pairSettleLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

func (d *Data) startPairScheduler(helper *log.Helper) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		d.maybeSettlePairing(time.Now())
		for {
			select {
			case <-d.stopPair:
				return
			case now := <-ticker.C:
				if d.maybeSettlePairing(now) {
					helper.Info("daily pair reward settled")
				}
			}
		}
	}()
}

func (d *Data) maybeSettlePairing(now time.Time) bool {
	today := now.In(pairSettleLocation()).Format("2006-01-02")
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lastPairSettleDate == "" {
		d.lastPairSettleDate = today
		_ = d.save()
		return false
	}
	if d.lastPairSettleDate >= today {
		return false
	}
	d.settlePairingLocked()
	d.lastPairSettleDate = today
	_ = d.save()
	return true
}

func (d *Data) settlePairingLocked() {
	ids := make([]int64, 0, len(d.users))
	for id := range d.users {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	orders := make([]*biz.Order, 0, len(d.orders))
	for _, o := range d.orders {
		orders = append(orders, o)
	}
	dayRef := int64(time.Now().Unix())
	for _, id := range ids {
		u := d.users[id]
		if u == nil {
			continue
		}
		left := biz.SumPerf(biz.LeftMembersOf(d.users, u))
		right := biz.SumPerf(biz.RightMembersOf(d.users, u))
		_, _, pairVol := biz.SmallHitsLarge(left, right, u.SettledPairFen, u.PairClearedRightFen)
		if pairVol > 0 {
			// 大小区当天全部对完：结算与认购封顶无关。
			u.SettledPairFen += pairVol
			bonus := biz.PairRewardFen(pairVol)
			pay := biz.CapPairPayout(bonus, biz.HighestPairCapFen(orders, u.ID))
			if pay > 0 {
				d.balances[u.ID] += pay
				d.appendPairLedger(u.ID, pay, biz.LedgerPairReward, "pair", dayRef)
				d.payManageRewardLocked(u, pay)
			}
		}
		u.PairClearedRightFen = right
	}
}

func (d *Data) payManageRewardLocked(from *biz.User, pairPayFen int64) {
	pool := biz.ManageRewardFen(pairPayFen)
	uplines := biz.InviteAncestors(d.users, from, biz.PairUplineLevels)
	parts := biz.SplitEvenFen(pool, len(uplines))
	for i, p := range parts {
		if p <= 0 {
			continue
		}
		u := uplines[i]
		d.balances[u.ID] += p
		d.appendPairLedger(u.ID, p, biz.LedgerManageReward, "manage", from.ID)
	}
}

func (d *Data) appendPairLedger(userID, amountFen int64, typ, refType string, refID int64) {
	d.ledgerSeq++
	d.ledger = append(d.ledger, &biz.LedgerEntry{
		ID:         d.ledgerSeq,
		UserID:     userID,
		AmountFen:  amountFen,
		BalanceFen: d.balances[userID],
		Type:       typ,
		RefType:    refType,
		RefID:      refID,
		CreatedAt:  time.Now().Unix(),
	})
}
