package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// OpenCode 上游按"客户端签名"分别判定：生产复现中 Python-urllib/3.11–3.14 被
// Cloudflare 稳定返回 1010 (browser_signature_banned)，同一出口换 UA 即 200。
// 因此出站必须收敛到网关身份，不能让下游客户端种类泄漏给上游。
func TestApplyOpenCodeGoUpstreamIdentityReplacesClientFingerprint(t *testing.T) {
	t.Parallel()

	h := http.Header{}
	h.Set("User-Agent", "Python-urllib/3.14")
	h.Set("Originator", "codex_cli_rs")
	h.Set("X-Stainless-Lang", "js")
	h.Set("X-Stainless-OS", "Darwin")
	h.Set("X-Stainless-Package-Version", "0.94.0")
	h.Set("X-App", "cli")
	h.Set("Accept-Language", "en-US")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("X-Claude-Code-Session-Id", "abc-123")
	h.Set("Authorization", "Bearer upstream-secret")
	h.Set("Content-Type", "application/json")
	h.Set("version", "0.153.4")

	applyOpenCodeGoUpstreamIdentity(h)

	require.Equal(t, openCodeGoUpstreamUserAgent, h.Get("User-Agent"),
		"下游客户端 UA 必须被替换为网关规范身份")
	for _, name := range openCodeGoClientFingerprintHeaders {
		require.Empty(t, h.Get(name), "客户端指纹头 %s 不得出站", name)
	}

	// 协议与认证头必须原样保留：身份收敛不能改动上游协议语义或凭据。
	require.Equal(t, "Bearer upstream-secret", h.Get("Authorization"))
	require.Equal(t, "application/json", h.Get("Content-Type"))
	require.Equal(t, "codex_cli_rs", h.Get("Originator"))
	require.Equal(t, "0.153.4", h.Get("version"))
}

// 覆写前先收敛：管理员显式配置的 header_overrides 仍然拥有最终优先级。
// 顺序反转会让运维无法针对性改回某个 UA。
func TestApplyOpenCodeGoUpstreamIdentityPrecedesHeaderOverrides(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformOpenCodeGo,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"header_override_enabled": true,
			"header_overrides":        map[string]any{"user-agent": "operator-pinned/9.9"},
		},
	}

	h := http.Header{}
	h.Set("User-Agent", "Python-urllib/3.14")

	applyOpenCodeGoUpstreamIdentityForAccount(account, h)
	account.ApplyHeaderOverrides(h)

	require.Equal(t, "operator-pinned/9.9", h.Get("User-Agent"),
		"账号级覆写必须能覆盖平台默认身份")
}

// 平台门控：非 OpenCode 账号不得被收敛，否则会改动其它平台的上游身份语义。
func TestApplyOpenCodeGoUpstreamIdentityIsPlatformGated(t *testing.T) {
	t.Parallel()

	for _, account := range []*Account{
		nil,
		{Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		{Platform: PlatformKimi, Type: AccountTypeAPIKey},
		{Platform: PlatformAnthropic, Type: AccountTypeOAuth},
	} {
		h := http.Header{}
		h.Set("User-Agent", "Python-urllib/3.14")

		applyOpenCodeGoUpstreamIdentityForAccount(account, h)

		require.Equal(t, "Python-urllib/3.14", h.Get("User-Agent"),
			"非 OpenCode 账号的 UA 不得被改动")
	}
}

// 幂等：同一请求重复收敛不得产生重复头或大小写重复键。
func TestApplyOpenCodeGoUpstreamIdentityIsIdempotent(t *testing.T) {
	t.Parallel()

	h := http.Header{}
	h.Set("user-agent", "Python-urllib/3.14")

	applyOpenCodeGoUpstreamIdentity(h)
	applyOpenCodeGoUpstreamIdentity(h)

	require.Equal(t, openCodeGoUpstreamUserAgent, h.Get("User-Agent"))
	require.Len(t, h.Values("User-Agent"), 1, "不得产生重复 User-Agent")
}

// nil header 不得 panic（部分路径会传 nil 头表）。
func TestApplyOpenCodeGoUpstreamIdentityHandlesNil(t *testing.T) {
	t.Parallel()

	applyOpenCodeGoUpstreamIdentity(nil)
	applyOpenCodeGoUpstreamIdentityForAccount(&Account{Platform: PlatformOpenCodeGo}, nil)
}

// 会话头在身份收敛之后写入，两者互不干扰：收敛不得删除 OpenCode 会话粘性头。
func TestOpenCodeSessionHeaderSurvivesIdentityNormalization(t *testing.T) {
	t.Parallel()

	h := http.Header{}
	h.Set("User-Agent", "Python-urllib/3.14")

	applyOpenCodeGoUpstreamIdentity(h)
	h.Set(openCodeSessionHeader, "session-abc")

	require.Equal(t, openCodeGoUpstreamUserAgent, h.Get("User-Agent"))
	require.Equal(t, "session-abc", h.Get(openCodeSessionHeader))
}
