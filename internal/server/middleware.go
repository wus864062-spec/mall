package server

import (
	"context"
	"strings"

	adminV1 "mall/api/admin/v1"
	orderV1 "mall/api/order/v1"
	userV1 "mall/api/user/v1"
	walletV1 "mall/api/wallet/v1"
	"mall/internal/biz"
	"mall/internal/service"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport"
)

func authMiddleware() middleware.Middleware {
	return selector.Server(requireLogin()).Match(needLogin).Build()
}

func needLogin(ctx context.Context, operation string) bool {
	switch operation {
	case userV1.OperationUserServiceGetMe,
		userV1.OperationUserServiceUpdateMe,
		userV1.OperationUserServiceLogout,
		userV1.OperationUserServiceGetInvite,
		userV1.OperationUserServiceBindInvite,
		walletV1.OperationWalletServiceGetWallet,
		walletV1.OperationWalletServiceRecharge,
		walletV1.OperationWalletServiceListLedger,
		orderV1.OperationOrderServiceCreateOrder,
		orderV1.OperationOrderServiceGetOrder,
		orderV1.OperationOrderServiceListOrders,
		orderV1.OperationOrderServiceCancelOrder,
		adminV1.OperationAdminServiceGetStats,
		adminV1.OperationAdminServiceListUsers,
		adminV1.OperationAdminServiceGetUser,
		adminV1.OperationAdminServiceListProducts,
		adminV1.OperationAdminServiceUpdateProduct,
		adminV1.OperationAdminServiceListLedger,
		adminV1.OperationAdminServiceForcePairSettle,
		service.OperationAdminHTTPListOrders:
		return true
	default:
		return false
	}
}

func JWTMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				auth := tr.RequestHeader().Get("Authorization")
				if strings.HasPrefix(auth, "Bearer ") {
					token := strings.TrimPrefix(auth, "Bearer ")
					if userID, wallet, ver, err := biz.ParseToken(token); err == nil {
						ctx = biz.WithUserID(ctx, userID)
						ctx = biz.WithWallet(ctx, wallet)
						ctx = biz.WithTokenVer(ctx, ver)
					}
				}
			}
			return handler(ctx, req)
		}
	}
}

func requireLogin() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if _, err := biz.RequireUserID(ctx); err != nil {
				return nil, err
			}
			return handler(ctx, req)
		}
	}
}
