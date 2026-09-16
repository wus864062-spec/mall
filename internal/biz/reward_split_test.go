package biz

import "testing"

func TestCompleteRewardSplitUsesOwnNative(t *testing.T) {
	s := RewardSplit{TodayFen: U(30), TodayIspayMicro: 100_000_000, Fen: U(30), IspayMicro: 100_000_000}
	CompleteRewardSplit(&s, DefaultIspayPriceFen)
	wantU := U(30) + IspayMicroToMarketFen(100_000_000, DefaultIspayPriceFen)
	wantI := int64(100_000_000) + MarketFenToIspayMicro(U(30), DefaultIspayPriceFen)
	if s.TodayTotalFen != wantU || s.TotalFen != wantU {
		t.Fatalf("总U=本类U+币转U %d %d want %d", s.TodayTotalFen, s.TotalFen, wantU)
	}
	if s.TodayTotalIspayMicro != wantI || s.TotalIspayMicro != wantI {
		t.Fatalf("总币=本类币+U转币 %d %d want %d", s.TodayTotalIspayMicro, s.TotalIspayMicro, wantI)
	}
	s.AddUSDT(U(10), false)
	s.AddIspay(50_000_000, true)
	if s.Fen != U(40) || s.TodayFen != U(30) {
		t.Fatalf("累计含往日，今日U不加往日 %d today %d", s.Fen, s.TodayFen)
	}
	if s.IspayMicro != 150_000_000 || s.TodayIspayMicro != 150_000_000 {
		t.Fatalf("今日币累加 %d %d", s.IspayMicro, s.TodayIspayMicro)
	}
}
