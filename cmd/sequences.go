package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/utils"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	null "gopkg.in/volatiletech/null.v6"
)

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

type sequenceReq struct {
	models.Sequence
	Lists []int `json:"lists"`
}

// CreateSequence creates a new sequence.
func (a *App) CreateSequence(c echo.Context) error {
	var req sequenceReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	seq, err := a.core.CreateSequence(req.Sequence, req.Lists)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{seq})
}

// UpdateSequence updates an existing sequence.
func (a *App) UpdateSequence(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var req sequenceReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}
	req.ID = id

	seq, err := a.core.UpdateSequence(req.Sequence, req.Lists)
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

// UpdateSequenceStatus handles sequence status modifications (active, paused, archived, cancelled).
func (a *App) UpdateSequenceStatus(c echo.Context) error {
	id := getID(c)
	var req struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	seq, err := a.core.UpdateSequenceStatus(id, req.Status)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{seq})
}

// UpdateSequenceArchive updates sequence web archive settings.
func (a *App) UpdateSequenceArchive(c echo.Context) error {
	id := getID(c)
	var req struct {
		Archive     bool        `json:"archive"`
		TemplateID  null.Int    `json:"archive_template_id"`
		ArchiveSlug null.String `json:"archive_slug"`
		Meta        models.JSON `json:"archive_meta"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	if err := a.core.UpdateSequenceArchive(id, req.Archive, req.TemplateID, req.Meta, req.ArchiveSlug); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}

// DeleteSequences bulk deletes sequences by ID list.
func (c *App) DeleteSequences(ctx echo.Context) error {
	pIDs := ctx.QueryParams()["id"]
	if len(pIDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, c.i18n.Ts("globals.messages.errorInvalidIDs", "error", "no IDs given"))
	}
	ids := make([]int, 0, len(pIDs))
	for _, p := range pIDs {
		if id, _ := strconv.Atoi(p); id > 0 {
			ids = append(ids, id)
		}
	}
	if err := c.core.DeleteSequences(ids); err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, okResp{true})
}

// PreviewSequence renders a sequence step preview.
func (a *App) PreviewSequence(c echo.Context) error {
	id := getID(c)
	steps, err := a.core.GetSequenceSteps(id)
	if err != nil {
		return err
	}
	if len(steps) == 0 {
		return c.JSON(http.StatusOK, okResp{""})
	}

	subID, _ := strconv.Atoi(c.FormValue("subscriber_id"))
	if subID == 0 {
		subID, _ = strconv.Atoi(c.QueryParam("subscriber_id"))
	}
	_ = a.getSubscriberForPreview(subID)

	return c.JSON(http.StatusOK, okResp{steps[0].Body})
}

// PreviewSequenceArchive renders sequence web archive preview.
func (a *App) PreviewSequenceArchive(c echo.Context) error {
	id := getID(c)
	seq, err := a.core.GetSequence(id, "")
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{seq})
}

type sequenceTestReq struct {
	ID               int            `json:"id"`
	StepNumber       int            `json:"step_number"`
	Messenger        string         `json:"messenger"`
	Subject          string         `json:"subject"`
	Body             string         `json:"body"`
	AltBody          null.String    `json:"altbody"`
	ContentType      string         `json:"content_type"`
	TemplateID       null.Int       `json:"template_id"`
	MediaIDs         []int          `json:"media"`
	Headers          models.Headers `json:"headers"`
	SubscriberEmails []string       `json:"subscribers"`
	SubscriberID     int            `json:"subscriber_id"`
	TestEmail        string         `json:"test_email"`
	TestPhone        string         `json:"test_phone"`
}

// TestSequence dispatches a test sequence step payload to test addresses.
func (a *App) TestSequence(c echo.Context) error {
	id := getID(c)
	var req sequenceTestReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	user := auth.GetUser(c)

	// Resolve preview subscriber for context
	var sampleSub models.Subscriber
	if req.SubscriberID > 0 {
		if s, err := a.core.GetSubscriber(req.SubscriberID, "", ""); err == nil {
			sampleSub = s
		}
	}
	if sampleSub.ID == 0 {
		sampleSub = a.getSubscriberForPreview(0)
	}

	// Prepare targets
	targets := req.SubscriberEmails
	if len(targets) == 0 {
		if req.TestEmail != "" {
			targets = append(targets, req.TestEmail)
		} else if user.Email.Valid && user.Email.String != "" {
			targets = append(targets, user.Email.String)
		}
		if req.TestPhone != "" {
			targets = append(targets, req.TestPhone)
		}
	}

	if req.Messenger == "" {
		req.Messenger = "email"
	}
	if req.ContentType == "" {
		req.ContentType = models.CampaignContentTypeRichtext
	}

	camp := models.Campaign{
		Name:        fmt.Sprintf("Sequence %d - Step %d Test", id, req.StepNumber),
		Subject:     req.Subject,
		Body:        req.Body,
		AltBody:     req.AltBody,
		Messenger:   req.Messenger,
		ContentType: req.ContentType,
		TemplateID:  req.TemplateID,
		Headers:     req.Headers,
		FromEmail:   a.cfg.FromEmail,
	}
	for _, mID := range req.MediaIDs {
		if mID > 0 {
			camp.MediaIDs = append(camp.MediaIDs, int64(mID))
		}
	}

	for _, target := range targets {
		sub := sampleSub
		target = strings.TrimSpace(target)
		if strings.Contains(target, "@") {
			sub.Email = strings.ToLower(target)
		} else if ph, err := utils.SanitizePhone(target); err == nil {
			sub.Phone = null.StringFrom(ph)
		} else {
			sub.Email = target
		}

		if err := a.sendTestMessage(sub, &camp); err != nil {
			a.log.Printf("error sending test sequence message: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError,
				a.i18n.Ts("campaigns.errorSendTest", "error", err.Error()))
		}
	}

	return c.JSON(http.StatusOK, okResp{true})
}
