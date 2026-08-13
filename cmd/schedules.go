package main

import (
	"net/http"
	"strconv"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

// GetSchedules returns all schedules.
func (a *App) GetSchedules(c echo.Context) error {
	schedules, err := a.core.GetSchedules()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{schedules})
}

// GetSchedule returns a schedule by ID or UUID.
func (a *App) GetSchedule(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	uid := c.Param("id")

	sched, err := a.core.GetSchedule(id, uid)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{sched})
}

// CreateSchedule creates a new schedule.
func (a *App) CreateSchedule(c echo.Context) error {
	var req models.Schedule
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	sched, err := a.core.CreateSchedule(req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{sched})
}

// UpdateSchedule updates an existing schedule.
func (a *App) UpdateSchedule(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.Schedule
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}
	req.ID = id

	sched, err := a.core.UpdateSchedule(req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{sched})
}

// SetDefaultSchedule sets a schedule as default.
func (a *App) SetDefaultSchedule(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.core.SetDefaultSchedule(id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}

// DeleteSchedule deletes a schedule.
func (a *App) DeleteSchedule(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.core.DeleteSchedule(id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}
