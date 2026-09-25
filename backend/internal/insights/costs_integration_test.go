package insights

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

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
		`CREATE TABLE insights_cost_account_months(account_id BIGINT NOT NULL,month DATE NOT NULL,registered BOOLEAN NOT NULL,contributor_user_id BIGINT,payment_method TEXT,actual_cost NUMERIC(20,2),notes TEXT NOT NULL DEFAULT '',updated_by BIGINT,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(account_id,month))`,
	}
	for _, statement := range statements {
		if _, err = db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO users VALUES(1,'contributor','c@example','active',NULL),(2,'consumer','u@example','active',NULL),(9,'admin','a@example','active',NULL); INSERT INTO accounts VALUES(100,'root','openai','oauth','active',NULL,NULL,NULL),(101,'shadow','openai','oauth','active',NULL,NULL,100),(200,'unconfigured','gemini','oauth','active',NULL,NULL,NULL); INSERT INTO user_attribute_definitions VALUES(11,'department','select','[{"value":"eng","label":"研发"},{"value":"ops","label":"运营"}]',true,NULL); INSERT INTO user_attribute_values VALUES(1,11,'eng'),(2,11,'ops'); INSERT INTO insights_settings VALUES('department_attribute_id','11'); INSERT INTO insights_cost_account_months(account_id,month,registered,contributor_user_id,payment_method,actual_cost,notes,updated_by) VALUES(100,'2026-09-01',true,1,'subscription',6.50,'confirmed',9)`); err != nil {
		t.Fatal(err)
	}
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err = db.ExecContext(ctx, `INSERT INTO usage_logs(user_id,account_id,input_tokens,cache_creation_tokens,cache_read_tokens,output_tokens,account_stats_cost,total_cost,account_rate_multiplier,created_at) VALUES(2,100,10,0,0,2,NULL,4,1,$1),(2,101,5,0,0,1,3,99,2,$1)`, month.Add(time.Hour)); err != nil {
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
	if len(data.Accounts.Items) != 2 || data.Accounts.Items[0].ID != 200 && data.Accounts.Items[1].ID != 200 {
		t.Fatalf("accounts=%+v", data.Accounts.Items)
	}
	var root *CostAccountItem
	for i := range data.Accounts.Items {
		if data.Accounts.Items[i].ID == 100 {
			root = &data.Accounts.Items[i]
		}
	}
	if root == nil || root.PlatformCost != "10.00" || root.RequestCount != 2 || root.Tokens.Total != 18 {
		t.Fatalf("root=%+v", root)
	}
	if len(data.Flows) != 1 || data.Flows[0].ContributionDepartment != "研发" || data.Flows[0].UsageDepartment != "运营" || data.Flows[0].PlatformCost != "10.00" {
		t.Fatalf("flows=%+v", data.Flows)
	}
}
