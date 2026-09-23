//go:build unit

package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration239CreatesInsightsFactsAndTruthfulCoverage(t *testing.T) {
	content, err := FS.ReadFile("239_insights_v1.sql")
	require.NoError(t, err)
	sql := string(content)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS insights_call_facts")
	require.Contains(t, sql, "call_id UUID NOT NULL UNIQUE")
	require.Contains(t, sql, "statistical_at TIMESTAMPTZ NOT NULL")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS insights_error_facts")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS insights_user_model_daily")
	require.Contains(t, sql, "usage_tpot_sum_ms DOUBLE PRECISION")
	require.Contains(t, sql, "gateway_pre_forward_sum_ms BIGINT")
	require.Contains(t, sql, "usage_first_token_sum_ms BIGINT")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS insights_user_lifecycle")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS insights_model_metadata")
	require.Contains(t, sql, `"call_facts":"unknown"`)
	require.Contains(t, sql, `"rebuild_required":true`)
	require.Contains(t, sql, "expected_version BIGINT NOT NULL DEFAULT 1")
	require.NotContains(t, sql, `"call_facts":"complete"`)
}
