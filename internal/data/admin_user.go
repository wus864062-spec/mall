package data

import (
	"context"
	"strings"

	"mall/internal/biz"
)

func (d *Data) adminUserViewLocked(u *biz.User) *biz.AdminUserView {
	d.ensureAssetMaps()
	if u == nil {
		return nil
	}
	left := biz.LeftMembersOf(d.users, u)
	right := biz.RightMembersOf(d.users, u)
	leftPerf := biz.SumPerf(left)
	rightPerf := biz.SumPerf(right)
	var invitees int64
	for _, x := range d.users {
		if x != nil && x.InviterID == u.ID {
			invitees++
		}
	}
	inviterAddr := ""
	if inv := d.users[u.InviterID]; inv != nil {
		inviterAddr = inv.WalletAddress
	}
	sv := biz.BuildStaticView(d.ispayLocked[u.ID], u.StaticDays, u.StaticReleasedDays, u.StaticPackageFen, d.priceFenLocked(), 0, 0)
	orders := d.orderListLocked()
	paidSum := biz.Web3PaidSumFen(orders, u.ID)
	var lastOrderAt int64
	for _, o := range d.orders {
		if o == nil || o.UserID != u.ID || o.Status != biz.OrderPaid {
			continue
		}
		if o.CreatedAt > lastOrderAt {
			lastOrderAt = o.CreatedAt
		}
	}
	return &biz.AdminUserView{
		User:                  cloneUser(u),
		BalanceFen:            d.balances[u.ID],
		FrozenUsdtFen:         d.frozenUsdt[u.ID],
		IspayLockedMicro:      d.ispayLocked[u.ID],
		IspayFreeMicro:        d.ispayFree[u.ID],
		FrozenIspayMicro:      d.ispayFrozen[u.ID],
		IsAdmin:               biz.IsAdmin(u),
		LeftPerfFen:           leftPerf,
		RightPerfFen:          rightPerf,
		InviteeCount:          invitees,
		InviterAddress:        inviterAddr,
		PackageFen:            u.StaticPackageFen,                    // 档位，列表不展示认购金额
		PaidSumFen:            paidSum,                               // 认购金额=Web3 实付合计
		RechargeFen:           biz.RechargeRemainFen(d.ledger, u.ID), // 充值余额=充值−认购−提现剩余
		StaticDays:            u.StaticDays,                          // 购买/天数分母
		StaticReleasedDays:    u.StaticReleasedDays,                  // 购买/天数分子
		StaticReleasedUsdtFen: sv.ReleasedUsdtFen,
		StaticRemainUsdtFen:   sv.RemainUsdtFen,
		StaticDailyUsdtFen:    sv.DailyUsdtFen,
		StaticFinished:        sv.Finished,
		LastOrderAt:           lastOrderAt,
		SettledPairFen:        u.SettledPairFen, // 已碰业绩=已消耗对碰，不是当前小区
	}
}

func (r *adminRepo) SetUserLocked(ctx context.Context, userID int64, locked, line bool) (int, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	root, ok := r.data.users[userID]
	if !ok {
		return 0, biz.ErrUserNotExist
	}
	ids := []int64{userID}
	if line {
		ids = biz.LineUserIDs(r.data.users, userID)
	}
	n := 0
	for _, id := range ids {
		u := r.data.users[id]
		if u == nil || biz.IsAdmin(u) {
			continue
		}
		u.Locked = locked
		if locked {
			u.TokenVersion++
		}
		n++
	}
	if n == 0 {
		if biz.IsAdmin(root) {
			return 0, biz.ErrAdminProtected
		}
		return 0, biz.ErrUserNotExist
	}
	if err := r.data.save(); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *adminRepo) SetSkipUplineReward(ctx context.Context, userID int64, skip bool) error {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	u, ok := r.data.users[userID]
	if !ok {
		return biz.ErrUserNotExist
	}
	u.SkipUplineReward = skip
	return r.data.save()
}

func (r *adminRepo) SetUserUSDT(ctx context.Context, userID, amountFen int64) error {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	r.data.ensureAssetMaps()
	if _, ok := r.data.users[userID]; !ok {
		return biz.ErrUserNotExist
	}
	cur := r.data.balances[userID]
	delta := amountFen - cur
	r.data.balances[userID] = amountFen
	if delta != 0 {
		r.data.appendAssetLedger(userID, delta, amountFen, biz.LedgerAdminAdjust, "admin", 0)
	}
	return r.data.save()
}

func (r *adminRepo) AddUserUSDT(ctx context.Context, userID, amountFen int64) error {
	r.data.mu.RLock()
	_, ok := r.data.users[userID]
	r.data.mu.RUnlock()
	if !ok {
		return biz.ErrUserNotExist
	}
	_, err := NewShopRepo(r.data).Recharge(ctx, userID, amountFen, "")
	return err
}

func (r *adminRepo) SetUserIspayFree(ctx context.Context, userID, micro int64) error {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	r.data.ensureAssetMaps()
	if _, ok := r.data.users[userID]; !ok {
		return biz.ErrUserNotExist
	}
	cur := r.data.ispayFree[userID]
	delta := micro - cur
	r.data.ispayFree[userID] = micro
	if delta != 0 {
		r.data.appendAssetLedger(userID, delta, micro, biz.LedgerAdminIspay, "admin", 0)
	}
	return r.data.save()
}

func (r *adminRepo) AdminPlaceOrder(ctx context.Context, userID, productID int64) (*biz.Order, error) {
	r.data.mu.RLock()
	_, ok := r.data.users[userID]
	r.data.mu.RUnlock()
	if !ok {
		return nil, biz.ErrUserNotExist
	}
	return NewShopRepo(r.data).PlaceOrder(ctx, userID, productID, 1)
}

func (r *adminRepo) ChangeUserWallet(ctx context.Context, userID int64, wallet string) error {
	wallet = strings.ToLower(biz.NormalizeEthAddress(wallet))
	if len(wallet) != 42 || !strings.HasPrefix(wallet, "0x") {
		return biz.ErrInvalidArgument
	}
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	u, ok := r.data.users[userID]
	if !ok {
		return biz.ErrUserNotExist
	}
	if strings.EqualFold(u.WalletAddress, wallet) {
		return nil
	}
	if other, taken := r.data.byWallet[wallet]; taken && other != userID {
		return biz.ErrWalletTaken
	}
	old := u.WalletAddress
	delete(r.data.byWallet, old)
	delete(r.data.byWallet, strings.ToLower(old))
	u.WalletAddress = wallet
	u.TokenVersion++
	r.data.byWallet[wallet] = userID
	return r.data.save()
}

func (r *adminRepo) FindWeb3ProductByPrice(ctx context.Context, priceFen int64) (*biz.Product, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	var found *biz.Product
	for _, p := range r.data.products {
		if p == nil || p.PriceFen != priceFen || !biz.IsWeb3Category(p.CategoryID) {
			continue
		}
		if p.Status == biz.ProductOnSale {
			return cloneProduct(p), nil
		}
		if found == nil {
			found = p
		}
	}
	if found == nil {
		return nil, biz.ErrProductNotFound
	}
	return cloneProduct(found), nil
}
