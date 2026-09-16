package data

import (
	"context"
	"testing"

	"mall/internal/biz"
)

func TestPlaceOrderInstantPairAndManageSkip(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{},
		products: map[int64]*biz.Product{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
		path:     t.TempDir() + "/mall.json",
	}
	a := &biz.User{ID: 1, Nickname: "A"}
	b := &biz.User{ID: 2, Nickname: "B"}
	c := &biz.User{ID: 3, Nickname: "C"}
	d.users[1], d.users[2], d.users[3] = a, b, c
	if err := biz.PlaceSharedTrack(d.users, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := biz.PlaceSharedTrack(d.users, 1, 3); err != nil {
		t.Fatal(err)
	}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.products[2] = &biz.Product{ID: 2, Name: "3000", PriceFen: biz.U(3000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.products[3] = &biz.Product{ID: 3, Name: "6000", PriceFen: biz.U(6000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.balances[1] = biz.U(1000)
	d.balances[2] = biz.U(3000)
	d.balances[3] = biz.U(6000)
	shop := NewShopRepo(d)
	ctx := context.Background()
	if _, err := shop.PlaceOrder(ctx, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.PlaceOrder(ctx, 2, 2, 1); err != nil {
		t.Fatal(err)
	}
	for _, e := range d.ledger {
		if e.Type == biz.LedgerPairReward {
			t.Fatal("no pair until both sides have volume")
		}
	}
	if _, err := shop.PlaceOrder(ctx, 3, 3, 1); err != nil {
		t.Fatal(err)
	}
	if a.SettledPairFen != biz.U(3000) {
		t.Fatalf("instant pair consume %d", a.SettledPairFen)
	}
	cap := biz.PairCapFen(biz.U(1000))
	if a.DynamicRewardFen != cap {
		t.Fatalf("B/C 直推先占额度，对碰分剩余；当日应满档 %d", a.DynamicRewardFen)
	}
	var pairUSDT int64
	for _, e := range d.ledger {
		if e.Type == biz.LedgerPairReward && e.UserID == 1 {
			pairUSDT += e.AmountFen
		}
	}
	if pairUSDT != 0 {
		t.Fatalf("额度已被直推占满，对碰应为 0 got %d", pairUSDT)
	}
	var manageN int
	for _, e := range d.ledger {
		if e.Type == biz.LedgerManageReward {
			manageN++
		}
	}
	if manageN != 0 {
		t.Fatalf("A has no upline, manage %d", manageN)
	}
}

func TestManageSkipsUnactivated(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
		path:     t.TempDir() + "/mall.json",
	}
	a := &biz.User{ID: 1}
	b := &biz.User{ID: 2, InviterID: 1}
	c := &biz.User{ID: 3, InviterID: 2}
	e := &biz.User{ID: 4, InviterID: 3}
	f := &biz.User{ID: 5, InviterID: 4, LeftID: 6, RightID: 7}
	l := &biz.User{ID: 6, ParentID: 5, Side: biz.TrackLeft, OccupiedPos: 1, PerfFen: biz.U(1000)}
	r := &biz.User{ID: 7, ParentID: 5, Side: biz.TrackRight, OccupiedPos: 1, PerfFen: biz.U(1000)}
	d.users[1], d.users[2], d.users[3], d.users[4], d.users[5], d.users[6], d.users[7] = a, b, c, e, f, l, r
	d.orders[1] = paidPkg(1, 5, biz.U(1000))
	d.orders[2] = paidPkg(2, 2, biz.U(1000))
	d.orders[3] = paidPkg(3, 1, biz.U(1000))
	d.settlePairingLocked()
	share := biz.ManageRewardFen(biz.PairRewardFen(biz.U(1000)))
	if d.balances[4] != 0 {
		t.Fatal("E unactivated should be skipped")
	}
	if d.balances[3] != 0 {
		t.Fatal("C unactivated skipped")
	}
	if d.frozenUsdt[2] != usdtHalf(share) {
		t.Fatalf("B got %d want full pool %d", d.frozenUsdt[2], usdtHalf(share))
	}
	if d.balances[1] != 0 {
		t.Fatal("A is 4th invite level, no manage")
	}
}
