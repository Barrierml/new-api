package controller

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/catfk"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

// 概览缓存:stock/orphan 每次都要打云猫 API(11 商品),缓存 5min 降频,避免触发云猫限流。
const overviewCacheTTL = 5 * time.Minute

var (
	overviewMu       sync.Mutex
	overviewCache    gin.H
	overviewCachedAt time.Time
)

type replenishStockRow struct {
	GoodsKey  string `json:"goods_key"`
	Kind      string `json:"kind"`
	Available int    `json:"available"` // -1 表示查询失败
	LowWater  int    `json:"low_water"`
}

// GetCatfkReplenishOverview 返回 daemon 状态 + 各商品可用库存 + 兑换码健康度 + 孤儿码数。
// 孤儿码 = 云猫货架有但生产 redemptions 表没有(本次事故根因指标)。
func GetCatfkReplenishOverview(c *gin.Context) {
	overviewMu.Lock()
	cached := overviewCache
	fresh := time.Since(overviewCachedAt) < overviewCacheTTL
	overviewMu.Unlock()
	if fresh && cached != nil {
		common.ApiSuccess(c, cached)
		return
	}

	status := catfk.ReplenisherStatusNow()
	lowWater := setting.CatfkStockLowWater

	var stock []replenishStockRow
	allSecrets := make([]string, 0)
	catfkErr := ""
	for gk, g := range catfk.Goods {
		avail, secrets, err := catfk.AvailableStockAndSecrets(gk)
		if err != nil {
			if catfkErr == "" {
				catfkErr = err.Error()
			}
			common.SysError("[catfk-replenish] overview stock query " + gk + " failed: " + err.Error())
			stock = append(stock, replenishStockRow{GoodsKey: gk, Kind: g.Kind, Available: -1, LowWater: lowWater})
			continue
		}
		stock = append(stock, replenishStockRow{GoodsKey: gk, Kind: g.Kind, Available: avail, LowWater: lowWater})
		allSecrets = append(allSecrets, secrets...)
	}

	enabledValid, used, disabled, expired, _ := model.RedemptionHealthCounts()

	orphanCount := -1
	if len(allSecrets) > 0 {
		if existing, err := model.ExistingRedemptionKeys(allSecrets); err == nil {
			orphans := 0
			for _, s := range allSecrets {
				if !existing[s] {
					orphans++
				}
			}
			orphanCount = orphans
		}
	}

	data := gin.H{
		"status":       status,
		"stock":        stock,
		"health":       gin.H{"valid": enabledValid, "used": used, "disabled": disabled, "expired": expired},
		"orphan_count": orphanCount,
		"generated_at": time.Now().Unix(),
	}
	if catfkErr != "" {
		data["catfk_error"] = catfkErr
	}

	overviewMu.Lock()
	overviewCache = data
	overviewCachedAt = time.Now()
	overviewMu.Unlock()

	common.ApiSuccess(c, data)
}

func GetCatfkReplenishEvents(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	events, total, err := model.ListCatfkReplenishEvents(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(events)
	common.ApiSuccess(c, pageInfo)
}

// RunCatfkReplenish 手动触发补货。goods_key 为空 → 全量巡检(按水位);否则对该商品强制补一批。
func RunCatfkReplenish(c *gin.Context) {
	var req struct {
		GoodsKey string `json:"goods_key"`
	}
	_ = c.ShouldBindJSON(&req)
	generated := catfk.ReplenishOnce(req.GoodsKey)
	common.ApiSuccess(c, gin.H{"generated": generated, "goods_key": req.GoodsKey})
}

// UpdateCatfkReplenishConfig 热更新补货开关/水位/批量。走 model.UpdateOptionsBulk(同系统设置)。
func UpdateCatfkReplenishConfig(c *gin.Context) {
	var req struct {
		ReplenishEnabled *bool `json:"replenish_enabled"`
		LowWater         *int  `json:"low_water"`
		Batch            *int  `json:"batch"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	values := map[string]string{}
	if req.ReplenishEnabled != nil {
		values["CatfkReplenishEnabled"] = strconv.FormatBool(*req.ReplenishEnabled)
	}
	if req.LowWater != nil {
		if *req.LowWater < 0 || *req.LowWater > 1000 {
			common.ApiError(c, fmt.Errorf("low_water out of range (0-1000)"))
			return
		}
		values["CatfkStockLowWater"] = strconv.Itoa(*req.LowWater)
	}
	if req.Batch != nil {
		if *req.Batch <= 0 || *req.Batch > 100 { // 与 AddRedemption 上限一致
			common.ApiError(c, fmt.Errorf("batch out of range (1-100)"))
			return
		}
		values["CatfkStockBatch"] = strconv.Itoa(*req.Batch)
	}
	if len(values) == 0 {
		common.ApiSuccess(c, gin.H{"updated": false})
		return
	}
	if err := model.UpdateOptionsBulk(values); err != nil {
		common.ApiError(c, err)
		return
	}
	overviewMu.Lock() // 配置变了,作废概览缓存
	overviewCache = nil
	overviewMu.Unlock()
	common.ApiSuccess(c, gin.H{"updated": true, "config": values})
}
