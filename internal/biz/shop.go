package biz

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	OrderPaid      = "paid"
	OrderCancelled = "cancelled"

	LedgerRecharge             = "recharge"
	LedgerOrderPay             = "order_pay"
	LedgerOrderRefund          = "order_refund"
	LedgerDirectReward         = "direct_reward"
	LedgerDirectRewardClawback = "direct_reward_clawback"
	LedgerPairReward           = "pair_reward"
	LedgerPairRewardClawback   = "pair_reward_clawback"
	LedgerPairUpline           = "pair_upline"
	LedgerManageReward         = "manage_reward"
	LedgerWithdraw             = "withdraw"
	LedgerAdminAdjust          = "admin_adjust"
	LedgerAdminIspay           = "admin_ispay"

	DirectRewardPercent int64 = 10
	WithdrawFeePercent  int64 = 10
	MinWithdrawFen      int64 = 10 * UstdScale      // 10 USDT
	MaxRechargeFen      int64 = 200_000 * UstdScale // 200,000 USDT
	MaxOrderQty         int64 = 99
)

// RewardUSDTKind 分红汇总只用 USDT 流水：静态 / 动态。IsPay 行不算进这三项。
func RewardUSDTKind(typ string) string {
	switch typ {
	case LedgerStaticReward:
		return "static"
	case LedgerDirectReward, LedgerDirectRewardClawback, LedgerPairReward, LedgerPairRewardClawback, LedgerPairUpline, LedgerManageReward:
		return "dynamic"
	default:
		return ""
	}
}

// RechargeRemainFen 充值页/后台「充值」用这个数：只计充值进来的 USDT，再扣认购实付和提现，退款加回。
// 用 LedgerRecharge / OrderPay / OrderRefund / Withdraw，不用余额、奖励、调账、解冻。
func RechargeRemainFen(entries []*LedgerEntry, userID int64) int64 {
	var n int64
	for _, e := range entries {
		if e == nil || e.UserID != userID {
			continue
		}
		switch e.Type {
		case LedgerRecharge, LedgerOrderPay, LedgerOrderRefund, LedgerWithdraw:
			n += e.AmountFen
		}
	}
	if n < 0 {
		return 0
	}
	return n
}

func WithdrawFeeFen(amountFen int64) int64 {
	if amountFen <= 0 {
		return 0
	}
	return amountFen * WithdrawFeePercent / 100
}

func DirectRewardFen(amountFen int64) int64 {
	if amountFen <= 0 {
		return 0
	}
	return amountFen * DirectRewardPercent / 100
}

type LedgerEntry struct {
	ID            int64
	UserID        int64
	AmountFen     int64
	BalanceFen    int64
	Type          string
	RefType       string
	RefID         int64
	CreatedAt     int64
	TxHash        string
	WalletAddress string `json:"-"`                     // 列表回包用，不入库
	DisplayUnit   string `json:"displayUnit,omitempty"` // 三项合计合成后的展示币种 USDT/IsPay
}

type OrderItem struct {
	ProductID int64
	Name      string
	PriceFen  int64
	Quantity  int64
	AmountFen int64
}

type Order struct {
	ID           int64
	UserID       int64
	Status       string
	TotalFen     int64
	Items        []*OrderItem
	CreatedAt    int64
	CategoryID   int64
	CoinsMicro   int64
	ReleaseDays  int64
	ReleasedDays int64
}

type ShopRepo interface {
	Recharge(ctx context.Context, userID, amountFen int64, txHash string) (*LedgerEntry, error)
	Withdraw(ctx context.Context, userID, amountFen int64) (*LedgerEntry, error)
	WithdrawAsset(ctx context.Context, userID, amount int64, asset, txHash string) (*LedgerEntry, error)
	GetBalance(ctx context.Context, userID int64) (int64, error)
	GetWalletView(ctx context.Context, userID int64) (*WalletView, error)
	ListLedger(ctx context.Context, userID int64, page, pageSize int32, typ string) ([]*LedgerEntry, int64, error)
	PlaceOrder(ctx context.Context, userID, productID, qty int64) (*Order, error)
	PlaceCart(ctx context.Context, userID int64, productIDs []int64) (*Order, error)
	GetOrder(ctx context.Context, userID, orderID int64) (*Order, error)
	ListOrders(ctx context.Context, userID int64, page, pageSize int32) ([]*Order, int64, error)
	CancelOrder(ctx context.Context, userID, orderID int64) (*Order, error)
	GetSettings(ctx context.Context) SiteSettings
	GetChainConfig(ctx context.Context) *ChainConfig
}

type WalletUsecase struct {
	shop       ShopRepo
	users      *UserUsecase
	log        *log.Helper
	chain      ChainVerifier
	payout     HotWalletPayer
	withdrawMu sync.Mutex
}

func NewWalletUsecase(shop ShopRepo, users *UserUsecase, logger log.Logger) *WalletUsecase {
	payout := DefaultHotWalletPayer()
	uc := &WalletUsecase{
		shop:   shop,
		users:  users,
		log:    log.NewHelper(logger),
		chain:  DefaultChainVerifier(),
		payout: payout,
	}
	if a := HotWalletAddress(payout); a != "" {
		uc.log.Infof("USDT withdraw hot wallet %s", a)
	}
	return uc
}

func (uc *WalletUsecase) SetChainVerifier(v ChainVerifier) {
	uc.chain = v
}

func (uc *WalletUsecase) SetHotWallet(p HotWalletPayer) {
	uc.payout = p
}

func (uc *WalletUsecase) buyAddress(ctx context.Context) string {
	if cfg := uc.shop.GetChainConfig(ctx); cfg != nil {
		if a := strings.TrimSpace(cfg.BuyAddress); a != "" {
			return NormalizeEthAddress(a)
		}
	}
	if a := strings.TrimSpace(os.Getenv("MALL_BUY_ADDRESS")); a != "" {
		return NormalizeEthAddress(a)
	}
	return ""
}

func (uc *WalletUsecase) GetWallet(ctx context.Context) (int64, error) {
	view, err := uc.GetWalletView(ctx)
	if err != nil {
		return 0, err
	}
	return view.BalanceFen, nil
}

func (uc *WalletUsecase) GetWalletView(ctx context.Context) (*WalletView, error) {
	u, err := uc.users.SessionUser(ctx)
	if err != nil {
		return nil, err
	}
	return uc.shop.GetWalletView(ctx, u.ID)
}

func (uc *WalletUsecase) ShopSettings(ctx context.Context) SiteSettings {
	return uc.shop.GetSettings(ctx)
}

func (uc *WalletUsecase) Recharge(ctx context.Context, amountFen int64, txHash string) (*LedgerEntry, int64, error) {
	u, err := uc.users.SessionUser(ctx)
	if err != nil {
		return nil, 0, err
	}
	s := uc.shop.GetSettings(ctx)
	if amountFen <= 0 {
		return nil, 0, ErrInvalidArgument
	}
	// 入账用整 U（丢掉不足 1U 的 fen）。充多少记多少，不按收款口拆款比例打折。
	amountFen = amountFen / UstdScale * UstdScale
	if amountFen <= 0 || amountFen > s.MaxRechargeFen {
		return nil, 0, ErrInvalidArgument
	}
	hash := NormalizeTxHash(txHash)
	if hash == "" {
		return nil, 0, ErrRechargeTxRequired
	}
	if uc.chain == nil {
		uc.chain = DefaultChainVerifier()
	}
	rec, err := uc.chain.VerifyRechargeTx(ctx, hash, uc.buyAddress(ctx))
	if err != nil {
		return nil, 0, err
	}
	if rec == nil {
		return nil, 0, ErrRechargeTxInvalid
	}
	if !strings.EqualFold(NormalizeEthAddress(rec.Payer), NormalizeEthAddress(u.WalletAddress)) {
		return nil, 0, ErrRechargePayer
	}
	// 以链上 num 为准：num*10000 必须等于截断后的 amount_fen，对不上拒绝，不改成按链上改账。
	if rec.Num <= 0 || rec.Num > s.MaxRechargeFen/UstdScale || rec.Num*UstdScale != amountFen {
		return nil, 0, ErrRechargeAmount
	}
	e, err := uc.shop.Recharge(ctx, u.ID, amountFen, hash)
	if err != nil {
		return nil, 0, err
	}
	return e, e.BalanceFen, nil
}

func (uc *WalletUsecase) Withdraw(ctx context.Context, amountFen int64) (*LedgerEntry, int64, error) {
	return uc.WithdrawAsset(ctx, amountFen, AssetUSDT)
}

func (uc *WalletUsecase) WithdrawAsset(ctx context.Context, amount int64, asset string) (*LedgerEntry, int64, error) {
	u, err := uc.users.SessionUser(ctx)
	if err != nil {
		return nil, 0, err
	}
	if asset == "" || asset == AssetUSDT {
		s := uc.shop.GetSettings(ctx)
		if amount < s.MinWithdrawFen {
			return nil, 0, ErrWithdrawTooSmall
		}
		if amount > s.MaxRechargeFen {
			return nil, 0, ErrInvalidArgument
		}
		net := WithdrawNetFen(amount, s.WithdrawFeePercent)
		if net <= 0 {
			return nil, 0, ErrInvalidArgument
		}
		uc.withdrawMu.Lock()
		defer uc.withdrawMu.Unlock()
		bal, err := uc.shop.GetBalance(ctx, u.ID)
		if err != nil {
			return nil, 0, err
		}
		if bal < amount {
			return nil, 0, ErrInsufficientBalance
		}
		payout := uc.payout
		if payout == nil {
			payout = disabledHotWallet{}
		}
		// 链上只打净额（申请额−手续费），账上扣申请额。打失败不扣账。
		hash, err := payout.SendUSDT(ctx, u.WalletAddress, net)
		if err != nil {
			return nil, 0, err
		}
		e, err := uc.shop.WithdrawAsset(ctx, u.ID, amount, AssetUSDT, hash)
		if err != nil {
			return nil, 0, err
		}
		return e, e.BalanceFen, nil
	}
	if asset != AssetIspay {
		return nil, 0, ErrInvalidArgument
	}
	if amount < MinWithdrawIspayMicro {
		return nil, 0, ErrWithdrawTooSmall
	}
	e, err := uc.shop.WithdrawAsset(ctx, u.ID, amount, AssetIspay, "")
	if err != nil {
		return nil, 0, err
	}
	return e, e.BalanceFen, nil
}

func (uc *WalletUsecase) ListLedger(ctx context.Context, page, pageSize int32, typ string) ([]*LedgerEntry, int64, error) {
	u, err := uc.users.SessionUser(ctx)
	if err != nil {
		return nil, 0, err
	}
	page, pageSize = normalizePage(page, pageSize)
	return uc.shop.ListLedger(ctx, u.ID, page, pageSize, typ)
}

type OrderUsecase struct {
	shop  ShopRepo
	users *UserUsecase
	log   *log.Helper
}

func NewOrderUsecase(shop ShopRepo, users *UserUsecase, logger log.Logger) *OrderUsecase {
	return &OrderUsecase{shop: shop, users: users, log: log.NewHelper(logger)}
}

func (uc *OrderUsecase) ShopSettings(ctx context.Context) SiteSettings {
	return uc.shop.GetSettings(ctx)
}

func (uc *OrderUsecase) Create(ctx context.Context, productID, qty int64) (*Order, error) {
	u, err := uc.users.SessionUser(ctx)
	if err != nil {
		return nil, err
	}
	if productID <= 0 || qty <= 0 || qty > MaxOrderQty {
		return nil, ErrInvalidArgument
	}
	return uc.shop.PlaceOrder(ctx, u.ID, productID, qty)
}

func (uc *OrderUsecase) CreateCart(ctx context.Context, productIDs []int64) (*Order, error) {
	u, err := uc.users.SessionUser(ctx)
	if err != nil {
		return nil, err
	}
	if len(productIDs) == 0 {
		return nil, ErrCartEmpty
	}
	if len(productIDs) > 20 {
		return nil, ErrInvalidArgument
	}
	return uc.shop.PlaceCart(ctx, u.ID, productIDs)
}

func (uc *OrderUsecase) Get(ctx context.Context, id int64) (*Order, error) {
	u, err := uc.users.SessionUser(ctx)
	if err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, ErrInvalidArgument
	}
	return uc.shop.GetOrder(ctx, u.ID, id)
}

func (uc *OrderUsecase) List(ctx context.Context, page, pageSize int32) ([]*Order, int64, error) {
	u, err := uc.users.SessionUser(ctx)
	if err != nil {
		return nil, 0, err
	}
	page, pageSize = normalizePage(page, pageSize)
	return uc.shop.ListOrders(ctx, u.ID, page, pageSize)
}

func (uc *OrderUsecase) Cancel(ctx context.Context, id int64) (*Order, error) {
	if _, err := uc.users.SessionUser(ctx); err != nil {
		return nil, err
	}
	return nil, ErrOrderNotCancellable
}

func normalizePage(page, pageSize int32) (int32, int32) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
