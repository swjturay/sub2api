ALTER TABLE insights_user_lifecycle
    ADD COLUMN IF NOT EXISTS first_observed_call_at TIMESTAMPTZ;

UPDATE insights_user_lifecycle
SET first_observed_call_at = COALESCE(
    first_observed_call_at,
    first_call_at,
    returned_day_1_at,
    returned_day_7_at,
    returned_day_30_at
)
WHERE first_observed_call_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_insights_user_lifecycle_first_observed
    ON insights_user_lifecycle (first_observed_call_at);
