-- emails

-- name: get-emails
SELECT id, name, email, smtp_config, imap_config, emails_per_day, emails_per_hour, emails_today, user_id, signature, created_at, updated_at
FROM emails
ORDER BY id DESC;

-- name: get-email
SELECT id, name, email, smtp_config, imap_config, emails_per_day, emails_per_hour, emails_today, user_id, signature, created_at, updated_at
FROM emails
WHERE id = $1;

-- name: create-email
INSERT INTO emails (name, email, smtp_config, imap_config, emails_per_day, emails_per_hour, emails_today, user_id, signature)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, name, email, smtp_config, imap_config, emails_per_day, emails_per_hour, emails_today, user_id, signature, created_at, updated_at;

-- name: update-email
UPDATE emails
SET name = $2, email = $3, smtp_config = $4, imap_config = $5, emails_per_day = $6, emails_per_hour = $7, user_id = $8, signature = $9, updated_at = NOW()
WHERE id = $1;

-- name: delete-email
DELETE FROM emails WHERE id = $1;

-- name: increment-email-sent
UPDATE emails
SET emails_today = emails_today + 1
WHERE id = $1;

-- name: reset-email-daily-counts
UPDATE emails
SET emails_today = 0;
