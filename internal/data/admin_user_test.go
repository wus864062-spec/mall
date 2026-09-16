package data

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"mall/internal/biz"
)

func TestAdminUserActions(t *testing.T) {
	d := &Data{
		path:        filepath.Join(t.TempDir(), "mall.json"),
		users:       map[int64]*biz.User{},
		byWallet:    map[string]int64{},
		byInvite:    map[string]int64{},
		products:    map[int64]*biz.Product{},
		balances:    map[int64]int64{},
		ispayLocked: map[int64]int64{},
		ispayFree:   map[int64]int64{},
		orders:      map[int64]*biz.Order{},
	}
	d.users[1] = &biz.User{ID: 1, WalletAddress: "0xadmin", Nickname: "admin"}
	d.users[2] = &biz.User{ID: 2, WalletAddress: "0xaaa1111111111111111111111111111111111111", InviterID: 1}
	d.users[3] = &biz.User{ID: 3, WalletAddress: "0xbbb1111111111111111111111111111111111111", InviterID: 2}
	d.byWallet[d.users[2].WalletAddress] = 2
	d.byWallet[d.users[3].WalletAddress] = 3
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	repo := NewAdminRepo(d).(*adminRepo)
	ctx := context.Background()

	if err := repo.SetUserUSDT(ctx, 2, biz.U(50)); err != nil {
		t.Fatal(err)
	}
	if d.balances[2] != biz.U(50) {
		t.Fatalf("set usdt %d", d.balances[2])
	}
	if err := repo.AddUserUSDT(ctx, 2, biz.U(10)); err != nil {
		t.Fatal(err)
	}
	if d.balances[2] != biz.U(60) {
		t.Fatalf("add usdt %d", d.balances[2])
	}
	if err := repo.SetUserIspayFree(ctx, 2, 3*biz.IspayMicroPerCoin); err != nil {
		t.Fatal(err)
	}
	if d.ispayFree[2] != 3*biz.IspayMicroPerCoin {
		t.Fatalf("ispay %d", d.ispayFree[2])
	}

	n, err := repo.SetUserLocked(ctx, 2, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || !d.users[2].Locked || !d.users[3].Locked || d.users[1].Locked {
		t.Fatalf("lock line n=%d u2=%v u3=%v admin=%v", n, d.users[2].Locked, d.users[3].Locked, d.users[1].Locked)
	}
	if _, err := repo.SetUserLocked(ctx, 1, true, false); err != biz.ErrAdminProtected {
		t.Fatalf("lock admin %v", err)
	}

	if err := repo.ChangeUserWallet(ctx, 2, "0xCCC1111111111111111111111111111111111111"); err != nil {
		t.Fatal(err)
	}
	if d.users[2].WalletAddress != "0xccc1111111111111111111111111111111111111" {
		t.Fatalf("wallet %s", d.users[2].WalletAddress)
	}
	if err := repo.ChangeUserWallet(ctx, 2, d.users[3].WalletAddress); err != biz.ErrWalletTaken {
		t.Fatalf("want taken, got %v", err)
	}
}

func TestSkipUplineDirectReward(t *testing.T) {
	d := &Data{
		path:        filepath.Join(t.TempDir(), "mall.json"),
		users:       map[int64]*biz.User{},
		products:    map[int64]*biz.Product{},
		balances:    map[int64]int64{},
		ispayLocked: map[int64]int64{},
		ispayFree:   map[int64]int64{},
		orders:      map[int64]*biz.Order{},
	}
	d.users[1] = &biz.User{ID: 1}
	d.users[2] = &biz.User{ID: 2, InviterID: 1, SkipUplineReward: true}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[2] = biz.U(1000)
	if _, err := NewShopRepo(d).PlaceOrder(context.Background(), 2, 1, 1); err != nil {
		t.Fatal(err)
	}
	for _, e := range d.ledger {
		if e != nil && (e.Type == biz.LedgerDirectReward || e.Type == biz.LedgerDirectRewardIspay) {
			t.Fatalf("unexpected direct reward %+v", e)
		}
	}
}

func TestLastOrderAtAndListByUser(t *testing.T) {
	d := &Data{
		path:        filepath.Join(t.TempDir(), "mall.json"),
		users:       map[int64]*biz.User{1: {ID: 1}, 2: {ID: 2}},
		products:    map[int64]*biz.Product{},
		balances:    map[int64]int64{},
		ispayLocked: map[int64]int64{},
		ispayFree:   map[int64]int64{},
		orders:      map[int64]*biz.Order{},
	}
	if v := d.adminUserViewLocked(d.users[2]); v == nil || v.LastOrderAt != 0 {
		t.Fatalf("empty last %+v", v)
	}
	d.orders[1] = &biz.Order{ID: 1, UserID: 2, Status: biz.OrderPaid, CreatedAt: 100, TotalFen: biz.U(1000), Items: []*biz.OrderItem{{Name: "1000U"}}}
	d.orders[2] = &biz.Order{ID: 2, UserID: 2, Status: biz.OrderPaid, CreatedAt: 300, TotalFen: biz.U(3000), Items: []*biz.OrderItem{{Name: "3000U"}}}
	d.orders[3] = &biz.Order{ID: 3, UserID: 1, Status: biz.OrderPaid, CreatedAt: 900, TotalFen: biz.U(1000)}
	d.orders[4] = &biz.Order{ID: 4, UserID: 2, Status: biz.OrderCancelled, CreatedAt: 800}
	if v := d.adminUserViewLocked(d.users[2]); v.LastOrderAt != 300 {
		t.Fatalf("last %d", v.LastOrderAt)
	}
	if v := d.adminUserViewLocked(d.users[2]); v.PaidSumFen != biz.U(4000) {
		t.Fatalf("paid sum %d", v.PaidSumFen)
	}
	repo := NewAdminRepo(d)
	orders, users, total, err := repo.ListAllOrders(context.Background(), 1, 10, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(orders) != 3 {
		t.Fatalf("user2 orders total=%d n=%d", total, len(orders))
	}
	for i, o := range orders {
		if o.UserID != 2 {
			t.Fatalf("row %d user %d", i, o.UserID)
		}
		if users[i] == nil || users[i].ID != 2 {
			t.Fatalf("user row %+v", users[i])
		}
	}
}

func TestListAllLedgerUSDTTypesComma(t *testing.T) {
	d := &Data{
		path:     filepath.Join(t.TempDir(), "mall.json"),
		users:    map[int64]*biz.User{1: {ID: 1}},
		ledger:   []*biz.LedgerEntry{},
		balances: map[int64]int64{},
	}
	d.ledger = []*biz.LedgerEntry{
		{ID: 1, UserID: 1, AmountFen: biz.U(100), Type: biz.LedgerRecharge},
		{ID: 2, UserID: 1, AmountFen: biz.U(10), Type: biz.LedgerStaticReward},
		{ID: 3, UserID: 1, AmountFen: 1, Type: biz.LedgerStaticRewardIspay},
		{ID: 4, UserID: 1, AmountFen: biz.U(5), Type: biz.LedgerDirectReward},
	}
	repo := NewAdminRepo(d)
	list, total, err := repo.ListAllLedger(context.Background(), 1, 20, 0, biz.LedgerStaticReward+","+biz.LedgerDirectReward)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total %d want 2 USDT rewards", total)
	}
	for _, e := range list {
		if e.Type == biz.LedgerRecharge || e.Type == biz.LedgerStaticRewardIspay {
			t.Fatalf("unexpected %s", e.Type)
		}
	}
}

func TestListAllLedgerRewardTypesBothAssets(t *testing.T) {
	// 奖励同屏：USDT 行 AmountFen 是 fen（展示 U），IsPay 行 AmountFen 是 micro（展示枚）。充值不进这组。
	d := &Data{
		path:     filepath.Join(t.TempDir(), "mall.json"),
		users:    map[int64]*biz.User{1: {ID: 1}},
		ledger:   []*biz.LedgerEntry{},
		balances: map[int64]int64{},
	}
	d.ledger = []*biz.LedgerEntry{
		{ID: 1, UserID: 1, AmountFen: biz.U(100), Type: biz.LedgerRecharge},
		{ID: 2, UserID: 1, AmountFen: biz.U(10), Type: biz.LedgerStaticReward},
		{ID: 3, UserID: 1, AmountFen: 30_000_000, Type: biz.LedgerStaticRewardIspay},
		{ID: 4, UserID: 1, AmountFen: biz.U(5), Type: biz.LedgerDirectReward},
		{ID: 5, UserID: 1, AmountFen: 15_000_000, Type: biz.LedgerDirectRewardIspay},
	}
	repo := NewAdminRepo(d)
	typ := strings.Join([]string{
		biz.LedgerStaticReward, biz.LedgerStaticRewardIspay,
		biz.LedgerDirectReward, biz.LedgerDirectRewardIspay,
	}, ",")
	list, total, err := repo.ListAllLedger(context.Background(), 1, 20, 0, typ)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 {
		t.Fatalf("total %d want 4 (USDT+IsPay, no recharge)", total)
	}
	got := map[string]int64{}
	for _, e := range list {
		got[e.Type] = e.AmountFen
	}
	if got[biz.LedgerStaticReward] != biz.U(10) || biz.FormatUstd(got[biz.LedgerStaticReward]) != "10.0000" {
		t.Fatalf("USDT 用 fen 折 U %+v", got)
	}
	if got[biz.LedgerStaticRewardIspay] != 30_000_000 || biz.FormatIspay(got[biz.LedgerStaticRewardIspay]) != "0.30000000" {
		t.Fatalf("IsPay 用 micro 折枚 %+v", got)
	}
	if _, ok := got[biz.LedgerRecharge]; ok {
		t.Fatal("recharge must stay out")
	}
}

func TestFindUserIDByWalletNotNumericID(t *testing.T) {
	w1 := "0xABCDEF0000000000000000000000000000000001"
	w2 := "0x9999999999999999999999999999999999999999"
	d := &Data{
		path:     filepath.Join(t.TempDir(), "mall.json"),
		users:    map[int64]*biz.User{1: {ID: 1, WalletAddress: w1}, 2: {ID: 2, WalletAddress: w2}},
		byWallet: map[string]int64{strings.ToLower(w1): 1, strings.ToLower(w2): 2},
		ledger: []*biz.LedgerEntry{
			{ID: 1, UserID: 1, AmountFen: biz.U(10), Type: biz.LedgerStaticReward},
			{ID: 2, UserID: 2, AmountFen: biz.U(20), Type: biz.LedgerStaticReward},
			{ID: 3, UserID: 1, AmountFen: 1, Type: biz.LedgerRecharge},
		},
	}
	repo := NewAdminRepo(d).(*adminRepo)
	ctx := context.Background()
	if got := repo.FindUserIDByWallet(ctx, strings.ToUpper(w1)); got != 1 {
		t.Fatalf("exact case-insensitive %d", got)
	}
	if got := repo.FindUserIDByWallet(ctx, "abcdef0000000000000000000000000000000001"); got != 1 {
		t.Fatalf("unique suffix %d", got)
	}
	if got := repo.FindUserIDByWallet(ctx, "2"); got != 0 {
		t.Fatalf("numeric must not be user id, got %d", got)
	}
	if got := repo.FindUserIDByWallet(ctx, "0x"); got != 0 {
		t.Fatalf("multiple contains must be empty, got %d", got)
	}
	if got := repo.FindUserIDByWallet(ctx, "nonesuch"); got != 0 {
		t.Fatalf("missing %d", got)
	}
	uid := repo.FindUserIDByWallet(ctx, w1)
	list, total, err := repo.ListAllLedger(ctx, 1, 20, uid, biz.LedgerStaticReward)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || list[0].UserID != 1 || list[0].WalletAddress != w1 {
		t.Fatalf("wallet list %+v total %d", list, total)
	}
}

func TestSumRewardFenStaticDynamicUSDT(t *testing.T) {
	d := &Data{
		path:   filepath.Join(t.TempDir(), "mall.json"),
		users:  map[int64]*biz.User{1: {ID: 1}, 2: {ID: 2}},
		ledger: []*biz.LedgerEntry{},
	}
	d.ledger = []*biz.LedgerEntry{
		{UserID: 1, AmountFen: biz.U(10), Type: biz.LedgerStaticReward},
		{UserID: 1, AmountFen: 1, Type: biz.LedgerStaticRewardIspay},
		{UserID: 1, AmountFen: biz.U(4), Type: biz.LedgerDirectReward},
		{UserID: 1, AmountFen: -biz.U(1), Type: biz.LedgerDirectRewardClawback},
		{UserID: 1, AmountFen: biz.U(2), Type: biz.LedgerPairReward},
		{UserID: 1, AmountFen: biz.U(3), Type: biz.LedgerManageReward},
		{UserID: 1, AmountFen: biz.U(100), Type: biz.LedgerRecharge},
		{UserID: 2, AmountFen: biz.U(50), Type: biz.LedgerStaticReward},
	}
	repo := NewAdminRepo(d)
	st, dyn := repo.SumRewardFen(context.Background(), 1)
	if st != biz.U(10) {
		t.Fatalf("static %d want 10U", st)
	}
	if dyn != biz.U(4-1+2+3) {
		t.Fatalf("dynamic %d", dyn)
	}
	stAll, dynAll := repo.SumRewardFen(context.Background(), 0)
	if stAll != biz.U(60) || dynAll != dyn {
		t.Fatalf("all static %d dynamic %d", stAll, dynAll)
	}
}

func TestAdminMemberViewRechargeNotBalance(t *testing.T) {
	d := &Data{
		path:        filepath.Join(t.TempDir(), "mall.json"),
		users:       map[int64]*biz.User{},
		products:    map[int64]*biz.Product{},
		orders:      map[int64]*biz.Order{},
		balances:    map[int64]int64{},
		ispayLocked: map[int64]int64{},
		ledger:      []*biz.LedgerEntry{},
	}
	d.users[2] = &biz.User{
		ID: 2, WalletAddress: "0xbbb", StaticDays: 300, StaticReleasedDays: 12,
		StaticPackageFen: biz.U(3000), SettledPairFen: biz.U(400),
	}
	d.balances[2] = biz.U(50)
	d.ispayLocked[2] = biz.CoinsMicroFromPay(biz.U(4000), biz.CategoryWeb3)
	d.ledger = []*biz.LedgerEntry{
		{UserID: 2, AmountFen: biz.U(2000), Type: biz.LedgerRecharge},
		{UserID: 2, AmountFen: -biz.U(1000), Type: biz.LedgerOrderPay},
		{UserID: 2, AmountFen: -biz.U(500), Type: biz.LedgerWithdraw},
		{UserID: 2, AmountFen: biz.U(50), Type: biz.LedgerAdminAdjust},
		{UserID: 2, AmountFen: biz.U(80), Type: biz.LedgerStaticReward},
	}
	d.orders[1] = &biz.Order{ID: 1, UserID: 2, Status: biz.OrderPaid, TotalFen: biz.U(1000), CategoryID: biz.CategoryWeb3}
	d.orders[2] = &biz.Order{ID: 2, UserID: 2, Status: biz.OrderPaid, TotalFen: biz.U(3000), CategoryID: biz.CategoryWeb3}
	v := d.adminUserViewLocked(d.users[2])
	if v.RechargeFen != biz.U(500) {
		t.Fatalf("充值2000-认购1000-提现500 剩余 %d，不是累计充值也不是可提 %d", v.RechargeFen, v.BalanceFen)
	}
	if v.PaidSumFen != biz.U(4000) {
		t.Fatalf("认购金额用实付 1000+3000=%d，不是档位 %d", v.PaidSumFen, v.PackageFen)
	}
	if v.PackageFen == v.PaidSumFen {
		t.Fatal("认购金额不能等于档位")
	}
	if v.StaticDays != 300 || v.StaticReleasedDays != 12 {
		t.Fatalf("购买/天数要 12/300，got %d/%d", v.StaticReleasedDays, v.StaticDays)
	}
	wantLock := biz.CoinsMicroFromPay(biz.U(4000), biz.CategoryWeb3)
	if v.IspayLockedMicro != wantLock {
		t.Fatalf("购买IsPay用锁仓 %d want %d", v.IspayLockedMicro, wantLock)
	}
	if v.IspayLockedMicro == biz.CoinsMicroFromPay(v.PackageFen, biz.CategoryWeb3) && v.PackageFen != biz.U(4000) {
		t.Fatal("锁仓不能只按档位折")
	}
	if v.SettledPairFen != biz.U(400) {
		t.Fatalf("已碰用已消耗对碰 %d", v.SettledPairFen)
	}
	if v.BalanceFen == v.RechargeFen {
		t.Fatal("可提 USDT 不能当成充值余额")
	}
}
