ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS client_request_type SMALLINT;

COMMENT ON COLUMN usage_logs.client_request_type IS
    'Client ingress request type: 1=sync HTTP, 2=SSE, 3=WebSocket; NULL=unknown/legacy';
