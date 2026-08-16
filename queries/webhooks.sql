-- name: get-webhooks
SELECT id, name, url, secret, events, enabled, created_at, updated_at
FROM webhooks
ORDER BY id DESC;

-- name: get-webhook-by-id
SELECT id, name, url, secret, events, enabled, created_at, updated_at
FROM webhooks
WHERE id = $1;

-- name: get-active-webhooks-for-event
SELECT id, name, url, secret, events, enabled, created_at, updated_at
FROM webhooks
WHERE enabled = true AND $1 = ANY(events);

-- name: insert-webhook
INSERT INTO webhooks (name, url, secret, events, enabled)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, name, url, secret, events, enabled, created_at, updated_at;

-- name: update-webhook
UPDATE webhooks
SET name = $1, url = $2, secret = $3, events = $4, enabled = $5, updated_at = NOW()
WHERE id = $6
RETURNING id, name, url, secret, events, enabled, created_at, updated_at;

-- name: delete-webhook
DELETE FROM webhooks
WHERE id = $1;

-- name: enqueue-webhook-log
INSERT INTO webhook_logs (webhook_id, event_type, payload, status, next_retry_at)
VALUES ($1, $2, $3, 'pending', NOW())
RETURNING id, webhook_id, event_type, payload, status, attempts, max_attempts, next_retry_at, response_code, response_body, created_at, updated_at;

-- name: pop-pending-webhook-logs
SELECT id, webhook_id, event_type, payload, status, attempts, max_attempts, next_retry_at, response_code, response_body, created_at, updated_at
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
SELECT id, webhook_id, event_type, status, attempts, max_attempts, next_retry_at, response_code, response_body, created_at, updated_at
FROM webhook_logs
ORDER BY id DESC
LIMIT $1 OFFSET $2;
