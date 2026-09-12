package biz

import (
	"context"

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

	DirectRewardPercent int64 = 10

	MaxRechargeFen int64 = 20_000_000 // 200,000.00 USTD
	MaxOrderQty    int64 = 99
)

func DirectRewardFen(amountFen int64) int64 {
	if amountFen <= 0 {
		return 0
	}
	return amountFen * DirectRewardPercent / 100
}

type LedgerEntry struct {
	ID         int64
	UserID     int64
	AmountFen  int64
	BalanceFen int64
	Type       string
	RefType    string
	RefID      int64
	CreatedAt  int64
}

type OrderItem struct {
	ProductID int64
	Name      string
	PriceFen  int64
	Quantity  int64
	AmountFen int64
}

type Order struct {
	ID        int64
	UserID    int64
	Status    string
	TotalFen  int64
	Items     []*OrderItem
	CreatedAt int64
}

type ShopRepo interface {
	Recharge(ctx context.Context, userID, amountFen int64) (*LedgerEntry, error)
	GetBalance(ctx context.Context, userID int64) (int64, error)
	ListLedger(ctx context.Context, userID int64, page, pageSize int32) ([]*LedgerEntry, int64, error)
	PlaceOrder(ctx context.Context, userID, productID, qty int64) (*Order, error)
	GetOrder(ctx context.Context, userID, orderID int64) (*Order, error)
	ListOrders(ctx context.Context, userID int64, page, pageSize int32) ([]*Order, int64, error)
	CancelOrder(ctx context.Context, userID, orderID int64) (*Order, error)
}

type WalletUsecase struct {
	shop  ShopRepo
	users *UserUsecase
	log   *log.Helper
}

func NewWalletUsecase(shop ShopRepo, users *UserUsecase, logger log.Logger) *WalletUsecase {
	return &WalletUsecase{shop: shop, users: users, log: log.NewHelper(logger)}
}

func (uc *WalletUsecase) GetWallet(ctx context.Context) (int64, error) {
	u, err := uc.users.SessionUser(ctx)
	if err != nil {
		return 0, err
	}
	return uc.shop.GetBalance(ctx, u.ID)
}

func (uc *WalletUsecase) Recharge(ctx context.Context, amountFen int64) (*LedgerEntry, int64, error) {
	u, err := uc.users.SessionUser(ctx)
	if err != nil {
		return nil, 0, err
	}
	if amountFen <= 0 || amountFen > MaxRechargeFen {
		return nil, 0, ErrInvalidArgument
	}
	e, err := uc.shop.Recharge(ctx, u.ID, amountFen)
	if err != nil {
		return nil, 0, err
	}
	return e, e.BalanceFen, nil
}

func (uc *WalletUsecase) ListLedger(ctx context.Context, page, pageSize int32) ([]*LedgerEntry, int64, error) {
	u, err := uc.users.SessionUser(ctx)
	if err != nil {
		return nil, 0, err
	}
	page, pageSize = normalizePage(page, pageSize)
	return uc.shop.ListLedger(ctx, u.ID, page, pageSize)
}

type OrderUsecase struct {
	shop  ShopRepo
	users *UserUsecase
	log   *log.Helper
}

func NewOrderUsecase(shop ShopRepo, users *UserUsecase, logger log.Logger) *OrderUsecase {
	return &OrderUsecase{shop: shop, users: users, log: log.NewHelper(logger)}
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
