package controller

import (
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetUpstreamQuotaDashboard(c *gin.Context) {
	reportPath := common.GetEnvOrDefaultString("UPSTREAM_QUOTA_REPORT_PATH", "")
	ownershipPath := common.GetEnvOrDefaultString("UPSTREAM_QUOTA_OWNERSHIP_PATH", "")
	maxAge := time.Duration(common.GetEnvOrDefault("UPSTREAM_QUOTA_MAX_AGE_SECONDS", 240)) * time.Second

	dashboard, err := service.LoadUpstreamQuotaDashboard(reportPath, ownershipPath, time.Now(), maxAge)
	if err != nil {
		logger.LogError(c, "failed to load upstream quota dashboard: "+err.Error())
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "Upstream quota snapshot is unavailable",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    dashboard,
	})
}
