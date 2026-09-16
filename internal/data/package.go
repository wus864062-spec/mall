package data

import (
	"context"
	"path/filepath"
	"sort"

	"mall/internal/biz"
)

func (d *Data) UploadsDir() string {
	if d == nil || d.path == "" {
		return "data/uploads"
	}
	return filepath.Join(filepath.Dir(d.path), "uploads")
}

func (d *Data) seedPackagesLocked() bool {
	if d.packages == nil {
		d.packages = map[int64]*biz.Package{}
	}
	if len(d.packages) > 0 {
		return false
	}
	var maxID int64
	for _, p := range d.products {
		if p == nil || !biz.IsWeb3Category(p.CategoryID) {
			continue
		}
		days := biz.ReleaseDaysByCategory(p.CategoryID)
		if days == 0 {
			continue
		}
		maxID++
		st := p.Status
		if st != 0 && st != biz.ProductOnSale {
			st = 0
		}
		d.packages[maxID] = &biz.Package{
			ID:          maxID,
			Name:        p.Name,
			Description: p.Description,
			AmountFen:   p.PriceFen,
			ReleaseDays: days,
			Image:       p.Image,
			Status:      st,
			ProductID:   p.ID,
		}
	}
	d.packageSeq = maxID
	return maxID > 0
}

func (d *Data) listPackagesLocked(days int64) []*biz.Package {
	out := make([]*biz.Package, 0, len(d.packages))
	for _, p := range d.packages {
		if p == nil {
			continue
		}
		if days > 0 && p.ReleaseDays != days {
			continue
		}
		out = append(out, biz.ClonePackage(p))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		if out[i].AmountFen != out[j].AmountFen {
			return out[i].AmountFen < out[j].AmountFen
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (d *Data) packageAmountTakenLocked(days, amountFen, exceptID int64) bool {
	for _, p := range d.packages {
		if p == nil || p.ID == exceptID {
			continue
		}
		if p.ReleaseDays == days && p.AmountFen == amountFen {
			return true
		}
	}
	return false
}

// syncPackageProductLocked 把套餐目录写回 products：用 amount_fen 当认购标价（fen，截断后的 USDT）。
// 不用 EffectivePackageFen，也不把该数交给 CoinsMicroFromPay；用户锁仓仍按实付总额。
func (d *Data) syncPackageProductLocked(pkg *biz.Package) {
	if pkg == nil {
		return
	}
	cat, ok := biz.CategoryByReleaseDays(pkg.ReleaseDays)
	if !ok {
		return
	}
	var p *biz.Product
	if pkg.ProductID > 0 {
		p = d.products[pkg.ProductID]
	}
	if p == nil {
		for _, x := range d.products {
			if x == nil {
				continue
			}
			if x.CategoryID == cat && x.PriceFen == pkg.AmountFen {
				p = x
				break
			}
		}
	}
	if p == nil {
		var maxID int64
		for id := range d.products {
			if id > maxID {
				maxID = id
			}
		}
		p = &biz.Product{ID: maxID + 1, Stock: 1_000_000}
		d.products[p.ID] = p
	}
	p.Name = pkg.Name
	p.Description = pkg.Description
	p.PriceFen = pkg.AmountFen
	p.Image = pkg.Image
	p.CategoryID = cat
	p.Status = pkg.Status
	if p.Stock < 1 {
		p.Stock = 1_000_000
	}
	pkg.ProductID = p.ID
}

func (r *adminRepo) UploadsDir() string {
	if r == nil || r.data == nil {
		return "data/uploads"
	}
	return r.data.UploadsDir()
}

func (r *adminRepo) ListPackages(ctx context.Context, days int64) ([]*biz.Package, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	return r.data.listPackagesLocked(days), nil
}

func (r *adminRepo) CreatePackage(ctx context.Context, in *biz.Package) (*biz.Package, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	if r.data.packageAmountTakenLocked(in.ReleaseDays, in.AmountFen, 0) {
		return nil, biz.ErrPackageAmountExists
	}
	r.data.packageSeq++
	p := biz.ClonePackage(in)
	p.ID = r.data.packageSeq
	r.data.syncPackageProductLocked(p)
	if r.data.packages == nil {
		r.data.packages = map[int64]*biz.Package{}
	}
	r.data.packages[p.ID] = p
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return biz.ClonePackage(p), nil
}

func (r *adminRepo) UpdatePackage(ctx context.Context, in *biz.Package, setImage bool) (*biz.Package, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	cur, ok := r.data.packages[in.ID]
	if !ok || cur.ReleaseDays != in.ReleaseDays {
		return nil, biz.ErrProductNotFound
	}
	if r.data.packageAmountTakenLocked(in.ReleaseDays, in.AmountFen, in.ID) {
		return nil, biz.ErrPackageAmountExists
	}
	cur.Name = in.Name
	cur.Description = in.Description
	cur.AmountFen = in.AmountFen
	cur.Status = in.Status
	if setImage {
		cur.Image = in.Image
	}
	r.data.syncPackageProductLocked(cur)
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return biz.ClonePackage(cur), nil
}

func (r *adminRepo) SetPackageStatus(ctx context.Context, id, days int64, status int32) (*biz.Package, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	cur, ok := r.data.packages[id]
	if !ok || cur.ReleaseDays != days {
		return nil, biz.ErrProductNotFound
	}
	cur.Status = status
	r.data.syncPackageProductLocked(cur)
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return biz.ClonePackage(cur), nil
}
