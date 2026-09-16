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
	if len(d.products) != 33 {
		t.Fatalf("got %d products", len(d.products))
	}
	if d.ensureSubscribePackages() {
		t.Fatal("second call should be no-op")
	}
	p := d.products[1]
	if p.Name != "1000U 牙刷挖矿" || p.PriceFen != biz.U(1000) || p.CategoryID != biz.CategoryWeb3 {
		t.Fatalf("first package %+v", p)
	}
	var got2000, got160 bool
	for _, x := range d.products {
		if x.PriceFen == biz.U(2000) && x.CategoryID == biz.CategoryWeb3 && x.Name == "2000U 手表挖矿" {
			got2000 = true
		}
		if x.PriceFen == biz.U(160000) && x.CategoryID == biz.CategoryWeb3 && x.Name == "160000U 分布式存储芯片挖矿 手机挖矿+黄金钻石💎项链+多肽" {
			got160 = true
		}
	}
	if !got2000 {
		t.Fatal("missing 2000 watch package")
	}
	if !got160 {
		t.Fatal("missing 160000 package")
	}
	counts := map[int64]int{}
	for _, x := range d.products {
		counts[x.CategoryID]++
	}
	for _, cat := range biz.Web3Categories() {
		if counts[cat] != 11 {
			t.Fatalf("category %d got %d", cat, counts[cat])
		}
	}
}

func TestMigrateOldSharePackagesToWeb3(t *testing.T) {
	d := &Data{products: map[int64]*biz.Product{
		1: {ID: 1, Name: "1000USTD认购", PriceFen: biz.U(1000), Stock: 10, Status: 1},
		2: {ID: 2, Name: "10000USTD认购", PriceFen: biz.U(10000), Stock: 10, Status: 1},
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
		if p.PriceFen == biz.U(12000) && p.Status == biz.ProductOnSale {
			got12k = true
		}
	}
	if !got12k {
		t.Fatal("missing 12000 package")
	}
}
