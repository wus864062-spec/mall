package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	ProductOnSale    int32 = 1
	CategoryShare    int64 = 1
	CategoryWeb3     int64 = 3 // 300 天
	CategoryWeb3_600 int64 = 4 // 600 天
	CategoryWeb3_750 int64 = 5 // 750 天
)

func Web3Categories() []int64 {
	return []int64{CategoryWeb3, CategoryWeb3_600, CategoryWeb3_750}
}

func IsWeb3Category(id int64) bool {
	return id == CategoryWeb3 || id == CategoryWeb3_600 || id == CategoryWeb3_750
}

type Product struct {
	ID          int64
	Name        string
	Description string
	PriceFen    int64
	Stock       int64
	Image       string
	CategoryID  int64
	Status      int32
}

type ProductRepo interface {
	Get(ctx context.Context, id int64) (*Product, error)
	List(ctx context.Context, page, pageSize int32, categoryID int64) ([]*Product, int64, error)
}

type ProductUsecase struct {
	repo ProductRepo
	log  *log.Helper
}

func NewProductUsecase(repo ProductRepo, logger log.Logger) *ProductUsecase {
	return &ProductUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *ProductUsecase) Get(ctx context.Context, id int64) (*Product, error) {
	if id <= 0 {
		return nil, ErrInvalidArgument
	}
	return uc.repo.Get(ctx, id)
}

func (uc *ProductUsecase) List(ctx context.Context, page, pageSize int32, categoryID int64) ([]*Product, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return uc.repo.List(ctx, page, pageSize, categoryID)
}
