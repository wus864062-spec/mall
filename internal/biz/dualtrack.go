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

// pairCapByPriceFen 每日动态奖励封顶（档位 -> 日封顶，单位 fen）。
// 直推奖 + 对碰奖 + 管理奖合计共用；不是只卡对碰。
// 1000→600、2000→1200、3000→1800、6000→4000、12000→8000、24000→16000、
// 36000→24000、50000→30000、70000→42000、100000→60000、160000→100000。
// 10000→8000 仅保留给历史订单。
var pairCapByPriceFen = map[int64]int64{
	U(1000):   U(600),
	U(2000):   U(1200),
	U(3000):   U(1800),
	U(6000):   U(4000),
	U(10000):  U(8000),
	U(12000):  U(8000),
	U(24000):  U(16000),
	U(36000):  U(24000),
	U(50000):  U(30000),
	U(70000):  U(42000),
	U(100000): U(60000),
	U(160000): U(100000),
}

func ClonePairCaps(src map[int64]int64) map[int64]int64 {
	out := make(map[int64]int64, len(pairCapByPriceFen))
	for k, v := range pairCapByPriceFen {
		out[k] = v
	}
	for k, v := range src {
		if _, ok := pairCapByPriceFen[k]; !ok {
			continue
		}
		if v < 0 {
			v = 0
		}
		out[k] = v
	}
	return out
}

func DefaultPairCaps() map[int64]int64 {
	return ClonePairCaps(nil)
}

// MergePairCaps 默认档位表 + 后台覆盖。覆盖值 0 表示该档当天不发对碰/管理奖、也不发提现额度。
func MergePairCaps(overlay map[int64]int64) map[int64]int64 {
	return ClonePairCaps(overlay)
}

func IsPairCapTier(priceFen int64) bool {
	_, ok := pairCapByPriceFen[priceFen]
	return ok
}

// PairCapFen 默认档位表。发奖 / 解冻要用 PairCapFenWith(档位, 配置项)。
func PairCapFen(priceFen int64) int64 {
	return pairCapByPriceFen[priceFen]
}

func PairCapFenWith(priceFen int64, overlay map[int64]int64) int64 {
	return MergePairCaps(overlay)[priceFen]
}

// UnfreezeCapFen 当天可发额度 = PairCapFen(认购合计折出的档位)。用当天最高档，不用各件 600 相加。
// 1000 → 600；1000+1000=2000 升 2000 档 → 1200；3000 档 → 1800；升 6000 档 → 4000。
func UnfreezeCapFen(paidFen int64) int64 {
	return UnfreezeCapFenWith(paidFen, nil)
}

func UnfreezeCapFenWith(paidFen int64, overlay map[int64]int64) int64 {
	return PairCapFenWith(EffectivePackageFen(paidFen), overlay)
}

// UnfreezeGrantedTodayFen 换日视为当天还没发过。grantedFen 是当天已发的档位封顶，不是剩余提现额度。
func UnfreezeGrantedTodayFen(grantDay, today string, grantedFen int64) int64 {
	if grantDay != today || grantedFen <= 0 {
		return 0
	}
	return grantedFen
}

// GrantUnfreezeFen 本单新给 = 当前档位封顶 − 当天已发。同档加 0；升档只补差额。换日已发按 0，要再买单才发。
func GrantUnfreezeFen(grantedTodayFen, paidFen int64) int64 {
	return GrantUnfreezeFenWith(grantedTodayFen, paidFen, nil)
}

func GrantUnfreezeFenWith(grantedTodayFen, paidFen int64, overlay map[int64]int64) int64 {
	n := UnfreezeCapFenWith(paidFen, overlay) - grantedTodayFen
	if n < 0 {
		return 0
	}
	return n
}

// TakeUnfreeze 实际解冻折前 = min(剩余额度, 冻结市值)。两个都是折前 fen，不是只 USDT。
func TakeUnfreeze(remainFen, frozenValueFen int64) int64 {
	if remainFen <= 0 || frozenValueFen <= 0 {
		return 0
	}
	return minFen(remainFen, frozenValueFen)
}

const FrozenTTLSeconds int64 = 72 * 60 * 60

// DailyDynamicCapFen 与 PairCapFen 同一张档位表。用于直推/对碰/管理奖合计日封顶。
func DailyDynamicCapFen(packageFen int64) int64 {
	return DailyDynamicCapFenWith(packageFen, nil)
}

func DailyDynamicCapFenWith(packageFen int64, overlay map[int64]int64) int64 {
	return PairCapFenWith(packageFen, overlay)
}

var catalogPackageFen = []int64{
	U(1000),
	U(2000),
	U(3000),
	U(6000),
	U(12000),
	U(24000),
	U(36000),
	U(50000),
	U(70000),
	U(100000),
	U(160000),
}

// EffectivePackageFen 把认购合计向下取整到目录档位。
// 1000+3000=4000 → 3000 档；3000+3000=6000 → 升 6000 档。
// 用于「当前档位」、动态奖励日封顶、管理奖资格；锁仓还要加上未入下一档的超额。
// 购物车：用各套餐实付合计（fen），不要用档位金额去折 IsPay。
func EffectivePackageFen(sumFen int64) int64 {
	var best int64
	for _, p := range catalogPackageFen {
		if p <= sumFen && p > best {
			best = p
		}
	}
	return best
}

// CartExcessFen 合计 − 档位。超额按与档位相同的兑换价折进锁仓，不是新档位。
func CartExcessFen(totalFen int64) int64 {
	eff := EffectivePackageFen(totalFen)
	if totalFen <= eff {
		return 0
	}
	return totalFen - eff
}

func OrderPayFen(o *Order) int64 {
	if o == nil {
		return 0
	}
	if o.TotalFen > 0 {
		return o.TotalFen
	}
	var s int64
	for _, it := range o.Items {
		if it == nil {
			continue
		}
		if it.AmountFen > 0 {
			s += it.AmountFen
		} else {
			s += it.PriceFen * it.Quantity
		}
	}
	return s
}

func IsSubscribeOrder(o *Order) bool {
	if o == nil || o.Status != OrderPaid {
		return false
	}
	if IsWeb3Category(o.CategoryID) {
		return true
	}
	if o.CategoryID != 0 {
		return false
	}
	return EffectivePackageFen(OrderPayFen(o)) > 0 || PairCapFen(OrderPayFen(o)) > 0
}

// Web3PaidSumFen 用户已付 Web3 认购总额（fen）。锁仓按这个数折 IsPay。
func Web3PaidSumFen(orders []*Order, userID int64) int64 {
	var sum int64
	for _, o := range orders {
		if o == nil || o.UserID != userID || !IsSubscribeOrder(o) {
			continue
		}
		sum += OrderPayFen(o)
	}
	return sum
}

func DominantWeb3Category(orders []*Order, userID int64) int64 {
	var bestAmt int64
	var bestID int64
	cat := CategoryWeb3
	for _, o := range orders {
		if o == nil || o.UserID != userID || !IsSubscribeOrder(o) {
			continue
		}
		amt := OrderPayFen(o)
		if amt > bestAmt || (amt == bestAmt && o.ID > bestID) {
			bestAmt = amt
			bestID = o.ID
			if IsWeb3Category(o.CategoryID) {
				cat = o.CategoryID
			} else {
				cat = CategoryWeb3
			}
		}
	}
	return cat
}

func HighestPairCapFen(orders []*Order, userID int64) int64 {
	return HighestPairCapFenWith(orders, userID, nil)
}

func HighestPairCapFenWith(orders []*Order, userID int64, overlay map[int64]int64) int64 {
	return DailyDynamicCapFenWith(EffectivePackageFen(Web3PaidSumFen(orders, userID)), overlay)
}

// CapDynamicPayout 当日剩余额度：cap − used，超出去掉不滚日。
// capFen=0 表示无档位：对碰/管理奖为 0；直推由调用方不走 cap。
func CapDynamicPayout(wantFen, capFen, usedFen int64) int64 {
	if wantFen <= 0 || capFen <= 0 {
		return 0
	}
	left := capFen - usedFen
	if left <= 0 {
		return 0
	}
	if wantFen > left {
		return left
	}
	return wantFen
}

func CapPairPayout(bonusFen, capFen int64) int64 {
	return CapDynamicPayout(bonusFen, capFen, 0)
}

func PairPending(leftTotal, rightTotal, settledFen int64) (availLeft, availRight, pairVol int64) {
	if leftTotal < 0 {
		leftTotal = 0
	}
	if rightTotal < 0 {
		rightTotal = 0
	}
	if settledFen < 0 {
		settledFen = 0
	}
	availLeft = leftTotal - settledFen
	if availLeft < 0 {
		availLeft = 0
	}
	availRight = rightTotal - settledFen
	if availRight < 0 {
		availRight = 0
	}
	pairVol = minFen(leftTotal, rightTotal) - settledFen
	if pairVol < 0 {
		pairVol = 0
	}
	return availLeft, availRight, pairVol
}

func SmallHitsLarge(leftTotal, rightTotal, settledFen, _ int64) (availLeft, availRight, pairVol int64) {
	return PairPending(leftTotal, rightTotal, settledFen)
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
	// 填邀请人钱包后按双轨占位：先左后右；已安置的人同一侧往后排。万能码根节点不走这里。
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

// AreaView 大小区：只用左右链成员认购合计 PerfFen，不用自己的认购、不用左右哪个链。
// 两侧都是 0：下线还没买过单，没有大小区。
// 有一侧 > 0：业绩大的为大区，小的为小区；相等时金额相同，展示大区落在左区。
type AreaView struct {
	HasAreas  bool
	LargeFen  int64
	SmallFen  int64
	LargeSide string
	SmallSide string
}

func AreasFromPerf(leftFen, rightFen int64) AreaView {
	if leftFen < 0 {
		leftFen = 0
	}
	if rightFen < 0 {
		rightFen = 0
	}
	if leftFen == 0 && rightFen == 0 {
		return AreaView{}
	}
	if leftFen >= rightFen {
		return AreaView{HasAreas: true, LargeFen: leftFen, SmallFen: rightFen, LargeSide: TrackLeft, SmallSide: TrackRight}
	}
	return AreaView{HasAreas: true, LargeFen: rightFen, SmallFen: leftFen, LargeSide: TrackRight, SmallSide: TrackLeft}
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
