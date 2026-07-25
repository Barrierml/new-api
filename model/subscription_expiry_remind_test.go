package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListExpiryReminderDueSubscriptionsFiltersWindowStatusAndDedup(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	ahead := int64(5 * 24 * 3600)

	// due: active, expiring within 5 days, not yet reminded
	require.NoError(t, DB.Create(&UserSubscription{Id: 9301, UserId: 201, PlanId: 9001, StartTime: now - 3600, EndTime: now + 3*24*3600, Status: "active"}).Error)
	// not due: already reminded
	require.NoError(t, DB.Create(&UserSubscription{Id: 9302, UserId: 201, PlanId: 9001, StartTime: now - 3600, EndTime: now + 3*24*3600, Status: "active", ExpiryRemindedAt: now - 100}).Error)
	// not due: expires beyond the 5-day window
	require.NoError(t, DB.Create(&UserSubscription{Id: 9303, UserId: 201, PlanId: 9001, StartTime: now - 3600, EndTime: now + ahead + 3600, Status: "active"}).Error)
	// not due: already expired
	require.NoError(t, DB.Create(&UserSubscription{Id: 9304, UserId: 201, PlanId: 9001, StartTime: now - 30*24*3600, EndTime: now - 1, Status: "active"}).Error)
	// not due: cancelled status
	require.NoError(t, DB.Create(&UserSubscription{Id: 9305, UserId: 201, PlanId: 9001, StartTime: now - 3600, EndTime: now + 3*24*3600, Status: "cancelled"}).Error)

	subs, err := ListExpiryReminderDueSubscriptions(now, ahead, 100)
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, 9301, subs[0].Id)
}

func TestMarkExpiryReminderSentOnlyStampsOnce(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	require.NoError(t, DB.Create(&UserSubscription{Id: 9311, UserId: 202, PlanId: 9001, StartTime: now - 3600, EndTime: now + 3*24*3600, Status: "active"}).Error)

	require.NoError(t, MarkExpiryReminderSent(9311, now))
	var sub UserSubscription
	require.NoError(t, DB.Where("id = ?", 9311).First(&sub).Error)
	assert.Equal(t, now, sub.ExpiryRemindedAt)

	// second mark is a no-op: timestamp must not move
	require.NoError(t, MarkExpiryReminderSent(9311, now+100))
	require.NoError(t, DB.Where("id = ?", 9311).First(&sub).Error)
	assert.Equal(t, now, sub.ExpiryRemindedAt)

	// and the subscription no longer shows up as due
	subs, err := ListExpiryReminderDueSubscriptions(now+100, 5*24*3600, 100)
	require.NoError(t, err)
	assert.Empty(t, subs)
}
