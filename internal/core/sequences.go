package core

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/media"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	null "gopkg.in/volatiletech/null.v6"
)

// GetSequences returns a list of all sequences.
func (c *Core) GetSequences() ([]models.Sequence, error) {
	var out []models.Sequence
	err := c.db.Select(&out, `SELECT s.id, s.uuid, s.name, s.description, s.status, s.schedule_id, s.send_window, s.email_ids, s.waha_sessions, s.archive, s.archive_template_id, s.archive_slug, s.archive_meta, s.created_at, s.updated_at,
	(
		SELECT COALESCE(ARRAY_TO_JSON(ARRAY_AGG(l)), '[]') FROM (
			SELECT COALESCE(sequence_lists.list_id, 0) AS id,
			sequence_lists.list_name AS name
			FROM sequence_lists WHERE sequence_lists.sequence_id = s.id
		) l
	) AS lists
	FROM sequences s
	ORDER BY s.id DESC`)
	if err != nil {
		c.log.Printf("error querying sequences: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	for i := range out {
		if out[i].ScheduleID.Valid {
			s, err := c.GetSchedule(out[i].ScheduleID.Int, "")
			if err == nil {
				out[i].Schedule = s
			}
		}
	}
	return out, nil
}

// GetSequence returns a sequence by ID or UUID.
func (c *Core) GetSequence(id int, uid string) (*models.Sequence, error) {
	var seq models.Sequence
	err := c.db.Get(&seq, `SELECT s.id, s.uuid, s.name, s.description, s.status, s.schedule_id, s.send_window, s.email_ids, s.waha_sessions, s.archive, s.archive_template_id, s.archive_slug, s.archive_meta, s.created_at, s.updated_at,
	(
		SELECT COALESCE(ARRAY_TO_JSON(ARRAY_AGG(l)), '[]') FROM (
			SELECT COALESCE(sequence_lists.list_id, 0) AS id,
			sequence_lists.list_name AS name
			FROM sequence_lists WHERE sequence_lists.sequence_id = s.id
		) l
	) AS lists
	FROM sequences s
	WHERE s.id = $1 OR s.uuid::text = $2`, id, uid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, echo.NewHTTPError(http.StatusNotFound, c.i18n.Ts("globals.messages.notFound"))
		}
		c.log.Printf("error getting sequence: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	if seq.ScheduleID.Valid {
		s, err := c.GetSchedule(seq.ScheduleID.Int, "")
		if err == nil {
			seq.Schedule = s
		}
	}
	return &seq, nil
}

// CreateSequence creates a new sequence.
func (c *Core) CreateSequence(seq models.Sequence, listIDs ...[]int) (*models.Sequence, error) {
	if seq.UUID == "" {
		seq.UUID = uuid.Must(uuid.NewV4()).String()
	}
	if seq.Status == "" {
		seq.Status = models.SequenceStatusActive
	}
	if seq.SendWindow == nil {
		seq.SendWindow = models.JSON{}
	}
	if seq.ArchiveMeta == nil {
		seq.ArchiveMeta = models.JSON{}
	}
	var out models.Sequence
	err := c.db.Get(&out, `INSERT INTO sequences (uuid, name, description, status, schedule_id, send_window, email_ids, waha_sessions, archive, archive_template_id, archive_slug, archive_meta)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, uuid, name, description, status, schedule_id, send_window, email_ids, waha_sessions, archive, archive_template_id, archive_slug, archive_meta, created_at, updated_at`,
		seq.UUID, seq.Name, seq.Description, seq.Status, seq.ScheduleID, seq.SendWindow, seq.EmailIDs, seq.WahaSessions, seq.Archive, seq.ArchiveTemplateID, seq.ArchiveSlug, seq.ArchiveMeta)
	if err != nil {
		c.log.Printf("error creating sequence: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}

	var targetLists []int
	if len(listIDs) > 0 && listIDs[0] != nil {
		targetLists = listIDs[0]
	}
	if err := c.syncSequenceLists(out.ID, targetLists, seq.Status); err != nil {
		c.log.Printf("error syncing sequence lists on create: %v", err)
	}

	res, _ := c.GetSequence(out.ID, "")
	if res != nil {
		_ = c.DispatchWebhookEvent("sequence.created", res)
	}
	return res, nil
}

// UpdateSequence updates an existing sequence.
func (c *Core) UpdateSequence(seq models.Sequence, listIDs ...[]int) (*models.Sequence, error) {
	if len(seq.EmailIDs) == 0 {
		seq.EmailIDs = pq.Int64Array{}
	}
	if len(seq.WahaSessions) == 0 {
		seq.WahaSessions = pq.StringArray{}
	}
	if seq.SendWindow == nil {
		seq.SendWindow = models.JSON{}
	}
	if seq.ArchiveMeta == nil {
		seq.ArchiveMeta = models.JSON{}
	}
	_, err := c.db.Exec(`UPDATE sequences
		SET name = $2, description = $3, status = $4, schedule_id = $5, send_window = $6, email_ids = $7, waha_sessions = $8, archive = $9, archive_template_id = $10, archive_slug = $11, archive_meta = $12, updated_at = NOW()
		WHERE id = $1`,
		seq.ID, seq.Name, seq.Description, seq.Status, seq.ScheduleID, seq.SendWindow, seq.EmailIDs, seq.WahaSessions, seq.Archive, seq.ArchiveTemplateID, seq.ArchiveSlug, seq.ArchiveMeta)
	if err != nil {
		c.log.Printf("error updating sequence: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}

	if len(listIDs) > 0 && listIDs[0] != nil {
		if err := c.syncSequenceLists(seq.ID, listIDs[0], seq.Status); err != nil {
			c.log.Printf("error syncing sequence lists on update: %v", err)
		}
	}

	return c.GetSequence(seq.ID, "")
}

func (c *Core) syncSequenceLists(seqID int, listIDs []int, status string) error {
	_, err := c.db.Exec("DELETE FROM sequence_lists WHERE sequence_id = $1", seqID)
	if err != nil {
		return err
	}
	if len(listIDs) > 0 {
		_, err = c.db.Exec(`INSERT INTO sequence_lists (sequence_id, list_id, list_name)
			SELECT $1, id, name FROM lists WHERE id = ANY($2::INT[])`, seqID, pq.Array(listIDs))
		if err != nil {
			return err
		}
	}

	if status == models.SequenceStatusActive && len(listIDs) > 0 {
		// Backfill/auto-enroll all active subscribers in target lists
		_, err = c.db.Exec(`INSERT INTO sequence_contacts (sequence_id, subscriber_id, status, current_step, next_send_at)
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
			ON CONFLICT (sequence_id, subscriber_id) DO UPDATE SET
				status = CASE
					WHEN sequence_contacts.status = 'opted_out' THEN 'scheduled'
					ELSE sequence_contacts.status
				END,
				next_send_at = CASE
					WHEN sequence_contacts.status = 'opted_out' THEN NOW()
					ELSE sequence_contacts.next_send_at
				END`, seqID)
		if err != nil {
			return err
		}
	}

	// Opt out any in-flight contacts who are no longer associated with any active list for this sequence
	_, err = c.db.Exec(`UPDATE sequence_contacts sc
		SET status = 'opted_out'
		WHERE sc.sequence_id = $1
		  AND sc.status IN ('scheduled', 'in_progress')
		  AND NOT EXISTS (
		      SELECT 1 FROM sequence_lists sl
		      JOIN lists l ON l.id = sl.list_id
		      JOIN subscriber_lists subl ON subl.list_id = sl.list_id
		          AND (
		              (l.optin = 'double' AND subl.status = 'confirmed') OR
		              (l.optin != 'double' AND subl.status != 'unsubscribed')
		          )
		      WHERE sl.sequence_id = sc.sequence_id AND subl.subscriber_id = sc.subscriber_id
		  )`, seqID)
	return err
}

// EnrollSubscribersByList enrolls active subscribers for given list IDs into all active sequences targeting those lists.
func (c *Core) EnrollSubscribersByList(subIDs []int, listIDs []int, userContext ...map[string]any) error {
	if len(subIDs) == 0 || len(listIDs) == 0 {
		return nil
	}

	var (
		explicitEmailID     null.Int
		explicitWahaSession null.String
	)

	if len(userContext) > 0 && len(userContext[0]) > 0 {
		ctx := userContext[0]
		if rawEID, ok := ctx["email_id"].(float64); ok && rawEID > 0 {
			explicitEmailID = null.IntFrom(int(rawEID))
		} else if rawEIDInt, ok := ctx["email_id"].(int); ok && rawEIDInt > 0 {
			explicitEmailID = null.IntFrom(rawEIDInt)
		}

		if rawWS, ok := ctx["waha_session"].(string); ok && strings.TrimSpace(rawWS) != "" {
			explicitWahaSession = null.StringFrom(strings.TrimSpace(rawWS))
		}

		var uid int
		if rawID, ok := ctx["id"].(float64); ok && rawID > 0 {
			uid = int(rawID)
		} else if rawIDInt, ok := ctx["id"].(int); ok && rawIDInt > 0 {
			uid = rawIDInt
		} else if rawUID, ok := ctx["user_id"].(float64); ok && rawUID > 0 {
			uid = int(rawUID)
		} else if rawUIDInt, ok := ctx["user_id"].(int); ok && rawUIDInt > 0 {
			uid = rawUIDInt
		}

		if uid > 0 {
			var u auth.User
			if err := c.db.Get(&u, "SELECT id, email_id, waha_session FROM users WHERE id = $1", uid); err == nil {
				if u.EmailID.Valid && !explicitEmailID.Valid {
					explicitEmailID = u.EmailID
				}
				if u.WahaSession.Valid && u.WahaSession.String != "" && (!explicitWahaSession.Valid || explicitWahaSession.String == "") {
					explicitWahaSession = u.WahaSession
				}
			}
			if !explicitEmailID.Valid {
				var emailAccountID int
				if err := c.db.Get(&emailAccountID, "SELECT id FROM emails WHERE user_id = $1 ORDER BY id ASC LIMIT 1", uid); err == nil && emailAccountID > 0 {
					explicitEmailID = null.IntFrom(emailAccountID)
				}
			}
		}
	}

	var mbVal any
	if explicitEmailID.Valid {
		mbVal = explicitEmailID.Int
	}
	var wsVal any
	if explicitWahaSession.Valid && explicitWahaSession.String != "" {
		wsVal = explicitWahaSession.String
	}

	_, err := c.db.Exec(`INSERT INTO sequence_contacts (sequence_id, subscriber_id, email_id, waha_session, status, current_step, next_send_at)
		SELECT DISTINCT sl.sequence_id, s.id, $3::INT, $4::TEXT, 'scheduled', 1, NOW()
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
		ON CONFLICT (sequence_id, subscriber_id) DO UPDATE SET
			status = CASE
				WHEN sequence_contacts.status = 'opted_out' THEN 'scheduled'
				ELSE sequence_contacts.status
			END,
			next_send_at = CASE
				WHEN sequence_contacts.status = 'opted_out' THEN NOW()
				ELSE sequence_contacts.next_send_at
			END,
			email_id = COALESCE(sequence_contacts.email_id, EXCLUDED.email_id),
			waha_session = COALESCE(sequence_contacts.waha_session, EXCLUDED.waha_session)`,
		pq.Array(subIDs), pq.Array(listIDs), mbVal, wsVal)
	if err != nil {
		c.log.Printf("error auto-enrolling subscribers by list into active sequences: %v", err)
		return err
	}
	return nil
}

// OptOutSubscribersByList marks sequence contacts as opted_out if they no longer belong to any active list attached to the sequence.
func (c *Core) OptOutSubscribersByList(subIDs []int, listIDs []int) error {
	if len(subIDs) == 0 || len(listIDs) == 0 {
		return nil
	}
	_, err := c.db.Exec(`UPDATE sequence_contacts sc
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
		  )`, pq.Array(subIDs), pq.Array(listIDs))
	if err != nil {
		c.log.Printf("error opting out subscribers from sequences for removed lists: %v", err)
		return err
	}
	return nil
}

// UpdateSequenceStatus updates a sequence's status.
func (c *Core) UpdateSequenceStatus(id int, status string) (*models.Sequence, error) {
	switch status {
	case models.SequenceStatusActive, models.SequenceStatusPaused, models.SequenceStatusArchived, "cancelled":
	default:
		return nil, echo.NewHTTPError(http.StatusBadRequest, c.i18n.Ts("globals.messages.invalidFields", "name", "status"))
	}

	_, err := c.db.Exec("UPDATE sequences SET status = $2, updated_at = NOW() WHERE id = $1", id, status)
	if err != nil {
		c.log.Printf("error updating sequence status: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}

	if status == models.SequenceStatusActive {
		// Auto-enroll all active subscribers in target lists on activation
		_, _ = c.db.Exec(`INSERT INTO sequence_contacts (sequence_id, subscriber_id, status, current_step, next_send_at)
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
			ON CONFLICT (sequence_id, subscriber_id) DO UPDATE SET
				status = CASE
					WHEN sequence_contacts.status = 'opted_out' THEN 'scheduled'
					ELSE sequence_contacts.status
				END,
				next_send_at = CASE
					WHEN sequence_contacts.status = 'opted_out' THEN NOW()
					ELSE sequence_contacts.next_send_at
				END`, id)
	}

	res, err := c.GetSequence(id, "")
	if err == nil && res != nil {
		_ = c.DispatchWebhookEvent("sequence.updated", res)
	}
	return res, err
}

// UpdateSequenceArchive updates sequence web archive settings.
func (c *Core) UpdateSequenceArchive(id int, archive bool, templateID null.Int, meta models.JSON, archiveSlug null.String) error {
	if meta == nil {
		meta = models.JSON{}
	}
	_, err := c.db.Exec(`UPDATE sequences
		SET archive = $2, archive_template_id = $3, archive_meta = $4, archive_slug = $5, updated_at = NOW()
		WHERE id = $1`, id, archive, templateID, meta, archiveSlug)
	if err != nil {
		c.log.Printf("error updating sequence archive: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return nil
}

// DeleteSequence deletes a sequence.
func (c *Core) DeleteSequence(id int) error {
	_, err := c.db.Exec("DELETE FROM sequences WHERE id = $1", id)
	if err != nil {
		c.log.Printf("error deleting sequence: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return nil
}

// DeleteSequences bulk deletes sequences by ID list.
func (c *Core) DeleteSequences(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := c.db.Exec("DELETE FROM sequences WHERE id = ANY($1)", pq.Array(ids))
	if err != nil {
		c.log.Printf("error bulk deleting sequences: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return nil
}

// ManageContactSequences modifies contact sequence memberships (enroll, disenroll, pause).
func (c *Core) ManageContactSequences(contactIDs []int, sequenceIDs []int, action string, status string) error {
	if len(contactIDs) == 0 || len(sequenceIDs) == 0 {
		return nil
	}
	switch action {
	case "add", "enroll":
		if status == "" {
			status = models.SequenceContactStatusScheduled
		}
		for _, seqID := range sequenceIDs {
			_, err := c.db.Exec(`INSERT INTO sequence_contacts (sequence_id, subscriber_id, status, current_step, next_send_at)
				SELECT $1, id, $3, 1, NOW()
				FROM subscribers
				WHERE id = ANY($2)
				ON CONFLICT (sequence_id, subscriber_id) DO UPDATE SET status = EXCLUDED.status`,
				seqID, pq.Array(contactIDs), status)
			if err != nil {
				c.log.Printf("error enrolling contacts into sequence %d: %v", seqID, err)
				return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
			}
		}
	case "remove", "disenroll":
		_, err := c.db.Exec("DELETE FROM sequence_contacts WHERE subscriber_id = ANY($1) AND sequence_id = ANY($2)",
			pq.Array(contactIDs), pq.Array(sequenceIDs))
		if err != nil {
			c.log.Printf("error disenrolling contacts: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
		}
	case "pause":
		_, err := c.db.Exec("UPDATE sequence_contacts SET status = 'paused' WHERE subscriber_id = ANY($1) AND sequence_id = ANY($2)",
			pq.Array(contactIDs), pq.Array(sequenceIDs))
		if err != nil {
			c.log.Printf("error pausing contacts in sequence: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
		}
	default:
		return echo.NewHTTPError(http.StatusBadRequest, c.i18n.Ts("globals.messages.invalidFields", "name", "action"))
	}
	return nil
}

// GetContactSequences returns sequence memberships for a given contact ID.
func (c *Core) GetContactSequences(contactID int) ([]models.SequenceContact, error) {
	var out []models.SequenceContact
	err := c.db.Select(&out, `SELECT sequence_id, subscriber_id, email_id, waha_session, status, current_step, next_send_at, last_read_at, last_clicked_at, last_message_id, created_at
		FROM sequence_contacts WHERE subscriber_id = $1 ORDER BY sequence_id ASC`, contactID)
	if err != nil {
		c.log.Printf("error getting contact sequences: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return out, nil
}

// GetSequenceSteps returns the steps for a given sequence.
func (c *Core) GetSequenceSteps(sequenceID int) ([]models.SequenceStep, error) {
	var steps []models.SequenceStep
	query := `SELECT
		s.id, s.sequence_id, s.step_number, s.delay, s.messenger, s.condition,
		s.subject, s.body, COALESCE(s.email_type, '') AS email_type, s.template_id,
		COALESCE(ARRAY_AGG(m.media_id) FILTER (WHERE m.media_id IS NOT NULL), '{}') AS media_ids
	FROM sequence_steps s
	LEFT JOIN sequence_step_media m ON s.id = m.sequence_step_id
	WHERE s.sequence_id = $1
	GROUP BY s.id
	ORDER BY s.step_number ASC`
	err := c.db.Select(&steps, query, sequenceID)
	if err != nil {
		c.log.Printf("error getting sequence steps: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return steps, nil
}

// SaveSequenceSteps updates or inserts steps for a sequence.
func (c *Core) SaveSequenceSteps(sequenceID int, steps []models.SequenceStep) error {
	tx, err := c.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM sequence_steps WHERE sequence_id = $1", sequenceID); err != nil {
		return err
	}

	for i, s := range steps {
		s.SequenceID = sequenceID
		s.StepNumber = i + 1
		if s.Delay == "" {
			s.Delay = "0s"
		}
		if s.Messenger == "" {
			s.Messenger = "email"
		}
		if s.Condition == "" {
			s.Condition = models.SequenceConditionAlways
		}
		// Step 1 email cannot have EmailType field (New Thread, Reply)
		if s.StepNumber == 1 {
			s.EmailType = ""
		}

		var newID int
		err := tx.Get(&newID, `INSERT INTO sequence_steps (sequence_id, step_number, delay, messenger, condition, subject, body, email_type, template_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
			s.SequenceID, s.StepNumber, s.Delay, s.Messenger, s.Condition, s.Subject, s.Body, s.EmailType, s.TemplateID)
		if err != nil {
			return err
		}

		if len(s.MediaIDs) > 0 {
			_, err = tx.Exec(`INSERT INTO sequence_step_media (sequence_step_id, media_id, filename)
				SELECT $1, id, filename FROM media WHERE id = ANY($2::INT[])`,
				newID, s.MediaIDs)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// GetStepAttachments loads attachments for a given list of media IDs using the media store.
func (c *Core) GetStepAttachments(store media.Store, mediaIDs []int64) ([]models.Attachment, error) {
	if len(mediaIDs) == 0 || store == nil {
		return nil, nil
	}
	var atts []models.Attachment
	for _, mid := range mediaIDs {
		m, err := c.GetMedia(int(mid), "", "", store)
		if err != nil {
			c.log.Printf("error fetching media %d: %v", mid, err)
			continue
		}
		b, err := store.GetBlob(m.URL)
		if err != nil {
			c.log.Printf("error fetching blob for media %d (%s): %v", mid, m.Filename, err)
			continue
		}
		atts = append(atts, models.Attachment{
			Name:    m.Filename,
			Content: b,
		})
	}
	return atts, nil
}

// AllocateSendersRoundRobinInt distributes subscriber IDs round-robin across an integer pool (e.g. email account IDs).
func AllocateSendersRoundRobinInt(subIDs []int, pool []int64) map[int]null.Int {
	alloc := make(map[int]null.Int, len(subIDs))
	if len(pool) == 0 {
		for _, id := range subIDs {
			alloc[id] = null.Int{}
		}
		return alloc
	}
	for i, subID := range subIDs {
		alloc[subID] = null.IntFrom(int(pool[i%len(pool)]))
	}
	return alloc
}

// AllocateSendersRoundRobinString distributes subscriber IDs round-robin across a string pool (e.g. WAHA sessions).
func AllocateSendersRoundRobinString(subIDs []int, pool []string) map[int]null.String {
	alloc := make(map[int]null.String, len(subIDs))
	if len(pool) == 0 {
		for _, id := range subIDs {
			alloc[id] = null.String{}
		}
		return alloc
	}
	for i, subID := range subIDs {
		alloc[subID] = null.StringFrom(pool[i%len(pool)])
	}
	return alloc
}

// AllocateSendersCapacityWeighted distributes subscriber IDs based on remaining email account daily capacity.
func AllocateSendersCapacityWeighted(subIDs []int, emails []models.Email) map[int]null.Int {
	alloc := make(map[int]null.Int, len(subIDs))
	if len(emails) == 0 {
		for _, id := range subIDs {
			alloc[id] = null.Int{}
		}
		return alloc
	}

	type capBox struct {
		id        int64
		remaining int
	}

	var active []capBox
	totalRemaining := 0
	for _, m := range emails {
		rem := 0
		if m.MaxSendPerDay > 0 {
			rem = m.MaxSendPerDay - m.SentToday
		} else {
			rem = 10000 // Unlimited fallback weight
		}
		if rem < 0 {
			rem = 0
		}
		if rem > 0 {
			active = append(active, capBox{id: int64(m.ID), remaining: rem})
			totalRemaining += rem
		}
	}

	if totalRemaining == 0 || len(active) == 0 {
		// Fallback to round-robin if no email account has remaining capacity
		var pool []int64
		for _, m := range emails {
			pool = append(pool, int64(m.ID))
		}
		return AllocateSendersRoundRobinInt(subIDs, pool)
	}

	// Calculate quotas
	n := len(subIDs)
	quotas := make(map[int64]int, len(active))
	assignedCount := 0

	for _, b := range active {
		q := (n * b.remaining) / totalRemaining
		quotas[b.id] = q
		assignedCount += q
	}

	// Distribute remainder
	remainder := n - assignedCount
	for i := 0; i < remainder; i++ {
		quotas[active[i%len(active)].id]++
	}

	// Assign subIDs based on quotas
	subIdx := 0
	for _, b := range active {
		q := quotas[b.id]
		for j := 0; j < q && subIdx < len(subIDs); j++ {
			alloc[subIDs[subIdx]] = null.IntFrom(int(b.id))
			subIdx++
		}
	}

	// Fallback for any leftover subscriber IDs
	for subIdx < len(subIDs) {
		alloc[subIDs[subIdx]] = null.IntFrom(int(active[subIdx%len(active)].id))
		subIdx++
	}

	return alloc
}

// EnrollSequenceContacts enrolls contacts into a sequence with automatic channel locking and optional User context.
func (c *Core) EnrollSequenceContacts(sequenceID int, subscriberIDs []int, userContext map[string]any) error {
	if len(subscriberIDs) == 0 {
		return nil
	}

	if _, err := c.GetSequence(sequenceID, ""); err != nil {
		return err
	}

	var explicitEmailID null.Int
	var explicitWahaSession null.String

	// Extract explicit channel options if present in userContext
	if len(userContext) > 0 {
		if rawEID, ok := userContext["email_id"].(float64); ok && rawEID > 0 {
			explicitEmailID = null.IntFrom(int(rawEID))
		} else if rawEIDInt, ok := userContext["email_id"].(int); ok && rawEIDInt > 0 {
			explicitEmailID = null.IntFrom(rawEIDInt)
		}

		if rawWS, ok := userContext["waha_session"].(string); ok && strings.TrimSpace(rawWS) != "" {
			explicitWahaSession = null.StringFrom(strings.TrimSpace(rawWS))
		}

		var uid int
		if rawID, ok := userContext["id"].(float64); ok && rawID > 0 {
			uid = int(rawID)
		} else if rawIDInt, ok := userContext["id"].(int); ok && rawIDInt > 0 {
			uid = rawIDInt
		} else if rawUID, ok := userContext["user_id"].(float64); ok && rawUID > 0 {
			uid = int(rawUID)
		} else if rawUIDInt, ok := userContext["user_id"].(int); ok && rawUIDInt > 0 {
			uid = rawUIDInt
		}

		if uid > 0 {
			var u auth.User
			if err := c.db.Get(&u, "SELECT id, email_id, waha_session FROM users WHERE id = $1", uid); err == nil {
				if u.EmailID.Valid && !explicitEmailID.Valid {
					explicitEmailID = u.EmailID
				}
				if u.WahaSession.Valid && u.WahaSession.String != "" && (!explicitWahaSession.Valid || explicitWahaSession.String == "") {
					explicitWahaSession = u.WahaSession
				}
			}
			// Fallback: look up email account owned by this user_id if email_id not set on user profile
			if !explicitEmailID.Valid {
				var emailAccountID int
				if err := c.db.Get(&emailAccountID, "SELECT id FROM emails WHERE user_id = $1 ORDER BY id ASC LIMIT 1", uid); err == nil && emailAccountID > 0 {
					explicitEmailID = null.IntFrom(emailAccountID)
				}
			}
		}
	}

	emailAlloc := make(map[int]null.Int, len(subscriberIDs))
	wahaAlloc := make(map[int]null.String, len(subscriberIDs))

	// Direct channel assignment for enrolled contacts
	for _, id := range subscriberIDs {
		emailAlloc[id] = explicitEmailID
		wahaAlloc[id] = explicitWahaSession
	}

	tx, err := c.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Preparex(`INSERT INTO sequence_contacts (sequence_id, subscriber_id, email_id, waha_session, status, current_step, next_send_at)
		VALUES ($1, $2, $3, $4, 'scheduled', 1, NOW())
		ON CONFLICT (sequence_id, subscriber_id) DO UPDATE SET email_id = EXCLUDED.email_id, waha_session = EXCLUDED.waha_session`)
	if err != nil {
		c.log.Printf("error preparing sequence enrollment stmt: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	defer stmt.Close()

	for _, subID := range subscriberIDs {
		emailID := emailAlloc[subID]
		wSession := wahaAlloc[subID]

		var mbVal any
		if emailID.Valid {
			mbVal = emailID.Int
		}
		var wsVal any
		if wSession.Valid && wSession.String != "" {
			wsVal = wSession.String
		}

		if _, err := stmt.Exec(sequenceID, subID, mbVal, wsVal); err != nil {
			c.log.Printf("error enrolling subscriber %d into sequence %d: %v", subID, sequenceID, err)
			return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
		}

		if len(userContext) > 0 {
			userBytes, err := json.Marshal(userContext)
			if err == nil {
				_, _ = tx.Exec(`UPDATE subscribers SET attribs = jsonb_set(COALESCE(attribs, '{}'::jsonb), '{user}', $1::jsonb) WHERE id = $2`, string(userBytes), subID)
			}
		}
	}

	return tx.Commit()
}

// GetDueSequenceContacts returns sequence contacts due for sending for active sequences.
func (c *Core) GetDueSequenceContacts(limit int) ([]models.SequenceContact, error) {
	var out []models.SequenceContact
	err := c.db.Select(&out, `SELECT sc.sequence_id, sc.subscriber_id, sc.email_id, sc.waha_session, sc.status, sc.current_step, sc.next_send_at, sc.last_read_at, sc.last_clicked_at, sc.last_message_id, sc.last_thread_msg_id, sc.created_at
		FROM sequence_contacts sc
		JOIN sequences s ON s.id = sc.sequence_id
		WHERE s.status = $1 AND sc.status IN ('scheduled', 'in_progress') AND sc.next_send_at <= NOW()
		LIMIT $2`, models.SequenceStatusActive, limit)
	if err != nil {
		c.log.Printf("error getting due sequence contacts: %v", err)
		return nil, err
	}
	return out, nil
}

// UpdateSequenceContactStatus updates progress of a contact in a sequence.
func (c *Core) UpdateSequenceContactStatus(sequenceID, subID int, status string, currentStep int, nextSendAt null.Time, lastMsgID null.String, lastThreadMsgID null.String) error {
	_, err := c.db.Exec(`UPDATE sequence_contacts
		SET status = $3, current_step = $4, next_send_at = $5, last_message_id = $6, last_thread_msg_id = $7
		WHERE sequence_id = $1 AND subscriber_id = $2`,
		sequenceID, subID, status, currentStep, nextSendAt, lastMsgID, lastThreadMsgID)
	return err
}

// RecordSequenceRead records an open/read event for a sequence subscriber.
func (c *Core) RecordSequenceRead(sequenceID, subID int) error {
	_, err := c.db.Exec(`UPDATE sequence_contacts SET last_read_at = NOW() WHERE sequence_id = $1 AND subscriber_id = $2`, sequenceID, subID)
	return err
}

// RecordSequenceReadByPhone marks sequence contacts as read matching a phone number.
func (c *Core) RecordSequenceReadByPhone(phone string) error {
	if c == nil || c.db == nil {
		return nil
	}
	cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(phone, "")
	if cleaned == "" {
		return nil
	}
	_, err := c.db.Exec(`UPDATE sequence_contacts
		SET last_read_at = NOW()
		WHERE subscriber_id IN (
			SELECT id FROM subscribers
			WHERE REGEXP_REPLACE(phone, '[^\d]', '', 'g') = $1
			   OR REGEXP_REPLACE(attribs->>'phone', '[^\d]', '', 'g') = $1
		) AND status IN ('scheduled', 'in_progress')`, cleaned)
	return err
}

// RecordSequenceClick records a link click event for a sequence subscriber.
func (c *Core) RecordSequenceClick(sequenceID, subID int) error {
	if c == nil || c.db == nil {
		return nil
	}
	_, err := c.db.Exec(`UPDATE sequence_contacts SET last_clicked_at = NOW() WHERE sequence_id = $1 AND subscriber_id = $2`, sequenceID, subID)
	return err
}

// RecordSequenceClickByPhone marks sequence contacts as clicked matching a phone number.
func (c *Core) RecordSequenceClickByPhone(phone string) error {
	if c == nil || c.db == nil {
		return nil
	}
	cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(phone, "")
	if cleaned == "" {
		return nil
	}
	_, err := c.db.Exec(`UPDATE sequence_contacts
		SET last_clicked_at = NOW()
		WHERE subscriber_id IN (
			SELECT id FROM subscribers
			WHERE REGEXP_REPLACE(phone, '[^\d]', '', 'g') = $1
			   OR REGEXP_REPLACE(attribs->>'phone', '[^\d]', '', 'g') = $1
		) AND status IN ('scheduled', 'in_progress')`, cleaned)
	return err
}

// RecordSequenceReply marks subscriber status as 'replied' by email.
func (c *Core) RecordSequenceReply(email string) error {
	_, err := c.db.Exec(`UPDATE sequence_contacts
		SET status = 'replied'
		WHERE subscriber_id = (SELECT id FROM subscribers WHERE email = $1 LIMIT 1)
		  AND status IN ('scheduled', 'in_progress')`, email)
	return err
}

// RecordSequenceReplyByPhone marks subscriber sequence status as 'replied' by phone number.
func (c *Core) RecordSequenceReplyByPhone(phone string) error {
	cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(phone, "")
	if cleaned == "" {
		return nil
	}
	_, err := c.db.Exec(`UPDATE sequence_contacts
		SET status = 'replied'
		WHERE subscriber_id IN (
			SELECT id FROM subscribers
			WHERE REGEXP_REPLACE(phone, '[^\d]', '', 'g') = $1
			   OR REGEXP_REPLACE(attribs->>'phone', '[^\d]', '', 'g') = $1
		) AND status IN ('scheduled', 'in_progress')`, cleaned)
	return err
}

// CancelSequenceContactForOptOut cancels active sequence contacts and unsubscribes the subscriber upon explicit opt-out.
func (c *Core) CancelSequenceContactForOptOut(identifier string, isPhone bool) error {
	if identifier == "" {
		return nil
	}

	if isPhone {
		cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(identifier, "")
		if cleaned == "" {
			return nil
		}
		_, err := c.db.Exec(`UPDATE sequence_contacts
			SET status = 'cancelled'
			WHERE subscriber_id IN (
				SELECT id FROM subscribers
				WHERE REGEXP_REPLACE(phone, '[^\d]', '', 'g') = $1
				   OR REGEXP_REPLACE(attribs->>'phone', '[^\d]', '', 'g') = $1
			) AND status IN ('scheduled', 'in_progress')`, cleaned)
		if err != nil {
			return err
		}
		// Also mark subscriber status as unsubscribed
		_, err = c.db.Exec(`UPDATE subscribers SET status = 'unsubscribed'
			WHERE REGEXP_REPLACE(phone, '[^\d]', '', 'g') = $1
			   OR REGEXP_REPLACE(attribs->>'phone', '[^\d]', '', 'g') = $1`, cleaned)
		return err
	}

	_, err := c.db.Exec(`UPDATE sequence_contacts
		SET status = 'cancelled'
		WHERE subscriber_id = (SELECT id FROM subscribers WHERE email = $1 LIMIT 1)
		  AND status IN ('scheduled', 'in_progress')`, identifier)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(`UPDATE subscribers SET status = 'unsubscribed' WHERE email = $1`, identifier)
	return err
}

// DeferSequenceContactOOO defers active sequence contacts to a future return date for Out-Of-Office replies.
func (c *Core) DeferSequenceContactOOO(identifier string, isPhone bool, returnDate time.Time) error {
	if identifier == "" {
		return nil
	}

	if isPhone {
		cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(identifier, "")
		if cleaned == "" {
			return nil
		}
		_, err := c.db.Exec(`UPDATE sequence_contacts
			SET next_send_at = $2, status = 'in_progress'
			WHERE subscriber_id IN (
				SELECT id FROM subscribers
				WHERE REGEXP_REPLACE(phone, '[^\d]', '', 'g') = $1
				   OR REGEXP_REPLACE(attribs->>'phone', '[^\d]', '', 'g') = $1
			) AND status IN ('scheduled', 'in_progress')`, cleaned, returnDate)
		return err
	}

	_, err := c.db.Exec(`UPDATE sequence_contacts
		SET next_send_at = $2, status = 'in_progress'
		WHERE subscriber_id = (SELECT id FROM subscribers WHERE email = $1 LIMIT 1)
		  AND status IN ('scheduled', 'in_progress')`, identifier, returnDate)
	return err
}

// RecordSequenceStepHistory appends a step execution record to a subscriber's sequence_history attribute.
func (c *Core) RecordSequenceStepHistory(subID int, stepNumber int, messenger, subject, body string) error {
	historyRecord := map[string]any{
		"step_number": stepNumber,
		"step":        stepNumber,
		"messenger":   messenger,
		"subject":     subject,
		"content":     body,
		"message":     body,
		"sent_at":     time.Now().Format(time.RFC3339),
	}

	var rawAttribs []byte
	err := c.db.Get(&rawAttribs, `SELECT COALESCE(attribs, '{}'::jsonb) FROM subscribers WHERE id = $1`, subID)
	if err != nil {
		return err
	}

	var attribs map[string]any
	if err := json.Unmarshal(rawAttribs, &attribs); err != nil {
		attribs = make(map[string]any)
	}

	var historyList []any
	if existingHistory, ok := attribs["sequence_history"].([]any); ok {
		historyList = existingHistory
	}
	historyList = append(historyList, historyRecord)
	attribs["sequence_history"] = historyList

	updatedBytes, err := json.Marshal(attribs)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(`UPDATE subscribers SET attribs = $1::jsonb WHERE id = $2`, string(updatedBytes), subID)
	return err
}

// IsNationalHoliday returns true if date falls on standard national holidays.
func IsNationalHoliday(t time.Time) bool {
	month, day, weekday := t.Month(), t.Day(), t.Weekday()

	if month == time.January && day == 1 {
		return true
	}
	if month == time.July && day == 4 {
		return true
	}
	if month == time.December && (day == 24 || day == 25) {
		return true
	}
	if month == time.May && weekday == time.Monday && day >= 25 {
		return true
	}
	if month == time.September && weekday == time.Monday && day <= 7 {
		return true
	}
	if month == time.November && weekday == time.Thursday && day >= 22 && day <= 28 {
		return true
	}
	return false
}

// IsInsideSchedule checks if current time in target timezone falls within the active Schedule window.
func IsInsideSchedule(sched *models.Schedule, contactLoc *time.Location, now time.Time) (bool, time.Time) {
	if sched == nil {
		return true, now
	}

	loc := time.UTC
	if sched.UseContactTimezone && contactLoc != nil {
		loc = contactLoc
	} else if sched.Timezone != "" {
		if l, err := time.LoadLocation(sched.Timezone); err == nil {
			loc = l
		}
	}

	localNow := now.In(loc)
	if sched.SkipHolidays && IsNationalHoliday(localNow) {
		nextDay := localNow.AddDate(0, 0, 1)
		return false, time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 8, 0, 0, 0, loc)
	}

	dayKey := strings.ToLower(localNow.Format("mon"))

	var startStr, endStr string

	if len(sched.SendingWindows) > 0 {
		var raw map[string]interface{}
		if b, err := json.Marshal(sched.SendingWindows); err == nil {
			_ = json.Unmarshal(b, &raw)
		}

		if dayVal, exists := raw[dayKey]; exists && dayVal != nil {
			if m, ok := dayVal.(map[string]interface{}); ok {
				// dict of dict: {"mon": {"start": "08:00", "end": "17:00"}}
				if s, ok := m["start"].(string); ok {
					startStr = s
				}
				if e, ok := m["end"].(string); ok {
					endStr = e
				}
			} else if slice, ok := dayVal.([]interface{}); ok && len(slice) > 0 {
				// dict of array: {"mon": [{"start": "08:00", "end": "17:00"}]}
				if m, ok := slice[0].(map[string]interface{}); ok {
					if s, ok := m["start"].(string); ok {
						startStr = s
					}
					if e, ok := m["end"].(string); ok {
						endStr = e
					}
				}
			}
		}
	}

	if startStr == "" || endStr == "" {
		// No active window configured for today
		nextDay := localNow.AddDate(0, 0, 1)
		return false, time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 8, 0, 0, 0, loc)
	}

	startHour, startMin := 8, 0
	endHour, endMin := 17, 0
	fmt.Sscanf(startStr, "%d:%d", &startHour, &startMin)
	fmt.Sscanf(endStr, "%d:%d", &endHour, &endMin)

	startTimeToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), startHour, startMin, 0, 0, loc)
	endTimeToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), endHour, endMin, 0, 0, loc)

	if (!localNow.Before(startTimeToday)) && localNow.Before(endTimeToday) {
		return true, localNow
	}

	nextDay := localNow.AddDate(0, 0, 1)
	return false, time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 8, 0, 0, 0, loc)
}

// IsInsideSequenceSchedule checks if current time in loc is within active schedule window.
// Returns whether currently inside the schedule and next valid start time if outside.
func IsInsideSequenceSchedule(sched models.SequenceSchedule, loc *time.Location, now time.Time) (bool, time.Time) {
	if loc == nil {
		loc = time.UTC
	}
	if !sched.Enabled {
		return true, now
	}

	localNow := now.In(loc)
	dayName := strings.ToLower(localNow.Format("Mon"))

	dayValid := false
	if len(sched.Days) == 0 {
		dayValid = true
	} else {
		for _, d := range sched.Days {
			if strings.EqualFold(strings.TrimSpace(d), dayName) {
				dayValid = true
				break
			}
		}
	}

	startHour, startMin := 9, 0
	if sched.StartTime != "" {
		fmt.Sscanf(sched.StartTime, "%d:%d", &startHour, &startMin)
	}

	endHour, endMin := 17, 0
	if sched.EndTime != "" {
		fmt.Sscanf(sched.EndTime, "%d:%d", &endHour, &endMin)
	}

	startTimeToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), startHour, startMin, 0, 0, loc)
	endTimeToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), endHour, endMin, 0, 0, loc)

	if dayValid && !localNow.Before(startTimeToday) && localNow.Before(endTimeToday) {
		return true, localNow
	}

	nextStart := startTimeToday
	if !dayValid || !localNow.Before(startTimeToday) {
		nextStart = nextStart.AddDate(0, 0, 1)
	}

	if len(sched.Days) > 0 {
		for i := 0; i < 7; i++ {
			dName := strings.ToLower(nextStart.Format("Mon"))
			valid := false
			for _, d := range sched.Days {
				if strings.EqualFold(strings.TrimSpace(d), dName) {
					valid = true
					break
				}
			}
			if valid {
				break
			}
			nextStart = nextStart.AddDate(0, 0, 1)
		}
	}

	return false, nextStart
}

// CalculatePacedInterval calculates the interval spacing in seconds between messages
// given remaining schedule time and total pending contacts.
func CalculatePacedInterval(sched models.SequenceSchedule, totalContacts int, remainingSeconds int) int {
	if totalContacts <= 1 || remainingSeconds <= 0 {
		return 0
	}

	interval := remainingSeconds / totalContacts
	if sched.MinIntervalSeconds > 0 && interval < sched.MinIntervalSeconds {
		interval = sched.MinIntervalSeconds
	}
	return interval
}

// CalculatePacedScheduleTimestamps computes staggered next_send_at timestamps for a batch of contacts.
func CalculatePacedScheduleTimestamps(sched models.SequenceSchedule, loc *time.Location, now time.Time, totalContacts int) []time.Time {
	timestamps := make([]time.Time, totalContacts)
	if totalContacts == 0 {
		return timestamps
	}

	inside, nextStart := IsInsideSequenceSchedule(sched, loc, now)
	start := now
	if !inside {
		start = nextStart
	}

	localStart := start.In(loc)
	endHour, endMin := 17, 0
	if sched.EndTime != "" {
		fmt.Sscanf(sched.EndTime, "%d:%d", &endHour, &endMin)
	}
	endTimeToday := time.Date(localStart.Year(), localStart.Month(), localStart.Day(), endHour, endMin, 0, 0, loc)

	remainingSec := int(endTimeToday.Sub(localStart).Seconds())
	if remainingSec < 0 {
		remainingSec = 0
	}

	interval := CalculatePacedInterval(sched, totalContacts, remainingSec)

	jitterMax := sched.JitterSeconds
	if sched.JitterSeconds < 0 {
		jitterMax = 0
	} else if jitterMax == 0 && interval > 0 {
		jitterMax = interval / 5
		if jitterMax > 300 {
			jitterMax = 300
		}
	}

	for i := 0; i < totalContacts; i++ {
		st := localStart.Add(time.Duration(i*interval) * time.Second)
		if jitterMax > 0 && interval > 0 {
			jitter := rand.Intn(jitterMax*2+1) - jitterMax
			st = st.Add(time.Duration(jitter) * time.Second)
		}
		timestamps[i] = st.UTC()
	}

	return timestamps
}

// GetSequenceAnalytics calculates real metrics across sequence contacts and sequence steps.
func (c *Core) GetSequenceAnalytics() (*models.SequenceAnalytics, error) {
	out := &models.SequenceAnalytics{
		Funnel: []models.SequenceStepFunnel{},
		AggregatedAnalytics: models.CampaignAnalytics{
			Breakdowns: models.CampaignBreakdownStats{
				Devices:   []models.DeviceBreakdown{},
				Locations: []models.LocationBreakdown{},
				Links:     []models.CampaignAnalyticsLink{},
				Variants:  []models.VariantPerformance{},
				Bots: models.CampaignBotStats{
					BotTypeBreakdown: make(map[string]int),
				},
			},
		},
	}

	// 1. Query active contacts (status 'scheduled' or 'in_progress')
	err := c.db.Get(&out.ActiveContacts, `SELECT COALESCE(COUNT(*), 0) FROM sequence_contacts WHERE status IN ('scheduled', 'in_progress')`)
	if err != nil && err != sql.ErrNoRows {
		c.log.Printf("error querying active sequence contacts: %v", err)
	}

	// 2. Query total step completions
	err = c.db.Get(&out.StepCompletions, `SELECT COALESCE(SUM(current_step), 0) FROM sequence_contacts`)
	if err != nil && err != sql.ErrNoRows {
		c.log.Printf("error querying sequence step completions: %v", err)
	}

	// 3. Query total enrolled contacts, replied contacts, and finished contacts
	var totalEnrolled, totalReplied, totalFinished int
	_ = c.db.Get(&totalEnrolled, `SELECT COALESCE(COUNT(*), 0) FROM sequence_contacts`)
	_ = c.db.Get(&totalReplied, `SELECT COALESCE(COUNT(*), 0) FROM sequence_contacts WHERE status = 'replied'`)
	_ = c.db.Get(&totalFinished, `SELECT COALESCE(COUNT(*), 0) FROM sequence_contacts WHERE status IN ('replied', 'finished')`)

	if totalEnrolled > 0 {
		out.ReplyRate = (float64(totalReplied) / float64(totalEnrolled)) * 100.0
		out.ConversionRate = (float64(totalFinished) / float64(totalEnrolled)) * 100.0
	}

	// 4. Query Aggregated Views across all sequence views
	viewRow := c.db.QueryRowx(`
		SELECT
			COALESCE(COUNT(*), 0) AS total,
			COALESCE(COUNT(DISTINCT subscriber_id), 0) AS unique_views,
			COALESCE(COUNT(*) FILTER (WHERE is_bot = FALSE), 0) AS human_total,
			COALESCE(COUNT(DISTINCT subscriber_id) FILTER (WHERE is_bot = FALSE), 0) AS human_unique,
			COALESCE(COUNT(*) FILTER (WHERE is_bot = TRUE), 0) AS bot_total,
			COALESCE(COUNT(*) FILTER (WHERE is_proxy = TRUE), 0) AS proxy_mpp_total
		FROM campaign_views
		WHERE sequence_step_id IS NOT NULL`)
	_ = viewRow.Scan(
		&out.AggregatedAnalytics.Views.Total,
		&out.AggregatedAnalytics.Views.Unique,
		&out.AggregatedAnalytics.Views.HumanTotal,
		&out.AggregatedAnalytics.Views.HumanUnique,
		&out.AggregatedAnalytics.Views.BotTotal,
		&out.AggregatedAnalytics.Views.ProxyMPPTotal,
	)

	// 5. Query Aggregated Clicks across all sequence clicks
	clickRow := c.db.QueryRowx(`
		SELECT
			COALESCE(COUNT(*), 0) AS total,
			COALESCE(COUNT(DISTINCT subscriber_id), 0) AS unique_clicks,
			COALESCE(COUNT(*) FILTER (WHERE is_bot = FALSE), 0) AS human_total,
			COALESCE(COUNT(DISTINCT subscriber_id) FILTER (WHERE is_bot = FALSE), 0) AS human_unique,
			COALESCE(COUNT(*) FILTER (WHERE is_bot = TRUE), 0) AS bot_clicks
		FROM link_clicks
		WHERE sequence_step_id IS NOT NULL`)
	_ = clickRow.Scan(
		&out.AggregatedAnalytics.Clicks.Total,
		&out.AggregatedAnalytics.Clicks.Unique,
		&out.AggregatedAnalytics.Clicks.HumanTotal,
		&out.AggregatedAnalytics.Clicks.HumanUnique,
		&out.AggregatedAnalytics.Clicks.BotClicks,
	)

	if out.AggregatedAnalytics.Views.HumanUnique > 0 {
		out.AggregatedAnalytics.Clicks.CTOR = (float64(out.AggregatedAnalytics.Clicks.HumanUnique) / float64(out.AggregatedAnalytics.Views.HumanUnique)) * 100.0
	}

	// 6. Query funnel steps with step-level analytics
	rows, err := c.db.Queryx(`
		SELECT
			st.id,
			st.step_number,
			COALESCE(st.subject, '') AS subject,
			COALESCE(st.messenger, 'email') AS messenger,
			COALESCE((SELECT COUNT(*) FROM sequence_contacts sc WHERE sc.sequence_id = st.sequence_id AND sc.current_step >= st.step_number), 0) AS reached,
			COALESCE((SELECT COUNT(*) FROM sequence_contacts sc WHERE sc.sequence_id = st.sequence_id AND sc.current_step = st.step_number AND sc.status = 'replied'), 0) AS replied
		FROM sequence_steps st
		ORDER BY st.sequence_id ASC, st.step_number ASC
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var (
				stepID int
				f      models.SequenceStepFunnel
			)
			f.Analytics = models.CampaignAnalytics{
				Breakdowns: models.CampaignBreakdownStats{
					Bots: models.CampaignBotStats{
						BotTypeBreakdown: make(map[string]int),
					},
				},
			}
			if err := rows.Scan(&stepID, &f.StepNumber, &f.Subject, &f.Messenger, &f.Reached, &f.Replied); err == nil {
				// Query step-level views and clicks
				_ = c.db.QueryRowx(`
					SELECT
						COALESCE(COUNT(*), 0) AS total,
						COALESCE(COUNT(DISTINCT subscriber_id) FILTER (WHERE is_bot = FALSE), 0) AS human_unique
					FROM campaign_views WHERE sequence_step_id = $1`, stepID).Scan(
					&f.Analytics.Views.Total,
					&f.Analytics.Views.HumanUnique,
				)

				_ = c.db.QueryRowx(`
					SELECT
						COALESCE(COUNT(*), 0) AS total,
						COALESCE(COUNT(DISTINCT subscriber_id) FILTER (WHERE is_bot = FALSE), 0) AS human_unique
					FROM link_clicks WHERE sequence_step_id = $1`, stepID).Scan(
					&f.Analytics.Clicks.Total,
					&f.Analytics.Clicks.HumanUnique,
				)

				out.Funnel = append(out.Funnel, f)
			}
		}
	}

	return out, nil
}
