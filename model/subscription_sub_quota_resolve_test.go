package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedPlanWithSubQuota(t *testing.T, id int, subQuotaLimits string) {
	t.Helper()
	plan := &SubscriptionPlan{
		Id:             id,
		Title:          fmt.Sprintf("plan-%d", id),
		PriceAmount:    0,
		DurationUnit:   SubscriptionDurationMonth,
		DurationValue:  1,
		TotalAmount:    1000,
		SubQuotaLimits: subQuotaLimits,
	}
	require.NoError(t, DB.Create(plan).Error)
	InvalidateSubscriptionPlanCache(id)
}

func updatePlanSubQuota(t *testing.T, id int, subQuotaLimits string) {
	t.Helper()
	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", id).
		Update("sub_quota_limits", subQuotaLimits).Error)
	InvalidateSubscriptionPlanCache(id)
}

const subQuotaOneHour = `[{"period_unit":"hour","period_value":5,"limit_usd":61}]`

// Live plan limits override the purchase-time snapshot.
func TestResolveSubQuotaLimits_PlanWinsOverSnapshot(t *testing.T) {
	truncateTables(t)
	seedPlanWithSubQuota(t, 9301, subQuotaOneHour)

	sub := &UserSubscription{PlanId: 9301, SubQuotaLimits: `[{"period_unit":"hour","period_value":5,"limit_usd":70}]`}
	limits := resolveSubQuotaLimits(sub)
	require.Len(t, limits, 1)
	assert.Equal(t, 61.0, limits[0].LimitUSD)

	// Editing the plan takes effect immediately on the same sub object.
	updatePlanSubQuota(t, 9301, `[{"period_unit":"hour","period_value":5,"limit_usd":45}]`)
	limits = resolveSubQuotaLimits(sub)
	require.Len(t, limits, 1)
	assert.Equal(t, 45.0, limits[0].LimitUSD)
}

// Clearing the plan limits removes limits entirely — the stale snapshot is
// not resurrected.
func TestResolveSubQuotaLimits_PlanClearedMeansNoLimits(t *testing.T) {
	truncateTables(t)
	seedPlanWithSubQuota(t, 9302, subQuotaOneHour)
	updatePlanSubQuota(t, 9302, "")

	sub := &UserSubscription{PlanId: 9302, SubQuotaLimits: subQuotaOneHour}
	assert.Empty(t, resolveSubQuotaLimits(sub))
}

// Snapshot still works when the plan cannot be read.
func TestResolveSubQuotaLimits_FallsBackToSnapshot(t *testing.T) {
	truncateTables(t)
	seedPlanWithSubQuota(t, 9303, "{not json")

	sub := &UserSubscription{PlanId: 9303, SubQuotaLimits: subQuotaOneHour}
	limits := resolveSubQuotaLimits(sub)
	require.Len(t, limits, 1)
	assert.Equal(t, 61.0, limits[0].LimitUSD)

	// Plan deleted entirely -> snapshot fallback.
	sub = &UserSubscription{PlanId: 9999, SubQuotaLimits: subQuotaOneHour}
	limits = resolveSubQuotaLimits(sub)
	require.Len(t, limits, 1)
	assert.Equal(t, 61.0, limits[0].LimitUSD)
}

// Both plan and snapshot empty -> no limits, and a malformed snapshot does
// not break enforcement/display.
func TestResolveSubQuotaLimits_EmptyAndMalformed(t *testing.T) {
	truncateTables(t)
	seedPlanWithSubQuota(t, 9304, "")

	sub := &UserSubscription{PlanId: 9304, SubQuotaLimits: ""}
	assert.Empty(t, resolveSubQuotaLimits(sub))

	sub = &UserSubscription{PlanId: 9999, SubQuotaLimits: "{not json"}
	assert.Empty(t, resolveSubQuotaLimits(sub))
}

// End-to-end: checkSubscriptionSubLimits enforces the edited plan limit even
// though the subscription row still holds the old snapshot.
func TestCheckSubscriptionSubLimits_UsesLivePlanLimits(t *testing.T) {
	truncateTables(t)
	seedPlanWithSubQuota(t, 9305, `[{"period_unit":"hour","period_value":5,"limit_usd":10,"anchor":"first_use"}]`)

	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:             9306,
		UserId:         9307,
		PlanId:         9305,
		AmountTotal:    1000,
		StartTime:      now - 3600,
		EndTime:        now + 30*24*3600,
		Status:         "active",
		SubQuotaLimits: `[{"period_unit":"hour","period_value":5,"limit_usd":100,"anchor":"first_use"}]`,
	}
	require.NoError(t, DB.Create(sub).Error)

	// USD 10 limit -> quota = 10 * QuotaPerUnit. An amount that would fit
	// the stale USD-100 snapshot but exceed the live USD-10 plan limit must
	// be rejected.
	overPlan := int64(20 * common.QuotaPerUnit)
	err := checkSubscriptionSubLimits(sub.UserId, sub, overPlan, now)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubQuotaExceeded), "unexpected error: %v", err)

	// Tighten the plan further: even a small amount is now rejected.
	updatePlanSubQuota(t, 9305, `[{"period_unit":"hour","period_value":5,"limit_usd":1,"anchor":"first_use"}]`)
	err = checkSubscriptionSubLimits(sub.UserId, sub, int64(2*common.QuotaPerUnit), now)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubQuotaExceeded), "unexpected error: %v", err)
}

// 读路径惰性重置:next_reset_time 已过的订阅,展示前自动推进重置。
func TestRefreshSubscriptionResetForDisplay_ResetsOverdue(t *testing.T) {
	truncateTables(t)
	seedPlanWithSubQuota(t, 9310, "")
	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", 9310).
		Update("quota_reset_period", SubscriptionResetWeekly).Error)
	InvalidateSubscriptionPlanCache(9310)

	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:            9311,
		UserId:        9312,
		PlanId:        9310,
		AmountTotal:   1000,
		AmountUsed:    700,
		StartTime:     now - 14*24*3600,
		EndTime:       now + 14*24*3600,
		Status:        "active",
		LastResetTime: now - 8*24*3600,
		NextResetTime: now - 1*24*3600, // 昨天就该重置
	}
	require.NoError(t, DB.Create(sub).Error)

	// 读路径触发
	refreshSubscriptionResetForDisplay(sub, now)

	assert.Equal(t, int64(0), sub.AmountUsed, "in-memory sub should be reset")
	assert.Greater(t, sub.NextResetTime, now, "next reset should be pushed forward")

	// 落库确认
	var stored UserSubscription
	require.NoError(t, DB.Where("id = ?", 9311).First(&stored).Error)
	assert.Equal(t, int64(0), stored.AmountUsed)
	assert.Greater(t, stored.NextResetTime, now)
}

// 未到期的订阅不受影响。
func TestRefreshSubscriptionResetForDisplay_NotDue(t *testing.T) {
	truncateTables(t)
	seedPlanWithSubQuota(t, 9313, "")
	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", 9313).
		Update("quota_reset_period", SubscriptionResetWeekly).Error)
	InvalidateSubscriptionPlanCache(9313)

	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:            9314,
		UserId:        9315,
		PlanId:        9313,
		AmountTotal:   1000,
		AmountUsed:    700,
		StartTime:     now - 2*24*3600,
		EndTime:       now + 26*24*3600,
		Status:        "active",
		LastResetTime: now - 2*24*3600,
		NextResetTime: now + 5*24*3600, // 5 天后才到期
	}
	require.NoError(t, DB.Create(sub).Error)

	refreshSubscriptionResetForDisplay(sub, now)

	assert.Equal(t, int64(700), sub.AmountUsed, "not-due sub must stay untouched")
}

// 计划已删除的订阅不报错,展示层静默回退。
func TestRefreshSubscriptionResetForDisplay_MissingPlan(t *testing.T) {
	truncateTables(t)
	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:            9316,
		UserId:        9317,
		PlanId:        9998, // 不存在
		AmountTotal:   1000,
		AmountUsed:    700,
		StartTime:     now - 14*24*3600,
		EndTime:       now + 14*24*3600,
		Status:        "active",
		LastResetTime: now - 8*24*3600,
		NextResetTime: now - 1*24*3600,
	}
	require.NoError(t, DB.Create(sub).Error)

	refreshSubscriptionResetForDisplay(sub, now)
	assert.Equal(t, int64(700), sub.AmountUsed, "missing plan should leave sub untouched")
}

// ---------- weekly 滑动锚定 ----------

// 购买时:weekly 订阅 next_reset_time 必须为 0(未锚定),等首次使用再落窗。
func TestWeeklySlidingAnchor_PurchaseLeavesUnanchored(t *testing.T) {
	truncateTables(t)
	seedPlanWithSubQuota(t, 9320, "")
	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", 9320).
		Updates(map[string]any{"quota_reset_period": SubscriptionResetWeekly, "total_amount": 1000}).Error)
	InvalidateSubscriptionPlanCache(9320)

	user := &User{Id: 9321, Username: "u9321", Password: "x", Group: "default", Quota: 0}
	require.NoError(t, DB.Create(user).Error)

	require.NoError(t, PurchaseSubscriptionWithBalance(9321, 9320))

	var sub UserSubscription
	require.NoError(t, DB.Where("user_id = ? AND plan_id = ?", 9321, 9320).First(&sub).Error)
	assert.Equal(t, int64(0), sub.NextResetTime, "weekly 购买后应保持未锚定")
	assert.Equal(t, int64(0), sub.LastResetTime)
}

// 首次 PreConsume 时:锚定 7 天窗口,amount_used 从 0 起算。
func TestWeeklySlidingAnchor_FirstConsumeAnchorsWindow(t *testing.T) {
	truncateTables(t)
	seedPlanWithSubQuota(t, 9322, "")
	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", 9322).
		Updates(map[string]any{"quota_reset_period": SubscriptionResetWeekly, "total_amount": 100000}).Error)
	InvalidateSubscriptionPlanCache(9322)

	user := &User{Id: 9323, Username: "u9323", Password: "x", Group: "default", Quota: 0}
	require.NoError(t, DB.Create(user).Error)
	require.NoError(t, PurchaseSubscriptionWithBalance(9323, 9322))

	// 首次扣费
	beforeConsume := GetDBTimestamp()
	result, err := PreConsumeUserSubscription("req-anchor-1", 9323, "gpt-4", 0, 100)
	require.NoError(t, err)
	require.NotNil(t, result)

	var sub UserSubscription
	require.NoError(t, DB.Where("user_id = ? AND plan_id = ?", 9323, 9322).First(&sub).Error)
	assert.Equal(t, beforeConsume, sub.LastResetTime, "首次使用时刻 = 窗口起点")
	expectedNext := beforeConsume + 7*24*3600
	assert.Equal(t, expectedNext, sub.NextResetTime, "窗口 = 首次使用 + 7 天")
	assert.Equal(t, int64(100), sub.AmountUsed)
}

// 窗口内继续 consume 不重新锚定;窗口过后下一次 consume 推进到下一窗。
func TestWeeklySlidingAnchor_RollsForwardAfterWindow(t *testing.T) {
	truncateTables(t)
	seedPlanWithSubQuota(t, 9324, "")
	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", 9324).
		Updates(map[string]any{"quota_reset_period": SubscriptionResetWeekly, "total_amount": 100000}).Error)
	InvalidateSubscriptionPlanCache(9324)

	user := &User{Id: 9325, Username: "u9325", Password: "x", Group: "default", Quota: 0}
	require.NoError(t, DB.Create(user).Error)
	require.NoError(t, PurchaseSubscriptionWithBalance(9325, 9324))

	now := GetDBTimestamp()
	// 手工造一个"窗口已过期"的状态:last_reset=8 天前,next_reset=1 天前
	require.NoError(t, DB.Model(&UserSubscription{}).
		Where("user_id = ? AND plan_id = ?", 9325, 9324).
		Updates(map[string]any{
			"amount_used":     int64(5000),
			"last_reset_time": now - 8*24*3600,
			"next_reset_time": now - 1*24*3600,
		}).Error)

	// 窗口已过 → 惰性重置会推进:amount_used 清零,窗口从"上一窗的结束"起再滑 7 天
	// (不是从当前时刻,保持窗口连续性 — 这是 A 语义"窗口连续滚动"的关键)
	_, err := PreConsumeUserSubscription("req-anchor-2", 9325, "gpt-4", 0, 100)
	require.NoError(t, err)

	var sub UserSubscription
	require.NoError(t, DB.Where("user_id = ? AND plan_id = ?", 9325, 9324).First(&sub).Error)
	assert.Equal(t, int64(100), sub.AmountUsed, "老窗口用量清零,只算这次")
	// 上一窗 last_reset = now-8d, 7 天后结束 = now-1d; 再 +7d = now+6d
	expectedNext := now + 6*24*3600
	assert.Equal(t, expectedNext, sub.NextResetTime)
	assert.Equal(t, now-1*24*3600, sub.LastResetTime)
}

// 读路径:next_reset_time=0 时不应被推进(等首次 consume 才锚定)。
func TestWeeklySlidingAnchor_DisplayDoesNotAnchor(t *testing.T) {
	truncateTables(t)
	seedPlanWithSubQuota(t, 9326, "")
	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", 9326).
		Updates(map[string]any{"quota_reset_period": SubscriptionResetWeekly, "total_amount": 1000}).Error)
	InvalidateSubscriptionPlanCache(9326)

	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:            9327,
		UserId:        9328,
		PlanId:        9326,
		AmountTotal:   1000,
		AmountUsed:    0,
		StartTime:     now - 3600,
		EndTime:       now + 30*24*3600,
		Status:        "active",
		LastResetTime: 0,
		NextResetTime: 0, // 未锚定
	}
	require.NoError(t, DB.Create(sub).Error)

	refreshSubscriptionResetForDisplay(sub, now)

	assert.Equal(t, int64(0), sub.NextResetTime, "读路径不应锚定,等首次 consume")
}
