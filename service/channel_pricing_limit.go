package service

import (
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const inputTokensPerMillion = 1_000_000

func ResolveTokenPricingLimit(c *gin.Context, originalModel string) model.ChannelPricingLimit {
	limit := model.ChannelPricingLimit{
		MaxChannelRatio: common.GetContextKeyTypeOrDefault[float64](c, constant.ContextKeyTokenMaxChannelRatio, 0),
		MaxInputPrice:   common.GetContextKeyTypeOrDefault[float64](c, constant.ContextKeyTokenMaxInputPrice, 0),
	}
	if limit.MaxChannelRatio <= 0 || billing_setting.GetBillingMode(originalModel) == billing_setting.BillingModeTieredExpr {
		return limit
	}
	if _, usePrice := ratio_setting.GetModelPrice(originalModel, false); usePrice {
		return limit
	}

	modelRatio, configured, _ := ratio_setting.GetModelRatio(originalModel)
	if !configured {
		userSetting, ok := common.GetContextKeyType[dto.UserSetting](c, constant.ContextKeyUserSetting)
		if !ok || !userSetting.AcceptUnsetRatioModel {
			return limit
		}
	}
	if modelRatio < 0 || math.IsNaN(modelRatio) || math.IsInf(modelRatio, 0) ||
		common.QuotaPerUnit <= 0 || math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) {
		return limit
	}

	limit.BaseInputPrice = decimal.NewFromFloat(modelRatio).
		Mul(decimal.NewFromInt(inputTokensPerMillion)).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		InexactFloat64()
	limit.InputPriceComparable = true
	return limit
}

func PricingLimitForGroup(limit model.ChannelPricingLimit, userGroup, group string) model.ChannelPricingLimit {
	return limit.WithGroupRatio(GetUserGroupRatio(userGroup, group))
}

func CurrentPricingGroup(c *gin.Context, fallback string) string {
	if autoGroup := common.GetContextKeyString(c, constant.ContextKeyAutoGroup); autoGroup != "" {
		return autoGroup
	}
	if usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup); usingGroup != "" {
		return usingGroup
	}
	return fallback
}
