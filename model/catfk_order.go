package model

import (
	"time"

	"gorm.io/gorm"
)

// CatfkOrder 记录一次云猫寄售(catfk)充值订单。替代原 catfk-checkout.py 的本地 json 文件。
// 发货(加 quota / 绑套餐)成功后 Granted 置 true;Cards 存已作废的兑换码(防重兑记录)。
type CatfkOrder struct {
	Id        int    `json:"id"`
	TradeNo   string `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	UserId    int    `json:"user_id" gorm:"index"`
	GoodsKey  string `json:"goods_key" gorm:"type:varchar(64)"`
	Kind      string `json:"kind" gorm:"type:varchar(16)"`   // "plan" | "quota"
	Value     int64  `json:"value"`                          // plan_id 或 quota 数
	Pay       string `json:"pay" gorm:"type:varchar(16)"`    // "alipay" | "wechat"
	PayUrl    string `json:"payurl" gorm:"type:text"`
	Granted   bool   `json:"granted" gorm:"default:false;index"`
	Cards     string `json:"cards" gorm:"type:text"`         // 已发/作废卡密,JSON 数组
	CreatedAt int64  `json:"created_at"`
}

func (o *CatfkOrder) Insert() error {
	o.CreatedAt = time.Now().Unix()
	return DB.Create(o).Error
}

func (o *CatfkOrder) Update() error {
	return DB.Save(o).Error
}

func GetCatfkOrderByTradeNo(tradeNo string) (*CatfkOrder, error) {
	var order CatfkOrder
	err := DB.Where("trade_no = ?", tradeNo).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// ListPendingCatfkOrders 返回未发货的订单(sweeper 轮询用)。限量避免全表扫。
func ListPendingCatfkOrders(limit int) ([]CatfkOrder, error) {
	var orders []CatfkOrder
	err := DB.Where("granted = ?", false).Order("id asc").Limit(limit).Find(&orders).Error
	return orders, err
}

// MarkCatfkOrderGranted 原子地把订单标记为已发货,返回是否成功抢到(防 sweeper 与
// CheckoutStatus 并发重复发货)。只有当前 granted=false 时才更新成功。
func MarkCatfkOrderGranted(tradeNo string, cards string) (bool, error) {
	res := DB.Model(&CatfkOrder{}).
		Where("trade_no = ? AND granted = ?", tradeNo, false).
		Updates(map[string]interface{}{"granted": true, "cards": cards})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

var _ = gorm.ErrRecordNotFound
