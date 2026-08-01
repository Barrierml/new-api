package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	grokUsageSyncTickInterval = 5 * time.Minute
	grokUsageSyncHTTPTimeout  = 10 * time.Second
	// Sub2API grok account ID(生产 account 99 = grok1)
	grokUsageSub2APIAccountID = 99
	// Tako channel ID(Grok 官方直连)
	grokUsageTakoChannelID = 33
)

var (
	grokUsageSyncOnce    sync.Once
	grokUsageSyncRunning = make(chan struct{}, 1)
)

// Sub2API grok usage response (admin API /api/v1/admin/accounts/:id/usage)
type grokUsageResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data struct {
		GrokRequestQuota struct {
			Limit     int `json:"limit"`
			Remaining int `json:"remaining"`
		} `json:"grok_request_quota"`
		GrokTokenQuota struct {
			Limit     int64 `json:"limit"`
			Remaining int64 `json:"remaining"`
		} `json:"grok_token_quota"`
		GrokBilling struct {
			PeriodType   string  `json:"period_type"`
			UsagePercent float64 `json:"usage_percent"`
			PeriodEnd    string  `json:"period_end"`
		} `json:"grok_billing"`
	} `json:"data"`
}

// StartGrokUsageSyncTask 定时从 Sub2API 拉 grok 上游额度,同步到 Tako channel
// 的 used_quota,让 admin 在 channel 列表页能看到上游剩余额度。
//
// 数据源:Sub2API admin API /api/v1/admin/accounts/99/usage(account 99 = grok1)。
// 同步字段:used_quota = token_limit - token_remaining(本周已用 token 数)。
// 展示:channel 列表页已渲染 used_quota,无需改前端。
func StartGrokUsageSyncTask() {
	grokUsageSyncOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("grok usage sync task started: tick=%s", grokUsageSyncTickInterval))
			ticker := time.NewTicker(grokUsageSyncTickInterval)
			defer ticker.Stop()

			runGrokUsageSyncOnce()
			for range ticker.C {
				runGrokUsageSyncOnce()
			}
		})
	})
}

func runGrokUsageSyncOnce() {
	select {
	case grokUsageSyncRunning <- struct{}{}:
		defer func() { <-grokUsageSyncRunning }()
	default:
		return
	}

	ctx := context.Background()
	sub2apiURL := common.GetEnvOrDefaultString("SUB2API_URL", "http://sub2api:8080")
	adminEmail := common.GetEnvOrDefaultString("SUB2API_ADMIN_EMAIL", "")
	adminPassword := common.GetEnvOrDefaultString("SUB2API_ADMIN_PASSWORD", "")
	if adminEmail == "" || adminPassword == "" {
		logger.LogWarn(ctx, "grok usage sync skipped: SUB2API_ADMIN_EMAIL/PASSWORD not set")
		return
	}

	// 1. 登录 Sub2API 拿 JWT
	jwt, err := sub2apiLogin(sub2apiURL, adminEmail, adminPassword)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("grok usage sync: login failed: %v", err))
		return
	}

	// 2. 拉 grok account usage
	usage, err := fetchGrokUsage(sub2apiURL, jwt, grokUsageSub2APIAccountID)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("grok usage sync: fetch usage failed: %v", err))
		return
	}

	// 3. 算本周已用 token 数,写入 channel.used_quota
	usedTokens := usage.Data.GrokTokenQuota.Limit - usage.Data.GrokTokenQuota.Remaining
	if usedTokens < 0 {
		usedTokens = 0
	}
	if err := model.DB.Model(&model.Channel{}).
		Where("id = ?", grokUsageTakoChannelID).
		Update("used_quota", usedTokens).Error; err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("grok usage sync: update channel failed: %v", err))
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("grok usage synced: channel=%d used_tokens=%d (limit=%d remaining=%d), request_remaining=%d/%d, period=%s, usage_pct=%.1f%%",
		grokUsageTakoChannelID, usedTokens,
		usage.Data.GrokTokenQuota.Limit, usage.Data.GrokTokenQuota.Remaining,
		usage.Data.GrokRequestQuota.Remaining, usage.Data.GrokRequestQuota.Limit,
		usage.Data.GrokBilling.PeriodType, usage.Data.GrokBilling.UsagePercent))
}

func sub2apiLogin(baseURL, email, password string) (string, error) {
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	req, err := http.NewRequest("POST", baseURL+"/api/v1/auth/login", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: grokUsageSyncHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		Code int `json:"code"`
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Code != 0 || result.Data.AccessToken == "" {
		return "", fmt.Errorf("login failed: code=%d", result.Code)
	}
	return result.Data.AccessToken, nil
}

func fetchGrokUsage(baseURL, jwt string, accountID int) (*grokUsageResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/admin/accounts/%d/usage", baseURL, accountID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	client := &http.Client{Timeout: grokUsageSyncHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var usage grokUsageResponse
	if err := json.NewDecoder(resp.Body).Decode(&usage); err != nil {
		return nil, err
	}
	if usage.Code != 0 {
		return nil, fmt.Errorf("usage API error: code=%d msg=%s", usage.Code, usage.Msg)
	}
	return &usage, nil
}
