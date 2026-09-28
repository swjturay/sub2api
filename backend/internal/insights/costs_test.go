package insights

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
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

func TestSaveCostMonthPreservesWriteErrors(t *testing.T) {
	for _, fail := range []string{"", "month"} {
		t.Run("failure_"+fail, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
			mock.ExpectQuery("SELECT parent_account_id,deleted_at").WithArgs(int64(100)).WillReturnRows(sqlmock.NewRows([]string{"parent", "deleted"}).AddRow(nil, nil))
			mock.ExpectQuery("SELECT EXISTS").WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			monthly := mock.ExpectExec("INSERT INTO insights_cost_account_months").WithArgs(int64(100), "2026-08-01", int64(2), "subscription", nil, "", int64(9))
			if fail == "month" {
				monthly.WillReturnError(errors.New("month write failed"))
			} else {
				monthly.WillReturnResult(sqlmock.NewResult(0, 1))
			}
			err = NewQuery(db, "UTC", nil).SaveCostMonth(context.Background(), SaveCostMonthInput{AccountID: 100, Month: month, ContributorID: 2, PaymentMethod: "subscription", UpdatedBy: 9})
			if (err != nil) != (fail != "") {
				t.Fatalf("save error=%v failure=%s", err, fail)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCostDashboardDepartmentCoverage(t *testing.T) {
	for _, status := range []string{"complete", "not_configured", "invalid"} {
		t.Run(status, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			mock.ExpectQuery("SELECT NULLIF").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(nil))
			attributes := sqlmock.NewRows([]string{"id", "key", "type", "options", "enabled"})
			if status != "not_configured" {
				attributes.AddRow(1, "department", "text", nil, status != "invalid")
			}
			mock.ExpectQuery("SELECT id,key,type,options,enabled").WillReturnRows(attributes)
			if status == "complete" {
				mock.ExpectQuery("SELECT DISTINCT v.value").WillReturnRows(sqlmock.NewRows([]string{"value"}))
			}
			for _, query := range []string{"WITH monthly_usage AS", "WITH months AS", "SELECT root.platform", "SELECT u.id"} {
				mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"empty"}))
			}
			q := NewQuery(db, "UTC", nil)
			envelope, err := q.CostDashboard(context.Background(), CostFilter{Month: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)})
			if err != nil {
				t.Fatal(err)
			}
			coverage := envelope.Meta.Coverage
			if len(coverage) != 2 || coverage[1].Dataset != "department_attribute" || coverage[1].Status != status {
				t.Fatalf("coverage=%+v want department status %s", coverage, status)
			}
			if status == "invalid" && coverage[1].Detail == "" {
				t.Fatal("invalid department must preserve its diagnostic detail")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCostDashboardTableHidesZeroRequestsBeforePagination(t *testing.T) {
	for _, tc := range []struct {
		name       string
		month      time.Month
		active     int
		page       int
		search     string
		wantTotal  int
		wantItems  int
		wantActive int
	}{
		{"historical_page", time.August, 21, 2, "", 21, 1, 22},
		{"historical_empty", time.August, 0, 2, "", 0, 0, 1},
		{"historical_search_idle", time.August, 1, 1, "idle", 0, 0, 2},
		{"current_page", time.September, 21, 2, "", 21, 1, 21},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			mock.ExpectQuery("SELECT NULLIF").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(nil))
			mock.ExpectQuery("SELECT id,key,type,options,enabled").WillReturnRows(sqlmock.NewRows([]string{"id", "key", "type", "options", "enabled"}))
			month := time.Date(2026, tc.month, 1, 0, 0, 0, 0, time.UTC)
			columns := []string{"id", "name", "platform", "type", "status", "expires_at", "deleted_at", "registered", "month", "contributor_id", "payment", "contributor_name", "contributor_email", "contributor_status", "contributor_deleted", "department", "actual", "notes", "updated_at", "editor_id", "editor_name", "editor_email", "editor_status", "editor_deleted", "requests", "input", "write", "read", "output", "cost"}
			rows := sqlmock.NewRows(columns)
			for id := 0; id <= tc.active; id++ {
				name, requests, tokens := "used", 1, 7
				if id == 0 {
					name, requests, tokens = "idle", 0, 0
				} else if tc.month == time.August {
					// Historical zero-token requests are not zero-request accounts.
					tokens = 0
				}
				rows.AddRow(id, name, "openai", "oauth", "active", nil, nil, true, month, nil, "subscription", "", "", "", nil, "__unassigned__", "2.00", "", nil, nil, "", "", "", nil, requests, tokens, 0, 0, 0, "0.00")
			}
			mock.ExpectQuery("WITH monthly_usage AS").WillReturnRows(rows)
			for _, query := range []string{"WITH months AS", "SELECT root.platform", "SELECT u.id"} {
				mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"empty"}))
			}
			q := NewQuery(db, "UTC", nil)
			q.now = func() time.Time { return time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC) }
			envelope, err := q.CostDashboard(context.Background(), CostFilter{Month: month, Page: tc.page, PageSize: 20, Search: tc.search})
			if err != nil {
				t.Fatal(err)
			}
			data, ok := envelope.Data.(CostDashboard)
			if !ok {
				t.Fatalf("data type=%T", envelope.Data)
			}
			if data.Accounts.Total != tc.wantTotal || len(data.Accounts.Items) != tc.wantItems {
				t.Fatalf("zero-request rows affected table pagination: %+v", data.Accounts)
			}
			for _, row := range data.Accounts.Items {
				if row.RequestCount == 0 {
					t.Fatalf("zero-request account displayed: %d", row.ID)
				}
			}
			if tc.wantTotal == 0 && (data.Accounts.Page != 1 || data.Accounts.Pages != 1) {
				t.Fatalf("empty table pagination=%+v", data.Accounts)
			}
			if data.Summary.AccountCount != tc.wantActive || data.Summary.ActualCost != money(decimal.NewFromInt(int64(tc.wantActive*2))) {
				t.Fatalf("table visibility changed stored-cost summary: %+v", data.Summary)
			}
			if data.Summary.TotalAccounts != tc.active || data.Summary.CompletedAccounts != tc.active {
				t.Fatalf("completeness includes accounts hidden from the monthly table: %+v", data.Summary)
			}
			for _, department := range data.ContributionDepartments {
				if department.TotalAccounts != tc.active || department.CompletedAccounts != tc.active {
					t.Fatalf("department completeness includes unused accounts: %+v", department)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCostAccountRowsExcludeZeroTokensOnlyInCurrentMonth(t *testing.T) {
	for _, current := range []bool{true, false} {
		t.Run(map[bool]string{true: "current", false: "historical"}[current], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
			columns := []string{"id", "name", "platform", "type", "status", "expires_at", "deleted_at", "registered", "month", "contributor_id", "payment", "contributor_name", "contributor_email", "contributor_status", "contributor_deleted", "department", "actual", "notes", "updated_at", "editor_id", "editor_name", "editor_email", "editor_status", "editor_deleted", "requests", "input", "write", "read", "output", "cost"}
			rows := sqlmock.NewRows(columns)
			for i, tokens := range []int64{0, 7} {
				rows.AddRow(i+1, "account", "openai", "oauth", "active", nil, nil, true, month, nil, "subscription", "", "", "", nil, "__unassigned__", "20.00", "", nil, nil, "", "", "", nil, 1, 0, 0, tokens, 0, "9.00")
			}
			mock.ExpectQuery("WITH monthly_usage AS").WillReturnRows(rows)
			q := NewQuery(db, "UTC", nil)
			items, err := q.loadCostAccountRows(context.Background(), month, departmentBinding{}, current)
			if err != nil {
				t.Fatal(err)
			}
			wantCount, wantCost := 2, "40.00"
			if current {
				wantCount, wantCost = 1, "20.00"
				if len(items) != 1 || items[0].ID != 2 {
					t.Fatalf("cache-only usage must remain visible: %+v", items)
				}
			}
			if len(items) != wantCount || summarizeCostRows(items).ActualCost != wantCost {
				t.Fatalf("items=%+v summary=%+v", items, summarizeCostRows(items))
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCostTrendExcludesCurrentZeroTokensWithoutChangingHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{"month", "platform", "deleted", "registered", "contributor", "department", "actual", "cost", "tokens"}).
		AddRow(month.AddDate(0, -1, 0), "openai", nil, true, 1, "eng", "20.00", "9.00", 0).
		AddRow(month, "openai", nil, true, 1, "eng", "20.00", "9.00", 0).
		AddRow(month, "openai", nil, true, 1, "eng", "6.50", "10.00", 7)
	mock.ExpectQuery("WITH months AS").WillReturnRows(rows)
	q := NewQuery(db, "UTC", nil)
	trend, err := q.loadCostTrend(context.Background(), CostFilter{Month: month}, departmentBinding{}, "2026-09")
	if err != nil {
		t.Fatal(err)
	}
	current, previous := trend[len(trend)-1], trend[len(trend)-2]
	if current.TotalAccounts != 1 || current.CompletedAccounts != 1 || current.ActualCost != "6.50" || current.PlatformCost != "10.00" || current.Savings != "3.50" || previous.ActualCost != "20.00" || previous.TotalAccounts != 1 {
		t.Fatalf("trend=%+v", trend)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCostUsageDepartmentsAreRestrictedToIncludedAccounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`AND root.id=ANY\(\$5::bigint\[\]\)`).
		WithArgs("2026-09-01", month, month.AddDate(0, 1, 0), int64(0), pq.Array([]int64{100})).
		WillReturnRows(sqlmock.NewRows([]string{"platform", "contribution", "usage", "requests", "tokens", "cost"}).AddRow("openai", "eng", "ops", 2, 7, "10.00"))
	q := NewQuery(db, "UTC", nil)
	usage, flows, err := q.loadCostUsageDepartments(context.Background(), CostFilter{Month: month}, departmentBinding{}, []int64{100})
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 1 || usage[0].PlatformCost != "10.00" || usage[0].RequestCount != 2 || len(flows) != 1 || flows[0].PlatformCost != "10.00" {
		t.Fatalf("usage=%+v flows=%+v", usage, flows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSummarizeCostRowsDistinguishesMissingAndConfirmedZero(t *testing.T) {
	zero := "0.00"
	rows := []CostAccountItem{
		{Registered: true, RequestCount: 1, Contributor: &CostContributor{ID: 1}, ActualCost: nil, platformCostExact: decimal.NewFromInt(10)},
		{Registered: true, RequestCount: 1, Contributor: &CostContributor{ID: 2}, ActualCost: &zero, platformCostExact: decimal.NewFromInt(5)},
	}
	summary := summarizeCostRows(rows)
	if summary.PlatformCost != "15.00" || summary.ActualCost != "0.00" || summary.Savings != "5.00" || summary.CompletedAccounts != 1 || summary.TotalAccounts != 2 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestCostCompletenessExcludesIdleAccountsWithAndWithoutSpend(t *testing.T) {
	zero, paid := "0.00", "20.00"
	rows := []CostAccountItem{
		{Registered: true, RequestCount: 1, ActualCost: &zero},
		{Registered: true, RequestCount: 2},
		{Registered: true, ActualCost: &paid},
		{Registered: true},
	}
	summary := summarizeCostRows(rows)
	if summary.CompletedAccounts != 1 || summary.TotalAccounts != 2 || summary.ActualCost != paid {
		t.Fatalf("summary=%+v", summary)
	}
	departments := summarizeContributionDepartments(rows)
	if len(departments) != 1 || departments[0].CompletedAccounts != 1 || departments[0].TotalAccounts != 2 || departments[0].ActualCost != paid {
		t.Fatalf("departments=%+v", departments)
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
