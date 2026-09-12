package service

import (
	"context"

	pb "mall/api/user/v1"
	"mall/internal/biz"
)

type UserService struct {
	pb.UnimplementedUserServiceServer
	uc *biz.UserUsecase
}

func NewUserService(uc *biz.UserUsecase) *UserService {
	return &UserService{uc: uc}
}

func (s *UserService) GetNonce(ctx context.Context, req *pb.GetNonceRequest) (*pb.GetNonceReply, error) {
	nonce, err := biz.GenerateNonce(req.WalletAddress)
	if err != nil {
		return nil, err
	}
	return &pb.GetNonceReply{Nonce: nonce}, nil
}

func (s *UserService) WalletLogin(ctx context.Context, req *pb.WalletLoginRequest) (*pb.WalletLoginReply, error) {
	user, token, err := s.uc.WalletLogin(ctx, req.Message, req.Signature, req.InviteCode)
	if err != nil {
		return nil, err
	}
	inviterNick := s.inviterNickname(ctx, user.InviterID)
	return &pb.WalletLoginReply{
		Id:              user.ID,
		Token:           token,
		WalletAddress:   user.WalletAddress,
		Nickname:        user.Nickname,
		Avatar:          user.Avatar,
		InviteCode:      user.InviteCode,
		InviterId:       user.InviterID,
		InviterNickname: inviterNick,
		ParentId:        user.ParentID,
		Side:            user.Side,
		IsAdmin:         biz.IsAdmin(user),
	}, nil
}

func (s *UserService) GetMe(ctx context.Context, req *pb.GetMeRequest) (*pb.GetMeReply, error) {
	u, err := s.uc.GetMe(ctx)
	if err != nil {
		return nil, err
	}
	return toMeReply(u, s.inviterNickname(ctx, u.InviterID)), nil
}

func (s *UserService) UpdateMe(ctx context.Context, req *pb.UpdateMeRequest) (*pb.GetMeReply, error) {
	u, err := s.uc.UpdateMe(ctx, req.Nickname, req.Avatar)
	if err != nil {
		return nil, err
	}
	return toMeReply(u, s.inviterNickname(ctx, u.InviterID)), nil
}

func (s *UserService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutReply, error) {
	if err := s.uc.Logout(ctx); err != nil {
		return nil, err
	}
	return &pb.LogoutReply{}, nil
}

func (s *UserService) GetInvite(ctx context.Context, req *pb.GetInviteRequest) (*pb.GetInviteReply, error) {
	sum, err := s.uc.GetInviteSummary(ctx)
	if err != nil {
		return nil, err
	}
	out := &pb.GetInviteReply{
		InviteCode:      sum.InviteCode,
		InviterId:       sum.InviterID,
		InviterNickname: sum.InviterNickname,
		ParentId:        sum.ParentID,
		Side:            sum.Side,
		LeftCount:       sum.LeftCount,
		RightCount:      sum.RightCount,
	}
	for _, m := range sum.LeftMembers {
		out.LeftMembers = append(out.LeftMembers, toInviteMember(m))
	}
	for _, m := range sum.RightMembers {
		out.RightMembers = append(out.RightMembers, toInviteMember(m))
	}
	for _, m := range sum.Invitees {
		out.Invitees = append(out.Invitees, toInviteMember(m))
	}
	return out, nil
}

func toInviteMember(m *biz.InviteMember) *pb.InviteMember {
	return &pb.InviteMember{Id: m.ID, Nickname: m.Nickname, InviteCode: m.InviteCode, Side: m.Side}
}

func (s *UserService) BindInvite(ctx context.Context, req *pb.BindInviteRequest) (*pb.GetMeReply, error) {
	u, err := s.uc.BindInvite(ctx, req.InviteCode)
	if err != nil {
		return nil, err
	}
	return toMeReply(u, s.inviterNickname(ctx, u.InviterID)), nil
}

func (s *UserService) inviterNickname(ctx context.Context, inviterID int64) string {
	if inviterID <= 0 {
		return ""
	}
	u, err := s.uc.GetByID(ctx, inviterID)
	if err != nil {
		return ""
	}
	return u.Nickname
}

func toMeReply(u *biz.User, inviterNick string) *pb.GetMeReply {
	return &pb.GetMeReply{
		Id:              u.ID,
		WalletAddress:   u.WalletAddress,
		Nickname:        u.Nickname,
		Avatar:          u.Avatar,
		InviteCode:      u.InviteCode,
		InviterId:       u.InviterID,
		InviterNickname: inviterNick,
		ParentId:        u.ParentID,
		Side:            u.Side,
		IsAdmin:         biz.IsAdmin(u),
	}
}
