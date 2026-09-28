-- Contributor identity belongs to the logical account, not an accounting month.
CREATE TABLE IF NOT EXISTS insights_cost_account_contributors (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE RESTRICT,
    contributor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Keep writes from both backend generations atomic during rollout or rollback.
CREATE OR REPLACE FUNCTION insights_sync_cost_account_contributor()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO insights_cost_account_contributors(account_id, contributor_user_id, updated_by, updated_at)
    VALUES (NEW.account_id, NEW.contributor_user_id, NEW.updated_by, NEW.updated_at)
    ON CONFLICT (account_id) DO UPDATE
    SET contributor_user_id = EXCLUDED.contributor_user_id,
        updated_by = EXCLUDED.updated_by,
        updated_at = EXCLUDED.updated_at
    WHERE insights_cost_account_contributors.contributor_user_id IS DISTINCT FROM EXCLUDED.contributor_user_id;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS insights_cost_account_contributor_sync ON insights_cost_account_months;
CREATE TRIGGER insights_cost_account_contributor_sync
AFTER INSERT OR UPDATE OF registered, contributor_user_id ON insights_cost_account_months
FOR EACH ROW WHEN (NEW.registered = TRUE)
EXECUTE FUNCTION insights_sync_cost_account_contributor();

-- Resolve legacy month-level disagreements using the most recently edited row.
INSERT INTO insights_cost_account_contributors(account_id, contributor_user_id, updated_by, updated_at)
SELECT DISTINCT ON (account_id) account_id, contributor_user_id, updated_by, updated_at
FROM insights_cost_account_months
WHERE registered = TRUE AND contributor_user_id IS NOT NULL
ORDER BY account_id, updated_at DESC, month DESC
ON CONFLICT (account_id) DO NOTHING;

COMMENT ON TABLE insights_cost_account_contributors IS
    'Account-wide contributor shared by all months, including historical months and stopped contributions.';
COMMENT ON COLUMN insights_cost_account_months.contributor_user_id IS
    'Legacy monthly snapshot retained for compatibility. Dashboard attribution uses insights_cost_account_contributors.';
COMMENT ON TABLE insights_cost_account_months IS
    'Monthly registration/payment events and exact-month spend. Before the first event, use the earliest registered event. Explicit stop events remain effective until re-registration.';
