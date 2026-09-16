package data

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mall/internal/biz"
	"mall/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewData, NewGreeterRepo, NewUserRepo, NewProductRepo, NewShopRepo, NewAdminRepo)

type subscribePkg struct {
	USTD   int64
	Name   string
	Detail string
}

var subscribePackages = []subscribePkg{
	{1000, "1000U 牙刷挖矿", ""},
	{2000, "2000U 手表挖矿", ""},
	{3000, "3000U AI眼镜挖矿", ""},
	{6000, "6000U 分布式存储芯片挖矿机", ""},
	{12000, "12000U 分布式存储芯片挖矿 手机挖矿+黄金钻石💎戒指 +多肽", ""},
	{24000, "24000U 分布式存储芯片挖矿 手机挖矿+黄金钻石💎手链+多肽", ""},
	{36000, "36000U 分布式存储芯片挖矿 手机挖矿+黄金钻石💎项链+多肽", ""},
	{50000, "50000U 分布式存储芯片挖矿 手机挖矿+黄金钻石💎项链+多肽", ""},
	{70000, "70000U 分布式存储芯片挖矿 手机挖矿+黄金钻石💎项链+多肽", ""},
	{100000, "100000U 分布式存储芯片挖矿 手机挖矿+黄金钻石💎项链+多肽", ""},
	{160000, "160000U 分布式存储芯片挖矿 手机挖矿+黄金钻石💎项链+多肽", ""},
}

type snapshot struct {
	Users              []*biz.User           `json:"users"`
	UserSeq            int64                 `json:"user_seq"`
	Balances           map[int64]int64       `json:"balances"`
	Ledger             []*biz.LedgerEntry    `json:"ledger"`
	LedgerSeq          int64                 `json:"ledger_seq"`
	Orders             []*biz.Order          `json:"orders"`
	OrderSeq           int64                 `json:"order_seq"`
	Products           []*biz.Product        `json:"products"`
	Packages           []*biz.Package        `json:"packages"`
	PackageSeq         int64                 `json:"package_seq"`
	LastPairSettleDate string                `json:"last_pair_settle_date"`
	IspayLocked        map[int64]int64       `json:"ispay_locked"`
	IspayFree          map[int64]int64       `json:"ispay_free"`
	FrozenUsdt         map[int64]int64       `json:"frozen_usdt"`
	IspayFrozen        map[int64]int64       `json:"ispay_frozen"`
	UnfreezeRemain     map[int64]int64       `json:"unfreeze_remain"`
	FrozenLots         map[int64][]frozenLot `json:"frozen_lots"`
	IspayPriceFen      int64                 `json:"ispay_price_fen"`
	PairCaps           map[string]float64    `json:"pair_caps,omitempty"`
}

type Data struct {
	mu                 sync.RWMutex
	db                 *sql.DB
	path               string
	users              map[int64]*biz.User
	byWallet           map[string]int64
	byInvite           map[string]int64
	userSeq            int64
	products           map[int64]*biz.Product
	packages           map[int64]*biz.Package
	packageSeq         int64
	balances           map[int64]int64
	ledger             []*biz.LedgerEntry
	ledgerSeq          int64
	orders             map[int64]*biz.Order
	orderSeq           int64
	lastPairSettleDate string
	ispayLocked        map[int64]int64
	ispayFree          map[int64]int64
	frozenUsdt         map[int64]int64
	ispayFrozen        map[int64]int64
	unfreezeRemain     map[int64]int64
	frozenLots         map[int64][]frozenLot
	now                time.Time
	ispayPriceFen      int64
	withdrawFeePercent int64
	minWithdrawFen     int64
	maxRechargeFen     int64
	pairCaps           map[int64]int64
	moneyScale         int64
	chainConfig        *biz.ChainConfig
	txByHash           map[string]int64
	stopPair           chan struct{}
}

func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	helper := log.NewHelper(logger)
	path := os.Getenv("MALL_DATA_FILE")
	if path == "" {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		path = filepath.Join(wd, "data", "mall.json")
	}
	d := &Data{
		path:           path,
		users:          make(map[int64]*biz.User),
		byWallet:       make(map[string]int64),
		byInvite:       make(map[string]int64),
		products:       make(map[int64]*biz.Product),
		packages:       make(map[int64]*biz.Package),
		balances:       make(map[int64]int64),
		orders:         make(map[int64]*biz.Order),
		ispayLocked:    make(map[int64]int64),
		ispayFree:      make(map[int64]int64),
		frozenUsdt:     make(map[int64]int64),
		ispayFrozen:    make(map[int64]int64),
		unfreezeRemain: make(map[int64]int64),
		frozenLots:     make(map[int64][]frozenLot),
		chainConfig:    biz.EnsureChainConfig(nil),
		txByHash:       make(map[string]int64),
	}
	dsn := mysqlDSN(c)
	if dsn == "" {
		return nil, nil, fmt.Errorf("未配置 MySQL：请设置 configs 里 data.database.source 或环境变量 MALL_MYSQL_DSN")
	}
	db, err := openMySQL(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("连接 MySQL 失败（15s 超时）: %w", err)
	}
	d.db = db
	if err := d.loadMySQL(); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	if len(d.users) == 0 && len(d.products) == 0 && len(d.orders) == 0 {
		if err := d.load(); err != nil {
			_ = db.Close()
			return nil, nil, err
		}
		if len(d.users) > 0 || len(d.products) > 0 {
			helper.Infof("imported json snapshot into mysql: %s", path)
		}
	}
	scaled := d.migrateMoneyScale()
	d.applySettingsLocked(d.settingsLocked())
	pkgs := d.ensureSubscribePackages()
	seeded := d.seedPackagesLocked()
	locks := d.resyncAllWeb3Locks() // 旧数据：锁仓从「档位」改为「认购总额」
	d.seedFrozenLotsIfNeededLocked()
	invites := d.syncInviteCodesLocked()
	if scaled || pkgs || seeded || locks || invites {
		if err := d.save(); err != nil {
			_ = db.Close()
			return nil, nil, err
		}
	}
	d.stopPair = make(chan struct{})
	d.startPairScheduler(helper)
	helper.Infof("mysql connected (users=%d products=%d orders=%d)", len(d.users), len(d.products), len(d.orders))
	cleanup := func() {
		helper.Info("closing the data resources")
		if d.stopPair != nil {
			close(d.stopPair)
		}
		_ = d.save()
		if d.db != nil {
			_ = d.db.Close()
		}
	}
	return d, cleanup, nil
}

func (d *Data) load() error {
	b, err := os.ReadFile(d.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var s snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	d.userSeq = s.UserSeq
	d.ledgerSeq = s.LedgerSeq
	d.orderSeq = s.OrderSeq
	d.lastPairSettleDate = s.LastPairSettleDate
	d.ispayPriceFen = s.IspayPriceFen
	if len(s.PairCaps) > 0 {
		b, err := json.Marshal(s.PairCaps)
		if err == nil {
			d.pairCaps = biz.DecodePairCapsJSON(string(b))
		}
	}
	if s.IspayLocked != nil {
		d.ispayLocked = s.IspayLocked
	} else {
		d.ispayLocked = map[int64]int64{}
	}
	if s.IspayFree != nil {
		d.ispayFree = s.IspayFree
	} else {
		d.ispayFree = map[int64]int64{}
	}
	if s.FrozenUsdt != nil {
		d.frozenUsdt = s.FrozenUsdt
	} else {
		d.frozenUsdt = map[int64]int64{}
	}
	if s.IspayFrozen != nil {
		d.ispayFrozen = s.IspayFrozen
	} else {
		d.ispayFrozen = map[int64]int64{}
	}
	if s.UnfreezeRemain != nil {
		d.unfreezeRemain = s.UnfreezeRemain
	} else {
		d.unfreezeRemain = map[int64]int64{}
	}
	if s.FrozenLots != nil {
		d.frozenLots = s.FrozenLots
	} else {
		d.frozenLots = map[int64][]frozenLot{}
	}
	d.seedFrozenLotsIfNeededLocked()
	for _, u := range s.Users {
		if u == nil {
			continue
		}
		cp := *u
		d.users[u.ID] = &cp
		d.byWallet[strings.ToLower(u.WalletAddress)] = u.ID
		if u.InviteCode != "" {
			d.byInvite[u.InviteCode] = u.ID
		}
	}
	if s.Balances != nil {
		d.balances = s.Balances
	}
	for _, e := range s.Ledger {
		if e == nil {
			continue
		}
		cp := *e
		d.ledger = append(d.ledger, &cp)
	}
	d.rebuildTxIndexLocked()
	for _, o := range s.Orders {
		if o == nil {
			continue
		}
		d.orders[o.ID] = cloneOrder(o)
	}
	for _, p := range s.Products {
		if p == nil {
			continue
		}
		d.products[p.ID] = cloneProduct(p)
	}
	if d.packages == nil {
		d.packages = map[int64]*biz.Package{}
	}
	for _, p := range s.Packages {
		if p == nil {
			continue
		}
		d.packages[p.ID] = biz.ClonePackage(p)
		if p.ID > d.packageSeq {
			d.packageSeq = p.ID
		}
	}
	if s.PackageSeq > d.packageSeq {
		d.packageSeq = s.PackageSeq
	}
	return nil
}

func (d *Data) reindexInviteLocked() {
	d.byInvite = make(map[string]int64, len(d.users))
	for _, u := range d.users {
		if u == nil || u.InviteCode == "" {
			continue
		}
		d.byInvite[u.InviteCode] = u.ID
	}
}

func (d *Data) syncInviteCodesLocked() bool {
	changed := false
	for _, u := range d.users {
		if u != nil && u.EnsureInviteCode() {
			changed = true
		}
	}
	d.reindexInviteLocked()
	return changed
}

func (d *Data) settingsLocked() biz.SiteSettings {
	return biz.SiteSettings{
		IspayPriceFen:      d.priceFenLocked(),
		WithdrawFeePercent: d.withdrawFeePercent,
		MinWithdrawFen:     d.minWithdrawFen,
		MaxRechargeFen:     d.maxRechargeFen,
		PairCaps:           d.pairCaps,
	}.Normalize()
}

func (d *Data) applySettingsLocked(s biz.SiteSettings) {
	s = s.Normalize()
	d.ispayPriceFen = s.IspayPriceFen
	d.withdrawFeePercent = s.WithdrawFeePercent
	d.minWithdrawFen = s.MinWithdrawFen
	d.maxRechargeFen = s.MaxRechargeFen
	d.pairCaps = biz.ClonePairCaps(s.PairCaps)
}

func (d *Data) highestPairCapLocked(userID int64) int64 {
	return biz.HighestPairCapFenWith(d.orderListLocked(), userID, d.pairCaps)
}

func (d *Data) save() error {
	if d.db != nil {
		return d.saveMySQL()
	}
	if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
		return err
	}
	s := snapshot{
		UserSeq:            d.userSeq,
		Balances:           d.balances,
		LedgerSeq:          d.ledgerSeq,
		OrderSeq:           d.orderSeq,
		LastPairSettleDate: d.lastPairSettleDate,
		IspayLocked:        d.ispayLocked,
		IspayFree:          d.ispayFree,
		FrozenUsdt:         d.frozenUsdt,
		IspayFrozen:        d.ispayFrozen,
		UnfreezeRemain:     d.unfreezeRemain,
		FrozenLots:         d.frozenLots,
		IspayPriceFen:      d.ispayPriceFen,
		PairCaps:           biz.PairCapsUstdMap(d.pairCaps),
		PackageSeq:         d.packageSeq,
	}
	for _, u := range d.users {
		cp := *u
		s.Users = append(s.Users, &cp)
	}
	for _, e := range d.ledger {
		cp := *e
		s.Ledger = append(s.Ledger, &cp)
	}
	for _, o := range d.orders {
		s.Orders = append(s.Orders, cloneOrder(o))
	}
	for _, p := range d.products {
		s.Products = append(s.Products, cloneProduct(p))
	}
	for _, p := range d.packages {
		s.Packages = append(s.Packages, biz.ClonePackage(p))
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := d.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, d.path)
}

func (d *Data) ensureSubscribePackages() bool {
	type catPrice struct {
		cat   int64
		price int64
	}
	byKey := map[catPrice]*biz.Product{}
	keepPrice := map[int64]struct{}{}
	var maxID int64
	for _, p := range d.products {
		if p == nil {
			continue
		}
		if p.ID > maxID {
			maxID = p.ID
		}
		if p.PriceFen > 0 && biz.IsWeb3Category(p.CategoryID) {
			byKey[catPrice{p.CategoryID, p.PriceFen}] = p
		}
	}
	for _, pkg := range subscribePackages {
		keepPrice[pkg.USTD*100] = struct{}{}
	}
	changed := false
	for _, cat := range biz.Web3Categories() {
		for _, pkg := range subscribePackages {
			price := pkg.USTD * biz.UstdScale
			k := catPrice{cat, price}
			if p, ok := byKey[k]; ok {
				if p.Stock < 1 {
					p.Stock = 1_000_000
					changed = true
				}
				if p.Name != pkg.Name || p.Description != pkg.Detail {
					p.Name = pkg.Name
					p.Description = pkg.Detail
					changed = true
				}
				if p.Status != biz.ProductOnSale {
					p.Status = biz.ProductOnSale
					changed = true
				}
				continue
			}
			if cat == biz.CategoryWeb3 {
				var adopted *biz.Product
				for _, p := range d.products {
					if p == nil || p.PriceFen != price {
						continue
					}
					if p.CategoryID == biz.CategoryShare || p.CategoryID == 0 || p.CategoryID == biz.CategoryWeb3 {
						if _, taken := byKey[catPrice{biz.CategoryWeb3, price}]; taken {
							continue
						}
						adopted = p
						break
					}
				}
				if adopted != nil {
					adopted.Name = pkg.Name
					adopted.Description = pkg.Detail
					adopted.CategoryID = biz.CategoryWeb3
					adopted.Status = biz.ProductOnSale
					if adopted.Stock < 1_000_000 {
						adopted.Stock = 1_000_000
					}
					byKey[k] = adopted
					changed = true
					continue
				}
			}
			maxID++
			np := &biz.Product{
				ID:          maxID,
				Name:        pkg.Name,
				Description: pkg.Detail,
				PriceFen:    price,
				Stock:       1_000_000,
				Status:      biz.ProductOnSale,
				CategoryID:  cat,
			}
			d.products[maxID] = np
			byKey[k] = np
			changed = true
		}
	}
	for _, p := range d.products {
		if p == nil || p.PriceFen <= 0 {
			continue
		}
		if _, keep := keepPrice[p.PriceFen]; keep {
			continue
		}
		if p.CategoryID == biz.CategoryShare || strings.Contains(p.Name, "USTD认购") || strings.Contains(p.Name, "USDT认购") {
			if p.Status == biz.ProductOnSale {
				p.Status = 0
				changed = true
			}
		}
	}
	return changed
}

func cloneUser(u *biz.User) *biz.User {
	if u == nil {
		return nil
	}
	cp := *u
	return &cp
}

func cloneProduct(p *biz.Product) *biz.Product {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

func cloneLedger(e *biz.LedgerEntry) *biz.LedgerEntry {
	if e == nil {
		return nil
	}
	cp := *e
	return &cp
}

func (d *Data) rebuildTxIndexLocked() {
	d.txByHash = map[string]int64{}
	for _, e := range d.ledger {
		if e == nil {
			continue
		}
		h := biz.NormalizeTxHash(e.TxHash)
		if h == "" {
			continue
		}
		d.txByHash[h] = e.ID
	}
}

func cloneOrder(o *biz.Order) *biz.Order {
	if o == nil {
		return nil
	}
	cp := *o
	cp.Items = make([]*biz.OrderItem, 0, len(o.Items))
	for _, it := range o.Items {
		if it == nil {
			continue
		}
		item := *it
		cp.Items = append(cp.Items, &item)
	}
	return &cp
}

func slicePage[T any](all []T, page, pageSize int32) ([]T, int64) {
	total := int64(len(all))
	start := (page - 1) * pageSize
	if start >= int32(total) {
		return []T{}, total
	}
	end := start + pageSize
	if end > int32(total) {
		end = int32(total)
	}
	return all[start:end], total
}
