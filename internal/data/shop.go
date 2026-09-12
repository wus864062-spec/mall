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
	return r.data.balances[userID], nil
}

func (r *shopRepo) Recharge(ctx context.Context, userID, amountFen int64) (*biz.LedgerEntry, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	r.data.balances[userID] += amountFen
	e := r.appendLedger(userID, amountFen, biz.LedgerRecharge, "recharge", 0)
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return cloneLedger(e), nil
}

func (r *shopRepo) ListLedger(ctx context.Context, userID int64, page, pageSize int32) ([]*biz.LedgerEntry, int64, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	var all []*biz.LedgerEntry
	for i := len(r.data.ledger) - 1; i >= 0; i-- {
		e := r.data.ledger[i]
		if e.UserID == userID {
			all = append(all, cloneLedger(e))
		}
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
		ID:        r.data.orderSeq,
		UserID:    userID,
		Status:    biz.OrderPaid,
		TotalFen:  total,
		CreatedAt: time.Now().Unix(),
		Items: []*biz.OrderItem{{
			ProductID: p.ID,
			Name:      p.Name,
			PriceFen:  p.PriceFen,
			Quantity:  qty,
			AmountFen: total,
		}},
	}
	r.data.orders[o.ID] = o
	r.appendLedger(userID, -total, biz.LedgerOrderPay, "order", o.ID)
	r.payDirectReward(userID, o.ID, total)
	if buyer := r.data.users[userID]; buyer != nil {
		buyer.PerfFen += total
	}
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return cloneOrder(o), nil
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
	r.appendLedger(userID, o.TotalFen, biz.LedgerOrderRefund, "order", o.ID)
	r.clawbackDirectReward(userID, o.ID, o.TotalFen)
	if buyer := r.data.users[userID]; buyer != nil {
		buyer.PerfFen -= o.TotalFen
		if buyer.PerfFen < 0 {
			buyer.PerfFen = 0
		}
	}
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return cloneOrder(o), nil
}

func (r *shopRepo) payDirectReward(buyerID, orderID, totalFen int64) {
	bonus := biz.DirectRewardFen(totalFen)
	inviterID := r.inviterID(buyerID)
	if bonus <= 0 || inviterID == 0 || inviterID == buyerID {
		return
	}
	if _, ok := r.data.users[inviterID]; !ok {
		return
	}
	r.data.balances[inviterID] += bonus
	r.appendLedger(inviterID, bonus, biz.LedgerDirectReward, "order", orderID)
}

func (r *shopRepo) clawbackDirectReward(buyerID, orderID, totalFen int64) {
	bonus := biz.DirectRewardFen(totalFen)
	inviterID := r.inviterID(buyerID)
	if bonus <= 0 || inviterID == 0 || inviterID == buyerID {
		return
	}
	if _, ok := r.data.users[inviterID]; !ok {
		return
	}
	r.data.balances[inviterID] -= bonus
	r.appendLedger(inviterID, -bonus, biz.LedgerDirectRewardClawback, "order", orderID)
}

func (r *shopRepo) inviterID(userID int64) int64 {
	u, ok := r.data.users[userID]
	if !ok || u == nil {
		return 0
	}
	return u.InviterID
}

func (r *shopRepo) appendLedger(userID, amountFen int64, typ, refType string, refID int64) *biz.LedgerEntry {
	r.data.ledgerSeq++
	e := &biz.LedgerEntry{
		ID:         r.data.ledgerSeq,
		UserID:     userID,
		AmountFen:  amountFen,
		BalanceFen: r.data.balances[userID],
		Type:       typ,
		RefType:    refType,
		RefID:      refID,
		CreatedAt:  time.Now().Unix(),
	}
	r.data.ledger = append(r.data.ledger, e)
	return e
}
