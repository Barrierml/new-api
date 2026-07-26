package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRetiredFrontendAPIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	_, hasQuotaDashboard := routes[http.MethodGet+" /api/system-info/upstream-quota"]
	_, hasAsyncCleanup := routes[http.MethodPost+" /api/system-task/log-cleanup"]
	_, hasDirectDelete := routes[http.MethodDelete+" /api/log/"]
	_, hasConsoleMigration := routes[http.MethodPost+" /api/option/migrate_console_setting"]
	assert.True(t, hasQuotaDashboard)
	assert.True(t, hasAsyncCleanup)
	assert.False(t, hasDirectDelete)
	assert.False(t, hasConsoleMigration)
}

func TestUpstreamQuotaDashboardRequiresRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
	})

	commonUserToken := "quota-dashboard-common-user"
	commonUser := &model.User{
		Username:    "quota-dashboard-user",
		Password:    "password-placeholder",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AccessToken: &commonUserToken,
		AuthVersion: 1,
		AffCode:     "quota-dashboard-user-aff",
	}
	require.NoError(t, model.DB.Create(commonUser).Error)
	rootUserToken := "quota-dashboard-root-user"
	rootUser := &model.User{
		Username:    "quota-dashboard-root",
		Password:    "password-placeholder",
		Role:        common.RoleRootUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AccessToken: &rootUserToken,
		AuthVersion: 1,
		AffCode:     "quota-dashboard-root-aff",
	}
	require.NoError(t, model.DB.Create(rootUser).Error)

	fixtureDir := t.TempDir()
	reportPath := filepath.Join(fixtureDir, "report.json")
	ownershipPath := filepath.Join(fixtureDir, "ownership.json")
	require.NoError(t, os.WriteFile(reportPath, []byte(`{
		"generated_at":"2026-07-26T11:59:30Z",
		"entities":[]
	}`), 0o600))
	require.NoError(t, os.WriteFile(ownershipPath, []byte(`{"entities":{}}`), 0o600))
	t.Setenv("UPSTREAM_QUOTA_REPORT_PATH", reportPath)
	t.Setenv("UPSTREAM_QUOTA_OWNERSHIP_PATH", ownershipPath)
	t.Setenv("UPSTREAM_QUOTA_MAX_AGE_SECONDS", "315360000")

	engine := gin.New()
	SetApiRouter(engine)

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "anonymous", wantStatus: http.StatusUnauthorized},
		{name: "common user", token: commonUserToken, wantStatus: http.StatusForbidden},
		{name: "root user", token: rootUserToken, wantStatus: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/system-info/upstream-quota", nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()

			engine.ServeHTTP(response, request)

			assert.Equal(t, test.wantStatus, response.Code)
		})
	}
}
