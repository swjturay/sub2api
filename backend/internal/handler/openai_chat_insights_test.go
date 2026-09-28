//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/insights"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionsInsightsTerminal(t *testing.T) {
	for _, tc := range []struct {
		name, mode, extra string
		success           bool
		stream            bool
	}{
		{"responses_bridge", "revoked", "", true, false},
		{"streaming_bridge", "revoked", "", true, true},
		{"raw_chat", "revoked", `,"stop":["END"]`, true, false},
		{"upstream_failure", "all_429", "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, _, _, cleanup := newGrokCredentialFailoverHandler(t, tc.mode)
			defer cleanup()
			groupID := int64(901)
			key := &service.APIKey{ID: 902, GroupID: &groupID, User: &service.User{ID: 903, Status: service.StatusActive}, Group: &service.Group{ID: groupID, Platform: service.PlatformGrok, Status: service.StatusActive}}
			router := gin.New()
			var call *insights.Call
			router.Use(InsightsCallMiddleware(), func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), key)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 903, Concurrency: 1})
				call, _ = insights.CallFromContext(c.Request.Context())
				c.Next()
			})
			router.POST("/v1/chat/completions", h.ChatCompletions)
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"grok","messages":[{"role":"user","content":"hello"}],"stream":`+strconv.FormatBool(tc.stream)+tc.extra+`}`))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			require.NotNil(t, call)
			fact, done := call.Finalized()
			require.True(t, done)
			if tc.success {
				require.Equal(t, http.StatusOK, response.Code)
				require.Equal(t, insights.OutcomeSuccess, fact.Outcome)
				require.NotNil(t, fact.OutputTokens)
			} else {
				require.NotEqual(t, insights.OutcomeSuccess, fact.Outcome)
			}
			require.Equal(t, "grok", fact.Model)
			require.Equal(t, service.PlatformGrok, fact.Platform)
			if tc.stream {
				require.Equal(t, insights.TransportHTTPStream, fact.Transport)
			} else {
				require.Equal(t, insights.TransportHTTPSync, fact.Transport)
			}
		})
	}
}
