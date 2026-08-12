-- sequences

-- name: get-sequences
SELECT id, uuid, name, status, schedule_id, send_window, created_at, updated_at
FROM sequences
ORDER BY id DESC;

-- name: get-sequence
SELECT id, uuid, name, status, schedule_id, send_window, created_at, updated_at
FROM sequences
WHERE id = $1 OR uuid::text = $2;

-- name: create-sequence
INSERT INTO sequences (uuid, name, status, schedule_id, send_window)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, uuid, name, status, schedule_id, send_window, created_at, updated_at;

-- name: update-sequence
UPDATE sequences
SET name = $2, status = $3, schedule_id = $4, send_window = $5, updated_at = NOW()
WHERE id = $1;

-- name: delete-sequence
DELETE FROM sequences WHERE id = $1;

-- name: get-sequence-steps
SELECT
    s.id, s.sequence_id, s.step_number, s.delay_days, s.messenger, s.condition,
    s.subject, s.body, s.email_type, s.template_id, s.created_at,
    COALESCE(ARRAY_AGG(m.media_id) FILTER (WHERE m.media_id IS NOT NULL), '{}') AS media_ids
FROM sequence_steps s
LEFT JOIN sequence_step_media m ON s.id = m.sequence_step_id
WHERE s.sequence_id = $1
GROUP BY s.id
ORDER BY s.step_number ASC;

-- name: create-sequence-step-media
INSERT INTO sequence_step_media (sequence_step_id, media_id, filename)
SELECT $1, id, filename FROM media WHERE id = ANY($2::INT[]);

-- name: create-sequence-step
INSERT INTO sequence_steps (sequence_id, step_number, delay_days, messenger, condition, subject, body, email_type, template_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, sequence_id, step_number, delay_days, messenger, condition, subject, body, email_type, template_id, created_at;

-- name: delete-sequence-steps
DELETE FROM sequence_steps WHERE sequence_id = $1;

-- name: enroll-sequence-subscribers
INSERT INTO sequence_contacts (sequence_id, subscriber_id, status, current_step, next_send_at)
SELECT $1, id, 'scheduled', 1, NOW()
FROM subscribers
WHERE id = ANY($2::INT[])
ON CONFLICT (sequence_id, subscriber_id) DO NOTHING;

-- name: get-due-sequence-subscribers
SELECT sequence_id, subscriber_id, email_id, waha_session, status, current_step, next_send_at, last_read_at, last_clicked_at, last_message_id, last_thread_msg_id, created_at
FROM sequence_contacts
WHERE status IN ('scheduled', 'in_progress') AND next_send_at <= NOW()
LIMIT $1;

-- name: update-sequence-subscriber-status
UPDATE sequence_contacts
SET status = $3, current_step = $4, next_send_at = $5, last_message_id = $6, last_thread_msg_id = $7
WHERE sequence_id = $1 AND subscriber_id = $2;

-- name: update-sequence-subscriber-read
UPDATE sequence_contacts
SET last_read_at = NOW()
WHERE sequence_id = $1 AND subscriber_id = $2;

-- name: update-sequence-subscriber-click
UPDATE sequence_contacts
SET last_clicked_at = NOW()
WHERE sequence_id = $1 AND subscriber_id = $2;

-- name: set-sequence-subscriber-replied
UPDATE sequence_contacts
SET status = 'replied'
WHERE subscriber_id = (SELECT id FROM subscribers WHERE email = $1 LIMIT 1)
  AND status IN ('scheduled', 'in_progress');
