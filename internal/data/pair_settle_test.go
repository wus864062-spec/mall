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
	d.products[1] = &biz.Product{ID: 1, Name: "1000USTD认购", PriceFen: 100000, Stock: 10}
	d.balances[2] = 100000
	shop := NewShopRepo(d)
	if _, err := shop.PlaceOrder(context.Background(), 2, 1, 1); err != nil {
		t.Fatal(err)
	}
	for _, e := range d.ledger {
		if e.Type == biz.LedgerPairReward || e.Type == biz.LedgerPairUpline {
			t.Fatal("pair should wait until midnight")
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
	const unit int64 = 100000
	b2.PerfFen = 5 * unit
	c.PerfFen = 4 * unit
	dnext.PerfFen = 4 * unit
	d1.PerfFen = 2 * unit
	d.orders = map[int64]*biz.Order{
		1: paidPkg(1, 1, 100000),
		2: paidPkg(2, 2, 100000),
		3: paidPkg(3, 4, 100000),
	}

	d.settlePairingLocked()
	aBonus := biz.PairRewardFen(4 * unit)
	bBonus := biz.PairRewardFen(4 * unit)
	dBonus := biz.PairRewardFen(2 * unit)
	if d.balances[1] != aBonus {
		t.Fatalf("A pair 10%% %d want %d", d.balances[1], aBonus)
	}
	if d.balances[2] != bBonus {
		t.Fatalf("B %d want %d", d.balances[2], bBonus)
	}
	if d.balances[4] != dBonus {
		t.Fatalf("D %d want %d", d.balances[4], dBonus)
	}
	if a.SettledPairFen != 4*unit {
		t.Fatalf("A left consumed %d", a.SettledPairFen)
	}
	if a.PairClearedRightFen != 4*unit {
		t.Fatalf("A right cleared %d", a.PairClearedRightFen)
	}

	c.PerfFen = 5 * unit
	d.settlePairingLocked()
	if a.SettledPairFen != 5*unit {
		t.Fatalf("大区剩余次日再碰 %d", a.SettledPairFen)
	}
	if d.balances[1] != aBonus+biz.PairRewardFen(unit) {
		t.Fatalf("A after leftover %d", d.balances[1])
	}

	c.PerfFen = 8 * unit
	d.settlePairingLocked()
	if a.PairClearedRightFen != 8*unit {
		t.Fatalf("小区清零水位 %d", a.PairClearedRightFen)
	}
	if a.SettledPairFen != 5*unit {
		t.Fatalf("小区多出部分丢弃，大区不再被多扣 %d", a.SettledPairFen)
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
	d.users[2] = &biz.User{ID: 2, Nickname: "B", PerfFen: 100000}
	d.users[3] = &biz.User{ID: 3, Nickname: "C", PerfFen: 100000}
	if err := biz.PlaceSharedTrack(d.users, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := biz.PlaceSharedTrack(d.users, 1, 3); err != nil {
		t.Fatal(err)
	}
	d.orders = map[int64]*biz.Order{1: paidPkg(1, 1, 100000)}
	loc := pairSettleLocation()
	now := time.Date(2026, 9, 11, 0, 0, 5, 0, loc)
	if !d.maybeSettlePairing(now) {
		t.Fatal("expected settle after midnight")
	}
	if d.lastPairSettleDate != "2026-09-11" {
		t.Fatalf("date %s", d.lastPairSettleDate)
	}
	if d.balances[1] == 0 {
		t.Fatal("A should receive pair at midnight")
	}
	if d.maybeSettlePairing(now.Add(time.Hour)) {
		t.Fatal("should not settle twice the same day")
	}
}

func paidPkg(id, userID, priceFen int64) *biz.Order {
	return &biz.Order{
		ID:     id,
		UserID: userID,
		Status: biz.OrderPaid,
		Items:  []*biz.OrderItem{{PriceFen: priceFen, Quantity: 1, AmountFen: priceFen}},
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
	b := &biz.User{ID: 2, Nickname: "B", ParentID: 1, Side: biz.TrackLeft, OccupiedPos: 1, PerfFen: 20000000}
	c := &biz.User{ID: 3, Nickname: "C", ParentID: 1, Side: biz.TrackRight, OccupiedPos: 1, PerfFen: 20000000}
	d.users[1], d.users[2], d.users[3] = a, b, c
	d.orders[1] = paidPkg(1, 1, 100000)
	d.settlePairingLocked()
	cap := biz.PairCapFen(100000)
	if cap != 60000 {
		t.Fatalf("cap %d", cap)
	}
	if d.balances[1] != cap {
		t.Fatalf("paid %d want cap %d", d.balances[1], cap)
	}
	if a.SettledPairFen != 20000000 {
		t.Fatalf("封顶不影响大小区结算 %d", a.SettledPairFen)
	}
	if a.PairClearedRightFen != 20000000 {
		t.Fatalf("小区应清零 %d", a.PairClearedRightFen)
	}

	before := d.balances[1]
	d.settlePairingLocked()
	if d.balances[1] != before {
		t.Fatalf("封顶未发部分不计入次日奖励 %d", d.balances[1])
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
	b := &biz.User{ID: 2, Nickname: "B", ParentID: 1, Side: biz.TrackLeft, OccupiedPos: 1, PerfFen: 20000000}
	c := &biz.User{ID: 3, Nickname: "C", ParentID: 1, Side: biz.TrackRight, OccupiedPos: 1, PerfFen: 5000000}
	d.users[1], d.users[2], d.users[3] = a, b, c
	d.orders[1] = paidPkg(1, 1, 100000)
	d.settlePairingLocked()
	if d.balances[1] != biz.PairCapFen(100000) {
		t.Fatalf("当日只发到封顶 %d", d.balances[1])
	}
	if a.SettledPairFen != 5000000 {
		t.Fatalf("大区应减去全部小区 %d", a.SettledPairFen)
	}
	if a.PairClearedRightFen != 5000000 {
		t.Fatalf("小区清零 %d", a.PairClearedRightFen)
	}

	c.PerfFen = 8000000
	d.settlePairingLocked()
	if a.SettledPairFen != 8000000 {
		t.Fatalf("次日只碰大区剩余 %d", a.SettledPairFen)
	}
	want := biz.PairCapFen(100000) + biz.CapPairPayout(biz.PairRewardFen(3000000), biz.PairCapFen(100000))
	if d.balances[1] != want {
		t.Fatalf("次日奖励 %d want %d", d.balances[1], want)
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
	l := &biz.User{ID: 5, Nickname: "L", ParentID: 4, Side: biz.TrackLeft, OccupiedPos: 1, PerfFen: 100000}
	r := &biz.User{ID: 6, Nickname: "R", ParentID: 4, Side: biz.TrackRight, OccupiedPos: 1, PerfFen: 100000}
	d.users[1], d.users[2], d.users[3], d.users[4], d.users[5], d.users[6] = a, b, x, y, l, r
	d.orders[1] = paidPkg(1, 4, 100000)
	d.settlePairingLocked()

	pairPay := biz.PairRewardFen(100000)
	if d.balances[4] != pairPay {
		t.Fatalf("Y 对碰奖应独立全额 %d", d.balances[4])
	}
	pool := biz.ManageRewardFen(pairPay)
	parts := biz.SplitEvenFen(pool, 3)
	if d.balances[3] != parts[0] || d.balances[2] != parts[1] || d.balances[1] != parts[2] {
		t.Fatalf("管理奖 X=%d B=%d A=%d want %v", d.balances[3], d.balances[2], d.balances[1], parts)
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
	l := &biz.User{ID: 3, ParentID: 2, Side: biz.TrackLeft, OccupiedPos: 1, PerfFen: 20000000}
	r := &biz.User{ID: 4, ParentID: 2, Side: biz.TrackRight, OccupiedPos: 1, PerfFen: 20000000}
	d.users[1], d.users[2], d.users[3], d.users[4] = a, y, l, r
	d.orders[1] = paidPkg(1, 2, 100000)
	d.settlePairingLocked()
	cap := biz.PairCapFen(100000)
	if d.balances[2] != cap {
		t.Fatalf("Y capped %d", d.balances[2])
	}
	if d.balances[1] != biz.ManageRewardFen(cap) {
		t.Fatalf("管理奖应按封顶后的对碰奖计算 %d", d.balances[1])
	}
}
