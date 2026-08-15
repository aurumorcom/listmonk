-- sequences

-- name: get-sequences
SELECT s.id, s.uuid, s.name, s.description, s.status, s.schedule_id, s.send_window, s.email_ids, s.waha_sessions, s.archive, s.archive_template_id, s.archive_slug, s.archive_meta, s.created_at, s.updated_at,
(
    SELECT COALESCE(ARRAY_TO_JSON(ARRAY_AGG(l)), '[]') FROM (
        SELECT COALESCE(sequence_lists.list_id, 0) AS id,
        sequence_lists.list_name AS name
        FROM sequence_lists WHERE sequence_lists.sequence_id = s.id
    ) l
) AS lists
FROM sequences s
ORDER BY s.id DESC;

-- name: get-sequence
SELECT s.id, s.uuid, s.name, s.description, s.status, s.schedule_id, s.send_window, s.email_ids, s.waha_sessions, s.archive, s.archive_template_id, s.archive_slug, s.archive_meta, s.created_at, s.updated_at,
(
    SELECT COALESCE(ARRAY_TO_JSON(ARRAY_AGG(l)), '[]') FROM (
        SELECT COALESCE(sequence_lists.list_id, 0) AS id,
        sequence_lists.list_name AS name
        FROM sequence_lists WHERE sequence_lists.sequence_id = s.id
    ) l
) AS lists
FROM sequences s
WHERE s.id = $1 OR s.uuid::text = $2;

-- name: create-sequence
INSERT INTO sequences (uuid, name, description, status, schedule_id, send_window, email_ids, waha_sessions, archive, archive_template_id, archive_slug, archive_meta)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id, uuid, name, description, status, schedule_id, send_window, email_ids, waha_sessions, archive, archive_template_id, archive_slug, archive_meta, created_at, updated_at;

-- name: update-sequence
UPDATE sequences
SET name = $2, description = $3, status = $4, schedule_id = $5, send_window = $6, email_ids = $7, waha_sessions = $8, archive = $9, archive_template_id = $10, archive_slug = $11, archive_meta = $12, updated_at = NOW()
WHERE id = $1;

-- name: delete-sequence
DELETE FROM sequences WHERE id = $1;

-- name: create-sequence-lists
INSERT INTO sequence_lists (sequence_id, list_id, list_name)
SELECT $1, id, name FROM lists WHERE id = ANY($2::INT[]);

-- name: delete-sequence-lists
DELETE FROM sequence_lists WHERE sequence_id = $1;

-- name: enroll-sequence-contacts-by-lists
INSERT INTO sequence_contacts (sequence_id, subscriber_id, status, current_step, next_send_at)
SELECT DISTINCT sl.sequence_id, subl.subscriber_id, 'scheduled', 1, NOW()
FROM sequence_lists sl
JOIN lists l ON l.id = sl.list_id
JOIN subscriber_lists subl ON subl.list_id = sl.list_id
    AND (
        (l.optin = 'double' AND subl.status = 'confirmed') OR
        (l.optin != 'double' AND subl.status != 'unsubscribed')
    )
JOIN subscribers s ON s.id = subl.subscriber_id AND s.status = 'enabled'
WHERE sl.sequence_id = $1
ON CONFLICT (sequence_id, subscriber_id) DO NOTHING;

-- name: enroll-subscribers-into-active-sequences-for-lists
INSERT INTO sequence_contacts (sequence_id, subscriber_id, status, current_step, next_send_at)
SELECT DISTINCT sl.sequence_id, s.id, 'scheduled', 1, NOW()
FROM subscribers s
JOIN subscriber_lists subl ON subl.subscriber_id = s.id
JOIN sequence_lists sl ON sl.list_id = subl.list_id
JOIN lists l ON l.id = sl.list_id
    AND (
        (l.optin = 'double' AND subl.status = 'confirmed') OR
        (l.optin != 'double' AND subl.status != 'unsubscribed')
    )
JOIN sequences seq ON seq.id = sl.sequence_id AND seq.status = 'active'
WHERE s.id = ANY($1::INT[]) AND subl.list_id = ANY($2::INT[]) AND s.status = 'enabled'
ON CONFLICT (sequence_id, subscriber_id) DO NOTHING;

-- name: optout-subscribers-from-sequences-for-removed-lists
UPDATE sequence_contacts sc
SET status = 'opted_out'
WHERE sc.subscriber_id = ANY($1::INT[])
  AND sc.status IN ('scheduled', 'in_progress')
  AND sc.sequence_id IN (
      SELECT sl.sequence_id FROM sequence_lists sl WHERE sl.list_id = ANY($2::INT[])
  )
  AND NOT EXISTS (
      SELECT 1 FROM sequence_lists sl2
      JOIN lists l2 ON l2.id = sl2.list_id
      JOIN subscriber_lists subl ON subl.list_id = sl2.list_id
          AND (
              (l2.optin = 'double' AND subl.status = 'confirmed') OR
              (l2.optin != 'double' AND subl.status != 'unsubscribed')
          )
      WHERE sl2.sequence_id = sc.sequence_id AND subl.subscriber_id = sc.subscriber_id
  );

-- name: get-sequence-steps
SELECT
    s.id, s.sequence_id, s.step_number, s.delay, s.messenger, s.condition,
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
INSERT INTO sequence_steps (sequence_id, step_number, delay, messenger, condition, subject, body, email_type, template_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, sequence_id, step_number, delay, messenger, condition, subject, body, email_type, template_id, created_at;

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
