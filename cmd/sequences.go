package main

import (
	"net/http"
	"strconv"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	null "gopkg.in/volatiletech/null.v6"
)

type enrollReq struct {
	SubscriberIDs []int          `json:"subscribers"`
	EmailID       null.Int       `json:"email_id"`
	WahaSession   null.String    `json:"waha_session"`
	User          map[string]any `json:"user"`
}

type reassignReq struct {
	EmailID     null.Int    `json:"email_id"`
	WahaSession null.String `json:"waha_session"`
}

type sequenceStepsReq struct {
	Steps []models.SequenceStep `json:"steps"`
}

// GetSequences returns all sequences.
func (a *App) GetSequences(c echo.Context) error {
	seqs, err := a.core.GetSequences()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{seqs})
}

// GetSequence returns a sequence by ID or UUID.
func (a *App) GetSequence(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	uid := c.Param("id")

	seq, err := a.core.GetSequence(id, uid)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{seq})
}

// CreateSequence creates a new sequence.
func (a *App) CreateSequence(c echo.Context) error {
	var req models.Sequence
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	seq, err := a.core.CreateSequence(req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{seq})
}

// UpdateSequence updates an existing sequence.
func (a *App) UpdateSequence(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.Sequence
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}
	req.ID = id

	seq, err := a.core.UpdateSequence(req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{seq})
}

// DeleteSequence deletes a sequence.
func (a *App) DeleteSequence(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.core.DeleteSequence(id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}

// GetSequenceSteps returns steps for a sequence.
func (a *App) GetSequenceSteps(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	steps, err := a.core.GetSequenceSteps(id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{steps})
}

// SaveSequenceSteps saves sequence steps for a sequence.
func (a *App) SaveSequenceSteps(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var req sequenceStepsReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	if err := a.core.SaveSequenceSteps(id, req.Steps); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}

// EnrollSequenceSubscribers enrolls subscriber IDs into a sequence.
func (a *App) EnrollSequenceSubscribers(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var req enrollReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	if err := a.core.EnrollSequenceContacts(id, req.SubscriberIDs, req.EmailID, req.WahaSession, req.User); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}

// ReassignSequenceContactSender updates the locked email account or WAHA session for a sequence contact.
func (a *App) ReassignSequenceContactSender(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	subID, _ := strconv.Atoi(c.Param("sub_id"))
	var req reassignReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	if err := a.core.ReassignSequenceContactSender(id, subID, req.EmailID, req.WahaSession); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}

// GetSequenceAnalytics returns sequence analytics metrics.
func (a *App) GetSequenceAnalytics(c echo.Context) error {
	stats, err := a.core.GetSequenceAnalytics()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{stats})
}
