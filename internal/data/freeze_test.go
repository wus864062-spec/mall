package data

import (
	"context"
	"testing"
	"time"

	"mall/internal/biz"
)

func freezeShop(t *testing.T) (*Data, biz.ShopRepo) {
	t.Helper()
	d := &Data{
		users:          map[int64]*biz.User{},
		products:       map[int64]*biz.Product{},
		balances:       map[int64]int64{},
		ispayLocked:    map[int64]int64{},
		ispayFree:      map[int64]int64{},
		frozenUsdt:     map[int64]int64{},
		ispayFrozen:    map[int64]int64{},
		unfreezeRemain: map[int64]int64{},
		frozenLots:     map[int64][]frozenLot{},
		orders:         map[int64]*biz.Order{},
		path:           t.TempDir() + "/mall.json",
	}
	return d, NewShopRepo(d)
}

func TestFirstBuyQuotaFillsLaterDynamic(t *testing.T) {
	d, shop := freezeShop(t)
	d.users[1] = &biz.User{ID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[1] = biz.U(1000)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if d.unfreezeRemain[1] != biz.U(600) {
		t.Fatalf("first buy remain %d want 600", d.unfreezeRemain[1])
	}
	d.creditSplitLocked(1, biz.U(1000), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "x", 1)
	wantU, wantM := biz.SplitHalf(biz.U(600), biz.DefaultIspayPriceFen)
	frozU, frozM := biz.SplitHalf(biz.U(400), biz.DefaultIspayPriceFen)
	if d.balances[1] != wantU || d.ispayFree[1] != wantM {
		t.Fatalf("quota fill usdt=%d ispay=%d", d.balances[1], d.ispayFree[1])
	}
	if d.frozenUsdt[1] != frozU || d.ispayFrozen[1] != frozM {
		t.Fatalf("overflow frozen usdt=%d ispay=%d", d.frozenUsdt[1], d.ispayFrozen[1])
	}
	if d.unfreezeRemain[1] != 0 {
		t.Fatalf("remain %d", d.unfreezeRemain[1])
	}
	beforeF := d.frozenUsdt[1]
	d.creditSplitLocked(1, biz.U(200), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "x", 2)
	extraU, extraM := biz.SplitHalf(biz.U(200), biz.DefaultIspayPriceFen)
	if d.frozenUsdt[1] != beforeF+extraU || d.ispayFrozen[1] != frozM+extraM {
		t.Fatalf("quota used, extra should freeze usdt=%d", d.frozenUsdt[1])
	}
	if d.balances[1] != wantU {
		t.Fatal("extra must not go withdrawable")
	}
}

func TestNeverBoughtDirectAllFrozen(t *testing.T) {
	d, shop := freezeShop(t)
	d.users[1] = &biz.User{ID: 1}
	d.users[2] = &biz.User{ID: 2, InviterID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[2] = biz.U(1000)
	if _, err := shop.PlaceOrder(context.Background(), 2, 1, 1); err != nil {
		t.Fatal(err)
	}
	wantUSDT, wantMicro := biz.SplitHalf(biz.DirectRewardFen(biz.U(1000)), biz.DefaultIspayPriceFen)
	if d.frozenUsdt[1] != wantUSDT || d.ispayFrozen[1] != wantMicro {
		t.Fatalf("frozen usdt=%d ispay=%d", d.frozenUsdt[1], d.ispayFrozen[1])
	}
	if d.balances[1] != 0 || d.unfreezeRemain[1] != 0 {
		t.Fatalf("inviter should have no quota")
	}
}

func TestBuyUnfreezesExistingFrozenHalf(t *testing.T) {
	d, shop := freezeShop(t)
	d.users[1] = &biz.User{ID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.addFrozenSplitLocked(1, biz.U(1000), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "x", 1)
	d.balances[1] = biz.U(1000)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	wantU, wantM := biz.SplitHalf(biz.U(600), biz.DefaultIspayPriceFen)
	leftU, leftM := biz.SplitHalf(biz.U(400), biz.DefaultIspayPriceFen)
	if d.balances[1] != wantU || d.ispayFree[1] != wantM {
		t.Fatalf("unfreeze usdt=%d ispay=%d want %d %d", d.balances[1], d.ispayFree[1], wantU, wantM)
	}
	if d.frozenUsdt[1] != leftU || d.ispayFrozen[1] != leftM {
		t.Fatalf("left frozen usdt=%d ispay=%d", d.frozenUsdt[1], d.ispayFrozen[1])
	}
}

func TestSecondBuySameTierDoesNotAddCap(t *testing.T) {
	d, shop := freezeShop(t)
	d.users[1] = &biz.User{ID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "3000", PriceFen: biz.U(3000), Stock: 20, Status: 1, CategoryID: biz.CategoryWeb3}
	d.products[2] = &biz.Product{ID: 2, Name: "1000", PriceFen: biz.U(1000), Stock: 20, Status: 1, CategoryID: biz.CategoryWeb3}
	d.addFrozenSplitLocked(1, biz.U(2400), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "x", 1)
	d.balances[1] = biz.U(4000)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	leftU, leftM := biz.SplitHalf(biz.U(600), biz.DefaultIspayPriceFen)
	if d.frozenUsdt[1] != leftU || d.ispayFrozen[1] != leftM {
		t.Fatalf("first buy leftover frozen usdt=%d ispay=%d", d.frozenUsdt[1], d.ispayFrozen[1])
	}
	if _, err := shop.PlaceOrder(context.Background(), 1, 2, 1); err != nil {
		t.Fatal(err)
	}
	if d.frozenUsdt[1] != leftU || d.ispayFrozen[1] != leftM {
		t.Fatalf("3000+1000 仍 3000 档, must not add cap frozen=%d/%d", d.frozenUsdt[1], d.ispayFrozen[1])
	}
	if d.unfreezeRemain[1] != 0 {
		t.Fatalf("same-tier remain %d", d.unfreezeRemain[1])
	}
}

func TestCartQuotaUsesPaidTierNotItemSum(t *testing.T) {
	d, shop := freezeShop(t)
	d.users[1] = &biz.User{ID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.products[2] = &biz.Product{ID: 2, Name: "3000", PriceFen: biz.U(3000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[1] = biz.U(4000)
	if _, err := shop.PlaceCart(context.Background(), 1, []int64{1, 2}); err != nil {
		t.Fatal(err)
	}
	if d.unfreezeRemain[1] != biz.U(1800) {
		t.Fatalf("cart 1000+3000 remain %d want 1800 not 2400", d.unfreezeRemain[1])
	}
}

func TestCartTwoThousandCapTwelveHundred(t *testing.T) {
	d, shop := freezeShop(t)
	d.users[1] = &biz.User{ID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[1] = biz.U(2000)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 2); err != nil {
		t.Fatal(err)
	}
	if d.unfreezeRemain[1] != biz.U(1200) {
		t.Fatalf("qty2 1000+1000 remain %d want 1200 not 600", d.unfreezeRemain[1])
	}
}

func TestConfigPairCapOverridesUnfreezeGrant(t *testing.T) {
	d, shop := freezeShop(t)
	d.pairCaps = map[int64]int64{biz.U(1000): biz.U(400)}
	d.users[1] = &biz.User{ID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[1] = biz.U(1000)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if d.unfreezeRemain[1] != biz.U(400) {
		t.Fatalf("config cap remain %d want 400 not default 600", d.unfreezeRemain[1])
	}
}

func TestConfigPairCapOverridesDynamicPayout(t *testing.T) {
	d, shop := freezeShop(t)
	d.pairCaps = map[int64]int64{biz.U(1000): biz.U(400)}
	d.users[1] = &biz.User{ID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[1] = biz.U(1000)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	pay := d.creditCappedDynamicLocked(1, biz.U(1000), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "t", 1)
	if pay != biz.U(400) {
		t.Fatalf("dynamic pay %d want config 400 not default 600", pay)
	}
}

func TestConfigZeroPairCapBlocksDynamic(t *testing.T) {
	d, shop := freezeShop(t)
	d.pairCaps = map[int64]int64{biz.U(1000): 0}
	d.users[1] = &biz.User{ID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[1] = biz.U(1000)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if d.unfreezeRemain[1] != 0 {
		t.Fatalf("cap 0 remain %d", d.unfreezeRemain[1])
	}
	pay := d.creditCappedDynamicLocked(1, biz.U(1000), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "t", 1)
	if pay != 0 {
		t.Fatalf("config 0 当天动态奖应为 0 got %d", pay)
	}
}

func TestStaticDoesNotConsumeQuota(t *testing.T) {
	d, _ := freezeShop(t)
	d.users[1] = &biz.User{ID: 1, StaticDays: 300, StaticReleasedDays: 0, StaticPackageFen: biz.U(1000)}
	d.ispayLocked[1] = biz.CoinsMicroFromPay(biz.U(1000), biz.CategoryWeb3)
	d.unfreezeRemain[1] = biz.U(600)
	d.settleStaticLocked()
	if d.unfreezeRemain[1] != biz.U(600) {
		t.Fatalf("static consumed quota %d", d.unfreezeRemain[1])
	}
	if d.balances[1] <= 0 {
		t.Fatal("static should go withdrawable")
	}
}

func TestSyncLockDoesNotDumpFrozenOrGrantQuota(t *testing.T) {
	d, _ := freezeShop(t)
	d.users[1] = &biz.User{ID: 1, StaticPackageFen: biz.U(1000), StaticDays: 300}
	d.orders[1] = &biz.Order{ID: 1, UserID: 1, Status: biz.OrderPaid, TotalFen: biz.U(1000), CategoryID: biz.CategoryWeb3, Items: []*biz.OrderItem{{PriceFen: biz.U(1000), Quantity: 1, AmountFen: biz.U(1000)}}}
	d.addFrozenSplitLocked(1, biz.U(400), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "x", 1)
	beforeU, beforeM := d.frozenUsdt[1], d.ispayFrozen[1]
	d.syncWeb3LockLocked(1)
	if d.frozenUsdt[1] != beforeU || d.ispayFrozen[1] != beforeM {
		t.Fatal("sync must not dump frozen")
	}
	if d.unfreezeRemain[1] != 0 {
		t.Fatal("sync must not grant quota")
	}
}

func TestFrozenExpiresAfter72Hours(t *testing.T) {
	d, _ := freezeShop(t)
	d.users[1] = &biz.User{ID: 1}
	d.now = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	d.addFrozenSplitLocked(1, biz.U(400), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "x", 1)
	d.unfreezeRemain[1] = biz.U(600)
	d.balances[1] = 123
	if d.frozenUsdt[1] == 0 {
		t.Fatal("need frozen")
	}
	d.now = d.now.Add(72*time.Hour + time.Second)
	if !d.expireFrozenLocked(d.now) {
		t.Fatal("expected expire")
	}
	if d.frozenUsdt[1] != 0 || d.ispayFrozen[1] != 0 {
		t.Fatalf("expired frozen usdt=%d ispay=%d", d.frozenUsdt[1], d.ispayFrozen[1])
	}
	if d.unfreezeRemain[1] != biz.U(600) || d.balances[1] != 123 {
		t.Fatal("expire must not touch quota or withdrawable")
	}
}

func TestFrozenExpireOnlyOldLot(t *testing.T) {
	d, _ := freezeShop(t)
	d.users[1] = &biz.User{ID: 1}
	d.now = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	d.addFrozenSplitLocked(1, biz.U(100), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "old", 1)
	oldU, oldM := d.frozenUsdt[1], d.ispayFrozen[1]
	d.now = d.now.Add(24 * time.Hour)
	d.addFrozenSplitLocked(1, biz.U(200), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "new", 2)
	d.now = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC).Add(72*time.Hour + time.Second)
	if !d.expireFrozenLocked(d.now) {
		t.Fatal("old lot should expire")
	}
	newU, newM := biz.SplitHalf(biz.U(200), biz.DefaultIspayPriceFen)
	if d.frozenUsdt[1] != newU || d.ispayFrozen[1] != newM {
		t.Fatalf("kept new lot usdt=%d ispay=%d (old was %d/%d)", d.frozenUsdt[1], d.ispayFrozen[1], oldU, oldM)
	}
}

func TestResubDoesNotResetStaticReleasedDays(t *testing.T) {
	d, shop := freezeShop(t)
	d.users[1] = &biz.User{ID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.products[2] = &biz.Product{ID: 2, Name: "3000", PriceFen: biz.U(3000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[1] = biz.U(4000)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	d.users[1].StaticReleasedDays = 10
	if _, err := shop.PlaceOrder(context.Background(), 1, 2, 1); err != nil {
		t.Fatal(err)
	}
	if d.users[1].StaticPackageFen != biz.U(3000) {
		t.Fatalf("tier %d", d.users[1].StaticPackageFen)
	}
	if d.users[1].StaticReleasedDays != 10 {
		t.Fatalf("released days reset %d", d.users[1].StaticReleasedDays)
	}
}

func TestUpgradeTo3000AddsDeltaCap(t *testing.T) {
	d, shop := freezeShop(t)
	d.users[1] = &biz.User{ID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.products[2] = &biz.Product{ID: 2, Name: "3000", PriceFen: biz.U(3000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[1] = biz.U(4000)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if d.unfreezeRemain[1] != biz.U(600) {
		t.Fatalf("first remain %d", d.unfreezeRemain[1])
	}
	if _, err := shop.PlaceOrder(context.Background(), 1, 2, 1); err != nil {
		t.Fatal(err)
	}
	if d.unfreezeRemain[1] != biz.U(1800) {
		t.Fatalf("升 3000 档 remain %d want 1800 = 600+1200", d.unfreezeRemain[1])
	}
}

func TestSameDayUpgradeTo6000AddsDelta(t *testing.T) {
	d, shop := freezeShop(t)
	d.users[1] = &biz.User{ID: 1}
	d.products[2] = &biz.Product{ID: 2, Name: "3000", PriceFen: biz.U(3000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[1] = biz.U(6000)
	if _, err := shop.PlaceOrder(context.Background(), 1, 2, 1); err != nil {
		t.Fatal(err)
	}
	if d.unfreezeRemain[1] != biz.U(1800) {
		t.Fatalf("3000档 remain %d", d.unfreezeRemain[1])
	}
	if _, err := shop.PlaceOrder(context.Background(), 1, 2, 1); err != nil {
		t.Fatal(err)
	}
	if d.unfreezeRemain[1] != biz.U(4000) {
		t.Fatalf("升 6000 档 remain %d want 4000", d.unfreezeRemain[1])
	}
}

func TestNextDayDoesNotAutoGrantUnfreeze(t *testing.T) {
	d, shop := freezeShop(t)
	d.now = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	d.users[1] = &biz.User{ID: 1}
	d.products[2] = &biz.Product{ID: 2, Name: "3000", PriceFen: biz.U(3000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.addFrozenSplitLocked(1, biz.U(2000), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "x", 1)
	d.balances[1] = biz.U(3000)
	if _, err := shop.PlaceOrder(context.Background(), 1, 2, 1); err != nil {
		t.Fatal(err)
	}
	leftU, leftM := biz.SplitHalf(biz.U(200), biz.DefaultIspayPriceFen)
	if d.frozenUsdt[1] != leftU || d.ispayFrozen[1] != leftM {
		t.Fatalf("day1 leftover frozen usdt=%d ispay=%d", d.frozenUsdt[1], d.ispayFrozen[1])
	}
	if d.unfreezeRemain[1] != 0 {
		t.Fatalf("day1 remain %d", d.unfreezeRemain[1])
	}
	d.now = d.now.Add(24 * time.Hour)
	d.expireFrozenLocked(d.now)
	if d.frozenUsdt[1] != leftU || d.ispayFrozen[1] != leftM {
		t.Fatalf("换日不得自动解冻 frozen=%d/%d", d.frozenUsdt[1], d.ispayFrozen[1])
	}
	if d.unfreezeRemain[1] != 0 {
		t.Fatalf("换日不得自动补额度 remain %d", d.unfreezeRemain[1])
	}
}

func TestNextDayBuyGrantsAgain(t *testing.T) {
	d, shop := freezeShop(t)
	d.now = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	d.users[1] = &biz.User{ID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.products[2] = &biz.Product{ID: 2, Name: "3000", PriceFen: biz.U(3000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.addFrozenSplitLocked(1, biz.U(2000), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "x", 1)
	d.balances[1] = biz.U(4000)
	if _, err := shop.PlaceOrder(context.Background(), 1, 2, 1); err != nil {
		t.Fatal(err)
	}
	leftU, leftM := biz.SplitHalf(biz.U(200), biz.DefaultIspayPriceFen)
	d.now = d.now.Add(24 * time.Hour)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if d.frozenUsdt[1] != 0 || d.ispayFrozen[1] != 0 {
		t.Fatalf("次日再买应解冻剩余 frozen=%d/%d (day1 leftover was %d/%d)", d.frozenUsdt[1], d.ispayFrozen[1], leftU, leftM)
	}
	if d.unfreezeRemain[1] != biz.U(1800)-biz.U(200) {
		t.Fatalf("次日仍 3000 档再发 1800，解冻 200 后 remain %d want 1600", d.unfreezeRemain[1])
	}
}

func lotValueFen(d *Data, lot frozenLot) int64 {
	return biz.ReconstructSplitValueFen(lot.UsdtFen, lot.IspayMicro, d.priceFenLocked())
}

func TestSameDayExtraFreezeDoesNotRefreshTimer(t *testing.T) {
	d, _ := freezeShop(t)
	d.users[1] = &biz.User{ID: 1}
	d.now = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	d.addFrozenSplitLocked(1, biz.U(1000), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "a", 1)
	t0 := d.frozenLots[1][0].CreatedAt
	d.now = d.now.Add(2 * time.Hour)
	d.addFrozenSplitLocked(1, biz.U(1000), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "b", 2)
	if len(d.frozenLots[1]) != 1 {
		t.Fatalf("same day should merge lots=%d", len(d.frozenLots[1]))
	}
	if d.frozenLots[1][0].CreatedAt != t0 {
		t.Fatal("same-day extra freeze must not refresh CreatedAt")
	}
	if lotValueFen(d, d.frozenLots[1][0]) != biz.U(2000) {
		t.Fatalf("merged value %d", lotValueFen(d, d.frozenLots[1][0]))
	}
}

func TestUnfreezeTakesOldestDayFirst(t *testing.T) {
	d, shop := freezeShop(t)
	d.now = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	d.users[1] = &biz.User{ID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.addFrozenSplitLocked(1, biz.U(2000), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "d1", 1)
	t1 := d.frozenLots[1][0].CreatedAt
	d.balances[1] = biz.U(2000)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if len(d.frozenLots[1]) != 1 || lotValueFen(d, d.frozenLots[1][0]) != biz.U(1400) {
		t.Fatalf("day1 after unfreeze 600 leftover %v", d.frozenLots[1])
	}
	if d.frozenLots[1][0].CreatedAt != t1 {
		t.Fatal("buy must not refresh day1 CreatedAt")
	}
	d.now = d.now.Add(24 * time.Hour)
	d.addFrozenSplitLocked(1, biz.U(1000), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "d2", 2)
	if len(d.frozenLots[1]) != 2 {
		t.Fatalf("day2 lot missing %d", len(d.frozenLots[1]))
	}
	t2 := d.frozenLots[1][1].CreatedAt
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	lots := d.frozenLots[1]
	if len(lots) != 2 {
		t.Fatalf("should keep two days lots=%d", len(lots))
	}
	if lots[0].CreatedAt != t1 || lotValueFen(d, lots[0]) != biz.U(200) {
		t.Fatalf("换日再买升 2000 档发 1200，先扣最早那天 leftover=%d at %d", lotValueFen(d, lots[0]), lots[0].CreatedAt)
	}
	if lots[1].CreatedAt != t2 || lotValueFen(d, lots[1]) != biz.U(1000) {
		t.Fatalf("day2 frozen must stay value=%d at %d", lotValueFen(d, lots[1]), lots[1].CreatedAt)
	}
}

func TestExpireOldestDayBeforeNewer(t *testing.T) {
	d, _ := freezeShop(t)
	d.users[1] = &biz.User{ID: 1}
	d.now = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	d.addFrozenSplitLocked(1, biz.U(800), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "d1", 1)
	t1 := d.now
	d.now = d.now.Add(24 * time.Hour)
	d.addFrozenSplitLocked(1, biz.U(1000), biz.LedgerDirectReward, biz.LedgerDirectRewardIspay, "d2", 2)
	d.now = t1.Add(72*time.Hour + time.Second)
	if !d.expireFrozenLocked(d.now) {
		t.Fatal("day1 remainder should expire first")
	}
	if len(d.frozenLots[1]) != 1 || lotValueFen(d, d.frozenLots[1][0]) != biz.U(1000) {
		t.Fatalf("kept day2 got %v", d.frozenLots[1])
	}
	d.now = t1.Add(24*time.Hour + 72*time.Hour + time.Second)
	if !d.expireFrozenLocked(d.now) {
		t.Fatal("day2 should expire on its own countdown")
	}
	if d.frozenUsdt[1] != 0 || d.ispayFrozen[1] != 0 || len(d.frozenLots[1]) != 0 {
		t.Fatalf("all expired usdt=%d lots=%d", d.frozenUsdt[1], len(d.frozenLots[1]))
	}
}
