package main

import (
	"net/http"
	"strconv"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

// GetMailboxes returns all sending mailboxes.
func (a *App) GetMailboxes(c echo.Context) error {
	mbs, err := a.core.GetMailboxes()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{mbs})
}

// GetMailbox returns a single mailbox by ID.
func (a *App) GetMailbox(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	mb, err := a.core.GetMailbox(id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{mb})
}

// CreateMailbox creates a new sending mailbox.
func (a *App) CreateMailbox(c echo.Context) error {
	var req models.Mailbox
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	mb, err := a.core.CreateMailbox(req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{mb})
}

// UpdateMailbox updates an existing mailbox.
func (a *App) UpdateMailbox(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.Mailbox
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}
	req.ID = id

	mb, err := a.core.UpdateMailbox(req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{mb})
}

// DeleteMailbox deletes a mailbox.
func (a *App) DeleteMailbox(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.core.DeleteMailbox(id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}
