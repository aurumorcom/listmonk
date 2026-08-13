package main

import (
	"net/http"

	"github.com/knadh/listmonk/internal/auth"
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

// ManageContactSequences handles enrolling, disenrolling, or pausing contacts in target sequences.
func (a *App) ManageContactSequences(c echo.Context) error {
	var (
		pID        = c.Param("id")
		contactIDs []int
	)
	if pID != "" {
		id := getID(c)
		if id > 0 {
			contactIDs = append(contactIDs, id)
		}
	}

	var req struct {
		Action            string `json:"action"`
		Status            string `json:"status"`
		ContactIDs        []int  `json:"contact_ids"`
		SubscriberIDs     []int  `json:"subscriber_ids"`
		TargetSequenceIDs []int  `json:"target_sequence_ids"`
		SequenceIDs       []int  `json:"sequence_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	if len(contactIDs) == 0 {
		if len(req.ContactIDs) > 0 {
			contactIDs = req.ContactIDs
		} else {
			contactIDs = req.SubscriberIDs
		}
	}
	seqIDs := req.TargetSequenceIDs
	if len(seqIDs) == 0 {
		seqIDs = req.SequenceIDs
	}

	if len(contactIDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("subscribers.errorNoIDs"))
	}
	if len(seqIDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidFields", "name", "target_sequence_ids"))
	}

	if req.Action == "" {
		req.Action = "add"
	}

	if err := a.core.ManageContactSequences(contactIDs, seqIDs, req.Action, req.Status); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}

// ManageContactSequencesByQuery manages sequence memberships for contacts matching a query.
func (a *App) ManageContactSequencesByQuery(c echo.Context) error {
	return a.ManageContactSequences(c)
}

// GetContactSequences retrieves sequence memberships for a contact.
func (a *App) GetContactSequences(c echo.Context) error {
	id := getID(c)
	seqs, err := a.core.GetContactSequences(id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{seqs})
}
