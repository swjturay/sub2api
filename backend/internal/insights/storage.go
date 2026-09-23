package insights

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type DB interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type Store struct{ db DB }

func NewStore(db DB) *Store { return &Store{db: db} }

func (s *Store) StoreCall(ctx context.Context, f CallFact) error {
	if s == nil || s.db == nil {
		return errors.New("insights store database is nil")
	}
	if f.CallID == "" || f.StatisticalAt.IsZero() {
		return errors.New("insights call id and statistical time are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
INSERT INTO insights_call_facts (
 call_id, request_id, user_id, api_key_id, platform, model, transport, outcome, client_request_id,
 error_type, error_summary, gateway_pre_forward_ms, model_duration_ms, first_token_ms,
 output_tokens, attempt_count, statistical_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$17,$9,$10,$11,$12,$13,$14,$15,$16)
ON CONFLICT (call_id) DO NOTHING`,
		f.CallID, nullString(f.RequestID), f.UserID, f.APIKeyID, nullString(f.Platform), nullString(f.Model), f.Transport, f.Outcome,
		nullString(f.ErrorType), nullString(f.ErrorSummary), durationMillis(f.GatewayPreForward), durationMillis(f.ModelDuration), durationMillis(f.FirstToken),
		f.OutputTokens, f.AttemptCount, f.StatisticalAt, nullString(f.ClientRequestID))
	if err != nil {
		return err
	}
	if f.Outcome != OutcomeSuccess {
		_, err = tx.ExecContext(ctx, `
INSERT INTO insights_error_facts (call_id,user_id,platform,model,error_type,error_summary,statistical_at)
VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (call_id) DO NOTHING`,
			f.CallID, f.UserID, nullString(f.Platform), nullString(f.Model), fallbackErrorType(f.ErrorType), nullString(f.ErrorSummary), f.StatisticalAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpsertDailyUsage(ctx context.Context, d DailyUsage) error {
	if s == nil || s.db == nil {
		return errors.New("insights store database is nil")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO insights_user_model_daily (
 stat_date,user_id,platform,model,usage_count,success_count,failure_count,
 input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,
 usage_duration_sum_ms,usage_duration_samples,usage_first_token_sum_ms,usage_first_token_samples,
 usage_tpot_sum_ms,usage_tpot_samples,gateway_model_duration_sum_ms,gateway_model_duration_samples,
 gateway_pre_forward_sum_ms,gateway_pre_forward_samples
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
ON CONFLICT (stat_date,user_id,platform,model) DO UPDATE SET
 usage_count=EXCLUDED.usage_count, success_count=EXCLUDED.success_count, failure_count=EXCLUDED.failure_count,
 input_tokens=EXCLUDED.input_tokens, output_tokens=EXCLUDED.output_tokens,
 cache_creation_tokens=EXCLUDED.cache_creation_tokens, cache_read_tokens=EXCLUDED.cache_read_tokens,
 usage_duration_sum_ms=EXCLUDED.usage_duration_sum_ms, usage_duration_samples=EXCLUDED.usage_duration_samples,
 usage_first_token_sum_ms=EXCLUDED.usage_first_token_sum_ms, usage_first_token_samples=EXCLUDED.usage_first_token_samples,
 usage_tpot_sum_ms=EXCLUDED.usage_tpot_sum_ms, usage_tpot_samples=EXCLUDED.usage_tpot_samples,
 gateway_model_duration_sum_ms=EXCLUDED.gateway_model_duration_sum_ms, gateway_model_duration_samples=EXCLUDED.gateway_model_duration_samples,
 gateway_pre_forward_sum_ms=EXCLUDED.gateway_pre_forward_sum_ms, gateway_pre_forward_samples=EXCLUDED.gateway_pre_forward_samples, updated_at=NOW()`,
		d.Date, d.UserID, d.Platform, d.Model, d.UsageCount, d.SuccessCount, d.FailureCount,
		d.InputTokens, d.OutputTokens, d.CacheCreationTokens, d.CacheReadTokens,
		d.UsageDurationSumMS, d.UsageDurationSamples, d.UsageFirstTokenSumMS, d.UsageFirstTokenSamples,
		d.UsageTPOTSumMS, d.UsageTPOTSamples, d.GatewayModelDurationSumMS, d.GatewayModelDurationSamples,
		d.GatewayPreForwardSumMS, d.GatewayPreForwardSamples)
	return err
}

// DeleteExpiredDetails processes one bounded batch and freezes the complete
// source day in the same transaction as deletion. Repeated runs cannot erase
// daily facts whose raw source is already gone.
func (s *Store) DeleteExpiredDetails(ctx context.Context, now time.Time, timezone string) error {
	if s == nil || s.db == nil {
		return errors.New("insights store database is nil")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return err
	}
	cutoff := DayAt(now.In(loc).AddDate(0, 0, -365), loc)
	keepFrom := time.Date(now.In(loc).Year()-2, time.January, 1, 0, 0, 0, 0, loc)
	// Materialize affected user lifetime milestones before raw call deletion.
	readTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	var oldest sql.NullTime
	if err = readTx.QueryRowContext(ctx, `SELECT MIN(statistical_at) FROM insights_call_facts WHERE statistical_at<$1`, cutoff).Scan(&oldest); err != nil {
		_ = readTx.Rollback()
		return err
	}
	var sourceCoverage Coverage
	var raw []byte
	if readTx.QueryRowContext(ctx, `SELECT value->'call_facts' FROM insights_settings WHERE key='coverage'`).Scan(&raw) == nil {
		_ = json.Unmarshal(raw, &sourceCoverage)
	}
	var ids []int64
	if oldest.Valid {
		rows, e := readTx.QueryContext(ctx, `SELECT DISTINCT user_id FROM (SELECT user_id FROM insights_call_facts WHERE statistical_at<$1 ORDER BY statistical_at,call_id LIMIT 10000) f WHERE user_id IS NOT NULL`, cutoff)
		if e != nil {
			_ = readTx.Rollback()
			return e
		}
		for rows.Next() {
			var id int64
			if e = rows.Scan(&id); e != nil {
				_ = rows.Close()
				_ = readTx.Rollback()
				return e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		_ = rows.Close()
		if e != nil {
			_ = readTx.Rollback()
			return e
		}
	}
	if err = readTx.Commit(); err != nil {
		return err
	}
	if err = s.RebuildLifecycleUsers(ctx, timezone, sourceCoverage, ids); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('insights_daily_rollup_v1'))`); err != nil {
		return err
	}
	if oldest.Valid {
		from := DayAt(oldest.Time, loc)
		to := from.AddDate(0, 0, 1)
		if !from.Before(keepFrom) {
			if _, err = tx.ExecContext(ctx, callZeroSQL, from, to); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, callRollupSQL, from, to, timezone); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, rollupCoverageSQL, "call", from, to, timezone); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO insights_settings(key,value,updated_at) VALUES('rollup_watermarks',jsonb_build_object('call_frozen_before',$1::date::text),NOW()) ON CONFLICT(key) DO UPDATE SET value=insights_settings.value||jsonb_build_object('call_frozen_before',GREATEST(COALESCE((insights_settings.value->>'call_frozen_before')::date,'0001-01-01'),$1::date)::text),updated_at=NOW()`, to.Format("2006-01-02")); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM insights_call_facts WHERE call_id IN (SELECT call_id FROM insights_call_facts WHERE statistical_at<$1 ORDER BY statistical_at,call_id LIMIT 10000) AND statistical_at<$2`, to, cutoff); err != nil {
			return err
		}
	}
	for _, q := range []string{`DELETE FROM insights_error_facts WHERE statistical_at < $1`, `DELETE FROM insights_user_model_daily WHERE stat_date < $1`, `DELETE FROM insights_rollup_coverage WHERE stat_date < $1`, `DELETE FROM insights_usage_fact_archive WHERE created_at < $1`} {
		bound := keepFrom
		if q == `DELETE FROM insights_error_facts WHERE statistical_at < $1` {
			bound = now.AddDate(0, 0, -30)
		}
		if _, err = tx.ExecContext(ctx, q, bound); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ExtendTrustedUsageHistory is an explicit operator assertion that the existing
// billing usage ledger is complete from the supplied local calendar date. It
// only extends a healthy usage interval and never crosses a recorded gap.
func (s *Store) ExtendTrustedUsageHistory(ctx context.Context, coverage Coverage, since time.Time, timezone string) (Coverage, error) {
	if s == nil || s.db == nil {
		return coverage, errors.New("insights store database is nil")
	}
	if coverage.Status != CoverageComplete || coverage.TrustedSince == nil {
		return coverage, errors.New("live usage collection must be trusted before historical usage can be certified")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return coverage, err
	}
	since = DayAt(since.In(loc), loc)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return coverage, err
	}
	defer func() { _ = tx.Rollback() }()
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT value FROM insights_settings WHERE key='coverage' FOR UPDATE`).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return coverage, err
	}
	stored := map[string]json.RawMessage{}
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &stored); err != nil {
			return coverage, err
		}
	}
	if storedUsage, ok := stored["usage"]; ok {
		var current Coverage
		if err = json.Unmarshal(storedUsage, &current); err != nil {
			return coverage, err
		}
		coverage = current
	}
	if coverage.Status != CoverageComplete || coverage.TrustedSince == nil {
		return coverage, errors.New("stored usage coverage is not currently trusted")
	}
	if !since.Before(*coverage.TrustedSince) {
		return coverage, nil
	}
	if coverage.LastGapAt != nil && !coverage.LastGapAt.Before(since) {
		return coverage, errors.New("historical usage start would cross a recorded collection gap")
	}
	coverage.TrustedSince = timePtr(since)
	encoded, err := json.Marshal(coverage)
	if err != nil {
		return coverage, err
	}
	stored["usage"] = encoded
	payload, err := json.Marshal(stored)
	if err != nil {
		return coverage, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO insights_settings(key,value,updated_at) VALUES('coverage',$1::jsonb,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW()`, payload); err != nil {
		return coverage, err
	}
	today := DayAt(time.Now().In(loc), loc)
	if since.Before(today) {
		if _, err = tx.ExecContext(ctx, `INSERT INTO insights_rollup_coverage(source,stat_date,complete,updated_at) SELECT 'usage',d::date,true,NOW() FROM generate_series($1::date,$2::date-1,interval '1 day') d ON CONFLICT(source,stat_date) DO UPDATE SET complete=true,updated_at=NOW()`, since, today); err != nil {
			return coverage, err
		}
	}
	if err = tx.Commit(); err != nil {
		return coverage, err
	}
	return coverage, nil
}

// SaveCoverage serializes fleet writers. A healthy replica must not overwrite
// another replica's reported gap or move a new trusted boundary backwards.
func (s *Store) SaveCoverage(ctx context.Context, coverage map[string]Coverage) error {
	if s == nil || s.db == nil {
		return errors.New("insights store database is nil")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT value FROM insights_settings WHERE key='coverage' FOR UPDATE`).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	stored := map[string]json.RawMessage{}
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &stored); err != nil {
			return err
		}
	}
	for dataset, incoming := range coverage {
		var previous Coverage
		if json.Unmarshal(stored[dataset], &previous) == nil {
			incoming = mergeCoverage(previous, incoming)
		}
		encoded, e := json.Marshal(incoming)
		if e != nil {
			return e
		}
		stored[dataset] = encoded
	}
	payload, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO insights_settings(key,value,updated_at) VALUES('coverage',$1::jsonb,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW()`, payload); err != nil {
		return err
	}
	return tx.Commit()
}

func mergeCoverage(previous, incoming Coverage) Coverage {
	if previous.Status == CoverageComplete && incoming.Status == CoverageComplete && previous.TrustedSince != nil {
		// A complete fleet interval keeps its persisted boundary. This protects an
		// explicit historical certification from stale healthy replicas. A real
		// collection gap first makes the persisted state partial, after which a
		// later activation may establish a new boundary.
		incoming.TrustedSince = previous.TrustedSince
	} else if previous.TrustedSince != nil && (incoming.TrustedSince == nil || incoming.TrustedSince.Before(*previous.TrustedSince)) {
		incoming.TrustedSince = previous.TrustedSince
	}
	if previous.LastGapAt != nil && (incoming.TrustedSince == nil || !incoming.TrustedSince.After(*previous.LastGapAt)) && incoming.Status == CoverageComplete {
		incoming.Status = CoveragePartial
		incoming.Reason = previous.Reason
	}
	if previous.LastGapAt != nil && (incoming.LastGapAt == nil || incoming.LastGapAt.Before(*previous.LastGapAt)) {
		incoming.LastGapAt = previous.LastGapAt
	}
	if previous.ObservedThrough != nil && (incoming.ObservedThrough == nil || incoming.ObservedThrough.Before(*previous.ObservedThrough)) {
		incoming.ObservedThrough = previous.ObservedThrough
	}
	return incoming
}
func (s *Store) SaveAggregationConfig(ctx context.Context, cfg AggregationConfig) error {
	if s == nil || s.db == nil {
		return errors.New("insights store database is nil")
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO insights_settings (key,value,updated_at) VALUES ('aggregation_config',$1::jsonb,NOW())
ON CONFLICT (key) DO UPDATE SET value=insights_settings.value || EXCLUDED.value, updated_at=EXCLUDED.updated_at`, payload)
	return err
}

func durationMillis(d *time.Duration) any {
	if d == nil {
		return nil
	}
	return d.Milliseconds()
}
func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func fallbackErrorType(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
}
