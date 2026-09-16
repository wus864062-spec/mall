package data

import (
	"os"
	"testing"

	"mall/internal/biz"
	"mall/internal/conf"
)

func TestMySQLSaveLoad(t *testing.T) {
	dsn := os.Getenv("MALL_MYSQL_DSN")
	if dsn == "" {
		dsn = "root:123456@tcp(127.0.0.1:3306)/mall_test?charset=utf8mb4&parseTime=True&loc=Local"
	}
	db, err := openMySQL(dsn)
	if err != nil {
		t.Skip("mysql not available: ", err)
	}
	_ = db.Close()

	d := &Data{
		users:    map[int64]*biz.User{},
		byWallet: map[string]int64{},
		byInvite: map[string]int64{},
		products: map[int64]*biz.Product{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
	}
	d.db, err = openMySQL(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer d.db.Close()
	d.userSeq = 2
	d.users[2] = &biz.User{ID: 2, WalletAddress: "0xabc", Nickname: "B", InviteCode: "INVB", PerfFen: 1000}
	d.balances[2] = 5000
	d.products[1] = &biz.Product{ID: 1, Name: "p", PriceFen: 100000, Stock: 9, Status: 1, CategoryID: 3}
	d.packages = map[int64]*biz.Package{}
	d.packages[1] = &biz.Package{ID: 1, Name: "pkg", Description: "d", AmountFen: 100000, ReleaseDays: 300, Image: "http://x/a.png", Status: 1, ProductID: 1}
	if err := d.saveMySQL(); err != nil {
		t.Fatal(err)
	}
	d2 := &Data{
		db:       d.db,
		users:    map[int64]*biz.User{},
		byWallet: map[string]int64{},
		byInvite: map[string]int64{},
		products: map[int64]*biz.Product{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
	}
	if err := d2.loadMySQL(); err != nil {
		t.Fatal(err)
	}
	if d2.userSeq != 2 || d2.users[2] == nil || d2.users[2].InviteCode != "INVB" || d2.balances[2] != 5000 {
		t.Fatalf("loaded %+v bal=%v", d2.users[2], d2.balances)
	}
	if d2.products[1] == nil || d2.products[1].PriceFen != 100000 {
		t.Fatalf("product %+v", d2.products[1])
	}
	if d2.packages[1] == nil || d2.packages[1].ReleaseDays != 300 || d2.packages[1].Image != "http://x/a.png" {
		t.Fatalf("package %+v", d2.packages[1])
	}
}

func TestMySQLDSNFromConfig(t *testing.T) {
	if got := mysqlDSN(&conf.Data{Database: &conf.Data_Database{Source: "root:root@tcp(127.0.0.1:3306)/mall"}}); got == "" {
		t.Fatal("expected dsn")
	}
}
