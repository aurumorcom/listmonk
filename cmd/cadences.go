package main

import (
	"net/http"
	"strconv"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

type enrollReq struct {
	SubscriberIDs []int `json:"subscribers"`
}

type cadenceStepsReq struct {
	Steps []models.CadenceStep `json:"steps"`
}

// GetCadences returns all cadences.
func (a *App) GetCadences(c echo.Context) error {
	cads, err := a.core.GetCadences()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{cads})
}

// GetCadence returns a cadence by ID or UUID.
func (a *App) GetCadence(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	uid := c.Param("id")

	cad, err := a.core.GetCadence(id, uid)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{cad})
}

// CreateCadence creates a new cadence.
func (a *App) CreateCadence(c echo.Context) error {
	var req models.Cadence
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	cad, err := a.core.CreateCadence(req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{cad})
}

// UpdateCadence updates an existing cadence.
func (a *App) UpdateCadence(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.Cadence
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}
	req.ID = id

	cad, err := a.core.UpdateCadence(req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{cad})
}

// DeleteCadence deletes a cadence.
func (a *App) DeleteCadence(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.core.DeleteCadence(id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}

// GetCadenceSteps returns steps for a cadence.
func (a *App) GetCadenceSteps(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	steps, err := a.core.GetCadenceSteps(id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{steps})
}

// SaveCadenceSteps saves sequence steps for a cadence.
func (a *App) SaveCadenceSteps(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var req cadenceStepsReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	if err := a.core.SaveCadenceSteps(id, req.Steps); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}

// EnrollCadenceSubscribers enrolls subscriber IDs into a cadence.
func (a *App) EnrollCadenceSubscribers(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var req enrollReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	if err := a.core.EnrollCadenceSubscribers(id, req.SubscriberIDs); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}
