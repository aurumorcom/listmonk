package core

import (
	"database/sql"
	"net/http"

	"github.com/gofrs/uuid/v5"
	"github.com/knadh/listmonk/internal/media"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	null "gopkg.in/volatiletech/null.v6"
)

// GetSequences returns a list of all sequences.
func (c *Core) GetSequences() ([]models.Sequence, error) {
	var out []models.Sequence
	err := c.db.Select(&out, "SELECT id, uuid, name, status, send_window, mailbox_ids, waha_sessions, load_balance_mode, created_at, updated_at FROM sequences ORDER BY id DESC")
	if err != nil {
		c.log.Printf("error querying sequences: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return out, nil
}

// GetSequence returns a sequence by ID or UUID.
func (c *Core) GetSequence(id int, uid string) (*models.Sequence, error) {
	var seq models.Sequence
	err := c.db.Get(&seq, "SELECT id, uuid, name, status, send_window, mailbox_ids, waha_sessions, load_balance_mode, created_at, updated_at FROM sequences WHERE id = $1 OR uuid::text = $2", id, uid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, echo.NewHTTPError(http.StatusNotFound, c.i18n.Ts("globals.messages.notFound"))
		}
		c.log.Printf("error getting sequence: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return &seq, nil
}

// CreateSequence creates a new sequence.
func (c *Core) CreateSequence(seq models.Sequence) (*models.Sequence, error) {
	if seq.UUID == "" {
		seq.UUID = uuid.Must(uuid.NewV4()).String()
	}
	if seq.Status == "" {
		seq.Status = models.SequenceStatusActive
	}
	if seq.SendWindow == nil {
		seq.SendWindow = models.JSON{}
	}
	if seq.LoadBalanceMode == "" {
		seq.LoadBalanceMode = models.LoadBalanceModeRoundRobin
	}

	var out models.Sequence
	err := c.db.Get(&out, `INSERT INTO sequences (uuid, name, status, send_window, mailbox_ids, waha_sessions, load_balance_mode)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, uuid, name, status, send_window, mailbox_ids, waha_sessions, load_balance_mode, created_at, updated_at`,
		seq.UUID, seq.Name, seq.Status, seq.SendWindow, seq.MailboxIDs, seq.WahaSessions, seq.LoadBalanceMode)
	if err != nil {
		c.log.Printf("error creating sequence: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return &out, nil
}

// UpdateSequence updates an existing sequence.
func (c *Core) UpdateSequence(seq models.Sequence) (*models.Sequence, error) {
	if seq.LoadBalanceMode == "" {
		seq.LoadBalanceMode = models.LoadBalanceModeRoundRobin
	}
	_, err := c.db.Exec(`UPDATE sequences
		SET name = $2, status = $3, send_window = $4, mailbox_ids = $5, waha_sessions = $6, load_balance_mode = $7, updated_at = NOW()
		WHERE id = $1`,
		seq.ID, seq.Name, seq.Status, seq.SendWindow, seq.MailboxIDs, seq.WahaSessions, seq.LoadBalanceMode)
	if err != nil {
		c.log.Printf("error updating sequence: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return c.GetSequence(seq.ID, "")
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

// GetSequenceSteps returns the steps for a given sequence.
func (c *Core) GetSequenceSteps(sequenceID int) ([]models.SequenceStep, error) {
	var steps []models.SequenceStep
	query := `SELECT
		s.id, s.sequence_id, s.step_number, s.delay_days, s.messenger, s.condition,
		s.subject, s.body, s.template_id,
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
		if s.Messenger == "" {
			s.Messenger = "email"
		}
		if s.Condition == "" {
			s.Condition = models.SequenceConditionAlways
		}

		var newID int
		err := tx.Get(&newID, `INSERT INTO sequence_steps (sequence_id, step_number, delay_days, messenger, condition, subject, body, template_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
			s.SequenceID, s.StepNumber, s.DelayDays, s.Messenger, s.Condition, s.Subject, s.Body, s.TemplateID)
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

// AllocateSendersRoundRobinInt distributes subscriber IDs round-robin across an integer pool (e.g. mailbox IDs).
func AllocateSendersRoundRobinInt(subIDs []int, pool []int64) map[int]null.Int {
	alloc := make(map[int]null.Int, len(subIDs))
	if len(pool) == 0 {
		for _, id := range subIDs {
			alloc[id] = null.Int{}
		}
		return alloc
	}
	for i, subID := range subIDs {
		alloc[subID] = null.IntFrom(pool[i%len(pool)])
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

// AllocateSendersCapacityWeighted distributes subscriber IDs based on remaining mailbox daily capacity.
func AllocateSendersCapacityWeighted(subIDs []int, mailboxes []models.Mailbox) map[int]null.Int {
	alloc := make(map[int]null.Int, len(subIDs))
	if len(mailboxes) == 0 {
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
	for _, m := range mailboxes {
		rem := m.DailyLimit - m.SentToday
		if rem < 0 {
			rem = 0
		}
		if rem > 0 {
			active = append(active, capBox{id: int64(m.ID), remaining: rem})
			totalRemaining += rem
		}
	}

	if totalRemaining == 0 || len(active) == 0 {
		// Fallback to round-robin if no mailbox has remaining capacity
		var pool []int64
		for _, m := range mailboxes {
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
			alloc[subIDs[subIdx]] = null.IntFrom(b.id)
			subIdx++
		}
	}

	// Fallback for any leftover subscriber IDs
	for subIdx < len(subIDs) {
		alloc[subIDs[subIdx]] = null.IntFrom(active[subIdx%len(active)].id)
		subIdx++
	}

	return alloc
}

// EnrollSequenceContacts enrolls contacts into a sequence with sender locking and load balancing.
func (c *Core) EnrollSequenceContacts(sequenceID int, subscriberIDs []int, explicitMailboxID null.Int, explicitWahaSession null.String) error {
	if len(subscriberIDs) == 0 {
		return nil
	}

	seq, err := c.GetSequence(sequenceID, "")
	if err != nil {
		return err
	}

	mailboxAlloc := make(map[int]null.Int)
	wahaAlloc := make(map[int]null.String)

	// Email Mailbox Allocation
	if explicitMailboxID.Valid {
		for _, id := range subscriberIDs {
			mailboxAlloc[id] = explicitMailboxID
		}
	} else if len(seq.MailboxIDs) > 0 {
		if seq.LoadBalanceMode == models.LoadBalanceModeCapacityWeighted {
			mailboxes, err := c.GetMailboxes()
			if err == nil && len(mailboxes) > 0 {
				var pool []models.Mailbox
				poolMap := make(map[int]bool)
				for _, mid := range seq.MailboxIDs {
					poolMap[int(mid)] = true
				}
				for _, m := range mailboxes {
					if poolMap[m.ID] {
						pool = append(pool, m)
					}
				}
				mailboxAlloc = AllocateSendersCapacityWeighted(subscriberIDs, pool)
			} else {
				mailboxAlloc = AllocateSendersRoundRobinInt(subscriberIDs, seq.MailboxIDs)
			}
		} else {
			mailboxAlloc = AllocateSendersRoundRobinInt(subscriberIDs, seq.MailboxIDs)
		}
	}

	// WAHA Session Allocation
	if explicitWahaSession.Valid && explicitWahaSession.String != "" {
		for _, id := range subscriberIDs {
			wahaAlloc[id] = explicitWahaSession
		}
	} else if len(seq.WahaSessions) > 0 {
		wahaAlloc = AllocateSendersRoundRobinString(subscriberIDs, seq.WahaSessions)
	}

	tx, err := c.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Preparex(`INSERT INTO sequence_contacts (sequence_id, subscriber_id, mailbox_id, waha_session, status, current_step, next_send_at)
		VALUES ($1, $2, $3, $4, 'scheduled', 1, NOW())
		ON CONFLICT (sequence_id, subscriber_id) DO UPDATE SET mailbox_id = EXCLUDED.mailbox_id, waha_session = EXCLUDED.waha_session`)
	if err != nil {
		c.log.Printf("error preparing sequence enrollment stmt: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	defer stmt.Close()

	for _, subID := range subscriberIDs {
		mbID := mailboxAlloc[subID]
		wSession := wahaAlloc[subID]

		var mbVal any
		if mbID.Valid {
			mbVal = mbID.Int64
		}
		var wsVal any
		if wSession.Valid && wSession.String != "" {
			wsVal = wSession.String
		}

		if _, err := stmt.Exec(sequenceID, subID, mbVal, wsVal); err != nil {
			c.log.Printf("error enrolling subscriber %d into sequence %d: %v", subID, sequenceID, err)
			return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
		}
	}

	return tx.Commit()
}

// ReassignSequenceContactSender updates the mailbox or WAHA session locked to a sequence contact.
func (c *Core) ReassignSequenceContactSender(sequenceID, subID int, mailboxID null.Int, wahaSession null.String) error {
	var mbVal any
	if mailboxID.Valid {
		mbVal = mailboxID.Int64
	}
	var wsVal any
	if wahaSession.Valid && wahaSession.String != "" {
		wsVal = wahaSession.String
	}

	_, err := c.db.Exec(`UPDATE sequence_contacts
		SET mailbox_id = $3, waha_session = $4
		WHERE sequence_id = $1 AND subscriber_id = $2`,
		sequenceID, subID, mbVal, wsVal)
	if err != nil {
		c.log.Printf("error reassigning sequence contact sender: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return nil
}

// GetDueSequenceContacts returns sequence contacts due for sending.
func (c *Core) GetDueSequenceContacts(limit int) ([]models.SequenceContact, error) {
	var out []models.SequenceContact
	err := c.db.Select(&out, `SELECT sequence_id, subscriber_id, mailbox_id, waha_session, status, current_step, next_send_at, last_read_at, last_clicked_at, last_message_id, created_at
		FROM sequence_contacts
		WHERE status IN ('scheduled', 'in_progress') AND next_send_at <= NOW()
		LIMIT $1`, limit)
	if err != nil {
		c.log.Printf("error getting due sequence contacts: %v", err)
		return nil, err
	}
	return out, nil
}

// UpdateSequenceContactStatus updates progress of a contact in a sequence.
func (c *Core) UpdateSequenceContactStatus(sequenceID, subID int, status string, currentStep int, nextSendAt null.Time, lastMsgID null.String) error {
	_, err := c.db.Exec(`UPDATE sequence_contacts
		SET status = $3, current_step = $4, next_send_at = $5, last_message_id = $6
		WHERE sequence_id = $1 AND subscriber_id = $2`,
		sequenceID, subID, status, currentStep, nextSendAt, lastMsgID)
	return err
}

// RecordSequenceRead records an open/read event for a sequence subscriber.
func (c *Core) RecordSequenceRead(sequenceID, subID int) error {
	_, err := c.db.Exec(`UPDATE sequence_contacts SET last_read_at = NOW() WHERE sequence_id = $1 AND subscriber_id = $2`, sequenceID, subID)
	return err
}

// RecordSequenceClick records a link click event for a sequence subscriber.
func (c *Core) RecordSequenceClick(sequenceID, subID int) error {
	_, err := c.db.Exec(`UPDATE sequence_contacts SET last_clicked_at = NOW() WHERE sequence_id = $1 AND subscriber_id = $2`, sequenceID, subID)
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
