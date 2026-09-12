package service

import (
	"context"

	pb "mall/api/order/v1"
	"mall/internal/biz"
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
