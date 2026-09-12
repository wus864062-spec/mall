package biz

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/go-kratos/kratos/v2/log"
)

type User struct {
	ID                  int64
	WalletAddress       string
	Nickname            string
	Avatar              string
	InviteCode          string
	InviterID           int64
	ParentID            int64
	Side                string
	LeftID              int64
	RightID             int64
	ChainNextID         int64
	OccupiedPos         int64
	PerfFen             int64
	SettledPairFen      int64 // 已对碰消耗的大区业绩（与封顶无关）
	PairClearedRightFen int64 // 小区已清零到的业绩水位
	TokenVersion        int64
	CreatedAt           int64
}

const (
	TrackLeft  = "left"  // 大区
	TrackRight = "right" // 小区
)

type UserRepo interface {
	GetOrCreateByWallet(ctx context.Context, wallet string) (*User, bool, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByInviteCode(ctx context.Context, code string) (*User, error)
	ListInvitees(ctx context.Context, inviterID int64) ([]*User, error)
	Update(ctx context.Context, u *User) error
	PlaceInBinary(ctx context.Context, sponsorID, inviteeID int64) error
}

type UserUsecase struct {
	repo UserRepo
	log  *log.Helper
}

func NewUserUsecase(repo UserRepo, logger log.Logger) *UserUsecase {
	return &UserUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *UserUsecase) WalletLogin(ctx context.Context, message, signature, inviteCode string) (*User, string, error) {
	siwe, err := ParseSIWEMessage(message)
	if err != nil {
		return nil, "", err
	}
	if err := siwe.Validate(ctx); err != nil {
		return nil, "", err
	}
	if err := ConsumeNonce(siwe.Address, siwe.Nonce); err != nil {
		return nil, "", err
	}

	recovered, err := VerifySignature(message, signature)
	if err != nil {
		return nil, "", ErrInvalidArgument
	}
	if !strings.EqualFold(recovered, siwe.Address) {
		return nil, "", ErrInvalidArgument
	}

	wallet := strings.ToLower(siwe.Address)
	user, created, err := uc.repo.GetOrCreateByWallet(ctx, wallet)
	if err != nil {
		return nil, "", err
	}
	if inviteCode != "" && user.InviterID == 0 {
		if err := uc.bindInvite(ctx, user, inviteCode); err != nil && created {
			return nil, "", err
		}
		user, err = uc.repo.GetByID(ctx, user.ID)
		if err != nil {
			return nil, "", err
		}
	}

	token, err := GenerateToken(user.ID, user.WalletAddress, user.TokenVersion)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (uc *UserUsecase) SessionUser(ctx context.Context) (*User, error) {
	id, err := RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	u, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if TokenVerFromContext(ctx) != u.TokenVersion {
		return nil, ErrUnauthorized
	}
	return u, nil
}

func (uc *UserUsecase) GetMe(ctx context.Context) (*User, error) {
	return uc.SessionUser(ctx)
}

func (uc *UserUsecase) GetByID(ctx context.Context, id int64) (*User, error) {
	return uc.repo.GetByID(ctx, id)
}

func (uc *UserUsecase) UpdateMe(ctx context.Context, nickname, avatar string) (*User, error) {
	u, err := uc.SessionUser(ctx)
	if err != nil {
		return nil, err
	}
	nickname, avatar, err = NormalizeProfile(nickname, avatar)
	if err != nil {
		return nil, err
	}
	u.Nickname = nickname
	u.Avatar = avatar
	if err := uc.repo.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (uc *UserUsecase) Logout(ctx context.Context) error {
	u, err := uc.SessionUser(ctx)
	if err != nil {
		return err
	}
	u.TokenVersion++
	return uc.repo.Update(ctx, u)
}

func NormalizeProfile(nickname, avatar string) (string, string, error) {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" || utf8.RuneCountInString(nickname) > 32 {
		return "", "", ErrInvalidArgument
	}
	if strings.ContainsAny(nickname, "\n\r\t") {
		return "", "", ErrInvalidArgument
	}
	avatar = strings.TrimSpace(avatar)
	if avatar == "" {
		return nickname, "", nil
	}
	if len(avatar) > 512 {
		return "", "", ErrInvalidArgument
	}
	u, err := url.Parse(avatar)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", "", ErrInvalidArgument
	}
	return nickname, avatar, nil
}

func DefaultNickname(wallet string) string {
	if len(wallet) < 6 {
		return "用户"
	}
	return "用户" + wallet[len(wallet)-6:]
}

func InviteCodeFor(id int64) string {
	return fmt.Sprintf("INV%06d", id)
}
