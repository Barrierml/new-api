package router

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetRouter(router *gin.Engine, assets WebAssets) {
	SetApiRouter(router)
	SetDashboardRouter(router)
	SetRelayRouter(router)
	SetVideoRouter(router)
	// PAR/tako-cli compatibility endpoints (root path, not under /api)
	// Existing CLI calls https://tako.shiroha.tech/apiStats/api/*
	apiStats := router.Group("/apiStats/api")
	apiStats.Use(middleware.RouteTag("api"))
	{
		apiStats.POST("/get-key-id", controller.GetKeyID)
		apiStats.POST("/user-stats", controller.UserStats)
		apiStats.GET("/user-stats", controller.UserStats)
		apiStats.POST("/user-quota", controller.UserQuota)
		apiStats.GET("/user-quota", controller.UserQuota)
	}

	// Legacy PAR OAuth callback compatibility: GitHub/Google OAuth Apps still
	// have /par/user/auth/oauth/{provider}/callback registered as redirect URI.
	// 302 to the SPA callback route, preserving code/state.
	router.GET("/par/user/auth/oauth/:provider/callback", func(c *gin.Context) {
		target := "/oauth/" + c.Param("provider")
		if c.Request.URL.RawQuery != "" {
			target += "?" + c.Request.URL.RawQuery
		}
		c.Redirect(http.StatusFound, target)
	})
	frontendBaseUrl := os.Getenv("FRONTEND_BASE_URL")
	if common.IsMasterNode && frontendBaseUrl != "" {
		frontendBaseUrl = ""
		common.SysLog("FRONTEND_BASE_URL is ignored on master node")
	}
	if frontendBaseUrl == "" {
		SetWebRouter(router, assets)
	} else {
		frontendBaseUrl = strings.TrimSuffix(frontendBaseUrl, "/")
		router.NoRoute(func(c *gin.Context) {
			c.Set(middleware.RouteTagKey, "web")
			c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s%s", frontendBaseUrl, c.Request.RequestURI))
		})
	}
}
