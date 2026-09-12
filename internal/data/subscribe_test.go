package data

import (
	"testing"

	"mall/internal/biz"
)

func TestEnsureSubscribePackages(t *testing.T) {
	d := &Data{products: map[int64]*biz.Product{}}
	if !d.ensureSubscribePackages() {
		t.Fatal("expected seed")
	}
	if len(d.products) != 9 {
		t.Fatalf("got %d products", len(d.products))
	}
	if d.ensureSubscribePackages() {
		t.Fatal("second call should be no-op")
	}
	p := d.products[1]
	if p.Name != "1000U 牙刷挖矿" || p.PriceFen != 100000 || p.CategoryID != biz.CategoryWeb3 {
		t.Fatalf("first package %+v", p)
	}
	last := d.products[9]
	if last.PriceFen != 10000000 || last.Name != "100000U 分布式存储芯片挖矿" || last.Description == "" {
		t.Fatalf("last package %+v", last)
	}
}

func TestMigrateOldSharePackagesToWeb3(t *testing.T) {
	d := &Data{products: map[int64]*biz.Product{
		1: {ID: 1, Name: "1000USTD认购", PriceFen: 100000, Stock: 10, Status: 1},
		2: {ID: 2, Name: "10000USTD认购", PriceFen: 1000000, Stock: 10, Status: 1},
	}}
	if !d.ensureSubscribePackages() {
		t.Fatal("expected migrate")
	}
	if d.products[1].Name != "1000U 牙刷挖矿" || d.products[1].CategoryID != biz.CategoryWeb3 {
		t.Fatalf("1000 migrated %+v", d.products[1])
	}
	if d.products[2].Status != 0 {
		t.Fatalf("old 10000 should be off shelf %+v", d.products[2])
	}
	var got12k bool
	for _, p := range d.products {
		if p.PriceFen == 12000*100 && p.Status == biz.ProductOnSale {
			got12k = true
		}
	}
	if !got12k {
		t.Fatal("missing 12000 package")
	}
}
