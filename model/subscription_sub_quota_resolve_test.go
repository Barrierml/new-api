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
		PriceAmount:    10,
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
