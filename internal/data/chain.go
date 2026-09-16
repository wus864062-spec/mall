package data

import (
	"context"
	"time"

	"mall/internal/biz"
)

func (d *Data) chainConfigCloneLocked() *biz.ChainConfig {
	return biz.EnsureChainConfig(d.chainConfig)
}

func (d *Data) moneyFlowLocked(typ string) biz.MoneyFlow {
	loc := pairSettleLocation()
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).Unix()
	var f biz.MoneyFlow
	for _, e := range d.ledger {
		if e == nil || e.Type != typ {
			continue
		}
		amt := e.AmountFen
		if amt < 0 {
			amt = -amt
		}
		f.TotalFen += amt
		f.TotalCount++
		if e.CreatedAt >= start {
			f.TodayFen += amt
			f.TodayCount++
		}
	}
	return f
}

func (r *shopRepo) GetChainConfig(ctx context.Context) *biz.ChainConfig {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	return r.data.chainConfigCloneLocked()
}

func (r *shopRepo) SetChainConfig(ctx context.Context, cfg *biz.ChainConfig) error {
	n, err := biz.NormalizeChainConfig(cfg)
	if err != nil {
		return err
	}
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	r.data.chainConfig = n
	return r.data.save()
}

func (r *adminRepo) GetChainConfig(ctx context.Context) *biz.ChainConfig {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	return r.data.chainConfigCloneLocked()
}

func (r *adminRepo) SetChainConfig(ctx context.Context, cfg *biz.ChainConfig) error {
	n, err := biz.NormalizeChainConfig(cfg)
	if err != nil {
		return err
	}
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	r.data.chainConfig = n
	return r.data.save()
}

func (r *adminRepo) MoneyFlows(ctx context.Context) (biz.MoneyFlow, biz.MoneyFlow) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	return r.data.moneyFlowLocked(biz.LedgerRecharge), r.data.moneyFlowLocked(biz.LedgerWithdraw)
}
