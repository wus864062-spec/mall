package data

import (
	"context"
	"testing"
	"time"

	"mall/internal/biz"
)

func TestAdminSearchByIDAndInvite(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{},
		balances: map[int64]int64{2: 12300},
		orders:   map[int64]*biz.Order{},
		path:     t.TempDir() + "/mall.json",
	}
	d.users[1] = &biz.User{ID: 1, Nickname: "A", InviteCode: "INVA", WalletAddress: "0xaaa"}
	d.users[2] = &biz.User{ID: 2, Nickname: "B", InviteCode: "INVB", WalletAddress: "0xbbb", InviterID: 1, ParentID: 1, Side: biz.TrackLeft}
	repo := NewAdminRepo(d)
	ctx, cancel := context.WithTimeout(context.Background(), biz.SearchTimeout)
	defer cancel()
	res, err := repo.Search(ctx, "INVB")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 1 || res.Hits[0].User.ID != 2 {
		t.Fatalf("invite hit %+v", res.Hits)
	}
	if res.Hits[0].BalanceFen != 12300 {
		t.Fatalf("balance %d", res.Hits[0].BalanceFen)
	}
	byID, err := repo.Search(ctx, "2")
	if err != nil || len(byID.Hits) != 1 || byID.Hits[0].User.ID != 2 {
		t.Fatalf("id hit %+v %v", byID, err)
	}
}

func TestAdminSearchHonorsTimeout(t *testing.T) {
	d := &Data{users: map[int64]*biz.User{1: {ID: 1, InviteCode: "INVA"}}, path: t.TempDir() + "/mall.json"}
	repo := NewAdminRepo(d)
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)
	if _, err := repo.Search(ctx, "INVA"); err == nil {
		t.Fatal("expected timeout")
	}
}

func TestSyncInviteCodes(t *testing.T) {
	d := &Data{
		users: map[int64]*biz.User{
			1: {ID: 1, WalletAddress: "0xabc", InviteCode: "INV000001"},
		},
		byWallet: map[string]int64{"0xabc": 1},
		byInvite: map[string]int64{"INV000001": 1},
	}
	if !d.syncInviteCodesLocked() {
		t.Fatal("expected rewrite")
	}
	if d.users[1].InviteCode != "0xabc" {
		t.Fatalf("got %s", d.users[1].InviteCode)
	}
	if _, ok := d.byInvite["INV000001"]; ok {
		t.Fatal("old INV index should be gone")
	}
	if d.byInvite["0xabc"] != 1 {
		t.Fatalf("byInvite %+v", d.byInvite)
	}
}
