# Data

持久化与结算入口。领域规则在 [`internal/biz/README.md`](../biz/README.md)。

锁仓对齐：`ispay.go` 的 `syncWeb3LockLocked`（下单/取消）和启动时的 `resyncAllWeb3Locks`。
