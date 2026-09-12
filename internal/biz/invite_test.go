package biz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
)

func TestDualTrackInvite(t *testing.T) {
	InitAuth("s")
	repo := newMemUserRepo()
	users := NewUserUsecase(repo, log.DefaultLogger)
	a, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	b, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	c, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xcccccccccccccccccccccccccccccccccccccccc")
	d, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xdddddddddddddddddddddddddddddddddddddddd")

	bind := func(u *User, code string) {
		t.Helper()
		tok, _ := GenerateToken(u.ID, u.WalletAddress, 0)
		uid, w, ver, _ := ParseToken(tok)
		ctx := WithTokenVer(WithWallet(WithUserID(context.Background(), uid), w), ver)
		if _, err := users.BindInvite(ctx, code); err != nil {
			t.Fatal(err)
		}
	}
	bind(b, a.InviteCode)
	bind(c, a.InviteCode)
	bind(d, a.InviteCode)

	b2, _ := repo.GetByID(context.Background(), b.ID)
	c2, _ := repo.GetByID(context.Background(), c.ID)
	d2, _ := repo.GetByID(context.Background(), d.ID)
	if b2.Side != TrackLeft || b2.ParentID != a.ID || b2.OccupiedPos != 1 {
		t.Fatalf("B %+v", b2)
	}
	if c2.Side != TrackRight || c2.ParentID != a.ID || c2.OccupiedPos != 1 {
		t.Fatalf("C %+v", c2)
	}
	if d2.ParentID != a.ID || d2.Side != TrackLeft || d2.OccupiedPos != 2 {
		t.Fatalf("D should fill A-L pos2: %+v", d2)
	}

	tokA, _ := GenerateToken(a.ID, a.WalletAddress, 0)
	aid, aw, aver, _ := ParseToken(tokA)
	ctxA := WithTokenVer(WithWallet(WithUserID(context.Background(), aid), aw), aver)
	sum, err := users.GetInviteSummary(ctxA)
	if err != nil {
		t.Fatal(err)
	}
	if sum.LeftCount != 2 || sum.RightCount != 1 {
		t.Fatalf("counts L=%d R=%d", sum.LeftCount, sum.RightCount)
	}
}
