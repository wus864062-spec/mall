package biz

func minFen(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

const (
	PairRewardPercent int64 = 10
	PairUplinePercent int64 = 30
	PairUplineLevels  int   = 3
	maxTrackSlots     int   = 1024
)

func PairRewardFen(pairFen int64) int64 {
	return pairFen * PairRewardPercent / 100
}

func PairVolumeFromBonus(bonusFen int64) int64 {
	if bonusFen <= 0 || PairRewardPercent <= 0 {
		return 0
	}
	return bonusFen * 100 / PairRewardPercent
}

// pairCapByPriceFen 每日对碰收入封顶（认购价 -> 日封顶，单位分）。
// 1000→600、3000→1800、6000→4000、12000→8000、24000→16000、
// 36000→24000、50000→30000、70000→42000、100000→60000、160000→100000。
// 10000→8000 仅保留给历史订单。
var pairCapByPriceFen = map[int64]int64{
	1000 * 100:   600 * 100,
	3000 * 100:   1800 * 100,
	6000 * 100:   4000 * 100,
	10000 * 100:  8000 * 100,
	12000 * 100:  8000 * 100,
	24000 * 100:  16000 * 100,
	36000 * 100:  24000 * 100,
	50000 * 100:  30000 * 100,
	70000 * 100:  42000 * 100,
	100000 * 100: 60000 * 100,
	160000 * 100: 100000 * 100,
}

func PairCapFen(priceFen int64) int64 {
	return pairCapByPriceFen[priceFen]
}

func HighestPairCapFen(orders []*Order, userID int64) int64 {
	var cap int64
	for _, o := range orders {
		if o == nil || o.UserID != userID || o.Status != OrderPaid {
			continue
		}
		for _, it := range o.Items {
			if it == nil {
				continue
			}
			if c := PairCapFen(it.PriceFen); c > cap {
				cap = c
			}
		}
	}
	return cap
}

func CapPairPayout(bonusFen, capFen int64) int64 {
	if bonusFen <= 0 || capFen <= 0 {
		return 0
	}
	if bonusFen > capFen {
		return capFen
	}
	return bonusFen
}

func SmallHitsLarge(leftTotal, rightTotal, settledLeft, clearedRight int64) (availLeft, availRight, pairVol int64) {
	availLeft = leftTotal - settledLeft
	if availLeft < 0 {
		availLeft = 0
	}
	availRight = rightTotal - clearedRight
	if availRight < 0 {
		availRight = 0
	}
	return availLeft, availRight, minFen(availLeft, availRight)
}

func PlaceSharedTrack(users map[int64]*User, sponsorID, inviteeID int64) error {
	sponsor, ok := users[sponsorID]
	if !ok {
		return ErrInviteInvalid
	}
	invitee, ok := users[inviteeID]
	if !ok {
		return ErrUserNotExist
	}
	if invitee.InviterID != 0 || invitee.ParentID != 0 {
		return ErrInviteAlreadyBound
	}
	for k := 1; k <= maxTrackSlots; k++ {
		ownerID, side, pos := leftSlot(sponsor, k)
		if chainAt(users, ownerID, side, pos) == nil {
			return appendToChain(users, ownerID, side, pos, sponsorID, invitee)
		}
		if chainAt(users, sponsor.ID, TrackRight, int64(k)) == nil {
			return appendToChain(users, sponsor.ID, TrackRight, int64(k), sponsorID, invitee)
		}
	}
	return ErrInviteInvalid
}

func leftSlot(sponsor *User, k int) (ownerID int64, side string, pos int64) {
	if sponsor.ParentID == 0 {
		return sponsor.ID, TrackLeft, int64(k)
	}
	return sponsor.ParentID, sponsor.Side, sponsor.OccupiedPos + int64(k)
}

func chainHeadID(owner *User, side string) int64 {
	if side == TrackLeft {
		return owner.LeftID
	}
	return owner.RightID
}

func chainAt(users map[int64]*User, ownerID int64, side string, pos int64) *User {
	owner, ok := users[ownerID]
	if !ok || pos <= 0 {
		return nil
	}
	cur, ok := users[chainHeadID(owner, side)]
	if !ok {
		return nil
	}
	for i := int64(1); cur != nil && i < pos; i++ {
		if cur.ChainNextID == 0 {
			return nil
		}
		cur = users[cur.ChainNextID]
	}
	return cur
}

func appendToChain(users map[int64]*User, ownerID int64, side string, pos int64, sponsorID int64, invitee *User) error {
	owner, ok := users[ownerID]
	if !ok {
		return ErrInviteInvalid
	}
	invitee.InviterID = sponsorID
	invitee.ParentID = ownerID
	invitee.Side = side
	invitee.OccupiedPos = pos
	invitee.ChainNextID = 0
	if pos == 1 {
		if side == TrackLeft {
			owner.LeftID = invitee.ID
		} else {
			owner.RightID = invitee.ID
		}
		return nil
	}
	prev := chainAt(users, ownerID, side, pos-1)
	if prev == nil {
		return ErrInviteInvalid
	}
	prev.ChainNextID = invitee.ID
	return nil
}

func WalkChain(users map[int64]*User, headID int64) []*User {
	var out []*User
	seen := map[int64]struct{}{}
	id := headID
	for id != 0 && len(out) < 500 {
		if _, ok := seen[id]; ok {
			break
		}
		seen[id] = struct{}{}
		u, ok := users[id]
		if !ok {
			break
		}
		out = append(out, u)
		id = u.ChainNextID
	}
	return out
}

func LeftMembersOf(users map[int64]*User, u *User) []*User {
	if u == nil {
		return nil
	}
	if u.ParentID == 0 {
		return WalkChain(users, u.LeftID)
	}
	return WalkChain(users, u.ChainNextID)
}

func RightMembersOf(users map[int64]*User, u *User) []*User {
	if u == nil {
		return nil
	}
	return WalkChain(users, u.RightID)
}

func SumPerf(members []*User) int64 {
	var n int64
	for _, m := range members {
		if m != nil {
			n += m.PerfFen
		}
	}
	return n
}

func PairFen(leftPerf, rightPerf int64) int64 {
	return minFen(leftPerf, rightPerf)
}

func SettlePairBonus(u *User, leftPerf, rightPerf int64) int64 {
	paired := PairFen(leftPerf, rightPerf)
	delta := paired - u.SettledPairFen
	u.SettledPairFen = paired
	return PairRewardFen(delta)
}

func PlacementAncestors(users map[int64]*User, u *User, levels int) []*User {
	return walkAncestors(users, u, levels, func(cur *User) int64 { return cur.ParentID })
}

func InviteAncestors(users map[int64]*User, u *User, levels int) []*User {
	return walkAncestors(users, u, levels, func(cur *User) int64 { return cur.InviterID })
}

func walkAncestors(users map[int64]*User, u *User, levels int, next func(*User) int64) []*User {
	if u == nil || levels <= 0 {
		return nil
	}
	var out []*User
	seen := map[int64]struct{}{u.ID: {}}
	cur := u
	for i := 0; i < levels; i++ {
		nid := next(cur)
		if nid == 0 {
			break
		}
		p, ok := users[nid]
		if !ok || p.ID == cur.ID {
			break
		}
		if _, dup := seen[p.ID]; dup {
			break
		}
		seen[p.ID] = struct{}{}
		out = append(out, p)
		cur = p
	}
	return out
}

func ManageRewardFen(pairPayFen int64) int64 {
	if pairPayFen <= 0 {
		return 0
	}
	return pairPayFen * PairUplinePercent / 100
}

func SplitEvenFen(total int64, n int) []int64 {
	if total <= 0 || n <= 0 {
		return nil
	}
	each := total / int64(n)
	rem := total % int64(n)
	out := make([]int64, n)
	for i := 0; i < n; i++ {
		out[i] = each
	}
	out[0] += rem
	return out
}
