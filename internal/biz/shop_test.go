package biz

import (
	"context"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type memShop struct {
	products  map[int64]*Product
	balances  map[int64]int64
	ledger    []*LedgerEntry
	orders    map[int64]*Order
	inviterOf map[int64]int64
	ledgerN   int64
	orderN    int64
}

func newMemShop() *memShop {
	return &memShop{
		products: map[int64]*Product{
			1: {ID: 1, Name: "测试商品", PriceFen: 1000, Stock: 10},
		},
		balances:  map[int64]int64{},
		orders:    map[int64]*Order{},
		inviterOf: map[int64]int64{},
	}
}

func (s *memShop) Recharge(ctx context.Context, userID, amountFen int64) (*LedgerEntry, error) {
	s.balances[userID] += amountFen
	s.ledgerN++
	e := &LedgerEntry{ID: s.ledgerN, UserID: userID, AmountFen: amountFen, BalanceFen: s.balances[userID], Type: LedgerRecharge, CreatedAt: time.Now().Unix()}
	s.ledger = append(s.ledger, e)
	cp := *e
	return &cp, nil
}

func (s *memShop) GetBalance(ctx context.Context, userID int64) (int64, error) {
	return s.balances[userID], nil
}

func (s *memShop) ListLedger(ctx context.Context, userID int64, page, pageSize int32) ([]*LedgerEntry, int64, error) {
	var all []*LedgerEntry
	for i := len(s.ledger) - 1; i >= 0; i-- {
		if s.ledger[i].UserID == userID {
			cp := *s.ledger[i]
			all = append(all, &cp)
		}
	}
	return all, int64(len(all)), nil
}

func (s *memShop) PlaceOrder(ctx context.Context, userID, productID, qty int64) (*Order, error) {
	p, ok := s.products[productID]
	if !ok {
		return nil, ErrProductNotFound
	}
	if p.Stock < qty {
		return nil, ErrInsufficientStock
	}
	total := p.PriceFen * qty
	if s.balances[userID] < total {
		return nil, ErrInsufficientBalance
	}
	p.Stock -= qty
	s.balances[userID] -= total
	s.orderN++
	o := &Order{ID: s.orderN, UserID: userID, Status: OrderPaid, TotalFen: total, CreatedAt: time.Now().Unix(), Items: []*OrderItem{{ProductID: p.ID, Name: p.Name, PriceFen: p.PriceFen, Quantity: qty, AmountFen: total}}}
	s.orders[o.ID] = o
	s.payDirect(userID, o.ID, total)
	cp := *o
	return &cp, nil
}

func (s *memShop) GetOrder(ctx context.Context, userID, orderID int64) (*Order, error) {
	o, ok := s.orders[orderID]
	if !ok || o.UserID != userID {
		return nil, ErrOrderNotFound
	}
	cp := *o
	return &cp, nil
}

func (s *memShop) ListOrders(ctx context.Context, userID int64, page, pageSize int32) ([]*Order, int64, error) {
	var all []*Order
	for _, o := range s.orders {
		if o.UserID == userID {
			cp := *o
			all = append(all, &cp)
		}
	}
	return all, int64(len(all)), nil
}

func (s *memShop) CancelOrder(ctx context.Context, userID, orderID int64) (*Order, error) {
	o, ok := s.orders[orderID]
	if !ok || o.UserID != userID {
		return nil, ErrOrderNotFound
	}
	if o.Status != OrderPaid {
		return nil, ErrOrderNotCancellable
	}
	for _, it := range o.Items {
		if p, ok := s.products[it.ProductID]; ok {
			p.Stock += it.Quantity
		}
	}
	s.balances[userID] += o.TotalFen
	o.Status = OrderCancelled
	s.clawbackDirect(userID, o.ID, o.TotalFen)
	cp := *o
	return &cp, nil
}

func (s *memShop) payDirect(buyerID, orderID, totalFen int64) {
	bonus := DirectRewardFen(totalFen)
	inviterID := s.inviterOf[buyerID]
	if bonus <= 0 || inviterID == 0 {
		return
	}
	s.balances[inviterID] += bonus
	s.ledgerN++
	s.ledger = append(s.ledger, &LedgerEntry{ID: s.ledgerN, UserID: inviterID, AmountFen: bonus, BalanceFen: s.balances[inviterID], Type: LedgerDirectReward, RefType: "order", RefID: orderID})
}

func (s *memShop) clawbackDirect(buyerID, orderID, totalFen int64) {
	bonus := DirectRewardFen(totalFen)
	inviterID := s.inviterOf[buyerID]
	if bonus <= 0 || inviterID == 0 {
		return
	}
	s.balances[inviterID] -= bonus
	s.ledgerN++
	s.ledger = append(s.ledger, &LedgerEntry{ID: s.ledgerN, UserID: inviterID, AmountFen: -bonus, BalanceFen: s.balances[inviterID], Type: LedgerDirectRewardClawback, RefType: "order", RefID: orderID})
}

func TestShopRechargeAndOrder(t *testing.T) {
	InitAuth("test-secret")
	repo := newMemUserRepo()
	users := NewUserUsecase(repo, log.DefaultLogger)
	shop := newMemShop()
	wallets := NewWalletUsecase(shop, users, log.DefaultLogger)
	orders := NewOrderUsecase(shop, users, log.DefaultLogger)

	u, _, err := repo.GetOrCreateByWallet(context.Background(), "0x1234567890abcdef1234567890abcdef12345678")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := GenerateToken(u.ID, u.WalletAddress, 0)
	if err != nil {
		t.Fatal(err)
	}
	uid, wallet, ver, err := ParseToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithTokenVer(WithWallet(WithUserID(context.Background(), uid), wallet), ver)

	if _, err := orders.Create(ctx, 1, 1); err == nil {
		t.Fatal("expected insufficient balance")
	}
	if _, _, err := wallets.Recharge(ctx, 3000); err != nil {
		t.Fatal(err)
	}
	o, err := orders.Create(ctx, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if o.TotalFen != 2000 {
		t.Fatalf("total %d", o.TotalFen)
	}
	bal, err := wallets.GetWallet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bal != 1000 {
		t.Fatalf("bal %d", bal)
	}
	if shop.products[1].Stock != 8 {
		t.Fatalf("stock %d", shop.products[1].Stock)
	}
	if _, err := orders.Cancel(ctx, o.ID); err == nil {
		t.Fatal("user must not cancel own subscription")
	}
	bal, _ = wallets.GetWallet(ctx)
	if bal != 1000 {
		t.Fatalf("bal after forbidden cancel %d", bal)
	}
	if shop.products[1].Stock != 8 {
		t.Fatalf("stock must stay sold %d", shop.products[1].Stock)
	}
}

func TestDirectRewardOnSubscribe(t *testing.T) {
	InitAuth("test-secret")
	repo := newMemUserRepo()
	users := NewUserUsecase(repo, log.DefaultLogger)
	shop := newMemShop()
	wallets := NewWalletUsecase(shop, users, log.DefaultLogger)
	orders := NewOrderUsecase(shop, users, log.DefaultLogger)

	a, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	b, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	shop.inviterOf[b.ID] = a.ID

	tokB, _ := GenerateToken(b.ID, b.WalletAddress, 0)
	bid, bw, bver, _ := ParseToken(tokB)
	ctxB := WithTokenVer(WithWallet(WithUserID(context.Background(), bid), bw), bver)

	if _, _, err := wallets.Recharge(ctxB, 10000); err != nil {
		t.Fatal(err)
	}
	o, err := orders.Create(ctxB, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if o.TotalFen != 2000 {
		t.Fatalf("total %d", o.TotalFen)
	}
	if got := shop.balances[a.ID]; got != DirectRewardFen(2000) {
		t.Fatalf("inviter bonus %d", got)
	}
	if DirectRewardFen(2000) != 200 {
		t.Fatalf("10%% of 2000 = %d", DirectRewardFen(2000))
	}
	if _, err := orders.Cancel(ctxB, o.ID); err == nil {
		t.Fatal("user must not cancel own subscription")
	}
	if got := shop.balances[a.ID]; got != DirectRewardFen(2000) {
		t.Fatalf("direct reward should remain %d", got)
	}
}
