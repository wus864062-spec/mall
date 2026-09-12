package data

import (
	"context"
	"sort"

	"mall/internal/biz"
)

type productRepo struct {
	data *Data
}

func NewProductRepo(data *Data) biz.ProductRepo {
	return &productRepo{data: data}
}

func (r *productRepo) Get(ctx context.Context, id int64) (*biz.Product, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	p, ok := r.data.products[id]
	if !ok {
		return nil, biz.ErrProductNotFound
	}
	return cloneProduct(p), nil
}

func (r *productRepo) List(ctx context.Context, page, pageSize int32, categoryID int64) ([]*biz.Product, int64, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	var all []*biz.Product
	for _, p := range r.data.products {
		if p.Status != biz.ProductOnSale {
			continue
		}
		if categoryID > 0 && p.CategoryID != categoryID {
			continue
		}
		all = append(all, cloneProduct(p))
	}
	sort.Slice(all, func(i, j int) bool { return all[i].PriceFen < all[j].PriceFen })
	out, total := slicePage(all, page, pageSize)
	return out, total, nil
}
