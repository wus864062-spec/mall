package biz

import "testing"

func TestMergeSplitLedgerStaticAsUSDT(t *testing.T) {
	typ := LedgerStaticReward + "," + LedgerStaticRewardIspay
	in := []*LedgerEntry{
		{UserID: 1, AmountFen: 27777, Type: LedgerStaticReward, RefType: "static", RefID: 1, CreatedAt: 100, WalletAddress: "0xabc"},
		{UserID: 1, AmountFen: 138890, Type: LedgerStaticRewardIspay, RefType: "static", RefID: 1, CreatedAt: 100, WalletAddress: "0xabc"},
	}
	got := MergeSplitLedger(in, "币转U", typ, DefaultIspayPriceFen)
	if len(got) != 1 {
		t.Fatalf("rows %d", len(got))
	}
	if got[0].Type != LedgerSumStatic || got[0].DisplayUnit != "USDT" {
		t.Fatalf("type %s unit %s", got[0].Type, got[0].DisplayUnit)
	}
	if got[0].AmountFen != 55555 {
		t.Fatalf("sum fen %d want 55555 (5.5555U)", got[0].AmountFen)
	}
}

func TestMergeSplitLedgerStaticAsIspay(t *testing.T) {
	typ := LedgerStaticReward + "," + LedgerStaticRewardIspay
	in := []*LedgerEntry{
		{UserID: 1, AmountFen: 27778, Type: LedgerStaticReward, RefType: "static", RefID: 1, CreatedAt: 100},
		{UserID: 1, AmountFen: 138890, Type: LedgerStaticRewardIspay, RefType: "static", RefID: 1, CreatedAt: 100},
	}
	got := MergeSplitLedger(in, "U转币", typ, DefaultIspayPriceFen)
	if len(got) != 1 {
		t.Fatalf("rows %d", len(got))
	}
	if got[0].DisplayUnit != "IsPay" {
		t.Fatalf("unit %s", got[0].DisplayUnit)
	}
	want := MarketFenToIspayMicro(27778, DefaultIspayPriceFen) + 138890
	if got[0].AmountFen != want {
		t.Fatalf("sum micro %d want %d", got[0].AmountFen, want)
	}
}

func TestShouldMergeSplitLedger(t *testing.T) {
	typ := LedgerStaticReward + "," + LedgerStaticRewardIspay
	if !ShouldMergeSplitLedger(typ, "U转币") {
		t.Fatal("should merge")
	}
	if ShouldMergeSplitLedger(typ, "") {
		t.Fatal("全部不合成")
	}
	if ShouldMergeSplitLedger(LedgerDirectReward, "USDT") {
		t.Fatal("直推单项不合成")
	}
}
