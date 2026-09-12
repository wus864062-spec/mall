package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
	{3000, "3000U AI眼镜挖矿", ""},
	{6000, "6000U 分布式存储芯片挖矿机", ""},
	{12000, "12000U 分布式存储芯片挖矿", "手机挖矿+黄金钻石💎戒指 +多肽"},
	{24000, "24000U 分布式存储芯片挖矿", "手机挖矿+黄金钻石💎手链+多肽"},
	{36000, "36000U 分布式存储芯片挖矿", "手机挖矿+黄金钻石💎项链+多肽"},
	{50000, "50000U 分布式存储芯片挖矿", "手机挖矿+黄金钻石💎项链+多肽"},
	{70000, "70000U 分布式存储芯片挖矿", "手机挖矿+黄金钻石💎项链+多肽"},
	{100000, "100000U 分布式存储芯片挖矿", "手机挖矿+黄金钻石💎项链+多肽"},
}

type snapshot struct {
	Users              []*biz.User        `json:"users"`
	UserSeq            int64              `json:"user_seq"`
	Balances           map[int64]int64    `json:"balances"`
	Ledger             []*biz.LedgerEntry `json:"ledger"`
	LedgerSeq          int64              `json:"ledger_seq"`
	Orders             []*biz.Order       `json:"orders"`
	OrderSeq           int64              `json:"order_seq"`
	Products           []*biz.Product     `json:"products"`
	LastPairSettleDate string             `json:"last_pair_settle_date"`
}

type Data struct {
	mu                 sync.RWMutex
	path               string
	users              map[int64]*biz.User
	byWallet           map[string]int64
	byInvite           map[string]int64
	userSeq            int64
	products           map[int64]*biz.Product
	balances           map[int64]int64
	ledger             []*biz.LedgerEntry
	ledgerSeq          int64
	orders             map[int64]*biz.Order
	orderSeq           int64
	lastPairSettleDate string
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
		path:     path,
		users:    make(map[int64]*biz.User),
		byWallet: make(map[string]int64),
		byInvite: make(map[string]int64),
		products: make(map[int64]*biz.Product),
		balances: make(map[int64]int64),
		orders:   make(map[int64]*biz.Order),
	}
	if err := d.load(); err != nil {
		return nil, nil, err
	}
	if d.ensureSubscribePackages() {
		_ = d.save()
	}
	d.stopPair = make(chan struct{})
	d.startPairScheduler(helper)
	helper.Infof("data file: %s (users=%d products=%d orders=%d)", path, len(d.users), len(d.products), len(d.orders))
	cleanup := func() {
		helper.Info("closing the data resources")
		if d.stopPair != nil {
			close(d.stopPair)
		}
		_ = d.save()
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
	for _, u := range s.Users {
		if u == nil {
			continue
		}
		cp := *u
		d.users[u.ID] = &cp
		d.byWallet[u.WalletAddress] = u.ID
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
	return nil
}

func (d *Data) save() error {
	if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
		return err
	}
	s := snapshot{
		UserSeq:            d.userSeq,
		Balances:           d.balances,
		LedgerSeq:          d.ledgerSeq,
		OrderSeq:           d.orderSeq,
		LastPairSettleDate: d.lastPairSettleDate,
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
	byPrice := map[int64]*biz.Product{}
	keepPrice := map[int64]struct{}{}
	var maxID int64
	for _, p := range d.products {
		if p == nil {
			continue
		}
		if p.ID > maxID {
			maxID = p.ID
		}
		if p.PriceFen > 0 {
			byPrice[p.PriceFen] = p
		}
	}
	changed := false
	for _, pkg := range subscribePackages {
		price := pkg.USTD * 100
		keepPrice[price] = struct{}{}
		if p, ok := byPrice[price]; ok {
			if p.Name != pkg.Name || p.Description != pkg.Detail || p.CategoryID != biz.CategoryWeb3 || p.Status != biz.ProductOnSale {
				p.Name = pkg.Name
				p.Description = pkg.Detail
				p.CategoryID = biz.CategoryWeb3
				p.Status = biz.ProductOnSale
				if p.Stock < 1_000_000 {
					p.Stock = 1_000_000
				}
				changed = true
			}
			continue
		}
		maxID++
		d.products[maxID] = &biz.Product{
			ID:          maxID,
			Name:        pkg.Name,
			Description: pkg.Detail,
			PriceFen:    price,
			Stock:       1_000_000,
			Status:      biz.ProductOnSale,
			CategoryID:  biz.CategoryWeb3,
		}
		changed = true
	}
	for _, p := range d.products {
		if p == nil || p.PriceFen <= 0 {
			continue
		}
		if _, keep := keepPrice[p.PriceFen]; keep {
			continue
		}
		if p.CategoryID == biz.CategoryShare || strings.Contains(p.Name, "USTD认购") {
			if p.Status == biz.ProductOnSale || p.CategoryID != biz.CategoryWeb3 {
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
