package data

import (
	"fmt"
	"strings"
	"time"

	"mall/internal/biz"
)

// ClearTestData 清空测试用户与账本，保留商品目录、套餐、链上配置和站点设置。
// 后台登录账号重建为 ID=1 / wallet=admin，不沿用原用户余额、订单、邀请位。
func (d *Data) ClearTestData() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.users = map[int64]*biz.User{}
	d.byWallet = map[string]int64{}
	d.byInvite = map[string]int64{}
	d.balances = map[int64]int64{}
	d.ledger = nil
	d.ledgerSeq = 0
	d.orders = map[int64]*biz.Order{}
	d.orderSeq = 0
	d.lastPairSettleDate = ""
	d.ispayLocked = map[int64]int64{}
	d.ispayFree = map[int64]int64{}
	d.frozenUsdt = map[int64]int64{}
	d.ispayFrozen = map[int64]int64{}
	d.unfreezeRemain = map[int64]int64{}
	d.frozenLots = map[int64][]frozenLot{}
	d.txByHash = map[string]int64{}
	d.ensureAssetMaps()
	admin := &biz.User{
		ID:            1,
		WalletAddress: "admin",
		Nickname:      "admin",
		InviteCode:    "admin",
		CreatedAt:     time.Now().Unix(),
	}
	d.users[1] = admin
	d.byWallet["admin"] = 1
	d.byInvite["admin"] = 1
	d.userSeq = 1
	return d.save()
}

func (d *Data) KeepOnlyUser(id int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	keep := d.users[id]
	if keep == nil {
		return fmt.Errorf("user %d not found", id)
	}
	cp := *keep
	cp.ID = 1
	cp.InviterID = 0
	cp.ParentID = 0
	cp.Side = ""
	cp.LeftID = 0
	cp.RightID = 0
	cp.ChainNextID = 0
	cp.OccupiedPos = 0
	cp.InviteCode = cp.WalletAddress
	bal := d.balances[id]
	locked := d.ispayLocked[id]
	free := d.ispayFree[id]
	frozenUsdt := d.frozenUsdt[id]
	frozenIspay := d.ispayFrozen[id]
	d.users = map[int64]*biz.User{1: &cp}
	d.byWallet = map[string]int64{strings.ToLower(cp.WalletAddress): 1}
	d.byInvite = map[string]int64{cp.InviteCode: 1}
	d.userSeq = 1
	d.balances = map[int64]int64{1: bal}
	d.ispayLocked = map[int64]int64{1: locked}
	d.ispayFree = map[int64]int64{1: free}
	d.frozenUsdt = map[int64]int64{1: frozenUsdt}
	d.ispayFrozen = map[int64]int64{1: frozenIspay}
	d.unfreezeRemain = map[int64]int64{1: d.unfreezeRemain[id]}
	d.frozenLots = map[int64][]frozenLot{1: append([]frozenLot(nil), d.frozenLots[id]...)}
	var ledger []*biz.LedgerEntry
	for _, e := range d.ledger {
		if e == nil || e.UserID != id {
			continue
		}
		ne := *e
		ne.UserID = 1
		ledger = append(ledger, &ne)
	}
	d.ledger = ledger
	d.rebuildTxIndexLocked()
	orders := map[int64]*biz.Order{}
	for oid, o := range d.orders {
		if o == nil || o.UserID != id {
			continue
		}
		oc := cloneOrder(o)
		oc.UserID = 1
		orders[oid] = oc
	}
	d.orders = orders
	return d.save()
}
