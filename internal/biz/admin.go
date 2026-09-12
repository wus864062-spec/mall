package biz

import (
	"context"
	"crypto/subtle"
	"os"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type AdminStats struct {
	UserCount          int64
	PaidOrderFen       int64
	DirectRewardFen    int64
	PairRewardFen      int64
	ManageRewardFen    int64
	LastPairSettleDate string
}

type AdminUserView struct {
	User       *User
	BalanceFen int64
	IsAdmin    bool
}

type AdminUserDetail struct {
	AdminUserView
	LeftPerfFen   int64
	RightPerfFen  int64
	AvailLeftFen  int64
	AvailRightFen int64
	PairVolFen    int64
	PairCapFen    int64
	EstPairPayFen int64
	LeftMembers   []*User
	RightMembers  []*User
}

type AdminRepo interface {
	ListUsers(ctx context.Context, page, pageSize int32, query string) ([]*AdminUserView, int64, error)
	GetUserDetail(ctx context.Context, id int64) (*AdminUserDetail, error)
	ListAllProducts(ctx context.Context) ([]*Product, error)
	UpdateProduct(ctx context.Context, id, priceFen, stock int64, status int32, setPrice, setStock bool) (*Product, error)
	ListAllLedger(ctx context.Context, page, pageSize int32, userID int64, typ string) ([]*LedgerEntry, int64, error)
	ListAllOrders(ctx context.Context, page, pageSize int32, query string) ([]*Order, []*User, int64, error)
	Stats(ctx context.Context) (*AdminStats, error)
	ForcePairSettle(ctx context.Context) (string, error)
}

type AdminUsecase struct {
	repo     AdminRepo
	users    *UserUsecase
	userRepo UserRepo
	log      *log.Helper
}

func NewAdminUsecase(repo AdminRepo, users *UserUsecase, userRepo UserRepo, logger log.Logger) *AdminUsecase {
	return &AdminUsecase{repo: repo, users: users, userRepo: userRepo, log: log.NewHelper(logger)}
}

func IsAdmin(u *User) bool {
	if u == nil {
		return false
	}
	if u.ID == 1 {
		return true
	}
	for _, w := range strings.Split(os.Getenv("ADMIN_WALLETS"), ",") {
		w = strings.TrimSpace(w)
		if w != "" && strings.EqualFold(w, u.WalletAddress) {
			return true
		}
	}
	return false
}

func (uc *AdminUsecase) RequireAdmin(ctx context.Context) (*User, error) {
	u, err := uc.users.SessionUser(ctx)
	if err != nil {
		return nil, err
	}
	if !IsAdmin(u) {
		return nil, ErrForbidden
	}
	return u, nil
}

func (uc *AdminUsecase) PasswordLogin(ctx context.Context, username, password string) (*User, string, error) {
	wantUser := os.Getenv("ADMIN_USER")
	if wantUser == "" {
		wantUser = "admin"
	}
	wantPass := os.Getenv("ADMIN_PASSWORD")
	if wantPass == "" {
		wantPass = "admin"
	}
	username = strings.TrimSpace(username)
	if subtle.ConstantTimeCompare([]byte(username), []byte(wantUser)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(wantPass)) != 1 {
		return nil, "", ErrUnauthorized
	}
	u, err := uc.users.GetByID(ctx, 1)
	if err != nil {
		u, _, err = uc.userRepo.GetOrCreateByWallet(ctx, "admin")
		if err != nil {
			return nil, "", err
		}
	}
	if !IsAdmin(u) {
		return nil, "", ErrForbidden
	}
	token, err := GenerateToken(u.ID, u.WalletAddress, u.TokenVersion)
	if err != nil {
		return nil, "", err
	}
	return u, token, nil
}

func (uc *AdminUsecase) GetStats(ctx context.Context) (*AdminStats, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	return uc.repo.Stats(ctx)
}

func (uc *AdminUsecase) ListUsers(ctx context.Context, page, pageSize int32, query string) ([]*AdminUserView, int64, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return uc.repo.ListUsers(ctx, page, pageSize, query)
}

func (uc *AdminUsecase) GetUser(ctx context.Context, id int64) (*AdminUserDetail, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, ErrInvalidArgument
	}
	return uc.repo.GetUserDetail(ctx, id)
}

func (uc *AdminUsecase) ListProducts(ctx context.Context) ([]*Product, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	return uc.repo.ListAllProducts(ctx)
}

func (uc *AdminUsecase) UpdateProduct(ctx context.Context, id, priceFen, stock int64, status int32, setPrice, setStock bool) (*Product, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, ErrInvalidArgument
	}
	return uc.repo.UpdateProduct(ctx, id, priceFen, stock, status, setPrice, setStock)
}

func (uc *AdminUsecase) ListLedger(ctx context.Context, page, pageSize int32, userID int64, typ string) ([]*LedgerEntry, int64, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return uc.repo.ListAllLedger(ctx, page, pageSize, userID, typ)
}

func (uc *AdminUsecase) ListOrders(ctx context.Context, page, pageSize int32, query string) ([]*Order, []*User, int64, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return uc.repo.ListAllOrders(ctx, page, pageSize, query)
}

func (uc *AdminUsecase) ForcePairSettle(ctx context.Context) (string, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return "", err
	}
	return uc.repo.ForcePairSettle(ctx)
}

func FormatUnix(sec int64) string {
	if sec <= 0 {
		return ""
	}
	return time.Unix(sec, 0).In(time.Local).Format("2006-01-02 15:04:05")
}
