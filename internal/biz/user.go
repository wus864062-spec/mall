package biz

import (
	"context"
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
	SettledPairFen      int64  // 已对碰消耗的业绩（左右共用，与封顶无关）
	PairClearedRightFen int64  // 历史字段，不再按右区日清
	StaticDays          int64  // 静态窗长度：300 / 600 / 750
	StaticReleasedDays  int64  // 已释放天数；再认购不清零
	StaticPackageFen    int64  // 当前档位（目录向下取整）。锁仓枚数不存这里，见 ispayLocked
	DynamicRewardDay    string // 上海日历日 YYYY-MM-DD；换日清零 DynamicRewardFen
	DynamicRewardFen    int64  // 当日已发动态奖励（直推+对碰+管理奖，折前 fen）
	UnfreezeGrantDay    string // 上海日历日；当天已按最高档发过提现额度
	UnfreezeGrantedFen  int64  // 当天已发的档位封顶（1800/4000），不是剩余提现额度
	TokenVersion        int64
	CreatedAt           int64
	Locked              bool // 后台锁定后无法登录
	SkipUplineReward    bool // true=关闭上级分红（本用户产生的直推/管理奖不发）
	InviteRoot          bool // 用万能邀请码成为首个用户（无邀请人、不落左区）
}

const (
	TrackLeft  = "left"  // 左区（安置链，不是大区）
	TrackRight = "right" // 右区（安置链，不是小区）
)

type UserRepo interface {
	GetOrCreateByWallet(ctx context.Context, wallet string) (*User, bool, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByInviteCode(ctx context.Context, code string) (*User, error)
	ListInvitees(ctx context.Context, inviterID int64) ([]*User, error)
	HasEarlierNonAdmin(ctx context.Context, userID int64) (bool, error)
	HasInviteRoot(ctx context.Context, excludeID int64) (bool, error)
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
	if inviteCode != "" && user.InviterID == 0 && !user.InviteRoot {
		if err := uc.bindInvite(ctx, user, inviteCode); err != nil && created {
			return nil, "", err
		}
		user, err = uc.repo.GetByID(ctx, user.ID)
		if err != nil {
			return nil, "", err
		}
	}

	if user.Locked {
		return nil, "", ErrUserLocked
	}

	token, err := GenerateToken(user.ID, user.WalletAddress, user.TokenVersion)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (uc *UserUsecase) NeedInvite(ctx context.Context) (bool, error) {
	u, err := uc.SessionUser(ctx)
	if err != nil {
		return true, err
	}
	if u.InviterID > 0 || u.InviteRoot || IsAdmin(u) {
		return false, nil
	}
	return true, nil
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
	if u.Locked {
		return nil, ErrUserLocked
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

// EnsureInviteCode 邀请码就是钱包地址原样，不再生成 INV000001。
func (u *User) EnsureInviteCode() bool {
	if u == nil || u.WalletAddress == "" {
		return false
	}
	if u.InviteCode == u.WalletAddress {
		return false
	}
	u.InviteCode = u.WalletAddress
	return true
}

func LineUserIDs(users map[int64]*User, rootID int64) []int64 {
	if users == nil || users[rootID] == nil {
		return nil
	}
	seen := map[int64]struct{}{rootID: {}}
	q := []int64{rootID}
	var out []int64
	for len(q) > 0 {
		id := q[0]
		q = q[1:]
		out = append(out, id)
		for _, u := range users {
			if u == nil || u.InviterID != id {
				continue
			}
			if _, ok := seen[u.ID]; ok {
				continue
			}
			seen[u.ID] = struct{}{}
			q = append(q, u.ID)
		}
	}
	return out
}
