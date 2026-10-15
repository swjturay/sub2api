package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Commit quarantine and its scheduler event together. A stale failure cannot
// overwrite reauthorization, rebinding, manual suspension or recovery.
func (r *accountRepository) SetCPRAccountErrorIfMatch(ctx context.Context, id int64, credentials string, updatedAt time.Time, reason string) (bool, error) {
	result, err := r.sql.ExecContext(ctx, `
 WITH updated AS (
  UPDATE accounts SET status = $1, schedulable = false, error_message = $2, updated_at = NOW()
  WHERE id = $3 AND deleted_at IS NULL AND platform = $4 AND type = $5
   AND status = $6 AND schedulable IS TRUE AND credentials = $7::jsonb AND updated_at = $8
  RETURNING id
 )
 INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload)
 SELECT $9, id, NULL, NULL FROM updated
 `, service.StatusError, reason, id, service.PlatformOpenAI, service.AccountTypeCPR,
		service.StatusActive, credentials, updatedAt, service.SchedulerOutboxEventAccountChanged)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return false, err
	}
	r.syncSchedulerAccountSnapshotDetached(ctx, id)
	return true, nil
}
