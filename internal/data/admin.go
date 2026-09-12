package data

import (
	"context"
	"sort"
	"strings"
	"time"

	"mall/internal/biz"
)

type adminRepo struct {
	data *Data
}

func NewAdminRepo(data *Data) biz.AdminRepo {
	return &adminRepo{data: data}
}

func (r *adminRepo) ListUsers(ctx context.Context, page, pageSize int32, query string) ([]*biz.AdminUserView, int64, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	query = strings.ToLower(strings.TrimSpace(query))
	all := make([]*biz.User, 0, len(r.data.users))
	for _, u := range r.data.users {
		if query != "" &&
			!strings.Contains(strings.ToLower(u.WalletAddress), query) &&
			!strings.Contains(strings.ToLower(u.Nickname), query) &&
			!strings.Contains(strings.ToLower(u.InviteCode), query) {
			continue
		}
		all = append(all, cloneUser(u))
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	pageItems, total := slicePage(all, page, pageSize)
	out := make([]*biz.AdminUserView, 0, len(pageItems))
	for _, u := range pageItems {
		out = append(out, &biz.AdminUserView{
			User:       u,
			BalanceFen: r.data.balances[u.ID],
			IsAdmin:    biz.IsAdmin(u),
		})
	}
	return out, total, nil
}

func (r *adminRepo) GetUserDetail(ctx context.Context, id int64) (*biz.AdminUserDetail, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	u, ok := r.data.users[id]
	if !ok {
		return nil, biz.ErrUserNotExist
	}
	left := biz.LeftMembersOf(r.data.users, u)
	right := biz.RightMembersOf(r.data.users, u)
	leftPerf := biz.SumPerf(left)
	rightPerf := biz.SumPerf(right)
	availL, availR, pairVol := biz.SmallHitsLarge(leftPerf, rightPerf, u.SettledPairFen, u.PairClearedRightFen)
	orders := make([]*biz.Order, 0, len(r.data.orders))
	for _, o := range r.data.orders {
		orders = append(orders, o)
	}
	capFen := biz.HighestPairCapFen(orders, u.ID)
	bonus := biz.PairRewardFen(pairVol)
	pay := biz.CapPairPayout(bonus, capFen)
	return &biz.AdminUserDetail{
		AdminUserView: biz.AdminUserView{
			User:       cloneUser(u),
			BalanceFen: r.data.balances[u.ID],
			IsAdmin:    biz.IsAdmin(u),
		},
		LeftPerfFen:   leftPerf,
		RightPerfFen:  rightPerf,
		AvailLeftFen:  availL,
		AvailRightFen: availR,
		PairVolFen:    pairVol,
		PairCapFen:    capFen,
		EstPairPayFen: pay,
		LeftMembers:   cloneUsers(left),
		RightMembers:  cloneUsers(right),
	}, nil
}

func (r *adminRepo) ListAllProducts(ctx context.Context) ([]*biz.Product, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	all := make([]*biz.Product, 0, len(r.data.products))
	for _, p := range r.data.products {
		all = append(all, cloneProduct(p))
	}
	sort.Slice(all, func(i, j int) bool { return all[i].PriceFen < all[j].PriceFen })
	return all, nil
}

func (r *adminRepo) UpdateProduct(ctx context.Context, id, priceFen, stock int64, status int32, setPrice, setStock bool) (*biz.Product, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	p, ok := r.data.products[id]
	if !ok {
		return nil, biz.ErrProductNotFound
	}
	if setPrice {
		if priceFen <= 0 {
			return nil, biz.ErrInvalidArgument
		}
		p.PriceFen = priceFen
	}
	if setStock {
		if stock < 0 {
			return nil, biz.ErrInvalidArgument
		}
		p.Stock = stock
	}
	p.Status = status
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return cloneProduct(p), nil
}

func (r *adminRepo) ListAllLedger(ctx context.Context, page, pageSize int32, userID int64, typ string) ([]*biz.LedgerEntry, int64, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	typ = strings.TrimSpace(typ)
	var all []*biz.LedgerEntry
	for i := len(r.data.ledger) - 1; i >= 0; i-- {
		e := r.data.ledger[i]
		if userID > 0 && e.UserID != userID {
			continue
		}
		if typ != "" && e.Type != typ {
			continue
		}
		all = append(all, cloneLedger(e))
	}
	out, total := slicePage(all, page, pageSize)
	return out, total, nil
}

func (r *adminRepo) ListAllOrders(ctx context.Context, page, pageSize int32, query string) ([]*biz.Order, []*biz.User, int64, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	query = strings.ToLower(strings.TrimSpace(query))
	all := make([]*biz.Order, 0, len(r.data.orders))
	for _, o := range r.data.orders {
		u := r.data.users[o.UserID]
		if query != "" {
			ok := false
			if u != nil && (strings.Contains(strings.ToLower(u.WalletAddress), query) || strings.Contains(strings.ToLower(u.Nickname), query)) {
				ok = true
			}
			if !ok {
				continue
			}
		}
		all = append(all, cloneOrder(o))
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID > all[j].ID })
	pageItems, total := slicePage(all, page, pageSize)
	users := make([]*biz.User, 0, len(pageItems))
	for _, o := range pageItems {
		users = append(users, cloneUser(r.data.users[o.UserID]))
	}
	return pageItems, users, total, nil
}

func (r *adminRepo) Stats(ctx context.Context) (*biz.AdminStats, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	s := &biz.AdminStats{
		UserCount:          int64(len(r.data.users)),
		LastPairSettleDate: r.data.lastPairSettleDate,
	}
	for _, o := range r.data.orders {
		if o != nil && o.Status == biz.OrderPaid {
			s.PaidOrderFen += o.TotalFen
		}
	}
	for _, e := range r.data.ledger {
		if e == nil {
			continue
		}
		switch e.Type {
		case biz.LedgerDirectReward:
			s.DirectRewardFen += e.AmountFen
		case biz.LedgerPairReward:
			s.PairRewardFen += e.AmountFen
		case biz.LedgerManageReward:
			s.ManageRewardFen += e.AmountFen
		}
	}
	return s, nil
}

func (r *adminRepo) ForcePairSettle(ctx context.Context) (string, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	r.data.settlePairingLocked()
	r.data.lastPairSettleDate = time.Now().In(pairSettleLocation()).Format("2006-01-02")
	if err := r.data.save(); err != nil {
		return "", err
	}
	return r.data.lastPairSettleDate, nil
}

func cloneUsers(in []*biz.User) []*biz.User {
	out := make([]*biz.User, 0, len(in))
	for _, u := range in {
		out = append(out, cloneUser(u))
	}
	return out
}
