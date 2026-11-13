package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCostTrendIndexesMigration(t *testing.T) {
	content, err := FS.ReadFile("250_add_cost_trend_indexes_notx.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_created_account_cost_trend ON usage_logs (created_at, account_id)")
	require.Contains(t, sql, "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_attribute_values_attribute_user_cost_trend ON user_attribute_values (attribute_id, user_id)")
}
