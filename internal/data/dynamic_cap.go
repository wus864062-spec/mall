package data

import (
	"time"

	"mall/internal/biz"
)

func shanghaiDayString(t time.Time) string {
	return t.In(pairSettleLocation()).Format("2006-01-02")
}

func (d *Data) dynamicUsedLocked(u *biz.User, now time.Time) int64 {
	if u == nil {
		return 0
	}
	if u.DynamicRewardDay != shanghaiDayString(now) {
		return 0
	}
	if u.DynamicRewardFen < 0 {
		return 0
	}
	return u.DynamicRewardFen
}

func (d *Data) takeDynamicPayoutLocked(u *biz.User, wantFen, capFen int64, now time.Time) int64 {
	if u == nil || wantFen <= 0 {
		return 0
	}
	day := shanghaiDayString(now)
	if u.DynamicRewardDay != day {
		u.DynamicRewardDay = day
		u.DynamicRewardFen = 0
	}
	pay := wantFen
	if capFen > 0 {
		pay = biz.CapDynamicPayout(wantFen, capFen, u.DynamicRewardFen)
	}
	u.DynamicRewardFen += pay
	return pay
}

func (d *Data) refundDynamicUsedLocked(u *biz.User, valueFen int64, paidAt time.Time) {
	if u == nil || valueFen <= 0 {
		return
	}
	if u.DynamicRewardDay != shanghaiDayString(paidAt) {
		return
	}
	u.DynamicRewardFen -= valueFen
	if u.DynamicRewardFen < 0 {
		u.DynamicRewardFen = 0
	}
}

// creditCappedDynamicLocked 按当日剩余动态奖励额度入账。返回实际折前 fen。
// 封顶用配置项里该档的日封顶（PairCaps），不是认购总额，也不是默认 600/1200 表（除非没改过）。
func (d *Data) creditCappedDynamicLocked(userID, wantFen int64, usdtType, ispayType, refType string, refID int64) int64 {
	u := d.users[userID]
	if u == nil || wantFen <= 0 {
		return 0
	}
	capFen := d.highestPairCapLocked(userID)
	if biz.EffectivePackageFen(biz.Web3PaidSumFen(d.orderListLocked(), userID)) > 0 && capFen <= 0 {
		// 有档位但配置项把日封顶改成 0：当天动态奖不发。无档位仍走下面，直推不卡。
		return 0
	}
	pay := d.takeDynamicPayoutLocked(u, wantFen, capFen, time.Now())
	if pay > 0 {
		d.creditSplitLocked(userID, pay, usdtType, ispayType, refType, refID)
	}
	return pay
}
