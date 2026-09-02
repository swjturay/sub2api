package service

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIClientTransport_SetAndGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	require.Equal(t, OpenAIClientTransportUnknown, GetOpenAIClientTransport(c))

	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	require.Equal(t, OpenAIClientTransportHTTP, GetOpenAIClientTransport(c))

	SetOpenAIClientTransport(c, OpenAIClientTransportWS)
	require.Equal(t, OpenAIClientTransportWS, GetOpenAIClientTransport(c))
}

func TestOpenAIClientTransport_GetNormalizesRawContextValue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		rawValue any
		want     OpenAIClientTransport
	}{
		{
			name:     "type_value_ws",
			rawValue: OpenAIClientTransportWS,
			want:     OpenAIClientTransportWS,
		},
		{
			name:     "http_sse_alias",
			rawValue: "http_sse",
			want:     OpenAIClientTransportHTTP,
		},
		{
			name:     "sse_alias",
			rawValue: "sSe",
			want:     OpenAIClientTransportHTTP,
		},
		{
			name:     "websocket_alias",
			rawValue: "WebSocket",
			want:     OpenAIClientTransportWS,
		},
		{
			name:     "invalid_string",
			rawValue: "tcp",
			want:     OpenAIClientTransportUnknown,
		},
		{
			name:     "invalid_type",
			rawValue: 123,
			want:     OpenAIClientTransportUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set(openAIClientTransportContextKey, tt.rawValue)
			require.Equal(t, tt.want, GetOpenAIClientTransport(c))
		})
	}
}

func TestOpenAIClientTransport_NilAndUnknownInput(t *testing.T) {
	SetOpenAIClientTransport(nil, OpenAIClientTransportHTTP)
	require.Equal(t, OpenAIClientTransportUnknown, GetOpenAIClientTransport(nil))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	SetOpenAIClientTransport(c, OpenAIClientTransportUnknown)
	_, exists := c.Get(openAIClientTransportContextKey)
	require.False(t, exists)

	SetOpenAIClientTransport(c, OpenAIClientTransport("   "))
	_, exists = c.Get(openAIClientTransportContextKey)
	require.False(t, exists)
}

func TestResolveOpenAIWSDecisionByClientTransport(t *testing.T) {
	base := OpenAIWSProtocolDecision{
		Transport: OpenAIUpstreamTransportResponsesWebsocketV2,
		Reason:    "ws_v2_enabled",
	}

	httpDecision := resolveOpenAIWSDecisionByClientTransport(base, OpenAIClientTransportHTTP)
	require.Equal(t, OpenAIUpstreamTransportHTTPSSE, httpDecision.Transport)
	require.Equal(t, "client_protocol_http", httpDecision.Reason)

	wsDecision := resolveOpenAIWSDecisionByClientTransport(base, OpenAIClientTransportWS)
	require.Equal(t, base, wsDecision)

	unknownDecision := resolveOpenAIWSDecisionByClientTransport(base, OpenAIClientTransportUnknown)
	require.Equal(t, base, unknownDecision)
}

func TestOpenAIHTTPIngressWSBridgeRequiresCtxPoolAndFlag(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.HTTPIngressEnabled = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.IngressModeDefault = OpenAIWSIngressModeCtxPool
	svc := &OpenAIGatewayService{cfg: cfg, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg)}
	account := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Extra: map[string]any{
			"openai_apikey_responses_websockets_v2_mode": OpenAIWSIngressModeCtxPool,
		},
	}
	require.True(t, svc.IsOpenAIHTTPIngressWSBridgeEnabled(nil, account, true, false))
	require.False(t, svc.IsOpenAIHTTPIngressWSBridgeEnabled(nil, account, false, false))
	require.False(t, svc.IsOpenAIHTTPIngressWSBridgeEnabled(nil, account, true, true))
	account.Extra["openai_apikey_responses_websockets_v2_mode"] = OpenAIWSIngressModePassthrough
	require.False(t, svc.IsOpenAIHTTPIngressWSBridgeEnabled(nil, account, true, false))
	account.Extra["openai_apikey_responses_websockets_v2_mode"] = OpenAIWSIngressModeCtxPool
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	setOpenAIHTTPIngressWSFallbackActive(c, true)
	require.False(t, svc.IsOpenAIHTTPIngressWSBridgeEnabled(c, account, true, false))
	setOpenAIHTTPIngressWSFallbackActive(c, false)
	MarkOpenAINativeCompactionV2(c)
	require.False(t, svc.IsOpenAIHTTPIngressWSBridgeEnabled(c, account, true, false),
		"native remote compaction must stay on HTTP/SSE even when the HTTP ingress bridge is enabled")
	cfg.Gateway.OpenAIWS.HTTPIngressEnabled = false
	require.False(t, svc.IsOpenAIHTTPIngressWSBridgeEnabled(nil, account, true, false))
}
