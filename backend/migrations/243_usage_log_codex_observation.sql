-- Passive per-attempt telemetry. NULL means historical/not observed, not false.
SET LOCAL lock_timeout = '5s';
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS codex_observation JSONB;
COMMENT ON COLUMN usage_logs.codex_observation IS 'HTTP safety header presence/values and account-scoped HMAC route digests; response gateway is an unverified hint. WS explicitly unobserved. No raw cookies.';
