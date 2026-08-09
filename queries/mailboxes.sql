-- mailboxes

-- name: get-mailboxes
SELECT id, name, email, smtp_config, imap_config, daily_limit, sent_today, created_at, updated_at
FROM mailboxes
ORDER BY id DESC;

-- name: get-mailbox
SELECT id, name, email, smtp_config, imap_config, daily_limit, sent_today, created_at, updated_at
FROM mailboxes
WHERE id = $1;

-- name: create-mailbox
INSERT INTO mailboxes (name, email, smtp_config, imap_config, daily_limit, sent_today)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, name, email, smtp_config, imap_config, daily_limit, sent_today, created_at, updated_at;

-- name: update-mailbox
UPDATE mailboxes
SET name = $2, email = $3, smtp_config = $4, imap_config = $5, daily_limit = $6, updated_at = NOW()
WHERE id = $1;

-- name: delete-mailbox
DELETE FROM mailboxes WHERE id = $1;

-- name: increment-mailbox-sent
UPDATE mailboxes
SET sent_today = sent_today + 1
WHERE id = $1;

-- name: reset-mailbox-daily-counts
UPDATE mailboxes
SET sent_today = 0;
