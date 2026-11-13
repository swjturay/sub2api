-- Monthly account cost rollup used by the department trend chart. The
-- maintenance worker refreshes historical months in one transaction so the
-- trend query does not repeatedly aggregate raw usage_logs.
CREATE TABLE IF NOT EXISTS insights_cost_monthly_usage (
    month DATE NOT NULL,
    account_id BIGINT NOT NULL,
    request_count BIGINT NOT NULL DEFAULT 0,
    tokens BIGINT NOT NULL DEFAULT 0,
    platform_cost NUMERIC(30,10) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (month, account_id)
);

CREATE INDEX IF NOT EXISTS idx_insights_cost_monthly_usage_account_month
    ON insights_cost_monthly_usage (account_id, month);
