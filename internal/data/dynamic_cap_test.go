package data

import (
	"context"
	"testing"

	"mall/internal/biz"
)

func TestDirectAndPairShareDailyDynamicCap(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{},
		products: map[int64]*biz.Product{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
		path:     t.TempDir() + "/mall.json",
	}
	a := &biz.User{ID: 1, Nickname: "A"}
	b := &biz.User{ID: 2, Nickname: "B", InviterID: 1}
	d.users[1], d.users[2] = a, b
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[1] = biz.U(1000)
	d.balances[2] = biz.U(1000)
	shop := NewShopRepo(d)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.PlaceOrder(context.Background(), 2, 1, 1); err != nil {
		t.Fatal(err)
	}
	direct := biz.DirectRewardFen(biz.U(1000))
	cap := biz.PairCapFen(biz.U(1000))
	if a.DynamicRewardFen != direct {
		t.Fatalf("直推占用额度 %d want %d", a.DynamicRewardFen, direct)
	}
	l := &biz.User{ID: 3, ParentID: 1, Side: biz.TrackLeft, OccupiedPos: 1, PerfFen: biz.U(200000)}
	r := &biz.User{ID: 4, ParentID: 1, Side: biz.TrackRight, OccupiedPos: 1, PerfFen: biz.U(200000)}
	a.LeftID, a.RightID = 3, 4
	d.users[3], d.users[4] = l, r
	d.settlePairingLocked()
	remain := cap - direct
	if a.DynamicRewardFen != cap {
		t.Fatalf("当日动态额度应满 %d", a.DynamicRewardFen)
	}
	if d.balances[1] != usdtHalf(direct)+usdtHalf(remain) {
		t.Fatalf("A 直推+剩余对碰 %d want %d", d.balances[1], usdtHalf(direct)+usdtHalf(remain))
	}
}

func TestDirectCappedByPackageTier(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{},
		products: map[int64]*biz.Product{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
		path:     t.TempDir() + "/mall.json",
	}
	d.users[1] = &biz.User{ID: 1}
	d.users[2] = &biz.User{ID: 2, InviterID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.products[2] = &biz.Product{ID: 2, Name: "12000", PriceFen: biz.U(12000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[1] = biz.U(1000)
	d.balances[2] = biz.U(12000)
	shop := NewShopRepo(d)
	if _, err := shop.PlaceOrder(context.Background(), 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.PlaceOrder(context.Background(), 2, 2, 1); err != nil {
		t.Fatal(err)
	}
	cap := biz.PairCapFen(biz.U(1000))
	if d.users[1].DynamicRewardFen != cap {
		t.Fatalf("直推 1200 应按 1000 档封顶到 %d got %d", cap, d.users[1].DynamicRewardFen)
	}
	want, _ := biz.SplitHalf(cap, biz.DefaultIspayPriceFen)
	if d.balances[1] != want {
		t.Fatalf("capped direct usdt %d want %d", d.balances[1], want)
	}
}
