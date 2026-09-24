package insights

import (
	"context"
	"errors"
	"time"

	"github.com/lib/pq"
)

// RebuildLifecycle retains lifetime milestones even after call detail expires.
// First-use anchors are scoped to the fixed statistics start. Accounts that
// predate the system launch use their first in-scope request as the anchor.
func (s *Store) RebuildLifecycle(ctx context.Context, timezone string, statisticsStart *time.Time) error {
	return s.rebuildLifecycle(ctx, timezone, statisticsStart, nil)
}

// RebuildLifecycleUsers bounds routine maintenance to a batch of users. A full
// rebuild is reserved for controlled backfills and tests.
func (s *Store) RebuildLifecycleUsers(ctx context.Context, timezone string, statisticsStart *time.Time, users []int64) error {
	if len(users) == 0 {
		return nil
	}
	return s.rebuildLifecycle(ctx, timezone, statisticsStart, users)
}
func (s *Store) rebuildLifecycle(ctx context.Context, timezone string, statisticsStart *time.Time, users []int64) error {
	if s == nil || s.db == nil {
		return errors.New("insights store database is nil")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('sub2api_insights_lifecycle'))`); err != nil {
		return err
	}
	var since any
	if statisticsStart != nil {
		since = *statisticsStart
	}
	_, err = tx.ExecContext(ctx, `
WITH raw_first AS (
 SELECT user_id,MIN(statistical_at) at FROM insights_call_facts WHERE user_id IS NOT NULL AND ($2::timestamptz IS NULL OR statistical_at >= $2) AND ($3::bigint[] IS NULL OR user_id=ANY($3)) GROUP BY user_id
), observed_evidence AS (
 SELECT user_id,statistical_at at FROM insights_call_facts WHERE user_id IS NOT NULL AND ($3::bigint[] IS NULL OR user_id=ANY($3))
 UNION ALL SELECT user_id,created_at FROM usage_logs WHERE user_id IS NOT NULL AND ($3::bigint[] IS NULL OR user_id=ANY($3))
 UNION ALL SELECT user_id,stat_date::timestamp AT TIME ZONE $1 FROM insights_user_model_daily WHERE user_id>0 AND usage_count+success_count+failure_count>0 AND ($3::bigint[] IS NULL OR user_id=ANY($3))
 UNION ALL SELECT user_id,first_observed_call_at FROM insights_user_lifecycle WHERE first_observed_call_at IS NOT NULL AND ($3::bigint[] IS NULL OR user_id=ANY($3))
 UNION ALL SELECT user_id,first_call_at FROM insights_user_lifecycle WHERE first_call_at IS NOT NULL AND ($3::bigint[] IS NULL OR user_id=ANY($3))
 UNION ALL SELECT user_id,returned_day_1_at FROM insights_user_lifecycle WHERE returned_day_1_at IS NOT NULL AND ($3::bigint[] IS NULL OR user_id=ANY($3))
 UNION ALL SELECT user_id,returned_day_7_at FROM insights_user_lifecycle WHERE returned_day_7_at IS NOT NULL AND ($3::bigint[] IS NULL OR user_id=ANY($3))
 UNION ALL SELECT user_id,returned_day_30_at FROM insights_user_lifecycle WHERE returned_day_30_at IS NOT NULL AND ($3::bigint[] IS NULL OR user_id=ANY($3))
), observed AS (SELECT user_id,MIN(at) at FROM observed_evidence GROUP BY user_id), eligible AS (
 SELECT u.id,u.created_at,o.at observed_at,
   COALESCE(l.first_call_coverage_complete,false) OR $2::timestamptz IS NOT NULL first_complete,
   CASE WHEN COALESCE(l.first_call_coverage_complete,false) OR $2::timestamptz IS NOT NULL
        THEN LEAST(CASE WHEN $2::timestamptz IS NULL OR l.first_call_at >= $2 THEN l.first_call_at END,f.at) END first_at,
   l.first_call_at previous_first,l.returned_day_1_at r1,l.returned_day_7_at r7,l.returned_day_30_at r30,
   COALESCE(l.return_coverage_complete,false) OR $2::timestamptz IS NOT NULL return_complete
 FROM users u LEFT JOIN insights_user_lifecycle l ON l.user_id=u.id LEFT JOIN raw_first f ON f.user_id=u.id LEFT JOIN observed o ON o.user_id=u.id
 WHERE u.deleted_at IS NULL AND ($3::bigint[] IS NULL OR u.id=ANY($3))
), events AS (
 SELECT f.user_id,f.statistical_at at FROM insights_call_facts f JOIN eligible e ON e.id=f.user_id AND e.first_at IS NOT NULL
 UNION ALL SELECT id,previous_first FROM eligible WHERE previous_first IS NOT NULL
 UNION ALL SELECT id,r1 FROM eligible WHERE r1 IS NOT NULL
 UNION ALL SELECT id,r7 FROM eligible WHERE r7 IS NOT NULL
 UNION ALL SELECT id,r30 FROM eligible WHERE r30 IS NOT NULL
), materialized AS (
 SELECT e.id,e.observed_at,e.first_at,e.first_complete,e.return_complete,e.r1,e.r7,e.r30,
 MIN(v.at) FILTER(WHERE (v.at AT TIME ZONE $1)::date-(e.first_at AT TIME ZONE $1)::date>=1) calculated_day1,
 MIN(v.at) FILTER(WHERE (v.at AT TIME ZONE $1)::date-(e.first_at AT TIME ZONE $1)::date>=7) calculated_day7,
 MIN(v.at) FILTER(WHERE (v.at AT TIME ZONE $1)::date-(e.first_at AT TIME ZONE $1)::date>=30) calculated_day30
 FROM eligible e LEFT JOIN events v ON v.user_id=e.id
 GROUP BY e.id,e.observed_at,e.first_at,e.first_complete,e.return_complete,e.r1,e.r7,e.r30
)
INSERT INTO insights_user_lifecycle(user_id,first_observed_call_at,first_call_date,first_call_at,returned_day_1_at,returned_day_7_at,returned_day_30_at,first_call_coverage_complete,return_coverage_complete,updated_at)
SELECT id,observed_at,(first_at AT TIME ZONE $1)::date,first_at,LEAST(r1,calculated_day1),LEAST(r7,calculated_day7),LEAST(r30,calculated_day30),first_complete,return_complete,NOW() FROM materialized
ON CONFLICT(user_id) DO UPDATE SET
 first_observed_call_at=LEAST(insights_user_lifecycle.first_observed_call_at,EXCLUDED.first_observed_call_at),
 first_call_date=(LEAST(insights_user_lifecycle.first_call_at,EXCLUDED.first_call_at) AT TIME ZONE $1)::date,
 first_call_at=LEAST(insights_user_lifecycle.first_call_at,EXCLUDED.first_call_at),
 returned_day_1_at=LEAST(insights_user_lifecycle.returned_day_1_at,EXCLUDED.returned_day_1_at),
 returned_day_7_at=LEAST(insights_user_lifecycle.returned_day_7_at,EXCLUDED.returned_day_7_at),
 returned_day_30_at=LEAST(insights_user_lifecycle.returned_day_30_at,EXCLUDED.returned_day_30_at),
 first_call_coverage_complete=insights_user_lifecycle.first_call_coverage_complete OR EXCLUDED.first_call_coverage_complete,
 return_coverage_complete=insights_user_lifecycle.return_coverage_complete OR EXCLUDED.return_coverage_complete,updated_at=NOW()`, timezone, since, pq.Array(users))
	if err != nil {
		return err
	}
	var unknown int64
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u LEFT JOIN insights_user_lifecycle l ON l.user_id=u.id WHERE u.deleted_at IS NULL AND (NOT COALESCE(l.first_call_coverage_complete,false) OR NOT COALESCE(l.return_coverage_complete,false))`).Scan(&unknown); err != nil {
		return err
	}
	status := "complete"
	if unknown > 0 {
		status = "partial"
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO insights_settings(key,value,updated_at) VALUES('coverage',jsonb_build_object('lifecycle',jsonb_build_object('status',$1::text)),NOW()) ON CONFLICT(key) DO UPDATE SET value=insights_settings.value || EXCLUDED.value,updated_at=NOW()`, status)
	if err != nil {
		return err
	}
	return tx.Commit()
}
