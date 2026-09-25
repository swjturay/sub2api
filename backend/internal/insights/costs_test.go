package insights

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestNormalizeCostAmount(t *testing.T) {
	for input, expected := range map[string]string{"0": "0.00", "6.5": "6.50", "12.00": "12.00"} {
		actual, err := NormalizeCostAmount(input)
		if err != nil || actual != expected {
			t.Fatalf("NormalizeCostAmount(%q)=%q,%v want %q", input, actual, err, expected)
		}
	}
	for _, input := range []string{"-1", "1.234", "1e3", "not-money"} {
		if _, err := NormalizeCostAmount(input); err == nil {
			t.Fatalf("NormalizeCostAmount(%q) accepted invalid value", input)
		}
	}
}

func TestSummarizeCostRowsDistinguishesMissingAndConfirmedZero(t *testing.T) {
	zero := "0.00"
	rows := []CostAccountItem{
		{Registered: true, Contributor: &CostContributor{ID: 1}, ActualCost: nil, platformCostExact: decimal.NewFromInt(10)},
		{Registered: true, Contributor: &CostContributor{ID: 2}, ActualCost: &zero, platformCostExact: decimal.NewFromInt(5)},
	}
	summary := summarizeCostRows(rows)
	if summary.PlatformCost != "15.00" || summary.ActualCost != "0.00" || summary.Savings != "5.00" || summary.CompletedAccounts != 1 || summary.TotalAccounts != 2 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestFilterCostRowsKeepsUnregisteredOutOfMissingActualCost(t *testing.T) {
	rows := []CostAccountItem{{ID: 1, Registered: true}, {ID: 2, Registered: false}}
	filtered := filterCostRows(rows, CostFilter{Completeness: "missing"})
	if len(filtered) != 1 || filtered[0].ID != 1 {
		t.Fatalf("filtered=%+v", filtered)
	}
}

func TestFilterCostRowsTreatsDeletedAsASeparateStatus(t *testing.T) {
	now := time.Now()
	rows := []CostAccountItem{{ID: 1, Status: "active"}, {ID: 2, Status: "active", DeletedAt: &now}}
	deleted := filterCostRows(rows, CostFilter{Status: "deleted"})
	active := filterCostRows(rows, CostFilter{Status: "active"})
	if len(deleted) != 1 || deleted[0].ID != 2 || len(active) != 1 || active[0].ID != 1 {
		t.Fatalf("deleted=%+v active=%+v", deleted, active)
	}
}
