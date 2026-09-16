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
					helper.Info("daily static reward settled")
				}
			}
		}
	}()
}

func (d *Data) maybeSettlePairing(now time.Time) bool {
	today := now.In(pairSettleLocation()).Format("2006-01-02")
	d.mu.Lock()
	defer d.mu.Unlock()
	expired := d.expireFrozenLocked(now)
	if d.lastPairSettleDate == "" {
		d.lastPairSettleDate = today
		_ = d.save()
		return false
	}
	if d.lastPairSettleDate >= today {
		if expired {
			_ = d.save()
		}
		return false
	}
	d.settleStaticLocked()
	d.lastPairSettleDate = today
	_ = d.save()
	return true
}

func (d *Data) tryInstantPairLocked(buyerID int64) {
	buyer := d.users[buyerID]
	if buyer == nil {
		return
	}
	ancs := biz.PlacementAncestors(d.users, buyer, 1024)
	orders := d.orderListLocked()
	for _, u := range ancs {
		d.settlePairUserLocked(u, orders)
	}
}

func (d *Data) settlePairingLocked() {
	ids := make([]int64, 0, len(d.users))
	for id := range d.users {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	orders := d.orderListLocked()
	for _, id := range ids {
		d.settlePairUserLocked(d.users[id], orders)
	}
}

func (d *Data) settlePairUserLocked(u *biz.User, orders []*biz.Order) {
	if u == nil {
		return
	}
	left := biz.SumPerf(biz.LeftMembersOf(d.users, u))
	right := biz.SumPerf(biz.RightMembersOf(d.users, u))
	_, _, pairVol := biz.PairPending(left, right, u.SettledPairFen)
	if pairVol <= 0 {
		return
	}
	u.SettledPairFen += pairVol
	bonus := biz.PairRewardFen(pairVol)
	pay := d.creditCappedDynamicLocked(u.ID, bonus, biz.LedgerPairReward, biz.LedgerPairRewardIspay, "pair", u.ID)
	if pay > 0 {
		d.payManageRewardLocked(u, pay)
	}
}

func (d *Data) payManageRewardLocked(from *biz.User, pairPayFen int64) {
	if from != nil && from.SkipUplineReward {
		return
	}
	pool := biz.ManageRewardFen(pairPayFen)
	uplines := biz.InviteAncestors(d.users, from, biz.PairUplineLevels)
	orders := d.orderListLocked()
	var qualified []*biz.User
	for _, p := range uplines {
		if p != nil && biz.EffectivePackageFen(biz.Web3PaidSumFen(orders, p.ID)) > 0 {
			qualified = append(qualified, p)
		}
	}
	parts := biz.SplitEvenFen(pool, len(qualified))
	for i, amt := range parts {
		if amt <= 0 {
			continue
		}
		u := qualified[i]
		d.creditCappedDynamicLocked(u.ID, amt, biz.LedgerManageReward, biz.LedgerManageRewardIspay, "manage", from.ID)
	}
}
