package controller

import (
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

// 运营总览(/operations 管理员页面)聚合端点。
// 一次返回统计卡 + 今日活跃用户表 + 最近错误/消费日志,页面 30s 轮询只打一次。

const (
	operationsMaxActiveUsers = 50 // 活跃用户表封顶(同 PAR)
	operationsRecentLogNum   = 10
	operationsWindowWorkers  = 8
)

type operationsCards struct {
	TodayQuota            int64 `json:"today_quota"`
	TodayPromptTokens     int64 `json:"today_prompt_tokens"`
	TodayCompletionTokens int64 `json:"today_completion_tokens"`
	ActiveUsersToday      int   `json:"active_users_today"`
	TotalUsers            int64 `json:"total_users"`
	ActiveConnections     int64 `json:"active_connections"`
}

type operationsWindow struct {
	LimitUSD  float64 `json:"limit_usd"`
	UsedUSD   float64 `json:"used_usd"`
	Percent   float64 `json:"percent"`
	WindowEnd int64   `json:"window_end"`
}

type operationsUserRow struct {
	UserId                int               `json:"user_id"`
	Username              string            `json:"username"`
	DisplayName           string            `json:"display_name"`
	PlanTitle             string            `json:"plan_title"`
	TodayQuota            int64             `json:"today_quota"`
	TodayPromptTokens     int64             `json:"today_prompt_tokens"`
	TodayCompletionTokens int64             `json:"today_completion_tokens"`
	RequestCount          int64             `json:"request_count"`
	LastUsedAt            int64             `json:"last_used_at"`
	WalletQuota           int               `json:"wallet_quota"`
	Window5h              *operationsWindow `json:"window_5h"`
	Weekly                *operationsWindow `json:"weekly"`
}

// pickOperationsWindows 把订阅的窗口用量映射到运营总览的两个槽位:
// hour 周期 → 5h 窗口;week 周期 → 周消费;无 week 子限制时回退主额度(语义最接近,同 UserQuota)。
func pickOperationsWindows(main *model.MainQuotaUsage, subs []model.SubscriptionSubQuotaUsage) (window5h, weekly *operationsWindow) {
	for _, u := range subs {
		if u.LimitUSD <= 0 {
			continue
		}
		switch u.PeriodUnit {
		case "hour":
			if window5h == nil {
				window5h = &operationsWindow{LimitUSD: u.LimitUSD, UsedUSD: u.UsedUSD, Percent: u.Percent, WindowEnd: u.WindowEnd}
			}
		case "week":
			if weekly == nil {
				weekly = &operationsWindow{LimitUSD: u.LimitUSD, UsedUSD: u.UsedUSD, Percent: u.Percent, WindowEnd: u.WindowEnd}
			}
		}
	}
	if weekly == nil && main != nil && main.LimitUSD > 0 {
		weekly = &operationsWindow{LimitUSD: main.LimitUSD, UsedUSD: main.UsedUSD, Percent: main.Percent, WindowEnd: main.ResetTime}
	}
	return window5h, weekly
}

// GetOperationsOverview 管理员运营总览:今日消费/Token/活跃用户数 + 活跃用户表 + 最近日志。
func GetOperationsOverview(c *gin.Context) {
	now := common.GetTimestamp()
	dayStart := shanghaiDayStartUnix(time.Now())

	stats, err := model.SumConsumeQuotaByUser(dayStart, now)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	cards := operationsCards{
		ActiveUsersToday:  len(stats),
		ActiveConnections: middleware.GetStats().ActiveConnections,
	}
	for _, s := range stats {
		cards.TodayQuota += s.Quota
		cards.TodayPromptTokens += s.PromptTokens
		cards.TodayCompletionTokens += s.CompletionTokens
	}
	if total, countErr := model.CountUsers(); countErr == nil {
		cards.TotalUsers = total
	}

	// 今日消费 top N,批量取用户/订阅信息
	sort.Slice(stats, func(i, j int) bool { return stats[i].Quota > stats[j].Quota })
	if len(stats) > operationsMaxActiveUsers {
		stats = stats[:operationsMaxActiveUsers]
	}
	userIds := make([]int, 0, len(stats))
	for _, s := range stats {
		userIds = append(userIds, s.UserId)
	}

	type userBasics struct {
		Id          int    `gorm:"column:id"`
		Username    string `gorm:"column:username"`
		DisplayName string `gorm:"column:display_name"`
		Quota       int    `gorm:"column:quota"`
	}
	userMap := make(map[int]userBasics, len(userIds))
	if len(userIds) > 0 {
		var users []userBasics
		if err := model.DB.Table("users").Select("id, username, display_name, quota").Where("id IN ?", userIds).Find(&users).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		for _, u := range users {
			userMap[u.Id] = u
		}
	}

	// 每用户取最新一条活跃订阅
	subMap := make(map[int]*model.UserSubscription, len(userIds))
	if len(userIds) > 0 {
		var subs []*model.UserSubscription
		if err := model.DB.Where("user_id IN ? AND status = ? AND end_time > ?", userIds, "active", now).
			Order("end_time desc, id desc").Find(&subs).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		for _, sub := range subs {
			if _, exists := subMap[sub.UserId]; !exists {
				subMap[sub.UserId] = sub
			}
		}
	}

	rows := make([]operationsUserRow, len(stats))
	for i, s := range stats {
		row := operationsUserRow{
			UserId:                s.UserId,
			TodayQuota:            s.Quota,
			TodayPromptTokens:     s.PromptTokens,
			TodayCompletionTokens: s.CompletionTokens,
			RequestCount:          s.RequestCount,
			LastUsedAt:            s.LastUsedAt,
		}
		if u, ok := userMap[s.UserId]; ok {
			row.Username = u.Username
			row.DisplayName = u.DisplayName
			row.WalletQuota = u.Quota
		}
		if sub, ok := subMap[s.UserId]; ok {
			if plan, planErr := model.GetSubscriptionPlanById(sub.PlanId); planErr == nil && plan != nil {
				row.PlanTitle = plan.Title
			}
		}
		rows[i] = row
	}

	// 5h/周窗口用量:每用户每窗口 1 条 indexed SUM,并发限流
	var g errgroup.Group
	g.SetLimit(operationsWindowWorkers)
	for i := range rows {
		sub, ok := subMap[rows[i].UserId]
		if !ok {
			continue
		}
		i, sub := i, sub
		g.Go(func() error {
			subUsage, err := model.BuildSubQuotaUsage(rows[i].UserId, sub, now)
			if err != nil {
				return err
			}
			rows[i].Window5h, rows[i].Weekly = pickOperationsWindows(model.BuildMainQuotaUsage(sub), subUsage)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		common.ApiError(c, err)
		return
	}

	errorLogs, err := model.GetRecentLogs(model.LogTypeError, operationsRecentLogNum)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	consumeLogs, err := model.GetRecentLogs(model.LogTypeConsume, operationsRecentLogNum)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"generated_at": now,
		"day_start":    dayStart,
		"cards":        cards,
		"users":        rows,
		"error_logs":   errorLogs,
		"consume_logs": consumeLogs,
	})
}
