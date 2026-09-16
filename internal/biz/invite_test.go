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
	if len(sum.Invitees) != 3 {
		t.Fatalf("直推应按 InviterID 列出 B/C/D，got %d", len(sum.Invitees))
	}
	if sum.HasAreas {
		t.Fatal("还没认购时没有大小区")
	}
	if len(sum.LeftMembers) != 2 || sum.LeftMembers[0].WalletAddress != b.WalletAddress {
		t.Fatalf("left members %+v", sum.LeftMembers)
	}
	if sum.LeftMembers[0].InviteCode != b.WalletAddress {
		t.Fatalf("invite code want wallet, got %s", sum.LeftMembers[0].InviteCode)
	}

	b2.PerfFen = 30000
	c2.PerfFen = 10000
	d2.PerfFen = 20000
	_ = repo.Update(context.Background(), b2)
	_ = repo.Update(context.Background(), c2)
	_ = repo.Update(context.Background(), d2)
	sum, err = users.GetInviteSummary(ctxA)
	if err != nil {
		t.Fatal(err)
	}
	if sum.LeftPerfFen != 50000 || sum.RightPerfFen != 10000 {
		t.Fatalf("perf L=%d R=%d", sum.LeftPerfFen, sum.RightPerfFen)
	}
	if !sum.HasAreas || sum.LargePerfFen != 50000 || sum.SmallPerfFen != 10000 || sum.LargeSide != TrackLeft {
		t.Fatalf("认购后左大右小 %+v", sum)
	}
	if sum.LeftMembers[0].PerfFen != 30000 || sum.LeftMembers[0].WalletAddress != b.WalletAddress {
		t.Fatalf("left member %+v", sum.LeftMembers[0])
	}
}

func TestNormalizeInviteCode(t *testing.T) {
	if got := NormalizeInviteCode(" 0xAbC "); got != "0xAbC" {
		t.Fatalf("got %s", got)
	}
	if got := NormalizeInviteCode("INV000002"); got != "INV000002" {
		t.Fatalf("trim only, got %s", got)
	}
}

func TestBindInviteRejectsGeneratedINV(t *testing.T) {
	InitAuth("s")
	repo := newMemUserRepo()
	users := NewUserUsecase(repo, log.DefaultLogger)
	a, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	b, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	tok, _ := GenerateToken(b.ID, b.WalletAddress, 0)
	uid, w, ver, _ := ParseToken(tok)
	ctx := WithTokenVer(WithWallet(WithUserID(context.Background(), uid), w), ver)
	if _, err := users.BindInvite(ctx, "INV000001"); err != ErrInviteInvalid {
		t.Fatalf("INV code should fail, got %v", err)
	}
	if a.InviteCode != a.WalletAddress {
		t.Fatalf("invite code %s want wallet", a.InviteCode)
	}
}

func TestBindInviteWalletCaseInsensitive(t *testing.T) {
	InitAuth("s")
	repo := newMemUserRepo()
	users := NewUserUsecase(repo, log.DefaultLogger)
	a, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	b, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	tok, _ := GenerateToken(b.ID, b.WalletAddress, 0)
	uid, w, ver, _ := ParseToken(tok)
	ctx := WithTokenVer(WithWallet(WithUserID(context.Background(), uid), w), ver)
	if _, err := users.BindInvite(ctx, "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"); err != nil {
		t.Fatal(err)
	}
	b2, _ := repo.GetByID(context.Background(), b.ID)
	if b2.InviterID != a.ID {
		t.Fatalf("want inviter A, got %+v", b2)
	}
}

func TestBindInviteSelfAndAlreadyBound(t *testing.T) {
	InitAuth("s")
	repo := newMemUserRepo()
	users := NewUserUsecase(repo, log.DefaultLogger)
	a, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	tok, _ := GenerateToken(a.ID, a.WalletAddress, 0)
	uid, w, ver, _ := ParseToken(tok)
	ctx := WithTokenVer(WithWallet(WithUserID(context.Background(), uid), w), ver)
	if _, err := users.BindInvite(ctx, a.InviteCode); err != nil {
		t.Fatal(err)
	}
}

func TestMasterInviteMakesFirstUser(t *testing.T) {
	InitAuth("s")
	repo := newMemUserRepo()
	uc := NewUserUsecase(repo, log.DefaultLogger)
	a, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	b, _, _ := repo.GetOrCreateByWallet(context.Background(), "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	bind := func(u *User, code string) error {
		t.Helper()
		tok, _ := GenerateToken(u.ID, u.WalletAddress, 0)
		uid, w, ver, _ := ParseToken(tok)
		ctx := WithTokenVer(WithWallet(WithUserID(context.Background(), uid), w), ver)
		_, err := uc.BindInvite(ctx, code)
		return err
	}
	if err := bind(a, MasterInviteCode()); err != nil {
		t.Fatal(err)
	}
	a2, _ := repo.GetByID(context.Background(), a.ID)
	if !a2.InviteRoot || a2.InviterID != 0 || a2.ParentID != 0 {
		t.Fatalf("万能码后应是根节点 %+v", a2)
	}
	if err := bind(b, MasterInviteCode()); err != ErrInviteInvalid {
		t.Fatalf("第二人不能再用万能码, got %v", err)
	}
	if err := bind(b, a.InviteCode); err != nil {
		t.Fatal(err)
	}
	b2, _ := repo.GetByID(context.Background(), b.ID)
	if b2.InviterID != a.ID || b2.Side != TrackLeft {
		t.Fatalf("第一人填钱包进左区 %+v", b2)
	}
}
