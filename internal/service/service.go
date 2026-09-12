package service

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewGreeterService,
	NewUserService,
	NewProductService,
	NewWalletService,
	NewOrderService,
	NewAdminService,
)
