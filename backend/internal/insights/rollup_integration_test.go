package insights

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"os"
	"testing"
	"time"
)

func TestRollupRangePostgresLateWriteIntegration(t *testing.T) {
	dsn := os.Getenv("INSIGHTS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("INSIGHTS_TEST_POSTGRES_DSN is not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	ctx := context.Background()
	schema := fmt.Sprintf("insights_rollup_%d", time.Now().UnixNano())
	if _, err = db.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})
	if _, err = db.ExecContext(ctx, `SET search_path TO `+schema); err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`CREATE TABLE accounts(id bigint primary key,platform text)`,
		`CREATE TABLE usage_logs(id bigserial primary key,user_id bigint not null,account_id bigint not null,model text not null,requested_model text,input_tokens int not null default 0,output_tokens int not null default 0,cache_creation_tokens int not null default 0,cache_read_tokens int not null default 0,duration_ms int,first_token_ms int,created_at timestamptz not null,api_key_id bigint,request_id text,stream boolean not null default false)`,
		`CREATE TABLE insights_call_facts(call_id uuid primary key,user_id bigint,platform text,model text,outcome smallint not null,model_duration_ms bigint,gateway_pre_forward_ms bigint,statistical_at timestamptz not null,request_id text,api_key_id bigint)`,
		`CREATE TABLE insights_user_model_daily(stat_date date,user_id bigint,platform text,model text,usage_count bigint default 0,success_count bigint default 0,failure_count bigint default 0,input_tokens bigint default 0,output_tokens bigint default 0,cache_creation_tokens bigint default 0,cache_read_tokens bigint default 0,usage_duration_sum_ms bigint default 0,usage_duration_samples bigint default 0,usage_first_token_sum_ms bigint default 0,usage_first_token_samples bigint default 0,usage_tpot_sum_ms double precision default 0,usage_tpot_samples bigint default 0,gateway_model_duration_sum_ms bigint default 0,gateway_model_duration_samples bigint default 0,gateway_pre_forward_sum_ms bigint default 0,gateway_pre_forward_samples bigint default 0,updated_at timestamptz default now(),primary key(stat_date,user_id,platform,model))`,
		`CREATE TABLE insights_settings(key text primary key,value jsonb not null,updated_at timestamptz default now())`,
		`CREATE TABLE insights_rollup_coverage(source text,stat_date date,complete boolean,updated_at timestamptz default now(),primary key(source,stat_date))`,
		`CREATE TABLE insights_usage_fact_archive(usage_id bigint primary key,user_id bigint,account_id bigint,platform text,model text,requested_model text,input_tokens bigint,output_tokens bigint,cache_creation_tokens bigint,cache_read_tokens bigint,duration_ms int,first_token_ms int,stream boolean,created_at timestamptz)`,
	}
	for _, q := range stmts {
		if _, err = db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	st := NewStore(db)
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_call_facts VALUES('00000000-0000-0000-0000-000000000002',NULL,'openai','m',1,900,100,$1)`, from.Add(20*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = st.RollupRange(ctx, from, to, "UTC"); err != nil {
		t.Fatal(err)
	}
	// An earlier fact arrives after the first rebuild. Rebuilding the same range must include it.
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_call_facts VALUES('00000000-0000-0000-0000-000000000001',NULL,'openai','m',2,NULL,NULL,$1)`, from.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = st.RollupRange(ctx, from, to, "UTC"); err != nil {
		t.Fatal(err)
	}
	var success, failure, sum, samples int64
	if err = db.QueryRowContext(ctx, `SELECT success_count,failure_count,gateway_pre_forward_sum_ms,gateway_pre_forward_samples FROM insights_user_model_daily WHERE user_id=0`).Scan(&success, &failure, &sum, &samples); err != nil {
		t.Fatal(err)
	}
	if success != 1 || failure != 1 || sum != 100 || samples != 1 {
		t.Fatalf("unexpected rollup %d %d %d %d", success, failure, sum, samples)
	}
	// A partial-time request still rebuilds the complete local day.
	_, _ = db.ExecContext(ctx, `INSERT INTO accounts VALUES(1,'openai')`)
	_, err = db.ExecContext(ctx, `INSERT INTO usage_logs(user_id,account_id,model,input_tokens,output_tokens,created_at) VALUES(7,1,'m',3,4,$1)`, from.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err = st.RollupRange(ctx, from.Add(12*time.Hour), from.Add(13*time.Hour), "UTC"); err != nil {
		t.Fatal(err)
	}
	var usage int64
	if err = db.QueryRowContext(ctx, `SELECT usage_count FROM insights_user_model_daily WHERE user_id=7`).Scan(&usage); err != nil || usage != 1 {
		t.Fatalf("whole-day usage=%d err=%v", usage, err)
	}
	// Once usage is frozen and raw detail removed, call-source rebuilds and a
	// second identical maintenance pass must preserve the durable usage totals.
	if err = st.FreezeSourceBefore(ctx, "usage", to, "UTC"); err != nil {
		t.Fatal(err)
	}
	_, _ = db.ExecContext(ctx, `DELETE FROM usage_logs`)
	if err = st.RollupRange(ctx, from, to, "UTC"); err != nil {
		t.Fatal(err)
	}
	if err = st.RollupRange(ctx, from, to, "UTC"); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT usage_count FROM insights_user_model_daily WHERE user_id=7`).Scan(&usage); err != nil || usage != 1 {
		t.Fatalf("frozen usage lost=%d err=%v", usage, err)
	}
}
