//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCPRProxyEndpointWireStates(t *testing.T) {
	for _, tc := range []struct{ name, field, status, endpoint string }{
		{"credentials", ",\"outboundProxyEndpoint\":\"socks5h://user:p@ss@host:1080/path?token=secret#secret\"", "proxy", "socks5h://host:1080"},
		{"encoded_path", ",\"outboundProxyEndpoint\":\"http://user:pass@host:8080/%73ecret?token=secret#secret\"", "proxy", "http://host:8080"},
		{"socks5", ",\"outboundProxyEndpoint\":\"socks5://host:1080\"", "proxy", "socks5h://host:1080"},
		{"ipv6", ",\"outboundProxyEndpoint\":\"socks5h://u:p@[::1]:1080\"", "proxy", "socks5h://[::1]:1080"},
		{"direct_empty", ",\"outboundProxyEndpoint\":\"\"", "direct", ""},
		{"direct_null", ",\"outboundProxyEndpoint\":null", "direct", ""},
		{"missing", "", "unknown", ""},
		{"invalid", ",\"outboundProxyEndpoint\":\"http://user:secret@host:bad\"", "unknown", ""},
		{"unsupported", ",\"outboundProxyEndpoint\":\"ftp://user:secret@host\"", "unknown", ""},
		{"wrong_type", ",\"outboundProxyEndpoint\":42", "unknown", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := strings.Replace(cprTestDetailJSON("normal", "pro", ""), "\"enabled\":true", "\"enabled\":true"+tc.field, 1)
			server, _ := startCPRAdminStub(t, http.StatusOK, payload)
			account := newCPRTestAccount()
			account.Credentials["admin_base_url"] = server.URL
			state, err := NewCPRQuotaService(cprTestConfig()).FetchAccountState(context.Background(), account)
			require.NoError(t, err)
			require.Equal(t, tc.status, state.OutboundProxyStatus)
			require.Equal(t, tc.endpoint, state.OutboundProxyEndpoint)
			updates := buildCPRCodexExtraUpdates(account, state)
			require.Equal(t, tc.endpoint, updates[CPROutboundProxyExtraKey])
			require.Equal(t, tc.status, updates[CPROutboundProxyStatusExtraKey])
			require.NotContains(t, updates, "codex_usage_updated_at", "proxy observations must not freshen stale quota")
			encoded, err := json.Marshal(updates)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "secret")
			require.NotContains(t, string(encoded), "p@ss")
		})
	}
}

func TestCPRProxySnapshotTransitionsAndFreshness(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	account := newCPRTestAccount()
	account.Extra = map[string]any{CPROutboundProxyExtraKey: "socks5h://old:1080", CPROutboundProxyStatusExtraKey: "proxy", CPROutboundProxyUpdatedAtExtraKey: now.Format(time.RFC3339)}
	same := &CPRAccountState{OutboundProxyEndpoint: "socks5h://old:1080", OutboundProxyStatus: "proxy", FetchedAt: now.Add(time.Minute)}
	require.Empty(t, buildCPRCodexExtraUpdates(account, same), "do not write unchanged display data on every request")
	same.FetchedAt = now.Add(5 * time.Minute)
	require.Equal(t, same.FetchedAt.Format(time.RFC3339), buildCPRCodexExtraUpdates(account, same)[CPROutboundProxyUpdatedAtExtraKey])
	for _, status := range []string{"direct", "unknown"} {
		updates := buildCPRCodexExtraUpdates(account, &CPRAccountState{OutboundProxyStatus: status, FetchedAt: now.Add(time.Second)})
		require.Equal(t, "", updates[CPROutboundProxyExtraKey], "clear the old endpoint immediately")
		require.Equal(t, status, updates[CPROutboundProxyStatusExtraKey])
	}
	require.Empty(t, buildCPRCodexExtraUpdates(account, nil), "failed refresh preserves the last observation")
}

func TestCPRProxyRefreshDoesNotFreshenOldQuota(t *testing.T) {
	oldTime := time.Unix(1800000000, 0).UTC()
	account := newCPRTestAccount()
	account.Extra["codex_usage_updated_at"] = oldTime.Format(time.RFC3339)
	payload := strings.Replace(cprTestDetailJSON("normal", "pro", ""), "\"enabled\":true", "\"enabled\":true,\"outboundProxyEndpoint\":\"socks5h://host:1080\"", 1)
	server, _ := startCPRAdminStub(t, http.StatusOK, payload)
	account.Credentials["admin_base_url"] = server.URL
	svc := &AccountUsageService{cprQuotaService: NewCPRQuotaService(cprTestConfig())}
	usage := &UsageInfo{UpdatedAt: &oldTime}
	svc.refreshCPRCodexSnapshot(context.Background(), account, usage, oldTime.Add(time.Hour))
	require.Equal(t, "socks5h://host:1080", account.Extra[CPROutboundProxyExtraKey])
	require.Equal(t, oldTime.Format(time.RFC3339), account.Extra["codex_usage_updated_at"])
	require.Equal(t, oldTime, *usage.UpdatedAt)
}

func TestCPRFailedProxyRefreshPreservesObservation(t *testing.T) {
	account := newCPRTestAccount()
	account.Extra = map[string]any{CPROutboundProxyExtraKey: "socks5h://old:1080", CPROutboundProxyStatusExtraKey: "proxy", CPROutboundProxyUpdatedAtExtraKey: "2026-09-21T00:00:00Z"}
	before, err := json.Marshal(account.Extra)
	require.NoError(t, err)
	server, _ := startCPRAdminStub(t, http.StatusServiceUnavailable, "{}")
	account.Credentials["admin_base_url"] = server.URL
	svc := &AccountUsageService{cprQuotaService: NewCPRQuotaService(cprTestConfig())}
	svc.refreshCPRCodexSnapshot(context.Background(), account, nil, time.Now())
	after, err := json.Marshal(account.Extra)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
}
