package core

import (
	"database/sql"
	"net/http"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

// GetEmails returns a list of all sending email accounts.
func (c *Core) GetEmails() ([]models.Email, error) {
	var out []models.Email
	err := c.db.Select(&out, "SELECT id, name, email, smtp_config, imap_config, max_send_per_day, sent_today, user_id, signature, created_at, updated_at FROM emails ORDER BY id DESC")
	if err != nil {
		c.log.Printf("error querying emails: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return out, nil
}

// GetEmail returns an email account by ID.
func (c *Core) GetEmail(id int) (*models.Email, error) {
	var m models.Email
	err := c.db.Get(&m, "SELECT id, name, email, smtp_config, imap_config, max_send_per_day, sent_today, user_id, signature, created_at, updated_at FROM emails WHERE id = $1", id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, echo.NewHTTPError(http.StatusNotFound, c.i18n.Ts("globals.messages.notFound"))
		}
		c.log.Printf("error getting email account: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return &m, nil
}

// GetEmailsByUserID returns all email accounts assigned to a user ID.
func (c *Core) GetEmailsByUserID(userID int) ([]models.Email, error) {
	var out []models.Email
	err := c.db.Select(&out, "SELECT id, name, email, smtp_config, imap_config, max_send_per_day, sent_today, user_id, signature, created_at, updated_at FROM emails WHERE user_id = $1 ORDER BY id DESC", userID)
	if err != nil {
		c.log.Printf("error querying emails by user_id %d: %v", userID, err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return out, nil
}

// CreateEmail creates a new sending email account.
func (c *Core) CreateEmail(m models.Email) (*models.Email, error) {
	if m.SMTPConfig == nil {
		m.SMTPConfig = models.JSON{}
	}
	if m.IMAPConfig == nil {
		m.IMAPConfig = models.JSON{}
	}

	var uVal any
	if m.UserID.Valid && m.UserID.Int > 0 {
		uVal = m.UserID.Int
	}

	var out models.Email
	err := c.db.Get(&out, `INSERT INTO emails (name, email, smtp_config, imap_config, max_send_per_day, sent_today, user_id, signature)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, name, email, smtp_config, imap_config, max_send_per_day, sent_today, user_id, signature, created_at, updated_at`,
		m.Name, m.Email, m.SMTPConfig, m.IMAPConfig, m.MaxSendPerDay, 0, uVal, m.Signature)
	if err != nil {
		c.log.Printf("error creating email: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return &out, nil
}

// UpdateEmail updates an existing email account.
func (c *Core) UpdateEmail(m models.Email) (*models.Email, error) {
	var uVal any
	if m.UserID.Valid && m.UserID.Int > 0 {
		uVal = m.UserID.Int
	}

	_, err := c.db.Exec(`UPDATE emails SET name = $2, email = $3, smtp_config = $4, imap_config = $5, max_send_per_day = $6, user_id = $7, signature = $8, updated_at = NOW() WHERE id = $1`,
		m.ID, m.Name, m.Email, m.SMTPConfig, m.IMAPConfig, m.MaxSendPerDay, uVal, m.Signature)
	if err != nil {
		c.log.Printf("error updating email: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return c.GetEmail(m.ID)
}

// DeleteEmail deletes an email account.
func (c *Core) DeleteEmail(id int) error {
	_, err := c.db.Exec("DELETE FROM emails WHERE id = $1", id)
	if err != nil {
		c.log.Printf("error deleting email: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return nil
}

// IncrementEmailSent increments sent count for an email account.
func (c *Core) IncrementEmailSent(id int) error {
	_, err := c.db.Exec("UPDATE emails SET sent_today = sent_today + 1 WHERE id = $1", id)
	return err
}

// ResetEmailDailyCounts resets daily counts across all email accounts.
func (c *Core) ResetEmailDailyCounts() error {
	_, err := c.db.Exec("UPDATE emails SET sent_today = 0")
	return err
}
