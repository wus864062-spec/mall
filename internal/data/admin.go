package data

import (
	"context"
	"sort"
	"strconv"
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
	r.data.ensureAssetMaps()
	query = strings.ToLower(strings.TrimSpace(query))
	all := make([]*biz.User, 0, len(r.data.users))
	for _, u := range r.data.users {
		if !biz.UserMatchesQuery(u, query) {
			continue
		}
		all = append(all, cloneUser(u))
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	pageItems, total := slicePage(all, page, pageSize)
	out := make([]*biz.AdminUserView, 0, len(pageItems))
	for _, u := range pageItems {
		out = append(out, r.data.adminUserViewLocked(u))
	}
	return out, total, nil
}

func (r *adminRepo) GetUserDetail(ctx context.Context, id int64) (*biz.AdminUserDetail, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	r.data.ensureAssetMaps()
	u, ok := r.data.users[id]
	if !ok {
		return nil, biz.ErrUserNotExist
	}
	left := biz.LeftMembersOf(r.data.users, u)
	right := biz.RightMembersOf(r.data.users, u)
	leftPerf := biz.SumPerf(left)
	rightPerf := biz.SumPerf(right)
	availL, availR, pairVol := biz.PairPending(leftPerf, rightPerf, u.SettledPairFen)
	capFen := r.data.highestPairCapLocked(u.ID)
	bonus := biz.PairRewardFen(pairVol)
	pay := biz.CapDynamicPayout(bonus, capFen, r.data.dynamicUsedLocked(u, time.Now()))
	view := r.data.adminUserViewLocked(u)
	return &biz.AdminUserDetail{
		AdminUserView: *view,
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

func (r *adminRepo) CreateProduct(ctx context.Context, in *biz.Product) (*biz.Product, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	var maxID int64
	for id := range r.data.products {
		if id > maxID {
			maxID = id
		}
	}
	p := cloneProduct(in)
	p.ID = maxID + 1
	r.data.products[p.ID] = p
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return cloneProduct(p), nil
}

func (r *adminRepo) UpdateProductMeta(ctx context.Context, in *biz.Product, setName, setDesc, setPrice, setStock, setStatus, setCategory bool) (*biz.Product, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	p, ok := r.data.products[in.ID]
	if !ok {
		return nil, biz.ErrProductNotFound
	}
	if setName {
		if strings.TrimSpace(in.Name) == "" {
			return nil, biz.ErrInvalidArgument
		}
		p.Name = strings.TrimSpace(in.Name)
	}
	if setDesc {
		p.Description = in.Description
	}
	if setPrice {
		if in.PriceFen <= 0 {
			return nil, biz.ErrInvalidArgument
		}
		p.PriceFen = in.PriceFen
	}
	if setStock {
		if in.Stock < 0 {
			return nil, biz.ErrInvalidArgument
		}
		p.Stock = in.Stock
	}
	if setStatus {
		p.Status = in.Status
	}
	if setCategory {
		p.CategoryID = in.CategoryID
	}
	if err := r.data.save(); err != nil {
		return nil, err
	}
	return cloneProduct(p), nil
}

func (r *adminRepo) matchLedgerLocked(userID int64, typ string) []*biz.LedgerEntry {
	typ = strings.TrimSpace(typ)
	var all []*biz.LedgerEntry
	for i := len(r.data.ledger) - 1; i >= 0; i-- {
		e := r.data.ledger[i]
		if userID > 0 && e.UserID != userID {
			continue
		}
		if !ledgerTypeMatch(e.Type, typ) {
			continue
		}
		all = append(all, cloneLedger(e))
		cp := all[len(all)-1]
		if u := r.data.users[e.UserID]; u != nil && cp != nil {
			cp.WalletAddress = u.WalletAddress
		}
	}
	return all
}

func (r *adminRepo) ListAllLedger(ctx context.Context, page, pageSize int32, userID int64, typ string) ([]*biz.LedgerEntry, int64, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	_ = ctx
	out, total := slicePage(r.matchLedgerLocked(userID, typ), page, pageSize)
	return out, total, nil
}

func (r *adminRepo) ListAllLedgerMerged(ctx context.Context, page, pageSize int32, userID int64, typ, unit string, priceFen int64) ([]*biz.LedgerEntry, int64, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	_ = ctx
	merged := biz.MergeSplitLedger(r.matchLedgerLocked(userID, typ), unit, typ, priceFen)
	out, total := slicePage(merged, page, pageSize)
	return out, total, nil
}

func ledgerTypeMatch(got, want string) bool {
	if want == "" {
		return true
	}
	for _, t := range strings.Split(want, ",") {
		if strings.TrimSpace(t) == got {
			return true
		}
	}
	return false
}

func (r *adminRepo) FindUserIDByWallet(ctx context.Context, wallet string) int64 {
	_ = ctx
	q := strings.ToLower(strings.TrimSpace(wallet))
	if q == "" {
		return 0
	}
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	if id, ok := r.data.byWallet[q]; ok {
		return id
	}
	if n := biz.NormalizeEthAddress(wallet); n != "" {
		if id, ok := r.data.byWallet[strings.ToLower(n)]; ok {
			return id
		}
	}
	var hit int64
	n := 0
	for _, u := range r.data.users {
		if u == nil {
			continue
		}
		if !strings.Contains(strings.ToLower(u.WalletAddress), q) {
			continue
		}
		n++
		hit = u.ID
		if n > 1 {
			return 0
		}
	}
	if n == 1 {
		return hit
	}
	return 0
}

func (r *adminRepo) SumRewardFen(ctx context.Context, userID int64) (staticFen, dynamicFen int64) {
	_ = ctx
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	for _, e := range r.data.ledger {
		if e == nil {
			continue
		}
		if userID > 0 && e.UserID != userID {
			continue
		}
		switch biz.RewardUSDTKind(e.Type) {
		case "static":
			staticFen += e.AmountFen
		case "dynamic":
			dynamicFen += e.AmountFen
		}
	}
	return staticFen, dynamicFen
}

func (r *adminRepo) ListAllOrders(ctx context.Context, page, pageSize int32, query string, userID int64) ([]*biz.Order, []*biz.User, int64, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	query = strings.ToLower(strings.TrimSpace(query))
	all := make([]*biz.Order, 0, len(r.data.orders))
	for _, o := range r.data.orders {
		if o == nil {
			continue
		}
		if userID > 0 && o.UserID != userID {
			continue
		}
		u := r.data.users[o.UserID]
		if query != "" {
			ok := false
			if u != nil && (strings.Contains(strings.ToLower(u.WalletAddress), query) || strings.Contains(strings.ToLower(u.Nickname), query) || strconv.FormatInt(u.ID, 10) == query) {
				ok = true
			}
			if !ok && strconv.FormatInt(o.UserID, 10) == query {
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

func (r *adminRepo) Search(ctx context.Context, query string) (*biz.AdminSearchResult, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	r.data.ensureAssetMaps()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids := make([]int64, 0)
	for _, u := range r.data.users {
		if !biz.UserMatchesQuery(u, query) {
			continue
		}
		ids = append(ids, u.ID)
		if len(ids) >= biz.SearchMaxHits {
			break
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := &biz.AdminSearchResult{Query: query}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		u := r.data.users[id]
		if u == nil {
			continue
		}
		left := biz.LeftMembersOf(r.data.users, u)
		right := biz.RightMembersOf(r.data.users, u)
		leftPerf := biz.SumPerf(left)
		rightPerf := biz.SumPerf(right)
		availL, availR, pairVol := biz.PairPending(leftPerf, rightPerf, u.SettledPairFen)
		orders := make([]*biz.Order, 0)
		for _, o := range r.data.orders {
			if o != nil && o.UserID == id {
				orders = append(orders, cloneOrder(o))
			}
		}
		sort.Slice(orders, func(i, j int) bool { return orders[i].ID > orders[j].ID })
		if len(orders) > 20 {
			orders = orders[:20]
		}
		ledger := make([]*biz.LedgerEntry, 0)
		for i := len(r.data.ledger) - 1; i >= 0; i-- {
			e := r.data.ledger[i]
			if e == nil || e.UserID != id {
				continue
			}
			ledger = append(ledger, cloneLedger(e))
			if len(ledger) >= 20 {
				break
			}
		}
		capFen := r.data.highestPairCapLocked(id)
		bonus := biz.PairRewardFen(pairVol)
		pay := biz.CapDynamicPayout(bonus, capFen, r.data.dynamicUsedLocked(u, time.Now()))
		out.Hits = append(out.Hits, &biz.AdminSearchHit{
			AdminUserDetail: &biz.AdminUserDetail{
				AdminUserView: biz.AdminUserView{
					User:             cloneUser(u),
					BalanceFen:       r.data.balances[id],
					FrozenUsdtFen:    r.data.frozenUsdt[id],
					IspayLockedMicro: r.data.ispayLocked[id],
					IspayFreeMicro:   r.data.ispayFree[id],
					FrozenIspayMicro: r.data.ispayFrozen[id],
					IsAdmin:          biz.IsAdmin(u),
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
			},
			Orders: orders,
			Ledger: ledger,
		})
	}
	return out, nil
}

func ordersForUser(all map[int64]*biz.Order, userID int64) []*biz.Order {
	out := make([]*biz.Order, 0)
	for _, o := range all {
		if o != nil && o.UserID == userID {
			out = append(out, o)
		}
	}
	return out
}

func shanghaiDayBounds(now time.Time) (start, end int64) {
	loc := pairSettleLocation()
	t := now.In(loc)
	s := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	return s.Unix(), s.Add(24 * time.Hour).Unix()
}

func inUnixRange(ts, start, end int64) bool {
	return ts >= start && ts < end
}

func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func (r *adminRepo) Stats(ctx context.Context) (*biz.AdminStats, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	r.data.ensureAssetMaps()
	start, end := shanghaiDayBounds(time.Now())
	price := r.data.priceFenLocked()
	s := &biz.AdminStats{
		LastPairSettleDate: r.data.lastPairSettleDate,
		IspayPriceFen:      price,
	}
	for id, u := range r.data.users {
		if u == nil {
			continue
		}
		s.UserCount++
		if inUnixRange(u.CreatedAt, start, end) {
			s.TodayUserCount++
		}
		s.BalanceUsdtFen += r.data.balances[id] // 可提用不含冻结
		s.TotalIspayMicro += r.data.ispayLocked[id] + r.data.ispayFree[id] + r.data.ispayFrozen[id]
	}
	firstSub := map[int64]int64{}
	for _, o := range r.data.orders {
		if o == nil || !biz.IsSubscribeOrder(o) {
			continue
		}
		s.PaidOrderCount++
		pay := biz.OrderPayFen(o)
		s.PaidOrderFen += pay
		if inUnixRange(o.CreatedAt, start, end) {
			s.TodayPaidOrderCount++
			s.TodayPaidOrderFen += pay
		}
		if t, ok := firstSub[o.UserID]; !ok || o.CreatedAt < t {
			firstSub[o.UserID] = o.CreatedAt
		}
	}
	s.ActivatedCount = int64(len(firstSub))
	for _, ts := range firstSub {
		if inUnixRange(ts, start, end) {
			s.TodayActivatedCount++
		}
	}
	for _, e := range r.data.ledger {
		if e == nil {
			continue
		}
		today := inUnixRange(e.CreatedAt, start, end)
		switch e.Type {
		case biz.LedgerRecharge:
			if e.TxHash != "" || e.RefType == "tx" {
				s.RechargeFen += e.AmountFen
				if today {
					s.TodayRechargeFen += e.AmountFen
				}
			} else {
				s.AdminAdjustFen += e.AmountFen // 后台「设置充值USDT」无哈希
			}
		case biz.LedgerAdminAdjust:
			if e.AmountFen > 0 {
				s.AdminAdjustFen += e.AmountFen
			}
		case biz.LedgerStaticReward:
			s.StaticFen += e.AmountFen
			s.TotalRewardFen += e.AmountFen
			if today {
				s.TodayStaticFen += e.AmountFen
			}
		case biz.LedgerStaticRewardIspay:
			s.StaticIspayMicro += e.AmountFen
			if today {
				s.TodayStaticIspayMicro += e.AmountFen
			}
		case biz.LedgerDirectReward, biz.LedgerDirectRewardClawback:
			s.Direct.AddUSDT(e.AmountFen, today) // 直推只用直推流水，含回退
			s.DynamicFen += e.AmountFen
			s.TotalRewardFen += e.AmountFen
			if today {
				s.TodayDynamicFen += e.AmountFen
			}
		case biz.LedgerPairReward, biz.LedgerPairRewardClawback, biz.LedgerPairUpline:
			s.Pair.AddUSDT(e.AmountFen, today)
			s.DynamicFen += e.AmountFen
			s.TotalRewardFen += e.AmountFen
			if today {
				s.TodayDynamicFen += e.AmountFen
			}
		case biz.LedgerManageReward:
			s.Manage.AddUSDT(e.AmountFen, today)
			s.DynamicFen += e.AmountFen
			s.TotalRewardFen += e.AmountFen
			if today {
				s.TodayDynamicFen += e.AmountFen
			}
		case biz.LedgerDirectRewardIspay, biz.LedgerDirectRewardIspayClawback:
			s.Direct.AddIspay(e.AmountFen, today)
			s.DynamicIspayMicro += e.AmountFen
			if today {
				s.TodayDynamicIspayMicro += e.AmountFen
			}
		case biz.LedgerPairRewardIspay:
			s.Pair.AddIspay(e.AmountFen, today)
			s.DynamicIspayMicro += e.AmountFen
			if today {
				s.TodayDynamicIspayMicro += e.AmountFen
			}
		case biz.LedgerManageRewardIspay:
			s.Manage.AddIspay(e.AmountFen, today)
			s.DynamicIspayMicro += e.AmountFen
			if today {
				s.TodayDynamicIspayMicro += e.AmountFen
			}
		case biz.LedgerWithdraw:
			w := absInt64(e.AmountFen)
			s.TotalWithdrawFen += w
			if today {
				s.TodayWithdrawFen += w
			}
		case biz.LedgerIspayWithdraw:
			w := absInt64(e.AmountFen)
			s.TotalWithdrawIspayMicro += w
			if today {
				s.TodayWithdrawIspayMicro += w
			}
		}
	}
	s.TodayRewardFen = s.TodayStaticFen + s.TodayDynamicFen
	// 总静态/总动态跟分红数据一样：USDT=U+币转U，IsPay=币+U转币。用行情价，不用认购兑换价。
	s.TodayStaticTotalFen = s.TodayStaticFen + biz.IspayMicroToMarketFen(s.TodayStaticIspayMicro, price)
	s.TodayStaticTotalIspayMicro = s.TodayStaticIspayMicro + biz.MarketFenToIspayMicro(s.TodayStaticFen, price)
	s.StaticTotalFen = s.StaticFen + biz.IspayMicroToMarketFen(s.StaticIspayMicro, price)
	s.StaticTotalIspayMicro = s.StaticIspayMicro + biz.MarketFenToIspayMicro(s.StaticFen, price)
	s.TodayDynamicTotalFen = s.TodayDynamicFen + biz.IspayMicroToMarketFen(s.TodayDynamicIspayMicro, price)
	s.TodayDynamicTotalIspayMicro = s.TodayDynamicIspayMicro + biz.MarketFenToIspayMicro(s.TodayDynamicFen, price)
	s.DynamicTotalFen = s.DynamicFen + biz.IspayMicroToMarketFen(s.DynamicIspayMicro, price)
	s.DynamicTotalIspayMicro = s.DynamicIspayMicro + biz.MarketFenToIspayMicro(s.DynamicFen, price)
	biz.CompleteRewardSplit(&s.Direct, price)
	biz.CompleteRewardSplit(&s.Pair, price)
	biz.CompleteRewardSplit(&s.Manage, price)
	s.DirectRewardFen = s.Direct.Fen // 旧字段=直推累计USDT原币
	s.PairRewardFen = s.Pair.Fen
	s.ManageRewardFen = s.Manage.Fen
	return s, nil
}

func (r *adminRepo) ForcePairSettle(ctx context.Context) (string, error) {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	r.data.settlePairingLocked()
	r.data.settleStaticLocked()
	r.data.lastPairSettleDate = time.Now().In(pairSettleLocation()).Format("2006-01-02")
	if err := r.data.save(); err != nil {
		return "", err
	}
	return r.data.lastPairSettleDate, nil
}

func (r *adminRepo) ClearTestData(ctx context.Context) error {
	return r.data.ClearTestData()
}

func (r *adminRepo) GetIspayPriceFen(ctx context.Context) (int64, error) {
	s, err := r.GetSettings(ctx)
	if err != nil {
		return 0, err
	}
	return s.IspayPriceFen, nil
}

func (r *adminRepo) SetIspayPriceFen(ctx context.Context, priceFen int64) error {
	s, err := r.GetSettings(ctx)
	if err != nil {
		return err
	}
	s.IspayPriceFen = priceFen
	return r.SetSettings(ctx, s)
}

func (r *adminRepo) GetSettings(ctx context.Context) (biz.SiteSettings, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	return r.data.settingsLocked(), nil
}

func (r *adminRepo) SetSettings(ctx context.Context, s biz.SiteSettings) error {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	r.data.applySettingsLocked(s)
	return r.data.save()
}

func cloneUsers(in []*biz.User) []*biz.User {
	out := make([]*biz.User, 0, len(in))
	for _, u := range in {
		out = append(out, cloneUser(u))
	}
	return out
}
