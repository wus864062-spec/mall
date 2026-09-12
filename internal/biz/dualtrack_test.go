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

func TestOwnPerfNotInOwnTracks(t *testing.T) {
	users := map[int64]*User{
		1: {ID: 1, Nickname: "A", LeftID: 2, RightID: 3},
		2: {ID: 2, Nickname: "B", ParentID: 1, Side: TrackLeft, OccupiedPos: 1, PerfFen: 100000, RightID: 4},
		3: {ID: 3, Nickname: "C", ParentID: 1, Side: TrackRight, OccupiedPos: 1, PerfFen: 100000},
		4: {ID: 4, Nickname: "D", ParentID: 2, Side: TrackRight, OccupiedPos: 1, PerfFen: 50000},
	}
	if SumPerf(LeftMembersOf(users, users[2])) != 0 {
		t.Fatal("B 自己认购不算进自己的大区")
	}
	if SumPerf(RightMembersOf(users, users[2])) != 50000 {
		t.Fatal("B 小区只含下线")
	}
	if SumPerf(LeftMembersOf(users, users[1])) != 100000 || SumPerf(RightMembersOf(users, users[1])) != 100000 {
		t.Fatal("B/C 认购仍进 A 的双轨")
	}
}

func TestPairCapBySubscribeTier(t *testing.T) {
	cases := []struct{ price, cap int64 }{
		{100000, 60000},
		{300000, 180000},
		{600000, 400000},
		{1000000, 800000},
		{1200000, 800000},
		{2400000, 1600000},
		{3600000, 2400000},
		{5000000, 3000000},
		{7000000, 4200000},
		{10000000, 6000000},
		{16000000, 10000000},
	}
	for _, c := range cases {
		if got := PairCapFen(c.price); got != c.cap {
			t.Fatalf("price %d cap %d want %d", c.price, got, c.cap)
		}
	}
	if CapPairPayout(100000, 60000) != 60000 {
		t.Fatal("1000档封顶 600")
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
