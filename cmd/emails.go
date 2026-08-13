package main

import (
	"net/http"
	"strconv"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

// GetEmails returns all sending email accounts.
func (a *App) GetEmails(c echo.Context) error {
	mbs, err := a.core.GetEmails()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{mbs})
}

// GetEmail returns a single email account by ID.
func (a *App) GetEmail(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	mb, err := a.core.GetEmail(id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{mb})
}

// CreateEmail creates a new sending email account.
func (a *App) CreateEmail(c echo.Context) error {
	var req models.Email
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	mb, err := a.core.CreateEmail(req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{mb})
}

// UpdateEmail updates an existing email account.
func (a *App) UpdateEmail(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.Email
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}
	req.ID = id

	mb, err := a.core.UpdateEmail(req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{mb})
}

// DeleteEmail deletes an email account.
func (a *App) DeleteEmail(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.core.DeleteEmail(id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}
