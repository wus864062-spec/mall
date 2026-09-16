package biz

import (
	"fmt"
	"math"
)

// UstdScale 1 USDT = 10000 内部单位，对应小数点后 4 位。第 5 位起直接丢掉，不四舍五入。
const UstdScale int64 = 10_000

func U(ustd int64) int64 {
	if ustd <= 0 {
		return 0
	}
	return ustd * UstdScale
}

func UstdToFen(ustd float64) int64 {
	if math.IsNaN(ustd) || math.IsInf(ustd, 0) {
		return 0
	}
	x := ustd * float64(UstdScale)
	if x > 0 {
		return int64(math.Floor(x + 1e-9))
	}
	if x < 0 {
		return int64(math.Ceil(x - 1e-9))
	}
	return 0
}

func FenToUstd(fen int64) float64 {
	return float64(fen) / float64(UstdScale)
}

// FormatUstd 给人看 U，不用内部 fen，也不写「分」。只截断到 4 位。
func FormatUstd(fen int64) string {
	sign := ""
	if fen < 0 {
		sign = "-"
		fen = -fen
	}
	return sign + fmt.Sprintf("%d.%04d", fen/UstdScale, fen%UstdScale)
}
