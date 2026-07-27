package model

import (
	"time"
)

// CatfkReplenishEvent 记录一次兑换码自动补货:某商品低于水位时生成了多少码、
// 上架云猫是否成功、手动还是定时触发。供后台「自动补货」看板展示历史。
type CatfkReplenishEvent struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	GoodsKey       string `json:"goods_key" gorm:"type:varchar(32);index"`
	Kind           string `json:"kind" gorm:"type:varchar(16)"` // "plan" | "quota"
	Value          int64  `json:"value"`                        // plan_id 或 quota 数
	CodesGenerated int    `json:"codes_generated"`
	CatfkUploadOk  bool   `json:"catfk_upload_ok"` // 由 service 逻辑显式设置,不用 gorm default
	ErrorMessage   string `json:"error_message" gorm:"type:text"`
	Trigger        string `json:"trigger" gorm:"type:varchar(16)"` // "auto" | "manual"
	CreatedAt      int64  `json:"created_at"`
}

func (e *CatfkReplenishEvent) Insert() error {
	e.CreatedAt = time.Now().Unix()
	return DB.Create(e).Error
}

// ListCatfkReplenishEvents 分页返回补货事件(最新在前),给看板历史表用。
func ListCatfkReplenishEvents(startIdx, num int) (events []CatfkReplenishEvent, total int64, err error) {
	err = DB.Model(&CatfkReplenishEvent{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = DB.Order("id desc").Limit(num).Offset(startIdx).Find(&events).Error
	return events, total, err
}
