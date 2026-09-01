package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConcurrencyErrorResponse(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		slotType    string
		wantStatus  int
		wantType    string
		wantMessage string
	}{
		{
			name:        "true concurrency timeout remains rate limit",
			err:         &ConcurrencyError{SlotType: "account", IsTimeout: true},
			slotType:    "user",
			wantStatus:  http.StatusTooManyRequests,
			wantType:    "rate_limit_error",
			wantMessage: "Concurrency limit exceeded for account, please retry later",
		},
		{
			name:        "client cancellation is not classified as concurrency limit",
			err:         context.Canceled,
			slotType:    "user",
			wantStatus:  statusClientClosedRequest,
			wantType:    "api_error",
			wantMessage: "context canceled",
		},
		{
			name:        "deadline exceeded is service unavailable",
			err:         context.DeadlineExceeded,
			slotType:    "user",
			wantStatus:  http.StatusServiceUnavailable,
			wantType:    "api_error",
			wantMessage: "Service temporarily unavailable, please retry later",
		},
		{
			name:        "redis acquire error is service unavailable",
			err:         errors.New("redis unavailable"),
			slotType:    "user",
			wantStatus:  http.StatusServiceUnavailable,
			wantType:    "api_error",
			wantMessage: "Service temporarily unavailable, please retry later",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, errType, message := concurrencyErrorResponse(tt.err, tt.slotType)
			require.Equal(t, tt.wantStatus, status)
			require.Equal(t, tt.wantType, errType)
			require.Equal(t, tt.wantMessage, message)
		})
	}
}

func TestConcurrencyErrorRetryAfterSeconds(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "concurrency timeout", err: &ConcurrencyError{SlotType: "user", IsTimeout: true}, want: defaultConcurrencyRetryAfterSeconds},
		{name: "wait queue full", err: &WaitQueueFullError{SlotType: "user"}, want: defaultConcurrencyRetryAfterSeconds},
		{name: "canceled", err: context.Canceled, want: 0},
		{name: "backend error", err: errors.New("redis unavailable"), want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, concurrencyErrorRetryAfterSeconds(tt.err))
		})
	}
}

func TestOpenAIGatewayHandleConcurrencyErrorWritesRetryAfterBeforeSSECommit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, EndpointResponses, nil)

	h := &OpenAIGatewayHandler{}
	h.handleConcurrencyError(c, &ConcurrencyError{SlotType: "user", IsTimeout: true}, "user", false)

	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	require.Equal(t, "5", recorder.Header().Get("Retry-After"))
	require.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
	require.Contains(t, recorder.Body.String(), "rate_limit_error")
}

func TestOpenAIConcurrencyWaitShouldEmitHeartbeat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		path string
		want bool
	}{
		{path: EndpointResponses, want: false},
		{path: "/responses", want: false},
		{path: "/backend-api/codex/responses", want: false},
		{path: EndpointChatCompletions, want: false},
		{path: EndpointMessages, want: true},
	} {
		t.Run(tt.path, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, tt.path, nil)
			require.Equal(t, tt.want, openAIConcurrencyWaitShouldEmitHeartbeat(c))
		})
	}
}
