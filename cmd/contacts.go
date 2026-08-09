package main

import (
	"net/http"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

// GetContact handles the retrieval of a single contact by ID.
func (a *App) GetContact(c echo.Context) error {
	user := auth.GetUser(c)

	id := getID(c)
	if err := a.hasSubPerm(user, []int{id}); err != nil {
		return err
	}

	out, err := a.core.GetContact(id, "", "")
	if err != nil {
		return err
	}

	maskRestrictedSubLists(user, &out)

	return c.JSON(http.StatusOK, okResp{out})
}

// GetContacts handles querying contacts.
func (a *App) GetContacts(c echo.Context) error {
	return a.QuerySubscribers(c)
}

// CreateContact handles creation of a new contact.
func (a *App) CreateContact(c echo.Context) error {
	return a.CreateSubscriber(c)
}

// UpdateContact handles updating an existing contact.
func (a *App) UpdateContact(c echo.Context) error {
	return a.UpdateSubscriber(c)
}

// DeleteContacts handles deletion of contacts.
func (a *App) DeleteContacts(c echo.Context) error {
	return a.DeleteSubscribers(c)
}
