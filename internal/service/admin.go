package service

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	adminV1 "mall/api/admin/v1"
	"mall/internal/biz"

	"github.com/go-kratos/kratos/v2/transport"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

type AdminService struct {
	adminV1.UnimplementedAdminServiceServer
	uc *biz.AdminUsecase
}

func NewAdminService(uc *biz.AdminUsecase) *AdminService {
	return &AdminService{uc: uc}
}

func (s *AdminService) GetStats(ctx context.Context, req *adminV1.GetStatsRequest) (*adminV1.GetStatsReply, error) {
	st, err := s.uc.GetStats(ctx)
	if err != nil {
		return nil, err
	}
	return &adminV1.GetStatsReply{
		UserCount:          st.UserCount,
		PaidOrderFen:       st.PaidOrderFen,
		DirectRewardFen:    st.DirectRewardFen,
		PairRewardFen:      st.PairRewardFen,
		ManageRewardFen:    st.ManageRewardFen,
		LastPairSettleDate: st.LastPairSettleDate,
	}, nil
}

func (s *AdminService) ListUsers(ctx context.Context, req *adminV1.ListUsersRequest) (*adminV1.ListUsersReply, error) {
	q := queryFromCtx(ctx, "address")
	if q == "" {
		q = queryFromCtx(ctx, "q")
	}
	list, total, err := s.uc.ListUsers(ctx, req.Page, req.PageSize, q)
	if err != nil {
		return nil, err
	}
	out := &adminV1.ListUsersReply{Total: total}
	for _, v := range list {
		out.Users = append(out.Users, toAdminUser(v))
	}
	return out, nil
}

func (s *AdminService) GetUser(ctx context.Context, req *adminV1.GetUserRequest) (*adminV1.GetUserReply, error) {
	d, err := s.uc.GetUser(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	reply := &adminV1.GetUserReply{
		User:          toAdminUser(&d.AdminUserView),
		LeftPerfFen:   d.LeftPerfFen,
		RightPerfFen:  d.RightPerfFen,
		AvailLeftFen:  d.AvailLeftFen,
		AvailRightFen: d.AvailRightFen,
		PairVolFen:    d.PairVolFen,
		PairCapFen:    d.PairCapFen,
		EstPairPayFen: d.EstPairPayFen,
	}
	for _, m := range d.LeftMembers {
		reply.LeftMembers = append(reply.LeftMembers, toTrackMember(m))
	}
	for _, m := range d.RightMembers {
		reply.RightMembers = append(reply.RightMembers, toTrackMember(m))
	}
	return reply, nil
}

func (s *AdminService) ListProducts(ctx context.Context, req *adminV1.ListProductsRequest) (*adminV1.ListProductsReply, error) {
	list, err := s.uc.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	out := &adminV1.ListProductsReply{}
	for _, p := range list {
		out.Products = append(out.Products, toAdminProduct(p))
	}
	return out, nil
}

func (s *AdminService) UpdateProduct(ctx context.Context, req *adminV1.UpdateProductRequest) (*adminV1.UpdateProductReply, error) {
	p, err := s.uc.UpdateProduct(ctx, req.Id, req.PriceFen, req.Stock, req.Status, req.PriceFen > 0, req.Stock > 0)
	if err != nil {
		return nil, err
	}
	return &adminV1.UpdateProductReply{Product: toAdminProduct(p)}, nil
}

func (s *AdminService) ListLedger(ctx context.Context, req *adminV1.ListLedgerRequest) (*adminV1.ListLedgerReply, error) {
	typ := req.Type
	if typ == "" {
		typ = queryFromCtx(ctx, "type")
	}
	addr := queryFromCtx(ctx, "address")
	list, total, err := s.uc.ListLedger(ctx, req.Page, req.PageSize, 0, typ, addr, queryFromCtx(ctx, "unit"))
	if err != nil {
		return nil, err
	}
	out := &adminV1.ListLedgerReply{Total: total}
	for _, e := range list {
		out.Entries = append(out.Entries, &adminV1.LedgerEntry{
			Id: e.ID, UserId: e.UserID, AmountFen: e.AmountFen, BalanceFen: e.BalanceFen,
			Type: e.Type, RefType: e.RefType, RefId: e.RefID, CreatedAt: e.CreatedAt,
		})
	}
	return out, nil
}

const OperationAdminHTTPListLedger = "/api.admin.v1.AdminService/HTTPListLedger"

func (s *AdminService) HTTPListLedger(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPListLedger)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		page, _ := strconv.Atoi(ctx.Query().Get("page"))
		pageSize, _ := strconv.Atoi(ctx.Query().Get("page_size"))
		typ := ctx.Query().Get("type")
		addr := ctx.Query().Get("address")
		unit := ctx.Query().Get("unit")
		list, total, err := s.uc.ListLedger(c, int32(page), int32(pageSize), 0, typ, addr, unit)
		if err != nil {
			return nil, err
		}
		staticFen, dynamicFen, err := s.uc.RewardTotals(c, addr)
		if err != nil {
			return nil, err
		}
		priceFen, err := s.uc.GetIspayPriceFen(c)
		if err != nil {
			return nil, err
		}
		return []interface{}{list, total, staticFen, dynamicFen, priceFen}, nil
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	pack := out.([]interface{})
	list := pack[0].([]*biz.LedgerEntry)
	total := pack[1].(int64)
	staticFen := pack[2].(int64)
	dynamicFen := pack[3].(int64)
	priceFen := pack[4].(int64)
	entries := make([]map[string]interface{}, 0, len(list))
	for _, e := range list {
		if e == nil {
			continue
		}
		entries = append(entries, map[string]interface{}{
			"id":            e.ID,
			"userId":        e.UserID,
			"amountFen":     e.AmountFen,
			"balanceFen":    e.BalanceFen,
			"type":          e.Type,
			"refType":       e.RefType,
			"refId":         e.RefID,
			"createdAt":     e.CreatedAt,
			"txHash":        e.TxHash,
			"walletAddress": e.WalletAddress,
			"displayUnit":   e.DisplayUnit,
		})
	}
	return ctx.Result(200, map[string]interface{}{
		"entries":       entries,
		"total":         total,
		"staticFen":     staticFen,
		"dynamicFen":    dynamicFen,
		"rewardFen":     staticFen + dynamicFen,
		"ispayPriceFen": priceFen,
	})
}

func (s *AdminService) ForcePairSettle(ctx context.Context, req *adminV1.ForcePairSettleRequest) (*adminV1.ForcePairSettleReply, error) {
	day, err := s.uc.ForcePairSettle(ctx)
	if err != nil {
		return nil, err
	}
	return &adminV1.ForcePairSettleReply{LastPairSettleDate: day}, nil
}

const OperationAdminHTTPClearTestData = "/api.admin.v1.AdminService/HTTPClearTestData"

func (s *AdminService) HTTPClearTestData(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPClearTestData)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		return nil, s.uc.ClearTestData(c)
	})
	if _, err := h(ctx, nil); err != nil {
		return err
	}
	return ctx.Result(200, map[string]string{"ok": "1"})
}

func (s *AdminService) HTTPLogin(ctx khttp.Context) error {
	var body struct {
		Username string `json:"username"`
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	if err := readJSON(ctx, &body); err != nil {
		return biz.ErrInvalidArgument
	}
	name := body.Username
	if name == "" {
		name = body.Account
	}
	_, token, err := s.uc.PasswordLogin(ctx, name, body.Password)
	if err != nil {
		return err
	}
	return ctx.Result(200, map[string]string{"token": token})
}

const OperationAdminHTTPListOrders = "/api.admin.v1.AdminService/HTTPListOrders"

func (s *AdminService) HTTPListOrders(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPListOrders)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		page, _ := strconv.Atoi(ctx.Query().Get("page"))
		pageSize, _ := strconv.Atoi(ctx.Query().Get("page_size"))
		q := ctx.Query().Get("address")
		userID, _ := strconv.ParseInt(ctx.Query().Get("user_id"), 10, 64)
		orders, users, total, err := s.uc.ListOrders(c, int32(page), int32(pageSize), q, userID)
		if err != nil {
			return nil, err
		}
		return []interface{}{orders, users, total}, nil
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	pack := out.([]interface{})
	orders := pack[0].([]*biz.Order)
	users := pack[1].([]*biz.User)
	total := pack[2].(int64)
	rows := make([]map[string]interface{}, 0, len(orders))
	for i, o := range orders {
		name := ""
		if len(o.Items) > 0 {
			name = o.Items[0].Name
		}
		addr := ""
		if i < len(users) && users[i] != nil {
			addr = users[i].WalletAddress
		}
		rows = append(rows, map[string]interface{}{
			"id":        o.ID,
			"userId":    o.UserID,
			"createdAt": biz.FormatUnix(o.CreatedAt),
			"amount":    biz.FenToUstd(o.TotalFen),
			"address":   addr,
			"one":       name,
			"status":    o.Status,
		})
	}
	return ctx.Result(200, map[string]interface{}{"rewards": rows, "orders": rows, "count": total, "total": total})
}

const OperationAdminHTTPSearch = "/api.admin.v1.AdminService/HTTPSearch"

func (s *AdminService) HTTPSearch(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPSearch)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		q := ctx.Query().Get("q")
		if q == "" {
			q = ctx.Query().Get("address")
		}
		return s.uc.Search(c, q)
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	res := out.(*biz.AdminSearchResult)
	hits := make([]map[string]interface{}, 0, len(res.Hits))
	for _, hit := range res.Hits {
		if hit == nil || hit.AdminUserDetail == nil || hit.User == nil {
			continue
		}
		u := hit.User
		left := make([]map[string]interface{}, 0, len(hit.LeftMembers))
		for _, m := range hit.LeftMembers {
			left = append(left, map[string]interface{}{"id": m.ID, "nickname": m.Nickname, "inviteCode": m.InviteCode, "perfFen": m.PerfFen})
		}
		right := make([]map[string]interface{}, 0, len(hit.RightMembers))
		for _, m := range hit.RightMembers {
			right = append(right, map[string]interface{}{"id": m.ID, "nickname": m.Nickname, "inviteCode": m.InviteCode, "perfFen": m.PerfFen})
		}
		orders := make([]map[string]interface{}, 0, len(hit.Orders))
		for _, o := range hit.Orders {
			name := ""
			if len(o.Items) > 0 {
				name = o.Items[0].Name
			}
			orders = append(orders, map[string]interface{}{
				"id": o.ID, "status": o.Status, "totalFen": o.TotalFen, "name": name, "createdAt": o.CreatedAt,
			})
		}
		ledger := make([]map[string]interface{}, 0, len(hit.Ledger))
		for _, e := range hit.Ledger {
			ledger = append(ledger, map[string]interface{}{
				"id": e.ID, "type": e.Type, "amountFen": e.AmountFen, "balanceFen": e.BalanceFen, "createdAt": e.CreatedAt,
			})
		}
		hits = append(hits, map[string]interface{}{
			"id":               u.ID,
			"walletAddress":    u.WalletAddress,
			"nickname":         u.Nickname,
			"inviteCode":       u.InviteCode,
			"inviterId":        u.InviterID,
			"parentId":         u.ParentID,
			"side":             u.Side,
			"perfFen":          u.PerfFen,
			"balanceFen":       hit.BalanceFen,
			"frozenUsdtFen":    hit.FrozenUsdtFen,
			"ispayLockedMicro": hit.IspayLockedMicro,
			"ispayFreeMicro":   hit.IspayFreeMicro,
			"frozenIspayMicro": hit.FrozenIspayMicro,
			"leftPerfFen":      hit.LeftPerfFen,
			"rightPerfFen":     hit.RightPerfFen,
			"availLeftFen":     hit.AvailLeftFen,
			"availRightFen":    hit.AvailRightFen,
			"pairVolFen":       hit.PairVolFen,
			"pairCapFen":       hit.PairCapFen,
			"estPairPayFen":    hit.EstPairPayFen,
			"leftMembers":      left,
			"rightMembers":     right,
			"orders":           orders,
			"ledger":           ledger,
		})
	}
	return ctx.Result(200, map[string]interface{}{
		"query":   res.Query,
		"timeout": int(biz.SearchTimeout / time.Second),
		"hits":    hits,
		"count":   len(hits),
	})
}

func productJSON(p *biz.Product) map[string]interface{} {
	if p == nil {
		return nil
	}
	return map[string]interface{}{
		"id":          p.ID,
		"name":        p.Name,
		"description": p.Description,
		"priceFen":    p.PriceFen,
		"stock":       p.Stock,
		"status":      p.Status,
		"categoryId":  p.CategoryID,
		"image":       p.Image,
	}
}

const OperationAdminHTTPListGoods = "/api.admin.v1.AdminService/HTTPListGoods"

func (s *AdminService) HTTPListGoods(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPListGoods)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		list, err := s.uc.ListProducts(c)
		if err != nil {
			return nil, err
		}
		st, err := s.uc.GetSettings(c)
		if err != nil {
			return nil, err
		}
		rows := make([]map[string]interface{}, 0, len(list))
		for _, p := range list {
			row := productJSON(p)
			if row != nil {
				row["pairCapFen"] = biz.PairCapFenWith(p.PriceFen, st.PairCaps)
			}
			rows = append(rows, row)
		}
		return rows, nil
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return ctx.Result(200, map[string]interface{}{"products": out})
}

const OperationAdminHTTPCreateProduct = "/api.admin.v1.AdminService/HTTPCreateProduct"

func (s *AdminService) HTTPCreateProduct(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPCreateProduct)
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		PriceFen    int64  `json:"price_fen"`
		Stock       int64  `json:"stock"`
		Status      int32  `json:"status"`
		CategoryID  int64  `json:"category_id"`
		Image       string `json:"image"`
	}
	if err := readJSON(ctx, &body); err != nil {
		return biz.ErrInvalidArgument
	}
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		return s.uc.CreateProduct(c, &biz.Product{
			Name:        body.Name,
			Description: body.Description,
			PriceFen:    body.PriceFen,
			Stock:       body.Stock,
			Status:      body.Status,
			CategoryID:  body.CategoryID,
			Image:       body.Image,
		})
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return ctx.Result(200, map[string]interface{}{"product": productJSON(out.(*biz.Product))})
}

const OperationAdminHTTPUpdateProductMeta = "/api.admin.v1.AdminService/HTTPUpdateProductMeta"

func (s *AdminService) HTTPUpdateProductMeta(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPUpdateProductMeta)
	id, _ := strconv.ParseInt(ctx.Vars().Get("id"), 10, 64)
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		PriceFen    *int64  `json:"price_fen"`
		Stock       *int64  `json:"stock"`
		Status      *int32  `json:"status"`
		CategoryID  *int64  `json:"category_id"`
	}
	if err := readJSON(ctx, &body); err != nil {
		return biz.ErrInvalidArgument
	}
	p := &biz.Product{ID: id}
	if body.Name != nil {
		p.Name = *body.Name
	}
	if body.Description != nil {
		p.Description = *body.Description
	}
	if body.PriceFen != nil {
		p.PriceFen = *body.PriceFen
	}
	if body.Stock != nil {
		p.Stock = *body.Stock
	}
	if body.Status != nil {
		p.Status = *body.Status
	}
	if body.CategoryID != nil {
		p.CategoryID = *body.CategoryID
	}
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		return s.uc.UpdateProductMeta(c, p, body.Name != nil, body.Description != nil, body.PriceFen != nil, body.Stock != nil, body.Status != nil, body.CategoryID != nil)
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return ctx.Result(200, map[string]interface{}{"product": productJSON(out.(*biz.Product))})
}

const OperationAdminHTTPConfig = "/api.admin.v1.AdminService/HTTPConfig"

func (s *AdminService) HTTPGetConfig(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPConfig)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		return s.uc.GetSettings(c)
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	st := out.(biz.SiteSettings)
	rows := []map[string]interface{}{
		{"id": "ispay_price", "name": "IsPay当日价格(USDT/枚)", "value": biz.FenToUstd(st.IspayPriceFen)},
		{"id": "max_recharge", "name": "最高充值上限(USDT)", "value": biz.FenToUstd(st.MaxRechargeFen)},
		{"id": "withdraw_fee", "name": "提现手续费(%)", "value": st.WithdrawFeePercent},
		{"id": "min_withdraw", "name": "最低提现(USDT)", "value": biz.FenToUstd(st.MinWithdrawFen)},
	}
	for _, price := range biz.PairCapConfigTiers() {
		rows = append(rows, map[string]interface{}{
			"id":    biz.PairCapConfigID(price),
			"name":  biz.PairCapConfigName(price),
			"value": biz.FenToUstd(biz.PairCapFenWith(price, st.PairCaps)),
		})
	}
	return ctx.Result(200, map[string]interface{}{"config": rows})
}

func (s *AdminService) HTTPUpdateConfig(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPConfig)
	var body struct {
		ID    string      `json:"id"`
		Value interface{} `json:"value"`
	}
	if err := readJSON(ctx, &body); err != nil {
		return biz.ErrInvalidArgument
	}
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		st, err := s.uc.GetSettings(c)
		if err != nil {
			return nil, err
		}
		switch body.ID {
		case "ispay_price":
			st.IspayPriceFen = parsePriceFen(body.Value)
		case "max_recharge":
			st.MaxRechargeFen = parsePriceFen(body.Value)
		case "withdraw_fee":
			st.WithdrawFeePercent = parseInt64(body.Value)
		case "min_withdraw":
			st.MinWithdrawFen = parsePriceFen(body.Value)
		default:
			price, ok := biz.ParsePairCapConfigID(body.ID)
			if !ok {
				return nil, biz.ErrInvalidArgument
			}
			if st.PairCaps == nil {
				st.PairCaps = biz.DefaultPairCaps()
			}
			cap := parsePriceFen(body.Value)
			if cap < 0 {
				cap = 0
			}
			st.PairCaps[price] = cap
		}
		return nil, s.uc.SetSettings(c, st)
	})
	if _, err := h(ctx, nil); err != nil {
		return err
	}
	return ctx.Result(200, map[string]string{"ok": "1"})
}

const OperationAdminHTTPStats = "/api.admin.v1.AdminService/HTTPStats"

func (s *AdminService) HTTPStats(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPStats)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		return s.uc.GetStats(c)
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	st := out.(*biz.AdminStats)
	return ctx.Result(200, statsJSON(st))
}

func statsJSON(st *biz.AdminStats) map[string]interface{} {
	if st == nil {
		return map[string]interface{}{}
	}
	m := map[string]interface{}{
		"userCount":                   st.UserCount,
		"todayUserCount":              st.TodayUserCount,
		"activatedCount":              st.ActivatedCount,
		"todayActivatedCount":         st.TodayActivatedCount,
		"rechargeFen":                 st.RechargeFen,
		"todayRechargeFen":            st.TodayRechargeFen,
		"adminAdjustFen":              st.AdminAdjustFen,
		"paidOrderCount":              st.PaidOrderCount,
		"todayPaidOrderCount":         st.TodayPaidOrderCount,
		"paidOrderFen":                st.PaidOrderFen,
		"todayPaidOrderFen":           st.TodayPaidOrderFen,
		"balanceUsdtFen":              st.BalanceUsdtFen,
		"todayStaticFen":              st.TodayStaticFen,
		"todayStaticIspayMicro":       st.TodayStaticIspayMicro,
		"todayStaticTotalFen":         st.TodayStaticTotalFen,
		"todayStaticTotalIspayMicro":  st.TodayStaticTotalIspayMicro,
		"staticFen":                   st.StaticFen,
		"staticIspayMicro":            st.StaticIspayMicro,
		"staticTotalFen":              st.StaticTotalFen,
		"staticTotalIspayMicro":       st.StaticTotalIspayMicro,
		"todayDynamicFen":             st.TodayDynamicFen,
		"todayDynamicIspayMicro":      st.TodayDynamicIspayMicro,
		"todayDynamicTotalFen":        st.TodayDynamicTotalFen,
		"todayDynamicTotalIspayMicro": st.TodayDynamicTotalIspayMicro,
		"dynamicFen":                  st.DynamicFen,
		"dynamicIspayMicro":           st.DynamicIspayMicro,
		"dynamicTotalFen":             st.DynamicTotalFen,
		"dynamicTotalIspayMicro":      st.DynamicTotalIspayMicro,
		"todayRewardFen":              st.TodayRewardFen,
		"totalRewardFen":              st.TotalRewardFen,
		"todayWithdrawFen":            st.TodayWithdrawFen,
		"totalWithdrawFen":            st.TotalWithdrawFen,
		"todayWithdrawIspayMicro":     st.TodayWithdrawIspayMicro,
		"totalWithdrawIspayMicro":     st.TotalWithdrawIspayMicro,
		"totalIspayMicro":             st.TotalIspayMicro,
		"directRewardFen":             st.DirectRewardFen,
		"pairRewardFen":               st.PairRewardFen,
		"manageRewardFen":             st.ManageRewardFen,
		"lastPairSettleDate":          st.LastPairSettleDate,
		"ispayPriceFen":               st.IspayPriceFen,
	}
	putSplitJSON(m, "direct", st.Direct)
	putSplitJSON(m, "pair", st.Pair)
	putSplitJSON(m, "manage", st.Manage)
	return m
}

func putSplitJSON(m map[string]interface{}, name string, s biz.RewardSplit) {
	capName := strings.ToUpper(name[:1]) + name[1:]
	m["today"+capName+"Fen"] = s.TodayFen
	m["today"+capName+"IspayMicro"] = s.TodayIspayMicro
	m["today"+capName+"TotalFen"] = s.TodayTotalFen
	m["today"+capName+"TotalIspayMicro"] = s.TodayTotalIspayMicro
	m[name+"Fen"] = s.Fen
	m[name+"IspayMicro"] = s.IspayMicro
	m[name+"TotalFen"] = s.TotalFen
	m[name+"TotalIspayMicro"] = s.TotalIspayMicro
}

const OperationAdminHTTPMembers = "/api.admin.v1.AdminService/HTTPListMembers"

func (s *AdminService) HTTPListMembers(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPMembers)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		page, _ := strconv.Atoi(ctx.Query().Get("page"))
		pageSize, _ := strconv.Atoi(ctx.Query().Get("page_size"))
		q := ctx.Query().Get("address")
		if q == "" {
			q = ctx.Query().Get("q")
		}
		list, total, err := s.uc.ListUsers(c, int32(page), int32(pageSize), q)
		if err != nil {
			return nil, err
		}
		return []interface{}{list, total}, nil
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	pack := out.([]interface{})
	list := pack[0].([]*biz.AdminUserView)
	total := pack[1].(int64)
	users := make([]map[string]interface{}, 0, len(list))
	for _, v := range list {
		users = append(users, memberJSON(v))
	}
	return ctx.Result(200, map[string]interface{}{"users": users, "total": total, "count": total})
}

const OperationAdminHTTPInvitees = "/api.admin.v1.AdminService/HTTPListInvitees"

func (s *AdminService) HTTPListInvitees(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPInvitees)
	id, _ := strconv.ParseInt(ctx.Vars().Get("id"), 10, 64)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		return s.uc.ListInvitees(c, id)
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	list := out.([]*biz.User)
	users := make([]map[string]interface{}, 0, len(list))
	for _, u := range list {
		if u == nil {
			continue
		}
		users = append(users, map[string]interface{}{
			"id":            u.ID,
			"userId":        u.ID,
			"walletAddress": u.WalletAddress,
			"nickname":      u.Nickname,
			"inviteCode":    u.InviteCode,
			"perfFen":       u.PerfFen,
		})
	}
	return ctx.Result(200, map[string]interface{}{"users": users, "count": len(users)})
}

const OperationAdminHTTPUserAction = "/api.admin.v1.AdminService/HTTPUserAction"

func (s *AdminService) HTTPUserAction(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPUserAction)
	id, _ := strconv.ParseInt(ctx.Vars().Get("id"), 10, 64)
	var body struct {
		Action      string      `json:"action"`
		Lock        interface{} `json:"lock"`
		SkipUpline  interface{} `json:"skip_upline"`
		AmountUstd  interface{} `json:"amount_ustd"`
		AmountIspay interface{} `json:"amount_ispay"`
		ProductID   interface{} `json:"product_id"`
		Wallet      string      `json:"wallet"`
	}
	if err := readJSON(ctx, &body); err != nil {
		return biz.ErrInvalidArgument
	}
	in := biz.AdminUserAction{
		Action:    body.Action,
		Locked:    parseBoolish(body.Lock),
		Skip:      parseBoolish(body.SkipUpline),
		AmountFen: parsePriceFen(body.AmountUstd),
		Micro:     parseIspayMicro(body.AmountIspay),
		ProductID: parseInt64(body.ProductID),
		Wallet:    body.Wallet,
	}
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		msg, err := s.uc.ApplyUserAction(c, id, in)
		if err != nil {
			return nil, err
		}
		return msg, nil
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return ctx.Result(200, map[string]interface{}{"ok": "1", "message": out.(string)})
}

func toAdminUser(v *biz.AdminUserView) *adminV1.AdminUser {
	if v == nil || v.User == nil {
		return nil
	}
	u := v.User
	return &adminV1.AdminUser{
		Id: u.ID, WalletAddress: u.WalletAddress, Nickname: u.Nickname, InviteCode: u.InviteCode,
		InviterId: u.InviterID, ParentId: u.ParentID, Side: u.Side, PerfFen: u.PerfFen,
		SettledPairFen: u.SettledPairFen, BalanceFen: v.BalanceFen, IsAdmin: v.IsAdmin,
	}
}

func memberJSON(v *biz.AdminUserView) map[string]interface{} {
	if v == nil || v.User == nil {
		return nil
	}
	u := v.User
	return map[string]interface{}{
		"id":                    u.ID,
		"walletAddress":         u.WalletAddress,
		"nickname":              u.Nickname,
		"inviteCode":            u.InviteCode,
		"inviterId":             u.InviterID,
		"parentId":              u.ParentID,
		"side":                  u.Side,
		"perfFen":               u.PerfFen,
		"createdAt":             biz.FormatUnix(u.CreatedAt),
		"lastOrderAt":           biz.FormatUnix(v.LastOrderAt),
		"packageFen":            v.PackageFen,
		"paidSumFen":            v.PaidSumFen,
		"rechargeFen":           v.RechargeFen,
		"balanceFen":            v.BalanceFen,
		"frozenUsdtFen":         v.FrozenUsdtFen,
		"ispayFreeMicro":        v.IspayFreeMicro,
		"frozenIspayMicro":      v.FrozenIspayMicro,
		"ispayLockedMicro":      v.IspayLockedMicro,
		"staticDays":            v.StaticDays,
		"staticReleasedDays":    v.StaticReleasedDays,
		"staticReleasedUsdtFen": v.StaticReleasedUsdtFen,
		"staticRemainUsdtFen":   v.StaticRemainUsdtFen,
		"staticDailyUsdtFen":    v.StaticDailyUsdtFen,
		"staticFinished":        v.StaticFinished,
		"leftPerfFen":           v.LeftPerfFen,
		"rightPerfFen":          v.RightPerfFen,
		"settledPairFen":        v.SettledPairFen,
		"inviteeCount":          v.InviteeCount,
		"inviterAddress":        v.InviterAddress,
		"locked":                u.Locked,
		"skipUplineReward":      u.SkipUplineReward,
		"isAdmin":               v.IsAdmin,
	}
}

func toTrackMember(u *biz.User) *adminV1.TrackMember {
	if u == nil {
		return nil
	}
	return &adminV1.TrackMember{Id: u.ID, Nickname: u.Nickname, InviteCode: u.InviteCode, PerfFen: u.PerfFen}
}

func toAdminProduct(p *biz.Product) *adminV1.AdminProduct {
	if p == nil {
		return nil
	}
	return &adminV1.AdminProduct{Id: p.ID, Name: p.Name, PriceFen: p.PriceFen, Stock: p.Stock, Status: p.Status}
}

func readJSON(ctx khttp.Context, dest interface{}) error {
	r := ctx.Request()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, dest); err != nil {
		// form login: username= & password=
		ct := r.Header.Get("Content-Type")
		if strings.Contains(ct, "application/x-www-form-urlencoded") {
			_ = r.ParseForm()
			m := map[string]string{
				"username": r.Form.Get("username"),
				"account":  r.Form.Get("account"),
				"password": r.Form.Get("password"),
			}
			raw, _ := json.Marshal(m)
			return json.Unmarshal(raw, dest)
		}
		return err
	}
	return nil
}

func parseInt64(v interface{}) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n
	case int:
		return int64(x)
	case int64:
		return x
	default:
		return 0
	}
}

func parseIspayMicro(v interface{}) int64 {
	var n float64
	switch x := v.(type) {
	case float64:
		n = x
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return 0
		}
		n = f
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return 0
		}
		n = f
	default:
		return 0
	}
	if n <= 0 {
		return 0
	}
	return int64(n * float64(biz.IspayMicroPerCoin))
}

func parseBoolish(v interface{}) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case json.Number:
		n, _ := x.Int64()
		return n != 0
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		return s == "1" || s == "true" || s == "yes"
	default:
		return false
	}
}

func parsePriceFen(v interface{}) int64 {
	switch x := v.(type) {
	case float64:
		return biz.UstdToFen(x)
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return 0
		}
		return biz.UstdToFen(f)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return 0
		}
		return biz.UstdToFen(f)
	default:
		return 0
	}
}

func queryFromCtx(ctx context.Context, key string) string {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	ht, ok := tr.(khttp.Transporter)
	if !ok {
		return ""
	}
	return ht.Request().URL.Query().Get(key)
}
