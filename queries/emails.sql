-- emails

-- name: get-emails
SELECT id, name, email, smtp_config, imap_config, max_send_per_day, sent_today, user_id, signature, created_at, updated_at
FROM emails
ORDER BY id DESC;

-- name: get-email
SELECT id, name, email, smtp_config, imap_config, max_send_per_day, sent_today, user_id, signature, created_at, updated_at
FROM emails
WHERE id = $1;

-- name: create-email
INSERT INTO emails (name, email, smtp_config, imap_config, max_send_per_day, sent_today, user_id, signature)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, name, email, smtp_config, imap_config, max_send_per_day, sent_today, user_id, signature, created_at, updated_at;

-- name: update-email
UPDATE emails
SET name = $2, email = $3, smtp_config = $4, imap_config = $5, max_send_per_day = $6, user_id = $7, signature = $8, updated_at = NOW()
WHERE id = $1;

-- name: delete-email
DELETE FROM emails WHERE id = $1;

-- name: increment-email-sent
UPDATE emails
SET sent_today = sent_today + 1
WHERE id = $1;

-- name: reset-email-daily-counts
UPDATE emails
SET sent_today = 0;
