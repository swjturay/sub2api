package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Exercise HTTP streaming, JSON and SSE-to-JSON relay at the production handlers.
func TestOpenAIResponseHandlers_RelaySafetyBufferingHeadersToClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Gateway: config.GatewayConfig{OpenAIFirstOutputTimeoutSeconds: 1, MaxLineSize: defaultMaxLineSize}}
	account := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	upstreamHeaders := func(contentType string) http.Header {
		h := http.Header{"Content-Type": []string{contentType}}
		h.Set("X-Codex-Safety-Buffering-Enabled", "true")
		h.Set("X-Codex-Safety-Buffering-Faster-Model", "gpt-5.6-luna")
		return h
	}
	sseBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_sb"}}`, "",
		`data: {"type":"response.output_text.delta","delta":"hello"}`, "",
		`data: {"type":"response.completed","response":{"id":"resp_sb","usage":{"input_tokens":1,"output_tokens":1}}}`, "", "",
	}, "\n")
	const jsonBody = `{"id":"resp_sb","object":"response","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`

	newService := func() *OpenAIGatewayService {
		return &OpenAIGatewayService{cfg: cfg, responseHeaderFilter: compileResponseHeaderFilter(cfg)}
	}
	newContext := func() (*httptest.ResponseRecorder, *gin.Context) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-6-astra"}`))
		return rec, c
	}
	requireRelayed := func(t *testing.T, rec *httptest.ResponseRecorder) {
		t.Helper()
		require.Equal(t, "true", rec.Result().Header.Get("X-Codex-Safety-Buffering-Enabled"))
		require.Equal(t, "gpt-5.6-luna", rec.Result().Header.Get("X-Codex-Safety-Buffering-Faster-Model"))
	}

	t.Run("streaming_committed_with_first_output", func(t *testing.T) {
		rec, c := newContext()
		resp := &http.Response{StatusCode: http.StatusOK, Header: upstreamHeaders("text/event-stream"), Body: io.NopCloser(strings.NewReader(sseBody))}
		_, err := newService().handleStreamingResponse(c.Request.Context(), resp, c, account, time.Now(), "gpt-6-astra", "gpt-6-astra")
		require.NoError(t, err)
		requireRelayed(t, rec)
	})

	t.Run("non_streaming_json", func(t *testing.T) {
		rec, c := newContext()
		resp := &http.Response{StatusCode: http.StatusOK, Header: upstreamHeaders("application/json"), Body: io.NopCloser(strings.NewReader(jsonBody))}
		_, err := newService().handleNonStreamingResponse(c.Request.Context(), resp, c, account, "gpt-6-astra", "gpt-6-astra")
		require.NoError(t, err)
		requireRelayed(t, rec)
	})

	t.Run("non_streaming_upstream_sse_to_json", func(t *testing.T) {
		rec, c := newContext()
		resp := &http.Response{StatusCode: http.StatusOK, Header: upstreamHeaders("text/event-stream"), Body: io.NopCloser(strings.NewReader(sseBody))}
		_, err := newService().handleNonStreamingResponse(c.Request.Context(), resp, c, account, "gpt-6-astra", "gpt-6-astra")
		require.NoError(t, err)
		requireRelayed(t, rec)
	})
}
