package core

import (
	"database/sql"
	"net/http"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

// GetMailboxes returns a list of all sending mailboxes.
func (c *Core) GetMailboxes() ([]models.Mailbox, error) {
	var out []models.Mailbox
	err := c.db.Select(&out, "SELECT id, name, email, smtp_config, imap_config, daily_limit, sent_today, created_at, updated_at FROM mailboxes ORDER BY id DESC")
	if err != nil {
		c.log.Printf("error querying mailboxes: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return out, nil
}

// GetMailbox returns a mailbox by ID.
func (c *Core) GetMailbox(id int) (*models.Mailbox, error) {
	var m models.Mailbox
	err := c.db.Get(&m, "SELECT id, name, email, smtp_config, imap_config, daily_limit, sent_today, created_at, updated_at FROM mailboxes WHERE id = $1", id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, echo.NewHTTPError(http.StatusNotFound, c.i18n.Ts("globals.messages.notFound"))
		}
		c.log.Printf("error getting mailbox: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return &m, nil
}

// CreateMailbox creates a new sending mailbox.
func (c *Core) CreateMailbox(m models.Mailbox) (*models.Mailbox, error) {
	if m.DailyLimit <= 0 {
		m.DailyLimit = 50
	}
	if m.SMTPConfig == nil {
		m.SMTPConfig = models.JSON{}
	}
	if m.IMAPConfig == nil {
		m.IMAPConfig = models.JSON{}
	}

	var out models.Mailbox
	err := c.db.Get(&out, `INSERT INTO mailboxes (name, email, smtp_config, imap_config, daily_limit, sent_today)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, email, smtp_config, imap_config, daily_limit, sent_today, created_at, updated_at`,
		m.Name, m.Email, m.SMTPConfig, m.IMAPConfig, m.DailyLimit, 0)
	if err != nil {
		c.log.Printf("error creating mailbox: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return &out, nil
}

// UpdateMailbox updates an existing mailbox.
func (c *Core) UpdateMailbox(m models.Mailbox) (*models.Mailbox, error) {
	_, err := c.db.Exec(`UPDATE mailboxes SET name = $2, email = $3, smtp_config = $4, imap_config = $5, daily_limit = $6, updated_at = NOW() WHERE id = $1`,
		m.ID, m.Name, m.Email, m.SMTPConfig, m.IMAPConfig, m.DailyLimit)
	if err != nil {
		c.log.Printf("error updating mailbox: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return c.GetMailbox(m.ID)
}

// DeleteMailbox deletes a mailbox.
func (c *Core) DeleteMailbox(id int) error {
	_, err := c.db.Exec("DELETE FROM mailboxes WHERE id = $1", id)
	if err != nil {
		c.log.Printf("error deleting mailbox: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return nil
}

// IncrementMailboxSent increments sent count for a mailbox.
func (c *Core) IncrementMailboxSent(id int) error {
	_, err := c.db.Exec("UPDATE mailboxes SET sent_today = sent_today + 1 WHERE id = $1", id)
	return err
}

// ResetMailboxDailyCounts resets daily counts across all mailboxes.
func (c *Core) ResetMailboxDailyCounts() error {
	_, err := c.db.Exec("UPDATE mailboxes SET sent_today = 0")
	return err
}
