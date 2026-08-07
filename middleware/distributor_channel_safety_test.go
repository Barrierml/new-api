package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTokenChannelSafetyLimitEndToEnd(t *testing.T) {
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()

	db, err := gorm.Open(sqlite.Open("file:token-channel-safety-e2e?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSubscription{}, &model.Token{}, &model.Channel{}, &model.Ability{}))
	model.DB = db
	model.LOG_DB = db
	common.MemoryCacheEnabled = true
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, i18n.Init())
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
	})

	user := &model.User{
		Username: "safety-e2e-user",
		Password: "password-placeholder",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
		Quota:    100000,
		Group:    "default",
		AffCode:  "safety-e2e-aff",
	}
	require.NoError(t, db.Create(user).Error)
	allowUnsafeChannels := false
	token := &model.Token{
		UserId:              user.Id,
		Key:                 "safetye2etoken",
		Name:                "safety-e2e-key",
		Status:              common.TokenStatusEnabled,
		ExpiredTime:         -1,
		UnlimitedQuota:      true,
		Group:               "default",
		AllowUnsafeChannels: &allowUnsafeChannels,
	}
	require.NoError(t, db.Create(token).Error)

	highPriority := int64(100)
	lowPriority := int64(50)
	unsafeTag := "无法验证安全性"
	safeTag := model.ChannelTagSafe
	weight := uint(100)
	channels := []model.Channel{
		{Id: 401, Name: "unsafe-high-priority", Status: common.ChannelStatusEnabled, Models: "safety-e2e-model", Group: "default", Priority: &highPriority, Weight: &weight, Tag: &unsafeTag},
		{Id: 402, Name: "safe-low-priority", Status: common.ChannelStatusEnabled, Models: "safety-e2e-model", Group: "default", Priority: &lowPriority, Weight: &weight, Tag: &safeTag},
	}
	require.NoError(t, db.Create(&channels).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "safety-e2e-model", ChannelId: 401, Enabled: true, Priority: &highPriority, Weight: weight},
		{Group: "default", Model: "safety-e2e-model", ChannelId: 402, Enabled: true, Priority: &lowPriority, Weight: weight},
	}).Error)
	model.InitChannelCache()
	service.ClearChannelAffinityCacheAll()
	t.Cleanup(func() {
		service.ClearChannelAffinityCacheAll()
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/responses", TokenAuth(), Distribute(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"channel_id": c.GetInt("channel_id")})
	})

	requestChannel := func(authorization string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"safety-e2e-model"}`))
		request.Header.Set("Authorization", authorization)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	response := requestChannel("Bearer sk-" + token.Key)
	assert.Equal(t, http.StatusOK, response.Code)
	var successBody struct {
		ChannelID int `json:"channel_id"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &successBody))
	assert.Equal(t, 402, successBody.ChannelID)

	response = requestChannel("Bearer sk-" + token.Key + "-401")
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	var errorBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &errorBody))
	assert.Equal(t, string(types.ErrorCodeChannelSafetyLimitExceeded), errorBody.Error.Code)

	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 402).Update("tag", "").Error)
	model.InitChannelCache()
	response = requestChannel("Bearer sk-" + token.Key)
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &errorBody))
	assert.Equal(t, string(types.ErrorCodeChannelSafetyLimitExceeded), errorBody.Error.Code)

	allowUnsafeChannels = true
	require.NoError(t, db.Model(token).Update("allow_unsafe_channels", allowUnsafeChannels).Error)
	response = requestChannel("Bearer sk-" + token.Key)
	assert.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &successBody))
	assert.Equal(t, 401, successBody.ChannelID)
}
