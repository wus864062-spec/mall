package data

import (
	"context"
	"path/filepath"
	"testing"

	"mall/internal/biz"
)

func TestShopWithdraw(t *testing.T) {
	d := &Data{
		path:     filepath.Join(t.TempDir(), "mall.json"),
		users:    map[int64]*biz.User{1: {ID: 1}},
		byWallet: map[string]int64{},
		byInvite: map[string]int64{},
		products: map[int64]*biz.Product{},
		balances: map[int64]int64{1: 5000},
		orders:   map[int64]*biz.Order{},
	}
	repo := NewShopRepo(d)
	ctx := context.Background()
	if _, err := repo.Withdraw(ctx, 1, 6000); err != biz.ErrInsufficientBalance {
		t.Fatalf("want insufficient, got %v", err)
	}
	e, err := repo.Withdraw(ctx, 1, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if e.Type != biz.LedgerWithdraw || e.AmountFen != -2000 || e.BalanceFen != 3000 {
		t.Fatalf("entry %+v", e)
	}
	bal, _ := repo.GetBalance(ctx, 1)
	if bal != 3000 {
		t.Fatalf("balance %d", bal)
	}
}
