package biz

import "strings"

type RechargePort struct {
	Address string `json:"address"`
	Percent int64  `json:"percent"` // 千分比：80% = 800。1.5% 不能用百分整数。
}

// DefaultRechargePorts 与 BuySomething 链上常量一致（用户端不展示，由合约 buy() 拆分）。
// 拆链上打款用这些口和比例；平台入账仍按整 U 1:1，不用这些数。
var DefaultRechargePorts = []RechargePort{
	{Address: "0xa1E54373034aaE3C00Df1b9b89B20d2df55e2CAD", Percent: 800},
	{Address: "0xE7Da6c5D90f6a88fEEa228d5C9a0611c61F7500D", Percent: 100},
	{Address: "0x623ecc54647605C220199F4d273Cf9F43fDdD5c1", Percent: 50},
	{Address: "0x907D9173ab226C698C178981c4D135f8168dD6eb", Percent: 30},
	{Address: "0x279F2B0B788b50c90ceCd74C9134D152083A87B7", Percent: 15},
	{Address: "0xd3E7fE539c291010B8948Fe19372ac9223288109", Percent: 5},
}

const PortPercentScale int64 = 1000

func NormalizeEthAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if len(addr) >= 42 && strings.HasPrefix(strings.ToLower(addr), "0x") {
		return addr[:42]
	}
	return addr
}

func SplitByPorts(total int64, ports []RechargePort) []int64 {
	n := len(ports)
	out := make([]int64, n)
	if n == 0 || total <= 0 {
		return out
	}
	var used int64
	for i := 0; i < n; i++ {
		if i == n-1 {
			out[i] = total - used
			if out[i] < 0 {
				out[i] = 0
			}
			break
		}
		out[i] = total * ports[i].Percent / PortPercentScale
		if out[i] < 0 {
			out[i] = 0
		}
		used += out[i]
	}
	return out
}
