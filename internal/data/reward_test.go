package data

import (
	"context"
	"testing"

	"mall/internal/biz"
)

func TestPlaceOrderPaysDirectReward(t *testing.T) {
	d := &Data{
		users:    map[int64]*biz.User{},
		products: map[int64]*biz.Product{},
		balances: map[int64]int64{},
		orders:   map[int64]*biz.Order{},
		path:     t.TempDir() + "/mall.json",
	}
	d.users[1] = &biz.User{ID: 1, Nickname: "inviter"}
	d.users[2] = &biz.User{ID: 2, Nickname: "invitee", InviterID: 1}
	d.products[1] = &biz.Product{ID: 1, Name: "1000USTD认购", PriceFen: biz.U(1000), Stock: 10}
	d.balances[2] = biz.U(1000)

	shop := NewShopRepo(d)
	o, err := shop.PlaceOrder(context.Background(), 2, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if o.TotalFen != biz.U(1000) {
		t.Fatalf("total %d", o.TotalFen)
	}
	if d.balances[2] != 0 {
		t.Fatalf("buyer bal %d", d.balances[2])
	}
	want, _ := biz.SplitHalf(biz.DirectRewardFen(biz.U(1000)), biz.DefaultIspayPriceFen)
	if d.frozenUsdt[1] != want {
		t.Fatalf("inviter frozen %d want %d", d.frozenUsdt[1], want)
	}
	if d.balances[1] != 0 {
		t.Fatalf("inviter withdrawable %d", d.balances[1])
	}
	found := false
	for _, e := range d.ledger {
		if e.UserID == 1 && e.Type == biz.LedgerDirectReward && e.AmountFen == want && e.RefID == o.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("missing direct reward ledger")
	}

	if _, err := shop.CancelOrder(context.Background(), 2, o.ID); err != nil {
		t.Fatal(err)
	}
	if d.frozenUsdt[1] != 0 || d.balances[1] != 0 {
		t.Fatalf("clawback frozen %d bal %d", d.frozenUsdt[1], d.balances[1])
	}
}
