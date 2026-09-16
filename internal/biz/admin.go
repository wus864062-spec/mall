package biz

import (
	"context"
	"crypto/subtle"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type AdminStats struct {
	UserCount                   int64 // 注册人数
	TodayUserCount              int64 // 今日注册人数
	ActivatedCount              int64 // 激活总人数=有过认购
	TodayActivatedCount         int64 // 今日激活=今日首次认购
	RechargeFen                 int64 // 总充值USDT=链上充值（有 tx），不含后台加
	TodayRechargeFen            int64 // 今日充值USDT=当日链上充值
	AdminAdjustFen              int64 // 后台加USDT=无哈希充值 + 正数 AdminAdjust
	PaidOrderCount              int64 // 购买订单总数=已付认购笔数，不是金额
	TodayPaidOrderCount         int64 // 今日购买订单数量
	PaidOrderFen                int64
	TodayPaidOrderFen           int64
	BalanceUsdtFen              int64 // 全网可提USDT=balances，不含冻结
	TodayStaticFen              int64 // 今日USDT静态=static_reward，原币
	TodayStaticIspayMicro       int64 // 今日IsPay静态=static_reward_ispay，原币
	TodayStaticTotalFen         int64 // 今日总USDT静态=U+币转U，行情价
	TodayStaticTotalIspayMicro  int64 // 今日总IsPay静态=币+U转币，行情价
	StaticFen                   int64 // 总USDT静态收益=static_reward 原币，含往日
	StaticIspayMicro            int64 // 总IsPay静态收益=static_reward_ispay 原币，含往日
	StaticTotalFen              int64 // 总U+币静态=U+币转U；总静态收益同这个数
	StaticTotalIspayMicro       int64 // 总币+U静态=币+U转币
	TodayDynamicFen             int64 // 今日USDT动态=直推/对碰/管理 USDT 原币，含回退
	TodayDynamicIspayMicro      int64 // 今日IsPay动态=直推/对碰/管理 IsPay 原币，含回退
	TodayDynamicTotalFen        int64 // 今日总USDT动态=U+币转U，行情价
	TodayDynamicTotalIspayMicro int64 // 今日总IsPay动态=币+U转币，行情价
	DynamicFen                  int64 // 总USDT动态收益=直推/对碰/管理 USDT 原币，含往日含回退
	DynamicIspayMicro           int64 // 总IsPay动态收益=直推/对碰/管理 IsPay 原币，含往日含回退
	DynamicTotalFen             int64 // 总U+币动态=U+币转U；总动态收益同这个数
	DynamicTotalIspayMicro      int64 // 总币+U动态=币+U转币
	Direct                      RewardSplit
	Pair                        RewardSplit
	Manage                      RewardSplit
	TodayWithdrawFen            int64 // 今日提现USDT
	TotalWithdrawFen            int64 // 总提USDT
	TodayWithdrawIspayMicro     int64 // 今日IsPay提现
	TotalWithdrawIspayMicro     int64 // 总IsPay提现
	TodayRewardFen              int64
	TotalRewardFen              int64
	TotalIspayMicro             int64
	DirectRewardFen             int64
	PairRewardFen               int64
	ManageRewardFen             int64
	LastPairSettleDate          string
	IspayPriceFen               int64 // 行情价，总静态折算用这个
}

type AdminUserView struct {
	User                  *User
	BalanceFen            int64
	FrozenUsdtFen         int64
	IspayLockedMicro      int64
	IspayFreeMicro        int64
	FrozenIspayMicro      int64
	IsAdmin               bool
	LeftPerfFen           int64
	RightPerfFen          int64
	InviteeCount          int64
	InviterAddress        string
	PackageFen            int64
	PaidSumFen            int64
	RechargeFen           int64 // 充值余额=充值−认购−提现后的剩余，不是累计充值、不是可提 USDT
	StaticDays            int64
	StaticReleasedDays    int64
	StaticReleasedUsdtFen int64
	StaticRemainUsdtFen   int64
	StaticDailyUsdtFen    int64
	StaticFinished        bool
	LastOrderAt           int64
	SettledPairFen        int64
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

type AdminSearchHit struct {
	*AdminUserDetail
	Orders []*Order
	Ledger []*LedgerEntry
}

type AdminSearchResult struct {
	Query string
	Hits  []*AdminSearchHit
}

const SearchTimeout = 15 * time.Second
const SearchMaxHits = 20

func UserMatchesQuery(u *User, query string) bool {
	if u == nil {
		return false
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	if strconv.FormatInt(u.ID, 10) == q {
		return true
	}
	if strings.Contains(strings.ToLower(u.WalletAddress), q) {
		return true
	}
	if strings.Contains(strings.ToLower(u.InviteCode), q) {
		return true
	}
	if strings.Contains(strings.ToLower(u.Nickname), q) {
		return true
	}
	return false
}

type AdminRepo interface {
	ListUsers(ctx context.Context, page, pageSize int32, query string) ([]*AdminUserView, int64, error)
	GetUserDetail(ctx context.Context, id int64) (*AdminUserDetail, error)
	ListAllProducts(ctx context.Context) ([]*Product, error)
	UpdateProduct(ctx context.Context, id, priceFen, stock int64, status int32, setPrice, setStock bool) (*Product, error)
	CreateProduct(ctx context.Context, p *Product) (*Product, error)
	UpdateProductMeta(ctx context.Context, p *Product, setName, setDesc, setPrice, setStock, setStatus, setCategory bool) (*Product, error)
	ListAllLedger(ctx context.Context, page, pageSize int32, userID int64, typ string) ([]*LedgerEntry, int64, error)
	ListAllLedgerMerged(ctx context.Context, page, pageSize int32, userID int64, typ, unit string, priceFen int64) ([]*LedgerEntry, int64, error)
	FindUserIDByWallet(ctx context.Context, wallet string) int64
	SumRewardFen(ctx context.Context, userID int64) (staticFen, dynamicFen int64)
	ListAllOrders(ctx context.Context, page, pageSize int32, query string, userID int64) ([]*Order, []*User, int64, error)
	Search(ctx context.Context, query string) (*AdminSearchResult, error)
	Stats(ctx context.Context) (*AdminStats, error)
	ForcePairSettle(ctx context.Context) (string, error)
	ClearTestData(ctx context.Context) error
	SetUserLocked(ctx context.Context, userID int64, locked, line bool) (int, error)
	SetSkipUplineReward(ctx context.Context, userID int64, skip bool) error
	SetUserUSDT(ctx context.Context, userID, amountFen int64) error
	AddUserUSDT(ctx context.Context, userID, amountFen int64) error
	SetUserIspayFree(ctx context.Context, userID, micro int64) error
	AdminPlaceOrder(ctx context.Context, userID, productID int64) (*Order, error)
	ChangeUserWallet(ctx context.Context, userID int64, wallet string) error
	FindWeb3ProductByPrice(ctx context.Context, priceFen int64) (*Product, error)
	ListPackages(ctx context.Context, days int64) ([]*Package, error)
	CreatePackage(ctx context.Context, p *Package) (*Package, error)
	UpdatePackage(ctx context.Context, p *Package, setImage bool) (*Package, error)
	SetPackageStatus(ctx context.Context, id, days int64, status int32) (*Package, error)
	UploadsDir() string
	GetIspayPriceFen(ctx context.Context) (int64, error)
	SetIspayPriceFen(ctx context.Context, priceFen int64) error
	GetSettings(ctx context.Context) (SiteSettings, error)
	SetSettings(ctx context.Context, s SiteSettings) error
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
	if u.ID == 1 || strings.EqualFold(u.WalletAddress, "admin") {
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

func (uc *AdminUsecase) ListInvitees(ctx context.Context, id int64) ([]*User, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, ErrInvalidArgument
	}
	return uc.userRepo.ListInvitees(ctx, id)
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

func (uc *AdminUsecase) CreateProduct(ctx context.Context, p *Product) (*Product, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if p == nil || strings.TrimSpace(p.Name) == "" || p.PriceFen <= 0 {
		return nil, ErrInvalidArgument
	}
	if p.Stock < 0 {
		return nil, ErrInvalidArgument
	}
	if p.Stock == 0 {
		p.Stock = 1_000_000
	}
	if p.Status == 0 {
		p.Status = ProductOnSale
	}
	return uc.repo.CreateProduct(ctx, p)
}

func (uc *AdminUsecase) UpdateProductMeta(ctx context.Context, p *Product, setName, setDesc, setPrice, setStock, setStatus, setCategory bool) (*Product, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if p == nil || p.ID <= 0 {
		return nil, ErrInvalidArgument
	}
	return uc.repo.UpdateProductMeta(ctx, p, setName, setDesc, setPrice, setStock, setStatus, setCategory)
}

func (uc *AdminUsecase) ListPackages(ctx context.Context, days int64) ([]*Package, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if days != 0 && !ValidReleaseDays(days) {
		return nil, ErrReleaseDays
	}
	return uc.repo.ListPackages(ctx, days)
}

// CreatePackage 金额只用请求里的 amount_fen（认购标价，fen）。不用 EffectivePackageFen，也不折 IsPay。
func (uc *AdminUsecase) CreatePackage(ctx context.Context, p *Package) (*Package, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrInvalidArgument
	}
	if !ValidReleaseDays(p.ReleaseDays) {
		return nil, ErrReleaseDays
	}
	p.Name = strings.TrimSpace(p.Name)
	p.Description = strings.TrimSpace(p.Description)
	if p.Name == "" {
		return nil, ErrPackageName
	}
	if p.Description == "" {
		return nil, ErrPackageDesc
	}
	if p.AmountFen <= 0 {
		return nil, ErrPackageAmount
	}
	if p.Status != 0 {
		p.Status = ProductOnSale
	}
	return uc.repo.CreatePackage(ctx, p)
}

func (uc *AdminUsecase) UpdatePackage(ctx context.Context, p *Package, setImage bool) (*Package, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if p == nil || p.ID <= 0 {
		return nil, ErrProductNotFound
	}
	if !ValidReleaseDays(p.ReleaseDays) {
		return nil, ErrReleaseDays
	}
	p.Name = strings.TrimSpace(p.Name)
	p.Description = strings.TrimSpace(p.Description)
	if p.Name == "" {
		return nil, ErrPackageName
	}
	if p.Description == "" {
		return nil, ErrPackageDesc
	}
	if p.AmountFen <= 0 {
		return nil, ErrPackageAmount
	}
	if p.Status != 0 {
		p.Status = ProductOnSale
	}
	return uc.repo.UpdatePackage(ctx, p, setImage)
}

func (uc *AdminUsecase) SetPackageStatus(ctx context.Context, id, days int64, status int32) (*Package, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, ErrProductNotFound
	}
	if !ValidReleaseDays(days) {
		return nil, ErrReleaseDays
	}
	if status != 0 {
		status = ProductOnSale
	}
	return uc.repo.SetPackageStatus(ctx, id, days, status)
}

func (uc *AdminUsecase) UploadsDir() string {
	return uc.repo.UploadsDir()
}

func (uc *AdminUsecase) ListLedger(ctx context.Context, page, pageSize int32, userID int64, typ, wallet, unit string) ([]*LedgerEntry, int64, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	wallet = strings.TrimSpace(wallet)
	if wallet != "" {
		id := uc.repo.FindUserIDByWallet(ctx, wallet)
		if id == 0 {
			return nil, 0, nil
		}
		userID = id
	}
	if ShouldMergeSplitLedger(typ, unit) {
		priceFen, err := uc.GetIspayPriceFen(ctx)
		if err != nil {
			return nil, 0, err
		}
		return uc.repo.ListAllLedgerMerged(ctx, page, pageSize, userID, typ, unit, priceFen)
	}
	return uc.repo.ListAllLedger(ctx, page, pageSize, userID, typ)
}

func (uc *AdminUsecase) RewardTotals(ctx context.Context, wallet string) (staticFen, dynamicFen int64, err error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return 0, 0, err
	}
	wallet = strings.TrimSpace(wallet)
	var userID int64
	if wallet != "" {
		userID = uc.repo.FindUserIDByWallet(ctx, wallet)
		if userID == 0 {
			return 0, 0, nil
		}
	}
	staticFen, dynamicFen = uc.repo.SumRewardFen(ctx, userID)
	return staticFen, dynamicFen, nil
}

func (uc *AdminUsecase) ListOrders(ctx context.Context, page, pageSize int32, query string, userID int64) ([]*Order, []*User, int64, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return uc.repo.ListAllOrders(ctx, page, pageSize, query, userID)
}

func (uc *AdminUsecase) Search(ctx context.Context, query string) (*AdminSearchResult, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrInvalidArgument
	}
	ctx, cancel := context.WithTimeout(ctx, SearchTimeout)
	defer cancel()
	return uc.repo.Search(ctx, query)
}

func (uc *AdminUsecase) ForcePairSettle(ctx context.Context) (string, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return "", err
	}
	return uc.repo.ForcePairSettle(ctx)
}

func (uc *AdminUsecase) ClearTestData(ctx context.Context) error {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return err
	}
	return uc.repo.ClearTestData(ctx)
}

type AdminUserAction struct {
	Action    string
	Locked    bool
	Skip      bool
	AmountFen int64
	Micro     int64
	ProductID int64
	Wallet    string
}

func (uc *AdminUsecase) ApplyUserAction(ctx context.Context, userID int64, in AdminUserAction) (string, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return "", err
	}
	if userID <= 0 {
		return "", ErrInvalidArgument
	}
	switch strings.ToLower(strings.TrimSpace(in.Action)) {
	case "lock":
		n, err := uc.repo.SetUserLocked(ctx, userID, in.Locked, false)
		if err != nil {
			return "", err
		}
		if in.Locked {
			return fmt.Sprintf("已锁定 %d 个账户", n), nil
		}
		return fmt.Sprintf("已解锁 %d 个账户", n), nil
	case "lock_line":
		n, err := uc.repo.SetUserLocked(ctx, userID, in.Locked, true)
		if err != nil {
			return "", err
		}
		if in.Locked {
			return fmt.Sprintf("已锁定一条线 %d 人", n), nil
		}
		return fmt.Sprintf("已解锁一条线 %d 人", n), nil
	case "skip_upline":
		if err := uc.repo.SetSkipUplineReward(ctx, userID, in.Skip); err != nil {
			return "", err
		}
		if in.Skip {
			return "已关闭上级分红", nil
		}
		return "已开启上级分红", nil
	case "set_usdt":
		if in.AmountFen < 0 {
			return "", ErrInvalidArgument
		}
		if err := uc.repo.SetUserUSDT(ctx, userID, in.AmountFen); err != nil {
			return "", err
		}
		return "可提 USDT 已设置", nil
	case "add_usdt":
		if in.AmountFen <= 0 {
			return "", ErrInvalidArgument
		}
		if err := uc.repo.AddUserUSDT(ctx, userID, in.AmountFen); err != nil {
			return "", err
		}
		return "充值 USDT 已入账", nil
	case "set_ispay":
		if in.Micro < 0 {
			return "", ErrInvalidArgument
		}
		if err := uc.repo.SetUserIspayFree(ctx, userID, in.Micro); err != nil {
			return "", err
		}
		return "可提 IsPay 已设置", nil
	case "order":
		pid := in.ProductID
		if pid <= 0 && in.AmountFen > 0 {
			p, err := uc.repo.FindWeb3ProductByPrice(ctx, in.AmountFen)
			if err != nil {
				return "", err
			}
			pid = p.ID
		}
		if pid <= 0 {
			return "", ErrInvalidArgument
		}
		o, err := uc.repo.AdminPlaceOrder(ctx, userID, pid)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("认购完成 #%d", o.ID), nil
	case "address":
		if err := uc.repo.ChangeUserWallet(ctx, userID, in.Wallet); err != nil {
			return "", err
		}
		return "钱包地址已修改", nil
	default:
		return "", ErrInvalidArgument
	}
}

func (uc *AdminUsecase) GetIspayPriceFen(ctx context.Context) (int64, error) {
	s, err := uc.GetSettings(ctx)
	if err != nil {
		return 0, err
	}
	return s.IspayPriceFen, nil
}

func (uc *AdminUsecase) SetIspayPriceFen(ctx context.Context, priceFen int64) error {
	s, err := uc.GetSettings(ctx)
	if err != nil {
		return err
	}
	s.IspayPriceFen = priceFen
	return uc.SetSettings(ctx, s)
}

func (uc *AdminUsecase) GetSettings(ctx context.Context) (SiteSettings, error) {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return SiteSettings{}, err
	}
	return uc.repo.GetSettings(ctx)
}

func (uc *AdminUsecase) SetSettings(ctx context.Context, s SiteSettings) error {
	if _, err := uc.RequireAdmin(ctx); err != nil {
		return err
	}
	s = s.Normalize()
	if s.IspayPriceFen <= 0 || s.MaxRechargeFen <= 0 || s.MinWithdrawFen <= 0 {
		return ErrInvalidArgument
	}
	if s.WithdrawFeePercent < 0 || s.WithdrawFeePercent > 100 {
		return ErrInvalidArgument
	}
	return uc.repo.SetSettings(ctx, s)
}

func FormatUnix(sec int64) string {
	if sec <= 0 {
		return ""
	}
	return time.Unix(sec, 0).In(time.Local).Format("2006-01-02 15:04:05")
}
