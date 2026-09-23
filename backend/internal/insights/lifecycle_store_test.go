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

func TestLifecyclePostgresRetainsMilestonesAndLateFirst(t *testing.T) {
	dsn := os.Getenv("INSIGHTS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("INSIGHTS_TEST_DATABASE_URL is not set")
	}
	db, e := sql.Open("postgres", dsn)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	db.SetMaxOpenConns(1)
	schema := fmt.Sprintf("insights_lifecycle_%d", time.Now().UnixNano())
	if _, e = db.Exec(`CREATE SCHEMA ` + schema); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})
	if _, e = db.Exec(`SET search_path TO ` + schema); e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{`CREATE TABLE users(id bigint primary key,created_at timestamptz not null,deleted_at timestamptz)`, `CREATE TABLE insights_call_facts(user_id bigint,statistical_at timestamptz)`, `CREATE TABLE usage_logs(user_id bigint,created_at timestamptz)`, `CREATE TABLE insights_user_model_daily(stat_date date,user_id bigint,usage_count bigint default 0,success_count bigint default 0,failure_count bigint default 0)`, `CREATE TABLE insights_settings(key text primary key,value jsonb,updated_at timestamptz default now())`, `CREATE TABLE user_attribute_values(user_id bigint,attribute_id bigint,value text)`, `CREATE TABLE insights_user_lifecycle(user_id bigint primary key,first_observed_call_at timestamptz,first_call_date date,first_call_at timestamptz,returned_day_1_at timestamptz,returned_day_7_at timestamptz,returned_day_30_at timestamptz,first_call_coverage_complete boolean not null default false,return_coverage_complete boolean not null default false,updated_at timestamptz default now())`} {
		if _, e = db.Exec(s); e != nil {
			t.Fatal(e)
		}
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	anchor := since.AddDate(0, 0, 2)
	ret := anchor.AddDate(0, 0, 31)
	ctx := context.Background()
	store := NewStore(db)
	coverage := Coverage{Status: CoverageComplete, TrustedSince: &since}
	if _, e = db.Exec(`WITH inserted AS (INSERT INTO users VALUES(1,$1,NULL),(2,$2,NULL),(3,$1,NULL)) INSERT INTO insights_call_facts VALUES(1,$3),(1,$4),(2,$3)`, since, since.AddDate(0, 0, -2), anchor, ret); e != nil {
		t.Fatal(e)
	}
	if e = store.RebuildLifecycle(ctx, "Asia/Shanghai", coverage); e != nil {
		t.Fatal(e)
	}
	var first, d1, d7, d30 time.Time
	var trusted bool
	if e = db.QueryRow(`SELECT first_call_at,returned_day_1_at,returned_day_7_at,returned_day_30_at,first_call_coverage_complete FROM insights_user_lifecycle WHERE user_id=1`).Scan(&first, &d1, &d7, &d30, &trusted); e != nil {
		t.Fatal(e)
	}
	if !first.Equal(anchor) || !d1.Equal(ret) || !d7.Equal(ret) || !d30.Equal(ret) || !trusted {
		t.Fatalf("incorrect nested return: %v %v %v %v %v", first, d1, d7, d30, trusted)
	}
	var unknown sql.NullTime
	var observed time.Time
	if e = db.QueryRow(`SELECT first_observed_call_at,first_call_at FROM insights_user_lifecycle WHERE user_id=2`).Scan(&observed, &unknown); e != nil || unknown.Valid || !observed.Equal(anchor) {
		t.Fatalf("old account evidence/anchor wrong: observed=%v first=%v err=%v", observed, unknown, e)
	}
	earlier := since.AddDate(0, 0, 1)
	if _, e = db.Exec(`INSERT INTO insights_call_facts VALUES(1,$1)`, earlier); e != nil {
		t.Fatal(e)
	}
	if e = store.RebuildLifecycle(ctx, "Asia/Shanghai", coverage); e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow(`SELECT first_call_at,returned_day_1_at FROM insights_user_lifecycle WHERE user_id=1`).Scan(&first, &d1); e != nil || !first.Equal(earlier) || !d1.Equal(anchor) {
		t.Fatalf("late anchor not reconciled: %v %v %v", first, d1, e)
	}
	if _, e = db.Exec(`DELETE FROM insights_call_facts`); e != nil {
		t.Fatal(e)
	}
	if e = store.RebuildLifecycle(ctx, "Asia/Shanghai", coverage); e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow(`SELECT first_call_at,returned_day_1_at,returned_day_7_at,returned_day_30_at FROM insights_user_lifecycle WHERE user_id=1`).Scan(&first, &d1, &d7, &d30); e != nil || !first.Equal(earlier) || !d1.Equal(anchor) || !d7.Equal(ret) || !d30.Equal(ret) {
		t.Fatalf("expired raw destroyed milestones: %v %v %v %v %v", first, d1, d7, d30, e)
	}
	if e = db.QueryRow(`SELECT first_observed_call_at FROM insights_user_lifecycle WHERE user_id=2`).Scan(&observed); e != nil || !observed.Equal(anchor) {
		t.Fatalf("observed evidence lost after raw cleanup: %v %v", observed, e)
	}
	gap := Coverage{Status: CoveragePartial, TrustedSince: &since, LastGapAt: &ret}
	if e = store.RebuildLifecycle(ctx, "Asia/Shanghai", gap); e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow(`SELECT returned_day_30_at FROM insights_user_lifecycle WHERE user_id=1`).Scan(&d30); e != nil || !d30.Equal(ret) {
		t.Fatalf("coverage gap erased milestone: %v %v", d30, e)
	}
	if e = db.QueryRow(`SELECT first_call_coverage_complete FROM insights_user_lifecycle WHERE user_id=3`).Scan(&trusted); e != nil || !trusted {
		t.Fatalf("covered unused account not classified: %v %v", trusted, e)
	}
	if _, e = db.Exec(`UPDATE users SET deleted_at=NOW() WHERE id=1`); e != nil {
		t.Fatal(e)
	}
	q := NewQuery(db, "Asia/Shanghai")
	q.now = func() time.Time { return ret.AddDate(0, 0, 1) }
	envelope, e := q.GatewayRetention(ctx, ret.AddDate(0, 0, 1))
	if e != nil {
		t.Fatal(e)
	}
	data := envelope.Data.(map[string]any)
	firstLayer := data["first_request"].(RetentionLayer)
	day1Layer := data["next_day"].(RetentionLayer)
	if firstLayer.Count == nil || *firstLayer.Count != 1 || firstLayer.Ratio == nil || firstLayer.Status != "partial" {
		t.Fatalf("observed first layer=%+v", firstLayer)
	}
	if day1Layer.Count != nil || day1Layer.Ratio != nil || day1Layer.Status != "unknown" {
		t.Fatalf("unknown return layer=%+v", day1Layer)
	}
}
