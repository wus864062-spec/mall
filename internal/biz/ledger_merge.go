package biz

import (
	"sort"
	"strings"
)

const (
	LedgerSumStatic  = "sum_static"
	LedgerSumDynamic = "sum_dynamic"
	LedgerSumReward  = "sum_reward"
)

func typeSetKey(typ string) string {
	var parts []string
	for _, p := range strings.Split(typ, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func combinedStaticKey() string {
	return typeSetKey(LedgerStaticReward + "," + LedgerStaticRewardIspay)
}

func combinedDynamicKey() string {
	return typeSetKey(strings.Join([]string{
		LedgerDirectReward, LedgerDirectRewardIspay, LedgerDirectRewardClawback, LedgerDirectRewardIspayClawback,
		LedgerPairReward, LedgerPairRewardIspay, LedgerPairRewardClawback, LedgerPairUpline,
		LedgerManageReward, LedgerManageRewardIspay,
	}, ","))
}

func combinedRewardKey() string {
	return typeSetKey(combinedStaticKey() + "," + combinedDynamicKey())
}

func CombinedSumType(typ string) string {
	k := typeSetKey(typ)
	switch k {
	case combinedStaticKey():
		return LedgerSumStatic
	case combinedDynamicKey():
		return LedgerSumDynamic
	case combinedRewardKey():
		return LedgerSumReward
	default:
		return ""
	}
}

func IsCombinedRewardTypes(typ string) bool {
	return CombinedSumType(typ) != ""
}

// ShouldMergeSplitLedger 三项合计且选了币种时，把同一笔折半的 U / IsPay 合成一行。
func ShouldMergeSplitLedger(typ, unit string) bool {
	if !IsCombinedRewardTypes(typ) {
		return false
	}
	switch strings.TrimSpace(unit) {
	case "USDT", "IsPay", "U转币", "币转U":
		return true
	default:
		return false
	}
}

func mergeAsIspay(unit string) bool {
	return unit == "IsPay" || unit == "U转币"
}

type splitMergeKey struct {
	UserID    int64
	CreatedAt int64
	RefID     int64
	RefType   string
}

func entryToMergeUnit(e *LedgerEntry, asIspay bool, priceFen int64) int64 {
	if e == nil {
		return 0
	}
	if IsIspayAmountType(e.Type) {
		if asIspay {
			return e.AmountFen
		}
		return IspayMicroToMarketFen(e.AmountFen, priceFen)
	}
	if asIspay {
		return MarketFenToIspayMicro(e.AmountFen, priceFen)
	}
	return e.AmountFen
}

// MergeSplitLedger 同一用户、同一 ref、同一秒的折半流水合成一条。
// asIspay（IsPay/U转币）按行情把 U 折进枚数；否则（USDT/币转U）把枚数折进 U。用 IspayPriceFen，不用认购兑换价。
func MergeSplitLedger(in []*LedgerEntry, unit, typ string, priceFen int64) []*LedgerEntry {
	sumType := CombinedSumType(typ)
	if sumType == "" || len(in) == 0 {
		return in
	}
	asIspay := mergeAsIspay(unit)
	display := "USDT"
	if asIspay {
		display = "IsPay"
	}
	order := make([]splitMergeKey, 0)
	seen := map[splitMergeKey]bool{}
	sum := map[splitMergeKey]int64{}
	first := map[splitMergeKey]*LedgerEntry{}
	for _, e := range in {
		if e == nil {
			continue
		}
		k := splitMergeKey{UserID: e.UserID, CreatedAt: e.CreatedAt, RefID: e.RefID, RefType: e.RefType}
		if !seen[k] {
			seen[k] = true
			order = append(order, k)
			cp := *e
			first[k] = &cp
		}
		sum[k] += entryToMergeUnit(e, asIspay, priceFen)
	}
	out := make([]*LedgerEntry, 0, len(order))
	for _, k := range order {
		e := first[k]
		e.AmountFen = sum[k]
		e.Type = sumType
		e.DisplayUnit = display
		e.BalanceFen = 0
		out = append(out, e)
	}
	return out
}
