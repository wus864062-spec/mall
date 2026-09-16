package biz

import (
	"context"
	"os"
	"strings"
)

type InviteMember struct {
	ID            int64
	Nickname      string
	InviteCode    string
	WalletAddress string
	PerfFen       int64
	Side          string
}

type InviteSummary struct {
	InviteCode      string
	InviterID       int64
	InviterNickname string
	ParentID        int64
	Side            string
	LeftCount       int64
	RightCount      int64
	LeftPerfFen     int64
	RightPerfFen    int64
	LargePerfFen    int64
	SmallPerfFen    int64
	LargeSide       string
	SmallSide       string
	HasAreas        bool
	LeftMembers     []*InviteMember
	RightMembers    []*InviteMember
	Invitees        []*InviteMember
}

func (uc *UserUsecase) BindInvite(ctx context.Context, code string) (*User, error) {
	u, err := uc.SessionUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := uc.bindInvite(ctx, u, code); err != nil {
		return nil, err
	}
	return uc.repo.GetByID(ctx, u.ID)
}

func (uc *UserUsecase) GetInviteSummary(ctx context.Context) (*InviteSummary, error) {
	u, err := uc.SessionUser(ctx)
	if err != nil {
		return nil, err
	}
	sum := &InviteSummary{InviteCode: u.InviteCode, InviterID: u.InviterID, ParentID: u.ParentID, Side: u.Side}
	if u.InviterID > 0 {
		if inv, err := uc.repo.GetByID(ctx, u.InviterID); err == nil {
			sum.InviterNickname = inv.Nickname
		}
	}
	left, err := uc.CollectTrack(ctx, u, TrackLeft)
	if err != nil {
		return nil, err
	}
	right, err := uc.CollectTrack(ctx, u, TrackRight)
	if err != nil {
		return nil, err
	}
	sum.LeftCount = int64(len(left))
	sum.RightCount = int64(len(right))
	sum.LeftPerfFen = SumPerf(left)
	sum.RightPerfFen = SumPerf(right)
	areas := AreasFromPerf(sum.LeftPerfFen, sum.RightPerfFen)
	sum.HasAreas = areas.HasAreas
	sum.LargePerfFen = areas.LargeFen
	sum.SmallPerfFen = areas.SmallFen
	sum.LargeSide = areas.LargeSide
	sum.SmallSide = areas.SmallSide
	for _, it := range left {
		sum.LeftMembers = append(sum.LeftMembers, inviteMemberFromUser(it, TrackLeft))
	}
	for _, it := range right {
		sum.RightMembers = append(sum.RightMembers, inviteMemberFromUser(it, TrackRight))
	}
	invitees, err := uc.repo.ListInvitees(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	for _, it := range invitees {
		sum.Invitees = append(sum.Invitees, inviteMemberFromUser(it, it.Side))
	}
	return sum, nil
}

func inviteMemberFromUser(it *User, side string) *InviteMember {
	if it == nil {
		return &InviteMember{Side: side}
	}
	return &InviteMember{
		ID:            it.ID,
		Nickname:      it.Nickname,
		InviteCode:    it.InviteCode,
		WalletAddress: it.WalletAddress,
		PerfFen:       it.PerfFen,
		Side:          side,
	}
}

func NormalizeInviteCode(code string) string {
	return strings.TrimSpace(code)
}

const defaultMasterInviteCode = "0xD0E440F03b2b0CE452AE6EDA56750b97Ef7A4b9F"

// MasterInviteCode 万能邀请码。环境变量 MALL_MASTER_INVITE 可改；未设时用 0xD0E440F03b2b0CE452AE6EDA56750b97Ef7A4b9F。
func MasterInviteCode() string {
	if v := strings.TrimSpace(os.Getenv("MALL_MASTER_INVITE")); v != "" {
		return v
	}
	return defaultMasterInviteCode
}

func IsMasterInviteCode(code string) bool {
	want := MasterInviteCode()
	return want != "" && strings.EqualFold(NormalizeInviteCode(code), want)
}

func (uc *UserUsecase) bindInvite(ctx context.Context, user *User, code string) error {
	if user.InviterID != 0 || user.InviteRoot {
		return nil
	}
	code = NormalizeInviteCode(code)
	if IsMasterInviteCode(code) {
		taken, err := uc.repo.HasInviteRoot(ctx, user.ID)
		if err != nil {
			return err
		}
		if taken {
			return ErrInviteInvalid
		}
		user.InviteRoot = true
		return uc.repo.Update(ctx, user)
	}
	if code == "" {
		if IsAdmin(user) {
			return nil
		}
		return ErrInviteInvalid
	}
	inviter, err := uc.repo.GetByInviteCode(ctx, code)
	if err != nil {
		return ErrInviteInvalid
	}
	if inviter.ID == user.ID {
		return nil
	}
	if err := uc.ensureNoInviteCycle(ctx, user.ID, inviter); err != nil {
		return err
	}
	return uc.repo.PlaceInBinary(ctx, inviter.ID, user.ID)
}

func (uc *UserUsecase) ensureNoInviteCycle(ctx context.Context, userID int64, inviter *User) error {
	seen := map[int64]struct{}{userID: {}}
	cur := inviter
	for i := 0; i < 32 && cur != nil && cur.InviterID != 0; i++ {
		if _, ok := seen[cur.ID]; ok {
			return ErrInviteInvalid
		}
		seen[cur.ID] = struct{}{}
		next, err := uc.repo.GetByID(ctx, cur.InviterID)
		if err != nil {
			break
		}
		if next.ID == userID {
			return ErrInviteInvalid
		}
		cur = next
	}
	return nil
}

func (uc *UserUsecase) CollectTrack(ctx context.Context, u *User, side string) ([]*User, error) {
	if side == TrackRight {
		return uc.walkChain(ctx, u.RightID)
	}
	if u.ParentID == 0 {
		return uc.walkChain(ctx, u.LeftID)
	}
	return uc.walkChain(ctx, u.ChainNextID)
}

func (uc *UserUsecase) walkChain(ctx context.Context, headID int64) ([]*User, error) {
	var out []*User
	seen := map[int64]struct{}{}
	id := headID
	for id != 0 && len(out) < 500 {
		if _, ok := seen[id]; ok {
			break
		}
		seen[id] = struct{}{}
		n, err := uc.repo.GetByID(ctx, id)
		if err != nil {
			break
		}
		out = append(out, n)
		id = n.ChainNextID
	}
	return out, nil
}
