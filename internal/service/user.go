package service

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	pb "mall/api/user/v1"
	"mall/internal/biz"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

type UserService struct {
	pb.UnimplementedUserServiceServer
	uc *biz.UserUsecase
}

func NewUserService(uc *biz.UserUsecase) *UserService {
	return &UserService{uc: uc}
}

const OperationUserHTTPNeedInvite = "/api.user.v1.UserService/HTTPNeedInvite"

func (s *UserService) HTTPNeedInvite(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationUserHTTPNeedInvite)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		need, err := s.uc.NeedInvite(c)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"needInvite": need}, nil
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return ctx.Result(200, out)
}

func (s *UserService) HTTPBindInvite(ctx khttp.Context) error {
	khttp.SetOperation(ctx, pb.OperationUserServiceBindInvite)
	raw, _ := io.ReadAll(ctx.Request().Body)
	var body struct {
		InviteCode string `json:"invite_code"`
		Camel      string `json:"inviteCode"`
	}
	_ = json.Unmarshal(raw, &body)
	code := strings.TrimSpace(body.InviteCode)
	if code == "" {
		code = strings.TrimSpace(body.Camel)
	}
	if code == "" {
		code = strings.TrimSpace(ctx.Query().Get("invite_code"))
	}
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		u, err := s.uc.BindInvite(c, code)
		if err != nil {
			return nil, err
		}
		return toMeReply(u, s.inviterNickname(c, u.InviterID)), nil
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return ctx.Result(200, out)
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

const OperationUserHTTPGetInvite = "/api.user.v1.UserService/HTTPGetInvite"

func (s *UserService) HTTPGetInvite(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationUserHTTPGetInvite)
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		return s.uc.GetInviteSummary(c)
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return ctx.Result(200, inviteJSON(out.(*biz.InviteSummary)))
}

func inviteJSON(sum *biz.InviteSummary) map[string]interface{} {
	if sum == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"inviteCode":      sum.InviteCode,
		"inviterId":       sum.InviterID,
		"inviterNickname": sum.InviterNickname,
		"parentId":        sum.ParentID,
		"side":            sum.Side,
		"leftCount":       sum.LeftCount,
		"rightCount":      sum.RightCount,
		"leftPerfFen":     sum.LeftPerfFen,
		"rightPerfFen":    sum.RightPerfFen,
		"largePerfFen":    sum.LargePerfFen,
		"smallPerfFen":    sum.SmallPerfFen,
		"largeSide":       sum.LargeSide,
		"smallSide":       sum.SmallSide,
		"hasAreas":        sum.HasAreas,
		"leftMembers":     inviteMembersJSON(sum.LeftMembers),
		"rightMembers":    inviteMembersJSON(sum.RightMembers),
		"invitees":        inviteMembersJSON(sum.Invitees),
	}
}

func inviteMembersJSON(list []*biz.InviteMember) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(list))
	for _, m := range list {
		if m == nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"id":            m.ID,
			"nickname":      m.Nickname,
			"inviteCode":    m.InviteCode,
			"side":          m.Side,
			"perfFen":       m.PerfFen,
			"walletAddress": m.WalletAddress,
		})
	}
	return out
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
