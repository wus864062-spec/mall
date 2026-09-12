package biz

import (
	"context"
	"strings"
)

type InviteMember struct {
	ID         int64
	Nickname   string
	InviteCode string
	Side       string
}

type InviteSummary struct {
	InviteCode      string
	InviterID       int64
	InviterNickname string
	ParentID        int64
	Side            string
	LeftCount       int64
	RightCount      int64
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
	for _, it := range left {
		sum.LeftMembers = append(sum.LeftMembers, &InviteMember{ID: it.ID, Nickname: it.Nickname, InviteCode: it.InviteCode, Side: TrackLeft})
	}
	for _, it := range right {
		sum.RightMembers = append(sum.RightMembers, &InviteMember{ID: it.ID, Nickname: it.Nickname, InviteCode: it.InviteCode, Side: TrackRight})
	}
	invitees, err := uc.repo.ListInvitees(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	for _, it := range invitees {
		sum.Invitees = append(sum.Invitees, &InviteMember{ID: it.ID, Nickname: it.Nickname, InviteCode: it.InviteCode, Side: it.Side})
	}
	return sum, nil
}

func (uc *UserUsecase) bindInvite(ctx context.Context, user *User, code string) error {
	if user.InviterID != 0 {
		return ErrInviteAlreadyBound
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return ErrInviteInvalid
	}
	inviter, err := uc.repo.GetByInviteCode(ctx, code)
	if err != nil {
		return ErrInviteInvalid
	}
	if inviter.ID == user.ID {
		return ErrInviteSelf
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
