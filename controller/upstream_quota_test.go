package controller

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUpstreamQuotaDashboardDoesNotExposeLoadErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("UPSTREAM_QUOTA_REPORT_PATH", filepath.Join(t.TempDir(), "missing.json"))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/system-info/upstream-quota", nil)

	GetUpstreamQuotaDashboard(ctx)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
	assert.Equal(t, "Upstream quota snapshot is unavailable", response.Message)
	assert.NotContains(t, recorder.Body.String(), t.TempDir())
}

func TestGetUpstreamQuotaDashboardReturnsProjectedData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	path := filepath.Join(t.TempDir(), "report.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"generated_at":"2099-07-26T11:59:30Z",
		"entities":[{
			"entity_id":"deepseek-main",
			"display_name":"DeepSeek Main",
			"channel":"deepseek",
			"status":"unknown",
			"reason":"private reason",
			"fetched_at":"2099-07-26T11:59:20Z"
		}]
	}`), 0o600))
	t.Setenv("UPSTREAM_QUOTA_REPORT_PATH", path)
	t.Setenv("UPSTREAM_QUOTA_MAX_AGE_SECONDS", "999999999")

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/system-info/upstream-quota", nil)

	GetUpstreamQuotaDashboard(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"entity_id":"deepseek-main"`)
	assert.Contains(t, recorder.Body.String(), `"status":"unknown"`)
	assert.NotContains(t, recorder.Body.String(), "private reason")
}
