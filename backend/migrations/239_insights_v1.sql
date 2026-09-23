CREATE TABLE IF NOT EXISTS insights_call_facts (
    id BIGSERIAL PRIMARY KEY,
    call_id UUID NOT NULL UNIQUE,
    request_id TEXT,
    client_request_id TEXT,
    user_id BIGINT,
    api_key_id BIGINT,
    platform TEXT,
    model TEXT,
    transport SMALLINT NOT NULL DEFAULT 0,
    outcome SMALLINT NOT NULL,
    error_type VARCHAR(96),
    error_summary VARCHAR(512),
    gateway_pre_forward_ms BIGINT,
    model_duration_ms BIGINT,
    first_token_ms BIGINT,
    output_tokens BIGINT,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    statistical_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT insights_call_facts_transport_check CHECK (transport BETWEEN 0 AND 3),
    CONSTRAINT insights_call_facts_outcome_check CHECK (outcome BETWEEN 1 AND 6),
    CONSTRAINT insights_call_facts_nonnegative_check CHECK (
        (gateway_pre_forward_ms IS NULL OR gateway_pre_forward_ms >= 0) AND
        (model_duration_ms IS NULL OR model_duration_ms >= 0) AND
        (first_token_ms IS NULL OR first_token_ms >= 0) AND
        (output_tokens IS NULL OR output_tokens >= 0) AND attempt_count >= 0
    )
);

CREATE INDEX IF NOT EXISTS idx_insights_call_facts_statistical_at ON insights_call_facts (statistical_at DESC);
CREATE INDEX IF NOT EXISTS idx_insights_call_facts_user_time ON insights_call_facts (user_id, statistical_at DESC);
CREATE INDEX IF NOT EXISTS idx_insights_call_facts_model_time ON insights_call_facts (platform, model, statistical_at DESC);
CREATE INDEX IF NOT EXISTS idx_insights_call_facts_outcome_time ON insights_call_facts (outcome, statistical_at DESC);
CREATE INDEX IF NOT EXISTS idx_insights_call_facts_unknown_identity ON insights_call_facts (statistical_at DESC) WHERE user_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_insights_call_facts_client_request ON insights_call_facts (client_request_id) WHERE client_request_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_insights_call_facts_usage_link ON insights_call_facts (user_id, api_key_id, request_id, model) WHERE request_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS insights_error_facts (
    call_id UUID PRIMARY KEY REFERENCES insights_call_facts(call_id) ON DELETE CASCADE,
    user_id BIGINT,
    platform TEXT,
    model TEXT,
    error_type VARCHAR(96) NOT NULL,
    error_summary VARCHAR(512),
    statistical_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_insights_error_facts_user_time ON insights_error_facts (user_id, statistical_at DESC);
CREATE INDEX IF NOT EXISTS idx_insights_error_facts_time ON insights_error_facts (statistical_at DESC);

CREATE TABLE IF NOT EXISTS insights_user_model_daily (
    stat_date DATE NOT NULL,
    user_id BIGINT NOT NULL,
    platform TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    usage_count BIGINT NOT NULL DEFAULT 0,
    success_count BIGINT NOT NULL DEFAULT 0,
    failure_count BIGINT NOT NULL DEFAULT 0,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_creation_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens BIGINT NOT NULL DEFAULT 0,
    usage_duration_sum_ms BIGINT NOT NULL DEFAULT 0,
    usage_duration_samples BIGINT NOT NULL DEFAULT 0,
    usage_first_token_sum_ms BIGINT NOT NULL DEFAULT 0,
    usage_first_token_samples BIGINT NOT NULL DEFAULT 0,
    usage_tpot_sum_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
    usage_tpot_samples BIGINT NOT NULL DEFAULT 0,
    gateway_model_duration_sum_ms BIGINT NOT NULL DEFAULT 0,
    gateway_model_duration_samples BIGINT NOT NULL DEFAULT 0,
    gateway_pre_forward_sum_ms BIGINT NOT NULL DEFAULT 0,
    gateway_pre_forward_samples BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (stat_date, user_id, platform, model)
);

CREATE INDEX IF NOT EXISTS idx_insights_user_model_daily_user_date ON insights_user_model_daily (user_id, stat_date DESC);
CREATE INDEX IF NOT EXISTS idx_insights_user_model_daily_model_date ON insights_user_model_daily (platform, model, stat_date DESC);

CREATE TABLE IF NOT EXISTS insights_user_lifecycle (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    first_call_date DATE,
    first_call_at TIMESTAMPTZ,
    returned_day_1_at TIMESTAMPTZ,
    returned_day_7_at TIMESTAMPTZ,
    returned_day_30_at TIMESTAMPTZ,
    first_call_coverage_complete BOOLEAN NOT NULL DEFAULT FALSE,
    return_coverage_complete BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS insights_model_metadata (
    platform TEXT NOT NULL,
    model TEXT NOT NULL,
    introduction TEXT,
    use_cases JSONB,
    capabilities JSONB,
    source_url TEXT,
    source_label TEXT,
    source_updated_at TIMESTAMPTZ,
    expected_version BIGINT NOT NULL DEFAULT 1,
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (platform, model)
);

CREATE TABLE IF NOT EXISTS insights_settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO insights_settings (key, value)
VALUES ('coverage', '{"call_facts":"unknown","usage":"partial","errors":"unknown","daily_rollups":"unknown","lifecycle":"unknown"}'::jsonb)
ON CONFLICT (key) DO NOTHING;

INSERT INTO insights_settings (key, value)
VALUES ('aggregation_config', '{"version":1,"timezone":null,"rebuild_required":true}'::jsonb)
ON CONFLICT (key) DO NOTHING;


CREATE TABLE IF NOT EXISTS insights_rollup_coverage (
    source TEXT NOT NULL CHECK (source IN ('usage','call')),
    stat_date DATE NOT NULL,
    complete BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (source, stat_date)
);
CREATE INDEX IF NOT EXISTS idx_insights_rollup_coverage_date ON insights_rollup_coverage (stat_date, source);

CREATE TABLE IF NOT EXISTS insights_usage_fact_archive (
    usage_id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    account_id BIGINT,
    platform TEXT,
    model TEXT NOT NULL,
    requested_model TEXT,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_creation_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens BIGINT NOT NULL DEFAULT 0,
    duration_ms INT,
    first_token_ms INT,
    stream BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_insights_usage_fact_archive_created ON insights_usage_fact_archive(created_at);
