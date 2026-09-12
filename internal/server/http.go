package server

import (
	"net/http"

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
	transhttp "github.com/go-kratos/kratos/v2/transport/http"
)

func NewHTTPServer(
	c *conf.Server,
	greeter *service.GreeterService,
	user *service.UserService,
	product *service.ProductService,
	wallet *service.WalletService,
	order *service.OrderService,
	admin *service.AdminService,
	logger log.Logger,
) *transhttp.Server {
	_ = logger
	var opts = []transhttp.ServerOption{
		transhttp.Filter(corsFilter),
		transhttp.Middleware(
			recovery.Recovery(),
			JWTMiddleware(),
			authMiddleware(),
		),
	}
	if c.Http.Network != "" {
		opts = append(opts, transhttp.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, transhttp.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, transhttp.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := transhttp.NewServer(opts...)

	v1.RegisterGreeterHTTPServer(srv, greeter)
	userV1.RegisterUserServiceHTTPServer(srv, user)
	productV1.RegisterProductServiceHTTPServer(srv, product)
	walletV1.RegisterWalletServiceHTTPServer(srv, wallet)
	orderV1.RegisterOrderServiceHTTPServer(srv, order)
	adminV1.RegisterAdminServiceHTTPServer(srv, admin)
	r := srv.Route("/")
	r.POST("/v1/admin/login", admin.HTTPLogin)
	r.GET("/v1/admin/orders", admin.HTTPListOrders)

	return srv
}

func corsFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
