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

func TestTokenChannelRatioLimitEndToEnd(t *testing.T) {
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()

	db, err := gorm.Open(sqlite.Open("file:token-channel-ratio-e2e?mode=memory&cache=shared"), &gorm.Config{})
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
		Username: "ratio-e2e-user",
		Password: "password-placeholder",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    100000,
		Group:    "default",
		AffCode:  "ratio-e2e-aff",
	}
	require.NoError(t, db.Create(user).Error)
	maxChannelRatio := 3.0
	token := &model.Token{
		UserId:          user.Id,
		Key:             "ratioe2etoken",
		Name:            "ratio-e2e-key",
		Status:          common.TokenStatusEnabled,
		ExpiredTime:     -1,
		UnlimitedQuota:  true,
		Group:           "default",
		MaxChannelRatio: &maxChannelRatio,
	}
	require.NoError(t, db.Create(token).Error)

	highPriority := int64(100)
	lowPriority := int64(50)
	expensiveRatio := 2.0
	cheapRatio := 1.0
	weight := uint(100)
	channels := []model.Channel{
		{Id: 201, Name: "expensive", Status: common.ChannelStatusEnabled, Models: "gpt-ratio-e2e", Group: "default", Priority: &highPriority, Weight: &weight, Ratio: &expensiveRatio},
		{Id: 202, Name: "cheap", Status: common.ChannelStatusEnabled, Models: "gpt-ratio-e2e", Group: "default", Priority: &lowPriority, Weight: &weight, Ratio: &cheapRatio},
	}
	require.NoError(t, db.Create(&channels).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "gpt-ratio-e2e", ChannelId: 201, Enabled: true, Priority: &highPriority, Weight: weight},
		{Group: "default", Model: "gpt-ratio-e2e", ChannelId: 202, Enabled: true, Priority: &lowPriority, Weight: weight},
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

	requestBody := `{"model":"gpt-ratio-e2e","prompt_cache_key":"ratio-affinity-e2e"}`
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(requestBody))
	request.Header.Set("Authorization", "Bearer sk-"+token.Key)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	var successBody struct {
		ChannelID int `json:"channel_id"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &successBody))
	assert.Equal(t, 201, successBody.ChannelID)

	maxChannelRatio = 1.5
	require.NoError(t, db.Model(token).Update("max_channel_ratio", maxChannelRatio).Error)
	request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(requestBody))
	request.Header.Set("Authorization", "Bearer sk-"+token.Key)
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &successBody))
	assert.Equal(t, 202, successBody.ChannelID)

	tooLowRatio := 0.5
	require.NoError(t, db.Model(token).Update("max_channel_ratio", tooLowRatio).Error)
	request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(requestBody))
	request.Header.Set("Authorization", "Bearer sk-"+token.Key)
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	var errorBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &errorBody))
	assert.Equal(t, string(types.ErrorCodeChannelRatioLimitExceeded), errorBody.Error.Code)
}
