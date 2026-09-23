//go:build unit

package migrations

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestMigration240AddsDurableObservedActivity(t *testing.T) {
	content, err := FS.ReadFile("240_insights_observed_activity.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS first_observed_call_at TIMESTAMPTZ")
	require.Contains(t, sql, "first_observed_call_at, first_call_at")
	require.Contains(t, sql, "idx_insights_user_lifecycle_first_observed")
}
