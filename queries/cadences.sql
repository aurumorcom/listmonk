-- cadences

-- name: get-cadences
SELECT id, uuid, name, status, send_window, created_at, updated_at
FROM cadences
ORDER BY id DESC;

-- name: get-cadence
SELECT id, uuid, name, status, send_window, created_at, updated_at
FROM cadences
WHERE id = $1 OR uuid::text = $2;

-- name: create-cadence
INSERT INTO cadences (uuid, name, status, send_window)
VALUES ($1, $2, $3, $4)
RETURNING id, uuid, name, status, send_window, created_at, updated_at;

-- name: update-cadence
UPDATE cadences
SET name = $2, status = $3, send_window = $4, updated_at = NOW()
WHERE id = $1;

-- name: delete-cadence
DELETE FROM cadences WHERE id = $1;

-- name: get-cadence-steps
SELECT
    s.id, s.cadence_id, s.step_number, s.delay_days, s.messenger, s.condition,
    s.subject, s.body, s.template_id, s.created_at,
    COALESCE(ARRAY_AGG(m.media_id) FILTER (WHERE m.media_id IS NOT NULL), '{}') AS media_ids
FROM cadence_steps s
LEFT JOIN cadence_step_media m ON s.id = m.cadence_step_id
WHERE s.cadence_id = $1
GROUP BY s.id
ORDER BY s.step_number ASC;

-- name: create-cadence-step-media
INSERT INTO cadence_step_media (cadence_step_id, media_id, filename)
SELECT $1, id, filename FROM media WHERE id = ANY($2::INT[]);

-- name: create-cadence-step
INSERT INTO cadence_steps (cadence_id, step_number, delay_days, messenger, condition, subject, body, template_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, cadence_id, step_number, delay_days, messenger, condition, subject, body, template_id, created_at;

-- name: delete-cadence-steps
DELETE FROM cadence_steps WHERE cadence_id = $1;

-- name: enroll-cadence-subscribers
INSERT INTO cadence_contacts (cadence_id, subscriber_id, status, current_step, next_send_at)
SELECT $1, id, 'scheduled', 1, NOW()
FROM subscribers
WHERE id = ANY($2::INT[])
ON CONFLICT (cadence_id, subscriber_id) DO NOTHING;

-- name: get-due-cadence-subscribers
SELECT cadence_id, subscriber_id, status, current_step, next_send_at, last_read_at, last_clicked_at, last_message_id, created_at
FROM cadence_contacts
WHERE status IN ('scheduled', 'in_progress') AND next_send_at <= NOW()
LIMIT $1;

-- name: update-cadence-subscriber-status
UPDATE cadence_contacts
SET status = $3, current_step = $4, next_send_at = $5, last_message_id = $6
WHERE cadence_id = $1 AND subscriber_id = $2;

-- name: update-cadence-subscriber-read
UPDATE cadence_contacts
SET last_read_at = NOW()
WHERE cadence_id = $1 AND subscriber_id = $2;

-- name: update-cadence-subscriber-click
UPDATE cadence_contacts
SET last_clicked_at = NOW()
WHERE cadence_id = $1 AND subscriber_id = $2;

-- name: set-cadence-subscriber-replied
UPDATE cadence_contacts
SET status = 'replied'
WHERE subscriber_id = (SELECT id FROM subscribers WHERE email = $1 LIMIT 1)
  AND status IN ('scheduled', 'in_progress');
