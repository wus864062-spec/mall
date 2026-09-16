package biz

import "fmt"

const (
	IspayMicroPerCoin int64 = 100_000_000
	// DefaultIspayPriceFen 行情单价，用于释放/分红折 U；与认购锁仓兑换价不是同一个数。
	DefaultIspayPriceFen  int64 = 2000 * UstdScale
	ConvertPrice300Fen    int64 = 1200 * UstdScale // 300 天认购：1200U 换 1 枚
	ConvertPrice600Fen    int64 = 1000 * UstdScale
	ConvertPrice750Fen    int64 = 800 * UstdScale
	MinWithdrawIspayMicro int64 = 10 * IspayMicroPerCoin
	AssetUSDT                   = "usdt"
	AssetIspay                  = "ispay"
)

const (
	LedgerIspayConvert              = "ispay_convert"
	LedgerIspayConvertClawback      = "ispay_convert_clawback"
	LedgerStaticReward              = "static_reward"
	LedgerStaticRewardIspay         = "static_reward_ispay"
	LedgerDirectRewardIspay         = "direct_reward_ispay"
	LedgerDirectRewardIspayClawback = "direct_reward_ispay_clawback"
	LedgerPairRewardIspay           = "pair_reward_ispay"
	LedgerManageRewardIspay         = "manage_reward_ispay"
	LedgerIspayWithdraw             = "ispay_withdraw"
	LedgerUnfreeze                  = "unfreeze"
	LedgerUnfreezeIspay             = "unfreeze_ispay"
	LedgerFrozenExpire              = "frozen_expire"
	LedgerFrozenExpireIspay         = "frozen_expire_ispay"
)

func IsIspayAmountType(t string) bool {
	switch t {
	case LedgerIspayConvert, LedgerIspayConvertClawback, LedgerStaticRewardIspay,
		LedgerDirectRewardIspay, LedgerDirectRewardIspayClawback,
		LedgerPairRewardIspay, LedgerManageRewardIspay, LedgerIspayWithdraw,
		LedgerAdminIspay, LedgerUnfreezeIspay, LedgerFrozenExpireIspay:
		return true
	default:
		return false
	}
}

type WalletView struct {
	BalanceFen            int64
	RechargeFen           int64 // 充值页 USDT：充值−认购−提现剩余，不算奖励/调账
	FrozenUsdtFen         int64
	IspayLockedMicro      int64
	IspayFreeMicro        int64
	FrozenIspayMicro      int64
	IspayPriceFen         int64
	MaxRechargeFen        int64
	MinWithdrawFen        int64
	WithdrawFeePercent    int64
	StaticPackageFen      int64
	StaticDays            int64
	StaticReleasedDays    int64
	StaticRemainDays      int64
	StaticDailyMicro      int64
	StaticDailyUsdtFen    int64
	StaticDailyIspayMicro int64
	StaticReleasedMicro   int64
	StaticRemainMicro     int64
	StaticReleasedUsdtFen int64
	StaticRemainUsdtFen   int64
	StaticGotUsdtFen      int64
	StaticGotIspayMicro   int64
	DirectUsdtFen         int64
	DirectIspayMicro      int64
	PairUsdtFen           int64
	PairIspayMicro        int64
	ManageUsdtFen         int64
	ManageIspayMicro      int64
	StaticFinished        bool
	PairCapsUstd          map[string]float64 // 档位 USDT → 日封顶 USDT，给用户端展示
}

type StaticView struct {
	PackageFen      int64
	Days            int64
	ReleasedDays    int64
	RemainDays      int64
	DailyMicro      int64
	DailyUsdtFen    int64
	DailyIspayMicro int64
	ReleasedMicro   int64
	RemainMicro     int64
	ReleasedUsdtFen int64
	RemainUsdtFen   int64
	GotUsdtFen      int64
	GotIspayMicro   int64
	Finished        bool
}

func ReleaseDaysByCategory(cat int64) int64 {
	switch cat {
	case CategoryWeb3:
		return 300
	case CategoryWeb3_600:
		return 600
	case CategoryWeb3_750:
		return 750
	default:
		return 0
	}
}

func CategoryByReleaseDays(days int64) (int64, bool) {
	switch days {
	case 300:
		return CategoryWeb3, true
	case 600:
		return CategoryWeb3_600, true
	case 750:
		return CategoryWeb3_750, true
	default:
		return 0, false
	}
}

func ValidReleaseDays(days int64) bool {
	_, ok := CategoryByReleaseDays(days)
	return ok
}

// ConvertPriceFen 认购锁仓兑换价（USDT/枚），由 Web3 品类天数决定。
func ConvertPriceFen(cat int64) int64 {
	switch cat {
	case CategoryWeb3:
		return ConvertPrice300Fen
	case CategoryWeb3_600:
		return ConvertPrice600Fen
	case CategoryWeb3_750:
		return ConvertPrice750Fen
	default:
		return 0
	}
}

// CoinsMicroFromPay 按认购实付 USDT 折锁仓 IsPay（档位折 + 超额折，同兑换价即总额折）。
// totalFen 用已付合计（Web3PaidSumFen 或单笔实付），不要只传入档位。
// 只做整数除法，余数丢掉。
func CoinsMicroFromPay(totalFen, cat int64) int64 {
	price := ConvertPriceFen(cat)
	if price <= 0 || totalFen <= 0 {
		return 0
	}
	return totalFen * IspayMicroPerCoin / price
}

func DailyReleaseCoins(coinsMicro, days, releasedDays int64) int64 {
	if days <= 0 || coinsMicro <= 0 || releasedDays >= days {
		return 0
	}
	daily := coinsMicro / days
	if releasedDays == days-1 {
		return coinsMicro - daily*(days-1)
	}
	return daily
}

func ReleasedCoinsMicro(coinsMicro, days, releasedDays int64) int64 {
	if coinsMicro <= 0 || days <= 0 || releasedDays <= 0 {
		return 0
	}
	n := releasedDays
	if n > days {
		n = days
	}
	var sum int64
	for i := int64(0); i < n; i++ {
		sum += DailyReleaseCoins(coinsMicro, days, i)
	}
	return sum
}

func BuildStaticView(locked, days, releasedDays, packageFen, priceFen, gotUsdt, gotIspay int64) StaticView {
	v := StaticView{
		PackageFen:    packageFen,
		Days:          days,
		ReleasedDays:  releasedDays,
		GotUsdtFen:    gotUsdt,
		GotIspayMicro: gotIspay,
	}
	if v.ReleasedDays < 0 {
		v.ReleasedDays = 0
	}
	if days > 0 {
		if v.ReleasedDays > days {
			v.ReleasedDays = days
		}
		v.RemainDays = days - v.ReleasedDays
		v.Finished = v.RemainDays == 0
	}
	v.DailyMicro = DailyReleaseCoins(locked, days, v.ReleasedDays)
	dailyValue := StaticValueFen(v.DailyMicro, priceFen)
	v.DailyUsdtFen, v.DailyIspayMicro = SplitHalf(dailyValue, priceFen)
	v.ReleasedMicro = ReleasedCoinsMicro(locked, days, v.ReleasedDays)
	if locked > v.ReleasedMicro {
		v.RemainMicro = locked - v.ReleasedMicro
	}
	releasedValue := StaticValueFen(v.ReleasedMicro, priceFen)
	v.ReleasedUsdtFen, _ = SplitHalf(releasedValue, priceFen)
	remainValue := StaticValueFen(v.RemainMicro, priceFen)
	v.RemainUsdtFen, _ = SplitHalf(remainValue, priceFen)
	return v
}

func (w *WalletView) ApplyStatic(s StaticView) {
	if w == nil {
		return
	}
	w.StaticPackageFen = s.PackageFen
	w.StaticDays = s.Days
	w.StaticReleasedDays = s.ReleasedDays
	w.StaticRemainDays = s.RemainDays
	w.StaticDailyMicro = s.DailyMicro
	w.StaticDailyUsdtFen = s.DailyUsdtFen
	w.StaticDailyIspayMicro = s.DailyIspayMicro
	w.StaticReleasedMicro = s.ReleasedMicro
	w.StaticRemainMicro = s.RemainMicro
	w.StaticReleasedUsdtFen = s.ReleasedUsdtFen
	w.StaticRemainUsdtFen = s.RemainUsdtFen
	w.StaticGotUsdtFen = s.GotUsdtFen
	w.StaticGotIspayMicro = s.GotIspayMicro
	w.StaticFinished = s.Finished
}

func StaticValueFen(dailyCoins, priceFen int64) int64 {
	if dailyCoins <= 0 || priceFen <= 0 {
		return 0
	}
	return dailyCoins * priceFen / IspayMicroPerCoin
}

// ReconstructSplitValueFen 从折半后的 USDT fen + IsPay micro 还原折前奖励（截断误差可能少 1 fen）。
func ReconstructSplitValueFen(usdtFen, ispayMicro, priceFen int64) int64 {
	if usdtFen < 0 {
		usdtFen = -usdtFen
	}
	if ispayMicro < 0 {
		ispayMicro = -ispayMicro
	}
	return usdtFen + StaticValueFen(ispayMicro, priceFen)
}

func SplitHalf(valueFen, priceFen int64) (usdtFen, ispayMicro int64) {
	if valueFen <= 0 || priceFen <= 0 {
		return 0, 0
	}
	usdtFen = valueFen / 2
	coinValue := valueFen - usdtFen
	ispayMicro = coinValue * IspayMicroPerCoin / priceFen
	return
}

func NormalizeIspayPriceFen(priceFen int64) int64 {
	if priceFen <= 0 {
		return DefaultIspayPriceFen
	}
	return priceFen
}

// MarketFenToIspayMicro 后台「U转币」：把 USDT fen 按行情单价折 IsPay。用 IspayPriceFen，不用认购 ConvertPriceFen。只截断。
func MarketFenToIspayMicro(fen, priceFen int64) int64 {
	priceFen = NormalizeIspayPriceFen(priceFen)
	if fen == 0 {
		return 0
	}
	return fen * IspayMicroPerCoin / priceFen
}

// IspayMicroToMarketFen 后台「币转U」：把 IsPay micro 按行情单价折 USDT fen。用 IspayPriceFen，不用认购 ConvertPriceFen。只截断。
func IspayMicroToMarketFen(micro, priceFen int64) int64 {
	priceFen = NormalizeIspayPriceFen(priceFen)
	if micro == 0 {
		return 0
	}
	return micro * priceFen / IspayMicroPerCoin
}

func MicroToCoins(micro int64) float64 {
	return float64(micro) / float64(IspayMicroPerCoin)
}

// FormatIspay 给人看枚数。用 micro/1e8，不用 fen，也不写「分」。只截断到 8 位。
func FormatIspay(micro int64) string {
	sign := ""
	if micro < 0 {
		sign = "-"
		micro = -micro
	}
	return sign + fmt.Sprintf("%d.%08d", micro/IspayMicroPerCoin, micro%IspayMicroPerCoin)
}
