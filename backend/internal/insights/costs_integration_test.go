package insights

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
)

func TestCostDashboardPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("INSIGHTS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("INSIGHTS_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	schema := fmt.Sprintf("insights_cost_%d", time.Now().UnixNano())
	if _, err = db.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = db.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`) }()
	if _, err = db.ExecContext(ctx, `SET search_path TO `+schema); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE users(id BIGINT PRIMARY KEY,username TEXT NOT NULL,email TEXT NOT NULL,status TEXT NOT NULL,deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE accounts(id BIGINT PRIMARY KEY,name TEXT NOT NULL,platform TEXT NOT NULL,type TEXT NOT NULL,status TEXT NOT NULL,expires_at TIMESTAMPTZ,deleted_at TIMESTAMPTZ,parent_account_id BIGINT)`,
		`CREATE TABLE usage_logs(id BIGSERIAL PRIMARY KEY,user_id BIGINT NOT NULL,account_id BIGINT NOT NULL,input_tokens BIGINT NOT NULL,cache_creation_tokens BIGINT NOT NULL,cache_read_tokens BIGINT NOT NULL,output_tokens BIGINT NOT NULL,account_stats_cost NUMERIC,total_cost NUMERIC NOT NULL,account_rate_multiplier NUMERIC,created_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE insights_settings(key TEXT PRIMARY KEY,value JSONB NOT NULL)`,
		`CREATE TABLE user_attribute_definitions(id BIGINT PRIMARY KEY,key TEXT NOT NULL,type TEXT NOT NULL,options JSONB NOT NULL,enabled BOOLEAN NOT NULL,deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE user_attribute_values(user_id BIGINT NOT NULL,attribute_id BIGINT NOT NULL,value TEXT NOT NULL)`,
	}
	for _, statement := range statements {
		if _, err = db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	applyMigration := func(name string) {
		t.Helper()
		content, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			t.Fatal(err)
		}
	}
	applyMigration("241_insights_cost_data.sql")
	if _, err = db.ExecContext(ctx, `INSERT INTO users VALUES(1,'contributor','c@example','active',NULL),(2,'consumer','u@example','active',NULL),(9,'admin','a@example','active',NULL); INSERT INTO accounts VALUES(100,'root','openai','oauth','active',NULL,NULL,NULL),(101,'shadow','openai','oauth','active',NULL,NULL,100),(200,'unconfigured','gemini','oauth','active',NULL,NULL,NULL); INSERT INTO user_attribute_definitions VALUES(11,'department','select','[{"value":"eng","label":"研发"},{"value":"ops","label":"运营"}]',true,NULL); INSERT INTO user_attribute_values VALUES(1,11,'eng'),(2,11,'ops'); INSERT INTO insights_settings VALUES('department_attribute_id','11'); INSERT INTO insights_cost_account_months(account_id,month,registered,contributor_user_id,payment_method,actual_cost,notes,updated_by) VALUES(100,'2026-09-01',true,1,'subscription',6.50,'confirmed',9)`); err != nil {
		t.Fatal(err)
	}
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err = db.ExecContext(ctx, `INSERT INTO accounts VALUES(300,'idle-paid','openai','oauth','active',NULL,NULL,NULL);
INSERT INTO insights_cost_account_months(account_id,month,registered,contributor_user_id,payment_method,actual_cost,updated_by) VALUES(300,'2026-08-01',true,1,'subscription',20,9),(300,'2026-09-01',true,1,'subscription',20,9)`); err != nil {
		t.Fatal(err)
	}
	applyMigration("242_insights_cost_account_contributors.sql")
	if _, err = db.ExecContext(ctx, `INSERT INTO usage_logs(user_id,account_id,input_tokens,cache_creation_tokens,cache_read_tokens,output_tokens,account_stats_cost,total_cost,account_rate_multiplier,created_at) VALUES(2,100,0,0,0,0,NULL,4,1,$1),(2,101,5,0,0,1,3,99,2,$1),(2,300,0,0,0,0,NULL,9,1,$1)`, month.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	q := NewQuery(db, "UTC", &start)
	q.now = func() time.Time { return time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC) }
	envelope, err := q.CostDashboard(ctx, CostFilter{Month: month, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := envelope.Data.(CostDashboard)
	if !ok {
		t.Fatalf("data type=%T", envelope.Data)
	}
	if data.Summary.PlatformCost != "10.00" || data.Summary.ActualCost != "6.50" || data.Summary.Savings != "3.50" || data.Summary.AccountCount != 1 {
		t.Fatalf("summary=%+v", data.Summary)
	}
	if len(data.Accounts.Items) != 1 || data.Accounts.Items[0].ID != 100 || data.Accounts.Total != 1 {
		t.Fatalf("accounts=%+v", data.Accounts.Items)
	}
	var root *CostAccountItem
	for i := range data.Accounts.Items {
		if data.Accounts.Items[i].ID == 100 {
			root = &data.Accounts.Items[i]
		}
	}
	if root == nil || root.PlatformCost != "10.00" || root.RequestCount != 2 || root.Tokens.Total != 6 {
		t.Fatalf("root=%+v", root)
	}
	if len(data.Flows) != 1 || data.Flows[0].ContributionDepartment != "研发" || data.Flows[0].UsageDepartment != "运营" || data.Flows[0].PlatformCost != "10.00" {
		t.Fatalf("flows=%+v", data.Flows)
	}
	if len(data.UsageDepartments) != 1 || data.UsageDepartments[0].PlatformCost != "10.00" || data.UsageDepartments[0].RequestCount != 2 {
		t.Fatalf("usage departments=%+v", data.UsageDepartments)
	}
	if len(data.ContributionDepartments) != 1 || data.ContributionDepartments[0].ActualCost != "6.50" || data.ContributionDepartments[0].AccountCount != 1 {
		t.Fatalf("contribution departments=%+v", data.ContributionDepartments)
	}
	current := data.Trend[len(data.Trend)-1]
	previous := data.Trend[len(data.Trend)-2]
	if current.TotalAccounts != 1 || current.ActualCost != "6.50" || current.PlatformCost != "10.00" || current.Savings != "3.50" || previous.ActualCost != "20.00" {
		t.Fatalf("current=%+v previous=%+v", current, previous)
	}
	// A zero-token account's saved cost reappears in historical months.
	q.timezone = "Asia/Tokyo"
	q.now = func() time.Time { return time.Date(2026, 9, 30, 16, 0, 0, 0, time.UTC) }
	loc := mustLocation(q.timezone)
	filter := CostFilter{Month: time.Date(2026, 9, 1, 0, 0, 0, 0, loc), Page: 1, PageSize: 20}
	historical, err := q.CostDashboard(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	history, ok := historical.Data.(CostDashboard)
	if !ok {
		t.Fatalf("historical data type=%T", historical.Data)
	}
	if history.InProgress || history.Accounts.Total != 3 || history.Summary.ActualCost != "26.50" || history.Summary.PlatformCost != "19.00" {
		t.Fatalf("historical=%+v", history)
	}
	// Even cache-only usage makes an account visible again without editing its saved cost.
	q.now = func() time.Time { return month.AddDate(0, 0, 23) }
	if _, err = db.ExecContext(ctx, `INSERT INTO usage_logs(user_id,account_id,input_tokens,cache_creation_tokens,cache_read_tokens,output_tokens,total_cost,created_at) VALUES(2,300,0,0,7,0,0,$1)`, month.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	resumed, err := q.CostDashboard(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	visible, ok := resumed.Data.(CostDashboard)
	if !ok {
		t.Fatalf("resumed data type=%T", resumed.Data)
	}
	if !visible.InProgress || visible.Accounts.Total != 2 || visible.Summary.ActualCost != "26.50" || visible.Summary.PlatformCost != "19.00" || visible.Summary.Savings != "-7.50" {
		t.Fatalf("resumed=%+v", visible)
	}
	if len(visible.Flows) != 1 || visible.Flows[0].PlatformCost != "19.00" || visible.Trend[len(visible.Trend)-1].TotalAccounts != 2 {
		t.Fatalf("resumed flow/trend=%+v", visible)
	}

	q.timezone = "UTC"
	readMonth := func(selected time.Time) CostDashboard {
		t.Helper()
		envelope, err := q.CostDashboard(ctx, CostFilter{Month: selected, Page: 1, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		data, ok := envelope.Data.(CostDashboard)
		if !ok {
			t.Fatalf("monthly data type=%T", envelope.Data)
		}
		return data
	}
	account := func(data CostDashboard, id int64) CostAccountItem {
		t.Helper()
		for _, item := range data.Accounts.Items {
			if item.ID == id {
				return item
			}
		}
		t.Fatalf("missing account %d in %s", id, data.Month)
		return CostAccountItem{}
	}
	// A September-only registration also configures August, without copying spend.
	august := month.AddDate(0, -1, 0)
	before := account(readMonth(august), 100)
	if !before.Registered || before.Contributor == nil || before.Contributor.ID != 1 || before.ActualCost != nil {
		t.Fatalf("historical configuration=%+v", before)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO usage_logs(user_id,account_id,input_tokens,cache_creation_tokens,cache_read_tokens,output_tokens,total_cost,created_at) VALUES(2,101,5,0,0,0,4,$1)`, august.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	spend := "8.00"
	if err := q.SaveCostMonth(ctx, SaveCostMonthInput{AccountID: 100, Month: august, ContributorID: 2, PaymentMethod: "subscription", ActualCost: &spend, UpdatedBy: 9}); err != nil {
		t.Fatal(err)
	}
	for _, selected := range []time.Time{month.AddDate(0, -2, 0), august, month} {
		item := account(readMonth(selected), 100)
		if item.Contributor == nil || item.Contributor.ID != 2 || item.Contributor.DepartmentID != "ops" {
			t.Fatalf("account contributor differs in %s: %+v", selected, item)
		}
	}
	september := readMonth(month)
	septAccount := account(september, 100)
	augAccount := account(readMonth(august), 100)
	if septAccount.ActualCost == nil || *septAccount.ActualCost != "6.50" || augAccount.ActualCost == nil || *augAccount.ActualCost != "8.00" || account(readMonth(month.AddDate(0, -2, 0)), 100).ActualCost != nil {
		t.Fatal("changing the account contributor must not copy or overwrite another month's spend")
	}
	filtered, err := q.CostDashboard(ctx, CostFilter{Month: month, Department: "ops"})
	if err != nil {
		t.Fatal(err)
	}
	ops, ok := filtered.Data.(CostDashboard)
	if !ok {
		t.Fatalf("filtered data type=%T", filtered.Data)
	}
	if ops.Summary.AccountCount != 1 || ops.Summary.PlatformCost != "10.00" || len(ops.Flows) != 1 || ops.Flows[0].ContributionDepartmentID != "ops" || ops.Flows[0].PlatformCost != "10.00" || len(ops.ContributionDepartments) != 1 || ops.ContributionDepartments[0].ID != "ops" || len(ops.UsageDepartments) != 1 || ops.UsageDepartments[0].PlatformCost != "10.00" {
		t.Fatalf("account-wide department attribution=%+v", ops)
	}
	if ops.Trend[len(ops.Trend)-2].PlatformCost != "4.00" || ops.Trend[len(ops.Trend)-1].PlatformCost != "10.00" {
		t.Fatalf("historical trend attribution=%+v", ops.Trend)
	}
	// A failed monthly write cannot change the shared contributor.
	invalid := "-1"
	if err := q.SaveCostMonth(ctx, SaveCostMonthInput{AccountID: 100, Month: august, ContributorID: 1, PaymentMethod: "subscription", ActualCost: &invalid, UpdatedBy: 9}); err == nil {
		t.Fatal("negative monthly spend accepted")
	}
	if item := account(readMonth(month), 100); item.Contributor.ID != 2 {
		t.Fatal("failed save leaked an account contributor update")
	}
	// A legacy backend still writes only the monthly table during mixed-version rollout.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE insights_cost_account_months SET contributor_user_id=1,updated_at=NOW() WHERE account_id=100 AND month='2026-08-01'`); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	var transactionalOwner int64
	if err := tx.QueryRowContext(ctx, `SELECT contributor_user_id FROM insights_cost_account_contributors WHERE account_id=100`).Scan(&transactionalOwner); err != nil || transactionalOwner != 1 {
		_ = tx.Rollback()
		t.Fatalf("legacy writer was not synchronized: owner=%d err=%v", transactionalOwner, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if item := account(readMonth(month), 100); item.Contributor.ID != 2 {
		t.Fatal("rollback failed to restore the shared contributor")
	}
	// Stopping and later re-registering never clears ownership or bypasses a stop.
	if err := q.StopCostAccountAfter(ctx, 100, month, 9); err != nil {
		t.Fatal(err)
	}
	october := month.AddDate(0, 1, 0)
	if item := account(readMonth(october), 100); item.Registered || item.Contributor == nil || item.Contributor.ID != 2 {
		t.Fatalf("stopped account=%+v", item)
	}
	if err := q.SaveCostMonth(ctx, SaveCostMonthInput{AccountID: 100, Month: month.AddDate(0, 2, 0), ContributorID: 2, PaymentMethod: "payg", UpdatedBy: 9}); err != nil {
		t.Fatal(err)
	}
	if account(readMonth(october), 100).Registered {
		t.Fatal("future registration overrode an explicit stop")
	}
	// Migration chooses the latest edit (not latest month), ignores stops, and is replay-safe.
	if _, err := db.ExecContext(ctx, `INSERT INTO accounts VALUES(400,'legacy','openai','oauth','active',NULL,NULL,NULL);
INSERT INTO insights_cost_account_months(account_id,month,registered,contributor_user_id,payment_method,actual_cost,updated_at) VALUES
(400,'2026-07-01',true,1,'subscription',3,'2026-09-25'),(400,'2026-09-01',true,2,'subscription',7,'2026-09-20'),(400,'2026-10-01',false,NULL,NULL,NULL,'2026-09-26')`); err != nil {
		t.Fatal(err)
	}
	// Recreate a pre-migration legacy account, without an account-level record.
	if _, err := db.ExecContext(ctx, `DELETE FROM insights_cost_account_contributors WHERE account_id=400`); err != nil {
		t.Fatal(err)
	}
	applyMigration("242_insights_cost_account_contributors.sql")
	applyMigration("242_insights_cost_account_contributors.sql")
	var owner int64
	if err := db.QueryRowContext(ctx, `SELECT contributor_user_id FROM insights_cost_account_contributors WHERE account_id=400`).Scan(&owner); err != nil || owner != 1 {
		t.Fatalf("legacy owner=%d err=%v", owner, err)
	}
	if item := account(readMonth(month), 100); item.Contributor.ID != 2 {
		t.Fatal("migration replay overwrote an existing account contributor")
	}
}
