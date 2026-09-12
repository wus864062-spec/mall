package server

import (
	adminV1 "mall/api/admin/v1"
	v1 "mall/api/mall/v1"
	orderV1 "mall/api/order/v1"
	productV1 "mall/api/product/v1"
	userV1 "mall/api/user/v1"
	walletV1 "mall/api/wallet/v1"
	"mall/internal/conf"
	"mall/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

func NewGRPCServer(
	c *conf.Server,
	greeter *service.GreeterService,
	user *service.UserService,
	product *service.ProductService,
	wallet *service.WalletService,
	order *service.OrderService,
	admin *service.AdminService,
	logger log.Logger,
) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
			JWTMiddleware(),
			authMiddleware(),
		),
	}
	if c.Grpc.Network != "" {
		opts = append(opts, grpc.Network(c.Grpc.Network))
	}
	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}
	srv := grpc.NewServer(opts...)

	v1.RegisterGreeterServer(srv, greeter)
	userV1.RegisterUserServiceServer(srv, user)
	productV1.RegisterProductServiceServer(srv, product)
	walletV1.RegisterWalletServiceServer(srv, wallet)
	orderV1.RegisterOrderServiceServer(srv, order)
	adminV1.RegisterAdminServiceServer(srv, admin)

	return srv
}
