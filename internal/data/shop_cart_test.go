package data

import (
	"context"
	"testing"

	"mall/internal/biz"
)

func TestPlaceCartSumsToTierAndLockFromTotal(t *testing.T) {
	d := &Data{
		users:       map[int64]*biz.User{1: {ID: 1}},
		products:    map[int64]*biz.Product{},
		balances:    map[int64]int64{1: biz.U(4000)},
		ispayLocked: map[int64]int64{},
		orders:      map[int64]*biz.Order{},
		path:        t.TempDir() + "/mall.json",
	}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.products[2] = &biz.Product{ID: 2, Name: "3000", PriceFen: biz.U(3000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	shop := NewShopRepo(d)
	o, err := shop.PlaceCart(context.Background(), 1, []int64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if o.TotalFen != biz.U(4000) || len(o.Items) != 2 {
		t.Fatalf("order %+v items=%d", o, len(o.Items))
	}
	if d.users[1].StaticPackageFen != biz.U(3000) {
		t.Fatalf("tier %d", d.users[1].StaticPackageFen)
	}
	wantLock := biz.CoinsMicroFromPay(biz.U(4000), biz.CategoryWeb3)
	if d.ispayLocked[1] != wantLock {
		t.Fatalf("lock %d want %d (must use paid sum, not tier)", d.ispayLocked[1], wantLock)
	}
	if biz.CartExcessFen(o.TotalFen) != biz.U(1000) {
		t.Fatalf("excess %d", biz.CartExcessFen(o.TotalFen))
	}
}

func TestPlaceCartUpgradesWhenSumHitsNextTier(t *testing.T) {
	d := &Data{
		users:       map[int64]*biz.User{1: {ID: 1}},
		products:    map[int64]*biz.Product{},
		balances:    map[int64]int64{1: biz.U(6000)},
		ispayLocked: map[int64]int64{},
		orders:      map[int64]*biz.Order{},
		path:        t.TempDir() + "/mall.json",
	}
	d.products[1] = &biz.Product{ID: 1, Name: "3000a", PriceFen: biz.U(3000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.products[2] = &biz.Product{ID: 2, Name: "3000b", PriceFen: biz.U(3000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	shop := NewShopRepo(d)
	o, err := shop.PlaceCart(context.Background(), 1, []int64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if o.TotalFen != biz.U(6000) {
		t.Fatalf("total %d", o.TotalFen)
	}
	if d.users[1].StaticPackageFen != biz.U(6000) {
		t.Fatalf("tier %d", d.users[1].StaticPackageFen)
	}
	if biz.CartExcessFen(o.TotalFen) != 0 {
		t.Fatalf("excess %d", biz.CartExcessFen(o.TotalFen))
	}
	wantLock := biz.CoinsMicroFromPay(biz.U(6000), biz.CategoryWeb3)
	if d.ispayLocked[1] != wantLock {
		t.Fatalf("lock %d want %d", d.ispayLocked[1], wantLock)
	}
}

func TestPlaceCartRejectsMixedDays(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{1: {ID: 1}},
		products: map[int64]*biz.Product{},
		balances: map[int64]int64{1: biz.U(10000)},
		orders:   map[int64]*biz.Order{},
		path:     t.TempDir() + "/mall.json",
	}
	d.products[1] = &biz.Product{ID: 1, Name: "1000", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3}
	d.products[2] = &biz.Product{ID: 2, Name: "1000-600", PriceFen: biz.U(1000), Stock: 10, Status: 1, CategoryID: biz.CategoryWeb3_600}
	shop := NewShopRepo(d)
	if _, err := shop.PlaceCart(context.Background(), 1, []int64{1, 2}); err != biz.ErrCartMixedDays {
		t.Fatalf("got %v", err)
	}
}
