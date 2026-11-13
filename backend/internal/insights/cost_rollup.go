package insights

import (
	"context"
	"fmt"
	"time"
)

// RefreshCostMonthlyUsage rebuilds the recent account cost rollup. The
// operation is transactionally replaced and guarded by a non-blocking advisory
// lock so multiple backend replicas do not repeat the same refresh.
func (s *Store) RefreshCostMonthlyUsage(ctx context.Context, from, to time.Time, timezone string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("insights store database is nil")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return fmt.Errorf("invalid insights timezone: %w", err)
	}
	from = time.Date(from.In(loc).Year(), from.In(loc).Month(), 1, 0, 0, 0, 0, loc)
	to = time.Date(to.In(loc).Year(), to.In(loc).Month(), 1, 0, 0, 0, 0, loc)
	if !from.Before(to) {
		return nil
	}
	fromDate, toDate := from.Format("2006-01-02"), to.Format("2006-01-02")
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var acquired bool
	if err = tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtext('insights_cost_monthly_usage_v1'))`).Scan(&acquired); err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM insights_cost_monthly_usage WHERE month >= $1::date AND month < $2::date`, fromDate, toDate); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO insights_cost_monthly_usage(month,account_id,request_count,tokens,platform_cost,updated_at)
SELECT date_trunc('month',ul.created_at AT TIME ZONE $3)::date,
       COALESCE(source.parent_account_id,source.id),
       COUNT(*)::bigint,
       COALESCE(SUM(ul.input_tokens),0)+COALESCE(SUM(ul.cache_creation_tokens),0)+COALESCE(SUM(ul.cache_read_tokens),0)+COALESCE(SUM(ul.output_tokens),0),
       COALESCE(SUM(COALESCE(ul.account_stats_cost,ul.total_cost)*COALESCE(ul.account_rate_multiplier,1)),0),
       NOW()
FROM usage_logs ul
JOIN accounts source ON source.id=ul.account_id
WHERE ul.created_at >= $1::date AT TIME ZONE $3
  AND ul.created_at < $2::date AT TIME ZONE $3
		GROUP BY 1,2`, fromDate, toDate, timezone)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func CostRollupRange(now time.Time, timezone string) (time.Time, time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	current := now.In(loc)
	to := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, loc)
	return to.AddDate(0, -12, 0), to.AddDate(0, 1, 0), nil
}
