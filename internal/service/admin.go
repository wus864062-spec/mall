package service

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"

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
	if addr := queryFromCtx(ctx, "address"); addr != "" && req.UserId == 0 {
		if id, err := strconv.ParseInt(addr, 10, 64); err == nil {
			req.UserId = id
		}
	}
	list, total, err := s.uc.ListLedger(ctx, req.Page, req.PageSize, req.UserId, typ)
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

func (s *AdminService) ForcePairSettle(ctx context.Context, req *adminV1.ForcePairSettleRequest) (*adminV1.ForcePairSettleReply, error) {
	day, err := s.uc.ForcePairSettle(ctx)
	if err != nil {
		return nil, err
	}
	return &adminV1.ForcePairSettleReply{LastPairSettleDate: day}, nil
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
		orders, users, total, err := s.uc.ListOrders(c, int32(page), int32(pageSize), q)
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
			"createdAt": biz.FormatUnix(o.CreatedAt),
			"amount":    float64(o.TotalFen) / 100,
			"address":   addr,
			"one":       name,
			"status":    o.Status,
		})
	}
	return ctx.Result(200, map[string]interface{}{"rewards": rows, "orders": rows, "count": total, "total": total})
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
