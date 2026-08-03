package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTokenPricingLimitUsesOnlyComparableTokenPricing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	savedModelPrices := ratio_setting.ModelPrice2JSONString()
	savedModelRatios := ratio_setting.ModelRatio2JSONString()
	savedSelfUseMode := operation_setting.SelfUseModeEnabled
	savedConfig := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		savedConfig[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedModelPrices))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedModelRatios))
		operation_setting.SelfUseModeEnabled = savedSelfUseMode
		require.NoError(t, config.GlobalConfig.LoadFromDB(savedConfig))
	})

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"fixed-model":0.02}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"ratio-model":2}`))
	operation_setting.SelfUseModeEnabled = false
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"tiered-model":"tiered_expr"}`,
	}))

	newContext := func(acceptUnset bool) *gin.Context {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		common.SetContextKey(ctx, constant.ContextKeyTokenMaxChannelRatio, 10.0)
		common.SetContextKey(ctx, constant.ContextKeyTokenMaxInputPrice, 999.0)
		common.SetContextKey(ctx, constant.ContextKeyUserSetting, dto.UserSetting{AcceptUnsetRatioModel: acceptUnset})
		return ctx
	}

	ratioLimit := ResolveTokenPricingLimit(newContext(false), "ratio-model")
	assert.True(t, ratioLimit.InputPriceComparable)
	assert.Equal(t, 4.0, ratioLimit.BaseInputPrice)

	assert.False(t, ResolveTokenPricingLimit(newContext(false), "fixed-model").InputPriceComparable)
	assert.False(t, ResolveTokenPricingLimit(newContext(false), "tiered-model").InputPriceComparable)
	assert.False(t, ResolveTokenPricingLimit(newContext(false), "unset-model").InputPriceComparable)
	zeroLimitContext := newContext(false)
	common.SetContextKey(zeroLimitContext, constant.ContextKeyTokenMaxInputPrice, 0.0)
	assert.False(t, ResolveTokenPricingLimit(zeroLimitContext, "fixed-model").Evaluate(20).Blocked)

	unsetLimit := ResolveTokenPricingLimit(newContext(true), "unset-model")
	assert.True(t, unsetLimit.InputPriceComparable)
	assert.Equal(t, 75.0, unsetLimit.BaseInputPrice)
}
