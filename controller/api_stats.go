package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// RS-compat endpoints used by tako-cli:
//   POST /apiStats/api/get-key-id
//   POST|GET /apiStats/api/user-stats
//   POST|GET /apiStats/api/user-quota
//
// These used to be served by PAR (.22). After cutover to new-api/Tako they must
// remain available so existing CLI installs keep working against tako.shiroha.tech.

func apiStatsJSON(c *gin.Context, payload gin.H) {
	c.JSON(http.StatusOK, payload)
}

func normalizeAPIKey(raw string) string {
	key := strings.TrimSpace(raw)
	key = strings.TrimPrefix(key, "Bearer ")
	key = strings.TrimPrefix(key, "bearer ")
	key = strings.TrimPrefix(key, "sk-")
	return strings.TrimSpace(key)
}

func shanghaiDayStartUnix(now time.Time) int64 {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	local := now.In(loc)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	return start.Unix()
}

func resolveTokenByAPIKey(apiKey string) (*model.Token, error) {
	key := normalizeAPIKey(apiKey)
	if key == "" {
		return nil, fmt.Errorf("empty key")
	}
	return model.GetTokenByKey(key, false)
}

// GetKeyID maps a cr_ API key to its owner identity id for tako-cli.
// PAR returned par_users.uuid; Tako returns the numeric user id as string.
func GetKeyID(c *gin.Context) {
	var body struct {
		APIKey string `json:"apiKey"`
	}
	_ = c.ShouldBindJSON(&body)
	token, err := resolveTokenByAPIKey(body.APIKey)
	if err != nil || token == nil {
		apiStatsJSON(c, gin.H{"success": false, "error": "Invalid key"})
		return
	}
	if token.Status != common.TokenStatusEnabled {
		apiStatsJSON(c, gin.H{"success": false, "error": "Invalid key"})
		return
	}
	user, err := model.GetUserById(token.UserId, false)
	if err != nil || user == nil || user.Status != common.UserStatusEnabled {
		apiStatsJSON(c, gin.H{"success": false, "error": "Invalid key"})
		return
	}
	apiStatsJSON(c, gin.H{
		"success": true,
		"data": gin.H{
			"id": strconv.Itoa(user.Id),
		},
	})
}

func parseAPIStatsUserID(c *gin.Context) (int, bool) {
	var apiID string
	if c.Request.Method == http.MethodPost {
		var body struct {
			ApiId string `json:"apiId"`
		}
		_ = c.ShouldBindJSON(&body)
		apiID = body.ApiId
	} else {
		apiID = c.Query("apiId")
	}
	apiID = strings.TrimSpace(apiID)
	if apiID == "" {
		return 0, false
	}
	id, err := strconv.Atoi(apiID)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// UserStats is the RS-compat daily usage endpoint used by tako-cli fallback.
func UserStats(c *gin.Context) {
	userID, ok := parseAPIStatsUserID(c)
	if !ok {
		apiStatsJSON(c, gin.H{"success": false, "error": "apiId required"})
		return
	}
	user, err := model.GetUserById(userID, false)
	if err != nil || user == nil {
		apiStatsJSON(c, gin.H{"success": false, "error": "User not found"})
		return
	}

	dayStart := shanghaiDayStartUnix(time.Now())
	now := common.GetTimestamp()
	stat, err := model.SumUsedQuota(model.LogTypeConsume, dayStart, now, "", user.Username, "", 0, "")
	if err != nil {
		apiStatsJSON(c, gin.H{"success": false, "error": "usage query failed"})
		return
	}
	dailyCost := float64(stat.Quota) / common.QuotaPerUnit
	if dailyCost < 0 {
		dailyCost = 0
	}

	apiStatsJSON(c, gin.H{
		"success": true,
		"data": gin.H{
			"id":   strconv.Itoa(user.Id),
			"name": firstNonEmpty(user.DisplayName, user.Username),
			"usage": gin.H{
				"total": gin.H{
					"tokens":            0,
					"inputTokens":       0,
					"outputTokens":      0,
					"cacheCreateTokens": 0,
					"cacheReadTokens":   0,
					"allTokens":         0,
					"requests":          0,
					"cost":              dailyCost,
					"formattedCost":     fmt.Sprintf("$%.2f", dailyCost),
				},
			},
			"limits": gin.H{
				"currentDailyCost":      dailyCost,
				"currentTotalCost":      dailyCost,
				"currentWindowRequests": 0,
			},
		},
	})
}

// UserQuota is the preferred tako-cli endpoint with plan limits + usage.
func UserQuota(c *gin.Context) {
	userID, ok := parseAPIStatsUserID(c)
	if !ok {
		apiStatsJSON(c, gin.H{"error": "apiId required"})
		return
	}
	user, err := model.GetUserById(userID, false)
	if err != nil || user == nil {
		apiStatsJSON(c, gin.H{})
		return
	}

	active, err := model.GetAllActiveUserSubscriptions(userID)
	if err != nil {
		active = nil
	}

	var (
		windowLimitUSD float64
		windowUsedUSD  float64
		windowMinutes  float64
		dailyLimitUSD  float64
		dailyUsedUSD   float64
		weeklyLimitUSD float64
		weeklyUsedUSD  float64
		hasPlan        bool
	)

	for _, summary := range active {
		if summary.Subscription == nil {
			continue
		}
		hasPlan = true
		if summary.MainQuotaUsage != nil && summary.MainQuotaUsage.LimitUSD > 0 {
			// Map main plan quota to weekly slot for CLI (closest semantic match).
			if weeklyLimitUSD <= 0 {
				weeklyLimitUSD = summary.MainQuotaUsage.LimitUSD
				weeklyUsedUSD = summary.MainQuotaUsage.UsedUSD
			}
		}
		for _, u := range summary.SubQuotaUsage {
			switch strings.ToLower(u.PeriodUnit) {
			case "hour":
				if windowLimitUSD <= 0 && u.LimitUSD > 0 {
					windowLimitUSD = u.LimitUSD
					windowUsedUSD = u.UsedUSD
					windowMinutes = u.PeriodValue * 60
				}
			case "day":
				if dailyLimitUSD <= 0 && u.LimitUSD > 0 {
					dailyLimitUSD = u.LimitUSD
					dailyUsedUSD = u.UsedUSD
				}
			case "week":
				if weeklyLimitUSD <= 0 && u.LimitUSD > 0 {
					weeklyLimitUSD = u.LimitUSD
					weeklyUsedUSD = u.UsedUSD
				}
			}
		}
		if windowLimitUSD > 0 || dailyLimitUSD > 0 || weeklyLimitUSD > 0 {
			break
		}
	}

	// Always include today's spend as dailyUsed fallback.
	dayStart := shanghaiDayStartUnix(time.Now())
	now := common.GetTimestamp()
	if stat, err := model.SumUsedQuota(model.LogTypeConsume, dayStart, now, "", user.Username, "", 0, ""); err == nil {
		spent := float64(stat.Quota) / common.QuotaPerUnit
		if dailyUsedUSD <= 0 {
			dailyUsedUSD = spent
		}
	}

	if !hasPlan && dailyUsedUSD <= 0 && windowLimitUSD <= 0 && weeklyLimitUSD <= 0 {
		// Match PAR behavior: empty object when no useful plan/usage.
		apiStatsJSON(c, gin.H{})
		return
	}
	if windowMinutes <= 0 && windowLimitUSD > 0 {
		windowMinutes = 300
	}

	apiStatsJSON(c, gin.H{
		"plan": gin.H{
			"window_cost_limit": windowLimitUSD,
			"window_minutes":    windowMinutes,
			"daily_cost_limit":  dailyLimitUSD,
			"weekly_cost_limit": weeklyLimitUSD,
		},
		"usage": gin.H{
			"windowCost": windowUsedUSD,
			"dailyCost":  dailyUsedUSD,
			"weeklyCost": weeklyUsedUSD,
		},
	})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
