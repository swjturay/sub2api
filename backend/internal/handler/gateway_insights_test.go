package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/insights"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"time"
)

func TestInsightsHTTPFailureRequiresExplicitTerminal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name    string
		setup   func(*gin.Context, *insights.Call)
		outcome insights.Outcome
		typ     string
	}{
		{"rejected", func(c *gin.Context, call *insights.Call) {}, insights.OutcomeRejected, "request_rejected"},
		{"cancelled", func(c *gin.Context, call *insights.Call) {
			ctx, cancel := context.WithCancel(c.Request.Context())
			cancel()
			c.Request = c.Request.WithContext(ctx)
		}, insights.OutcomeCancelled, "client_cancelled"},
		{"http200_stream_without_terminal", func(c *gin.Context, call *insights.Call) {
			call.MarkUpstreamSend(time.Now())
			c.Writer.WriteHeader(200)
			_, _ = c.Writer.Write([]byte("partial"))
		}, insights.OutcomeStreamInterrupted, "stream_interrupted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
			call := insights.NewCall(insights.Identity{}, time.Now(), nil)
			tc.setup(c, call)
			o, typ := insightsHTTPFailure(c, call)
			require.Equal(t, tc.outcome, o)
			require.Equal(t, tc.typ, typ)
		})
	}
}

func TestInsightsMiddlewareUsesCanonicalUsageIdentityAndPreservesAlias(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(InsightsCallMiddleware())
	facts := make(chan insights.CallFact, 1)
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		call, ok := insights.CallFromContext(c.Request.Context())
		require.True(t, ok)
		call.MarkUpstreamSend(time.Now())
		insightsUpdateCall(c, "client-model-alias", "openai", false)
		statisticalAt := insightsFinishSuccess(c, "upstream-model", 3, nil)
		fact, done := call.Finalized()
		require.True(t, done)
		require.Equal(t, statisticalAt, fact.StatisticalAt)
		facts <- fact
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx := context.WithValue(req.Context(), ctxkey.ClientRequestID, "raw-client-id")
	ctx = context.WithValue(ctx, ctxkey.RequestID, "local-id")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fact := <-facts
	require.Equal(t, "client:raw-client-id", fact.RequestID)
	require.Equal(t, "raw-client-id", fact.ClientRequestID)
	require.Equal(t, "client-model-alias", fact.Model)
	require.Equal(t, "openai", fact.Platform)
	require.Equal(t, insights.OutcomeSuccess, fact.Outcome)
	require.Equal(t, 1, fact.AttemptCount)
}

func TestInsightsKnownUninstrumentedGenerationInventory(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/gateway/live"},
		{http.MethodPost, "/backend-api/codex/realtime/calls"},
	} {
		require.True(t, insightsKnownUninstrumentedGeneration(tc.method, tc.path), tc.path)
	}
	require.False(t, insightsKnownUninstrumentedGeneration(http.MethodGet, "/api/v1/gateway/realtime"))
	require.False(t, insightsKnownUninstrumentedGeneration(http.MethodPost, "/api/v1/gateway/images/generations/async"))
	require.False(t, insightsKnownUninstrumentedGeneration(http.MethodPost, "/api/v1/gateway/images/batches"))
	require.False(t, insightsKnownUninstrumentedGeneration(http.MethodGet, "/api/v1/gateway/images/batches"))
}

func TestInsightsHTTPTransportIncludesCompactButExcludesTokenCounting(t *testing.T) {
	transport, ok := insightsHTTPTransport(http.MethodPost, "/v1/responses/compact")
	require.True(t, ok)
	require.Equal(t, insights.TransportHTTPSync, transport)

	for _, path := range []string{"/v1/responses/input_tokens", "/v1/messages/count_tokens"} {
		_, ok := insightsHTTPTransport(http.MethodPost, path)
		require.False(t, ok, path)
	}
}

func TestInsightsIngressRecordsBodyLimitAndAuthRejections(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name       string
		middleware gin.HandlerFunc
		handler    gin.HandlerFunc
		wantStatus int
	}{
		{
			name:       "body_limit",
			middleware: servermiddleware.RequestBodyLimit(4),
			handler: func(c *gin.Context) {
				_, err := io.ReadAll(c.Request.Body)
				require.Error(t, err)
				c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			},
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name: "auth_rejection",
			middleware: func(c *gin.Context) {
				c.AbortWithStatus(http.StatusUnauthorized)
			},
			handler:    func(c *gin.Context) { t.Fatal("aborted request reached handler") },
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := make(chan insights.CallFact, 1)
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Next()
				call, ok := insights.CallFromContext(c.Request.Context())
				if !ok {
					return
				}
				fact, done := call.Finalized()
				require.True(t, done)
				facts <- fact
			})
			r.Use(servermiddleware.ClientRequestID())
			r.Use(InsightsCallMiddleware())
			r.Use(tc.middleware)
			r.POST("/v1/responses/compact", tc.handler)

			req := httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewBufferString("oversized"))
			req = req.WithContext(context.WithValue(req.Context(), ctxkey.ClientRequestID, "raw-ingress-id"))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, tc.wantStatus, w.Code)
			fact := <-facts
			require.Equal(t, insights.OutcomeRejected, fact.Outcome)
			require.Equal(t, "request_rejected", fact.ErrorType)
			require.Equal(t, 0, fact.AttemptCount)
			require.Equal(t, "client:raw-ingress-id", fact.RequestID)
			require.Equal(t, "raw-ingress-id", fact.ClientRequestID)
		})
	}
}

func TestInsightsMiddlewareRecordsPanicAndPreservesFinalizedFact(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name        string
		finalize    bool
		wantOutcome insights.Outcome
		wantType    string
	}{
		{name: "unfinalized", wantOutcome: insights.OutcomeError, wantType: "handler_panic"},
		{name: "already_finalized", finalize: true, wantOutcome: insights.OutcomeSuccess},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var observed *insights.Call
			r := gin.New()
			r.Use(gin.Recovery())
			r.Use(servermiddleware.ClientRequestID())
			r.Use(InsightsCallMiddleware())
			r.POST("/v1/messages", func(c *gin.Context) {
				call, ok := insights.CallFromContext(c.Request.Context())
				require.True(t, ok)
				observed = call
				if tc.finalize {
					call.MarkUpstreamSend(time.Now())
					insightsFinishSuccess(c, "model", 1, nil)
				}
				panic("boom")
			})

			req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusInternalServerError, w.Code)
			require.NotNil(t, observed)
			fact, done := observed.Finalized()
			require.True(t, done)
			require.Equal(t, tc.wantOutcome, fact.Outcome)
			require.Equal(t, tc.wantType, fact.ErrorType)
		})
	}
}
