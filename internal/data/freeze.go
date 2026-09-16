package data

import (
	"sort"
	"time"

	"mall/internal/biz"
)

type frozenLot struct {
	UsdtFen    int64 `json:"usdt_fen"`
	IspayMicro int64 `json:"ispay_micro"`
	CreatedAt  int64 `json:"created_at"`
}

func (d *Data) clock() time.Time {
	if !d.now.IsZero() {
		return d.now
	}
	return time.Now()
}

func (d *Data) creditWithdrawableSplitLocked(userID, valueFen int64, usdtType, ispayType, refType string, refID int64) {
	d.ensureAssetMaps()
	usdt, micro := biz.SplitHalf(valueFen, d.priceFenLocked())
	if usdt != 0 {
		d.balances[userID] += usdt
		d.appendAssetLedger(userID, usdt, d.balances[userID], usdtType, refType, refID)
	}
	if micro != 0 {
		d.ispayFree[userID] += micro
		d.appendAssetLedger(userID, micro, d.ispayFree[userID], ispayType, refType, refID)
	}
}

// creditSplitLocked 动态奖折半。先用剩余提现额度进可提现（扣额度），超额进冻结批次。
func (d *Data) creditSplitLocked(userID, valueFen int64, usdtType, ispayType, refType string, refID int64) {
	d.ensureAssetMaps()
	d.expireFrozenLocked(d.clock())
	if valueFen <= 0 {
		return
	}
	freeVal := d.unfreezeRemain[userID]
	if freeVal > valueFen {
		freeVal = valueFen
	}
	if freeVal > 0 {
		d.creditWithdrawableSplitLocked(userID, freeVal, usdtType, ispayType, refType, refID)
		d.unfreezeRemain[userID] -= freeVal
	}
	rest := valueFen - freeVal
	if rest > 0 {
		d.addFrozenSplitLocked(userID, rest, usdtType, ispayType, refType, refID)
	}
}

func (d *Data) addFrozenSplitLocked(userID, valueFen int64, usdtType, ispayType, refType string, refID int64) {
	usdt, micro := biz.SplitHalf(valueFen, d.priceFenLocked())
	if usdt == 0 && micro == 0 {
		return
	}
	now := d.clock().Unix()
	day := shanghaiDayString(d.clock())
	lots := coalesceFrozenLots(d.frozenLots[userID])
	merged := false
	for i := range lots {
		if frozenLotDay(lots[i].CreatedAt) == day {
			lots[i].UsdtFen += usdt
			lots[i].IspayMicro += micro
			merged = true
			break
		}
	}
	if !merged {
		lots = append(lots, frozenLot{UsdtFen: usdt, IspayMicro: micro, CreatedAt: now})
	}
	d.frozenLots[userID] = lots
	if usdt != 0 {
		d.frozenUsdt[userID] += usdt
		d.appendAssetLedger(userID, usdt, d.frozenUsdt[userID], usdtType, refType, refID)
	}
	if micro != 0 {
		d.ispayFrozen[userID] += micro
		d.appendAssetLedger(userID, micro, d.ispayFrozen[userID], ispayType, refType, refID)
	}
}

// grantUnfreezeByOrderLocked 额度 += 当天最高档封顶 − 当天已发，再从最早冻结日 FIFO 解冻。
// 不改剩余批次 CreatedAt。用认购合计折档。同一上海日同档只发一次；升档补差额；换日不自动发，要再买单。
func (d *Data) grantUnfreezeByOrderLocked(userID int64, o *biz.Order) {
	d.ensureAssetMaps()
	d.expireFrozenLocked(d.clock())
	if !biz.IsSubscribeOrder(o) {
		return
	}
	u := d.users[userID]
	if u == nil {
		return
	}
	day := shanghaiDayString(d.clock())
	paid := biz.Web3PaidSumFen(d.orderListLocked(), userID)
	granted := biz.UnfreezeGrantedTodayFen(u.UnfreezeGrantDay, day, u.UnfreezeGrantedFen)
	// 额度用配置项日封顶（PairCaps），不是默认表，也不是认购总额。
	add := biz.GrantUnfreezeFenWith(granted, paid, d.pairCaps)
	if add <= 0 {
		return
	}
	if u.UnfreezeGrantDay != day {
		u.UnfreezeGrantDay = day
		u.UnfreezeGrantedFen = 0
	}
	u.UnfreezeGrantedFen += add
	d.unfreezeRemain[userID] += add
	d.drainFrozenToQuotaLocked(userID, o)
}

// drainFrozenToQuotaLocked 用剩余额度从最早冻结日 FIFO 解冻。不改剩余批次 CreatedAt。
func (d *Data) drainFrozenToQuotaLocked(userID int64, o *biz.Order) {
	remain := d.unfreezeRemain[userID]
	frozenVal := biz.ReconstructSplitValueFen(d.frozenUsdt[userID], d.ispayFrozen[userID], d.priceFenLocked())
	take := biz.TakeUnfreeze(remain, frozenVal)
	if take <= 0 {
		return
	}
	usdt, micro := d.consumeFrozenValueLocked(userID, take)
	if usdt == 0 && micro == 0 {
		return
	}
	used := biz.ReconstructSplitValueFen(usdt, micro, d.priceFenLocked())
	d.unfreezeRemain[userID] -= used
	if d.unfreezeRemain[userID] < 0 {
		d.unfreezeRemain[userID] = 0
	}
	var refID int64
	if o != nil {
		refID = o.ID
	}
	if usdt != 0 {
		d.balances[userID] += usdt
		d.appendAssetLedger(userID, usdt, d.balances[userID], biz.LedgerUnfreeze, "order", refID)
	}
	if micro != 0 {
		d.ispayFree[userID] += micro
		d.appendAssetLedger(userID, micro, d.ispayFree[userID], biz.LedgerUnfreezeIspay, "order", refID)
	}
}

func (d *Data) consumeFrozenValueLocked(userID, wantFen int64) (movedUsdt, movedMicro int64) {
	if wantFen <= 0 {
		return 0, 0
	}
	price := d.priceFenLocked()
	lots := coalesceFrozenLots(d.frozenLots[userID])
	keep := make([]frozenLot, 0, len(lots))
	for _, lot := range lots {
		if wantFen <= 0 {
			keep = append(keep, lot)
			continue
		}
		lotVal := biz.ReconstructSplitValueFen(lot.UsdtFen, lot.IspayMicro, price)
		if lotVal <= 0 {
			continue
		}
		take := wantFen
		if take > lotVal {
			take = lotVal
		}
		tu, tm := biz.SplitHalf(take, price)
		if tu > lot.UsdtFen {
			tu = lot.UsdtFen
		}
		if tm > lot.IspayMicro {
			tm = lot.IspayMicro
		}
		lot.UsdtFen -= tu
		lot.IspayMicro -= tm
		movedUsdt += tu
		movedMicro += tm
		d.frozenUsdt[userID] -= tu
		d.ispayFrozen[userID] -= tm
		wantFen -= take
		if lot.UsdtFen > 0 || lot.IspayMicro > 0 {
			keep = append(keep, lot)
		}
	}
	d.frozenLots[userID] = keep
	return movedUsdt, movedMicro
}

// expireFrozenLocked 各日批次按自己的 CreatedAt+72h 作废。买单不刷新倒计时；先到期的先清。
func (d *Data) expireFrozenLocked(now time.Time) bool {
	d.ensureAssetMaps()
	cutoff := now.Unix() - biz.FrozenTTLSeconds
	changed := false
	for uid, lots := range d.frozenLots {
		lots = coalesceFrozenLots(lots)
		keep := make([]frozenLot, 0, len(lots))
		var expUsdt, expMicro int64
		for _, lot := range lots {
			if lot.CreatedAt <= cutoff {
				expUsdt += lot.UsdtFen
				expMicro += lot.IspayMicro
				continue
			}
			keep = append(keep, lot)
		}
		if expUsdt == 0 && expMicro == 0 {
			d.frozenLots[uid] = lots
			continue
		}
		d.frozenLots[uid] = keep
		d.frozenUsdt[uid] -= expUsdt
		d.ispayFrozen[uid] -= expMicro
		if d.frozenUsdt[uid] < 0 {
			d.frozenUsdt[uid] = 0
		}
		if d.ispayFrozen[uid] < 0 {
			d.ispayFrozen[uid] = 0
		}
		if expUsdt != 0 {
			d.appendAssetLedger(uid, -expUsdt, d.frozenUsdt[uid], biz.LedgerFrozenExpire, "expire", 0)
		}
		if expMicro != 0 {
			d.appendAssetLedger(uid, -expMicro, d.ispayFrozen[uid], biz.LedgerFrozenExpireIspay, "expire", 0)
		}
		changed = true
	}
	return changed
}

func (d *Data) peelFrozenUsdtLocked(userID, amount int64) {
	if amount <= 0 {
		return
	}
	lots := coalesceFrozenLots(d.frozenLots[userID])
	keep := make([]frozenLot, 0, len(lots))
	for _, lot := range lots {
		if amount <= 0 {
			keep = append(keep, lot)
			continue
		}
		take := amount
		if take > lot.UsdtFen {
			take = lot.UsdtFen
		}
		lot.UsdtFen -= take
		amount -= take
		if lot.UsdtFen > 0 || lot.IspayMicro > 0 {
			keep = append(keep, lot)
		}
	}
	d.frozenLots[userID] = keep
}

func (d *Data) peelFrozenIspayLocked(userID, amount int64) {
	if amount <= 0 {
		return
	}
	lots := coalesceFrozenLots(d.frozenLots[userID])
	keep := make([]frozenLot, 0, len(lots))
	for _, lot := range lots {
		if amount <= 0 {
			keep = append(keep, lot)
			continue
		}
		take := amount
		if take > lot.IspayMicro {
			take = lot.IspayMicro
		}
		lot.IspayMicro -= take
		amount -= take
		if lot.UsdtFen > 0 || lot.IspayMicro > 0 {
			keep = append(keep, lot)
		}
	}
	d.frozenLots[userID] = keep
}

func (d *Data) debitUsdtPreferFrozenLocked(userID, amount int64) int64 {
	d.ensureAssetMaps()
	if amount == 0 {
		return d.balances[userID]
	}
	if amount < 0 {
		amount = -amount
	}
	take := amount
	if d.frozenUsdt[userID] > 0 {
		fromFrozen := take
		if d.frozenUsdt[userID] < fromFrozen {
			fromFrozen = d.frozenUsdt[userID]
		}
		d.peelFrozenUsdtLocked(userID, fromFrozen)
		d.frozenUsdt[userID] -= fromFrozen
		take -= fromFrozen
		if take == 0 {
			return d.frozenUsdt[userID]
		}
	}
	d.balances[userID] -= take
	return d.balances[userID]
}

func (d *Data) debitIspayPreferFrozenLocked(userID, amount int64) int64 {
	d.ensureAssetMaps()
	if amount == 0 {
		return d.ispayFree[userID]
	}
	if amount < 0 {
		amount = -amount
	}
	take := amount
	if d.ispayFrozen[userID] > 0 {
		fromFrozen := take
		if d.ispayFrozen[userID] < fromFrozen {
			fromFrozen = d.ispayFrozen[userID]
		}
		d.peelFrozenIspayLocked(userID, fromFrozen)
		d.ispayFrozen[userID] -= fromFrozen
		take -= fromFrozen
		if take == 0 {
			return d.ispayFrozen[userID]
		}
	}
	d.ispayFree[userID] -= take
	return d.ispayFree[userID]
}

func (d *Data) seedFrozenLotsIfNeededLocked() {
	d.ensureAssetMaps()
	now := d.clock().Unix()
	for id, usdt := range d.frozenUsdt {
		micro := d.ispayFrozen[id]
		if (usdt == 0 && micro == 0) || len(d.frozenLots[id]) > 0 {
			continue
		}
		d.frozenLots[id] = []frozenLot{{UsdtFen: usdt, IspayMicro: micro, CreatedAt: now}}
	}
	for id, micro := range d.ispayFrozen {
		if micro == 0 || len(d.frozenLots[id]) > 0 {
			continue
		}
		d.frozenLots[id] = []frozenLot{{UsdtFen: d.frozenUsdt[id], IspayMicro: micro, CreatedAt: now}}
	}
}

func frozenLotDay(createdAt int64) string {
	return shanghaiDayString(time.Unix(createdAt, 0))
}

// coalesceFrozenLots 同一上海日合成一笔，CreatedAt 取当天最早一次。买单/追加冻结都不刷新倒计时。
func coalesceFrozenLots(lots []frozenLot) []frozenLot {
	if len(lots) == 0 {
		return lots
	}
	sort.SliceStable(lots, func(i, j int) bool {
		return lots[i].CreatedAt < lots[j].CreatedAt
	})
	out := make([]frozenLot, 0, len(lots))
	for _, lot := range lots {
		if lot.UsdtFen == 0 && lot.IspayMicro == 0 {
			continue
		}
		if n := len(out); n > 0 && frozenLotDay(out[n-1].CreatedAt) == frozenLotDay(lot.CreatedAt) {
			out[n-1].UsdtFen += lot.UsdtFen
			out[n-1].IspayMicro += lot.IspayMicro
			continue
		}
		out = append(out, lot)
	}
	return out
}
