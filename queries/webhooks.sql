-- name: get-webhook-endpoints
SELECT id, name, url, secret, events, enabled, created_at, updated_at
FROM webhook_endpoints
ORDER BY id DESC;

-- name: get-webhook-endpoint-by-id
SELECT id, name, url, secret, events, enabled, created_at, updated_at
FROM webhook_endpoints
WHERE id = $1;

-- name: get-active-endpoints-for-event
SELECT id, name, url, secret, events, enabled, created_at, updated_at
FROM webhook_endpoints
WHERE enabled = true AND $1 = ANY(events);

-- name: insert-webhook-endpoint
INSERT INTO webhook_endpoints (name, url, secret, events, enabled)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, name, url, secret, events, enabled, created_at, updated_at;

-- name: update-webhook-endpoint
UPDATE webhook_endpoints
SET name = $1, url = $2, secret = $3, events = $4, enabled = $5, updated_at = NOW()
WHERE id = $6
RETURNING id, name, url, secret, events, enabled, created_at, updated_at;

-- name: delete-webhook-endpoint
DELETE FROM webhook_endpoints
WHERE id = $1;

-- name: enqueue-webhook-log
INSERT INTO webhook_logs (endpoint_id, event_type, payload, status, next_retry_at)
VALUES ($1, $2, $3, 'pending', NOW())
RETURNING id, endpoint_id, event_type, payload, status, attempts, max_attempts, next_retry_at, response_code, response_body, created_at, updated_at;

-- name: pop-pending-webhook-logs
SELECT id, endpoint_id, event_type, payload, status, attempts, max_attempts, next_retry_at, response_code, response_body, created_at, updated_at
FROM webhook_logs
WHERE status = 'pending' AND next_retry_at <= NOW()
ORDER BY id ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: update-webhook-log-status
UPDATE webhook_logs
SET status = $1, attempts = $2, next_retry_at = $3, response_code = $4, response_body = $5, updated_at = NOW()
WHERE id = $6;

-- name: get-webhook-logs
SELECT id, endpoint_id, event_type, status, attempts, max_attempts, next_retry_at, response_code, response_body, created_at, updated_at
FROM webhook_logs
ORDER BY id DESC
LIMIT $1 OFFSET $2;
