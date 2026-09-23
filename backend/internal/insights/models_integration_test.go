package insights

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"time"
)

func TestModelStorePostgresIntegration(t *testing.T) {
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
	ctx := context.Background()
	schema := fmt.Sprintf("insights_models_%d", time.Now().UnixNano())
	if _, err = db.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = db.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`) }()
	if _, err = db.ExecContext(ctx, `SET search_path TO `+schema); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`CREATE TABLE accounts(id BIGINT PRIMARY KEY,platform TEXT NOT NULL)`, `CREATE TABLE usage_logs(id BIGSERIAL PRIMARY KEY,request_id TEXT,user_id BIGINT NOT NULL,api_key_id BIGINT NOT NULL,account_id BIGINT NOT NULL,stream BOOLEAN NOT NULL,model TEXT NOT NULL,requested_model TEXT,input_tokens BIGINT NOT NULL,cache_creation_tokens BIGINT NOT NULL,cache_read_tokens BIGINT NOT NULL,output_tokens BIGINT NOT NULL,duration_ms INT,first_token_ms INT,actual_cost NUMERIC NOT NULL DEFAULT 0,created_at TIMESTAMPTZ NOT NULL)`, `CREATE TABLE insights_call_facts(id BIGSERIAL PRIMARY KEY,call_id UUID,request_id TEXT,user_id BIGINT,api_key_id BIGINT,platform TEXT,model TEXT,statistical_at TIMESTAMPTZ NOT NULL)`, `CREATE TABLE insights_model_metadata(platform TEXT NOT NULL,model TEXT NOT NULL,introduction TEXT,use_cases JSONB,capabilities JSONB,source_url TEXT,source_label TEXT,source_updated_at TIMESTAMPTZ,expected_version BIGINT NOT NULL DEFAULT 1,updated_by BIGINT,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(platform,model))`} {
		if _, err = db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC().Truncate(time.Hour)
	if _, err = db.ExecContext(ctx, `INSERT INTO accounts VALUES(1,'OpenAI'),(2,'Anthropic'),(3,'antigravity')`); err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO usage_logs(request_id,user_id,api_key_id,account_id,stream,model,requested_model,input_tokens,cache_creation_tokens,cache_read_tokens,output_tokens,duration_ms,first_token_ms,actual_cost,created_at) VALUES ('a',7,1,1,true,'upstream','alpha:latest',100,20,30,11,1100,100,0,$1),('b',7,1,1,false,'upstream','alpha:latest',0,0,0,3,500,100,0,$1),('c',7,1,1,true,'upstream','alpha:latest',5,0,0,1,900,200,2,$1),('d',7,1,1,true,'upstream','alpha:latest',7,0,0,2,100,NULL,0,$1),('legacy-anthropic',7,1,2,true,'upstream','alpha:latest',60,0,0,0,600,100,0,$1),('legacy-bridge',7,1,3,true,'upstream','alpha:latest',70,0,0,0,700,100,0,$1)`, now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO insights_call_facts(call_id,request_id,user_id,api_key_id,platform,model,statistical_at) VALUES('10000000-0000-0000-0000-000000000001','a',7,1,'OpenAI','alpha:latest',$1),('10000000-0000-0000-0000-000000000002','b',7,1,'OpenAI','alpha:latest',$1),('10000000-0000-0000-0000-000000000003','c',7,1,'OpenAI','alpha:latest',$1),('10000000-0000-0000-0000-000000000004','d',7,1,'OpenAI','alpha:latest',$1),('10000000-0000-0000-0000-000000000005','a',8,1,'Collision','alpha:latest',$1),('10000000-0000-0000-0000-000000000006','a',7,1,'WrongModel','different',$1)`, now)
	if err != nil {
		t.Fatal(err)
	}
	store := NewModelStore(db, "UTC")
	id := ModelIdentity{Platform: "OpenAI", Name: "alpha:latest"}
	all, err := store.Performance(ctx, []ModelIdentity{id}, now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	got := all[NormalizeModelKey(id)]
	if got.AverageTPM == nil || *got.AverageTPM != 179.0/60.0 || got.AverageRPM == nil || *got.AverageRPM != 4.0/60.0 {
		t.Fatalf("rates=%+v", got)
	}
	if got.TTFTSamples != 3 || got.AverageTTFTMS == nil || *got.AverageTTFTMS != 400.0/3.0 {
		t.Fatalf("ttft=%+v", got)
	}
	if got.TPOTSamples != 1 || got.EstimatedTPOTMS == nil || *got.EstimatedTPOTMS != 100 {
		t.Fatalf("tpot=%+v", got)
	}
	anthropicID := ModelIdentity{Platform: "Anthropic", Name: "alpha:latest"}
	crossPlatform, err := store.Performance(ctx, []ModelIdentity{anthropicID}, now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	anthropic := crossPlatform[NormalizeModelKey(anthropicID)]
	if anthropic.RPMSamples != 1 || anthropic.AverageTPM == nil || *anthropic.AverageTPM != 1 {
		t.Fatalf("historical fallback=%+v", anthropic)
	}
	bridgeID := ModelIdentity{Platform: "antigravity", Name: "alpha:latest"}
	bridge, err := store.Performance(ctx, []ModelIdentity{bridgeID}, now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := bridge[NormalizeModelKey(bridgeID)]; ok {
		t.Fatalf("bridge platform must remain unknown: %+v", bridge)
	}
	trend, err := store.Trend(ctx, id, now, now.Add(time.Hour), "hour")
	if err != nil || len(trend) != 1 || trend[0].Performance.RPMSamples != 4 {
		t.Fatalf("trend=%+v err=%v", trend, err)
	}
	partial, err := store.Trend(ctx, id, now.Add(5*time.Minute), now.Add(20*time.Minute), "hour")
	if err != nil || len(partial) != 1 || partial[0].Performance.AverageRPM == nil || *partial[0].Performance.AverageRPM != 4.0/15.0 || partial[0].Complete {
		t.Fatalf("partial trend=%+v err=%v", partial, err)
	}
	description := "profile"
	p, err := store.PutProfile(ctx, id, ModelProfileInput{Description: &description, UseCases: []string{"chat"}, Reasoning: CapabilitySupported, ExpectedVersion: 0}, 99)
	if err != nil || p.Version != 1 {
		t.Fatalf("create=%+v err=%v", p, err)
	}
	_, err = store.PutProfile(ctx, id, ModelProfileInput{ExpectedVersion: 0}, 99)
	if !errors.Is(err, ErrModelProfileConflict) {
		t.Fatalf("stale create=%v", err)
	}
	p, err = store.PutProfile(ctx, id, ModelProfileInput{Description: &description, ExpectedVersion: 1}, 99)
	if err != nil || p.Version != 2 {
		t.Fatalf("update=%+v err=%v", p, err)
	}
	if err = store.DeleteProfile(ctx, id, 1); !errors.Is(err, ErrModelProfileConflict) {
		t.Fatalf("stale delete=%v", err)
	}
	if err = store.DeleteProfile(ctx, id, 2); err != nil {
		t.Fatal(err)
	}
}
