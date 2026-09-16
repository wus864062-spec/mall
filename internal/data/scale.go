package data

import "mall/internal/biz"

func (d *Data) migrateMoneyScale() bool {
	if d.moneyScale == biz.UstdScale {
		return false
	}
	empty := len(d.users) == 0 && len(d.products) == 0 && len(d.ledger) == 0 && len(d.orders) == 0
	if empty {
		d.moneyScale = biz.UstdScale
		return true
	}
	from := d.moneyScale
	if from <= 0 {
		from = 100
	}
	if from == biz.UstdScale {
		d.moneyScale = biz.UstdScale
		return true
	}
	if from <= 0 || biz.UstdScale%from != 0 {
		d.moneyScale = biz.UstdScale
		return true
	}
	mul := biz.UstdScale / from
	if mul <= 1 {
		d.moneyScale = biz.UstdScale
		return true
	}
	d.ispayPriceFen *= mul
	d.minWithdrawFen *= mul
	d.maxRechargeFen *= mul
	for id, v := range d.balances {
		d.balances[id] = v * mul
	}
	for id, v := range d.frozenUsdt {
		d.frozenUsdt[id] = v * mul
	}
	for id, v := range d.unfreezeRemain {
		d.unfreezeRemain[id] = v * mul
	}
	for id, lots := range d.frozenLots {
		for i := range lots {
			lots[i].UsdtFen *= mul
		}
		d.frozenLots[id] = lots
	}
	for _, u := range d.users {
		if u == nil {
			continue
		}
		u.PerfFen *= mul
		u.SettledPairFen *= mul
		u.PairClearedRightFen *= mul
		u.StaticPackageFen *= mul
		u.DynamicRewardFen *= mul
		u.UnfreezeGrantedFen *= mul
	}
	for _, p := range d.products {
		if p == nil {
			continue
		}
		p.PriceFen *= mul
	}
	for _, o := range d.orders {
		if o == nil {
			continue
		}
		o.TotalFen *= mul
		for _, it := range o.Items {
			if it == nil {
				continue
			}
			it.PriceFen *= mul
			it.AmountFen *= mul
		}
	}
	for _, e := range d.ledger {
		if e == nil || biz.IsIspayAmountType(e.Type) {
			continue
		}
		e.AmountFen *= mul
		e.BalanceFen *= mul
	}
	d.moneyScale = biz.UstdScale
	return true
}
