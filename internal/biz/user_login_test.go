package biz

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-kratos/kratos/v2/log"
)

type memUserRepo struct {
	users    map[int64]*User
	byWallet map[string]int64
	seq      int64
}

func newMemUserRepo() *memUserRepo {
	return &memUserRepo{users: map[int64]*User{}, byWallet: map[string]int64{}}
}

func (r *memUserRepo) GetOrCreateByWallet(ctx context.Context, wallet string) (*User, bool, error) {
	wallet = strings.ToLower(wallet)
	if id, ok := r.byWallet[wallet]; ok {
		cp := *r.users[id]
		return &cp, false, nil
	}
	r.seq++
	u := &User{
		ID:            r.seq,
		WalletAddress: wallet,
		Nickname:      DefaultNickname(wallet),
		InviteCode:    wallet,
	}
	r.users[u.ID] = u
	r.byWallet[wallet] = u.ID
	cp := *u
	return &cp, true, nil
}

func (r *memUserRepo) GetByInviteCode(ctx context.Context, code string) (*User, error) {
	code = NormalizeInviteCode(code)
	if code == "" {
		return nil, ErrInviteInvalid
	}
	if id, ok := r.byWallet[strings.ToLower(code)]; ok {
		cp := *r.users[id]
		return &cp, nil
	}
	return nil, ErrInviteInvalid
}

func (r *memUserRepo) ListInvitees(ctx context.Context, inviterID int64) ([]*User, error) {
	var out []*User
	for _, u := range r.users {
		if u.InviterID == inviterID {
			cp := *u
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *memUserRepo) GetByID(ctx context.Context, id int64) (*User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, ErrUserNotExist
	}
	cp := *u
	return &cp, nil
}

func (r *memUserRepo) Update(ctx context.Context, u *User) error {
	cur, ok := r.users[u.ID]
	if !ok {
		return ErrUserNotExist
	}
	*cur = *u
	return nil
}

func (r *memUserRepo) HasEarlierNonAdmin(ctx context.Context, userID int64) (bool, error) {
	for _, u := range r.users {
		if u == nil || u.ID == userID {
			continue
		}
		if IsAdmin(u) {
			continue
		}
		if u.ID < userID {
			return true, nil
		}
	}
	return false, nil
}

func (r *memUserRepo) HasInviteRoot(ctx context.Context, excludeID int64) (bool, error) {
	for _, u := range r.users {
		if u == nil || u.ID == excludeID {
			continue
		}
		if u.InviteRoot {
			return true, nil
		}
	}
	return false, nil
}

func (r *memUserRepo) PlaceInBinary(ctx context.Context, sponsorID, inviteeID int64) error {
	return PlaceSharedTrack(r.users, sponsorID, inviteeID)
}

func TestWalletLoginAndUpdateMe(t *testing.T) {
	InitAuth("test-secret")
	uc := NewUserUsecase(newMemUserRepo(), log.DefaultLogger)

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := crypto.PubkeyToAddress(key.PublicKey).Hex()
	nonce, err := GenerateNonce(addr)
	if err != nil {
		t.Fatal(err)
	}

	issued := time.Now().UTC().Format(time.RFC3339)
	message := fmt.Sprintf("example.com wants you to sign in with your Ethereum account:\n%s\n\nSign in to Mall\n\nURI: http://example.com\nVersion: 1\nChain ID: 1\nNonce: %s\nIssued At: %s", addr, nonce, issued)

	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	hash := crypto.Keccak256Hash([]byte(prefix + message))
	sig, err := crypto.Sign(hash.Bytes(), key)
	if err != nil {
		t.Fatal(err)
	}

	user, token, err := uc.WalletLogin(context.Background(), message, fmt.Sprintf("0x%x", sig), "")
	if err != nil {
		t.Fatal(err)
	}
	if user.InviteCode == "" || token == "" {
		t.Fatalf("user=%+v token=%s", user, token)
	}

	uid, wallet, ver, err := ParseToken(token)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithTokenVer(WithWallet(WithUserID(context.Background(), uid), wallet), ver)

	me, err := uc.GetMe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if me.ID != user.ID {
		t.Fatal(me)
	}

	updated, err := uc.UpdateMe(ctx, "小明", "https://cdn.example.com/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Nickname != "小明" {
		t.Fatal(updated.Nickname)
	}

	if err := uc.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.GetMe(ctx); err == nil {
		t.Fatal("old token should fail after logout")
	}
}

func TestSessionUserLocked(t *testing.T) {
	InitAuth("test-secret")
	repo := newMemUserRepo()
	uc := NewUserUsecase(repo, log.DefaultLogger)
	u, _, err := repo.GetOrCreateByWallet(context.Background(), "0xabc")
	if err != nil {
		t.Fatal(err)
	}
	u.Locked = true
	if err := repo.Update(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	ctx := WithTokenVer(WithUserID(context.Background(), u.ID), u.TokenVersion)
	if _, err := uc.SessionUser(ctx); err != ErrUserLocked {
		t.Fatalf("want locked, got %v", err)
	}
}

func TestNeedInviteFirstWalletSkipsCode(t *testing.T) {
	repo := newMemUserRepo()
	uc := NewUserUsecase(repo, log.DefaultLogger)
	ctx := context.Background()

	admin, _, err := repo.GetOrCreateByWallet(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := repo.GetOrCreateByWallet(ctx, "0xaaa")
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := repo.GetOrCreateByWallet(ctx, "0xbbb")
	if err != nil {
		t.Fatal(err)
	}

	need, err := uc.NeedInvite(WithTokenVer(WithUserID(ctx, admin.ID), admin.TokenVersion))
	if err != nil || need {
		t.Fatalf("admin need=%v err=%v", need, err)
	}
	need, err = uc.NeedInvite(WithTokenVer(WithUserID(ctx, first.ID), first.TokenVersion))
	if err != nil || !need {
		t.Fatalf("first wallet 要用万能码 need=%v err=%v", need, err)
	}
	tok, _ := GenerateToken(first.ID, first.WalletAddress, 0)
	uid, w, ver, _ := ParseToken(tok)
	if _, err := uc.BindInvite(WithTokenVer(WithWallet(WithUserID(ctx, uid), w), ver), MasterInviteCode()); err != nil {
		t.Fatal(err)
	}
	need, err = uc.NeedInvite(WithTokenVer(WithUserID(ctx, first.ID), first.TokenVersion))
	if err != nil || need {
		t.Fatalf("用过万能码后 need=%v err=%v", need, err)
	}
	need, err = uc.NeedInvite(WithTokenVer(WithUserID(ctx, second.ID), second.TokenVersion))
	if err != nil || !need {
		t.Fatalf("second wallet need=%v err=%v", need, err)
	}
}
