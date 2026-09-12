package service

import (
	"context"

	pb "mall/api/product/v1"
	"mall/internal/biz"
)

type ProductService struct {
	pb.UnimplementedProductServiceServer
	uc *biz.ProductUsecase
}

func NewProductService(uc *biz.ProductUsecase) *ProductService {
	return &ProductService{uc: uc}
}

func (s *ProductService) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.GetProductReply, error) {
	p, err := s.uc.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.GetProductReply{Product: toProtoProduct(p)}, nil
}

func (s *ProductService) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ListProductsReply, error) {
	list, total, err := s.uc.List(ctx, req.Page, req.PageSize, req.CategoryId)
	if err != nil {
		return nil, err
	}
	var out []*pb.Product
	for _, p := range list {
		out = append(out, toProtoProduct(p))
	}
	return &pb.ListProductsReply{Products: out, Total: total}, nil
}

func toProtoProduct(p *biz.Product) *pb.Product {
	return &pb.Product{
		Id: p.ID, Name: p.Name, Description: p.Description, PriceFen: p.PriceFen,
		Stock: p.Stock, Image: p.Image, CategoryId: p.CategoryID, Status: p.Status,
	}
}
