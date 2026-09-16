package biz

import (
	"encoding/json"
	"strconv"
	"strings"
)

type SiteSettings struct {
	IspayPriceFen      int64
	WithdrawFeePercent int64
	MinWithdrawFen     int64
	MaxRechargeFen     int64
	// PairCaps 档位 fen → 日封顶 fen。发对碰/管理奖和解冻额度用这个数，不用默认表。
	PairCaps map[int64]int64
}

func DefaultSiteSettings() SiteSettings {
	return SiteSettings{
		IspayPriceFen:      DefaultIspayPriceFen,
		WithdrawFeePercent: WithdrawFeePercent,
		MinWithdrawFen:     MinWithdrawFen,
		MaxRechargeFen:     MaxRechargeFen,
		PairCaps:           DefaultPairCaps(),
	}
}

func (s SiteSettings) Normalize() SiteSettings {
	out := DefaultSiteSettings()
	if s.IspayPriceFen > 0 {
		out.IspayPriceFen = s.IspayPriceFen
	}
	if s.WithdrawFeePercent >= 0 && s.WithdrawFeePercent <= 100 {
		out.WithdrawFeePercent = s.WithdrawFeePercent
	}
	if s.MinWithdrawFen > 0 {
		out.MinWithdrawFen = s.MinWithdrawFen
	}
	if s.MaxRechargeFen > 0 {
		out.MaxRechargeFen = s.MaxRechargeFen
	}
	out.PairCaps = MergePairCaps(s.PairCaps)
	return out
}

func WithdrawFeeAmount(amount, percent int64) int64 {
	if amount <= 0 || percent <= 0 {
		return 0
	}
	return amount * percent / 100
}

func PairCapConfigTiers() []int64 {
	out := make([]int64, 0, len(catalogPackageFen)+1)
	inserted := false
	hist := U(10000)
	for _, p := range catalogPackageFen {
		if !inserted && hist > 0 && p > hist && IsPairCapTier(hist) {
			out = append(out, hist)
			inserted = true
		}
		out = append(out, p)
	}
	if !inserted && IsPairCapTier(hist) {
		found := false
		for _, p := range catalogPackageFen {
			if p == hist {
				found = true
				break
			}
		}
		if !found {
			out = append(out, hist)
		}
	}
	return out
}

func PairCapConfigID(priceFen int64) string {
	return "pair_cap_" + strconv.FormatInt(priceFen/UstdScale, 10)
}

func PairCapConfigName(priceFen int64) string {
	u := priceFen / UstdScale
	if priceFen == U(10000) {
		return "日封顶 10000U(历史订单)"
	}
	return "日封顶 " + strconv.FormatInt(u, 10) + "U(USDT)"
}

func ParsePairCapConfigID(id string) (int64, bool) {
	if !strings.HasPrefix(id, "pair_cap_") {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimPrefix(id, "pair_cap_"), 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	price := U(n)
	if !IsPairCapTier(price) {
		return 0, false
	}
	return price, true
}

func PairCapsUstdMap(overlay map[int64]int64) map[string]float64 {
	merged := MergePairCaps(overlay)
	out := make(map[string]float64, len(merged))
	for k, v := range merged {
		if k <= 0 {
			continue
		}
		out[strconv.FormatInt(k/UstdScale, 10)] = FenToUstd(v)
	}
	return out
}

func EncodePairCapsJSON(overlay map[int64]int64) string {
	b, err := json.Marshal(PairCapsUstdMap(overlay))
	if err != nil {
		return "{}"
	}
	return string(b)
}

func DecodePairCapsJSON(s string) map[int64]int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var raw map[string]float64
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil
	}
	out := make(map[int64]int64, len(raw))
	for k, v := range raw {
		n, err := strconv.ParseInt(k, 10, 64)
		if err != nil || n <= 0 {
			continue
		}
		price := U(n)
		if !IsPairCapTier(price) {
			continue
		}
		if v < 0 {
			v = 0
		}
		out[price] = UstdToFen(v)
	}
	return out
}
