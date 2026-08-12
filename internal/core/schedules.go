package core

import (
	"database/sql"
	"net/http"

	"github.com/gofrs/uuid/v5"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

// GetSchedules returns a list of all schedules.
func (c *Core) GetSchedules() ([]models.Schedule, error) {
	var out []models.Schedule
	err := c.db.Select(&out, "SELECT id, uuid, name, timezone, use_contact_timezone, skip_holidays, sending_windows, created_at, updated_at FROM schedules ORDER BY id DESC")
	if err != nil {
		c.log.Printf("error querying schedules: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return out, nil
}

// GetSchedule returns a schedule by ID or UUID.
func (c *Core) GetSchedule(id int, uid string) (*models.Schedule, error) {
	var s models.Schedule
	err := c.db.Get(&s, "SELECT id, uuid, name, timezone, use_contact_timezone, skip_holidays, sending_windows, created_at, updated_at FROM schedules WHERE id = $1 OR uuid::text = $2", id, uid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, echo.NewHTTPError(http.StatusNotFound, c.i18n.Ts("globals.messages.notFound"))
		}
		c.log.Printf("error getting schedule: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return &s, nil
}

// CreateSchedule creates a new schedule.
func (c *Core) CreateSchedule(s models.Schedule) (*models.Schedule, error) {
	if s.UUID == "" {
		s.UUID = uuid.Must(uuid.NewV4()).String()
	}
	if s.Timezone == "" {
		s.Timezone = "UTC"
	}
	if s.SendingWindows == nil {
		s.SendingWindows = models.JSON{}
	}

	var out models.Schedule
	err := c.db.Get(&out, `INSERT INTO schedules (uuid, name, timezone, use_contact_timezone, skip_holidays, sending_windows)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, uuid, name, timezone, use_contact_timezone, skip_holidays, sending_windows, created_at, updated_at`,
		s.UUID, s.Name, s.Timezone, s.UseContactTimezone, s.SkipHolidays, s.SendingWindows)
	if err != nil {
		c.log.Printf("error creating schedule: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return &out, nil
}

// UpdateSchedule updates an existing schedule.
func (c *Core) UpdateSchedule(s models.Schedule) (*models.Schedule, error) {
	if s.Timezone == "" {
		s.Timezone = "UTC"
	}
	if s.SendingWindows == nil {
		s.SendingWindows = models.JSON{}
	}

	_, err := c.db.Exec(`UPDATE schedules
		SET name = $2, timezone = $3, use_contact_timezone = $4, skip_holidays = $5, sending_windows = $6, updated_at = NOW()
		WHERE id = $1`,
		s.ID, s.Name, s.Timezone, s.UseContactTimezone, s.SkipHolidays, s.SendingWindows)
	if err != nil {
		c.log.Printf("error updating schedule: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return c.GetSchedule(s.ID, "")
}

// DeleteSchedule deletes a schedule.
func (c *Core) DeleteSchedule(id int) error {
	_, err := c.db.Exec("DELETE FROM schedules WHERE id = $1", id)
	if err != nil {
		c.log.Printf("error deleting schedule: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return nil
}
