package biz

import (
	kerrors "github.com/go-kratos/kratos/v2/errors"
)

var (
	ErrForbidden           = kerrors.Forbidden("FORBIDDEN", "需要管理员权限")
	ErrUnauthorized        = kerrors.Unauthorized("UNAUTHORIZED", "请先登录")
	ErrInvalidToken        = kerrors.Unauthorized("UNAUTHORIZED", "无效的登录凭证")
	ErrInvalidArgument     = kerrors.BadRequest("INVALID_ARGUMENT", "请求参数无效")
	ErrUserNotExist        = kerrors.NotFound("USER_NOT_FOUND", "用户不存在")
	ErrProductNotFound     = kerrors.NotFound("PRODUCT_NOT_FOUND", "商品不存在")
	ErrInsufficientStock   = kerrors.BadRequest("INSUFFICIENT_STOCK", "库存不足")
	ErrInsufficientBalance = kerrors.BadRequest("INSUFFICIENT_BALANCE", "余额不足")
	ErrOrderNotFound       = kerrors.NotFound("ORDER_NOT_FOUND", "订单不存在")
	ErrOrderNotCancellable = kerrors.BadRequest("ORDER_NOT_CANCELLABLE", "认购后不可自行取消")
	ErrInviteAlreadyBound  = kerrors.BadRequest("INVITE_ALREADY_BOUND", "已绑定邀请人，不能更改")
	ErrInviteInvalid       = kerrors.BadRequest("INVITE_INVALID", "邀请码无效")
	ErrInviteSelf          = kerrors.BadRequest("INVITE_SELF", "不能邀请自己")
)
