package catfk

import (
	"encoding/json"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// Grant 按订单发货:quota 档加余额,plan 档续订套餐。复用后端 model 层函数(同进程,
// 不走 admin HTTP)。返回发货是否成功。
func Grant(order *model.CatfkOrder) error {
	if order.Kind == "plan" {
		_, _, err := model.AdminBindSubscription(order.UserId, int(order.Value),
			model.AdminGrantOptions{Mode: model.SubscriptionGrantRenew})
		return err
	}
	// quota 档:IncreaseUserQuota 的 quota 形参是 int
	return model.IncreaseUserQuota(order.UserId, int(order.Value), true)
}

// MarkCodesUsed 把 catfk 发的卡密对应的 new-api 兑换码置为已用,防买家自动发货后再手动兑换。
// 替代原 Python 的 docker exec psql。
func MarkCodesUsed(codes []string, userId int) {
	if len(codes) == 0 {
		return
	}
	now := time.Now().Unix()
	model.DB.Model(&model.Redemption{}).
		Where("key IN ? AND status = ?", codes, common.RedemptionCodeStatusEnabled).
		Updates(map[string]interface{}{
			"status":        common.RedemptionCodeStatusUsed,
			"used_user_id":  userId,
			"redeemed_time": now,
		})
}

// marshalCards 把卡密列表转 JSON 存进订单的 Cards 字段。
func marshalCards(cards []string) string {
	b, _ := json.Marshal(cards)
	return string(b)
}

// unmarshalCards 解析订单 Cards 字段的 JSON 数组。
func unmarshalCards(s string) []string {
	if s == "" {
		return nil
	}
	var cards []string
	_ = json.Unmarshal([]byte(s), &cards)
	return cards
}
