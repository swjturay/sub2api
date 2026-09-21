//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Read the UA on the wire: net/http supplies its default only during dispatch.
func outboundUserAgent(t *testing.T, req *http.Request) string {
	t.Helper()
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.UserAgent()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	send := req.Clone(context.Background())
	target, err := http.NewRequest(req.Method, srv.URL, nil)
	require.NoError(t, err)
	send.URL = target.URL
	resp, err := srv.Client().Do(send)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return <-got
}

func newOutboundUATestAccount(t *testing.T, accountType string) *Account {
	t.Helper()
	a := newTestOAuthAccount(9401, nil)
	a.Type = accountType
	a.Status, a.Schedulable, a.Concurrency = StatusActive, true, 1
	a.Credentials = map[string]any{"access_token": "offline-token", "chatgpt_account_id": "offline-account"}
	return a
}

func newOutboundUAGinContext(t *testing.T, clientUA string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", http.NoBody)
	c.Request.Header.Set("User-Agent", clientUA)
	c.Request.Header.Set("Accept-Language", "zh-CN")
	return c
}

func TestAlphaSearchSetupTokenCarriesCodexIdentity(t *testing.T) {
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeSetupToken} {
		t.Run(accountType, func(t *testing.T) {
			account := newOutboundUATestAccount(t, accountType)
			svc := &OpenAIGatewayService{}
			c := newOutboundUAGinContext(t, "curl/8.5.0")
			req, err := svc.buildOpenAIAlphaSearchRequest(context.Background(), c, account, []byte("{}"), "offline-token")
			require.NoError(t, err)
			require.Equal(t, "chatgpt.com", req.Host)
			require.Equal(t, "offline-account", req.Header.Get("chatgpt-account-id"))
			require.Equal(t, CodexCanonicalUserAgent(), outboundUserAgent(t, req))
		})
	}
}

func TestUpstreamModelsRequestNeverSendsGoDefaultUA(t *testing.T) {
	for _, typ := range []string{AccountTypeAPIKey, AccountTypeCPR} {
		t.Run(typ, func(t *testing.T) {
			acc := newCPRTestAccount()
			acc.Type = typ
			req, err := buildOpenAIAPIKeyModelsRequest(context.Background(), acc, func(s string) (string, error) { return s, nil })
			require.NoError(t, err)
			require.Equal(t, "sub2api", outboundUserAgent(t, req))
			require.Empty(t, req.Header.Get("originator"))
			require.Empty(t, req.Header.Get("chatgpt-account-id"))
			if typ == AccountTypeAPIKey {
				acc.Credentials[credKeyHeaderOverrideEnabled] = true
				acc.Credentials[credKeyHeaderOverrides] = map[string]any{"User-Agent": "provider-client/1.0"}
				req, err = buildOpenAIAPIKeyModelsRequest(context.Background(), acc, func(s string) (string, error) { return s, nil })
				require.NoError(t, err)
				require.Equal(t, "provider-client/1.0", outboundUserAgent(t, req))
			}
		})
	}
}

func TestCountTokensDoesNotLeakClientUserAgent(t *testing.T) {
	for _, typ := range []string{AccountTypeOAuth, AccountTypeSetupToken} {
		t.Run(typ, func(t *testing.T) {
			svc := &OpenAIGatewayService{}
			req, err := svc.buildInputTokensUpstreamRequest(context.Background(), newOutboundUAGinContext(t, "claude-cli/1.0.60"), newOutboundUATestAccount(t, typ), []byte("{}"), "offline-token")
			require.NoError(t, err)
			require.Equal(t, CodexCanonicalUserAgent(), outboundUserAgent(t, req))
			require.Equal(t, "zh-CN", req.Header.Get("Accept-Language"))
			require.Empty(t, req.Header.Get("originator"))
		})
	}
}

func TestCountTokensPreservesRelayIdentity(t *testing.T) {
	for _, typ := range []string{AccountTypeAPIKey, AccountTypeCPR} {
		t.Run(typ, func(t *testing.T) {
			acc := newCPRTestAccount()
			acc.Type = typ
			req, err := cprTestService().buildInputTokensUpstreamRequest(context.Background(), newOutboundUAGinContext(t, "relay-client/1.0"), acc, []byte("{}"), cprTestClientKey)
			require.NoError(t, err)
			require.Equal(t, "relay-client/1.0", outboundUserAgent(t, req))
			require.Equal(t, "127.0.0.1:18081", req.URL.Host)
			require.Empty(t, req.Header.Get("originator"))
			require.Empty(t, req.Header.Get("chatgpt-account-id"))
		})
	}
}

func TestResponsesBridgeNormalizesUAWithoutRestoringOriginator(t *testing.T) {
	body := []byte("{\"model\":\"gpt-5.5\",\"prompt_cache_key\":\"anthropic-metadata-session-1\",\"input\":[{\"role\":\"user\",\"content\":\"hello\"}]}")
	c := newOutboundUAGinContext(t, "curl/8.5.0")
	c.Request.URL.Path = "/v1/responses"
	svc := &OpenAIGatewayService{}
	req, err := svc.buildUpstreamRequest(context.Background(), c, newOutboundUATestAccount(t, AccountTypeOAuth), body, "offline-token", false, "anthropic-metadata-session-1", false)
	require.NoError(t, err)
	require.Empty(t, req.Header.Get("originator"))
	require.Equal(t, CodexCanonicalClientVersion(), req.Header.Get("version"))
	require.Equal(t, CodexCanonicalUserAgent(), outboundUserAgent(t, req))
}

func TestCountTokensCustomUAAndForceCodexPriority(t *testing.T) {
	acc := newOutboundUATestAccount(t, AccountTypeOAuth)
	acc.Credentials["user_agent"] = "codex-tui/0.145.2 (Mac OS X 14.0; arm64) iTerm (codex-tui; 0.145.2)"
	svc := cprTestService()
	for _, force := range []bool{false, true} {
		svc.cfg.Gateway.ForceCodexCLI = force
		req, err := svc.buildInputTokensUpstreamRequest(context.Background(), newOutboundUAGinContext(t, "curl/8.5.0"), acc, []byte("{}"), "offline-token")
		require.NoError(t, err)
		if force {
			require.Equal(t, CodexCanonicalUserAgent(), outboundUserAgent(t, req))
		} else {
			require.Contains(t, outboundUserAgent(t, req), "Mac OS X 14.0")
		}
	}
}
