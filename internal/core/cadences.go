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

// GetCadences returns a list of all cadences.
func (c *Core) GetCadences() ([]models.Cadence, error) {
	var out []models.Cadence
	err := c.db.Select(&out, "SELECT id, uuid, name, status, send_window, created_at, updated_at FROM cadences ORDER BY id DESC")
	if err != nil {
		c.log.Printf("error querying cadences: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return out, nil
}

// GetCadence returns a cadence by ID or UUID.
func (c *Core) GetCadence(id int, uid string) (*models.Cadence, error) {
	var cad models.Cadence
	err := c.db.Get(&cad, "SELECT id, uuid, name, status, send_window, created_at, updated_at FROM cadences WHERE id = $1 OR uuid::text = $2", id, uid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, echo.NewHTTPError(http.StatusNotFound, c.i18n.Ts("globals.messages.notFound"))
		}
		c.log.Printf("error getting cadence: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return &cad, nil
}

// CreateCadence creates a new cadence.
func (c *Core) CreateCadence(cad models.Cadence) (*models.Cadence, error) {
	if cad.UUID == "" {
		cad.UUID = uuid.Must(uuid.NewV4()).String()
	}
	if cad.Status == "" {
		cad.Status = models.CadenceStatusActive
	}
	if cad.SendWindow == nil {
		cad.SendWindow = models.JSON{}
	}

	var out models.Cadence
	err := c.db.Get(&out, "INSERT INTO cadences (uuid, name, status, send_window) VALUES ($1, $2, $3, $4) RETURNING id, uuid, name, status, send_window, created_at, updated_at",
		cad.UUID, cad.Name, cad.Status, cad.SendWindow)
	if err != nil {
		c.log.Printf("error creating cadence: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return &out, nil
}

// UpdateCadence updates an existing cadence.
func (c *Core) UpdateCadence(cad models.Cadence) (*models.Cadence, error) {
	_, err := c.db.Exec("UPDATE cadences SET name = $2, status = $3, send_window = $4, updated_at = NOW() WHERE id = $1",
		cad.ID, cad.Name, cad.Status, cad.SendWindow)
	if err != nil {
		c.log.Printf("error updating cadence: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return c.GetCadence(cad.ID, "")
}

// DeleteCadence deletes a cadence.
func (c *Core) DeleteCadence(id int) error {
	_, err := c.db.Exec("DELETE FROM cadences WHERE id = $1", id)
	if err != nil {
		c.log.Printf("error deleting cadence: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return nil
}

// GetCadenceSteps returns the steps for a given cadence.
func (c *Core) GetCadenceSteps(cadenceID int) ([]models.CadenceStep, error) {
	var steps []models.CadenceStep
	query := `SELECT 
		s.id, s.cadence_id, s.step_number, s.delay_days, s.messenger, s.condition, 
		s.subject, s.body, s.template_id,
		COALESCE(ARRAY_AGG(m.media_id) FILTER (WHERE m.media_id IS NOT NULL), '{}') AS media_ids
	FROM cadence_steps s
	LEFT JOIN cadence_step_media m ON s.id = m.cadence_step_id
	WHERE s.cadence_id = $1
	GROUP BY s.id
	ORDER BY s.step_number ASC`
	err := c.db.Select(&steps, query, cadenceID)
	if err != nil {
		c.log.Printf("error getting cadence steps: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return steps, nil
}

// SaveCadenceSteps updates or inserts steps for a cadence.
func (c *Core) SaveCadenceSteps(cadenceID int, steps []models.CadenceStep) error {
	tx, err := c.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM cadence_steps WHERE cadence_id = $1", cadenceID); err != nil {
		return err
	}

	for i, s := range steps {
		s.CadenceID = cadenceID
		s.StepNumber = i + 1
		if s.Messenger == "" {
			s.Messenger = "email"
		}
		if s.Condition == "" {
			s.Condition = models.CadenceConditionAlways
		}

		var newID int
		err := tx.Get(&newID, `INSERT INTO cadence_steps (cadence_id, step_number, delay_days, messenger, condition, subject, body, template_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
			s.CadenceID, s.StepNumber, s.DelayDays, s.Messenger, s.Condition, s.Subject, s.Body, s.TemplateID)
		if err != nil {
			return err
		}

		if len(s.MediaIDs) > 0 {
			_, err = tx.Exec(`INSERT INTO cadence_step_media (cadence_step_id, media_id, filename)
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

// EnrollCadenceSubscribers enrolls subscribers into a cadence.
func (c *Core) EnrollCadenceSubscribers(cadenceID int, subscriberIDs []int) error {
	if len(subscriberIDs) == 0 {
		return nil
	}

	query := `INSERT INTO cadence_contacts (cadence_id, subscriber_id, status, current_step, next_send_at)
		SELECT $1, unnest($2::int[]), 'scheduled', 1, NOW()
		ON CONFLICT (cadence_id, subscriber_id) DO NOTHING`

	_, err := c.db.Exec(query, cadenceID, subscriberIDs)
	if err != nil {
		c.log.Printf("error enrolling cadence subscribers: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return nil
}

// GetDueCadenceSubscribers returns cadence subscribers due for sending.
func (c *Core) GetDueCadenceSubscribers(limit int) ([]models.CadenceSubscriber, error) {
	var out []models.CadenceSubscriber
	err := c.db.Select(&out, `SELECT cadence_id, subscriber_id, status, current_step, next_send_at, last_read_at, last_clicked_at, last_message_id, created_at
		FROM cadence_contacts
		WHERE status IN ('scheduled', 'in_progress') AND next_send_at <= NOW()
		LIMIT $1`, limit)
	if err != nil {
		c.log.Printf("error getting due cadence subscribers: %v", err)
		return nil, err
	}
	return out, nil
}

// UpdateCadenceSubscriberStatus updates progress of a subscriber in a cadence.
func (c *Core) UpdateCadenceSubscriberStatus(cadenceID, subID int, status string, currentStep int, nextSendAt null.Time, lastMsgID null.String) error {
	_, err := c.db.Exec(`UPDATE cadence_contacts
		SET status = $3, current_step = $4, next_send_at = $5, last_message_id = $6
		WHERE cadence_id = $1 AND subscriber_id = $2`,
		cadenceID, subID, status, currentStep, nextSendAt, lastMsgID)
	return err
}

// RecordCadenceRead records an open/read event for a cadence subscriber.
func (c *Core) RecordCadenceRead(cadenceID, subID int) error {
	_, err := c.db.Exec(`UPDATE cadence_contacts SET last_read_at = NOW() WHERE cadence_id = $1 AND subscriber_id = $2`, cadenceID, subID)
	return err
}

// RecordCadenceClick records a link click event for a cadence subscriber.
func (c *Core) RecordCadenceClick(cadenceID, subID int) error {
	_, err := c.db.Exec(`UPDATE cadence_contacts SET last_clicked_at = NOW() WHERE cadence_id = $1 AND subscriber_id = $2`, cadenceID, subID)
	return err
}

// RecordCadenceReply marks subscriber status as 'replied' by email.
func (c *Core) RecordCadenceReply(email string) error {
	_, err := c.db.Exec(`UPDATE cadence_contacts
		SET status = 'replied'
		WHERE subscriber_id = (SELECT id FROM subscribers WHERE email = $1 LIMIT 1)
		  AND status IN ('scheduled', 'in_progress')`, email)
	return err
}
