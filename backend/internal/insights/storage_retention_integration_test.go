package insights

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	"os"
	"testing"
	"time"
)

func TestPostgresRepeatedRetentionPreservesDailyAndLifetime(t *testing.T) {
	dsn := os.Getenv("INSIGHTS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("INSIGHTS_TEST_DATABASE_URL is not set")
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
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	schema := fmt.Sprintf("insights_retention_%d", time.Now().UnixNano())
	if _, err = db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})
	if _, err = db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	ddl := []string{`CREATE TABLE users(id BIGINT PRIMARY KEY,created_at TIMESTAMPTZ NOT NULL,deleted_at TIMESTAMPTZ)`, `CREATE TABLE accounts(id BIGINT PRIMARY KEY,platform TEXT)`, `CREATE TABLE usage_logs(id BIGINT PRIMARY KEY,user_id BIGINT,account_id BIGINT,model TEXT,requested_model TEXT,input_tokens INT,output_tokens INT,cache_creation_tokens INT,cache_read_tokens INT,duration_ms INT,first_token_ms INT,stream BOOLEAN,created_at TIMESTAMPTZ,api_key_id BIGINT,request_id TEXT)`}
	for _, q := range ddl {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	migration, err := os.ReadFile("../../migrations/239_insights_v1.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	migration240, err := os.ReadFile("../../migrations/240_insights_observed_activity.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(migration240)); err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, loc)
	old := DayAt(now.AddDate(0, 0, -375), loc)
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, loc)
	returned := now.AddDate(0, 0, -2)
	cov := Coverage{Status: CoverageComplete, TrustedSince: &since, ObservedThrough: &now}
	raw, _ := json.Marshal(map[string]Coverage{"usage": cov, "call_facts": cov})
	if _, err = db.Exec(`UPDATE insights_settings SET value=$1 WHERE key='coverage'`, string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO users VALUES(1,$1,NULL)`, since.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO accounts VALUES(1,'openai')`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO usage_logs VALUES(1,1,1,'m','m',10,20,3,7,1000,100,true,$1,1,'usage')`, old.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	ids := []string{"00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000003"}
	for i, at := range []time.Time{old.Add(time.Hour), returned, now.AddDate(0, 0, -40)} {
		outcome := 1
		if i == 2 {
			outcome = 3
		}
		if _, err = db.Exec(`INSERT INTO insights_call_facts(call_id,user_id,platform,model,outcome,statistical_at) VALUES($1,1,'openai','m',$2,$3)`, ids[i], outcome, at); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`INSERT INTO insights_error_facts(call_id,user_id,platform,model,error_type,statistical_at) VALUES($1,1,'openai','m','upstream_error',$2)`, ids[2], now.AddDate(0, 0, -40)); err != nil {
		t.Fatal(err)
	}
	st := NewStore(db)
	if _, err = db.Exec(`SET TIME ZONE 'UTC'`); err != nil {
		t.Fatal(err)
	}
	// Trust beginning four hours into a Shanghai day must not certify the
	// complete day merely because the database session runs in UTC.
	startsMidday := old.Add(4 * time.Hour)
	through := old.AddDate(0, 0, 2)
	limited, _ := json.Marshal(map[string]Coverage{"usage": {Status: CoverageComplete, TrustedSince: &startsMidday, ObservedThrough: &through}, "call_facts": cov})
	if _, err = db.Exec(`UPDATE insights_settings SET value=$1 WHERE key='coverage'`, string(limited)); err != nil {
		t.Fatal(err)
	}
	if err = st.RollupRange(ctx, old, old.AddDate(0, 0, 1), "Asia/Shanghai"); err != nil {
		t.Fatal(err)
	}
	var dayComplete bool
	if err = db.QueryRow(`SELECT complete FROM insights_rollup_coverage WHERE source='usage' AND stat_date=$1`, old.Format("2006-01-02")).Scan(&dayComplete); err != nil {
		t.Fatal(err)
	}
	if dayComplete {
		t.Fatal("UTC session certified an uncovered Shanghai morning")
	}
	if _, err = db.Exec(`UPDATE insights_settings SET value=$1 WHERE key='coverage'`, string(raw)); err != nil {
		t.Fatal(err)
	}
	if err = st.RollupRange(ctx, old, old.AddDate(0, 0, 1), "Asia/Shanghai"); err != nil {
		t.Fatal(err)
	}
	if err = st.FreezeSourceBefore(ctx, "usage", old.AddDate(0, 0, 1), "Asia/Shanghai"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`DELETE FROM usage_logs`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = st.DeleteExpiredDetails(ctx, now, "Asia/Shanghai"); err != nil {
			t.Fatal(err)
		}
		if err = st.RollupRange(ctx, old, old.AddDate(0, 0, 1), "Asia/Shanghai"); err != nil {
			t.Fatal(err)
		}
	}
	var usage, success, tokens int64
	if err = db.QueryRow(`SELECT usage_count,success_count,input_tokens+output_tokens+cache_creation_tokens+cache_read_tokens FROM insights_user_model_daily WHERE user_id=1 AND stat_date=$1`, old.Format("2006-01-02")).Scan(&usage, &success, &tokens); err != nil {
		t.Fatal(err)
	}
	if usage != 1 || success != 1 || tokens != 40 {
		t.Fatalf("durable day lost after repeated cleanup: usage=%d success=%d tokens=%d", usage, success, tokens)
	}
	var first, d30 time.Time
	if err = db.QueryRow(`SELECT first_call_at,returned_day_30_at FROM insights_user_lifecycle WHERE user_id=1`).Scan(&first, &d30); err != nil {
		t.Fatal(err)
	}
	if !first.Equal(old.Add(time.Hour)) || d30.IsZero() {
		t.Fatalf("lifetime anchor/milestone lost: %v %v", first, d30)
	}
	var errors, calls int
	if err = db.QueryRow(`SELECT COUNT(*) FROM insights_error_facts`).Scan(&errors); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM insights_call_facts`).Scan(&calls); err != nil {
		t.Fatal(err)
	}
	if errors != 0 || calls != 2 {
		t.Fatalf("wrong detail retention: errors=%d calls=%d", errors, calls)
	}
	owner := int64(1)
	fact := CallFact{Identity: Identity{CallID: "00000000-0000-4000-8000-000000000099", UserID: &owner, Platform: "openai", Model: "m", RequestID: "client:retry-alias", ClientRequestID: "retry-alias"}, Outcome: OutcomeError, ErrorType: "upstream_error", StatisticalAt: now}
	if err = st.StoreCall(ctx, fact); err != nil {
		t.Fatal(err)
	}
	if err = st.StoreCall(ctx, fact); err != nil {
		t.Fatal(err)
	}
	var requestID, clientAlias string
	var storedAt, errorAt time.Time
	if err = db.QueryRow(`SELECT f.request_id,f.client_request_id,f.statistical_at,e.statistical_at FROM insights_call_facts f JOIN insights_error_facts e USING(call_id) WHERE f.call_id=$1`, fact.CallID).Scan(&requestID, &clientAlias, &storedAt, &errorAt); err != nil {
		t.Fatal(err)
	}
	if requestID != fact.RequestID || clientAlias != fact.ClientRequestID || !storedAt.Equal(now) || !errorAt.Equal(now) {
		t.Fatalf("identity or error time drift: %s %s %v %v", requestID, clientAlias, storedAt, errorAt)
	}
	if err = st.SaveCoverage(ctx, map[string]Coverage{"call_facts": cov.MarkGap(now, "replica A dropped a fact")}); err != nil {
		t.Fatal(err)
	}
	if err = st.SaveCoverage(ctx, map[string]Coverage{"call_facts": cov.Observe(now.Add(time.Minute))}); err != nil {
		t.Fatal(err)
	}
	var persisted []byte
	if err = db.QueryRow(`SELECT value->'call_facts' FROM insights_settings WHERE key='coverage'`).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	var fleet Coverage
	if err = json.Unmarshal(persisted, &fleet); err != nil {
		t.Fatal(err)
	}
	if fleet.Status != CoveragePartial || fleet.LastGapAt == nil || !fleet.LastGapAt.Equal(now) {
		t.Fatalf("healthy replica erased persisted gap: %+v", fleet)
	}
}
