package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// 渠道运营页 grok 用量弹窗数据源:Sub2API admin API。
//
// grok 渠道的 base_url 指向 Sub2API(如 http://sub2api:8080),Sub2API 自己
// 维护 grok 账号的周配额(limit/remaining/usage_percent/重置时间)。Tako 把
// 这些数据映射成 Codex 弹窗的 CodexRateLimitWindow 结构,前端直接复用
// codex-usage-dialog 渲染,展示 100% 可用 / 重置时间等。
//
// 账号定位:Sub2API admin API GET /api/v1/admin/accounts?name=grok
// (take=1),取第一个 id,再 GET /api/v1/admin/accounts/:id/usage。
// 凭据来自 env(SUB2API_ADMIN_EMAIL/PASSWORD),与 grok usage sync 任务同源。

const sub2apiGrokHTTPTimeout = 10 * time.Second

type sub2apiListItem struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

type sub2apiListResponse struct {
	Code int `json:"code"`
	Data struct {
		Items []sub2apiListItem `json:"items"`
	} `json:"data"`
}

// GetGrokChannelUsage 返回 grok 渠道上游(Sub2API)配额,映射成 codex usage 弹窗结构。
func GetGrokChannelUsage(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ch, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
		return
	}

	baseURL := strings.TrimRight(strings.TrimSpace(ch.GetBaseURL()), "/")
	if baseURL == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "grok channel base_url 未配置,无法定位 Sub2API"})
		return
	}

	adminEmail := common.GetEnvOrDefaultString("SUB2API_ADMIN_EMAIL", "")
	adminPassword := common.GetEnvOrDefaultString("SUB2API_ADMIN_PASSWORD", "")
	if adminEmail == "" || adminPassword == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "SUB2API_ADMIN_EMAIL/PASSWORD 未配置"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	jwt, err := sub2apiAdminLogin(ctx, baseURL, adminEmail, adminPassword)
	if err != nil {
		common.SysError("grok usage dialog login failed: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Sub2API 登录失败,请稍后重试"})
		return
	}

	accountID, accountName, err := sub2apiFindGrokAccount(ctx, baseURL, jwt)
	if err != nil {
		common.SysError("grok usage dialog find account failed: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Sub2API 未找到 grok 账号"})
		return
	}

	usage, err := sub2apiFetchAccountUsage(ctx, baseURL, jwt, accountID)
	if err != nil {
		common.SysError("grok usage dialog fetch usage failed: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取用量信息失败,请稍后重试"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message":         "",
		"upstream_status": http.StatusOK,
		"data":            buildGrokUsagePayload(accountName, usage),
	})
}

func sub2apiAdminLogin(ctx context.Context, baseURL, email, password string) (string, error) {
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/auth/login", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: sub2apiGrokHTTPTimeout}
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

func sub2apiFindGrokAccount(ctx context.Context, baseURL, jwt string) (int, string, error) {
	// name 是模糊搜索(会命中 kimi 等),拉一批再按 platform 过滤
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/admin/accounts?name=grok&take=50", nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	client := &http.Client{Timeout: sub2apiGrokHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	var result sub2apiListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, "", err
	}
	if result.Code != 0 {
		return 0, "", fmt.Errorf("list accounts error: code=%d", result.Code)
	}
	for _, item := range result.Data.Items {
		if item.Platform == "grok" {
			return item.Id, item.Name, nil
		}
	}
	return 0, "", fmt.Errorf("no grok platform account in %d name-matched items", len(result.Data.Items))
}

type sub2apiGrokUsage struct {
	Code int `json:"code"`
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
			Plan           string  `json:"plan"`
			PeriodType     string  `json:"period_type"`
			UsagePercent   float64 `json:"usage_percent"`
			PeriodStart    string  `json:"period_start"`
			PeriodEnd      string  `json:"period_end"`
			BillingUsedPct float64 `json:"used_percent"`
		} `json:"grok_billing"`
		GrokLocalUsage7d struct {
			Requests int     `json:"requests"`
			Tokens   int64   `json:"tokens"`
			Cost     float64 `json:"cost"`
		} `json:"grok_local_usage_7d"`
	} `json:"data"`
}

func sub2apiFetchAccountUsage(ctx context.Context, baseURL, jwt string, accountID int) (*sub2apiGrokUsage, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/v1/admin/accounts/%d/usage", baseURL, accountID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	client := &http.Client{Timeout: sub2apiGrokHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var usage sub2apiGrokUsage
	if err := json.NewDecoder(resp.Body).Decode(&usage); err != nil {
		return nil, err
	}
	if usage.Code != 0 {
		return nil, fmt.Errorf("usage API error: code=%d", usage.Code)
	}
	return &usage, nil
}

// buildGrokUsagePayload 把 Sub2API grok 用量映射成 CodexUsagePayload 结构,
// 前端 codex-usage-dialog 直接复用:
//   - rate_limit.primary_window(weekly): token 配额 → used_percent / reset_at
//   - additional_rate_limits[0](weekly): 请求配额
//   - additional_rate_limits[1](monthly): 账单月额度
func buildGrokUsagePayload(accountName string, usage *sub2apiGrokUsage) map[string]any {
	billing := usage.Data.GrokBilling

	periodEndUnix := int64(0)
	resetAfter := int64(0)
	periodWindowSeconds := int64(0)
	if t, err := time.Parse(time.RFC3339, billing.PeriodEnd); err == nil {
		periodEndUnix = t.Unix()
		if d := time.Until(t); d > 0 {
			resetAfter = int64(d.Seconds())
		}
	}
	if s, err := time.Parse(time.RFC3339, billing.PeriodStart); err == nil && periodEndUnix > 0 {
		periodWindowSeconds = periodEndUnix - s.Unix()
	}

	tokenLimit := usage.Data.GrokTokenQuota.Limit
	tokenRemaining := usage.Data.GrokTokenQuota.Remaining
	tokenUsedPct := 0.0
	if tokenLimit > 0 {
		tokenUsedPct = float64(tokenLimit-tokenRemaining) / float64(tokenLimit) * 100
		if tokenUsedPct < 0 {
			tokenUsedPct = 0
		}
	}

	reqLimit := usage.Data.GrokRequestQuota.Limit
	reqRemaining := usage.Data.GrokRequestQuota.Remaining
	reqUsedPct := 0.0
	if reqLimit > 0 {
		reqUsedPct = float64(reqLimit-reqRemaining) / float64(reqLimit) * 100
		if reqUsedPct < 0 {
			reqUsedPct = 0
		}
	}

	limited := (tokenLimit > 0 && tokenRemaining <= 0) || (reqLimit > 0 && reqRemaining <= 0)

	monthlyWindowSeconds := int64(30 * 24 * 3600)

	return map[string]any{
		"plan_type": billing.Plan,
		"email":     accountName,
		"rate_limit": map[string]any{
			"plan_type":     billing.Plan,
			"allowed":       !limited,
			"limit_reached": limited,
			"primary_window": map[string]any{
				"used_percent":        billing.UsagePercent,
				"reset_at":            periodEndUnix,
				"reset_after_seconds": resetAfter,
				"limit_window_seconds": periodWindowSeconds,
			},
		},
		"additional_rate_limits": []map[string]any{
			{
				"limit_name": fmt.Sprintf("Token Quota (%d/%d)", tokenLimit-tokenRemaining, tokenLimit),
				"rate_limit": map[string]any{
					"allowed":       tokenRemaining > 0,
					"limit_reached": tokenLimit > 0 && tokenRemaining <= 0,
					"primary_window": map[string]any{
						"used_percent":         tokenUsedPct,
						"reset_at":             periodEndUnix,
						"reset_after_seconds":  resetAfter,
						"limit_window_seconds": periodWindowSeconds,
					},
				},
			},
			{
				"limit_name": fmt.Sprintf("Request Quota (%d/%d)", reqLimit-reqRemaining, reqLimit),
				"rate_limit": map[string]any{
					"allowed":       reqRemaining > 0,
					"limit_reached": reqLimit > 0 && reqRemaining <= 0,
					"primary_window": map[string]any{
						"used_percent":         reqUsedPct,
						"reset_at":             periodEndUnix,
						"reset_after_seconds":  resetAfter,
						"limit_window_seconds": periodWindowSeconds,
					},
				},
			},
			{
				"limit_name": "Billing Month",
				"rate_limit": map[string]any{
					"allowed":       true,
					"limit_reached": false,
					"primary_window": map[string]any{
						"used_percent":         billing.BillingUsedPct,
						"limit_window_seconds": monthlyWindowSeconds,
					},
				},
			},
		},
		"grok_local_usage_7d": map[string]any{
			"requests": usage.Data.GrokLocalUsage7d.Requests,
			"tokens":   usage.Data.GrokLocalUsage7d.Tokens,
			"cost":     usage.Data.GrokLocalUsage7d.Cost,
		},
	}
}
