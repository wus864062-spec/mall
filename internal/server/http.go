package server

import (
	"net/http"
	"strings"

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
		transhttp.Filter(uploadsFilter(admin.UploadsDir())),
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
	r := srv.Route("/")
	r.POST("/v1/invite/bind", user.HTTPBindInvite)
	r.GET("/v1/invite", user.HTTPGetInvite)
	r.GET("/v1/admin/stats", admin.HTTPStats)
	r.GET("/v1/admin/dashboard", admin.HTTPStats)
	r.POST("/v1/admin/test-data/clear", admin.HTTPClearTestData)

	v1.RegisterGreeterHTTPServer(srv, greeter)
	userV1.RegisterUserServiceHTTPServer(srv, user)
	productV1.RegisterProductServiceHTTPServer(srv, product)
	walletV1.RegisterWalletServiceHTTPServer(srv, wallet)
	orderV1.RegisterOrderServiceHTTPServer(srv, order)
	adminV1.RegisterAdminServiceHTTPServer(srv, admin)
	r.POST("/v1/admin/login", admin.HTTPLogin)
	r.GET("/v1/admin/ledger", admin.HTTPListLedger)
	r.GET("/v1/admin/orders", admin.HTTPListOrders)
	r.GET("/v1/admin/search", admin.HTTPSearch)
	r.GET("/v1/admin/goods", admin.HTTPListGoods)
	r.POST("/v1/admin/products", admin.HTTPCreateProduct)
	r.PUT("/v1/admin/products/{id}/meta", admin.HTTPUpdateProductMeta)
	r.GET("/v1/admin/packages", admin.HTTPListPackages)
	r.POST("/v1/admin/packages", admin.HTTPCreatePackage)
	r.PUT("/v1/admin/packages/{id}", admin.HTTPUpdatePackage)
	r.PUT("/v1/admin/packages/{id}/status", admin.HTTPSetPackageStatus)
	r.POST("/v1/admin/packages/image", admin.HTTPUploadPackageImage)
	r.GET("/v1/admin/config", admin.HTTPGetConfig)
	r.PUT("/v1/admin/config", admin.HTTPUpdateConfig)
	r.GET("/v1/admin/members", admin.HTTPListMembers)
	r.GET("/v1/admin/members/{id}/invitees", admin.HTTPListInvitees)
	r.POST("/v1/admin/users/{id}/action", admin.HTTPUserAction)
	r.GET("/v1/wallet/summary", wallet.HTTPWalletSummary)
	r.GET("/v1/wallet/recharge-ports", wallet.HTTPRechargePorts)
	r.POST("/v1/wallet/recharge", wallet.HTTPRecharge)
	r.GET("/v1/wallet/ledger", wallet.HTTPListLedger)
	r.POST("/v1/wallet/withdraw", wallet.HTTPWithdraw)
	r.GET("/v1/users/need-invite", user.HTTPNeedInvite)
	r.GET("/v1/my/orders", order.HTTPListOrders)
	r.POST("/v1/orders/cart", order.HTTPCreateCart)

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

func uploadsFilter(dir string) func(http.Handler) http.Handler {
	if dir == "" {
		dir = "data/uploads"
	}
	fs := http.StripPrefix("/uploads/", http.FileServer(http.Dir(dir)))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/uploads/") {
				fs.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
