package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/internal/sequence"
	"github.com/knadh/listmonk/internal/utils"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	null "gopkg.in/volatiletech/null.v6"
)

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

	// Resolve subscriber context for template compilation (preferring explicit production contact)
	var sampleSub models.Subscriber
	if req.SubscriberID > 0 {
		if sub, err := a.core.GetSubscriber(req.SubscriberID, "", ""); err == nil && sub.ID > 0 {
			sampleSub = sub
		}
	}
	if sampleSub.ID == 0 {
		sampleSub = a.resolveTestPreviewSubscriber(req.SubscriberID, user)
	}

	if req.Messenger == "" && req.StepNumber > 0 {
		if steps, err := a.core.GetSequenceSteps(id); err == nil && len(steps) >= req.StepNumber {
			req.Messenger = steps[req.StepNumber-1].Messenger
		}
	}
	if req.ContentType == "" {
		req.ContentType = models.CampaignContentTypeRichtext
	}

	// Prepare targets based on messenger type and active user profile
	isWhatsApp := req.Messenger == "whatsapp" || req.Messenger == "waha" || strings.HasPrefix(req.Messenger, "whatsapp-") || strings.HasPrefix(req.Messenger, "waha-")

	var targets []string
	if isWhatsApp {
		if req.TestPhone != "" {
			targets = append(targets, req.TestPhone)
		} else if user.Phone.Valid && user.Phone.String != "" {
			targets = append(targets, user.Phone.String)
		}
	} else {
		if req.TestEmail != "" {
			targets = append(targets, req.TestEmail)
		} else if user.Email.Valid && user.Email.String != "" {
			targets = append(targets, user.Email.String)
		}
	}

	if len(targets) == 0 {
		if isWhatsApp {
			return echo.NewHTTPError(http.StatusBadRequest, "Please enter a test phone number or configure your phone in User Profile.")
		}
		return echo.NewHTTPError(http.StatusBadRequest, "Please enter a test email address or configure your email in User Profile.")
	}

	step := models.SequenceStep{
		SequenceID: id,
		StepNumber: req.StepNumber,
		Messenger:  req.Messenger,
		Subject:    req.Subject,
		Body:       req.Body,
		TemplateID: req.TemplateID,
		EmailType:  models.EmailTypeNewThread,
	}
	for _, mID := range req.MediaIDs {
		if mID > 0 {
			step.MediaIDs = append(step.MediaIDs, int64(mID))
		}
	}

	// Ensure sequence manager instance is available
	if a.seqManager == nil {
		msgrMap := make(map[string]manager.Messenger)
		for _, m := range a.messengers {
			msgrMap[m.Name()] = m
		}
		if a.emailMsgr != nil && msgrMap["email"] == nil {
			msgrMap["email"] = a.emailMsgr
		}
		a.seqManager = sequence.NewManager(a.core, msgrMap, a.media, a.log)
	}

	// Resolve sequence for assigned pool lookup
	var assignedEmailID null.Int
	var assignedWahaSession null.String
	if seq, err := a.core.GetSequence(id, ""); err == nil {
		if len(seq.EmailIDs) > 0 {
			assignedEmailID = null.IntFrom(int(seq.EmailIDs[0]))
		}
		if len(seq.WahaSessions) > 0 && seq.WahaSessions[0] != "" && seq.WahaSessions[0] != "default" {
			assignedWahaSession = null.StringFrom(seq.WahaSessions[0])
		}
	}
	if !assignedWahaSession.Valid || assignedWahaSession.String == "" || assignedWahaSession.String == "default" {
		if user.WahaSession.Valid && user.WahaSession.String != "" && user.WahaSession.String != "default" {
			assignedWahaSession = user.WahaSession
		} else if settings, err := a.core.GetSettings(); err == nil {
			for _, wm := range settings.WAHASettings {
				if wm.Enabled && wm.Session != "" && wm.Session != "default" {
					assignedWahaSession = null.StringFrom(wm.Session)
					break
				}
			}
		}
	}

	for _, target := range targets {
		sub := sampleSub
		target = strings.TrimSpace(target)
		targetMessenger := req.Messenger

		if targetMessenger == "" {
			if strings.Contains(target, "@") {
				targetMessenger = "email"
			} else if _, err := utils.SanitizePhone(target); err == nil {
				targetMessenger = "whatsapp"
			} else {
				targetMessenger = "email"
			}
		}

		testStep := step
		testStep.Messenger = targetMessenger

		// Provide user context in subscriber attributes for persona resolution if not present
		if sub.Attribs == nil {
			sub.Attribs = models.JSON{}
		}
		if _, ok := sub.Attribs["user"]; !ok && user.Name != "" {
			sub.Attribs["user"] = map[string]any{
				"name":  user.Name,
				"email": user.Email.String,
			}
		}

		seqContact := models.SequenceContact{
			SequenceID:   id,
			SubscriberID: sub.ID,
			CurrentStep:  req.StepNumber,
			EmailID:      assignedEmailID,
			WahaSession:  assignedWahaSession,
		}

		if err := a.seqManager.PrepareAndDispatchStep(seqContact, sub, testStep, target); err != nil {
			a.log.Printf("error sending test sequence message: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError,
				a.i18n.Ts("campaigns.errorSendTest", "error", err.Error()))
		}
	}

	return c.JSON(http.StatusOK, okResp{true})
}
