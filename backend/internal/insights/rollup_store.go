package insights

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// RollupRange rebuilds whole local calendar days. Usage and call columns are
// maintained independently, and a source is never rebuilt below its frozen
// watermark after raw detail has been deleted.
func (s *Store) RollupRange(ctx context.Context, from, to time.Time, timezone string) error {
	if s == nil || s.db == nil {
		return errors.New("insights store database is nil")
	}
	if !from.Before(to) {
		return errors.New("insights rollup range is empty")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return fmt.Errorf("invalid insights timezone: %w", err)
	}
	fromDay := DayAt(from, loc)
	last := to.Add(-time.Nanosecond)
	toDay := DayAt(last, loc).AddDate(0, 0, 1)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('insights_daily_rollup_v1'))`); err != nil {
		return err
	}
	// Each statement clamps its range to its own source watermark.
	if _, err = tx.ExecContext(ctx, usageZeroSQL, fromDay, toDay); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, usageRollupSQL, fromDay, toDay, timezone); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, rollupCoverageSQL, "usage", fromDay, toDay); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, callZeroSQL, fromDay, toDay); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, callRollupSQL, fromDay, toDay, timezone); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, rollupCoverageSQL, "call", fromDay, toDay); err != nil {
		return err
	}
	return tx.Commit()
}

const usageZeroSQL = `WITH b AS (SELECT GREATEST($1::date,COALESCE((SELECT (value->>'usage_frozen_before')::date FROM insights_settings WHERE key='rollup_watermarks'),'0001-01-01'::date)) d0,$2::date d1) UPDATE insights_user_model_daily d SET usage_count=0,input_tokens=0,output_tokens=0,cache_creation_tokens=0,cache_read_tokens=0,usage_duration_sum_ms=0,usage_duration_samples=0,usage_first_token_sum_ms=0,usage_first_token_samples=0,usage_tpot_sum_ms=0,usage_tpot_samples=0,updated_at=NOW() FROM b WHERE d.stat_date>=b.d0 AND d.stat_date<b.d1`
const callZeroSQL = `WITH b AS (SELECT GREATEST($1::date,COALESCE((SELECT (value->>'call_frozen_before')::date FROM insights_settings WHERE key='rollup_watermarks'),'0001-01-01'::date)) d0,$2::date d1) UPDATE insights_user_model_daily d SET success_count=0,failure_count=0,gateway_model_duration_sum_ms=0,gateway_model_duration_samples=0,gateway_pre_forward_sum_ms=0,gateway_pre_forward_samples=0,updated_at=NOW() FROM b WHERE d.stat_date>=b.d0 AND d.stat_date<b.d1`

const rollupCoverageSQL = `
WITH b AS (
 SELECT GREATEST($2::date,COALESCE((SELECT (value->>($1::text||'_frozen_before'))::date FROM insights_settings WHERE key='rollup_watermarks'),'0001-01-01'::date)) d0,$3::date d1
)
INSERT INTO insights_rollup_coverage(source,stat_date,complete,updated_at)
SELECT $1::text,d::date,true,NOW()
FROM b,LATERAL generate_series(b.d0,b.d1-1,interval '1 day') d
ON CONFLICT(source,stat_date) DO UPDATE SET complete=EXCLUDED.complete,updated_at=NOW()`

var usageRollupSQL = `
WITH b AS (SELECT GREATEST($1::date,COALESCE((SELECT (value->>'usage_frozen_before')::date FROM insights_settings WHERE key='rollup_watermarks'),'0001-01-01'::date)) d0,$2::date d1),
a AS (SELECT (ul.created_at AT TIME ZONE $3)::date stat_date,ul.user_id,COALESCE(NULLIF(BTRIM(ul.archived_platform),''),` + usageProviderSQL("cf.platform", "ac.platform") + `) platform,COALESCE(NULLIF(BTRIM(ul.requested_model),''),NULLIF(BTRIM(ul.model),''),'unknown') model,COUNT(*)::bigint usage_count,SUM(ul.input_tokens)::bigint input_tokens,SUM(ul.output_tokens)::bigint output_tokens,SUM(ul.cache_creation_tokens)::bigint cache_creation_tokens,SUM(ul.cache_read_tokens)::bigint cache_read_tokens,COALESCE(SUM(ul.duration_ms) FILTER(WHERE ul.duration_ms>=0),0)::bigint duration_sum,COUNT(*) FILTER(WHERE ul.duration_ms>=0)::bigint duration_n,COALESCE(SUM(ul.first_token_ms) FILTER(WHERE ul.first_token_ms>=0),0)::bigint ttft_sum,COUNT(*) FILTER(WHERE ul.first_token_ms>=0)::bigint ttft_n,COALESCE(SUM((ul.duration_ms-ul.first_token_ms)::double precision/(ul.output_tokens-1)) FILTER(WHERE ul.stream AND ul.output_tokens>1 AND ul.duration_ms>ul.first_token_ms AND ul.first_token_ms>=0),0) tpot_sum,COUNT(*) FILTER(WHERE ul.stream AND ul.output_tokens>1 AND ul.duration_ms>ul.first_token_ms AND ul.first_token_ms>=0)::bigint tpot_n FROM (SELECT id,user_id,account_id,NULL::text archived_platform,model,requested_model,input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,duration_ms,first_token_ms,stream,created_at,api_key_id,request_id FROM usage_logs UNION ALL SELECT usage_id,user_id,account_id,platform,model,requested_model,input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,duration_ms,first_token_ms,stream,created_at,NULL::bigint,NULL::text FROM insights_usage_fact_archive) ul LEFT JOIN accounts ac ON ac.id=ul.account_id LEFT JOIN LATERAL (SELECT MIN(f.platform) platform FROM insights_call_facts f WHERE f.request_id=ul.request_id AND f.user_id=ul.user_id AND f.api_key_id=ul.api_key_id AND f.model=COALESCE(NULLIF(BTRIM(ul.requested_model),''),ul.model) AND f.platform IS NOT NULL HAVING COUNT(DISTINCT f.call_id)=1) cf ON TRUE,b WHERE ul.created_at >= b.d0::timestamp AT TIME ZONE $3 AND ul.created_at < b.d1::timestamp AT TIME ZONE $3 GROUP BY 1,2,3,4)
INSERT INTO insights_user_model_daily(stat_date,user_id,platform,model,usage_count,input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,usage_duration_sum_ms,usage_duration_samples,usage_first_token_sum_ms,usage_first_token_samples,usage_tpot_sum_ms,usage_tpot_samples) SELECT * FROM a ON CONFLICT(stat_date,user_id,platform,model) DO UPDATE SET usage_count=EXCLUDED.usage_count,input_tokens=EXCLUDED.input_tokens,output_tokens=EXCLUDED.output_tokens,cache_creation_tokens=EXCLUDED.cache_creation_tokens,cache_read_tokens=EXCLUDED.cache_read_tokens,usage_duration_sum_ms=EXCLUDED.usage_duration_sum_ms,usage_duration_samples=EXCLUDED.usage_duration_samples,usage_first_token_sum_ms=EXCLUDED.usage_first_token_sum_ms,usage_first_token_samples=EXCLUDED.usage_first_token_samples,usage_tpot_sum_ms=EXCLUDED.usage_tpot_sum_ms,usage_tpot_samples=EXCLUDED.usage_tpot_samples,updated_at=NOW()`

const callRollupSQL = `
WITH b AS (SELECT GREATEST($1::date,COALESCE((SELECT (value->>'call_frozen_before')::date FROM insights_settings WHERE key='rollup_watermarks'),'0001-01-01'::date)) d0,$2::date d1),
a AS (SELECT (f.statistical_at AT TIME ZONE $3)::date stat_date,COALESCE(f.user_id,0),COALESCE(NULLIF(BTRIM(f.platform),''),'unknown'),COALESCE(NULLIF(BTRIM(f.model),''),'unknown'),COUNT(*) FILTER(WHERE outcome=1)::bigint,COUNT(*) FILTER(WHERE outcome<>1)::bigint,COALESCE(SUM(model_duration_ms) FILTER(WHERE outcome=1),0)::bigint,COUNT(model_duration_ms) FILTER(WHERE outcome=1)::bigint,COALESCE(SUM(gateway_pre_forward_ms) FILTER(WHERE outcome=1),0)::bigint,COUNT(gateway_pre_forward_ms) FILTER(WHERE outcome=1)::bigint FROM insights_call_facts f,b WHERE f.statistical_at >= b.d0::timestamp AT TIME ZONE $3 AND f.statistical_at < b.d1::timestamp AT TIME ZONE $3 GROUP BY 1,2,3,4)
INSERT INTO insights_user_model_daily(stat_date,user_id,platform,model,success_count,failure_count,gateway_model_duration_sum_ms,gateway_model_duration_samples,gateway_pre_forward_sum_ms,gateway_pre_forward_samples) SELECT * FROM a ON CONFLICT(stat_date,user_id,platform,model) DO UPDATE SET success_count=EXCLUDED.success_count,failure_count=EXCLUDED.failure_count,gateway_model_duration_sum_ms=EXCLUDED.gateway_model_duration_sum_ms,gateway_model_duration_samples=EXCLUDED.gateway_model_duration_samples,gateway_pre_forward_sum_ms=EXCLUDED.gateway_pre_forward_sum_ms,gateway_pre_forward_samples=EXCLUDED.gateway_pre_forward_samples,updated_at=NOW()`

func (s *Store) FreezeSourceBefore(ctx context.Context, source string, before time.Time, timezone string) error {
	if source != "usage" && source != "call" {
		return errors.New("invalid rollup source")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return err
	}
	day := DayAt(before, loc)
	key := source + "_frozen_before"
	_, err = s.db.ExecContext(ctx, `INSERT INTO insights_settings(key,value) VALUES('rollup_watermarks',jsonb_build_object($1::text,$2::date::text)) ON CONFLICT(key) DO UPDATE SET value=insights_settings.value||jsonb_build_object($1::text,GREATEST(COALESCE((insights_settings.value->>($1::text))::date,'0001-01-01'),$2::date)::text),updated_at=NOW()`, key, day)
	return err
}
