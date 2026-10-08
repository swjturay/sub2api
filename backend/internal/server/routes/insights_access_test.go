package routes

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type insightsRouteDefs struct {
	service.UserAttributeDefinitionRepository
}

func (insightsRouteDefs) GetByKey(context.Context, string) (*service.UserAttributeDefinition, error) {
	return &service.UserAttributeDefinition{ID: 41, Key: service.InsightsAccessKey, Description: service.InsightsAccessDescription, Type: service.AttributeTypeSelect, Enabled: true, Options: []service.UserAttributeOption{{Value: "enabled"}, {Value: "disabled"}}}, nil
}

type insightsRouteValues struct {
	service.UserAttributeValueRepository
	enabled bool
}

func (v *insightsRouteValues) GetByUserID(context.Context, int64) ([]service.UserAttributeValue, error) {
	value := "disabled"
	if v.enabled {
		value = "enabled"
	}
	return []service.UserAttributeValue{{AttributeID: 41, Value: value}}, nil
}
func TestInsightsRoutesSeparateViewerFromAdminWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	values := &insightsRouteValues{enabled: true}
	attrs := service.NewUserAttributeService(insightsRouteDefs{}, values)
	cfg := &config.Config{Timezone: "UTC"}
	h := &handler.Handlers{Auth: handler.NewAuthHandler(cfg, nil, nil, nil, nil, nil, nil, attrs), Insights: handler.NewInsightsHandler(nil, cfg, nil, nil)}
	role := "user"
	jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		c.Set(string(middleware.ContextKeyUserRole), role)
		c.Next()
	})
	admin := middleware.AdminAuthMiddleware(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		c.Set(string(middleware.ContextKeyUserRole), role)
		if role != "admin" {
			c.AbortWithStatus(403)
			return
		}
		c.Next()
	})
	r := gin.New()
	RegisterInsightsRoutes(r.Group("/api/v1"), h, jwt, admin, nil, nil)
	request := func(method, path string) int {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/api/v1"+path, nil))
		return w.Code
	}
	// Invalid filters prove the authorized viewer reaches the actual read handlers,
	// without requiring a database fixture for unrelated analytics formulas.
	for _, path := range []string{"/insights/departments?from=invalid", "/insights/gateway/quality?from=invalid", "/insights/costs?month=invalid"} {
		require.Equal(t, 400, request("GET", path), path)
	}
	require.Equal(t, 403, request("PUT", "/admin/insights/costs/accounts/7/months/2026-10"))
	require.Equal(t, 403, request("POST", "/admin/insights/costs/accounts/7/stop"))
	require.Equal(t, 404, request("PUT", "/insights/costs/accounts/7/months/2026-10"))
	require.Equal(t, 403, request("GET", "/admin/insights/dimensions"))
	values.enabled = false
	for _, path := range []string{"/dimensions", "/departments", "/gateway/quality", "/gateway/model-preferences", "/gateway/users", "/gateway/retention", "/costs"} {
		require.Equal(t, 403, request("GET", "/insights"+path), path)
	}
	role = "admin"
	require.Equal(t, 400, request("GET", "/insights/costs?month=invalid"))
	require.Equal(t, 400, request("PUT", "/admin/insights/costs/accounts/7/months/invalid"))
}
