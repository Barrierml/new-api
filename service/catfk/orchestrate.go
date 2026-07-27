package catfk

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/bytedance/gopkg/util/gopool"
)

// CheckAndGrant 查订单支付状态,已付则发货 + 作废卡密 + 原子标记 granted。
// 供 controller(用户轮询)和 sweeper(后台兜底)共用。返回状态字符串。
func CheckAndGrant(tradeNo string) (status string, err error) {
	order, err := model.GetCatfkOrderByTradeNo(tradeNo)
	if err != nil {
		return "error", err
	}
	if order.Granted {
		return "granted", nil
	}
	paid, err := QueryPaid(tradeNo)
	if err != nil {
		return "pending", nil // 查询失败当作未付,下次再试
	}
	if !paid {
		return "pending", nil
	}
	// 已付:先原子抢占 granted(防 sweeper 与 controller 并发重复发货)
	// 查已售卡密(排除已归属其它订单的)
	known := collectKnownCards()
	cards, cardErr := SoldCards(order.GoodsKey, known)
	if cardErr != nil {
		common.SysError("[catfk] 查已售卡密失败(不阻断发货): " + cardErr.Error())
		cards = nil
	}
	won, err := model.MarkCatfkOrderGranted(tradeNo, marshalCards(cards))
	if err != nil {
		return "error", err
	}
	if !won {
		// 别的 goroutine 已抢先发货
		return "granted", nil
	}
	if err := Grant(order); err != nil {
		// 发货失败:回滚 granted 让下次重试
		order.Granted = false
		order.Cards = ""
		_ = order.Update()
		common.SysError("[catfk] 发货失败 " + tradeNo + ": " + err.Error())
		return "error", err
	}
	MarkCodesUsed(cards, order.UserId)
	common.SysLog("[catfk] 发货成功 " + tradeNo)
	// 购买触发补货:发货成功 = 该商品 catfk 库存减 1,异步补回(低于水位才补)。
	// 让补货在 sweeper 一个轮询周期(30s)内响应购买,而非等 30min 定时。
	if setting.CatfkReplenishEnabled {
		if g, ok := Goods[order.GoodsKey]; ok {
			gopool.Go(func() { replenishGoods(order.GoodsKey, g, "purchase", false) })
		}
	}
	return "granted", nil
}

// collectKnownCards 汇总所有订单已记录的卡密,避免把一张卡归属给多个订单。
func collectKnownCards() map[string]bool {
	known := map[string]bool{}
	var orders []model.CatfkOrder
	if err := model.DB.Where("cards != ''").Find(&orders).Error; err != nil {
		return known
	}
	for _, o := range orders {
		for _, c := range unmarshalCards(o.Cards) {
			known[c] = true
		}
	}
	return known
}
