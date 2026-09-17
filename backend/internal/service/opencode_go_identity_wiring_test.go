package service

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// openCodeIdentityTestAccount 构造 opencode_go apikey 账号，用于验证真实构造器
// 是否收敛下游客户端指纹。带 base_url 使 native anthropic / CC 路径可解析目标。
func openCodeIdentityTestAccount(baseURL string) *Account {
	return &Account{
		ID:       30,
		Platform: PlatformOpenCodeGo,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":     baseURL,
			"api_protocol": APIProtocolAdaptive,
		},
	}
}

// 下游 Python-urllib 是本事故的直接触发者：它必须不能到达上游。
func openCodeIdentityTestContext(t *testing.T) *gin.Context {
	t.Helper()
	c := newOpenCodeSessionTestContext(t, "")
	c.Request.Header.Set("User-Agent", "Python-urllib/3.14")
	c.Request.Header.Set("X-Stainless-Lang", "python")
	c.Request.Header.Set("X-Stainless-OS", "Linux")
	c.Request.Header.Set("Accept-Language", "zh-CN")
	return c
}

// CC 直转路径（OpenCode 默认协议：glm/kimi/deepseek/hy/omen）。
func TestRawChatCompletionsConvergesOpenCodeClientUA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &openCodeSessionHTTPUpstream{}
	svc := openCodeSessionTestService()
	svc.httpUpstream = upstream
	account := openCodeIdentityTestAccount("https://opencode.ai/zen/go/v1")
	c := openCodeIdentityTestContext(t)

	resp, err := svc.sendCCUpstreamRequest(
		context.Background(), c, account,
		"https://opencode.ai/zen/go/v1/chat/completions",
		[]byte(`{"model":"deepseek-v4-flash","messages":[]}`),
		false, "upstream-token", "", "",
	)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.NotNil(t, upstream.request)

	h := upstream.request.Header
	require.Equal(t, openCodeGoUpstreamUserAgent, getHeaderRaw(h, "user-agent"),
		"CC 路径必须把下游客户端 UA 收敛为网关身份")
	require.Empty(t, getHeaderRaw(h, "x-stainless-lang"))
	require.Empty(t, getHeaderRaw(h, "x-stainless-os"))
	require.Empty(t, getHeaderRaw(h, "accept-language"))
	require.Equal(t, "Bearer upstream-token", getHeaderRaw(h, "authorization"),
		"身份收敛不得影响上游认证")
}

// Responses 协议路径（grok-* / gpt-* / muse-spark-*）经 buildUpstreamRequest。
func TestBuildUpstreamRequestConvergesOpenCodeClientUA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := openCodeSessionTestService()
	account := openCodeIdentityTestAccount("https://opencode.ai/zen/go/v1")
	c := openCodeIdentityTestContext(t)

	req, err := svc.buildUpstreamRequest(
		context.Background(), c, account,
		[]byte(`{"model":"gpt-5.6-luna","input":"hello"}`),
		"upstream-token", false, "", false,
	)
	require.NoError(t, err)
	require.Equal(t, openCodeGoUpstreamUserAgent, getHeaderRaw(req.Header, "user-agent"),
		"Responses 路径必须把下游客户端 UA 收敛为网关身份")
	require.Empty(t, getHeaderRaw(req.Header, "x-stainless-lang"))
}

// Anthropic 原生路径（minimax-* / qwen*）经 buildNativeAnthropicUpstreamRequest。
func TestNativeAnthropicConvergesOpenCodeClientUA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := openCodeSessionTestService()
	account := openCodeIdentityTestAccount("https://opencode.ai/zen/go")
	c := openCodeIdentityTestContext(t)

	req, _, err := svc.buildNativeAnthropicUpstreamRequest(
		context.Background(), c, account,
		[]byte(`{"model":"qwen3.8-max","max_tokens":16,"messages":[]}`),
		"upstream-token", "https://opencode.ai/zen/go/v1/messages",
	)
	require.NoError(t, err)
	require.Equal(t, openCodeGoUpstreamUserAgent, getHeaderRaw(req.Header, "user-agent"),
		"Anthropic 原生路径必须把下游客户端 UA 收敛为网关身份")
	require.Empty(t, getHeaderRaw(req.Header, "x-stainless-lang"))
	require.Empty(t, getHeaderRaw(req.Header, "accept-language"))
}

// 对照组：非 OpenCode 账号的 UA 透传行为必须保持不变，否则会改动其它平台
// 已有的身份语义（Grok 走自己的 CLI 头、OpenAI 走 Codex 身份收口）。
func TestNonOpenCodeAccountKeepsClientUAAfterWiring(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &openCodeSessionHTTPUpstream{}
	svc := openCodeSessionTestService()
	svc.httpUpstream = upstream
	account := &Account{
		ID:       99,
		Platform: PlatformDeepseek,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-upstream",
			"base_url": "https://api.deepseek.com/v1",
		},
	}
	c := openCodeIdentityTestContext(t)

	resp, err := svc.sendCCUpstreamRequest(
		context.Background(), c, account,
		"https://api.deepseek.com/v1/chat/completions",
		[]byte(`{"model":"deepseek-chat","messages":[]}`),
		false, "upstream-token", "", "",
	)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.NotNil(t, upstream.request)

	require.Equal(t, "Python-urllib/3.14", getHeaderRaw(upstream.request.Header, "user-agent"),
		"非 OpenCode 平台不得被身份收敛")
	// accept-language 在 CC 白名单内，非 OpenCode 账号必须照旧透传：
	// 证明收敛是按平台门控的，而不是把所有账号的头都删了。
	require.Equal(t, "zh-CN", getHeaderRaw(upstream.request.Header, "accept-language"))
}
