package catfk

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	sweepInterval  = 30 * time.Second
	sweepBatchSize = 100
)

var sweeperOnce sync.Once

// StartSweeper 启动后台兜底轮询:每 30s 扫未发货订单,已付则发货。即使用户关页也能兑现。
// 仿 subscription_reset_task.go,仅主节点跑。
func StartSweeper() {
	sweeperOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			common.SysLog("[catfk] sweeper started: tick=30s")
			ticker := time.NewTicker(sweepInterval)
			defer ticker.Stop()
			for range ticker.C {
				sweepOnce()
			}
		})
	})
}

func sweepOnce() {
	if !setting.CatfkEnabled {
		return
	}
	orders, err := model.ListPendingCatfkOrders(sweepBatchSize)
	if err != nil {
		return
	}
	for _, o := range orders {
		status, _ := CheckAndGrant(o.TradeNo)
		if status != "pending" {
			common.SysLog("[catfk][sweeper] " + o.TradeNo + ": " + status)
		}
	}
}
