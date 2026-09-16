package data

import (
	"sort"
	"time"

	"mall/internal/biz"
)

func (d *Data) ensureAssetMaps() {
	if d.balances == nil {
		d.balances = map[int64]int64{}
	}
	if d.ispayLocked == nil {
		d.ispayLocked = map[int64]int64{}
	}
	if d.ispayFree == nil {
		d.ispayFree = map[int64]int64{}
	}
	if d.frozenUsdt == nil {
		d.frozenUsdt = map[int64]int64{}
	}
	if d.ispayFrozen == nil {
		d.ispayFrozen = map[int64]int64{}
	}
	if d.unfreezeRemain == nil {
		d.unfreezeRemain = map[int64]int64{}
	}
	if d.frozenLots == nil {
		d.frozenLots = map[int64][]frozenLot{}
	}
	if d.txByHash == nil {
		d.txByHash = map[string]int64{}
	}
}

func (d *Data) priceFenLocked() int64 {
	return biz.NormalizeIspayPriceFen(d.ispayPriceFen)
}

func (d *Data) appendAssetLedger(userID, amount, postBal int64, typ, refType string, refID int64) *biz.LedgerEntry {
	d.ledgerSeq++
	e := &biz.LedgerEntry{
		ID:         d.ledgerSeq,
		UserID:     userID,
		AmountFen:  amount,
		BalanceFen: postBal,
		Type:       typ,
		RefType:    refType,
		RefID:      refID,
		CreatedAt:  time.Now().Unix(),
	}
	d.ledger = append(d.ledger, e)
	return e
}

func (d *Data) orderListLocked() []*biz.Order {
	orders := make([]*biz.Order, 0, len(d.orders))
	for _, o := range d.orders {
		orders = append(orders, o)
	}
	return orders
}

// syncWeb3LockLocked 按已付认购重算档位和 IsPay 锁仓。
//
// 档位 = EffectivePackageFen(总额)：未达下一档不变，碰到或超过则升档。
// 锁仓 = 档位折 IsPay + 超额折 IsPay（同兑换价即总额折）。
// 例：1000+3000=4000 → 档位仍 3000，多出的 1000U 进锁仓。
// 例：3000+3000=6000 → 档位升 6000，锁仓按 6000 折。再认购不把已释放天数归零。
func (d *Data) syncWeb3LockLocked(userID int64) {
	d.ensureAssetMaps()
	u := d.users[userID]
	if u == nil {
		return
	}
	orders := d.orderListLocked()
	sum := biz.Web3PaidSumFen(orders, userID)
	eff := biz.EffectivePackageFen(sum)
	cat := biz.DominantWeb3Category(orders, userID)
	var want, days int64
	if sum > 0 {
		want = biz.CoinsMicroFromPay(sum, cat)
		days = biz.ReleaseDaysByCategory(cat)
	}
	cur := d.ispayLocked[userID]
	if want != cur {
		delta := want - cur
		d.ispayLocked[userID] = want
		typ := biz.LedgerIspayConvert
		if delta < 0 {
			typ = biz.LedgerIspayConvertClawback
		}
		// RefType 历史值曾用 "tier"；锁仓已改按认购总额，新流水记 "lock"。
		d.appendAssetLedger(userID, delta, d.ispayLocked[userID], typ, "lock", 0)
	}
	if eff <= 0 {
		u.StaticDays = 0
		u.StaticReleasedDays = 0
	} else if u.StaticDays <= 0 && days > 0 {
		u.StaticDays = days
	} else if days > 0 {
		u.StaticDays = days
	}
	u.StaticPackageFen = eff
}

// resyncAllWeb3Locks 启动时按认购总额对齐锁仓。只改枚数，不因公式变化清零已释放天数。
func (d *Data) resyncAllWeb3Locks() bool {
	changed := false
	for id, u := range d.users {
		if u == nil {
			continue
		}
		beforeLock := d.ispayLocked[id]
		beforePkg, beforeDays, beforeReleased := u.StaticPackageFen, u.StaticDays, u.StaticReleasedDays
		d.syncWeb3LockLocked(id)
		if d.ispayLocked[id] != beforeLock || u.StaticPackageFen != beforePkg || u.StaticDays != beforeDays || u.StaticReleasedDays != beforeReleased {
			changed = true
		}
	}
	return changed
}

func (d *Data) settleStaticLocked() {
	d.ensureAssetMaps()
	ids := make([]int64, 0, len(d.users))
	for id := range d.users {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	price := d.priceFenLocked()
	for _, id := range ids {
		u := d.users[id]
		if u == nil || u.StaticDays <= 0 || u.StaticReleasedDays >= u.StaticDays {
			continue
		}
		locked := d.ispayLocked[id]
		if locked <= 0 {
			continue
		}
		daily := biz.DailyReleaseCoins(locked, u.StaticDays, u.StaticReleasedDays)
		value := biz.StaticValueFen(daily, price)
		if value > 0 {
			d.creditWithdrawableSplitLocked(id, value, biz.LedgerStaticReward, biz.LedgerStaticRewardIspay, "static", id)
		}
		u.StaticReleasedDays++
		for _, o := range d.orders {
			if o != nil && o.UserID == id && o.Status == biz.OrderPaid && o.ReleaseDays > 0 {
				o.ReleasedDays = u.StaticReleasedDays
			}
		}
	}
}
