package biz

import "testing"

func TestSiteSettingsNormalize(t *testing.T) {
	d := DefaultSiteSettings()
	if d.WithdrawFeePercent != 10 || d.MinWithdrawFen != U(10) || d.MaxRechargeFen != U(200000) {
		t.Fatalf("%+v", d)
	}
	got := SiteSettings{WithdrawFeePercent: 0, MinWithdrawFen: 500, MaxRechargeFen: 10000, IspayPriceFen: 300000}.Normalize()
	if got.WithdrawFeePercent != 0 || got.MinWithdrawFen != 500 || got.MaxRechargeFen != 10000 || got.IspayPriceFen != 300000 {
		t.Fatalf("%+v", got)
	}
	if WithdrawFeeAmount(2000, 8) != 160 {
		t.Fatal("8% fee")
	}
	if WithdrawFeeAmount(2000, 0) != 0 {
		t.Fatal("zero fee")
	}
}

func TestPairCapsJSONRoundTrip(t *testing.T) {
	over := map[int64]int64{U(1000): U(400), U(2000): 0}
	got := DecodePairCapsJSON(EncodePairCapsJSON(over))
	if got[U(1000)] != U(400) {
		t.Fatalf("1000 %d", got[U(1000)])
	}
	if got[U(2000)] != 0 {
		t.Fatal("2000 配置 0 要保住")
	}
	if MergePairCaps(got)[U(3000)] != U(1800) {
		t.Fatal("没写的档回默认")
	}
	if _, ok := ParsePairCapConfigID("pair_cap_1000"); !ok {
		t.Fatal("pair_cap_1000")
	}
	if _, ok := ParsePairCapConfigID("pair_cap_999"); ok {
		t.Fatal("非目录档")
	}
	n := DefaultSiteSettings().Normalize()
	if n.PairCaps[U(1000)] != U(600) {
		t.Fatal("默认设置带 1000→600")
	}
	n.PairCaps[U(1000)] = U(400)
	if n.Normalize().PairCaps[U(1000)] != U(400) {
		t.Fatal("Normalize 保留覆盖")
	}
}
