-- Monthly administrator-maintained contribution configuration and actual spend.
-- Platform statistical cost remains derived from usage_logs and is never copied here.
CREATE TABLE IF NOT EXISTS insights_cost_account_months (
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    month DATE NOT NULL,
    registered BOOLEAN NOT NULL DEFAULT TRUE,
    contributor_user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    payment_method VARCHAR(20),
    actual_cost NUMERIC(20,2),
    notes TEXT NOT NULL DEFAULT '',
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, month),
    CONSTRAINT insights_cost_account_months_month_check
        CHECK (month >= DATE '2026-06-01' AND month = date_trunc('month', month)::date),
    CONSTRAINT insights_cost_account_months_actual_cost_check
        CHECK (actual_cost IS NULL OR actual_cost >= 0),
    CONSTRAINT insights_cost_account_months_registration_check
        CHECK (
            (registered = TRUE AND contributor_user_id IS NOT NULL AND payment_method IN ('subscription', 'payg', 'other'))
            OR
            (registered = FALSE AND contributor_user_id IS NULL AND payment_method IS NULL AND actual_cost IS NULL)
        )
);

CREATE INDEX IF NOT EXISTS idx_insights_cost_account_months_month
    ON insights_cost_account_months (month);

CREATE INDEX IF NOT EXISTS idx_insights_cost_account_months_contributor_month
    ON insights_cost_account_months (contributor_user_id, month)
    WHERE registered = TRUE;

COMMENT ON TABLE insights_cost_account_months IS
    'Monthly cost-data events. The latest row at or before a month supplies registration, contributor, and payment method; actual_cost is exact-month only.';
COMMENT ON COLUMN insights_cost_account_months.actual_cost IS
    'Administrator-confirmed actual USD spend. NULL means not entered; 0.00 means confirmed zero spend.';
