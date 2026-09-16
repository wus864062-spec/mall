package data

import (
	"context"
	"testing"
	"time"

	"mall/internal/biz"
)

func TestPlaceOrderDoesNotPayPair(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{},
		products: map[int64]*biz.Product{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
		path:     t.TempDir() + "/mall.json",
	}
	d.users[1] = &biz.User{ID: 1, Nickname: "A"}
	d.users[2] = &biz.User{ID: 2, Nickname: "B"}
	if err := biz.PlaceSharedTrack(d.users, 1, 2); err != nil {
		t.Fatal(err)
	}
	d.products[1] = &biz.Product{ID: 1, Name: "1000USTD认购", PriceFen: biz.U(1000), Stock: 10}
	d.balances[2] = biz.U(1000)
	shop := NewShopRepo(d)
	if _, err := shop.PlaceOrder(context.Background(), 2, 1, 1); err != nil {
		t.Fatal(err)
	}
	for _, e := range d.ledger {
		if e.Type == biz.LedgerPairReward || e.Type == biz.LedgerPairUpline {
			t.Fatal("pair needs both sides")
		}
	}
}

func TestDailyPairLeftoverAndUpline(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
		path:     t.TempDir() + "/mall.json",
	}
	a := &biz.User{ID: 1, Nickname: "A", LeftID: 2, RightID: 3}
	b := &biz.User{ID: 2, Nickname: "B", ParentID: 1, Side: biz.TrackLeft, OccupiedPos: 1, RightID: 4, ChainNextID: 6}
	c := &biz.User{ID: 3, Nickname: "C", ParentID: 1, Side: biz.TrackRight, OccupiedPos: 1}
	dd := &biz.User{ID: 4, Nickname: "D", ParentID: 2, Side: biz.TrackRight, OccupiedPos: 1, RightID: 5, ChainNextID: 7}
	d1 := &biz.User{ID: 5, Nickname: "D1", ParentID: 4, Side: biz.TrackRight, OccupiedPos: 1}
	b2 := &biz.User{ID: 6, Nickname: "B2", ParentID: 1, Side: biz.TrackLeft, OccupiedPos: 2}
	dnext := &biz.User{ID: 7, Nickname: "Dnext", ParentID: 2, Side: biz.TrackRight, OccupiedPos: 2}
	d.users[1], d.users[2], d.users[3], d.users[4], d.users[5], d.users[6], d.users[7] = a, b, c, dd, d1, b2, dnext
	unit := biz.U(1000)
	b2.PerfFen = 5 * unit
	c.PerfFen = 4 * unit
	dnext.PerfFen = 4 * unit
	d1.PerfFen = 2 * unit
	d.orders = map[int64]*biz.Order{
		1: paidPkg(1, 1, biz.U(1000)),
		2: paidPkg(2, 2, biz.U(1000)),
		3: paidPkg(3, 4, biz.U(1000)),
	}

	d.settlePairingLocked()
	aBonus := biz.PairRewardFen(4 * unit)
	bBonus := biz.PairRewardFen(4 * unit)
	dBonus := biz.PairRewardFen(2 * unit)
	if d.frozenUsdt[1] != usdtHalf(aBonus) {
		t.Fatalf("A pair 10%% %d want %d", d.frozenUsdt[1], usdtHalf(aBonus))
	}
	if d.frozenUsdt[2] != usdtHalf(bBonus) {
		t.Fatalf("B %d want %d", d.frozenUsdt[2], usdtHalf(bBonus))
	}
	if d.frozenUsdt[4] != usdtHalf(dBonus) {
		t.Fatalf("D %d want %d", d.frozenUsdt[4], usdtHalf(dBonus))
	}
	if a.SettledPairFen != 4*unit {
		t.Fatalf("A left consumed %d", a.SettledPairFen)
	}

	c.PerfFen = 5 * unit
	d.settlePairingLocked()
	if a.SettledPairFen != 5*unit {
		t.Fatalf("大区剩余再碰 %d", a.SettledPairFen)
	}
	if d.frozenUsdt[1] != usdtHalf(aBonus)+usdtHalf(biz.PairRewardFen(unit)) {
		t.Fatalf("A after leftover %d", d.frozenUsdt[1])
	}

	c.PerfFen = 8 * unit
	d.settlePairingLocked()
	if a.SettledPairFen != 5*unit {
		t.Fatalf("小区多出部分等待对侧 %d", a.SettledPairFen)
	}
}

func TestMaybeSettlePairingOnMidnight(t *testing.T) {
	d := &Data{
		users:              map[int64]*biz.User{},
		balances:           map[int64]int64{},
		orders:             map[int64]*biz.Order{},
		path:               t.TempDir() + "/mall.json",
		lastPairSettleDate: "2026-09-10",
	}
	d.users[1] = &biz.User{ID: 1, Nickname: "A"}
	d.users[2] = &biz.User{ID: 2, Nickname: "B", PerfFen: biz.U(1000)}
	d.users[3] = &biz.User{ID: 3, Nickname: "C", PerfFen: biz.U(1000)}
	if err := biz.PlaceSharedTrack(d.users, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := biz.PlaceSharedTrack(d.users, 1, 3); err != nil {
		t.Fatal(err)
	}
	d.orders = map[int64]*biz.Order{1: paidPkg(1, 1, biz.U(1000))}
	loc := pairSettleLocation()
	now := time.Date(2026, 9, 11, 0, 0, 5, 0, loc)
	if !d.maybeSettlePairing(now) {
		t.Fatal("expected settle after midnight")
	}
	if d.lastPairSettleDate != "2026-09-11" {
		t.Fatalf("date %s", d.lastPairSettleDate)
	}
	if d.frozenUsdt[1] != 0 {
		t.Fatal("midnight should not pay pair")
	}
	if d.maybeSettlePairing(now.Add(time.Hour)) {
		t.Fatal("should not settle twice the same day")
	}
}

func usdtHalf(bonus int64) int64 {
	u, _ := biz.SplitHalf(bonus, biz.DefaultIspayPriceFen)
	return u
}

func paidPkg(id, userID, priceFen int64) *biz.Order {
	return &biz.Order{
		ID:         id,
		UserID:     userID,
		Status:     biz.OrderPaid,
		TotalFen:   priceFen,
		CategoryID: biz.CategoryWeb3,
		Items:      []*biz.OrderItem{{PriceFen: priceFen, Quantity: 1, AmountFen: priceFen}},
	}
}

func TestPairCap1000USTD(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
		path:     t.TempDir() + "/mall.json",
	}
	a := &biz.User{ID: 1, Nickname: "A", LeftID: 2, RightID: 3}
	b := &biz.User{ID: 2, Nickname: "B", ParentID: 1, Side: biz.TrackLeft, OccupiedPos: 1, PerfFen: biz.U(200000)}
	c := &biz.User{ID: 3, Nickname: "C", ParentID: 1, Side: biz.TrackRight, OccupiedPos: 1, PerfFen: biz.U(200000)}
	d.users[1], d.users[2], d.users[3] = a, b, c
	d.orders[1] = paidPkg(1, 1, biz.U(1000))
	d.settlePairingLocked()
	cap := biz.PairCapFen(biz.U(1000))
	if cap != biz.U(600) {
		t.Fatalf("cap %d", cap)
	}
	if d.frozenUsdt[1] != usdtHalf(cap) {
		t.Fatalf("paid %d want cap %d", d.frozenUsdt[1], cap)
	}
	if a.SettledPairFen != biz.U(200000) {
		t.Fatalf("封顶不影响大小区结算 %d", a.SettledPairFen)
	}

	before := d.frozenUsdt[1]
	d.settlePairingLocked()
	if d.frozenUsdt[1] != before {
		t.Fatalf("封顶未发部分不计入次日奖励 %d", d.frozenUsdt[1])
	}
}

func TestPairSettleIndependentOfCapLeftoverLarge(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
		path:     t.TempDir() + "/mall.json",
	}
	a := &biz.User{ID: 1, Nickname: "A", LeftID: 2, RightID: 3}
	b := &biz.User{ID: 2, Nickname: "B", ParentID: 1, Side: biz.TrackLeft, OccupiedPos: 1, PerfFen: biz.U(200000)}
	c := &biz.User{ID: 3, Nickname: "C", ParentID: 1, Side: biz.TrackRight, OccupiedPos: 1, PerfFen: biz.U(50000)}
	d.users[1], d.users[2], d.users[3] = a, b, c
	d.orders[1] = paidPkg(1, 1, biz.U(1000))
	d.settlePairingLocked()
	if d.frozenUsdt[1] != usdtHalf(biz.PairCapFen(biz.U(1000))) {
		t.Fatalf("当日只发到封顶 %d", d.frozenUsdt[1])
	}
	if a.SettledPairFen != biz.U(50000) {
		t.Fatalf("大区应减去全部小区 %d", a.SettledPairFen)
	}

	c.PerfFen = biz.U(80000)
	d.settlePairingLocked()
	if a.SettledPairFen != biz.U(80000) {
		t.Fatalf("大区剩余仍应消耗 %d", a.SettledPairFen)
	}
	if d.frozenUsdt[1] != usdtHalf(biz.PairCapFen(biz.U(1000))) {
		t.Fatalf("当日额度用完后不再发对碰 %d", d.frozenUsdt[1])
	}

	a.DynamicRewardDay = "2000-01-01"
	c.PerfFen = biz.U(110000)
	d.settlePairingLocked()
	if a.SettledPairFen != biz.U(110000) {
		t.Fatalf("换日后只碰大区剩余 %d", a.SettledPairFen)
	}
	want := usdtHalf(biz.PairCapFen(biz.U(1000))) + usdtHalf(biz.CapPairPayout(biz.PairRewardFen(biz.U(30000)), biz.PairCapFen(biz.U(1000))))
	if d.frozenUsdt[1] != want {
		t.Fatalf("换日后再发 %d want %d", d.frozenUsdt[1], want)
	}
}

func TestManageRewardFromPair(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
		path:     t.TempDir() + "/mall.json",
	}
	a := &biz.User{ID: 1, Nickname: "A"}
	b := &biz.User{ID: 2, Nickname: "B", InviterID: 1}
	x := &biz.User{ID: 3, Nickname: "X", InviterID: 2}
	y := &biz.User{ID: 4, Nickname: "Y", InviterID: 3, LeftID: 5, RightID: 6}
	l := &biz.User{ID: 5, Nickname: "L", ParentID: 4, Side: biz.TrackLeft, OccupiedPos: 1, PerfFen: biz.U(1000)}
	r := &biz.User{ID: 6, Nickname: "R", ParentID: 4, Side: biz.TrackRight, OccupiedPos: 1, PerfFen: biz.U(1000)}
	d.users[1], d.users[2], d.users[3], d.users[4], d.users[5], d.users[6] = a, b, x, y, l, r
	d.orders[1] = paidPkg(1, 4, biz.U(1000))
	d.orders[2] = paidPkg(2, 3, biz.U(1000))
	d.orders[3] = paidPkg(3, 2, biz.U(1000))
	d.orders[4] = paidPkg(4, 1, biz.U(1000))
	d.settlePairingLocked()

	pairPay := biz.PairRewardFen(biz.U(1000))
	if d.frozenUsdt[4] != usdtHalf(pairPay) {
		t.Fatalf("Y 对碰奖应独立全额 %d", d.frozenUsdt[4])
	}
	pool := biz.ManageRewardFen(pairPay)
	parts := biz.SplitEvenFen(pool, 3)
	if d.frozenUsdt[3] != usdtHalf(parts[0]) || d.frozenUsdt[2] != usdtHalf(parts[1]) || d.frozenUsdt[1] != usdtHalf(parts[2]) {
		t.Fatalf("管理奖 X=%d B=%d A=%d want %v", d.frozenUsdt[3], d.frozenUsdt[2], d.frozenUsdt[1], parts)
	}
	var manageN int
	for _, e := range d.ledger {
		if e.Type == biz.LedgerManageReward {
			manageN++
			if e.RefType != "manage" || e.RefID != 4 {
				t.Fatalf("ledger %+v", e)
			}
		}
	}
	if manageN != 3 {
		t.Fatalf("manage entries %d", manageN)
	}
}

func TestManageRewardUsesCappedPair(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
		path:     t.TempDir() + "/mall.json",
	}
	a := &biz.User{ID: 1, Nickname: "A"}
	y := &biz.User{ID: 2, Nickname: "Y", InviterID: 1, LeftID: 3, RightID: 4}
	l := &biz.User{ID: 3, ParentID: 2, Side: biz.TrackLeft, OccupiedPos: 1, PerfFen: biz.U(200000)}
	r := &biz.User{ID: 4, ParentID: 2, Side: biz.TrackRight, OccupiedPos: 1, PerfFen: biz.U(200000)}
	d.users[1], d.users[2], d.users[3], d.users[4] = a, y, l, r
	d.orders[1] = paidPkg(1, 2, biz.U(1000))
	d.orders[2] = paidPkg(2, 1, biz.U(1000))
	d.settlePairingLocked()
	cap := biz.PairCapFen(biz.U(1000))
	if d.frozenUsdt[2] != usdtHalf(cap) {
		t.Fatalf("Y capped %d", d.frozenUsdt[2])
	}
	if d.frozenUsdt[1] != usdtHalf(biz.ManageRewardFen(cap)) {
		t.Fatalf("管理奖应按封顶后的对碰奖计算 %d", d.frozenUsdt[1])
	}
}
