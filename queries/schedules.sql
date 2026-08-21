-- schedules

-- name: get-schedules
SELECT id, uuid, name, timezone, use_subscriber_timezone, skip_holidays, sending_windows, is_default, created_at, updated_at
FROM schedules
ORDER BY id DESC;

-- name: get-schedule
SELECT id, uuid, name, timezone, use_subscriber_timezone, skip_holidays, sending_windows, is_default, created_at, updated_at
FROM schedules
WHERE id = $1 OR uuid::text = $2;

-- name: create-schedule
INSERT INTO schedules (uuid, name, timezone, use_subscriber_timezone, skip_holidays, sending_windows)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, uuid, name, timezone, use_subscriber_timezone, skip_holidays, sending_windows, is_default, created_at, updated_at;

-- name: update-schedule
UPDATE schedules
SET name = $2, timezone = $3, use_subscriber_timezone = $4, skip_holidays = $5, sending_windows = $6, updated_at = NOW()
WHERE id = $1
RETURNING id, uuid, name, timezone, use_subscriber_timezone, skip_holidays, sending_windows, is_default, created_at, updated_at;

-- name: set-default-schedule
WITH u AS (
    UPDATE schedules SET is_default=true WHERE id=$1 RETURNING id
)
UPDATE schedules SET is_default=false WHERE id != $1;

-- name: delete-schedule
DELETE FROM schedules WHERE id = $1;
