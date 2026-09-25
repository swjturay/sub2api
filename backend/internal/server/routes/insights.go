package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterInsightsRoutes(v1 *gin.RouterGroup, h *handler.Handlers, jwtAuth middleware.JWTAuthMiddleware, adminAuth middleware.AdminAuthMiddleware, settingService *service.SettingService, limiter *middleware.PanelRateLimiter) {
	user := v1.Group("/insights")
	user.Use(gin.HandlerFunc(jwtAuth), middleware.BackendModeUserGuard(settingService), limiter.Heavy())
	user.GET("/me/usage", h.Insights.PersonalUsage)
	user.GET("/me/today", h.Insights.Today)
	user.GET("/me/heatmap", h.Insights.Heatmap)
	user.GET("/me/usage-logs", h.Insights.UsageLogs)
	user.GET("/me/errors", h.Insights.ErrorLogs)

	admin := v1.Group("/admin/insights")
	admin.Use(gin.HandlerFunc(adminAuth), limiter.Heavy())
	if h.InsightsModels != nil {
		h.InsightsModels.RegisterRoutes(user)
	}

	admin.GET("/dimensions", h.Insights.Dimensions)
	admin.GET("/departments", h.Insights.Departments)
	admin.GET("/gateway/quality", h.Insights.GatewayQuality)
	admin.GET("/gateway/model-preferences", h.Insights.GatewayModelPreferences)
	admin.GET("/gateway/users", h.Insights.GatewayUsers)
	admin.GET("/gateway/retention", h.Insights.GatewayRetention)
	admin.GET("/costs", h.Insights.CostData)
	admin.PUT("/costs/accounts/:account_id/months/:month", h.Insights.SaveCostMonth)
	admin.POST("/costs/accounts/:account_id/stop", h.Insights.StopCostAccount)
}
