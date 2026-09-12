package service

import (
	"context"

	pb "mall/api/wallet/v1"
	"mall/internal/biz"
)

type WalletService struct {
	pb.UnimplementedWalletServiceServer
	uc *biz.WalletUsecase
}

func NewWalletService(uc *biz.WalletUsecase) *WalletService {
	return &WalletService{uc: uc}
}

func (s *WalletService) GetWallet(ctx context.Context, req *pb.GetWalletRequest) (*pb.GetWalletReply, error) {
	bal, err := s.uc.GetWallet(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetWalletReply{BalanceFen: bal}, nil
}

func (s *WalletService) Recharge(ctx context.Context, req *pb.RechargeRequest) (*pb.RechargeReply, error) {
	e, bal, err := s.uc.Recharge(ctx, req.AmountFen)
	if err != nil {
		return nil, err
	}
	return &pb.RechargeReply{BalanceFen: bal, Entry: toProtoLedger(e)}, nil
}

func (s *WalletService) ListLedger(ctx context.Context, req *pb.ListLedgerRequest) (*pb.ListLedgerReply, error) {
	list, total, err := s.uc.ListLedger(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	var out []*pb.LedgerEntry
	for _, e := range list {
		out = append(out, toProtoLedger(e))
	}
	return &pb.ListLedgerReply{Entries: out, Total: total}, nil
}

func toProtoLedger(e *biz.LedgerEntry) *pb.LedgerEntry {
	return &pb.LedgerEntry{
		Id: e.ID, AmountFen: e.AmountFen, BalanceFen: e.BalanceFen,
		Type: e.Type, RefType: e.RefType, RefId: e.RefID, CreatedAt: e.CreatedAt,
	}
}
