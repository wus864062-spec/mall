package biz

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type memShop struct {
	products  map[int64]*Product
	balances  map[int64]int64
	ispayFree map[int64]int64
	ledger    []*LedgerEntry
	orders    map[int64]*Order
	inviterOf map[int64]int64
	ledgerN   int64
	orderN    int64
	settings  SiteSettings
	chain     ChainConfig
}

func newMemShop() *memShop {
	return &memShop{
		products: map[int64]*Product{
			1: {ID: 1, Name: "测试商品", PriceFen: U(10), Stock: 10},
		},
		balances:  map[int64]int64{},
		ispayFree: map[int64]int64{},
		orders:    map[int64]*Order{},
		inviterOf: map[int64]int64{},
		settings:  DefaultSiteSettings(),
	}
}

func dummyTx(n int) string {
	return fmt.Sprintf("0x%064x", n)
}

type fakePayout struct {
	hash    string
	err     error
	lastTo  string
	lastFen int64
}

func (f *fakePayout) SendUSDT(ctx context.Context, to string, netFen int64) (string, error) {
	f.lastTo = to
	f.lastFen = netFen
	if f.err != nil {
		return "", f.err
	}
	if f.hash != "" {
		return f.hash, nil
	}
	return dummyTx(77), nil
}

type fakeChain struct {
	payer string
	num   int64
	err   error
}

func (f *fakeChain) VerifyRechargeTx(ctx context.Context, txHash, buyAddr string) (*ChainReceipt, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &ChainReceipt{Payer: f.payer, Num: f.num, To: buyAddr}, nil
}

func (s *memShop) Recharge(ctx context.Context, userID, amountFen int64, txHash string) (*LedgerEntry, error) {
	hash := NormalizeTxHash(txHash)
	if hash != "" {
		for _, e := range s.ledger {
			if e != nil && NormalizeTxHash(e.TxHash) == hash {
				return nil, ErrRechargeTxUsed
			}
		}
	}
	s.balances[userID] += amountFen
	s.ledgerN++
	refType := "recharge"
	if hash != "" {
		refType = "tx"
	}
	e := &LedgerEntry{ID: s.ledgerN, UserID: userID, AmountFen: amountFen, BalanceFen: s.balances[userID], Type: LedgerRecharge, RefType: refType, TxHash: hash, CreatedAt: time.Now().Unix()}
	s.ledger = append(s.ledger, e)
	cp := *e
	return &cp, nil
}

func (s *memShop) Withdraw(ctx context.Context, userID, amountFen int64) (*LedgerEntry, error) {
	if s.balances[userID] < amountFen {
		return nil, ErrInsufficientBalance
	}
	s.balances[userID] -= amountFen
	s.ledgerN++
	e := &LedgerEntry{ID: s.ledgerN, UserID: userID, AmountFen: -amountFen, BalanceFen: s.balances[userID], Type: LedgerWithdraw, RefType: "withdraw", CreatedAt: time.Now().Unix()}
	s.ledger = append(s.ledger, e)
	cp := *e
	return &cp, nil
}

func (s *memShop) GetBalance(ctx context.Context, userID int64) (int64, error) {
	return s.balances[userID], nil
}

func (s *memShop) WithdrawAsset(ctx context.Context, userID, amount int64, asset, txHash string) (*LedgerEntry, error) {
	if asset == AssetIspay {
		if s.ispayFree[userID] < amount {
			return nil, ErrInsufficientBalance
		}
		s.ispayFree[userID] -= amount
		s.ledgerN++
		e := &LedgerEntry{ID: s.ledgerN, UserID: userID, AmountFen: -amount, BalanceFen: s.ispayFree[userID], Type: LedgerIspayWithdraw, RefType: "withdraw", TxHash: NormalizeTxHash(txHash), CreatedAt: time.Now().Unix()}
		s.ledger = append(s.ledger, e)
		cp := *e
		return &cp, nil
	}
	if s.balances[userID] < amount {
		return nil, ErrInsufficientBalance
	}
	s.balances[userID] -= amount
	s.ledgerN++
	e := &LedgerEntry{ID: s.ledgerN, UserID: userID, AmountFen: -amount, BalanceFen: s.balances[userID], Type: LedgerWithdraw, RefType: "withdraw", TxHash: NormalizeTxHash(txHash), CreatedAt: time.Now().Unix()}
	s.ledger = append(s.ledger, e)
	cp := *e
	return &cp, nil
}

func (s *memShop) GetWalletView(ctx context.Context, userID int64) (*WalletView, error) {
	return &WalletView{BalanceFen: s.balances[userID], IspayFreeMicro: s.ispayFree[userID], IspayPriceFen: DefaultIspayPriceFen}, nil
}

func (s *memShop) GetSettings(ctx context.Context) SiteSettings {
	if s.settings.IspayPriceFen == 0 && s.settings.MaxRechargeFen == 0 {
		return DefaultSiteSettings()
	}
	return s.settings.Normalize()
}

func (s *memShop) GetChainConfig(ctx context.Context) *ChainConfig {
	return EnsureChainConfig(&s.chain)
}

func (s *memShop) ListLedger(ctx context.Context, userID int64, page, pageSize int32, typ string) ([]*LedgerEntry, int64, error) {
	var all []*LedgerEntry
	for i := len(s.ledger) - 1; i >= 0; i-- {
		if s.ledger[i].UserID != userID {
			continue
		}
		if typ != "" {
			ok := false
			for _, t := range strings.Split(typ, ",") {
				if strings.TrimSpace(t) == s.ledger[i].Type {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		cp := *s.ledger[i]
		all = append(all, &cp)
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

func (s *memShop) PlaceCart(ctx context.Context, userID int64, productIDs []int64) (*Order, error) {
	if len(productIDs) == 0 {
		return nil, ErrCartEmpty
	}
	items := make([]*OrderItem, 0, len(productIDs))
	var total int64
	var cat int64
	seen := map[int64]struct{}{}
	for _, id := range productIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		p, ok := s.products[id]
		if !ok {
			return nil, ErrProductNotFound
		}
		if cat == 0 {
			cat = p.CategoryID
		} else if p.CategoryID != cat {
			return nil, ErrCartMixedDays
		}
		if p.Stock < 1 {
			return nil, ErrInsufficientStock
		}
		total += p.PriceFen
		items = append(items, &OrderItem{ProductID: p.ID, Name: p.Name, PriceFen: p.PriceFen, Quantity: 1, AmountFen: p.PriceFen})
	}
	if len(items) == 0 {
		return nil, ErrCartEmpty
	}
	if s.balances[userID] < total {
		return nil, ErrInsufficientBalance
	}
	for _, it := range items {
		s.products[it.ProductID].Stock -= 1
	}
	s.balances[userID] -= total
	s.orderN++
	o := &Order{ID: s.orderN, UserID: userID, Status: OrderPaid, TotalFen: total, CreatedAt: time.Now().Unix(), CategoryID: cat, Items: items}
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
	usdt, micro := SplitHalf(bonus, DefaultIspayPriceFen)
	s.balances[inviterID] += usdt
	s.ispayFree[inviterID] += micro
	s.ledgerN++
	s.ledger = append(s.ledger, &LedgerEntry{ID: s.ledgerN, UserID: inviterID, AmountFen: usdt, BalanceFen: s.balances[inviterID], Type: LedgerDirectReward, RefType: "order", RefID: orderID})
	if micro != 0 {
		s.ledgerN++
		s.ledger = append(s.ledger, &LedgerEntry{ID: s.ledgerN, UserID: inviterID, AmountFen: micro, BalanceFen: s.ispayFree[inviterID], Type: LedgerDirectRewardIspay, RefType: "order", RefID: orderID})
	}
}

func (s *memShop) clawbackDirect(buyerID, orderID, totalFen int64) {
	bonus := DirectRewardFen(totalFen)
	inviterID := s.inviterOf[buyerID]
	if bonus <= 0 || inviterID == 0 {
		return
	}
	usdt, micro := SplitHalf(bonus, DefaultIspayPriceFen)
	s.balances[inviterID] -= usdt
	s.ispayFree[inviterID] -= micro
	s.ledgerN++
	s.ledger = append(s.ledger, &LedgerEntry{ID: s.ledgerN, UserID: inviterID, AmountFen: -usdt, BalanceFen: s.balances[inviterID], Type: LedgerDirectRewardClawback, RefType: "order", RefID: orderID})
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
	wallets.SetChainVerifier(&fakeChain{payer: wallet, num: 30})

	if _, err := orders.Create(ctx, 1, 1); err == nil {
		t.Fatal("expected insufficient balance")
	}
	if _, _, err := wallets.Recharge(ctx, U(30), dummyTx(30)); err != nil {
		t.Fatal(err)
	}
	o, err := orders.Create(ctx, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if o.TotalFen != U(20) {
		t.Fatalf("total %d", o.TotalFen)
	}
	bal, err := wallets.GetWallet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bal != U(10) {
		t.Fatalf("bal %d", bal)
	}
	if shop.products[1].Stock != 8 {
		t.Fatalf("stock %d", shop.products[1].Stock)
	}
	if _, err := orders.Cancel(ctx, o.ID); err == nil {
		t.Fatal("user must not cancel own subscription")
	}
	bal, _ = wallets.GetWallet(ctx)
	if bal != U(10) {
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
	wallets.SetChainVerifier(&fakeChain{payer: bw, num: 100})

	if _, _, err := wallets.Recharge(ctxB, U(100), dummyTx(100)); err != nil {
		t.Fatal(err)
	}
	o, err := orders.Create(ctxB, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if o.TotalFen != U(20) {
		t.Fatalf("total %d", o.TotalFen)
	}
	wantUSDT, _ := SplitHalf(DirectRewardFen(U(20)), DefaultIspayPriceFen)
	if got := shop.balances[a.ID]; got != wantUSDT {
		t.Fatalf("inviter bonus %d", got)
	}
	if DirectRewardFen(U(20)) != U(2) {
		t.Fatalf("10%% of 20 = %d", DirectRewardFen(U(20)))
	}
	if _, err := orders.Cancel(ctxB, o.ID); err == nil {
		t.Fatal("user must not cancel own subscription")
	}
	if got := shop.balances[a.ID]; got != wantUSDT {
		t.Fatalf("direct reward should remain %d", got)
	}
}

func TestWalletWithdraw(t *testing.T) {
	InitAuth("test-secret")
	repo := newMemUserRepo()
	users := NewUserUsecase(repo, log.DefaultLogger)
	shop := newMemShop()
	wallets := NewWalletUsecase(shop, users, log.DefaultLogger)
	pay := &fakePayout{hash: dummyTx(77)}
	wallets.SetHotWallet(pay)
	u, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xcccccccccccccccccccccccccccccccccccccccc")
	tok, _ := GenerateToken(u.ID, u.WalletAddress, 0)
	uid, w, ver, _ := ParseToken(tok)
	ctx := WithTokenVer(WithWallet(WithUserID(context.Background(), uid), w), ver)
	wallets.SetChainVerifier(&fakeChain{payer: w, num: 50})
	if _, _, err := wallets.Withdraw(ctx, MinWithdrawFen); err == nil {
		t.Fatal("expected insufficient")
	}
	if _, _, err := wallets.Recharge(ctx, U(50), dummyTx(50)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := wallets.Withdraw(ctx, MinWithdrawFen-1); err == nil {
		t.Fatal("expected too small")
	}
	e, bal, err := wallets.Withdraw(ctx, U(20))
	if err != nil {
		t.Fatal(err)
	}
	if bal != U(30) || e.Type != LedgerWithdraw || e.AmountFen != -U(20) {
		t.Fatalf("entry %+v bal %d", e, bal)
	}
	if e.TxHash != dummyTx(77) {
		t.Fatalf("tx %s", e.TxHash)
	}
	if pay.lastFen != WithdrawNetFen(U(20), shop.GetSettings(ctx).WithdrawFeePercent) {
		t.Fatalf("chain send net %d, not gross", pay.lastFen)
	}
	if !strings.EqualFold(pay.lastTo, w) {
		t.Fatalf("payout to %s want %s", pay.lastTo, w)
	}
	if WithdrawFeeFen(U(20)) != U(2) {
		t.Fatal("fee 10%")
	}
}

func TestWithdrawUSDTSendFailDoesNotDeduct(t *testing.T) {
	InitAuth("test-secret")
	repo := newMemUserRepo()
	users := NewUserUsecase(repo, log.DefaultLogger)
	shop := newMemShop()
	wallets := NewWalletUsecase(shop, users, log.DefaultLogger)
	wallets.SetHotWallet(&fakePayout{err: ErrWithdrawChain})
	u, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	tok, _ := GenerateToken(u.ID, u.WalletAddress, 0)
	uid, w, ver, _ := ParseToken(tok)
	ctx := WithTokenVer(WithWallet(WithUserID(context.Background(), uid), w), ver)
	wallets.SetChainVerifier(&fakeChain{payer: w, num: 50})
	if _, _, err := wallets.Recharge(ctx, U(50), dummyTx(50)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := wallets.Withdraw(ctx, U(20)); err != ErrWithdrawChain {
		t.Fatalf("want chain fail, got %v", err)
	}
	if shop.balances[u.ID] != U(50) {
		t.Fatalf("send fail must not deduct, bal %d", shop.balances[u.ID])
	}
}

func TestWithdrawUSDTRequiresHotWallet(t *testing.T) {
	InitAuth("test-secret")
	repo := newMemUserRepo()
	users := NewUserUsecase(repo, log.DefaultLogger)
	shop := newMemShop()
	wallets := NewWalletUsecase(shop, users, log.DefaultLogger)
	wallets.SetHotWallet(disabledHotWallet{})
	u, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	tok, _ := GenerateToken(u.ID, u.WalletAddress, 0)
	uid, w, ver, _ := ParseToken(tok)
	ctx := WithTokenVer(WithWallet(WithUserID(context.Background(), uid), w), ver)
	wallets.SetChainVerifier(&fakeChain{payer: w, num: 50})
	if _, _, err := wallets.Recharge(ctx, U(50), dummyTx(50)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := wallets.Withdraw(ctx, U(20)); err != ErrWithdrawHotWallet {
		t.Fatalf("want hot wallet missing, got %v", err)
	}
	if shop.balances[u.ID] != U(50) {
		t.Fatal("no key must not deduct")
	}
}

func TestWalletRechargeAndWithdrawLimits(t *testing.T) {
	InitAuth("test-secret")
	repo := newMemUserRepo()
	users := NewUserUsecase(repo, log.DefaultLogger)
	shop := newMemShop()
	shop.settings.MaxRechargeFen = U(50)
	shop.settings.MinWithdrawFen = U(8)
	shop.settings.WithdrawFeePercent = 5
	wallets := NewWalletUsecase(shop, users, log.DefaultLogger)
	wallets.SetHotWallet(&fakePayout{hash: dummyTx(8)})
	u, _, err := repo.GetOrCreateByWallet(context.Background(), "0xdddddddddddddddddddddddddddddddddddddddd")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := GenerateToken(u.ID, u.WalletAddress, 0)
	if err != nil {
		t.Fatal(err)
	}
	uid, w, ver, err := ParseToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithTokenVer(WithWallet(WithUserID(context.Background(), uid), w), ver)
	wallets.SetChainVerifier(&fakeChain{payer: w, num: 20})

	if _, _, err := wallets.Recharge(ctx, 0, dummyTx(1)); err == nil {
		t.Fatal("expected invalid recharge")
	}
	if _, _, err := wallets.Recharge(ctx, U(51), dummyTx(2)); err == nil {
		t.Fatal("expected max recharge")
	}
	if _, _, err := wallets.Recharge(ctx, U(20), dummyTx(20)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := wallets.Withdraw(ctx, U(8)-1); err == nil {
		t.Fatal("expected min withdraw")
	}
	if _, _, err := wallets.Withdraw(ctx, U(8)); err != nil {
		t.Fatal(err)
	}
	if WithdrawFeeAmount(U(8), shop.GetSettings(ctx).WithdrawFeePercent) != U(8)*5/100 {
		t.Fatal("5% fee")
	}
}

func TestRechargeCreditsWholeUOneToOne(t *testing.T) {
	// 付 100U 记 100U（展示 100.0000），多余 fen 丢掉；不是收款口份额，也不把 fen 当展示单位。
	InitAuth("test-secret")
	repo := newMemUserRepo()
	users := NewUserUsecase(repo, log.DefaultLogger)
	shop := newMemShop()
	wallets := NewWalletUsecase(shop, users, log.DefaultLogger)
	u, _, err := repo.GetOrCreateByWallet(context.Background(), "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := GenerateToken(u.ID, u.WalletAddress, 0)
	if err != nil {
		t.Fatal(err)
	}
	uid, w, ver, err := ParseToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithTokenVer(WithWallet(WithUserID(context.Background(), uid), w), ver)
	wallets.SetChainVerifier(&fakeChain{payer: w, num: 100})
	e, bal, err := wallets.Recharge(ctx, U(100)+1234, dummyTx(100))
	if err != nil {
		t.Fatal(err)
	}
	if e.AmountFen != U(100) || FormatUstd(e.AmountFen) != "100.0000" {
		t.Fatalf("credited %d (%s) want 100U", e.AmountFen, FormatUstd(e.AmountFen))
	}
	if bal != U(100) {
		t.Fatalf("balance %d", bal)
	}
	if SplitByPorts(U(100), DefaultRechargePorts)[0] == e.AmountFen {
		t.Fatal("ledger must not use port share")
	}
}

func TestRechargeRequiresChainTx(t *testing.T) {
	InitAuth("test-secret")
	repo := newMemUserRepo()
	users := NewUserUsecase(repo, log.DefaultLogger)
	shop := newMemShop()
	wallets := NewWalletUsecase(shop, users, log.DefaultLogger)
	u, _, err := repo.GetOrCreateByWallet(context.Background(), "0xffffffffffffffffffffffffffffffffffffffff")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := GenerateToken(u.ID, u.WalletAddress, 0)
	if err != nil {
		t.Fatal(err)
	}
	uid, w, ver, err := ParseToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithTokenVer(WithWallet(WithUserID(context.Background(), uid), w), ver)

	if _, _, err := wallets.Recharge(ctx, U(10), ""); err == nil {
		t.Fatal("missing tx must fail")
	}
	wallets.SetChainVerifier(&fakeChain{payer: w, num: 10, err: ErrRechargeTxInvalid})
	if _, _, err := wallets.Recharge(ctx, U(10), dummyTx(10)); err == nil {
		t.Fatal("invalid tx must not credit")
	}
	if shop.balances[u.ID] != 0 {
		t.Fatalf("balance after fake tx %d", shop.balances[u.ID])
	}

	wallets.SetChainVerifier(&fakeChain{payer: "0x1111111111111111111111111111111111111111", num: 10})
	if _, _, err := wallets.Recharge(ctx, U(10), dummyTx(11)); err == nil {
		t.Fatal("wrong payer must fail")
	}

	wallets.SetChainVerifier(&fakeChain{payer: w, num: 10})
	if _, _, err := wallets.Recharge(ctx, U(10), dummyTx(12)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := wallets.Recharge(ctx, U(10), dummyTx(12)); err == nil {
		t.Fatal("reused hash must fail")
	}
	if shop.balances[u.ID] != U(10) {
		t.Fatalf("reuse must not add again %d", shop.balances[u.ID])
	}

	wallets.SetChainVerifier(&fakeChain{payer: w, num: 9})
	if _, _, err := wallets.Recharge(ctx, U(10), dummyTx(13)); err == nil {
		t.Fatal("amount mismatch must fail")
	}

	list, total, err := wallets.ListLedger(ctx, 1, 20, LedgerRecharge)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 || list[0].Type != LedgerRecharge {
		t.Fatalf("recharge ledger total=%d n=%d", total, len(list))
	}
}

func TestRechargeRemainFenExcludesRewards(t *testing.T) {
	// 充 2000、认购 1000、提 500 剩 500；静态/调账/别人的充值都不进这个数。
	got := RechargeRemainFen([]*LedgerEntry{
		{UserID: 1, AmountFen: U(2000), Type: LedgerRecharge},
		{UserID: 1, AmountFen: -U(1000), Type: LedgerOrderPay},
		{UserID: 1, AmountFen: -U(500), Type: LedgerWithdraw},
		{UserID: 1, AmountFen: U(80), Type: LedgerStaticReward},
		{UserID: 1, AmountFen: U(50), Type: LedgerAdminAdjust},
		{UserID: 1, AmountFen: U(30), Type: LedgerDirectReward},
		{UserID: 1, AmountFen: U(20), Type: LedgerUnfreeze},
		{UserID: 2, AmountFen: U(999), Type: LedgerRecharge},
	}, 1)
	if got != U(500) {
		t.Fatalf("remain %d want 500U，不能加奖励/调账/解冻", got)
	}
	if RechargeRemainFen([]*LedgerEntry{
		{UserID: 1, AmountFen: U(200), Type: LedgerRecharge},
		{UserID: 1, AmountFen: U(200), Type: LedgerOrderRefund},
		{UserID: 1, AmountFen: -U(100), Type: LedgerOrderPay},
	}, 1) != U(300) {
		t.Fatal("退款加回充值剩余")
	}
}
