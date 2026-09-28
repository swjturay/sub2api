//go:build unit

package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration241CreatesMonthlyCostDataWithoutDuplicatingUsageCost(t *testing.T) {
	content, err := FS.ReadFile("241_insights_cost_data.sql")
	require.NoError(t, err)
	sql := string(content)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS insights_cost_account_months")
	require.Contains(t, sql, "PRIMARY KEY (account_id, month)")
	require.Contains(t, sql, "payment_method IN ('subscription', 'payg', 'other')")
	require.Contains(t, sql, "actual_cost IS NULL OR actual_cost >= 0")
	require.Contains(t, sql, "updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL")
	require.NotContains(t, sql, "platform_cost NUMERIC")
}

func TestMigration242BackfillsAccountContributorsWithoutChangingMonthlySpend(t *testing.T) {
	content, err := FS.ReadFile("242_insights_cost_account_contributors.sql")
	require.NoError(t, err)
	sql := string(content)
	require.Contains(t, sql, "account_id BIGINT PRIMARY KEY REFERENCES accounts(id)")
	require.Contains(t, sql, "contributor_user_id BIGINT NOT NULL REFERENCES users(id)")
	require.Contains(t, sql, "WHERE registered = TRUE AND contributor_user_id IS NOT NULL")
	require.Contains(t, sql, "ORDER BY account_id, updated_at DESC, month DESC")
	require.Contains(t, sql, "ON CONFLICT (account_id) DO NOTHING")
	require.NotContains(t, sql, "UPDATE insights_cost_account_months")
	require.NotContains(t, sql, "DELETE FROM")
}
