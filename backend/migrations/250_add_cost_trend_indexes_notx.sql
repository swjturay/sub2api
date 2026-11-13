-- Keep the lightweight department cost trend path selective on large histories.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_created_account_cost_trend
    ON usage_logs (created_at, account_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_attribute_values_attribute_user_cost_trend
    ON user_attribute_values (attribute_id, user_id);

