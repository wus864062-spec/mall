package biz

import "testing"

func TestCoinsMicro750(t *testing.T) {
	got := CoinsMicroFromPay(U(12000), CategoryWeb3_750)
	if got != 15*IspayMicroPerCoin {
		t.Fatalf("got %d", got)
	}
}

func TestDailyReleaseLastDayRemainder(t *testing.T) {
	coins := int64(10) * IspayMicroPerCoin
	var sum int64
	for d := int64(0); d < 300; d++ {
		sum += DailyReleaseCoins(coins, 300, d)
	}
	if sum != coins {
		t.Fatalf("sum %d want %d", sum, coins)
	}
}

func TestSplitHalfStatic40U(t *testing.T) {
	daily := DailyReleaseCoins(15*IspayMicroPerCoin, 750, 0)
	if daily != 2_000_000 {
		t.Fatalf("daily coins micro %d", daily)
	}
	value := StaticValueFen(daily, DefaultIspayPriceFen)
	if value != U(40) {
		t.Fatalf("value fen %d want 40U", value)
	}
	usdt, micro := SplitHalf(value, DefaultIspayPriceFen)
	if usdt != U(20) {
		t.Fatalf("usdt %d", usdt)
	}
	if micro != IspayMicroPerCoin/100 {
		t.Fatalf("ispay %d want 0.01", micro)
	}
}

func TestReleasedCoinsSumsToPrincipal(t *testing.T) {
	coins := int64(15) * IspayMicroPerCoin
	if ReleasedCoinsMicro(coins, 750, 0) != 0 {
		t.Fatal("none released")
	}
	day0 := DailyReleaseCoins(coins, 750, 0)
	if ReleasedCoinsMicro(coins, 750, 1) != day0 {
		t.Fatalf("day1 %d", ReleasedCoinsMicro(coins, 750, 1))
	}
	if ReleasedCoinsMicro(coins, 750, 750) != coins {
		t.Fatalf("full %d want %d", ReleasedCoinsMicro(coins, 750, 750), coins)
	}
}

func TestBuildStaticViewDailySplit(t *testing.T) {
	locked := int64(15) * IspayMicroPerCoin
	v := BuildStaticView(locked, 750, 0, U(12000), DefaultIspayPriceFen, 0, 0)
	if v.RemainDays != 750 || v.Finished {
		t.Fatalf("remain %+v", v)
	}
	if v.DailyMicro != 2_000_000 {
		t.Fatalf("daily %d", v.DailyMicro)
	}
	if v.DailyUsdtFen != U(20) {
		t.Fatalf("daily usdt %d", v.DailyUsdtFen)
	}
	if v.RemainMicro != locked {
		t.Fatalf("remain coins %d", v.RemainMicro)
	}
	v1 := BuildStaticView(locked, 750, 1, U(12000), DefaultIspayPriceFen, U(20), IspayMicroPerCoin/100)
	if v1.ReleasedDays != 1 || v1.RemainDays != 749 {
		t.Fatalf("progress %+v", v1)
	}
	if v1.ReleasedMicro != 2_000_000 {
		t.Fatalf("released %d", v1.ReleasedMicro)
	}
	if v1.GotUsdtFen != U(20) {
		t.Fatalf("got %d", v1.GotUsdtFen)
	}
}

func TestFormatIspayUsesCoinsNotFen(t *testing.T) {
	// 展示用枚数：1 枚 = 1e8 micro。不要把 micro 当 fen 去除以 10000。
	if FormatIspay(IspayMicroPerCoin) != "1.00000000" {
		t.Fatalf("1 coin %s", FormatIspay(IspayMicroPerCoin))
	}
	if FormatIspay(30_000_000) != "0.30000000" {
		t.Fatalf("0.3 coin %s", FormatIspay(30_000_000))
	}
	if FormatIspay(-IspayMicroPerCoin/2) != "-0.50000000" {
		t.Fatalf("neg %s", FormatIspay(-IspayMicroPerCoin/2))
	}
	if FormatUstd(U(500)) != "500.0000" {
		t.Fatalf("USDT 用 U 的数字 %s", FormatUstd(U(500)))
	}
}

func TestSplitHalfDirect1200U(t *testing.T) {
	bonus := DirectRewardFen(U(12000))
	if bonus != U(1200) {
		t.Fatalf("bonus %d", bonus)
	}
	usdt, micro := SplitHalf(bonus, DefaultIspayPriceFen)
	if usdt != U(600) {
		t.Fatalf("usdt %d", usdt)
	}
	if micro != 30_000_000 {
		t.Fatalf("ispay %d want 0.3 coin", micro)
	}
}

func TestMarketFenIspayAdminDisplayTrunc(t *testing.T) {
	// 行情 2000U/枚：1000U → 0.5 枚。不是 300 天认购兑换价 1200。
	if ConvertPriceFen(CategoryWeb3) == DefaultIspayPriceFen {
		t.Fatal("market price must differ from lock convert price")
	}
	got := MarketFenToIspayMicro(U(1000), DefaultIspayPriceFen)
	if got != IspayMicroPerCoin/2 {
		t.Fatalf("1000U → %d want 0.5 coin", got)
	}
	if IspayMicroToMarketFen(1, DefaultIspayPriceFen) != 0 {
		t.Fatal("1 micro must truncate to 0 fen")
	}
	if MarketFenToIspayMicro(-U(1000), DefaultIspayPriceFen) != -IspayMicroPerCoin/2 {
		t.Fatal("negative USDT keeps sign")
	}
	back := IspayMicroToMarketFen(IspayMicroPerCoin/2, DefaultIspayPriceFen)
	if back != U(1000) {
		t.Fatalf("0.5 coin → %d want 1000U", back)
	}
}
