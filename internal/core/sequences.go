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
	null "gopkg.in/volatiletech/null.v6"
)

// GetSequences returns a list of all sequences.
func (c *Core) GetSequences() ([]models.Sequence, error) {
	var out []models.Sequence
	err := c.db.Select(&out, "SELECT id, uuid, name, status, schedule_id, send_window, email_ids, waha_sessions, load_balance_mode, created_at, updated_at FROM sequences ORDER BY id DESC")
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
	err := c.db.Get(&seq, "SELECT id, uuid, name, status, schedule_id, send_window, email_ids, waha_sessions, load_balance_mode, created_at, updated_at FROM sequences WHERE id = $1 OR uuid::text = $2", id, uid)
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
	err := c.db.Get(&out, `INSERT INTO sequences (uuid, name, status, schedule_id, send_window, email_ids, waha_sessions, load_balance_mode)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, uuid, name, status, schedule_id, send_window, email_ids, waha_sessions, load_balance_mode, created_at, updated_at`,
		seq.UUID, seq.Name, seq.Status, seq.ScheduleID, seq.SendWindow, seq.EmailIDs, seq.WahaSessions, seq.LoadBalanceMode)
	if err != nil {
		c.log.Printf("error creating sequence: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return c.GetSequence(out.ID, "")
}

// UpdateSequence updates an existing sequence.
func (c *Core) UpdateSequence(seq models.Sequence) (*models.Sequence, error) {
	if seq.LoadBalanceMode == "" {
		seq.LoadBalanceMode = models.LoadBalanceModeRoundRobin
	}
	_, err := c.db.Exec(`UPDATE sequences
		SET name = $2, status = $3, schedule_id = $4, send_window = $5, email_ids = $6, waha_sessions = $7, load_balance_mode = $8, updated_at = NOW()
		WHERE id = $1`,
		seq.ID, seq.Name, seq.Status, seq.ScheduleID, seq.SendWindow, seq.EmailIDs, seq.WahaSessions, seq.LoadBalanceMode)
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
		err := tx.Get(&newID, `INSERT INTO sequence_steps (sequence_id, step_number, delay_days, messenger, condition, subject, body, email_type, template_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
			s.SequenceID, s.StepNumber, s.DelayDays, s.Messenger, s.Condition, s.Subject, s.Body, s.EmailType, s.TemplateID)
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
		if m.EmailsPerDay > 0 {
			rem = m.EmailsPerDay - m.EmailsToday
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

// EnrollSequenceContacts enrolls contacts into a sequence with sender locking, load balancing, and optional User context.
func (c *Core) EnrollSequenceContacts(sequenceID int, subscriberIDs []int, explicitMailboxID null.Int, explicitWahaSession null.String, userContext map[string]any) error {
	if len(subscriberIDs) == 0 {
		return nil
	}

	seq, err := c.GetSequence(sequenceID, "")
	if err != nil {
		return err
	}

	mailboxAlloc := make(map[int]null.Int)
	wahaAlloc := make(map[int]null.String)

	// Check for User identity in userContext to bind User channels across Email and WAHA
	if len(userContext) > 0 {
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
				if u.EmailID.Valid && !explicitMailboxID.Valid {
					explicitMailboxID = u.EmailID
				}
				if u.WahaSession.Valid && u.WahaSession.String != "" && (!explicitWahaSession.Valid || explicitWahaSession.String == "") {
					explicitWahaSession = u.WahaSession
				}
			}
			// Fallback: look up email account owned by this user_id if email_id not set on user profile
			if !explicitMailboxID.Valid {
				var mbID int
				if err := c.db.Get(&mbID, "SELECT id FROM emails WHERE user_id = $1 ORDER BY id ASC LIMIT 1", uid); err == nil && mbID > 0 {
					explicitMailboxID = null.IntFrom(mbID)
				}
			}
		}
	}

	// Email Allocation
	if explicitMailboxID.Valid {
		for _, id := range subscriberIDs {
			mailboxAlloc[id] = explicitMailboxID
		}
	} else if len(seq.EmailIDs) > 0 {
		if seq.LoadBalanceMode == models.LoadBalanceModeCapacityWeighted {
			emails, err := c.GetEmails()
			if err == nil && len(emails) > 0 {
				var pool []models.Email
				poolMap := make(map[int]bool)
				for _, mid := range seq.EmailIDs {
					poolMap[int(mid)] = true
				}
				for _, m := range emails {
					if poolMap[m.ID] {
						pool = append(pool, m)
					}
				}
				mailboxAlloc = AllocateSendersCapacityWeighted(subscriberIDs, pool)
			} else {
				mailboxAlloc = AllocateSendersRoundRobinInt(subscriberIDs, seq.EmailIDs)
			}
		} else {
			mailboxAlloc = AllocateSendersRoundRobinInt(subscriberIDs, seq.EmailIDs)
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

	stmt, err := tx.Preparex(`INSERT INTO sequence_contacts (sequence_id, subscriber_id, email_id, waha_session, status, current_step, next_send_at)
		VALUES ($1, $2, $3, $4, 'scheduled', 1, NOW())
		ON CONFLICT (sequence_id, subscriber_id) DO UPDATE SET email_id = EXCLUDED.email_id, waha_session = EXCLUDED.waha_session`)
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
			mbVal = mbID.Int
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

// ReassignSequenceContactSender updates the email account or WAHA session locked to a sequence contact.
func (c *Core) ReassignSequenceContactSender(sequenceID, subID int, emailID null.Int, wahaSession null.String) error {
	var mbVal any
	if emailID.Valid {
		mbVal = emailID.Int
	}
	var wsVal any
	if wahaSession.Valid && wahaSession.String != "" {
		wsVal = wahaSession.String
	}

	_, err := c.db.Exec(`UPDATE sequence_contacts
		SET email_id = $3, waha_session = $4
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
	err := c.db.Select(&out, `SELECT sequence_id, subscriber_id, email_id, waha_session, status, current_step, next_send_at, last_read_at, last_clicked_at, last_message_id, last_thread_msg_id, created_at
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

	var windows map[string][]models.TimeBlock
	if len(sched.SendingWindows) > 0 {
		if b, err := json.Marshal(sched.SendingWindows); err == nil {
			_ = json.Unmarshal(b, &windows)
		}
	}

	blocks, exists := windows[dayKey]
	if !exists || len(blocks) == 0 {
		// No blocks configured for today
		nextDay := localNow.AddDate(0, 0, 1)
		return false, time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 8, 0, 0, 0, loc)
	}

	for _, block := range blocks {
		startHour, startMin := 8, 0
		endHour, endMin := 17, 0
		fmt.Sscanf(block.Start, "%d:%d", &startHour, &startMin)
		fmt.Sscanf(block.End, "%d:%d", &endHour, &endMin)

		startTimeToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), startHour, startMin, 0, 0, loc)
		endTimeToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), endHour, endMin, 0, 0, loc)

		if (!localNow.Before(startTimeToday)) && localNow.Before(endTimeToday) {
			return true, localNow
		}
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

	// 4. Query funnel steps
	rows, err := c.db.Queryx(`
		SELECT 
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
			var f models.SequenceStepFunnel
			if err := rows.Scan(&f.StepNumber, &f.Subject, &f.Messenger, &f.Reached, &f.Replied); err == nil {
				out.Funnel = append(out.Funnel, f)
			}
		}
	}

	return out, nil
}
