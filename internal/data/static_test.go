package data

import (
	"context"
	"testing"

	"mall/internal/biz"
)

func TestWeb3ConvertAndStaticSplit(t *testing.T) {
	d := &Data{
		users:       map[int64]*biz.User{},
		products:    map[int64]*biz.Product{},
		balances:    map[int64]int64{},
		ispayLocked: map[int64]int64{},
		ispayFree:   map[int64]int64{},
		orders:      map[int64]*biz.Order{},
		path:        t.TempDir() + "/mall.json",
	}
	d.users[1] = &biz.User{ID: 1}
	d.users[2] = &biz.User{ID: 2, InviterID: 1}
	d.products[1] = &biz.Product{
		ID: 1, Name: "12000U", PriceFen: 12000 * biz.UstdScale, Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3_750,
	}
	d.balances[2] = 12000 * biz.UstdScale
	shop := NewShopRepo(d)
	o, err := shop.PlaceOrder(context.Background(), 2, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if o.CoinsMicro != 15*biz.IspayMicroPerCoin || o.ReleaseDays != 750 {
		t.Fatalf("order %+v", o)
	}
	if d.ispayLocked[2] != 15*biz.IspayMicroPerCoin {
		t.Fatalf("locked %d", d.ispayLocked[2])
	}
	wantUSDT, wantMicro := biz.SplitHalf(biz.DirectRewardFen(biz.U(12000)), biz.DefaultIspayPriceFen)
	if d.frozenUsdt[1] != wantUSDT || d.ispayFrozen[1] != wantMicro {
		t.Fatalf("direct frozen usdt=%d ispay=%d", d.frozenUsdt[1], d.ispayFrozen[1])
	}
	if d.balances[1] != 0 || d.ispayFree[1] != 0 {
		t.Fatalf("direct withdrawable usdt=%d ispay=%d", d.balances[1], d.ispayFree[1])
	}

	d.settleStaticLocked()
	u2 := d.users[2]
	if u2.StaticReleasedDays != 1 || u2.StaticDays != 750 {
		t.Fatalf("static window %+v", u2)
	}
	pos := d.orders[o.ID]
	if pos.ReleasedDays != 1 {
		t.Fatalf("released %d", pos.ReleasedDays)
	}
	if d.ispayLocked[2] != 15*biz.IspayMicroPerCoin {
		t.Fatal("principal must stay locked")
	}
	staticUSDT, staticMicro := biz.SplitHalf(biz.U(40), biz.DefaultIspayPriceFen)
	if d.balances[2] != staticUSDT {
		t.Fatalf("static usdt %d want %d", d.balances[2], staticUSDT)
	}
	if d.ispayFree[2] != staticMicro {
		t.Fatalf("static ispay %d want %d", d.ispayFree[2], staticMicro)
	}
}

func TestPaidSumLockNotSnappedToTier(t *testing.T) {
	// 1000+3000=4000U：档位从 1000 升到 3000（未到 6000），超额 1000 进锁仓。
	d := &Data{
		users:       map[int64]*biz.User{1: {ID: 1}},
		products:    map[int64]*biz.Product{},
		balances:    map[int64]int64{1: 4000 * biz.UstdScale},
		ispayLocked: map[int64]int64{},
		ispayFree:   map[int64]int64{},
		orders:      map[int64]*biz.Order{},
		path:        t.TempDir() + "/mall.json",
	}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: 1000 * biz.UstdScale, Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.products[2] = &biz.Product{ID: 2, Name: "3000", PriceFen: 3000 * biz.UstdScale, Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	shop := NewShopRepo(d)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	want1000 := biz.CoinsMicroFromPay(biz.U(1000), biz.CategoryWeb3)
	if d.ispayLocked[1] != want1000 {
		t.Fatalf("1000 lock %d", d.ispayLocked[1])
	}
	d.users[1].StaticReleasedDays = 10
	if _, err := shop.PlaceOrder(context.Background(), 1, 2, 1); err != nil {
		t.Fatal(err)
	}
	want4000 := biz.CoinsMicroFromPay(biz.U(4000), biz.CategoryWeb3)
	if d.ispayLocked[1] != want4000 {
		t.Fatalf("1000+3000 lock %d want 认购总额 %d", d.ispayLocked[1], want4000)
	}
	if d.users[1].StaticPackageFen != biz.U(3000) || d.users[1].StaticReleasedDays != 10 {
		t.Fatalf("1000→3000 不重开静态 %+v", d.users[1])
	}
}

func TestPaidSumLockAtTwoThousandTier(t *testing.T) {
	// 1000+1000=2000U：档位升到 2000，超额 0；锁仓仍按认购总额 2000 折，不能只折 1000。
	d := &Data{
		users:       map[int64]*biz.User{1: {ID: 1}},
		products:    map[int64]*biz.Product{},
		balances:    map[int64]int64{1: 2000 * biz.UstdScale},
		ispayLocked: map[int64]int64{},
		orders:      map[int64]*biz.Order{},
		path:        t.TempDir() + "/mall.json",
	}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: 1000 * biz.UstdScale, Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	shop := NewShopRepo(d)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	want := biz.CoinsMicroFromPay(biz.U(2000), biz.CategoryWeb3)
	if d.ispayLocked[1] != want {
		t.Fatalf("1000+1000 lock %d want %d (not 1000档)", d.ispayLocked[1], want)
	}
	if d.users[1].StaticPackageFen != biz.U(2000) {
		t.Fatalf("display tier should be 2000, got %d", d.users[1].StaticPackageFen)
	}
}

func TestUpgrade3000Plus3000To6000(t *testing.T) {
	d := &Data{
		users:       map[int64]*biz.User{1: {ID: 1}},
		products:    map[int64]*biz.Product{},
		balances:    map[int64]int64{1: 6000 * biz.UstdScale},
		ispayLocked: map[int64]int64{},
		orders:      map[int64]*biz.Order{},
		path:        t.TempDir() + "/mall.json",
	}
	d.products[1] = &biz.Product{ID: 1, Name: "3000", PriceFen: 3000 * biz.UstdScale, Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	shop := NewShopRepo(d)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if d.users[1].StaticPackageFen != biz.U(3000) {
		t.Fatalf("first 3000 tier %d", d.users[1].StaticPackageFen)
	}
	d.users[1].StaticReleasedDays = 10
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	want := biz.CoinsMicroFromPay(biz.U(6000), biz.CategoryWeb3)
	if d.ispayLocked[1] != want {
		t.Fatalf("3000+3000 lock %d want %d", d.ispayLocked[1], want)
	}
	if d.users[1].StaticPackageFen != biz.U(6000) || d.users[1].StaticReleasedDays != 10 {
		t.Fatalf("升 6000 不重开静态 %+v", d.users[1])
	}
}

func TestEffectiveTier12000Plus24000(t *testing.T) {
	d := &Data{
		users:       map[int64]*biz.User{1: {ID: 1}},
		products:    map[int64]*biz.Product{},
		balances:    map[int64]int64{1: 36000 * biz.UstdScale},
		ispayLocked: map[int64]int64{},
		orders:      map[int64]*biz.Order{},
		path:        t.TempDir() + "/mall.json",
	}
	d.products[1] = &biz.Product{ID: 1, Name: "12000", PriceFen: 12000 * biz.UstdScale, Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3_750}
	d.products[2] = &biz.Product{ID: 2, Name: "24000", PriceFen: 24000 * biz.UstdScale, Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3_750}
	shop := NewShopRepo(d)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.PlaceOrder(context.Background(), 1, 2, 1); err != nil {
		t.Fatal(err)
	}
	want := biz.CoinsMicroFromPay(biz.U(36000), biz.CategoryWeb3_750)
	if d.ispayLocked[1] != want || d.users[1].StaticPackageFen != biz.U(36000) {
		t.Fatalf("lock %d pkg %d", d.ispayLocked[1], d.users[1].StaticPackageFen)
	}
}

func TestFreeIspayDoesNotBumpStatic(t *testing.T) {
	d := &Data{
		users:       map[int64]*biz.User{1: {ID: 1, StaticDays: 300, StaticReleasedDays: 0, StaticPackageFen: 1000 * biz.UstdScale}},
		ispayLocked: map[int64]int64{1: biz.CoinsMicroFromPay(biz.U(1000), biz.CategoryWeb3)},
		ispayFree:   map[int64]int64{1: 100 * biz.IspayMicroPerCoin},
		balances:    map[int64]int64{},
		orders:      map[int64]*biz.Order{},
		path:        t.TempDir() + "/mall.json",
	}
	locked := d.ispayLocked[1]
	d.settleStaticLocked()
	wantUSDT, wantMicro := biz.SplitHalf(biz.StaticValueFen(biz.DailyReleaseCoins(locked, 300, 0), biz.DefaultIspayPriceFen), biz.DefaultIspayPriceFen)
	if d.balances[1] != wantUSDT {
		t.Fatalf("static usdt %d want %d", d.balances[1], wantUSDT)
	}
	if d.ispayFree[1] != 100*biz.IspayMicroPerCoin+wantMicro {
		t.Fatalf("free ispay should only gain static half, got %d", d.ispayFree[1])
	}
	if d.ispayLocked[1] != locked {
		t.Fatal("lock unchanged")
	}
}

func TestGetWalletViewStaticNumbers(t *testing.T) {
	d := &Data{
		users:       map[int64]*biz.User{1: {ID: 1, StaticDays: 300, StaticReleasedDays: 0, StaticPackageFen: biz.U(1000)}},
		ispayLocked: map[int64]int64{1: biz.CoinsMicroFromPay(biz.U(1000), biz.CategoryWeb3)},
		ispayFree:   map[int64]int64{},
		balances:    map[int64]int64{},
		orders:      map[int64]*biz.Order{},
		path:        t.TempDir() + "/mall.json",
	}
	d.settleStaticLocked()
	shop := NewShopRepo(d)
	view, err := shop.GetWalletView(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if view.StaticDays != 300 || view.StaticReleasedDays != 1 || view.StaticRemainDays != 299 {
		t.Fatalf("progress %+v", view)
	}
	if view.StaticGotUsdtFen <= 0 || view.StaticReleasedMicro <= 0 {
		t.Fatalf("got usdt=%d released=%d", view.StaticGotUsdtFen, view.StaticReleasedMicro)
	}
	if view.StaticPackageFen != biz.U(1000) {
		t.Fatalf("pkg %d", view.StaticPackageFen)
	}
}

func TestGetWalletViewRechargeExcludesRewards(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{1: {ID: 1}},
		balances: map[int64]int64{1: biz.U(630)},
		ledger: []*biz.LedgerEntry{
			{UserID: 1, AmountFen: biz.U(2000), Type: biz.LedgerRecharge},
			{UserID: 1, AmountFen: -biz.U(1000), Type: biz.LedgerOrderPay},
			{UserID: 1, AmountFen: -biz.U(500), Type: biz.LedgerWithdraw},
			{UserID: 1, AmountFen: biz.U(80), Type: biz.LedgerStaticReward},
			{UserID: 1, AmountFen: biz.U(50), Type: biz.LedgerAdminAdjust},
		},
		path: t.TempDir() + "/mall.json",
	}
	view, err := NewShopRepo(d).GetWalletView(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if view.RechargeFen != biz.U(500) {
		t.Fatalf("充值页要用充值剩余 %d，不能用可提 %d", view.RechargeFen, view.BalanceFen)
	}
	if view.BalanceFen != biz.U(630) {
		t.Fatalf("可提仍是余额 %d", view.BalanceFen)
	}
}
