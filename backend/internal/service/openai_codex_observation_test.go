//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCodexObservationSafetyPresence(t *testing.T) {
	for _, value := range []string{"", "false", "true", "invalid"} {
		t.Run(value, func(t *testing.T) {
			h := http.Header{"x-codex-safety-buffering-enabled": []string{value}}
			o := codexSafetyObservation(h)
			require.True(t, o.EnabledPresent)
			require.False(t, o.FasterModelPresent)
			if value == "true" || value == "false" {
				require.NotNil(t, o.Enabled)
				require.Equal(t, value == "true", *o.Enabled)
			} else {
				require.Nil(t, o.Enabled)
			}
		})
	}
	require.False(t, codexSafetyObservation(nil).EnabledPresent)
	h := http.Header{}
	h.Set(openAICodexSafetyBufferingFasterModelHeader, "\x00gpt-6-luna\xff")
	require.Equal(t, "gpt-6-luna", codexSafetyObservation(h).FasterModel)
}

func TestCodexObservationAttemptSnapshotAndUsage(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{JWT: config.JWTConfig{Secret: "synthetic-test-key"}}}
	account := &Account{ID: 3000, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	req := httptest.NewRequest(http.MethodPost, "https://chatgpt.com/backend-api/codex/responses", nil)
	req.Header.Set("Cookie", "__cflb=A; unrelated=secret")
	outbound := svc.codexRouteDigest(account, req.Cookies())
	resp := &http.Response{Header: http.Header{"Set-Cookie": []string{"__oailb=B; Secure"}}}
	svc.observeCodexHTTPResponse(req, account, outbound, resp)
	saved := codexObservationFromResponse(resp)
	require.NotNil(t, saved)
	require.NotEqual(t, saved.Route.OutboundDigest, saved.Route.ResponseDigest)
	// Simulate a later overlapping request/header change before async recording.
	done := make(chan struct{})
	go func() {
		req.Header.Set("Cookie", "__cflb=concurrent")
		resp.Header.Set("Set-Cookie", "__cflb=new")
		close(done)
	}()
	<-done
	require.Equal(t, outbound, saved.Route.OutboundDigest)
	require.NotEqual(t, svc.codexRouteDigest(account, req.Cookies()), saved.Route.OutboundDigest)
	repo := &openAIRecordUsageLogRepoStub{inserted: true}
	billing := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	recorder := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	err := recorder.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{RequestID: "observation-test", Model: "gpt-6-astra", Duration: time.Second, CodexObservation: saved},
		APIKey: &APIKey{ID: 1000, Quota: 100, Group: &Group{RateMultiplier: 1}}, User: &User{ID: 2000}, Account: account, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{},
	})
	require.NoError(t, err)
	require.Equal(t, saved, repo.lastLog.CodexObservation)
	serialized, err := json.Marshal(repo.lastLog.CodexObservation)
	require.NoError(t, err)
	require.NotContains(t, string(serialized), "__cflb")
	require.NotContains(t, string(serialized), "secret")
}

func TestCodexObservationDigestIsolationAndHints(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{JWT: config.JWTConfig{Secret: "synthetic-test-key"}}}
	a := &Account{ID: 1}
	b := &Account{ID: 2}
	cookies := []*http.Cookie{{Name: "__cflb", Value: "route"}, {Name: "__oailb", Value: "ticket"}}
	digest := svc.codexRouteDigest(a, cookies)
	require.NotEmpty(t, digest)
	require.Equal(t, digest, svc.codexRouteDigest(a, []*http.Cookie{cookies[1], cookies[0]}))
	require.NotEqual(t, digest, svc.codexRouteDigest(b, cookies))
	require.Empty(t, (&OpenAIGatewayService{}).codexRouteDigest(a, cookies))
	require.Empty(t, svc.codexRouteDigest(a, []*http.Cookie{{Name: "session", Value: "private"}}))
	for _, host := range []string{"chat.gateway.unified-123.api.openai.com", "unified-123.evil.test", "chat.gateway.unified-x.api.openai.com"} {
		payload, _ := json.Marshal(map[string]string{"host": host})
		jwt := "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
		hint := codexResponseGatewayHint([]*http.Cookie{{Name: "__oailb", Value: jwt}})
		if strings.HasSuffix(host, "123.api.openai.com") {
			require.Equal(t, "unified-123", hint)
		} else {
			require.Empty(t, hint)
		}
	}
}
