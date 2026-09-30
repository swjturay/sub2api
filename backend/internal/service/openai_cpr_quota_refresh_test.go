//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCPRQuotaRefreshFailureIsNotZeroUsage(t *testing.T) {
	for _, cached := range []bool{false, true} {
		name := "without_snapshot"
		if cached {
			name = "with_snapshot"
		}
		t.Run(name, func(t *testing.T) {
			account := newCPRTestAccount()
			delete(account.Credentials, "admin_api_key")
			old := time.Now().Add(-time.Hour).Truncate(time.Second)
			if cached {
				account.Extra = map[string]any{"codex_5h_used_percent": 100.0, "codex_7d_used_percent": 76.0,
					"codex_5h_reset_at":      time.Now().Add(time.Hour).Format(time.RFC3339),
					"codex_7d_reset_at":      time.Now().Add(24 * time.Hour).Format(time.RFC3339),
					"codex_usage_updated_at": old.Format(time.RFC3339)}
			}
			svc := &AccountUsageService{cprQuotaService: NewCPRQuotaService(cprTestConfig()), usageLogRepo: &usageBatchLogRepoStub{}}
			usage, err := svc.getUsageForAccount(context.Background(), account, true)
			require.NoError(t, err)
			require.Equal(t, "cpr_not_configured", usage.ErrorCode)
			require.NotEmpty(t, usage.Error)
			if cached {
				require.Equal(t, 100.0, usage.FiveHour.Utilization)
				require.Equal(t, 76.0, usage.SevenDay.Utilization)
				require.NotNil(t, usage.UpdatedAt)
				require.True(t, usage.UpdatedAt.Equal(old), "failed refresh must preserve snapshot age")
			} else {
				require.Nil(t, usage.FiveHour, "local accounting must not invent a zero quota")
				require.Nil(t, usage.SevenDay)
				require.Nil(t, usage.UpdatedAt)
			}
		})
	}
}

func TestCPRQuotaRefreshResponseStates(t *testing.T) {
	now := time.Now()
	loc := time.FixedZone("UTC+8", 8*60*60)
	windows := cprTestWindow("primary", 18000, 100, now.Add(time.Hour).In(loc).Format(cprDisplayTimeLayout)) + "," +
		cprTestWindow("secondary", 604800, 76, now.Add(24*time.Hour).In(loc).Format(cprDisplayTimeLayout))
	for _, tc := range []struct {
		name       string
		status     int
		body, code string
	}{
		{"success", 200, cprTestDetailJSON("quota_exhausted", "plus", windows), ""},
		{"unavailable", 503, `{"code":503,"message":"PRIVATE_DIAGNOSTIC","data":null}`, "cpr_quota_unavailable"},
		{"rejected_key", 401, `{"code":40103,"message":"PRIVATE_DIAGNOSTIC","data":null}`, "cpr_admin_key_invalid"},
		{"missing_account", 404, `{"code":40401,"message":"PRIVATE_DIAGNOSTIC","data":null}`, "cpr_account_missing"},
		{"unknown_windows", 200, cprTestDetailJSON("normal", "plus", ""), "cpr_quota_unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := startCPRAdminStub(t, tc.status, tc.body)
			account := newCPRTestAccount()
			account.Credentials["admin_base_url"] = server.URL
			svc := &AccountUsageService{cprQuotaService: NewCPRQuotaService(cprTestConfig()), usageLogRepo: &usageBatchLogRepoStub{}}
			usage, err := svc.getOpenAIUsage(context.Background(), account, true)
			require.NoError(t, err)
			require.Equal(t, tc.code, usage.ErrorCode)
			require.NotContains(t, usage.Error, "PRIVATE_DIAGNOSTIC")
			if tc.code == "" {
				require.Empty(t, usage.Error)
				require.Equal(t, 100.0, usage.FiveHour.Utilization)
				require.Equal(t, 76.0, usage.SevenDay.Utilization)
				require.WithinDuration(t, now, *usage.UpdatedAt, 5*time.Second)
			} else {
				require.NotEmpty(t, usage.Error)
				require.Nil(t, usage.FiveHour)
				require.Nil(t, usage.SevenDay)
				require.Nil(t, usage.UpdatedAt)
			}
		})
	}
}
