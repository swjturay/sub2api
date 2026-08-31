package service

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// OpenAIClientTransport 表示客户端入站协议类型。
type OpenAIClientTransport string

const (
	OpenAIClientTransportUnknown OpenAIClientTransport = ""
	OpenAIClientTransportHTTP    OpenAIClientTransport = "http"
	OpenAIClientTransportWS      OpenAIClientTransport = "ws"
)

const openAIClientTransportContextKey = "openai_client_transport"
const openAIHTTPIngressWSFallbackContextKey = "openai_http_ingress_ws_fallback"

func isOpenAIHTTPIngressWSFallbackActive(c *gin.Context) bool {
	if c == nil {
		return false
	}
	v, ok := c.Get(openAIHTTPIngressWSFallbackContextKey)
	active, typed := v.(bool)
	return ok && typed && active
}

func setOpenAIHTTPIngressWSFallbackActive(c *gin.Context, active bool) {
	if c != nil {
		c.Set(openAIHTTPIngressWSFallbackContextKey, active)
	}
}

// SetOpenAIClientTransport 标记当前请求的客户端入站协议。
func SetOpenAIClientTransport(c *gin.Context, transport OpenAIClientTransport) {
	if c == nil {
		return
	}
	normalized := normalizeOpenAIClientTransport(transport)
	if normalized == OpenAIClientTransportUnknown {
		return
	}
	c.Set(openAIClientTransportContextKey, string(normalized))
}

// GetOpenAIClientTransport 读取当前请求的客户端入站协议。
func GetOpenAIClientTransport(c *gin.Context) OpenAIClientTransport {
	if c == nil {
		return OpenAIClientTransportUnknown
	}
	raw, ok := c.Get(openAIClientTransportContextKey)
	if !ok || raw == nil {
		return OpenAIClientTransportUnknown
	}

	switch v := raw.(type) {
	case OpenAIClientTransport:
		return normalizeOpenAIClientTransport(v)
	case string:
		return normalizeOpenAIClientTransport(OpenAIClientTransport(v))
	default:
		return OpenAIClientTransportUnknown
	}
}

func normalizeOpenAIClientTransport(transport OpenAIClientTransport) OpenAIClientTransport {
	switch strings.ToLower(strings.TrimSpace(string(transport))) {
	case string(OpenAIClientTransportHTTP), "http_sse", "sse":
		return OpenAIClientTransportHTTP
	case string(OpenAIClientTransportWS), "websocket":
		return OpenAIClientTransportWS
	default:
		return OpenAIClientTransportUnknown
	}
}

func resolveOpenAIWSDecisionByClientTransport(
	decision OpenAIWSProtocolDecision,
	clientTransport OpenAIClientTransport,
) OpenAIWSProtocolDecision {
	if clientTransport == OpenAIClientTransportHTTP {
		return openAIWSHTTPDecision("client_protocol_http")
	}
	return decision
}

// IsOpenAIHTTPIngressWSBridgeEnabled reports whether an HTTP Responses request
// may use the upstream Responses WebSocket pool. The bridge intentionally only
// accepts ordinary ctx_pool turns: passthrough remains a native downstream WS
// mode, http_bridge keeps its existing reverse (WS -> HTTP) meaning, and native
// remote compaction stays on HTTP/SSE because its response has no token phase
// that can release the WS pre-output buffer.
func (s *OpenAIGatewayService) IsOpenAIHTTPIngressWSBridgeEnabled(
	c *gin.Context,
	account *Account,
	stream bool,
	compactPath bool,
) bool {
	if s == nil || s.cfg == nil || !s.cfg.Gateway.OpenAIWS.HTTPIngressEnabled ||
		account == nil || !account.IsOpenAI() || !stream || compactPath ||
		isOpenAINativeCompactionV2(c) {
		return false
	}
	if isOpenAIHTTPIngressWSFallbackActive(c) {
		return false
	}
	decision := s.getOpenAIWSProtocolResolver().Resolve(account)
	if decision.Transport != OpenAIUpstreamTransportResponsesWebsocketV2 {
		return false
	}
	if !s.cfg.Gateway.OpenAIWS.ModeRouterV2Enabled {
		// Legacy per-account boolean had ctx_pool semantics before the mode
		// router was introduced.
		return true
	}
	return account.ResolveOpenAIResponsesWebSocketV2Mode(s.cfg.Gateway.OpenAIWS.IngressModeDefault) == OpenAIWSIngressModeCtxPool
}
