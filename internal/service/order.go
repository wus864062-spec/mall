package service

import (
	"context"
	"encoding/json"
	"io"
	"strconv"

	pb "mall/api/order/v1"
	"mall/internal/biz"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

type OrderService struct {
	pb.UnimplementedOrderServiceServer
	uc *biz.OrderUsecase
}

func NewOrderService(uc *biz.OrderUsecase) *OrderService {
	return &OrderService{uc: uc}
}

func (s *OrderService) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.GetOrderReply, error) {
	o, err := s.uc.Create(ctx, req.ProductId, req.Quantity)
	if err != nil {
		return nil, err
	}
	return &pb.GetOrderReply{Order: toProtoOrder(o)}, nil
}

const OperationOrderHTTPCart = "/api.order.v1.OrderService/HTTPCart"

func (s *OrderService) HTTPCreateCart(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationOrderHTTPCart)
	var body struct {
		ProductIDs []int64 `json:"product_ids"`
	}
	b, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return biz.ErrInvalidArgument
	}
	if len(b) > 0 {
		if err := json.Unmarshal(b, &body); err != nil {
			return biz.ErrInvalidArgument
		}
	}
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		return s.uc.CreateCart(c, body.ProductIDs)
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	o := out.(*biz.Order)
	pkg := biz.EffectivePackageFen(o.TotalFen)
	st := s.uc.ShopSettings(ctx)
	return ctx.Result(200, map[string]interface{}{
		"order":      toProtoOrder(o),
		"totalFen":   o.TotalFen,
		"packageFen": pkg,
		"excessFen":  biz.CartExcessFen(o.TotalFen),
		"coinsMicro": o.CoinsMicro,
		"pairCapFen": biz.PairCapFenWith(pkg, st.PairCaps),
	})
}

func (s *OrderService) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderReply, error) {
	o, err := s.uc.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.GetOrderReply{Order: toProtoOrder(o)}, nil
}

func (s *OrderService) ListOrders(ctx context.Context, req *pb.ListOrdersRequest) (*pb.ListOrdersReply, error) {
	list, total, err := s.uc.List(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	var out []*pb.Order
	for _, o := range list {
		out = append(out, toProtoOrder(o))
	}
	return &pb.ListOrdersReply{Orders: out, Total: total}, nil
}

func (s *OrderService) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.GetOrderReply, error) {
	o, err := s.uc.Cancel(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.GetOrderReply{Order: toProtoOrder(o)}, nil
}

func toProtoOrder(o *biz.Order) *pb.Order {
	items := make([]*pb.OrderItem, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, &pb.OrderItem{
			ProductId: it.ProductID, Name: it.Name, PriceFen: it.PriceFen,
			Quantity: it.Quantity, AmountFen: it.AmountFen,
		})
	}
	return &pb.Order{
		Id: o.ID, UserId: o.UserID, Status: o.Status,
		TotalFen: o.TotalFen, Items: items, CreatedAt: o.CreatedAt,
	}
}

const OperationOrderHTTPList = "/api.order.v1.OrderService/HTTPList"

func (s *OrderService) HTTPListOrders(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationOrderHTTPList)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		page, _ := strconv.Atoi(ctx.Query().Get("page"))
		pageSize, _ := strconv.Atoi(ctx.Query().Get("page_size"))
		list, total, err := s.uc.List(c, int32(page), int32(pageSize))
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
	list := pack[0].([]*biz.Order)
	total := pack[1].(int64)
	return ctx.Result(200, map[string]interface{}{"orders": orderRows(list), "total": total})
}

func orderRows(list []*biz.Order) []map[string]interface{} {
	rows := make([]map[string]interface{}, 0, len(list))
	for _, o := range list {
		if o == nil {
			continue
		}
		released := biz.ReleasedCoinsMicro(o.CoinsMicro, o.ReleaseDays, o.ReleasedDays)
		remain := o.CoinsMicro - released
		if remain < 0 {
			remain = 0
		}
		items := make([]map[string]interface{}, 0, len(o.Items))
		for _, it := range o.Items {
			if it == nil {
				continue
			}
			items = append(items, map[string]interface{}{
				"productId": it.ProductID,
				"name":      it.Name,
				"priceFen":  it.PriceFen,
				"quantity":  it.Quantity,
				"amountFen": it.AmountFen,
			})
		}
		rows = append(rows, map[string]interface{}{
			"id":            o.ID,
			"userId":        o.UserID,
			"status":        o.Status,
			"totalFen":      o.TotalFen,
			"createdAt":     o.CreatedAt,
			"categoryId":    o.CategoryID,
			"coinsMicro":    o.CoinsMicro,
			"releaseDays":   o.ReleaseDays,
			"releasedDays":  o.ReleasedDays,
			"releasedMicro": released,
			"remainMicro":   remain,
			"items":         items,
		})
	}
	return rows
}
