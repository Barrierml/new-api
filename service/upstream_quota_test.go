package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestLoadUpstreamQuotaDashboardProjectsSanitizedReport(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	reportPath := writeUpstreamQuotaFixture(t, "report.json", `{
		"generated_at":"2026-07-26T11:59:30Z",
		"entities":[{
			"entity_id":"opencode-main",
			"display_name":"OpenCode Main",
			"channel":"opencode",
			"status":"limited",
			"reason":"private provider detail",
			"error":"private upstream error",
			"route_ids":[100],
			"endpoint_hosts":["private.example"],
			"windows":[{
				"key":"weekly",
				"kind":"quota",
				"label":"Weekly",
				"unit":"tokens",
				"limit":100,
				"used":80,
				"remaining":20,
				"remaining_pct":20,
				"utilization_pct":80,
				"reset_at":1785146400,
				"duration_seconds":604800
			}],
			"fetched_at":"2026-07-26T11:59:20Z"
		}]
	}`)
	ownershipPath := writeUpstreamQuotaFixture(t, "ownership.json", `{
		"entities":{"opencode-main":{"account_ids":[100],"group_ids":[39],"channel_ids":[25]}}
	}`)

	dashboard, err := LoadUpstreamQuotaDashboard(reportPath, ownershipPath, now, 4*time.Minute)
	require.NoError(t, err)
	require.Len(t, dashboard.Entities, 1)
	entity := dashboard.Entities[0]
	assert.False(t, dashboard.Stale)
	assert.False(t, entity.Stale)
	assert.Equal(t, "limited", entity.Status)
	assert.Equal(t, "Quota is limited", entity.StatusMessage)
	assert.Equal(t, []int64{100}, entity.AccountIDs)
	assert.Equal(t, []int64{39}, entity.GroupIDs)
	assert.Equal(t, []int64{25}, entity.ChannelIDs)
	assert.Equal(t, 1, dashboard.Counts.Limited)

	encoded, err := common.Marshal(dashboard)
	require.NoError(t, err)
	body := string(encoded)
	assert.NotContains(t, body, "private provider detail")
	assert.NotContains(t, body, "private upstream error")
	assert.NotContains(t, body, "private.example")
	assert.NotContains(t, body, "utilization_pct")
}

func TestLoadUpstreamQuotaDashboardProjectsReviewedChannelPriorities(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	reportPath := writeUpstreamQuotaFixture(t, "report.json", `{
		"generated_at":"2026-07-26T11:59:30Z",
		"entities":[{
			"entity_id":"opencode-main",
			"display_name":"OpenCode Main",
			"channel":"opencode",
			"status":"available",
			"route_ids":[999],
			"fetched_at":"2026-07-26T11:59:20Z"
		}]
	}`)
	ownershipPath := writeUpstreamQuotaFixture(t, "ownership.json", `{
		"entities":{"opencode-main":{"channel_ids":[25,12,99,404]}}
	}`)

	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "quota.db")), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.AutoMigrate(&model.Channel{}))

	baseURL := "https://private.example"
	setting := `{"secret":"setting"}`
	headerOverride := `{"Authorization":"Bearer private-header"}`
	paramOverride := `{"private":"parameter"}`
	priorityHigh := int64(30)
	priorityLow := int64(10)
	require.NoError(t, db.Create([]model.Channel{
		{Id: 25, Name: "Secondary", Status: 1, Priority: &priorityLow, Key: "private-key-low", BaseURL: &baseURL, Setting: &setting},
		{Id: 12, Name: "Primary B", Status: 1, Priority: &priorityHigh, Key: "private-key-b", HeaderOverride: &headerOverride},
		{Id: 99, Name: "Primary A", Status: 2, Priority: &priorityHigh, Key: "private-key-a", ParamOverride: &paramOverride},
		{Id: 999, Name: "Unreviewed", Status: 1, Priority: &priorityHigh, Key: "private-key-unreviewed"},
	}).Error)

	dashboard, err := LoadUpstreamQuotaDashboard(reportPath, ownershipPath, now, 4*time.Minute)
	require.NoError(t, err)
	require.Len(t, dashboard.Entities, 1)
	assert.Equal(t, []UpstreamQuotaChannel{
		{ID: 12, Name: "Primary B", Priority: 30, Status: 1},
		{ID: 99, Name: "Primary A", Priority: 30, Status: 2},
		{ID: 25, Name: "Secondary", Priority: 10, Status: 1},
	}, dashboard.Entities[0].Channels)

	encoded, err := common.Marshal(dashboard)
	require.NoError(t, err)
	body := string(encoded)
	assert.NotContains(t, body, "Unreviewed")
	assert.NotContains(t, body, "private-key")
	assert.NotContains(t, body, "private.example")
	assert.NotContains(t, body, "private-header")
	assert.NotContains(t, body, "parameter")
	assert.NotContains(t, body, "setting")
}

func TestLoadUpstreamQuotaDashboardIgnoresUnreviewedRouteIDs(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	reportPath := writeUpstreamQuotaFixture(t, "report.json", `{
		"generated_at":"2026-07-26T11:59:30Z",
		"entities":[{
			"entity_id":"unmapped",
			"status":"available",
			"route_ids":[999],
			"fetched_at":"2026-07-26T11:59:20Z"
		}]
	}`)
	ownershipPath := writeUpstreamQuotaFixture(t, "ownership.json", `{"entities":{}}`)

	dashboard, err := LoadUpstreamQuotaDashboard(reportPath, ownershipPath, now, 4*time.Minute)
	require.NoError(t, err)
	require.Len(t, dashboard.Entities, 1)
	assert.Empty(t, dashboard.Entities[0].AccountIDs)

	encoded, err := common.Marshal(dashboard)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "999")
	assert.NotContains(t, string(encoded), "account_ids")
}

func TestLoadUpstreamQuotaDashboardMarksStaleWithoutTreatingAsHealthy(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	reportPath := writeUpstreamQuotaFixture(t, "report.json", `{
		"generated_at":"2026-07-26T11:50:00Z",
		"entities":[{
			"entity_id":"mimo-cn",
			"display_name":"MiMo CN",
			"channel":"mimo",
			"status":"available",
			"route_ids":[88],
			"fetched_at":"2026-07-26T11:59:30Z"
		}]
	}`)

	dashboard, err := LoadUpstreamQuotaDashboard(reportPath, "", now, 4*time.Minute)
	require.NoError(t, err)
	require.Len(t, dashboard.Entities, 1)
	assert.True(t, dashboard.Stale)
	assert.True(t, dashboard.Entities[0].Stale)
	assert.Equal(t, "Quota snapshot is stale", dashboard.Entities[0].StatusMessage)
	assert.Equal(t, 1, dashboard.Counts.Stale)
	assert.Zero(t, dashboard.Counts.Available)
}

func TestLoadUpstreamQuotaDashboardPreservesExplicitUnhealthyStates(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	reportPath := writeUpstreamQuotaFixture(t, "report.json", `{
		"generated_at":"2026-07-26T11:59:30Z",
		"entities":[
			{"entity_id":"unknown","status":"unknown","fetched_at":"2026-07-26T11:59:20Z"},
			{"entity_id":"error","status":"error","fetched_at":"2026-07-26T11:59:20Z"},
			{"entity_id":"deferred","status":"deferred","fetched_at":"2026-07-26T11:59:20Z"},
			{"entity_id":"future","status":"future-status","fetched_at":"2026-07-26T11:59:20Z"}
		]
	}`)

	dashboard, err := LoadUpstreamQuotaDashboard(reportPath, "", now, 4*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 2, dashboard.Counts.Unknown)
	assert.Equal(t, 1, dashboard.Counts.Error)
	assert.Equal(t, 1, dashboard.Counts.Deferred)
	assert.Zero(t, dashboard.Counts.Available)
	assert.Equal(t, "unknown", dashboard.Entities[3].Status)
}

func TestLoadUpstreamQuotaDashboardRejectsMissingAndMalformedReports(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)

	_, err := LoadUpstreamQuotaDashboard(filepath.Join(t.TempDir(), "missing.json"), "", now, 4*time.Minute)
	require.ErrorContains(t, err, "read quota report")

	malformedPath := writeUpstreamQuotaFixture(t, "malformed.json", `{`)
	_, err = LoadUpstreamQuotaDashboard(malformedPath, "", now, 4*time.Minute)
	require.ErrorContains(t, err, "decode quota report")

	missingGeneratedAtPath := writeUpstreamQuotaFixture(t, "missing-generated-at.json", `{"entities":[]}`)
	_, err = LoadUpstreamQuotaDashboard(missingGeneratedAtPath, "", now, 4*time.Minute)
	require.ErrorContains(t, err, "no generated_at")
}

func TestLoadUpstreamQuotaDashboardRejectsInvalidConfigurationAndDuplicateEntities(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	reportPath := writeUpstreamQuotaFixture(t, "report.json", `{
		"generated_at":"2026-07-26T11:59:30Z",
		"entities":[
			{"entity_id":"duplicate","fetched_at":"2026-07-26T11:59:20Z"},
			{"entity_id":"duplicate","fetched_at":"2026-07-26T11:59:20Z"}
		]
	}`)

	_, err := LoadUpstreamQuotaDashboard(reportPath, "", now, 0)
	require.ErrorContains(t, err, "max age must be positive")

	_, err = LoadUpstreamQuotaDashboard(reportPath, "", time.Time{}, 4*time.Minute)
	require.ErrorContains(t, err, "current time is required")

	_, err = LoadUpstreamQuotaDashboard(reportPath, "", now, 4*time.Minute)
	require.ErrorContains(t, err, "duplicate entity_id")

	missingEntityIDPath := writeUpstreamQuotaFixture(t, "missing-entity-id.json", `{
		"generated_at":"2026-07-26T11:59:30Z",
		"entities":[{"fetched_at":"2026-07-26T11:59:20Z"}]
	}`)
	_, err = LoadUpstreamQuotaDashboard(missingEntityIDPath, "", now, 4*time.Minute)
	require.ErrorContains(t, err, "no entity_id")
}

func writeUpstreamQuotaFixture(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
