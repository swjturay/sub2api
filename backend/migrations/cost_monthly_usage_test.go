package migrations

import (
	"strings"
	"testing"
)

func TestCostMonthlyUsageMigrationDefinesAccountMonthRollup(t *testing.T) {
	content, err := FS.ReadFile("251_insights_cost_monthly_usage.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(content))
	for _, fragment := range []string{
		"create table if not exists insights_cost_monthly_usage",
		"primary key (month, account_id)",
		"platform_cost numeric",
		"idx_insights_cost_monthly_usage_account_month",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}
