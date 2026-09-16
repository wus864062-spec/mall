package biz

import "testing"

func TestSplitByPortsCurrentShare(t *testing.T) {
	// 拆款用千分比 800/100/50/30/15，最后一口吃余数（0.5%）。不用旧的 89/5/5/1。
	parts := SplitByPorts(10000, DefaultRechargePorts)
	if len(parts) != 6 {
		t.Fatalf("%v", parts)
	}
	if parts[0] != 8000 || parts[1] != 1000 || parts[2] != 500 || parts[3] != 300 || parts[4] != 150 || parts[5] != 50 {
		t.Fatalf("%v", parts)
	}
	var sum int64
	for _, p := range parts {
		sum += p
	}
	if sum != 10000 {
		t.Fatalf("sum %d", sum)
	}
}

func TestNormalizeLongAddress(t *testing.T) {
	raw := "0x45d8cb58c330b559b430fe7c7298ea7f2ca8a8065527c2efaaefa101a99708b5"
	if got := NormalizeEthAddress(raw); got != "0xE7Da6c5D90f6a88fEEa228d5C9a0611c61F7500D" {
		t.Fatalf("%s", got)
	}
}
