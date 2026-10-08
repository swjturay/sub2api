package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RequireInsightsAccess follows JWT authentication, which validates current account status.
func (h *AuthHandler) RequireInsightsAccess(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		middleware.AbortWithError(c, 401, "UNAUTHORIZED", "Authentication required")
		return
	}
	role, _ := middleware.GetUserRoleFromContext(c)
	allowed, err := h.userAttributeService.CanViewInsights(c.Request.Context(), subject.UserID, role == service.RoleAdmin)
	if err != nil {
		middleware.AbortWithError(c, 503, "INSIGHTS_ACCESS_UNAVAILABLE", "Unable to verify Insights access")
		return
	}
	if !allowed {
		middleware.AbortWithError(c, 403, "INSIGHTS_ACCESS_DENIED", "Insights read access is required")
		return
	}
	c.Next()
}
