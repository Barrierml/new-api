package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/catfk"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

type createCheckoutRequest struct {
	GoodsKey string `json:"goods_key"`
	Pay      string `json:"pay"`     // alipay | wechat,默认 alipay
	Contact  string `json:"contact"` // 可选,默认用户邮箱
}

// CreateCheckout 用户发起一次 catfk 充值下单。用户态(JWT),从 gin context 取当前用户。
func CreateCheckout(c *gin.Context) {
	if !setting.CatfkEnabled {
		common.ApiErrorMsg(c, "充值渠道未启用")
		return
	}
	var req createCheckoutRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	grant, ok := catfk.Goods[req.GoodsKey]
	if !ok {
		common.ApiErrorMsg(c, "未知商品 "+req.GoodsKey)
		return
	}
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pay := req.Pay
	if pay == "" {
		pay = "alipay"
	}
	contact := req.Contact
	if contact == "" {
		contact = user.Email
	}
	channelID, err := catfk.GetPayChannelID(pay)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	tradeNo, payurl, err := catfk.CreateOrder(req.GoodsKey, channelID, contact)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	order := &model.CatfkOrder{
		TradeNo:  tradeNo,
		UserId:   userId,
		GoodsKey: req.GoodsKey,
		Kind:     grant.Kind,
		Value:    grant.Value,
		Pay:      pay,
		PayUrl:   payurl,
	}
	if err := order.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"trade_no": tradeNo, "payurl": payurl})
}

// CheckoutStatus 查订单状态,已付则发货。用户态。校验订单归属当前用户。
func CheckoutStatus(c *gin.Context) {
	tradeNo := c.Query("trade_no")
	if tradeNo == "" {
		common.ApiErrorMsg(c, "缺少 trade_no")
		return
	}
	order, err := model.GetCatfkOrderByTradeNo(tradeNo)
	if err != nil {
		common.ApiErrorMsg(c, "订单不存在")
		return
	}
	if order.UserId != c.GetInt("id") {
		common.ApiErrorMsg(c, "无权访问该订单")
		return
	}
	status, _ := catfk.CheckAndGrant(tradeNo)
	common.ApiSuccess(c, gin.H{"status": status, "kind": order.Kind})
}
