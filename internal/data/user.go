package data

import (
	"context"
	"strings"
	"time"

	"mall/internal/biz"
)

type userRepo struct {
	data *Data
}

func NewUserRepo(data *Data) biz.UserRepo {
	return &userRepo{data: data}
}

func (r *userRepo) GetOrCreateByWallet(ctx context.Context, wallet string) (*biz.User, bool, error) {
	wallet = strings.ToLower(wallet)
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	if id, ok := r.data.byWallet[wallet]; ok {
		u := r.data.users[id]
		changed := false
		if u.InviteCode == "" {
			u.InviteCode = biz.InviteCodeFor(u.ID)
			r.data.byInvite[u.InviteCode] = u.ID
			changed = true
		}
		if u.CreatedAt == 0 {
			u.CreatedAt = time.Now().Unix()
			changed = true
		}
		if changed {
			if err := r.data.save(); err != nil {
				return nil, false, err
			}
		}
		return cloneUser(u), false, nil
	}
	r.data.userSeq++
	u := &biz.User{
		ID:            r.data.userSeq,
		WalletAddress: wallet,
		Nickname:      biz.DefaultNickname(wallet),
		InviteCode:    biz.InviteCodeFor(r.data.userSeq),
		CreatedAt:     time.Now().Unix(),
	}
	r.data.users[u.ID] = u
	r.data.byWallet[wallet] = u.ID
	r.data.byInvite[u.InviteCode] = u.ID
	if err := r.data.save(); err != nil {
		return nil, false, err
	}
	return cloneUser(u), true, nil
}

func (r *userRepo) GetByID(ctx context.Context, id int64) (*biz.User, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	u, ok := r.data.users[id]
	if !ok {
		return nil, biz.ErrUserNotExist
	}
	return cloneUser(u), nil
}

func (r *userRepo) GetByInviteCode(ctx context.Context, code string) (*biz.User, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	id, ok := r.data.byInvite[code]
	if !ok {
		return nil, biz.ErrInviteInvalid
	}
	u, ok := r.data.users[id]
	if !ok {
		return nil, biz.ErrInviteInvalid
	}
	return cloneUser(u), nil
}

func (r *userRepo) ListInvitees(ctx context.Context, inviterID int64) ([]*biz.User, error) {
	r.data.mu.RLock()
	defer r.data.mu.RUnlock()
	var out []*biz.User
	for _, u := range r.data.users {
		if u.InviterID == inviterID {
			out = append(out, cloneUser(u))
		}
	}
	return out, nil
}

func (r *userRepo) PlaceInBinary(ctx context.Context, sponsorID, inviteeID int64) error {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	if _, ok := r.data.users[sponsorID]; !ok {
		return biz.ErrInviteInvalid
	}
	invitee, ok := r.data.users[inviteeID]
	if !ok {
		return biz.ErrUserNotExist
	}
	if invitee.InviterID != 0 || invitee.ParentID != 0 {
		return biz.ErrInviteAlreadyBound
	}
	if err := biz.PlaceSharedTrack(r.data.users, sponsorID, inviteeID); err != nil {
		return err
	}
	return r.data.save()
}

func (r *userRepo) Update(ctx context.Context, u *biz.User) error {
	r.data.mu.Lock()
	defer r.data.mu.Unlock()
	cur, ok := r.data.users[u.ID]
	if !ok {
		return biz.ErrUserNotExist
	}
	cur.Nickname = u.Nickname
	cur.Avatar = u.Avatar
	cur.InviteCode = u.InviteCode
	cur.InviterID = u.InviterID
	cur.ParentID = u.ParentID
	cur.Side = u.Side
	cur.LeftID = u.LeftID
	cur.RightID = u.RightID
	cur.ChainNextID = u.ChainNextID
	cur.OccupiedPos = u.OccupiedPos
	cur.TokenVersion = u.TokenVersion
	if u.CreatedAt != 0 {
		cur.CreatedAt = u.CreatedAt
	}
	return r.data.save()
}
