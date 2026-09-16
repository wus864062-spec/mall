package data

import (
	"context"
	"testing"

	"mall/internal/biz"
)

func TestClearTestDataWipesUsersKeepsCatalog(t *testing.T) {
	d := &Data{
		path:        t.TempDir() + "/mall.json",
		users:       map[int64]*biz.User{},
		byWallet:    map[string]int64{},
		byInvite:    map[string]int64{},
		products:    map[int64]*biz.Product{},
		packages:    map[int64]*biz.Package{},
		balances:    map[int64]int64{},
		orders:      map[int64]*biz.Order{},
		ispayLocked: map[int64]int64{},
		ispayFree:   map[int64]int64{},
		frozenUsdt:  map[int64]int64{},
		ispayFrozen: map[int64]int64{},
		frozenLots:  map[int64][]frozenLot{},
		txByHash:    map[string]int64{"0xabc": 1},
	}
	d.users[1] = &biz.User{ID: 1, WalletAddress: "0xaaa", InviteRoot: true, PerfFen: biz.U(1000)}
	d.users[2] = &biz.User{ID: 2, WalletAddress: "0xbbb", InviterID: 1, ParentID: 1, Side: biz.TrackLeft}
	d.byWallet["0xaaa"] = 1
	d.byWallet["0xbbb"] = 2
	d.userSeq = 2
	d.balances[1] = biz.U(500)
	d.balances[2] = biz.U(3000)
	d.ledger = []*biz.LedgerEntry{{ID: 1, UserID: 2, AmountFen: biz.U(3000), Type: biz.LedgerRecharge}}
	d.ledgerSeq = 1
	d.orders[1] = &biz.Order{ID: 1, UserID: 2, Status: biz.OrderPaid, TotalFen: biz.U(3000)}
	d.orderSeq = 1
	d.products[9] = &biz.Product{ID: 9, Name: "1000", PriceFen: biz.U(1000), Status: 1}
	d.packages[3] = &biz.Package{ID: 3, Name: "1000", AmountFen: biz.U(1000)}
	d.packageSeq = 3
	d.ispayPriceFen = biz.U(2000)
	d.lastPairSettleDate = "2026-09-15"

	if err := NewAdminRepo(d).ClearTestData(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(d.users) != 1 || d.users[1] == nil || d.users[1].WalletAddress != "admin" {
		t.Fatalf("want admin stub, got %+v", d.users)
	}
	if d.users[1].InviteRoot || d.users[1].InviterID != 0 || d.balances[1] != 0 {
		t.Fatalf("admin stub must be empty %+v bal=%d", d.users[1], d.balances[1])
	}
	if len(d.orders) != 0 || len(d.ledger) != 0 || d.userSeq != 1 {
		t.Fatalf("wiped ledger/orders seq=%d", d.userSeq)
	}
	if d.products[9] == nil || d.packages[3] == nil {
		t.Fatal("catalog must stay")
	}
	if d.ispayPriceFen != biz.U(2000) {
		t.Fatalf("settings price %d", d.ispayPriceFen)
	}
	if d.lastPairSettleDate != "" || len(d.txByHash) != 0 {
		t.Fatalf("settle/tx leftover %q %v", d.lastPairSettleDate, d.txByHash)
	}
	st, err := NewAdminRepo(d).Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.UserCount != 1 || st.PaidOrderFen != 0 || st.ActivatedCount != 0 {
		t.Fatalf("stats after clear %+v", st)
	}
}
