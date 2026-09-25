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
