package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSumConsumeQuotaByUserAggregatesOnlyRangeConsume(t *testing.T) {
	truncateTables(t)

	const dayStart = 1753401600 // fixed window start
	seed := []*Log{
		// user 1: two consume logs inside the window
		{UserId: 1, Username: "u1", ModelName: "m", Type: LogTypeConsume, CreatedAt: dayStart + 10, Quota: 100, PromptTokens: 10, CompletionTokens: 5},
		{UserId: 1, Username: "u1", ModelName: "m", Type: LogTypeConsume, CreatedAt: dayStart + 20, Quota: 200, PromptTokens: 20, CompletionTokens: 15},
		// user 2: one consume inside, one consume before the window, one error inside
		{UserId: 2, Username: "u2", ModelName: "m", Type: LogTypeConsume, CreatedAt: dayStart + 30, Quota: 50, PromptTokens: 7, CompletionTokens: 3},
		{UserId: 2, Username: "u2", ModelName: "m", Type: LogTypeConsume, CreatedAt: dayStart - 10, Quota: 999, PromptTokens: 999, CompletionTokens: 999},
		{UserId: 2, Username: "u2", ModelName: "m", Type: LogTypeError, CreatedAt: dayStart + 40, Quota: 777, PromptTokens: 777, CompletionTokens: 777},
	}
	for _, l := range seed {
		require.NoError(t, LOG_DB.Create(l).Error)
	}

	rows, err := SumConsumeQuotaByUser(dayStart, dayStart+3600)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byUser := make(map[int]UserDailyConsumeStat, len(rows))
	for _, r := range rows {
		byUser[r.UserId] = r
	}

	u1 := byUser[1]
	assert.Equal(t, int64(300), u1.Quota)
	assert.Equal(t, int64(30), u1.PromptTokens)
	assert.Equal(t, int64(20), u1.CompletionTokens)
	assert.Equal(t, int64(2), u1.RequestCount)
	assert.Equal(t, int64(dayStart+20), u1.LastUsedAt)

	u2 := byUser[2]
	assert.Equal(t, int64(50), u2.Quota, "昨天和 error 日志都不能算进来")
	assert.Equal(t, int64(7), u2.PromptTokens)
	assert.Equal(t, int64(3), u2.CompletionTokens)
	assert.Equal(t, int64(1), u2.RequestCount)
	assert.Equal(t, int64(dayStart+30), u2.LastUsedAt)
}

func TestGetRecentLogsFiltersTypeOrdersDescAndAttachesUser(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{Id: 9, Username: "alice", DisplayName: "Alice A", Email: "alice@example.com", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}).Error)

	const base = 1753401600
	seed := []*Log{
		{UserId: 9, Username: "alice", Type: LogTypeError, CreatedAt: base + 1, Content: "e1"},
		{UserId: 9, Username: "alice", Type: LogTypeConsume, CreatedAt: base + 2, ModelName: "m1"},
		{UserId: 9, Username: "alice", Type: LogTypeError, CreatedAt: base + 3, Content: "e2"},
		{UserId: 9, Username: "alice", Type: LogTypeConsume, CreatedAt: base + 4, ModelName: "m2"},
		{UserId: 9, Username: "alice", Type: LogTypeError, CreatedAt: base + 5, Content: "e3"},
	}
	for _, l := range seed {
		require.NoError(t, LOG_DB.Create(l).Error)
	}

	logs, err := GetRecentLogs(LogTypeError, 2)
	require.NoError(t, err)
	require.Len(t, logs, 2, "limit 必须生效")
	for _, l := range logs {
		assert.Equal(t, LogTypeError, l.Type)
	}
	assert.Equal(t, "e3", logs[0].Content, "必须按 created_at desc 排序")
	assert.Equal(t, "e2", logs[1].Content)
	assert.Equal(t, "Alice A", logs[0].UserDisplayName)
	assert.Equal(t, "alice@example.com", logs[0].UserEmail)

	consumeLogs, err := GetRecentLogs(LogTypeConsume, 10)
	require.NoError(t, err)
	require.Len(t, consumeLogs, 2)
	assert.Equal(t, "m2", consumeLogs[0].ModelName)
}
