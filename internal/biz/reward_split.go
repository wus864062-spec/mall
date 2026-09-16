package biz

// RewardSplit 一类动态奖的八个数。USDT=原币；总U=U+币转U；总币=币+U转币。含回退，不含另外两类、不含静态。
type RewardSplit struct {
	TodayFen             int64 // 今日USDT原币
	TodayIspayMicro      int64 // 今日IsPay原币
	TodayTotalFen        int64 // 今日总USDT=U+币转U，行情价
	TodayTotalIspayMicro int64 // 今日总IsPay=币+U转币，行情价
	Fen                  int64 // 累计USDT原币，含往日含回退
	IspayMicro           int64 // 累计IsPay原币
	TotalFen             int64 // 累计U+币；文件夹上用这个数
	TotalIspayMicro      int64 // 累计币+U
}

func (s *RewardSplit) AddUSDT(amt int64, today bool) {
	if s == nil {
		return
	}
	s.Fen += amt
	if today {
		s.TodayFen += amt
	}
}

func (s *RewardSplit) AddIspay(amt int64, today bool) {
	if s == nil {
		return
	}
	s.IspayMicro += amt
	if today {
		s.TodayIspayMicro += amt
	}
}

func CompleteRewardSplit(s *RewardSplit, priceFen int64) {
	if s == nil {
		return
	}
	s.TodayTotalFen = s.TodayFen + IspayMicroToMarketFen(s.TodayIspayMicro, priceFen)
	s.TodayTotalIspayMicro = s.TodayIspayMicro + MarketFenToIspayMicro(s.TodayFen, priceFen)
	s.TotalFen = s.Fen + IspayMicroToMarketFen(s.IspayMicro, priceFen)
	s.TotalIspayMicro = s.IspayMicro + MarketFenToIspayMicro(s.Fen, priceFen)
}
