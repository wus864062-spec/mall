package data

import (
	"context"
	"testing"
	"time"

	"mall/internal/biz"
)

func TestAdminPaidSumAndLiveStats(t *testing.T) {
	now := time.Now().Unix()
	d := &Data{
		users:       map[int64]*biz.User{},
		products:    map[int64]*biz.Product{},
		balances:    map[int64]int64{},
		ispayLocked: map[int64]int64{},
		ispayFree:   map[int64]int64{},
		orders:      map[int64]*biz.Order{},
		path:        t.TempDir() + "/mall.json",
	}
	d.users[1] = &biz.User{ID: 1, CreatedAt: now}
	d.users[2] = &biz.User{ID: 2, InviterID: 1, CreatedAt: now}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.products[2] = &biz.Product{ID: 2, Name: "3000", PriceFen: biz.U(3000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[2] = biz.U(4000)
	shop := NewShopRepo(d)
	if _, err := shop.PlaceOrder(context.Background(), 2, 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.PlaceOrder(context.Background(), 2, 2, 1); err != nil {
		t.Fatal(err)
	}

	v := d.adminUserViewLocked(d.users[2])
	if v.PackageFen != biz.U(3000) {
		t.Fatalf("tier %d want 3000", v.PackageFen)
	}
	if v.PaidSumFen != biz.U(4000) {
		t.Fatalf("paid %d want 4000", v.PaidSumFen)
	}
	wantLock := biz.CoinsMicroFromPay(biz.U(4000), biz.CategoryWeb3)
	if v.IspayLockedMicro != wantLock {
		t.Fatalf("lock %d want %d", v.IspayLockedMicro, wantLock)
	}

	st, err := NewAdminRepo(d).Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.UserCount != 2 {
		t.Fatalf("users %d", st.UserCount)
	}
	if st.ActivatedCount != 1 {
		t.Fatalf("activated %d", st.ActivatedCount)
	}
	if st.TodayActivatedCount != 1 {
		t.Fatalf("today activated %d", st.TodayActivatedCount)
	}
	if st.PaidOrderFen != biz.U(4000) || st.TodayPaidOrderFen != biz.U(4000) {
		t.Fatalf("paid %d today %d", st.PaidOrderFen, st.TodayPaidOrderFen)
	}
	if st.PaidOrderCount != 2 || st.TodayPaidOrderCount != 2 {
		t.Fatalf("order count %d today %d", st.PaidOrderCount, st.TodayPaidOrderCount)
	}
	if st.TotalIspayMicro < wantLock {
		t.Fatalf("ispay total %d want >= %d", st.TotalIspayMicro, wantLock)
	}
	if st.TodayUserCount != 2 {
		t.Fatalf("today register %d", st.TodayUserCount)
	}
}

func TestAdminStatsListedFields(t *testing.T) {
	now := time.Now().Unix()
	old := now - 48*3600
	d := &Data{
		users:       map[int64]*biz.User{},
		balances:    map[int64]int64{},
		frozenUsdt:  map[int64]int64{},
		ispayFree:   map[int64]int64{},
		ispayLocked: map[int64]int64{},
		orders:      map[int64]*biz.Order{},
		path:        t.TempDir() + "/mall.json",
	}
	d.users[1] = &biz.User{ID: 1, CreatedAt: old}
	d.users[2] = &biz.User{ID: 2, CreatedAt: now}
	d.balances[1] = biz.U(400)
	d.frozenUsdt[1] = biz.U(200)
	d.orders[1] = &biz.Order{ID: 1, UserID: 1, Status: biz.OrderPaid, TotalFen: biz.U(1000), CategoryID: biz.CategoryWeb3, CreatedAt: old}
	d.orders[2] = &biz.Order{ID: 2, UserID: 2, Status: biz.OrderPaid, TotalFen: biz.U(3000), CategoryID: biz.CategoryWeb3, CreatedAt: now}
	d.ledger = []*biz.LedgerEntry{
		{UserID: 1, AmountFen: biz.U(1000), Type: biz.LedgerRecharge, RefType: "tx", TxHash: "0x1", CreatedAt: old},
		{UserID: 2, AmountFen: biz.U(2000), Type: biz.LedgerRecharge, RefType: "tx", TxHash: "0x2", CreatedAt: now},
		{UserID: 2, AmountFen: biz.U(300), Type: biz.LedgerRecharge, RefType: "recharge", CreatedAt: now},
		{UserID: 2, AmountFen: biz.U(50), Type: biz.LedgerAdminAdjust, CreatedAt: now},
		{UserID: 2, AmountFen: -biz.U(10), Type: biz.LedgerAdminAdjust, CreatedAt: now},
		{UserID: 1, AmountFen: biz.U(20), Type: biz.LedgerStaticReward, CreatedAt: old},
		{UserID: 1, AmountFen: 4_000_000, Type: biz.LedgerStaticRewardIspay, CreatedAt: old},
		{UserID: 2, AmountFen: biz.U(80), Type: biz.LedgerStaticReward, CreatedAt: now},
		{UserID: 2, AmountFen: 12_000_000, Type: biz.LedgerStaticRewardIspay, CreatedAt: now},
		{UserID: 1, AmountFen: biz.U(10), Type: biz.LedgerPairReward, CreatedAt: old},
		{UserID: 1, AmountFen: 50_000_000, Type: biz.LedgerManageRewardIspay, CreatedAt: old},
		{UserID: 1, AmountFen: biz.U(30), Type: biz.LedgerDirectReward, CreatedAt: now},
		{UserID: 1, AmountFen: 100_000_000, Type: biz.LedgerDirectRewardIspay, CreatedAt: now},
		{UserID: 1, AmountFen: -biz.U(100), Type: biz.LedgerWithdraw, CreatedAt: old},
		{UserID: 2, AmountFen: -biz.U(500), Type: biz.LedgerWithdraw, CreatedAt: now},
		{UserID: 1, AmountFen: -200_000_000, Type: biz.LedgerIspayWithdraw, CreatedAt: old},
		{UserID: 2, AmountFen: -50_000_000, Type: biz.LedgerIspayWithdraw, CreatedAt: now},
	}
	st, err := NewAdminRepo(d).Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.UserCount != 2 || st.TodayUserCount != 1 {
		t.Fatalf("register %d today %d", st.UserCount, st.TodayUserCount)
	}
	if st.ActivatedCount != 2 || st.TodayActivatedCount != 1 {
		t.Fatalf("activated %d today %d", st.ActivatedCount, st.TodayActivatedCount)
	}
	if st.RechargeFen != biz.U(3000) || st.TodayRechargeFen != biz.U(2000) {
		t.Fatalf("recharge total %d today %d，不能加调账/奖励/后台加", st.RechargeFen, st.TodayRechargeFen)
	}
	if st.AdminAdjustFen != biz.U(350) {
		t.Fatalf("后台加=无哈希充值300+正数调账50，得 %d", st.AdminAdjustFen)
	}
	if st.PaidOrderCount != 2 || st.TodayPaidOrderCount != 1 {
		t.Fatalf("订单笔数 %d today %d，不是金额", st.PaidOrderCount, st.TodayPaidOrderCount)
	}
	if st.TodayStaticFen != biz.U(80) {
		t.Fatalf("今日USDT静态用原币 %d", st.TodayStaticFen)
	}
	if st.TodayStaticIspayMicro != 12_000_000 {
		t.Fatalf("今日IsPay静态用原币 %d", st.TodayStaticIspayMicro)
	}
	price := biz.DefaultIspayPriceFen
	wantU := biz.U(80) + biz.IspayMicroToMarketFen(12_000_000, price)
	wantI := int64(12_000_000) + biz.MarketFenToIspayMicro(biz.U(80), price)
	if st.TodayStaticTotalFen != wantU {
		t.Fatalf("今日总USDT静态=U+币转U %d want %d", st.TodayStaticTotalFen, wantU)
	}
	if st.TodayStaticTotalIspayMicro != wantI {
		t.Fatalf("今日总IsPay静态=币+U转币 %d want %d", st.TodayStaticTotalIspayMicro, wantI)
	}
	if st.StaticFen != biz.U(100) {
		t.Fatalf("总USDT静态=往日20+今日80 得 %d", st.StaticFen)
	}
	if st.StaticIspayMicro != 16_000_000 {
		t.Fatalf("总IsPay静态=往日4e6+今日12e6 得 %d", st.StaticIspayMicro)
	}
	wantAllU := biz.U(100) + biz.IspayMicroToMarketFen(16_000_000, price)
	if st.StaticTotalFen != wantAllU {
		t.Fatalf("总静态=U+币转U %d want %d", st.StaticTotalFen, wantAllU)
	}
	wantAllI := int64(16_000_000) + biz.MarketFenToIspayMicro(biz.U(100), price)
	if st.StaticTotalIspayMicro != wantAllI {
		t.Fatalf("总币+U静态 %d want %d", st.StaticTotalIspayMicro, wantAllI)
	}
	if st.TodayDynamicFen != biz.U(30) {
		t.Fatalf("今日USDT动态用原币，不含往日10 %d", st.TodayDynamicFen)
	}
	if st.TodayDynamicIspayMicro != 100_000_000 {
		t.Fatalf("今日IsPay动态用原币，不含往日50e6 %d", st.TodayDynamicIspayMicro)
	}
	wantDynU := biz.U(30) + biz.IspayMicroToMarketFen(100_000_000, price)
	wantDynI := int64(100_000_000) + biz.MarketFenToIspayMicro(biz.U(30), price)
	if st.TodayDynamicTotalFen != wantDynU {
		t.Fatalf("今日总USDT动态=U+币转U %d want %d", st.TodayDynamicTotalFen, wantDynU)
	}
	if st.TodayDynamicTotalIspayMicro != wantDynI {
		t.Fatalf("今日总IsPay动态=币+U转币 %d want %d", st.TodayDynamicTotalIspayMicro, wantDynI)
	}
	if st.DynamicFen != biz.U(40) {
		t.Fatalf("总USDT动态=往日对碰10+今日直推30 得 %d", st.DynamicFen)
	}
	if st.DynamicIspayMicro != 150_000_000 {
		t.Fatalf("总IsPay动态=往日50e6+今日100e6 得 %d", st.DynamicIspayMicro)
	}
	wantAllDynU := biz.U(40) + biz.IspayMicroToMarketFen(150_000_000, price)
	if st.DynamicTotalFen != wantAllDynU {
		t.Fatalf("总动态=U+币转U %d want %d", st.DynamicTotalFen, wantAllDynU)
	}
	wantAllDynI := int64(150_000_000) + biz.MarketFenToIspayMicro(biz.U(40), price)
	if st.DynamicTotalIspayMicro != wantAllDynI {
		t.Fatalf("总币+U动态 %d want %d", st.DynamicTotalIspayMicro, wantAllDynI)
	}
	if st.Direct.TodayFen != biz.U(30) || st.Direct.Fen != biz.U(30) {
		t.Fatalf("直推只用直推流水，不含对碰/管理 %d %d", st.Direct.TodayFen, st.Direct.Fen)
	}
	if st.Direct.TodayIspayMicro != 100_000_000 || st.Direct.IspayMicro != 100_000_000 {
		t.Fatalf("直推IsPay %d %d", st.Direct.TodayIspayMicro, st.Direct.IspayMicro)
	}
	if st.Pair.Fen != biz.U(10) || st.Pair.TodayFen != 0 {
		t.Fatalf("对碰USDT=往日10今日0 得 %d today %d", st.Pair.Fen, st.Pair.TodayFen)
	}
	if st.Manage.IspayMicro != 50_000_000 || st.Manage.TodayIspayMicro != 0 {
		t.Fatalf("管理IsPay=往日50e6今日0 得 %d today %d", st.Manage.IspayMicro, st.Manage.TodayIspayMicro)
	}
	if st.Direct.TodayFen+st.Pair.TodayFen+st.Manage.TodayFen != st.TodayDynamicFen {
		t.Fatal("今日USDT动态=三类今日U之和，不是某一类")
	}
	if st.Direct.TodayIspayMicro+st.Pair.TodayIspayMicro+st.Manage.TodayIspayMicro != st.TodayDynamicIspayMicro {
		t.Fatal("今日IsPay动态=三类今日币之和")
	}
	if st.Direct.Fen+st.Pair.Fen+st.Manage.Fen != st.DynamicFen {
		t.Fatal("累计USDT动态=三类累计U之和")
	}
	if st.Direct.IspayMicro+st.Pair.IspayMicro+st.Manage.IspayMicro != st.DynamicIspayMicro {
		t.Fatal("累计IsPay动态=三类累计币之和")
	}
	wantDirectU := biz.U(30) + biz.IspayMicroToMarketFen(100_000_000, price)
	if st.Direct.TotalFen != wantDirectU {
		t.Fatalf("直推总U=U+币转U %d want %d", st.Direct.TotalFen, wantDirectU)
	}
	if st.DirectRewardFen != st.Direct.Fen {
		t.Fatal("旧 DirectRewardFen 必须等于直推累计USDT原币")
	}
	if st.BalanceUsdtFen != biz.U(400) {
		t.Fatalf("可提只用余额 %d，不能加冻结", st.BalanceUsdtFen)
	}
	if st.TodayWithdrawFen != biz.U(500) || st.TotalWithdrawFen != biz.U(600) {
		t.Fatalf("提现今日 %d 总 %d", st.TodayWithdrawFen, st.TotalWithdrawFen)
	}
	if st.TodayWithdrawIspayMicro != 50_000_000 || st.TotalWithdrawIspayMicro != 250_000_000 {
		t.Fatalf("IsPay提现今日 %d 总 %d", st.TodayWithdrawIspayMicro, st.TotalWithdrawIspayMicro)
	}
}
