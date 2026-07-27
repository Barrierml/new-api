package catfk

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/bytedance/gopkg/util/gopool"
)

const replenishInterval = 30 * time.Minute

var (
	replenishOnce   sync.Once
	lastReplenishAt int64 // atomic;上次巡检完成时间,给看板用
)

// StartReplenisher 启动兑换码自动补货:每 30min 巡检各商品,可用库存低于水位时
// 生成一批永不过期的码并上架云猫。仅主节点跑(仿 StartSweeper)。
func StartReplenisher() {
	replenishOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			common.SysLog("[catfk] replenisher started: tick=30m")
			ticker := time.NewTicker(replenishInterval)
			defer ticker.Stop()
			replenishAll("auto", false)
			for range ticker.C {
				replenishAll("auto", false)
			}
		})
	})
}

// ReplenishOnce 手动触发补货,供 admin 端点调用。
// goodsKey 为空 → 全量巡检(按水位,不强制);否则对该商品强制补一批(忽略水位)。
// 返回本次生成的码总数。
func ReplenishOnce(goodsKey string) int {
	if goodsKey == "" {
		return replenishAll("manual", false)
	}
	g, ok := Goods[goodsKey]
	if !ok {
		return 0
	}
	return replenishGoods(goodsKey, g, "manual", true)
}

// ReplenisherStatus 暴露给「自动补货」看板。
type ReplenisherStatus struct {
	Enabled  bool   `json:"enabled"`
	IsMaster bool   `json:"is_master"`
	Interval int    `json:"interval_sec"`
	LastRunAt int64 `json:"last_run_at"`
	LowWater int    `json:"low_water"`
	Batch    int    `json:"batch"`
}

func ReplenisherStatusNow() ReplenisherStatus {
	return ReplenisherStatus{
		Enabled:   setting.CatfkEnabled && setting.CatfkReplenishEnabled && common.IsMasterNode,
		IsMaster:  common.IsMasterNode,
		Interval:  int(replenishInterval / time.Second),
		LastRunAt: atomic.LoadInt64(&lastReplenishAt),
		LowWater:  setting.CatfkStockLowWater,
		Batch:     setting.CatfkStockBatch,
	}
}

func replenishAll(trigger string, force bool) int {
	if !setting.CatfkEnabled || !setting.CatfkReplenishEnabled {
		return 0
	}
	total := 0
	for gk, g := range Goods {
		total += replenishGoods(gk, g, trigger, force)
	}
	atomic.StoreInt64(&lastReplenishAt, common.GetTimestamp())
	return total
}

// replenishGoods 检查单个商品库存,低于水位(或 force)则生成 batch 个码并上架。返回生成的码数。
func replenishGoods(goodsKey string, g GoodsGrant, trigger string, force bool) int {
	lowWater := setting.CatfkStockLowWater
	batch := setting.CatfkStockBatch
	avail, err := AvailableStock(goodsKey)
	if err != nil {
		common.SysError("[catfk][replenisher] " + goodsKey + " query stock failed: " + err.Error())
		return 0
	}
	if !force && avail >= lowWater {
		return 0
	}
	keys, err := generateCodes(g, batch)
	if err != nil {
		common.SysError("[catfk][replenisher] " + goodsKey + " generate codes failed: " + err.Error())
		recordEvent(goodsKey, g, 0, false, "generate: "+err.Error(), trigger)
		return 0
	}
	uploadErr := ""
	uploadOk := true
	if _, err := AddCards(goodsKey, keys); err != nil {
		uploadOk = false
		uploadErr = err.Error()
		common.SysError("[catfk][replenisher] " + goodsKey + " upload failed: " + uploadErr)
	}
	recordEvent(goodsKey, g, len(keys), uploadOk, uploadErr, trigger)
	common.SysLog("[catfk][replenisher] " + goodsKey + " stock=" + strconv.Itoa(avail) +
		" generated=" + strconv.Itoa(len(keys)) + " upload_ok=" + strconv.FormatBool(uploadOk))
	return len(keys)
}

// generateCodes 直接在 model 层生成 count 个兑换码(ExpiredTime=0 永不过期),返回 key 列表。
// 直接 Insert 进生产库(本进程即生产),不存在"对错库"问题。
func generateCodes(g GoodsGrant, count int) ([]string, error) {
	keys := make([]string, 0, count)
	for i := 0; i < count; i++ {
		key := common.GetUUID()
		r := model.Redemption{
			Name:           "catfk-replenish",
			Key:            key,
			CreatedTime:    common.GetTimestamp(),
			Status:         common.RedemptionCodeStatusEnabled, // 显式设 1,跨库安全
			ExpiredTime:    0,                                  // 永不过期
			RedemptionType: model.RedemptionTypeQuota,
			Quota:          int(g.Value),
		}
		if g.Kind == "plan" {
			r.RedemptionType = model.RedemptionTypeSubscription
			r.SubscriptionPlanId = int(g.Value)
			r.Quota = 0
		}
		if err := r.Insert(); err != nil {
			return keys, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func recordEvent(goodsKey string, g GoodsGrant, generated int, uploadOk bool, errMsg, trigger string) {
	ev := model.CatfkReplenishEvent{
		GoodsKey:       goodsKey,
		Kind:           g.Kind,
		Value:          g.Value,
		CodesGenerated: generated,
		CatfkUploadOk:  uploadOk,
		ErrorMessage:   errMsg,
		Trigger:        trigger,
	}
	if err := ev.Insert(); err != nil {
		common.SysError("[catfk][replenisher] record event failed: " + err.Error())
	}
}
