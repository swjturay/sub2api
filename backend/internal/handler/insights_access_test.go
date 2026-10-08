package handler

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accessDefs struct {
	service.UserAttributeDefinitionRepository
	err error
}

func (d accessDefs) GetByKey(context.Context, string) (*service.UserAttributeDefinition, error) {
	return &service.UserAttributeDefinition{ID: 1, Key: service.InsightsAccessKey, Description: service.InsightsAccessDescription, Enabled: true, Type: service.AttributeTypeSelect, Options: []service.UserAttributeOption{{Value: "enabled"}, {Value: "disabled"}}}, d.err
}

type accessValues struct {
	service.UserAttributeValueRepository
	value string
}

func (v *accessValues) GetByUserID(context.Context, int64) ([]service.UserAttributeValue, error) {
	return []service.UserAttributeValue{{AttributeID: 1, Value: v.value}}, nil
}
func TestRequireInsightsAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, role, value string
		subject           bool
		err               error
		status            int
	}{
		{name: "anonymous", status: 401},
		{name: "ordinary", subject: true, role: "user", status: 403},
		{name: "viewer", subject: true, role: "user", value: "enabled", status: 204},
		{name: "revoked", subject: true, role: "user", value: "disabled", status: 403},
		{name: "lookup failure", subject: true, role: "user", err: errors.New("db failed"), status: 503},
		{name: "admin", subject: true, role: "admin", status: 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &AuthHandler{userAttributeService: service.NewUserAttributeService(accessDefs{err: tc.err}, &accessValues{value: tc.value})}
			r := gin.New()
			r.GET("/read", func(c *gin.Context) {
				if tc.subject {
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
					c.Set(string(middleware.ContextKeyUserRole), tc.role)
				}
			}, h.RequireInsightsAccess, func(c *gin.Context) { c.Status(204) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/read", nil))
			require.Equal(t, tc.status, w.Code)
		})
	}
}
func TestDingTalkCannotWriteInsightsAuthorization(t *testing.T) {
	// Nil attribute service proves the reserved key is blocked before storage access.
	h := &AuthHandler{}
	require.ErrorIs(t, h.setUserAttributeByKey(context.Background(), 7, " insights_access ", "enabled"), service.ErrAttributeValidationFailed)
}
