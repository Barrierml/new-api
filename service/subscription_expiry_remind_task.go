package service

import (
	"context"
	"fmt"
	"html"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	subscriptionExpiryRemindTickInterval = time.Hour
	subscriptionExpiryRemindAheadSeconds = 5 * 24 * 3600 // 提前 5 天提醒
	subscriptionExpiryRemindBatchSize    = 200
)

var (
	subscriptionExpiryRemindOnce    sync.Once
	subscriptionExpiryRemindRunning atomic.Bool

	// 邮件里的到期时间固定按北京时间展示(用户主要在国内)。
	subscriptionExpiryRemindLoc = func() *time.Location {
		if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
			return loc
		}
		return time.FixedZone("CST", 8*3600)
	}()
)

func StartSubscriptionExpiryRemindTask() {
	subscriptionExpiryRemindOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("subscription expiry remind task started: tick=%s ahead=%ds", subscriptionExpiryRemindTickInterval, subscriptionExpiryRemindAheadSeconds))
			ticker := time.NewTicker(subscriptionExpiryRemindTickInterval)
			defer ticker.Stop()

			runSubscriptionExpiryRemindOnce()
			for range ticker.C {
				runSubscriptionExpiryRemindOnce()
			}
		})
	})
}

func runSubscriptionExpiryRemindOnce() {
	if !subscriptionExpiryRemindRunning.CompareAndSwap(false, true) {
		return
	}
	defer subscriptionExpiryRemindRunning.Store(false)

	ctx := context.Background()
	if common.SMTPServer == "" {
		// SMTP 未配置时提醒邮件无从发起到,静默跳过;每小时会再试。
		return
	}
	now := common.GetTimestamp()
	subs, err := model.ListExpiryReminderDueSubscriptions(now, subscriptionExpiryRemindAheadSeconds, subscriptionExpiryRemindBatchSize)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("subscription expiry remind scan failed: %v", err))
		return
	}
	if len(subs) == 0 {
		return
	}

	planTitleCache := make(map[int]string)
	sent := 0
	for _, sub := range subs {
		user, err := model.GetUserById(sub.UserId, false)
		if err != nil || user == nil || user.Email == "" || user.Status != common.UserStatusEnabled {
			continue
		}
		planTitle, ok := planTitleCache[sub.PlanId]
		if !ok {
			planTitle = ""
			if plan, perr := model.GetSubscriptionPlanById(sub.PlanId); perr == nil && plan != nil {
				planTitle = plan.Title
			}
			planTitleCache[sub.PlanId] = planTitle
		}
		if planTitle == "" {
			planTitle = fmt.Sprintf("#%d", sub.PlanId)
		}

		endAt := time.Unix(sub.EndTime, 0).In(subscriptionExpiryRemindLoc)
		daysLeft := int((sub.EndTime - now + 86399) / 86400)
		subject := fmt.Sprintf("【%s】套餐到期提醒:%s 将于 %d 天后到期", common.SystemName, planTitle, daysLeft)
		content := fmt.Sprintf(
			"<p>您好,</p>"+
				"<p>您的套餐 <strong>%s</strong> 将于 <strong>%s</strong>(北京时间)到期,剩余 <strong>%d</strong> 天。</p>"+
				"<p>到期后套餐分组将自动降级,为避免影响使用,请在到期前续费。</p>"+
				"<p>如已续费,请忽略本邮件。</p>",
			html.EscapeString(planTitle), endAt.Format("2006-01-02 15:04"), daysLeft)

		if err := common.SendEmail(subject, user.Email, content); err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("subscription expiry remind send failed: sub=%d user=%d err=%v", sub.Id, user.Id, err))
			continue
		}
		if err := model.MarkExpiryReminderSent(sub.Id, now); err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("subscription expiry remind mark failed: sub=%d err=%v", sub.Id, err))
		}
		sent++
	}
	if sent > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("subscription expiry remind sent: %d", sent))
	}
}
