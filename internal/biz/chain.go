package biz

type ChainConfig struct {
	BuyAddress      string
	UsdtAddress     string
	WithdrawAddress string
}

func EnsureChainConfig(c *ChainConfig) *ChainConfig {
	if c == nil {
		return &ChainConfig{}
	}
	cp := *c
	return &cp
}

func NormalizeChainConfig(c *ChainConfig) (*ChainConfig, error) {
	return EnsureChainConfig(c), nil
}

type MoneyFlow struct {
	TotalFen   int64
	TotalCount int64
	TodayFen   int64
	TodayCount int64
}
