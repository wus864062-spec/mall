package biz

import (
	"fmt"
	"testing"
)

func TestV6SharedAreaReport(t *testing.T) {
	users := map[int64]*User{}
	mk := func(id int64, name string) *User {
		u := &User{ID: id, Nickname: name, InviteCode: fmt.Sprintf("INV%06d", id)}
		users[id] = u
		return u
	}
	a := mk(1, "A")
	b := mk(2, "B")
	c := mk(3, "C")
	a3 := mk(4, "A3")
	a4 := mk(5, "A4")
	d := mk(6, "D")
	b2 := mk(7, "B2")
	b3 := mk(8, "B3")
	b4 := mk(9, "B4")
	b5 := mk(10, "B5")
	b6 := mk(11, "B6")
	e := mk(12, "E")
	c2 := mk(13, "C2")
	c3 := mk(14, "C3")
	c4 := mk(15, "C4")
	c5 := mk(16, "C5")
	d1 := mk(17, "D1")
	d2 := mk(18, "D2")
	d3 := mk(19, "D3")
	e1 := mk(20, "E1")
	e2 := mk(21, "E2")

	place := func(sponsor, invitee *User) {
		t.Helper()
		if err := PlaceSharedTrack(users, sponsor.ID, invitee.ID); err != nil {
			t.Fatalf("place %s under %s: %v", invitee.Nickname, sponsor.Nickname, err)
		}
	}
	place(a, b)
	place(a, c)
	place(a, a3)
	place(a, a4)
	place(b, d)
	place(b, b2)
	place(b, b3)
	place(b, b4)
	place(b, b5)
	place(b, b6)
	place(c, e)
	place(c, c2)
	place(c, c3)
	place(c, c4)
	place(c, c5)
	place(d, d1)
	place(d, d2)
	place(d, d3)
	place(e, e1)
	place(e, e2)

	names := func(list []*User) []string {
		out := make([]string, 0, len(list))
		for _, u := range list {
			out = append(out, u.Nickname)
		}
		return out
	}
	eq := func(got []string, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("got %v want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v want %v", got, want)
			}
		}
	}

	eq(names(LeftMembersOf(users, a)), "B", "A3", "B2", "B4", "B6")
	eq(names(RightMembersOf(users, a)), "C", "A4", "C2", "C4")
	eq(names(LeftMembersOf(users, b)), "A3", "B2", "B4", "B6")
	eq(names(RightMembersOf(users, b)), "D", "B3", "B5", "D3")
	eq(names(LeftMembersOf(users, c)), "A4", "C2", "C4")
	eq(names(RightMembersOf(users, c)), "E", "C3", "C5")
	eq(names(LeftMembersOf(users, d)), "B3", "B5", "D3")
	eq(names(RightMembersOf(users, d)), "D1", "D2")
	eq(names(LeftMembersOf(users, e)), "C3", "C5")
	eq(names(RightMembersOf(users, e)), "E1", "E2")

	if d.ParentID != b.ID || d.Side != TrackRight || d.OccupiedPos != 1 {
		t.Fatalf("D %+v", d)
	}
	if d3.ParentID != b.ID || d3.Side != TrackRight || d3.OccupiedPos != 4 {
		t.Fatalf("D3 %+v", d3)
	}

	const unit int64 = 100000
	for _, u := range users {
		if u.ID != a.ID {
			u.PerfFen = unit
		}
	}

	checkPair := func(u *User, leftN, rightN int) {
		t.Helper()
		lp := SumPerf(LeftMembersOf(users, u))
		rp := SumPerf(RightMembersOf(users, u))
		if lp != int64(leftN)*unit || rp != int64(rightN)*unit {
			t.Fatalf("%s perf L=%d R=%d want %d/%d", u.Nickname, lp, rp, leftN, rightN)
		}
		bonus := PairRewardFen(PairFen(lp, rp))
		want := PairRewardFen(int64(min(leftN, rightN)) * unit)
		if bonus != want {
			t.Fatalf("%s pair bonus %d want %d", u.Nickname, bonus, want)
		}
	}
	checkPair(a, 5, 4)
	checkPair(b, 4, 4)
	checkPair(c, 3, 3)
	checkPair(d, 3, 2)
	checkPair(e, 2, 2)

	if PairRewardFen(400000) != 40000 {
		t.Fatalf("A pairing 10%% of 4000")
	}
	if DirectRewardFen(4*unit) != 40000 {
		t.Fatalf("A direct 10%% of 4*1000")
	}
}

func TestAreasFromPerfUsesBuyNotSide(t *testing.T) {
	got := AreasFromPerf(0, 0)
	if got.HasAreas || got.LargeFen != 0 || got.SmallFen != 0 {
		t.Fatal("左右都 0：下线没买过，没有大小区")
	}
	got = AreasFromPerf(U(1000), 0)
	if !got.HasAreas || got.LargeFen != U(1000) || got.SmallFen != 0 || got.LargeSide != TrackLeft {
		t.Fatalf("只有左区有认购 %+v", got)
	}
	got = AreasFromPerf(U(1000), U(3000))
	if !got.HasAreas || got.LargeFen != U(3000) || got.SmallFen != U(1000) || got.LargeSide != TrackRight || got.SmallSide != TrackLeft {
		t.Fatalf("右区业绩更大才是大区 %+v", got)
	}
	got = AreasFromPerf(U(2000), U(2000))
	if !got.HasAreas || got.LargeFen != U(2000) || got.SmallFen != U(2000) || got.LargeSide != TrackLeft {
		t.Fatalf("相等时金额相同，展示大区落左 %+v", got)
	}
}

func TestOwnPerfNotInOwnTracks(t *testing.T) {
	users := map[int64]*User{
		1: {ID: 1, Nickname: "A", LeftID: 2, RightID: 3},
		2: {ID: 2, Nickname: "B", ParentID: 1, Side: TrackLeft, OccupiedPos: 1, PerfFen: 100000, RightID: 4},
		3: {ID: 3, Nickname: "C", ParentID: 1, Side: TrackRight, OccupiedPos: 1, PerfFen: 100000},
		4: {ID: 4, Nickname: "D", ParentID: 2, Side: TrackRight, OccupiedPos: 1, PerfFen: 50000},
	}
	if SumPerf(LeftMembersOf(users, users[2])) != 0 {
		t.Fatal("B 自己认购不算进自己的左区")
	}
	if SumPerf(RightMembersOf(users, users[2])) != 50000 {
		t.Fatal("B 右区只含下线")
	}
	if SumPerf(LeftMembersOf(users, users[1])) != 100000 || SumPerf(RightMembersOf(users, users[1])) != 100000 {
		t.Fatal("B/C 认购仍进 A 的双轨")
	}
}

func TestPairCapBySubscribeTier(t *testing.T) {
	cases := []struct{ price, cap int64 }{
		{U(1000), U(600)},
		{U(2000), U(1200)},
		{U(3000), U(1800)},
		{U(6000), U(4000)},
		{U(10000), U(8000)},
		{U(12000), U(8000)},
		{U(24000), U(16000)},
		{U(36000), U(24000)},
		{U(50000), U(30000)},
		{U(70000), U(42000)},
		{U(100000), U(60000)},
		{U(160000), U(100000)},
	}
	for _, c := range cases {
		if got := PairCapFen(c.price); got != c.cap {
			t.Fatalf("price %d cap %d want %d", c.price, got, c.cap)
		}
	}
	if CapPairPayout(100000, 60000) != 60000 {
		t.Fatal("1000档封顶 600")
	}
	if CapDynamicPayout(U(500), U(600), U(200)) != U(400) {
		t.Fatal("剩余额度")
	}
	if CapDynamicPayout(U(500), U(600), U(600)) != 0 {
		t.Fatal("用尽为 0")
	}
	if CapDynamicPayout(U(100), 0, 0) != 0 {
		t.Fatal("无档位对碰/管理奖为 0")
	}
}

func TestPairCapFenWithUsesConfigNotDefault(t *testing.T) {
	// 配置项改的是日封顶；认购档位仍按目录，锁仓仍按认购总额。
	over := map[int64]int64{U(1000): U(400)}
	if PairCapFenWith(U(1000), over) != U(400) {
		t.Fatal("1000 档用配置 400，不用默认 600")
	}
	if PairCapFen(U(1000)) != U(600) {
		t.Fatal("默认表仍是 600")
	}
	if UnfreezeCapFenWith(U(1000), over) != U(400) {
		t.Fatal("解冻额度用配置封顶")
	}
	if GrantUnfreezeFenWith(0, U(1000), over) != U(400) {
		t.Fatal("当天未发按配置给 400")
	}
	if GrantUnfreezeFenWith(U(400), U(1000), over) != 0 {
		t.Fatal("配置封顶已发满不加")
	}
	over[U(1000)] = 0
	if PairCapFenWith(U(1000), over) != 0 {
		t.Fatal("配置 0 表示该档当天不发")
	}
	if UnfreezeCapFenWith(U(2000), over) != U(1200) {
		t.Fatal("没改的档仍用默认")
	}
}

func TestUnfreezeCapUsesPaidTierNotItemSum(t *testing.T) {
	if got := UnfreezeCapFen(U(1000)); got != U(600) {
		t.Fatalf("1000 cap %d", got)
	}
	if got := UnfreezeCapFen(U(1000) + U(1000)); got != U(1200) {
		t.Fatalf("1000+1000 升 2000 档 cap %d want 1200 not 600", got)
	}
	if got := UnfreezeCapFen(U(2000)); got != U(1200) {
		t.Fatalf("2000 cap %d", got)
	}
	if got := UnfreezeCapFen(U(1000) + U(3000)); got != U(1800) {
		t.Fatalf("1000+3000 cap %d want 1800 not 2400", got)
	}
	if UnfreezeCapFen(U(500)) != 0 {
		t.Fatal("不到 1000 档封顶应为 0")
	}
	if GrantUnfreezeFen(0, U(1000)) != U(600) {
		t.Fatal("当天未发、1000 档给 600")
	}
	if GrantUnfreezeFen(U(600), U(2000)) != U(600) {
		t.Fatal("当天已发 600、升 2000 档只补 1200-600")
	}
	if GrantUnfreezeFen(U(1200), U(2000)) != 0 {
		t.Fatal("当天已发 1200、仍 2000 档不加")
	}
	if GrantUnfreezeFen(U(1800), U(3000)) != 0 {
		t.Fatal("当天已发 1800、仍 3000 档不加")
	}
	if GrantUnfreezeFen(U(1800), U(6000)) != U(2200) {
		t.Fatal("当天 3000→6000 只补 4000-1800")
	}
	if UnfreezeGrantedTodayFen("2026-09-01", "2026-09-02", U(1800)) != 0 {
		t.Fatal("换日视为未发")
	}
	if TakeUnfreeze(U(600), U(1000)) != U(600) {
		t.Fatal("min(额度, 冻结)")
	}
	if TakeUnfreeze(U(600), U(100)) != U(100) {
		t.Fatal("冻结不足")
	}
}

func TestEffectivePackageFen(t *testing.T) {
	if got := EffectivePackageFen(U(1000) + U(1000)); got != U(2000) {
		t.Fatalf("1000+1000 升 2000 档 got %d", got)
	}
	if CartExcessFen(U(2000)) != 0 {
		t.Fatal("2000 正好是档位，超额 0，锁仓按总额折")
	}
	if got := EffectivePackageFen(U(1500)); got != U(1000) {
		t.Fatalf("1500 不到 2000 仍 1000 档 got %d", got)
	}
	if got := EffectivePackageFen(U(1000) + U(3000)); got != U(3000) {
		t.Fatalf("1000+3000 got %d", got)
	}
	if got := EffectivePackageFen(U(12000) + U(24000)); got != U(36000) {
		t.Fatalf("12000+24000 got %d", got)
	}
	if got := EffectivePackageFen(U(3000) + U(3000)); got != U(6000) {
		t.Fatalf("3000+3000 got %d", got)
	}
}

func TestInviteAncestorsAndManageSplit(t *testing.T) {
	users := map[int64]*User{
		1: {ID: 1},
		2: {ID: 2, InviterID: 1},
		3: {ID: 3, InviterID: 2},
		4: {ID: 4, InviterID: 3, ParentID: 99},
	}
	got := InviteAncestors(users, users[4], 3)
	if len(got) != 3 || got[0].ID != 3 || got[1].ID != 2 || got[2].ID != 1 {
		t.Fatalf("直系上三级 %+v", got)
	}
	if ManageRewardFen(1000) != 300 {
		t.Fatal("管理奖池 30%")
	}
	parts := SplitEvenFen(300, 3)
	if len(parts) != 3 || parts[0]+parts[1]+parts[2] != 300 {
		t.Fatalf("%v", parts)
	}
	if parts[0] != 100 {
		t.Fatalf("余数给一代 %v", parts)
	}
}

func TestCartExcessUsesPaidSumNotTier(t *testing.T) {
	// 1000+3000=4000：档位 3000，超额 1000；锁仓必须按 4000 折，不能只折 3000。
	total := U(4000)
	if EffectivePackageFen(total) != U(3000) {
		t.Fatalf("tier %d", EffectivePackageFen(total))
	}
	if CartExcessFen(total) != U(1000) {
		t.Fatalf("excess %d", CartExcessFen(total))
	}
	got := CoinsMicroFromPay(total, CategoryWeb3)
	if got != CoinsMicroFromPay(U(3000), CategoryWeb3)+CoinsMicroFromPay(U(1000), CategoryWeb3) {
		t.Fatalf("lock from total %d", got)
	}
	if CoinsMicroFromPay(EffectivePackageFen(total), CategoryWeb3) == got {
		t.Fatal("must not pass tier into CoinsMicroFromPay only")
	}
}
