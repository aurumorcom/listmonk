package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/campaign"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/internal/notifs"
	"github.com/knadh/listmonk/internal/utils"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"gopkg.in/volatiletech/null.v6"
)

// campReq is a wrapper over the Campaign model for receiving
// campaign creation and update data from APIs.
type campReq struct {
	models.Campaign

	// This overrides Campaign.Lists to receive and
	// write a list of int IDs during creation and updation.
	// Campaign.Lists is JSONText for sending lists children
	// to the outside world.
	ListIDs []int `json:"lists"`

	MediaIDs []int `json:"media"`

	// This is only relevant to campaign test requests.
	SubscriberID     int            `json:"subscriber_id"`
	SubscriberEmails pq.StringArray `json:"subscribers"`
	TestEmail        string         `json:"test_email"`
	TestPhone        string         `json:"test_phone"`
}

// campContentReq wraps params coming from API requests for converting
// campaign content formats.
type campContentReq struct {
	models.Campaign
	From string `json:"from"`
	To   string `json:"to"`
}

var (
	reFromAddress = regexp.MustCompile(`((.+?)\s)?<(.+?)@(.+?)>`)
	reSlug        = regexp.MustCompile(`[^\p{L}\p{M}\p{N}]`)
)

// GetCampaigns handles retrieval of campaigns.
func (a *App) GetCampaigns(c echo.Context) error {
	// Get the authenticated user.
	user := auth.GetUser(c)

	var (
		hasAllPerm     = user.HasPerm(auth.PermCampaignsGetAll)
		permittedLists []int
	)

	if !hasAllPerm {
		// Either the user has campaigns:get_all permissions and can view all campaigns,
		// or the campaigns are filtered by the lists the user has get|manage access to.
		hasAllPerm, permittedLists = user.GetPermittedLists(auth.PermTypeGet | auth.PermTypeManage)
	}

	var (
		pg = a.pg.NewFromURL(c.Request().URL.Query())

		status    = c.QueryParams()["status"]
		tags      = c.QueryParams()["tag"]
		query     = strings.TrimSpace(c.FormValue("query"))
		orderBy   = c.FormValue("order_by")
		order     = c.FormValue("order")
		noBody, _ = strconv.ParseBool(c.QueryParam("no_body"))
	)

	// Query and retrieve campaigns from the DB.
	res, total, err := a.core.QueryCampaigns(query, status, tags, orderBy, order, hasAllPerm, permittedLists, pg.Offset, pg.Limit)
	if err != nil {
		return err
	}

	// Remove the body from the response if requested.
	if noBody {
		for i := range res {
			res[i].Body = ""
			res[i].BodySource.Valid = false
		}
	}

	// Paginate the response.
	if len(res) == 0 {
		return c.JSON(http.StatusOK, okResp{models.PageResults{Results: []models.Campaign{}}})
	}

	out := models.PageResults{
		Query:   query,
		Results: res,
		Total:   total,
		Page:    pg.Page,
		PerPage: pg.PerPage,
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// GetCampaign handles retrieval of campaigns.
func (a *App) GetCampaign(c echo.Context) error {
	// Get the campaign ID.
	id := getID(c)

	// Check if the user has access to the campaign.
	if err := a.checkCampaignPerm(auth.PermTypeGet, id, c); err != nil {
		return err
	}

	// Get the campaign from the DB.
	out, err := a.core.GetCampaign(id, "", "")
	if err != nil {
		return err
	}

	// Blank out the body if requested.
	noBody, _ := strconv.ParseBool(c.QueryParam("no_body"))
	if noBody {
		out.Body = ""
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// PreviewCampaign renders the HTML preview of a campaign body.
func (a *App) PreviewCampaign(c echo.Context) error {
	// Get the campaign ID.
	id := getID(c)

	// Check if the user has access to the campaign.
	if err := a.checkCampaignPerm(auth.PermTypeGet, id, c); err != nil {
		return err
	}

	var (
		isPost      = c.Request().Method == http.MethodPost
		contentType = c.FormValue("content_type")
		tplID, _    = strconv.Atoi(c.FormValue("template_id"))
	)
	// For visual content, template ID for previewing is irrelevant.
	if contentType == models.CampaignContentTypeVisual || tplID < 1 {
		tplID = 0
	}

	// Get the campaign from the DB for previewing with the `template_body` field.
	camp, err := a.core.GetCampaignForPreview(id, tplID)
	if err != nil {
		return err
	}

	// There's a body in the request to preview instead of the body in the DB.
	if isPost {
		camp.ContentType = contentType
		camp.Body = c.FormValue("body")

		// For visual campaigns, template body from the DB shouldn't be used.
		if contentType == models.CampaignContentTypeVisual {
			camp.TemplateBody = ""
		}
	}

	// Use a dummy campaign ID to prevent views and clicks from {{ TrackView }}
	// and {{ TrackLink }} being registered on preview.
	camp.UUID = dummySubscriber.UUID
	if err := camp.CompileTemplate(a.manager.TemplateFuncs(&camp)); err != nil {
		a.log.Printf("error compiling template: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("templates.errorCompiling", "error", err.Error()))
	}

	subID, _ := strconv.Atoi(c.FormValue("subscriber_id"))
	if subID == 0 {
		subID, _ = strconv.Atoi(c.QueryParam("subscriber_id"))
	}
	sub := a.getSubscriberForPreview(subID)

	// Render the message body.
	msg, err := a.manager.NewCampaignMessage(&camp, sub)
	if err != nil {
		a.log.Printf("error rendering message: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("templates.errorRendering", "error", err.Error()))
	}

	// Plaintext headers for plain body.
	if camp.ContentType == models.CampaignContentTypePlain {
		return c.String(http.StatusOK, string(msg.Body()))
	}

	return c.HTML(http.StatusOK, string(msg.Body()))
}

// PreviewCampaignArchive renders the public campaign archives page.
func (a *App) PreviewCampaignArchive(c echo.Context) error {
	// Get the campaign ID.
	id := getID(c)

	// Check if the user has access to the campaign.
	if err := a.checkCampaignPerm(auth.PermTypeGet, id, c); err != nil {
		return err
	}

	// Fetch the campaign body from the DB.
	tplID, _ := strconv.Atoi(c.FormValue("template_id"))
	camp, err := a.core.GetCampaignForPreview(id, tplID)
	if err != nil {
		return err
	}

	camp.ArchiveMeta = json.RawMessage([]byte(c.FormValue("archive_meta")))

	// "Compile" the campaign template with appropriate data.
	res, err := a.compileArchiveCampaigns([]models.Campaign{camp})
	if err != nil {
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorFetchingCampaign")))
	}

	// Render the campaign body.
	out := res[0].Campaign
	msg, err := a.manager.NewCampaignMessage(out, res[0].Subscriber)
	if err != nil {
		a.log.Printf("error rendering campaign: %v", err)
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorFetchingCampaign")))
	}

	return c.HTML(http.StatusOK, string(msg.Body()))
}

// CampaignContent handles campaign content (body) format conversions.
func (a *App) CampaignContent(c echo.Context) error {
	var camp campContentReq
	if err := c.Bind(&camp); err != nil {
		return err
	}

	// Convert formats, eg: markdown to HTML.
	out, err := camp.ConvertContent(camp.From, camp.To)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// CreateCampaign handles campaign creation.
// Newly created campaigns are always drafts.
func (a *App) CreateCampaign(c echo.Context) error {
	var o campReq
	if err := c.Bind(&o); err != nil {
		return err
	}

	// Filter lists against the current user's permitted lists.
	user := auth.GetUser(c)
	o.ListIDs = user.FilterListsByPerm(auth.PermTypeGet|auth.PermTypeManage, o.ListIDs)

	// If the campaign's 'opt-in', prepare a default message.
	switch o.Type {
	case models.CampaignTypeOptin:
		op, err := a.makeOptinCampaignMessage(o)
		if err != nil {
			return err
		}
		o = op
	case "":
		o.Type = models.CampaignTypeRegular
	}

	if o.Messenger == "" {
		o.Messenger = "email"
	}

	// Validate.
	if c, err := a.validateCampaignFields(o); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	} else {
		o = c
	}

	if o.ArchiveTemplateID.Valid && o.ArchiveTemplateID.Int != 0 {
		o.ArchiveTemplateID = o.TemplateID
	}

	out, err := a.core.CreateCampaign(o.Campaign, o.ListIDs, o.MediaIDs)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// UpdateCampaign handles campaign modification.
// Campaigns that are done cannot be modified.
func (a *App) UpdateCampaign(c echo.Context) error {
	// Get the campaign ID.
	id := getID(c)

	// Check if the user has access to the campaign.
	if err := a.checkCampaignPerm(auth.PermTypeManage, id, c); err != nil {
		return err
	}

	// Retrieve the campaign from the DB.
	cm, err := a.core.GetCampaign(id, "", "")
	if err != nil {
		return err
	}

	if !canEditCampaign(cm.Status) {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("campaigns.cantUpdate"))
	}

	// Clear attribs to avoid merging old and new values as json.Unmarshal in JSON.scan() merges maps,
	// merging values already in the DB and incoming values. If this is nil, then DB values remain
	// unchanged.
	cm.Attribs = nil

	// Read the incoming params into the existing campaign fields from the DB.
	// This allows updating of values that have been sent whereas fields
	// that are not in the request retain the old values.
	o := campReq{Campaign: cm}
	if err := c.Bind(&o); err != nil {
		return err
	}

	// Filter lists against the current user's permitted lists.
	user := auth.GetUser(c)
	o.ListIDs = user.FilterListsByPerm(auth.PermTypeGet|auth.PermTypeManage, o.ListIDs)

	if c, err := a.validateCampaignFields(o); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	} else {
		o = c
	}

	out, err := a.core.UpdateCampaign(id, o.Campaign, o.ListIDs, o.MediaIDs)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// UpdateCampaignStatus handles campaign status modification.
func (a *App) UpdateCampaignStatus(c echo.Context) error {
	// Get the campaign ID.
	id := getID(c)

	// Check if the user has access to the campaign.
	if err := a.checkCampaignPerm(auth.PermTypeManage, id, c); err != nil {
		return err
	}

	req := struct {
		Status string `json:"status"`
	}{}
	if err := c.Bind(&req); err != nil {
		return err
	}

	// Update the campaign status in the DB.
	out, err := a.core.UpdateCampaignStatus(id, req.Status)
	if err != nil {
		return err
	}

	// If the campaign is being stopped, send the signal to the manager to stop it in flight.
	if req.Status == models.CampaignStatusPaused || req.Status == models.CampaignStatusCancelled {
		a.manager.StopCampaign(id)
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// UpdateCampaignArchive handles campaign status modification.
func (a *App) UpdateCampaignArchive(c echo.Context) error {
	id := getID(c)

	// Check if the user has access to the campaign.
	if err := a.checkCampaignPerm(auth.PermTypeManage, id, c); err != nil {
		return err
	}

	req := struct {
		Archive     bool        `json:"archive"`
		TemplateID  int         `json:"archive_template_id"`
		Meta        models.JSON `json:"archive_meta"`
		ArchiveSlug string      `json:"archive_slug"`
	}{}
	if err := c.Bind(&req); err != nil {
		return err
	}

	if req.ArchiveSlug != "" {
		// Format the slug to be alpha-numeric-dash.
		s := strings.ToLower(req.ArchiveSlug)
		s = strings.TrimSpace(reSlug.ReplaceAllString(s, " "))
		s = regexpSpaces.ReplaceAllString(s, "-")
		req.ArchiveSlug = s
	}

	if err := a.core.UpdateCampaignArchive(id, req.Archive, req.TemplateID, req.Meta, req.ArchiveSlug); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{req})
}

// DeleteCampaign handles campaign deletion.
// Only scheduled campaigns that have not started yet can be deleted.
func (a *App) DeleteCampaign(c echo.Context) error {
	// Get the campaign ID.
	id := getID(c)

	// Check if the user has access to the campaign.
	if err := a.checkCampaignPerm(auth.PermTypeManage, id, c); err != nil {
		return err
	}

	// Delete the campaign from the DB.
	if err := a.core.DeleteCampaign(id); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{true})
}

// DeleteCampaigns deletes multiple campaigns by IDs or by query.
func (a *App) DeleteCampaigns(c echo.Context) error {
	// Get the authenticated user.
	user := auth.GetUser(c)

	var (
		hasAllPerm     = user.HasPerm(auth.PermCampaignsManageAll)
		permittedLists []int
	)

	if !hasAllPerm {
		// Either the user has campaigns:manage_all permissions and can manage all campaigns,
		// or the campaigns are filtered by the lists the user has get|manage access to.
		hasAllPerm, permittedLists = user.GetPermittedLists(auth.PermTypeGet | auth.PermTypeManage)
	}

	var (
		ids   []int
		query string
		all   bool
	)

	// Check for IDs in query params.
	if len(c.Request().URL.Query()["id"]) > 0 {
		var err error
		ids, err = parseStringIDs(c.Request().URL.Query()["id"])
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest,
				a.i18n.Ts("globals.messages.errorInvalidIDs", "error", err.Error()))
		}
	} else {
		// Check for query param.
		query = strings.TrimSpace(c.FormValue("query"))
		all = c.FormValue("all") == "true"
	}

	// Validate that either IDs or query is provided.
	if len(ids) == 0 && (query == "" && !all) {
		return echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("globals.messages.errorInvalidIDs", "error", "id or query required"))
	}

	// Delete the campaigns from the DB.
	if err := a.core.DeleteCampaigns(ids, query, hasAllPerm, permittedLists); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{true})
}

// GetRunningCampaignStats returns stats of a given set of campaign IDs.
func (a *App) GetRunningCampaignStats(c echo.Context) error {
	// Get the running campaign stats from the DB.
	out, err := a.core.GetRunningCampaignStats()
	if err != nil {
		return err
	}

	if len(out) == 0 {
		return c.JSON(http.StatusOK, okResp{[]struct{}{}})
	}

	// Compute rate.
	for i, c := range out {
		if c.Started.Valid && c.UpdatedAt.Valid {
			diff := max(int(c.UpdatedAt.Time.Sub(c.Started.Time).Minutes()), 1)

			rate := c.Sent / diff
			if rate > c.Sent || rate > c.ToSend {
				rate = c.Sent
			}

			// Rate since the starting of the campaign.
			out[i].NetRate = rate

			// Realtime running rate over the last minute.
			out[i].Rate = a.manager.GetCampaignStats(c.ID).SendRate
		}
	}

	return c.JSON(http.StatusOK, okResp{out})
}

func resolveTestRecipients(req campReq, user auth.User) ([]string, error) {
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
			return nil, errors.New("Please enter a test phone number or configure your phone in User Profile.")
		}
		return nil, errors.New("Please enter a test email address or configure your email in User Profile.")
	}
	return targets, nil
}

func buildTestCampaignOverride(camp *models.Campaign, req campReq, user *auth.User) *models.Campaign {
	c := *camp
	c.Name = req.Name
	c.Subject = req.Subject
	c.FromEmail = FormatTestMessageSender(user, req.FromEmail)
	c.Body = req.Body
	c.AltBody = req.AltBody
	c.Messenger = req.Messenger
	c.ContentType = req.ContentType
	c.Headers = req.Headers
	c.TemplateID = req.TemplateID
	for _, id := range req.MediaIDs {
		if id > 0 {
			c.MediaIDs = append(c.MediaIDs, int64(id))
		}
	}
	return &c
}

// TestCampaign handles the sending of a campaign message to
// arbitrary subscribers for testing.
func (a *App) TestCampaign(c echo.Context) error {
	id := getID(c)

	if err := a.checkCampaignPerm(auth.PermTypeManage, id, c); err != nil {
		return err
	}

	var req campReq
	if err := c.Bind(&req); err != nil {
		return err
	}

	if validatedReq, err := a.validateCampaignFields(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	} else {
		req = validatedReq
	}

	user := auth.GetUser(c)

	var contactInputs []string
	for _, s := range req.SubscriberEmails {
		if s = strings.TrimSpace(s); s != "" {
			contactInputs = append(contactInputs, s)
		}
	}
	if len(contactInputs) == 0 {
		if req.TestEmail != "" {
			contactInputs = append(contactInputs, strings.TrimSpace(req.TestEmail))
		}
		if req.TestPhone != "" {
			contactInputs = append(contactInputs, strings.TrimSpace(req.TestPhone))
		}
	}

	targets, err := resolveTestRecipients(req, user)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	tplID, _ := strconv.Atoi(c.FormValue("template_id"))
	baseCamp, err := a.core.GetCampaignForPreview(id, tplID)
	if err != nil {
		return err
	}

	camp := buildTestCampaignOverride(&baseCamp, req, &user)

	var contactSubs []models.Subscriber
	if len(contactInputs) > 0 {
		for _, input := range contactInputs {
			sub, err := a.resolveTestSubscriber(input)
			if err != nil {
				return err
			}
			contactSubs = append(contactSubs, sub)
		}
	} else if req.SubscriberID > 0 {
		sub, err := a.core.GetSubscriber(req.SubscriberID, "", "")
		if err != nil || sub.ID == 0 {
			return echo.NewHTTPError(http.StatusNotFound, a.i18n.Ts("globals.messages.notFound", "name", fmt.Sprintf("{globals.terms.subscriber} (%d)", req.SubscriberID)))
		}
		contactSubs = append(contactSubs, sub)
	} else {
		sub := a.resolveTestPreviewSubscriber(0, user)
		contactSubs = append(contactSubs, sub)
	}

	for _, sub := range contactSubs {
		for _, target := range targets {
			target = strings.TrimSpace(target)
			targetMessenger := camp.Messenger

			if targetMessenger == "" {
				if strings.Contains(target, "@") {
					targetMessenger = "email"
				} else if _, err := utils.SanitizePhone(target); err == nil {
					targetMessenger = "whatsapp"
				} else {
					targetMessenger = "email"
				}
			}

			testCamp := *camp
			testCamp.Messenger = targetMessenger

			var overrideEmail, overridePhone string
			if strings.Contains(target, "@") {
				overrideEmail = target
			} else {
				overridePhone = target
			}

			if err := a.sendTestMessage(sub, &testCamp, overrideEmail, overridePhone); err != nil {
				a.log.Printf("error sending test message: %v", err)
				return echo.NewHTTPError(http.StatusInternalServerError,
					a.i18n.Ts("campaigns.errorSendTest", "error", err.Error()))
			}
		}
	}

	return c.JSON(http.StatusOK, okResp{true})
}

// GetCampaignViewAnalytics retrieves view counts for a campaign.
func (a *App) GetCampaignViewAnalytics(c echo.Context) error {
	ids, err := parseStringIDs(c.Request().URL.Query()["id"])
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("globals.messages.errorInvalidIDs", "error", err.Error()))
	}

	if len(ids) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("globals.messages.missingFields", "name", "`id`"))
	}

	// Ensure the user has access to campaigns via lists.
	for _, id := range ids {
		if err := a.checkCampaignPerm(auth.PermTypeGet, id, c); err != nil {
			return err
		}
	}

	var (
		typ  = c.Param("type")
		from = c.QueryParams().Get("from")
		to   = c.QueryParams().Get("to")
	)
	if !strHasLen(from, 10, 30) || !strHasLen(to, 10, 30) {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("analytics.invalidDates"))
	}

	// Extended Campaign Analytics (overview, breakdowns, or bot analytics)
	if typ == "overview" || typ == "analytics" {
		var results []models.CampaignAnalytics
		for _, id := range ids {
			res, err := a.core.GetCampaignAnalytics(id, from, to)
			if err != nil {
				return err
			}
			results = append(results, *res)
		}
		return c.JSON(http.StatusOK, okResp{results})
	}

	// Campaign link stats.
	if typ == "links" {
		out, err := a.core.GetCampaignAnalyticsLinks(ids, typ, from, to)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, okResp{out})
	}

	// Get the analytics numbers from the DB for the campaigns.
	out, err := a.core.GetCampaignAnalyticsCounts(ids, typ, from, to)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// sendTestMessage takes a campaign and a subscriber and sends out a sample campaign message.
func (a *App) sendTestMessage(sub models.Subscriber, camp *models.Campaign, overrideEmail, overridePhone string) error {
	if err := a.manager.LoadInlineImages(camp); err != nil {
		a.log.Printf("error loading inline images: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if err := camp.CompileTemplate(a.manager.TemplateFuncs(camp)); err != nil {
		a.log.Printf("error compiling template: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			a.i18n.Ts("templates.errorCompiling", "error", err.Error()))
	}

	// Create a sample campaign message.
	msg, err := a.manager.NewCampaignMessage(camp, sub)
	if err != nil {
		a.log.Printf("error rendering message: %v", err)
		return echo.NewHTTPError(http.StatusNotFound, a.i18n.Ts("templates.errorRendering", "error", err.Error()))
	}

	if overrideEmail != "" || overridePhone != "" {
		msg.OverrideTo(overrideEmail, overridePhone)
	}

	return a.manager.PushCampaignMessage(msg)
}

// validateCampaignFields validates incoming campaign field values.
func (a *App) validateCampaignFields(c campReq) (campReq, error) {
	if c.FromEmail == "" {
		c.FromEmail = a.cfg.FromEmail
	} else if !reFromAddress.Match([]byte(c.FromEmail)) {
		if _, err := a.importer.SanitizeEmail(c.FromEmail); err != nil {
			return c, errors.New(a.i18n.T("campaigns.fieldInvalidFromEmail"))
		}
	}

	if !strHasLen(c.Name, 1, stdInputMaxLen) {
		return c, errors.New(a.i18n.T("campaigns.fieldInvalidName"))
	}

	// Larger char limit for subject as it can contain {{ go templating }} logic.
	if !strHasLen(c.Subject, 1, 5000) {
		return c, errors.New(a.i18n.T("campaigns.fieldInvalidSubject"))
	}

	// If no content-type is specified, default to richtext.
	if c.ContentType != models.CampaignContentTypeRichtext &&
		c.ContentType != models.CampaignContentTypeHTML &&
		c.ContentType != models.CampaignContentTypePlain &&
		c.ContentType != models.CampaignContentTypeVisual &&
		c.ContentType != models.CampaignContentTypeMarkdown {
		c.ContentType = models.CampaignContentTypeRichtext
	}

	if c.ContentType != models.CampaignContentTypeVisual {
		c.BodySource.Valid = false
	}

	// If there's a "send_at" date, it should be in the future.
	if c.SendAt.Valid {
		if c.SendAt.Time.Before(time.Now()) {
			return c, errors.New(a.i18n.T("campaigns.fieldInvalidSendAt"))
		}
	}

	if len(c.ListIDs) == 0 {
		return c, errors.New(a.i18n.T("campaigns.fieldInvalidListIDs"))
	}

	if !a.manager.HasMessenger(c.Messenger) {
		// If it's a specific SMTP, but it's no longer available (removed/disabled), fall back to general email messenger.
		if strings.HasPrefix(c.Messenger, "email-") {
			c.Messenger = "email"
		} else {
			return c, errors.New(a.i18n.Ts("campaigns.fieldInvalidMessenger", "name", c.Messenger))
		}
	}

	camp := models.Campaign{Body: c.Body, TemplateBody: tplTag}
	if err := c.CompileTemplate(a.manager.TemplateFuncs(&camp)); err != nil {
		return c, errors.New(a.i18n.Ts("campaigns.fieldInvalidBody", "error", err.Error()))
	}

	if len(c.Headers) == 0 {
		c.Headers = make([]map[string]string, 0)
	}

	// Validate and initialize attribs.
	if c.Attribs != nil {
		if _, err := json.Marshal(c.Attribs); err != nil {
			return c, errors.New(a.i18n.T("subscribers.invalidJSON"))
		}
	}

	if len(c.ArchiveMeta) == 0 {
		c.ArchiveMeta = json.RawMessage("{}")
	}

	if c.ArchiveSlug.String != "" {
		// Format the slug to be alpha-numeric-dash.
		s := strings.ToLower(c.ArchiveSlug.String)
		s = strings.TrimSpace(reSlug.ReplaceAllString(s, " "))
		s = regexpSpaces.ReplaceAllString(s, "-")

		c.ArchiveSlug = null.NewString(s, true)
	} else {
		// If there's no slug set, set it to NULL in the DB.
		c.ArchiveSlug.Valid = false
	}

	return c, nil
}

// makeOptinCampaignMessage makes a default opt-in campaign message body.
func (a *App) makeOptinCampaignMessage(o campReq) (campReq, error) {
	if len(o.ListIDs) == 0 {
		return o, echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("campaigns.fieldInvalidListIDs"))
	}

	// Fetch double opt-in lists from the given list IDs from the DB.
	lists, err := a.core.GetListsByOptin(o.ListIDs, models.ListOptinDouble)
	if err != nil {
		return o, err
	}

	// There are no double opt-in lists.
	if len(lists) == 0 {
		return o, echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("campaigns.noOptinLists"))
	}

	// Construct the opt-in URL with list IDs.
	listIDs := url.Values{}
	for _, l := range lists {
		listIDs.Add("l", l.UUID)
	}
	// optinURLFunc := template.URL("{{ OptinURL }}?" + listIDs.Encode())
	optinURLAttr := template.HTMLAttr(fmt.Sprintf(`href="{{ OptinURL }}%s"`, listIDs.Encode()))

	// Prepare sample opt-in message for the campaign.
	var b bytes.Buffer

	if err := notifs.Tpls.ExecuteTemplate(&b, "optin-campaign", struct {
		Lists        []models.List
		OptinURLAttr template.HTMLAttr
	}{lists, optinURLAttr}); err != nil {
		a.log.Printf("error compiling 'optin-campaign' template: %v", err)
		return o, echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("templates.errorCompiling", "error", err.Error()))
	}

	o.Body = b.String()
	return o, nil
}

// checkCampaignPerm checks if the user has get or manage access to the given campaign.
// Either the user has blanket get_all/manage_all permissions, or the campaign
// belongs to lists that the user has access to.
func (a *App) checkCampaignPerm(types auth.PermType, id int, c echo.Context) error {
	// Get the authenticated user.
	user := auth.GetUser(c)

	perm := auth.PermCampaignsGet
	if types&auth.PermTypeGet != 0 {
		// It's a get request and there's a blanket get all permission.
		if user.HasPerm(auth.PermCampaignsGetAll) {
			return nil
		}
	} else {
		// It's a manage request and there's a blanket manage_all permission.
		if user.HasPerm(auth.PermCampaignsManageAll) {
			return nil
		}

		perm = auth.PermCampaignsManage
	}

	// There are no *_all campaign permissions. Instead, check if the user access
	// blanket get_all/manage_all list permissions. If yes, then the user can access
	// all campaigns. If there are no *_all permissions, then ensure that the
	// campaign belongs to the lists that the user has access to.
	if hasAllPerm, permittedListIDs := user.GetPermittedLists(auth.PermTypeGet | auth.PermTypeManage); !hasAllPerm {
		if ok, err := a.core.CampaignHasLists(id, permittedListIDs); err != nil {
			return err
		} else if !ok {
			return echo.NewHTTPError(http.StatusForbidden,
				a.i18n.Ts("globals.messages.permissionDenied", "name", perm))
		}
	}

	return nil
}

// canEditCampaign returns true if a campaign is in a status where updating
// its properties is allowed.
func canEditCampaign(status string) bool {
	return status == models.CampaignStatusDraft ||
		status == models.CampaignStatusPaused ||
		status == models.CampaignStatusScheduled
}

// FormatTestMessageSender formats a sender address for test messages using Active User Name and fromEmail.
func FormatTestMessageSender(user *auth.User, fromEmail string) string {
	fromEmail = strings.TrimSpace(fromEmail)
	if fromEmail == "" {
		return ""
	}
	if strings.Contains(fromEmail, "<") && strings.Contains(fromEmail, ">") {
		return fromEmail
	}
	if user != nil && strings.TrimSpace(user.Name) != "" {
		return fmt.Sprintf("%s <%s>", strings.TrimSpace(user.Name), fromEmail)
	}
	return fromEmail
}

type sequenceStepsReq struct {
	Steps []models.CampaignStep `json:"steps"`
}

type sequenceReq struct {
	models.Campaign
	Lists []int `json:"lists"`
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

// GetSequences returns all sequence campaigns.
func (a *App) GetSequences(c echo.Context) error {
	seqs, err := a.core.GetSequences()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{seqs})
}

// GetSequence returns a sequence campaign by ID or UUID.
func (a *App) GetSequence(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil && c.Param("id") != "" {
		id = 0
	}
	uid := c.Param("id")

	seq, err := a.core.GetSequence(id, uid)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{seq})
}

// CreateSequence creates a new sequence campaign.
func (a *App) CreateSequence(c echo.Context) error {
	var req sequenceReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	seq, err := a.core.CreateSequence(req.Campaign, req.Lists)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{seq})
}

// UpdateSequence updates an existing sequence campaign.
func (a *App) UpdateSequence(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var req sequenceReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}
	req.ID = id

	seq, err := a.core.UpdateSequence(req.Campaign, req.Lists)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{seq})
}

// DeleteSequence deletes a sequence campaign.
func (a *App) DeleteSequence(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.core.DeleteSequence(id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{true})
}

// GetSequenceSteps returns steps for a sequence campaign.
func (a *App) GetSequenceSteps(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	steps, err := a.core.GetSequenceSteps(id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, okResp{steps})
}

// SaveSequenceSteps saves sequence steps for a sequence campaign.
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

// DeleteSequences bulk deletes sequence campaigns by ID list.
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

// TestSequence dispatches a test sequence step payload to test addresses.
func (a *App) TestSequence(c echo.Context) error {
	id := getID(c)
	var req sequenceTestReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}

	user := auth.GetUser(c)

	var contactInputs []string
	for _, s := range req.SubscriberEmails {
		if s = strings.TrimSpace(s); s != "" {
			contactInputs = append(contactInputs, s)
		}
	}
	if len(contactInputs) == 0 {
		if req.TestEmail != "" {
			contactInputs = append(contactInputs, strings.TrimSpace(req.TestEmail))
		}
		if req.TestPhone != "" {
			contactInputs = append(contactInputs, strings.TrimSpace(req.TestPhone))
		}
	}

	var contactSubs []models.Subscriber
	if len(contactInputs) > 0 {
		for _, input := range contactInputs {
			sub, err := a.resolveTestSubscriber(input)
			if err != nil {
				return err
			}
			contactSubs = append(contactSubs, sub)
		}
	} else if req.SubscriberID > 0 {
		sub, err := a.core.GetSubscriber(req.SubscriberID, "", "")
		if err != nil || sub.ID == 0 {
			return echo.NewHTTPError(http.StatusNotFound, a.i18n.Ts("globals.messages.notFound", "name", fmt.Sprintf("{globals.terms.subscriber} (%d)", req.SubscriberID)))
		}
		contactSubs = append(contactSubs, sub)
	} else {
		sub := a.resolveTestPreviewSubscriber(0, user)
		contactSubs = append(contactSubs, sub)
	}

	if req.Messenger == "" && req.StepNumber > 0 {
		if steps, err := a.core.GetSequenceSteps(id); err == nil && len(steps) >= req.StepNumber {
			req.Messenger = steps[req.StepNumber-1].Messenger
		}
	}
	if req.ContentType == "" {
		req.ContentType = models.CampaignContentTypeRichtext
	}

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

	step := models.CampaignStep{
		CampaignID: id,
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

	if a.stepManager == nil {
		msgrMap := make(map[string]manager.Messenger)
		for _, m := range a.messengers {
			msgrMap[m.Name()] = m
		}
		if a.emailMsgr != nil && msgrMap["email"] == nil {
			msgrMap["email"] = a.emailMsgr
		}
		a.stepManager = campaign.NewManager(a.core, msgrMap, a.media, appLogger)
	}

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

	for _, sub := range contactSubs {
		for _, target := range targets {
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

			if sub.Attribs == nil {
				sub.Attribs = models.JSON{}
			}
			if _, ok := sub.Attribs["user"]; !ok && user.Name != "" {
				sub.Attribs["user"] = map[string]any{
					"name":  user.Name,
					"email": user.Email.String,
				}
			}

			seqSub := models.CampaignSubscriber{
				CampaignID:   id,
				SubscriberID: sub.ID,
				CurrentStep:  req.StepNumber,
				EmailID:      assignedEmailID,
				WahaSession:  assignedWahaSession,
			}

			if err := a.stepManager.PrepareAndDispatchStep(seqSub, sub, testStep, target); err != nil {
				a.log.Printf("error sending test sequence message: %v", err)
				return echo.NewHTTPError(http.StatusInternalServerError,
					a.i18n.Ts("campaigns.errorSendTest", "error", err.Error()))
			}
		}
	}

	return c.JSON(http.StatusOK, okResp{true})
}
