package data

import (
	"context"
	"path/filepath"
	"testing"

	"mall/internal/biz"
)

func testData(t *testing.T) *Data {
	t.Helper()
	return &Data{
		path:     filepath.Join(t.TempDir(), "mall.json"),
		users:    map[int64]*biz.User{},
		byWallet: map[string]int64{},
		byInvite: map[string]int64{},
		products: map[int64]*biz.Product{},
		packages: map[int64]*biz.Package{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
	}
}

func TestSeedPackagesFromWeb3Products(t *testing.T) {
	d := testData(t)
	d.products[1] = &biz.Product{
		ID: 1, Name: "1000", Description: "d", PriceFen: biz.U(1000), Status: 1,
		CategoryID: biz.CategoryWeb3, Image: "http://x/a.png",
	}
	d.products[2] = &biz.Product{
		ID: 2, Name: "share", PriceFen: biz.U(1000), Status: 1, CategoryID: biz.CategoryShare,
	}
	d.products[3] = &biz.Product{
		ID: 3, Name: "6000", Description: "e", PriceFen: biz.U(6000), Status: 1,
		CategoryID: biz.CategoryWeb3_600,
	}
	if !d.seedPackagesLocked() {
		t.Fatal("expected seed")
	}
	if d.seedPackagesLocked() {
		t.Fatal("second seed should no-op")
	}
	if len(d.packages) != 2 {
		t.Fatalf("got %d packages", len(d.packages))
	}
	got300 := d.listPackagesLocked(300)
	if len(got300) != 1 || got300[0].AmountFen != biz.U(1000) || got300[0].Image != "http://x/a.png" {
		t.Fatalf("300 %+v", got300)
	}
	if len(d.listPackagesLocked(600)) != 1 {
		t.Fatal("want one 600-day package")
	}
	if len(d.listPackagesLocked(750)) != 0 {
		t.Fatal("750 should be empty")
	}
}

func TestPackageCRUDAndProductSync(t *testing.T) {
	d := testData(t)
	repo := NewAdminRepo(d)
	ctx := context.Background()

	p300, err := repo.CreatePackage(ctx, &biz.Package{
		Name: "A", Description: "da", AmountFen: biz.U(888), ReleaseDays: 300,
		Image: "http://host/uploads/a.png", Status: biz.ProductOnSale,
	})
	if err != nil {
		t.Fatal(err)
	}
	prod := d.products[p300.ProductID]
	if prod == nil || prod.CategoryID != biz.CategoryWeb3 || prod.PriceFen != biz.U(888) || prod.Image != p300.Image {
		t.Fatalf("synced product %+v", prod)
	}

	if _, err := repo.CreatePackage(ctx, &biz.Package{
		Name: "dup", Description: "d", AmountFen: biz.U(888), ReleaseDays: 300, Status: 1,
	}); err != biz.ErrPackageAmountExists {
		t.Fatalf("same days dup %v", err)
	}

	p600, err := repo.CreatePackage(ctx, &biz.Package{
		Name: "B", Description: "db", AmountFen: biz.U(888), ReleaseDays: 600, Status: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.products[p600.ProductID] == nil || d.products[p600.ProductID].CategoryID != biz.CategoryWeb3_600 {
		t.Fatal("600-day product missing")
	}

	list300, err := repo.ListPackages(ctx, 300)
	if err != nil || len(list300) != 1 || list300[0].ID != p300.ID {
		t.Fatalf("list 300 %+v %v", list300, err)
	}
	all, err := repo.ListPackages(ctx, 0)
	if err != nil || len(all) != 2 {
		t.Fatalf("all %+v %v", all, err)
	}

	if _, err := repo.UpdatePackage(ctx, &biz.Package{
		ID: p300.ID, Name: "A2", Description: "da2", AmountFen: biz.U(999), ReleaseDays: 600, Status: 1,
	}, false); err != biz.ErrProductNotFound {
		t.Fatalf("cross-day edit %v", err)
	}

	upd, err := repo.UpdatePackage(ctx, &biz.Package{
		ID: p300.ID, Name: "A2", Description: "da2", AmountFen: biz.U(999), ReleaseDays: 300,
		Image: "http://host/uploads/b.png", Status: 1,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	prod = d.products[upd.ProductID]
	if prod.PriceFen != biz.U(999) || prod.Image != "http://host/uploads/b.png" || prod.Name != "A2" {
		t.Fatalf("updated product %+v", prod)
	}

	off, err := repo.SetPackageStatus(ctx, p300.ID, 300, 0)
	if err != nil || off.Status != 0 {
		t.Fatalf("status %+v %v", off, err)
	}
	if d.products[off.ProductID].Status != 0 {
		t.Fatal("product status not synced")
	}
	if _, err := repo.SetPackageStatus(ctx, p300.ID, 600, 1); err != biz.ErrProductNotFound {
		t.Fatalf("status wrong days %v", err)
	}
}
