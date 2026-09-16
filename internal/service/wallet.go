package service

import (
	"context"
	"encoding/json"
	"io"
	"strconv"

	pb "mall/api/wallet/v1"
	"mall/internal/biz"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

type WalletService struct {
	pb.UnimplementedWalletServiceServer
	uc *biz.WalletUsecase
}

func NewWalletService(uc *biz.WalletUsecase) *WalletService {
	return &WalletService{uc: uc}
}

func (s *WalletService) GetWallet(ctx context.Context, req *pb.GetWalletRequest) (*pb.GetWalletReply, error) {
	view, err := s.uc.GetWalletView(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetWalletReply{BalanceFen: view.BalanceFen}, nil
}

const OperationWalletHTTPSummary = "/api.wallet.v1.WalletService/HTTPSummary"

func (s *WalletService) HTTPWalletSummary(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationWalletHTTPSummary)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		return s.uc.GetWalletView(c)
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	v := out.(*biz.WalletView)
	return ctx.Result(200, map[string]interface{}{
		"balanceFen":            v.BalanceFen,
		"rechargeFen":           v.RechargeFen, // 充值页：充值−认购−提现，不算奖励
		"frozenUsdtFen":         v.FrozenUsdtFen,
		"ispayLockedMicro":      v.IspayLockedMicro,
		"ispayFreeMicro":        v.IspayFreeMicro,
		"frozenIspayMicro":      v.FrozenIspayMicro,
		"ispayPriceFen":         v.IspayPriceFen,
		"maxRechargeFen":        v.MaxRechargeFen,
		"minWithdrawFen":        v.MinWithdrawFen,
		"withdrawFeePercent":    v.WithdrawFeePercent,
		"staticPackageFen":      v.StaticPackageFen,
		"staticDays":            v.StaticDays,
		"staticReleasedDays":    v.StaticReleasedDays,
		"staticRemainDays":      v.StaticRemainDays,
		"staticDailyMicro":      v.StaticDailyMicro,
		"staticDailyUsdtFen":    v.StaticDailyUsdtFen,
		"staticDailyIspayMicro": v.StaticDailyIspayMicro,
		"staticReleasedMicro":   v.StaticReleasedMicro,
		"staticRemainMicro":     v.StaticRemainMicro,
		"staticReleasedUsdtFen": v.StaticReleasedUsdtFen,
		"staticRemainUsdtFen":   v.StaticRemainUsdtFen,
		"staticGotUsdtFen":      v.StaticGotUsdtFen,
		"staticGotIspayMicro":   v.StaticGotIspayMicro,
		"directUsdtFen":         v.DirectUsdtFen,
		"directIspayMicro":      v.DirectIspayMicro,
		"pairUsdtFen":           v.PairUsdtFen,
		"pairIspayMicro":        v.PairIspayMicro,
		"manageUsdtFen":         v.ManageUsdtFen,
		"manageIspayMicro":      v.ManageIspayMicro,
		"staticFinished":        v.StaticFinished,
		"pairCaps":              v.PairCapsUstd,
	})
}

func (s *WalletService) Recharge(ctx context.Context, req *pb.RechargeRequest) (*pb.RechargeReply, error) {
	return nil, biz.ErrRechargeTxRequired
}

func (s *WalletService) ListLedger(ctx context.Context, req *pb.ListLedgerRequest) (*pb.ListLedgerReply, error) {
	list, total, err := s.uc.ListLedger(ctx, req.Page, req.PageSize, "")
	if err != nil {
		return nil, err
	}
	var out []*pb.LedgerEntry
	for _, e := range list {
		out = append(out, toProtoLedger(e))
	}
	return &pb.ListLedgerReply{Entries: out, Total: total}, nil
}

const OperationWalletHTTPRecharge = "/api.wallet.v1.WalletService/HTTPRecharge"

func (s *WalletService) HTTPRecharge(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationWalletHTTPRecharge)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		var body struct {
			AmountFen int64  `json:"amount_fen"`
			TxHash    string `json:"tx_hash"`
		}
		b, err := io.ReadAll(ctx.Request().Body)
		if err != nil {
			return nil, biz.ErrInvalidArgument
		}
		if len(b) > 0 {
			if err := json.Unmarshal(b, &body); err != nil {
				return nil, biz.ErrInvalidArgument
			}
		}
		e, bal, err := s.uc.Recharge(c, body.AmountFen, body.TxHash)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"balanceFen": bal,
			"entry":      toProtoLedger(e),
		}, nil
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return ctx.Result(200, out)
}

const OperationWalletHTTPLedger = "/api.wallet.v1.WalletService/HTTPListLedger"

func (s *WalletService) HTTPListLedger(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationWalletHTTPLedger)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		page, _ := strconv.Atoi(ctx.Query().Get("page"))
		pageSize, _ := strconv.Atoi(ctx.Query().Get("page_size"))
		typ := ctx.Query().Get("type")
		list, total, err := s.uc.ListLedger(c, int32(page), int32(pageSize), typ)
		if err != nil {
			return nil, err
		}
		return []interface{}{list, total}, nil
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	pack := out.([]interface{})
	list := pack[0].([]*biz.LedgerEntry)
	total := pack[1].(int64)
	return ctx.Result(200, map[string]interface{}{
		"entries": ledgerJSON(list),
		"total":   total,
	})
}

const OperationWalletHTTPWithdraw = "/api.wallet.v1.WalletService/HTTPWithdraw"

func (s *WalletService) HTTPWithdraw(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationWalletHTTPWithdraw)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		var body struct {
			AmountFen   int64  `json:"amount_fen"`
			AmountMicro int64  `json:"amount_micro"`
			Asset       string `json:"asset"`
		}
		b, err := io.ReadAll(ctx.Request().Body)
		if err != nil {
			return nil, biz.ErrInvalidArgument
		}
		if len(b) > 0 {
			if err := json.Unmarshal(b, &body); err != nil {
				return nil, biz.ErrInvalidArgument
			}
		}
		asset := body.Asset
		amount := body.AmountFen
		if asset == biz.AssetIspay {
			amount = body.AmountMicro
			if amount == 0 {
				amount = body.AmountFen
			}
		}
		e, bal, err := s.uc.WithdrawAsset(c, amount, asset)
		if err != nil {
			return nil, err
		}
		st := s.uc.ShopSettings(c)
		fee := biz.WithdrawFeeAmount(amount, st.WithdrawFeePercent)
		if asset == biz.AssetIspay {
			fee = amount * st.WithdrawFeePercent / 100
		}
		return map[string]interface{}{
			"balanceFen": bal,
			"feeFen":     fee,
			"netFen":     amount - fee,
			"asset":      asset,
			"txHash":     e.TxHash,
			"entry":      toProtoLedger(e),
		}, nil
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return ctx.Result(200, out)
}

func (s *WalletService) HTTPRechargePorts(ctx khttp.Context) error {
	ports := make([]map[string]interface{}, 0, len(biz.DefaultRechargePorts))
	for _, p := range biz.DefaultRechargePorts {
		ports = append(ports, map[string]interface{}{
			"address": biz.NormalizeEthAddress(p.Address),
			"percent": float64(p.Percent) / 10,
		})
	}
	return ctx.Result(200, map[string]interface{}{"ports": ports})
}

func toProtoLedger(e *biz.LedgerEntry) *pb.LedgerEntry {
	return &pb.LedgerEntry{
		Id: e.ID, AmountFen: e.AmountFen, BalanceFen: e.BalanceFen,
		Type: e.Type, RefType: e.RefType, RefId: e.RefID, CreatedAt: e.CreatedAt,
	}
}

func ledgerJSON(list []*biz.LedgerEntry) []map[string]interface{} {
	entries := make([]map[string]interface{}, 0, len(list))
	for _, e := range list {
		if e == nil {
			continue
		}
		entries = append(entries, map[string]interface{}{
			"id":         e.ID,
			"amountFen":  e.AmountFen,
			"balanceFen": e.BalanceFen,
			"type":       e.Type,
			"refType":    e.RefType,
			"refId":      e.RefID,
			"createdAt":  e.CreatedAt,
			"txHash":     e.TxHash,
		})
	}
	return entries
}
