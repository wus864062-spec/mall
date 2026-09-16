package data

import (
	"context"
	"sort"
	"time"

	"mall/internal/biz"
)

type shopRepo struct {
	data *Data
}

func NewShopRepo(data *Data) biz.ShopRepo {
	return &shopRepo{data: data}
}

func (r *shopRepo) GetBalance(ctx context.Context, userID int64) (int64, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	r.data.ensureAssetMaps()
	return r.data.balances[userID], nil
}

func (r *shopRepo) GetWalletView(ctx context.Context, userID int64) (*biz.WalletView, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	r.data.ensureAssetMaps()
	if r.data.expireFrozenLocked(r.data.clock()) {
		_ = r.data.save()
	}
	s := r.data.settingsLocked()
	view := &biz.WalletView{
		BalanceFen:         r.data.balances[userID],
		RechargeFen:        biz.RechargeRemainFen(r.data.ledger, userID), // 充值页：只计充值进来，不算奖励
		FrozenUsdtFen:      r.data.frozenUsdt[userID],
		IspayLockedMicro:   r.data.ispayLocked[userID],
		IspayFreeMicro:     r.data.ispayFree[userID],
		FrozenIspayMicro:   r.data.ispayFrozen[userID],
		IspayPriceFen:      s.IspayPriceFen,
		MaxRechargeFen:     s.MaxRechargeFen,
		MinWithdrawFen:     s.MinWithdrawFen,
		WithdrawFeePercent: s.WithdrawFeePercent,
		PairCapsUstd:       biz.PairCapsUstdMap(s.PairCaps),
	}
	var days, released, pkg int64
	if u := r.data.users[userID]; u != nil {
		days, released, pkg = u.StaticDays, u.StaticReleasedDays, u.StaticPackageFen
	}
	var gotUsdt, gotIspay, directUsdt, directIspay, pairUsdt, pairIspay, manageUsdt, manageIspay int64
	for _, e := range r.data.ledger {
		if e == nil || e.UserID != userID {
			continue
		}
		switch e.Type {
		case biz.LedgerStaticReward:
			gotUsdt += e.AmountFen
		case biz.LedgerStaticRewardIspay:
			gotIspay += e.AmountFen
		case biz.LedgerDirectReward, biz.LedgerDirectRewardClawback:
			directUsdt += e.AmountFen
		case biz.LedgerDirectRewardIspay, biz.LedgerDirectRewardIspayClawback:
			directIspay += e.AmountFen
		case biz.LedgerPairReward, biz.LedgerPairRewardClawback, biz.LedgerPairUpline:
			pairUsdt += e.AmountFen
		case biz.LedgerPairRewardIspay:
			pairIspay += e.AmountFen
		case biz.LedgerManageReward:
			manageUsdt += e.AmountFen
		case biz.LedgerManageRewardIspay:
			manageIspay += e.AmountFen
		}
	}
	view.ApplyStatic(biz.BuildStaticView(view.IspayLockedMicro, days, released, pkg, view.IspayPriceFen, gotUsdt, gotIspay))
	view.DirectUsdtFen = directUsdt
	view.DirectIspayMicro = directIspay
	view.PairUsdtFen = pairUsdt
	view.PairIspayMicro = pairIspay
	view.ManageUsdtFen = manageUsdt
	view.ManageIspayMicro = manageIspay
	return view, nil
}

func (r *shopRepo) GetSettings(ctx context.Context) biz.SiteSettings {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	return r.data.settingsLocked()
}

func (r *shopRepo) Recharge(ctx context.Context, userID, amountFen int64, txHash string) (*biz.LedgerEntry, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	r.data.ensureAssetMaps()
	hash := biz.NormalizeTxHash(txHash)
	if hash != "" {
		if _, ok := r.data.txByHash[hash]; ok {
			return nil, biz.ErrRechargeTxUsed
		}
	}
	r.data.balances[userID] += amountFen
	refType := "recharge"
	if hash != "" {
		refType = "tx"
	}
	e := r.data.appendAssetLedger(userID, amountFen, r.data.balances[userID], biz.LedgerRecharge, refType, 0)
	e.TxHash = hash
	if hash != "" {
		r.data.txByHash[hash] = e.ID
	}
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return cloneLedger(e), nil
}

func (r *shopRepo) Withdraw(ctx context.Context, userID, amountFen int64) (*biz.LedgerEntry, error) {
	return r.WithdrawAsset(ctx, userID, amountFen, biz.AssetUSDT, "")
}

func (r *shopRepo) WithdrawAsset(ctx context.Context, userID, amount int64, asset, txHash string) (*biz.LedgerEntry, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	r.data.ensureAssetMaps()
	if asset == "" || asset == biz.AssetUSDT {
		if r.data.balances[userID] < amount {
			return nil, biz.ErrInsufficientBalance
		}
		r.data.balances[userID] -= amount
		e := r.data.appendAssetLedger(userID, -amount, r.data.balances[userID], biz.LedgerWithdraw, "withdraw", 0)
		e.TxHash = biz.NormalizeTxHash(txHash)
		if err := r.data.save(); err != nil {
			return nil, err
		}
		return cloneLedger(e), nil
	}
	if asset != biz.AssetIspay {
		return nil, biz.ErrInvalidArgument
	}
	if r.data.ispayFree[userID] < amount {
		return nil, biz.ErrInsufficientBalance
	}
	r.data.ispayFree[userID] -= amount
	e := r.data.appendAssetLedger(userID, -amount, r.data.ispayFree[userID], biz.LedgerIspayWithdraw, "withdraw", 0)
	e.TxHash = biz.NormalizeTxHash(txHash)
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return cloneLedger(e), nil
}

func (r *shopRepo) ListLedger(ctx context.Context, userID int64, page, pageSize int32, typ string) ([]*biz.LedgerEntry, int64, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	var all []*biz.LedgerEntry
	for i := len(r.data.ledger) - 1; i >= 0; i-- {
		e := r.data.ledger[i]
		if e.UserID != userID {
			continue
		}
		if !ledgerTypeMatch(e.Type, typ) {
			continue
		}
		all = append(all, cloneLedger(e))
	}
	out, total := slicePage(all, page, pageSize)
	return out, total, nil
}

func (r *shopRepo) PlaceOrder(ctx context.Context, userID, productID, qty int64) (*biz.Order, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	p, ok := r.data.products[productID]
	if !ok {
		return nil, biz.ErrProductNotFound
	}
	if p.Stock < qty {
		return nil, biz.ErrInsufficientStock
	}
	if p.PriceFen <= 0 {
		return nil, biz.ErrInvalidArgument
	}
	total := p.PriceFen * qty
	if r.data.balances[userID] < total {
		return nil, biz.ErrInsufficientBalance
	}
	p.Stock -= qty
	r.data.balances[userID] -= total
	r.data.orderSeq++
	o := &biz.Order{
		ID:         r.data.orderSeq,
		UserID:     userID,
		Status:     biz.OrderPaid,
		TotalFen:   total,
		CreatedAt:  time.Now().Unix(),
		CategoryID: p.CategoryID,
		Items: []*biz.OrderItem{{
			ProductID: p.ID,
			Name:      p.Name,
			PriceFen:  p.PriceFen,
			Quantity:  qty,
			AmountFen: total,
		}},
	}
	if days := biz.ReleaseDaysByCategory(p.CategoryID); days > 0 {
		o.CoinsMicro = biz.CoinsMicroFromPay(total, p.CategoryID) // 本单折枚数；用户锁仓见 syncWeb3LockLocked（认购总额）
		o.ReleaseDays = days
		o.ReleasedDays = 0
	}
	r.data.orders[o.ID] = o
	r.data.appendAssetLedger(userID, -total, r.data.balances[userID], biz.LedgerOrderPay, "order", o.ID)
	r.payDirectReward(userID, o.ID, total)
	if buyer := r.data.users[userID]; buyer != nil {
		buyer.PerfFen += total
	}
	r.data.syncWeb3LockLocked(userID)
	r.data.tryInstantPairLocked(userID)
	r.data.grantUnfreezeByOrderLocked(userID, o)
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return cloneOrder(o), nil
}

// PlaceCart 一次扣认购合计。档位用 EffectivePackageFen(合计)，锁仓用 CoinsMicroFromPay(合计, 品类)。
// 合计不是档位，超额 = 合计 − 档位，同兑换价折 IsPay；不要把 EffectivePackageFen 传入 CoinsMicroFromPay。
func (r *shopRepo) PlaceCart(ctx context.Context, userID int64, productIDs []int64) (*biz.Order, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	ids := uniquePositiveIDs(productIDs)
	if len(ids) == 0 {
		return nil, biz.ErrCartEmpty
	}
	items := make([]*biz.OrderItem, 0, len(ids))
	var total int64
	var cat int64
	for _, id := range ids {
		p, ok := r.data.products[id]
		if !ok || p.Status != biz.ProductOnSale || !biz.IsWeb3Category(p.CategoryID) {
			return nil, biz.ErrProductNotFound
		}
		if p.PriceFen <= 0 {
			return nil, biz.ErrInvalidArgument
		}
		if p.Stock < 1 {
			return nil, biz.ErrInsufficientStock
		}
		if cat == 0 {
			cat = p.CategoryID
		} else if p.CategoryID != cat {
			return nil, biz.ErrCartMixedDays
		}
		total += p.PriceFen
		items = append(items, &biz.OrderItem{
			ProductID: p.ID,
			Name:      p.Name,
			PriceFen:  p.PriceFen,
			Quantity:  1,
			AmountFen: p.PriceFen,
		})
	}
	if len(items) == 0 || total <= 0 {
		return nil, biz.ErrCartEmpty
	}
	if r.data.balances[userID] < total {
		return nil, biz.ErrInsufficientBalance
	}
	for _, it := range items {
		r.data.products[it.ProductID].Stock -= 1
	}
	r.data.balances[userID] -= total
	r.data.orderSeq++
	o := &biz.Order{
		ID:         r.data.orderSeq,
		UserID:     userID,
		Status:     biz.OrderPaid,
		TotalFen:   total,
		CreatedAt:  time.Now().Unix(),
		CategoryID: cat,
		Items:      items,
	}
	if days := biz.ReleaseDaysByCategory(cat); days > 0 {
		o.CoinsMicro = biz.CoinsMicroFromPay(total, cat)
		o.ReleaseDays = days
		o.ReleasedDays = 0
	}
	r.data.orders[o.ID] = o
	r.data.appendAssetLedger(userID, -total, r.data.balances[userID], biz.LedgerOrderPay, "order", o.ID)
	r.payDirectReward(userID, o.ID, total)
	if buyer := r.data.users[userID]; buyer != nil {
		buyer.PerfFen += total
	}
	r.data.syncWeb3LockLocked(userID)
	r.data.tryInstantPairLocked(userID)
	r.data.grantUnfreezeByOrderLocked(userID, o)
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return cloneOrder(o), nil
}

func uniquePositiveIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (r *shopRepo) GetOrder(ctx context.Context, userID, orderID int64) (*biz.Order, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	o, ok := r.data.orders[orderID]
	if !ok || o.UserID != userID {
		return nil, biz.ErrOrderNotFound
	}
	return cloneOrder(o), nil
}

func (r *shopRepo) ListOrders(ctx context.Context, userID int64, page, pageSize int32) ([]*biz.Order, int64, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	var all []*biz.Order
	for _, o := range r.data.orders {
		if o.UserID == userID {
			all = append(all, cloneOrder(o))
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID > all[j].ID })
	out, total := slicePage(all, page, pageSize)
	return out, total, nil
}

func (r *shopRepo) CancelOrder(ctx context.Context, userID, orderID int64) (*biz.Order, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	o, ok := r.data.orders[orderID]
	if !ok || o.UserID != userID {
		return nil, biz.ErrOrderNotFound
	}
	if o.Status != biz.OrderPaid {
		return nil, biz.ErrOrderNotCancellable
	}
	for _, it := range o.Items {
		if p, ok := r.data.products[it.ProductID]; ok {
			p.Stock += it.Quantity
		}
	}
	r.data.balances[userID] += o.TotalFen
	o.Status = biz.OrderCancelled
	r.data.appendAssetLedger(userID, o.TotalFen, r.data.balances[userID], biz.LedgerOrderRefund, "order", o.ID)
	r.clawbackDirectReward(userID, o.ID)
	if buyer := r.data.users[userID]; buyer != nil {
		buyer.PerfFen -= o.TotalFen
		if buyer.PerfFen < 0 {
			buyer.PerfFen = 0
		}
	}
	r.data.syncWeb3LockLocked(userID)
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return cloneOrder(o), nil
}

func (r *shopRepo) payDirectReward(buyerID, orderID, totalFen int64) {
	if buyer := r.data.users[buyerID]; buyer != nil && buyer.SkipUplineReward {
		return
	}
	bonus := biz.DirectRewardFen(totalFen)
	inviterID := r.inviterID(buyerID)
	if bonus <= 0 || inviterID == 0 || inviterID == buyerID {
		return
	}
	if _, ok := r.data.users[inviterID]; !ok {
		return
	}
	r.data.creditCappedDynamicLocked(inviterID, bonus, biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "order", orderID)
}

func (r *shopRepo) clawbackDirectReward(buyerID, orderID int64) {
	inviterID := r.inviterID(buyerID)
	if inviterID == 0 || inviterID == buyerID {
		return
	}
	if _, ok := r.data.users[inviterID]; !ok {
		return
	}
	var usdt, micro, paidAt int64
	for _, e := range r.data.ledger {
		if e == nil || e.RefType != "order" || e.RefID != orderID || e.UserID != inviterID {
			continue
		}
		if e.Type == biz.LedgerDirectReward {
			usdt += e.AmountFen
			if paidAt == 0 {
				paidAt = e.CreatedAt
			}
		}
		if e.Type == biz.LedgerDirectRewardIspay {
			micro += e.AmountFen
			if paidAt == 0 {
				paidAt = e.CreatedAt
			}
		}
	}
	r.data.ensureAssetMaps()
	if usdt != 0 {
		post := r.data.debitUsdtPreferFrozenLocked(inviterID, usdt)
		r.data.appendAssetLedger(inviterID, -usdt, post, biz.LedgerDirectRewardClawback, "order", orderID)
	}
	if micro != 0 {
		post := r.data.debitIspayPreferFrozenLocked(inviterID, micro)
		r.data.appendAssetLedger(inviterID, -micro, post, biz.LedgerDirectRewardIspayClawback, "order", orderID)
	}
	if (usdt != 0 || micro != 0) && paidAt > 0 {
		valueFen := biz.ReconstructSplitValueFen(usdt, micro, r.data.priceFenLocked())
		r.data.refundDynamicUsedLocked(r.data.users[inviterID], valueFen, time.Unix(paidAt, 0))
	}
}

func (r *shopRepo) inviterID(userID int64) int64 {
	u, ok := r.data.users[userID]
	if !ok || u == nil {
		return 0
	}
	return u.InviterID
}
