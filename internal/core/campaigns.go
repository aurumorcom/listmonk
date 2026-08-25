package core

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/media"
	"github.com/knadh/listmonk/internal/utils"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"gopkg.in/volatiletech/null.v6"
)

const (
	CampaignAnalyticsViews   = "views"
	CampaignAnalyticsClicks  = "clicks"
	CampaignAnalyticsBounces = "bounces"

	campaignTplDefault = "default"
	campaignTplArchive = "archive"
)

// QueryCampaigns retrieves paginated campaigns optionally filtering them by the given arbitrary
// query expression. It also returns the total number of records in the DB.
func (c *Core) QueryCampaigns(searchStr string, statuses, tags []string, orderBy, order string, getAll bool, permittedLists []int, offset, limit int) (models.Campaigns, int, error) {
	queryStr, stmt := makeSearchQuery(searchStr, orderBy, order, c.q.QueryCampaigns, campQuerySortFields)

	if statuses == nil {
		statuses = []string{}
	}

	if tags == nil {
		tags = []string{}
	}

	// Unsafe to ignore scanning fields not present in models.Campaigns.
	var out models.Campaigns
	if err := c.db.Select(&out, stmt, 0, pq.StringArray(statuses), pq.StringArray(tags), queryStr, getAll, pq.Array(permittedLists), offset, limit); err != nil {
		c.log.Printf("error fetching campaigns: %v", err)
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	}

	for i := range out {
		// Replace null tags.
		if out[i].Tags == nil {
			out[i].Tags = []string{}
		}
	}

	// Lazy load stats.
	if err := out.LoadStats(c.q.GetCampaignStats); err != nil {
		c.log.Printf("error fetching campaign stats: %v", err)
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.campaigns}", "error", pqErrMsg(err)))
	}

	total := 0
	if len(out) > 0 {
		total = out[0].Total
	}

	return out, total, nil
}

// GetCampaign retrieves a campaign.
func (c *Core) GetCampaign(id int, uuid, archiveSlug string) (models.Campaign, error) {
	return c.getCampaign(id, uuid, archiveSlug, campaignTplDefault)
}

// GetArchivedCampaign retrieves a campaign with the archive template body.
func (c *Core) GetArchivedCampaign(id int, uuid, archiveSlug string) (models.Campaign, error) {
	out, err := c.getCampaign(id, uuid, archiveSlug, campaignTplArchive)
	if err != nil {
		return out, err
	}

	if !out.Archive {
		return models.Campaign{}, echo.NewHTTPError(http.StatusBadRequest,
			c.i18n.Ts("globals.messages.notFound", "name", "{globals.terms.campaign}"))
	}

	return out, nil
}

// getCampaign retrieves a campaign. If typlType=default, then the campaign's
// template body is returned as "template_body". If tplType="archive",
// the archive template is returned.
func (c *Core) getCampaign(id int, uuid, archiveSlug string, tplType string) (models.Campaign, error) {
	// Unsafe to ignore scanning fields not present in models.Campaigns.
	var uu any
	if uuid != "" {
		uu = uuid
	}

	var out models.Campaigns
	if err := c.q.GetCampaign.Select(&out, id, uu, archiveSlug, tplType); err != nil {
		// if err := c.db.Select(&out, stmt, 0, pq.Array([]string{}), queryStr, 0, 1); err != nil {
		c.log.Printf("error fetching campaign: %v", err)
		return models.Campaign{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	}

	if len(out) == 0 {
		return models.Campaign{}, echo.NewHTTPError(http.StatusBadRequest,
			c.i18n.Ts("globals.messages.notFound", "name", "{globals.terms.campaign}"))
	}

	for i := 0; i < len(out); i++ {
		// Replace null tags.
		if out[i].Tags == nil {
			out[i].Tags = []string{}
		}
	}

	// Lazy load stats.
	if err := out.LoadStats(c.q.GetCampaignStats); err != nil {
		c.log.Printf("error fetching campaign stats: %v", err)
		return models.Campaign{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	}

	return out[0], nil
}

// GetCampaignForPreview retrieves a campaign with a template body. If the optional tplID is > 0
// that particular template is used, otherwise, the template saved on the campaign is.
func (c *Core) GetCampaignForPreview(id, tplID int) (models.Campaign, error) {
	var out models.Campaign
	if err := c.q.GetCampaignForPreview.Get(&out, id, tplID); err != nil {
		if err == sql.ErrNoRows {
			return models.Campaign{}, echo.NewHTTPError(http.StatusBadRequest,
				c.i18n.Ts("globals.messages.notFound", "name", "{globals.terms.campaign}"))
		}

		c.log.Printf("error fetching campaign: %v", err)
		return models.Campaign{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	}

	return out, nil
}

// GetArchivedCampaigns retrieves campaigns with a template body.
func (c *Core) GetArchivedCampaigns(offset, limit int) (models.Campaigns, int, error) {
	var out models.Campaigns
	if err := c.q.GetArchivedCampaigns.Select(&out, offset, limit, campaignTplArchive); err != nil {
		c.log.Printf("error fetching public campaigns: %v", err)
		return models.Campaigns{}, 0, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	}

	total := 0
	if len(out) > 0 {
		total = out[0].Total
	}

	return out, total, nil
}

// CreateCampaign creates a new campaign.
func (c *Core) CreateCampaign(o models.Campaign, listIDs []int, mediaIDs []int) (models.Campaign, error) {
	uu, err := uuid.NewV4()
	if err != nil {
		c.log.Printf("error generating UUID: %v", err)
		return models.Campaign{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUUID", "error", err.Error()))
	}

	// Insert and read ID.
	var newID int
	if err := c.q.CreateCampaign.Get(&newID,
		uu,
		o.Type,
		o.Name,
		o.Subject,
		o.FromEmail,
		o.Body,
		o.AltBody,
		o.ContentType,
		o.SendAt,
		o.Headers,
		o.Attribs,
		pq.StringArray(normalizeTags(o.Tags)),
		o.Messenger,
		o.TemplateID,
		pq.Array(listIDs),
		o.Archive,
		o.ArchiveSlug,
		o.ArchiveTemplateID,
		o.ArchiveMeta,
		pq.Array(mediaIDs),
		o.BodySource,
		o.ScheduleID,
		o.SendWindow,
		o.EmailIDs,
		o.WahaSessions,
	); err != nil {
		if err == sql.ErrNoRows {
			return models.Campaign{}, echo.NewHTTPError(http.StatusBadRequest, c.i18n.T("campaigns.noSubs"))
		}

		c.log.Printf("error creating campaign: %v", err)
		return models.Campaign{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorCreating", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	}

	out, err := c.GetCampaign(newID, "", "")
	if err != nil {
		return models.Campaign{}, err
	}

	_ = c.DispatchWebhookEvent("campaign.created", out)
	return out, nil
}

// UpdateCampaign updates a campaign.
func (c *Core) UpdateCampaign(id int, o models.Campaign, listIDs []int, mediaIDs []int) (models.Campaign, error) {
	_, err := c.q.UpdateCampaign.Exec(id,
		o.Name,
		o.Subject,
		o.FromEmail,
		o.Body,
		o.AltBody,
		o.ContentType,
		o.SendAt,
		o.Headers,
		o.Attribs,
		pq.StringArray(normalizeTags(o.Tags)),
		o.Messenger,
		o.TemplateID,
		pq.Array(listIDs),
		o.Archive,
		o.ArchiveSlug,
		o.ArchiveTemplateID,
		o.ArchiveMeta,
		pq.Array(mediaIDs),
		o.BodySource,
		o.ScheduleID,
		o.SendWindow,
		o.EmailIDs,
		o.WahaSessions,
	)
	if err != nil {
		c.log.Printf("error updating campaign: %v", err)
		return models.Campaign{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	}

	out, err := c.GetCampaign(id, "", "")
	if err != nil {
		return models.Campaign{}, err
	}

	_ = c.DispatchWebhookEvent("campaign.updated", out)
	return out, nil
}

// UpdateCampaignStatus updates a campaign's status, eg: draft to running.
func (c *Core) UpdateCampaignStatus(id int, status string) (models.Campaign, error) {
	cm, err := c.GetCampaign(id, "", "")
	if err != nil {
		return models.Campaign{}, err
	}

	errMsg := ""
	switch status {
	case models.CampaignStatusDraft:
		if cm.Status != models.CampaignStatusScheduled {
			errMsg = c.i18n.T("campaigns.onlyScheduledAsDraft")
		}
	case models.CampaignStatusScheduled:
		if cm.Status != models.CampaignStatusDraft && cm.Status != models.CampaignStatusPaused {
			errMsg = c.i18n.T("campaigns.onlyDraftAsScheduled")
		}
		if !cm.SendAt.Valid {
			errMsg = c.i18n.T("campaigns.needsSendAt")
		}

	case models.CampaignStatusRunning:
		if cm.Status != models.CampaignStatusPaused && cm.Status != models.CampaignStatusDraft {
			errMsg = c.i18n.T("campaigns.onlyPausedDraft")
		}
	case models.CampaignStatusPaused:
		if cm.Status != models.CampaignStatusRunning {
			errMsg = c.i18n.T("campaigns.onlyActivePause")
		}
	case models.CampaignStatusCancelled:
		if cm.Status != models.CampaignStatusRunning && cm.Status != models.CampaignStatusPaused {
			errMsg = c.i18n.T("campaigns.onlyActiveCancel")
		}
	}

	if len(errMsg) > 0 {
		return models.Campaign{}, echo.NewHTTPError(http.StatusBadRequest, errMsg)
	}

	res, err := c.q.UpdateCampaignStatus.Exec(cm.ID, status)
	if err != nil {
		c.log.Printf("error updating campaign status: %v", err)

		return models.Campaign{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	}

	if n, _ := res.RowsAffected(); n == 0 {
		return models.Campaign{}, echo.NewHTTPError(http.StatusBadRequest,
			c.i18n.Ts("globals.messages.notFound", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	}

	cm.Status = status
	_ = c.DispatchWebhookEvent("campaign.status_changed", cm)
	if status == models.CampaignStatusFinished || status == "sent" {
		_ = c.DispatchWebhookEvent("campaign.sent", cm)
	}
	return cm, nil
}

// UpdateCampaignArchive updates a campaign's archive properties.
func (c *Core) UpdateCampaignArchive(id int, enabled bool, tplID int, meta models.JSON, archiveSlug string) error {
	if _, err := c.q.UpdateCampaignArchive.Exec(id, enabled, archiveSlug, tplID, meta); err != nil {
		c.log.Printf("error updating campaign: %v", err)

		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	}

	return nil
}

// DeleteCampaign deletes a campaign.
func (c *Core) DeleteCampaign(id int) error {
	res, err := c.q.DeleteCampaign.Exec(id)
	if err != nil {
		c.log.Printf("error deleting campaign: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorDeleting", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))

	}

	if n, _ := res.RowsAffected(); n == 0 {
		return echo.NewHTTPError(http.StatusBadRequest,
			c.i18n.Ts("globals.messages.notFound", "name", "{globals.terms.campaign}"))
	}

	_ = c.DispatchWebhookEvent("campaign.deleted", map[string]any{"id": id})
	return nil
}

// DeleteCampaigns deletes multiple campaigns by IDs or by query.
func (c *Core) DeleteCampaigns(ids []int, query string, hasAllPerm bool, permittedLists []int) error {
	var queryStr string

	if len(ids) > 0 {
		queryStr = ""
	} else {
		queryStr = makeSearchString(query)
	}

	if _, err := c.q.DeleteCampaigns.Exec(pq.Array(ids), queryStr, hasAllPerm, pq.Array(permittedLists)); err != nil {
		c.log.Printf("error deleting campaigns: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorDeleting", "name", "{globals.terms.campaigns}", "error", pqErrMsg(err)))
	}

	_ = c.DispatchWebhookEvent("campaign.deleted", map[string]any{"ids": ids})
	return nil
}

// CampaignHasLists checks if a campaign has any of the given list IDs.
func (c *Core) CampaignHasLists(id int, listIDs []int) (bool, error) {
	has := false
	if err := c.q.CampaignHasLists.Get(&has, id, pq.Array(listIDs)); err != nil {
		c.log.Printf("error checking campaign lists: %v", err)
		return false, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	}

	return has, nil
}

// GetRunningCampaignStats returns the progress stats of running campaigns.
func (c *Core) GetRunningCampaignStats() ([]models.CampaignStats, error) {
	out := []models.CampaignStats{}
	if err := c.q.GetCampaignStatus.Select(&out, models.CampaignStatusRunning); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		c.log.Printf("error fetching campaign stats: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	} else if len(out) == 0 {
		return nil, nil
	}

	return out, nil
}

func (c *Core) GetCampaignAnalyticsCounts(campIDs []int, typ, fromDate, toDate string) ([]models.CampaignAnalyticsCount, error) {
	// Pick campaign view counts or click counts.
	var stmt *sqlx.Stmt
	switch typ {
	case "views":
		stmt = c.q.GetCampaignViewCounts
	case "clicks":
		stmt = c.q.GetCampaignClickCounts
	case "bounces":
		stmt = c.q.GetCampaignBounceCounts
	default:
		return nil, echo.NewHTTPError(http.StatusBadRequest, c.i18n.T("globals.messages.invalidData"))
	}

	if !strHasLen(fromDate, 10, 30) || !strHasLen(toDate, 10, 30) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, c.i18n.T("analytics.invalidDates"))
	}

	out := []models.CampaignAnalyticsCount{}
	if err := stmt.Select(&out, pq.Array(campIDs), fromDate, toDate); err != nil {
		c.log.Printf("error fetching campaign %s: %v", typ, err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.analytics}", "error", pqErrMsg(err)))
	}

	return out, nil
}

// GetCampaignAnalyticsLinks returns link click analytics for the given campaign IDs.
func (c *Core) GetCampaignAnalyticsLinks(campIDs []int, typ, fromDate, toDate string) ([]models.CampaignAnalyticsLink, error) {
	out := []models.CampaignAnalyticsLink{}
	if err := c.q.GetCampaignLinkCounts.Select(&out, pq.Array(campIDs), fromDate, toDate); err != nil {
		c.log.Printf("error fetching campaign %s: %v", typ, err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.analytics}", "error", pqErrMsg(err)))
	}

	return out, nil
}

// RegisterCampaignView registers a subscriber's view on a campaign with client metadata.
func (c *Core) RegisterCampaignView(campUUID, subUUID string, meta ClientMeta) error {
	if _, err := c.q.RegisterCampaignView.Exec(
		campUUID,
		subUUID,
		meta.IPAddress,
		meta.GeoCountry,
		meta.GeoRegion,
		meta.GeoCity,
		meta.UserAgent,
		meta.DeviceType,
		meta.ClientOS,
		meta.ClientBrowser,
		meta.IsProxy,
		meta.IsBot,
		meta.BotType,
		meta.SequenceStepID,
		meta.VariantID,
	); err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Column == "campaign_id" {
			return nil
		}

		c.log.Printf("error registering campaign view: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	}
	_ = c.DispatchWebhookEvent("campaign.viewed", map[string]any{"campaign_uuid": campUUID, "subscriber_uuid": subUUID})
	return nil
}

// CreateLink registers a URL with a UUID for tracking clicks and returns the UUID.
func (c *Core) CreateLink(url string) (string, error) {
	if c == nil || c.q == nil || c.q.CreateLink == nil {
		return "", fmt.Errorf("core or query not initialized")
	}
	uu, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	var out string
	if err := c.q.CreateLink.Get(&out, uu, url); err != nil {
		return "", err
	}

	return out, nil
}

// GetLinkURL returns the original URL for a link UUID without recording a click.
func (c *Core) GetLinkURL(linkUUID string) (string, error) {
	var url string
	if err := c.q.GetLinkURL.Get(&url, linkUUID); err != nil {
		c.log.Printf("error getting link URL: %s", err)
		return "", echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return url, nil
}

// GetOrCreateLinkID gets or creates the link record in the links table and returns its integer ID.
func (c *Core) GetOrCreateLinkID(rawURL string) (int, error) {
	if c == nil || c.db == nil {
		return 0, fmt.Errorf("core or database not initialized")
	}
	var linkID int
	err := c.db.Get(&linkID, `INSERT INTO links (uuid, url) VALUES(gen_random_uuid(), $1) ON CONFLICT (url) DO UPDATE SET url=EXCLUDED.url RETURNING id`, rawURL)
	if err != nil {
		err = c.db.Get(&linkID, `SELECT id FROM links WHERE url = $1 LIMIT 1`, rawURL)
	}
	return linkID, err
}

// EncodeShortLink encodes link parameters into a Sqids short link token.
func (c *Core) EncodeShortLink(linkID int, isSequence bool, entityID int, subscriberID int, stepID ...int) string {
	return utils.EncodeSqidsLink(linkID, isSequence, entityID, subscriberID, stepID...)
}

// DecodeShortLink decodes a Sqids short link token back into its constituent parameters.
func (c *Core) DecodeShortLink(token string) (utils.ShortLinkPayload, error) {
	return utils.DecodeSqidsLink(token)
}

// ResolveLinkURL returns the destination URL for an integer link ID.
func (c *Core) ResolveLinkURL(linkID int) (string, error) {
	if c == nil || c.db == nil || linkID <= 0 {
		return "", fmt.Errorf("invalid link id or core not initialized")
	}
	var url string
	err := c.db.Get(&url, `SELECT url FROM links WHERE id = $1 LIMIT 1`, linkID)
	return url, err
}

// RegisterShortLinkClick registers a link click event for a resolved short link payload.
func (c *Core) RegisterShortLinkClick(payload utils.ShortLinkPayload, meta ClientMeta, subID int) error {
	if c == nil || c.db == nil || payload.LinkID <= 0 {
		return nil
	}

	var utmsJSON []byte
	if len(meta.UTMParams) > 0 {
		utmsJSON, _ = json.Marshal(meta.UTMParams)
	} else {
		utmsJSON = []byte("{}")
	}

	var campID int
	if !payload.IsSequence {
		campID = payload.EntityID
	}

	_, err := c.db.Exec(`
		INSERT INTO link_clicks (
			campaign_id, subscriber_id, link_id, ip_address, geo_country, geo_region, geo_city, geo_asn,
			user_agent, device_type, client_os, client_browser, email_client, is_bot, bot_type, sequence_step_id,
			variant_id, link_position, utm_params
		) VALUES (
			NULLIF($1, 0), NULLIF($2, 0), $3, NULLIF($4::TEXT, '')::INET, NULLIF($5::TEXT, ''), NULLIF($6::TEXT, ''),
			NULLIF($7::TEXT, ''), NULLIF($8::TEXT, ''), NULLIF($9::TEXT, ''), NULLIF($10::TEXT, ''), NULLIF($11::TEXT, ''),
			NULLIF($12::TEXT, ''), NULLIF($13::TEXT, ''), $14, NULLIF($15::TEXT, ''), NULLIF($16, 0),
			NULLIF($17::TEXT, ''), NULLIF($18::TEXT, ''), COALESCE($19::JSONB, '{}'::JSONB)
		)`,
		campID,
		subID,
		payload.LinkID,
		meta.IPAddress,
		meta.GeoCountry,
		meta.GeoRegion,
		meta.GeoCity,
		meta.GeoASN,
		meta.UserAgent,
		meta.DeviceType,
		meta.ClientOS,
		meta.ClientBrowser,
		meta.EmailClient,
		meta.IsBot,
		meta.BotType,
		meta.SequenceStepID,
		meta.VariantID,
		meta.LinkPosition,
		string(utmsJSON),
	)

	return err
}

// RegisterCampaignLinkClick registers a subscriber's link click on a campaign with client metadata.
func (c *Core) RegisterCampaignLinkClick(linkUUID, campUUID, subUUID string, meta ClientMeta) (string, error) {
	var (
		url      string
		utmsJSON []byte
	)
	if len(meta.UTMParams) > 0 {
		utmsJSON, _ = json.Marshal(meta.UTMParams)
	} else {
		utmsJSON = []byte("{}")
	}

	if err := c.q.RegisterLinkClick.Get(&url,
		linkUUID,
		campUUID,
		subUUID,
		meta.IPAddress,
		meta.GeoCountry,
		meta.GeoRegion,
		meta.GeoCity,
		meta.GeoASN,
		meta.UserAgent,
		meta.DeviceType,
		meta.ClientOS,
		meta.ClientBrowser,
		meta.EmailClient,
		meta.IsBot,
		meta.BotType,
		meta.SequenceStepID,
		meta.VariantID,
		meta.LinkPosition,
		string(utmsJSON),
	); err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Column == "link_id" {
			return "", echo.NewHTTPError(http.StatusBadRequest, c.i18n.Ts("public.invalidLink"))
		}

		c.log.Printf("error registering link click: %s", err)
		return "", echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}

	_ = c.DispatchWebhookEvent("campaign.clicked", map[string]any{"campaign_uuid": campUUID, "subscriber_uuid": subUUID, "url": url})
	return url, nil
}

// ExportCampaignViews returns an iterator with campaign views for streaming/exporting.
func (c *Core) ExportCampaignViews(since time.Time, batchSize int) func() ([]models.CampaignViewExport, error) {
	offset := 0
	return func() ([]models.CampaignViewExport, error) {
		var out []models.CampaignViewExport
		if err := c.q.ExportCampaignViews.Select(&out, since, batchSize, offset); err != nil {
			c.log.Printf("error exporting campaign views: %v", err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError,
				c.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.analytics}", "error", pqErrMsg(err)))
		}
		offset += len(out)
		return out, nil
	}
}

// ExportCampaignLinkClicks returns an iterator with campaign link click for streaming/exporting.
func (c *Core) ExportCampaignLinkClicks(since time.Time, batchSize int) func() ([]models.CampaignClickExport, error) {
	offset := 0
	return func() ([]models.CampaignClickExport, error) {
		var out []models.CampaignClickExport
		if err := c.q.ExportCampaignLinkClicks.Select(&out, since, batchSize, offset); err != nil {
			c.log.Printf("error exporting campaign link clicks: %v", err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError,
				c.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.analytics}", "error", pqErrMsg(err)))
		}
		offset += len(out)
		return out, nil
	}
}

// DeleteCampaignViews deletes campaign views older than a given date.
func (c *Core) DeleteCampaignViews(before time.Time) error {
	if _, err := c.q.DeleteCampaignViews.Exec(before); err != nil {
		c.log.Printf("error deleting campaign views: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}

	return nil
}

// DeleteCampaignLinkClicks deletes campaign views older than a given date.
func (c *Core) DeleteCampaignLinkClicks(before time.Time) error {
	if _, err := c.q.DeleteCampaignLinkClicks.Exec(before); err != nil {
		c.log.Printf("error deleting campaign link clicks: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}

	return nil
}

// GetSequences returns a list of sequence campaigns.
func (c *Core) GetSequences() ([]models.Campaign, error) {
	var out []models.Campaign
	err := c.db.Select(&out, `SELECT c.id, c.uuid, c.name, c.status, c.schedule_id, c.send_window, c.email_ids, c.waha_sessions, c.archive, c.archive_template_id, c.archive_slug, c.archive_meta, c.created_at, c.updated_at,
	(
		SELECT COALESCE(ARRAY_TO_JSON(ARRAY_AGG(l)), '[]') FROM (
			SELECT COALESCE(campaign_lists.list_id, 0) AS id,
			campaign_lists.list_name AS name
			FROM campaign_lists WHERE campaign_lists.campaign_id = c.id
		) l
	) AS lists
	FROM campaigns c
	WHERE c.type = 'sequence'
	ORDER BY c.id DESC`)
	if err != nil {
		c.log.Printf("error querying sequences: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	for i := range out {
		if out[i].ScheduleID.Valid {
			s, err := c.GetSchedule(out[i].ScheduleID.Int, "")
			if err == nil {
				out[i].Schedule = s
			}
		}
	}
	return out, nil
}

// GetSequence returns a sequence campaign by ID or UUID.
func (c *Core) GetSequence(id int, uid string) (*models.Campaign, error) {
	var seq models.Campaign
	err := c.db.Get(&seq, `SELECT c.id, c.uuid, c.name, c.status, c.schedule_id, c.send_window, c.email_ids, c.waha_sessions, c.archive, c.archive_template_id, c.archive_slug, c.archive_meta, c.created_at, c.updated_at,
	(
		SELECT COALESCE(ARRAY_TO_JSON(ARRAY_AGG(l)), '[]') FROM (
			SELECT COALESCE(campaign_lists.list_id, 0) AS id,
			campaign_lists.list_name AS name
			FROM campaign_lists WHERE campaign_lists.campaign_id = c.id
		) l
	) AS lists
	FROM campaigns c
	WHERE (c.id = $1 OR c.uuid::text = $2) AND c.type = 'sequence'`, id, uid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, echo.NewHTTPError(http.StatusNotFound, c.i18n.Ts("globals.messages.notFound"))
		}
		c.log.Printf("error getting sequence: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	if seq.ScheduleID.Valid {
		s, err := c.GetSchedule(seq.ScheduleID.Int, "")
		if err == nil {
			seq.Schedule = s
		}
	}
	return &seq, nil
}

func getFirstIntSlice(slices [][]int) []int {
	if len(slices) > 0 {
		return slices[0]
	}
	return nil
}

// CreateSequence creates a new sequence campaign.
func (c *Core) CreateSequence(seq models.Campaign, listIDs ...[]int) (*models.Campaign, error) {
	seq.Type = models.CampaignTypeSequence
	out, err := c.CreateCampaign(seq, getFirstIntSlice(listIDs), nil)
	if err != nil {
		return nil, err
	}
	if err := c.syncSequenceLists(out.ID, getFirstIntSlice(listIDs), seq.Status); err != nil {
		c.log.Printf("error syncing sequence lists on create: %v", err)
	}
	res, _ := c.GetSequence(out.ID, "")
	if res != nil {
		_ = c.DispatchWebhookEvent("sequence.created", res)
	}
	return res, nil
}

// UpdateSequence updates an existing sequence campaign.
func (c *Core) UpdateSequence(seq models.Campaign, listIDs ...[]int) (*models.Campaign, error) {
	if len(seq.EmailIDs) == 0 {
		seq.EmailIDs = pq.Int64Array{}
	}
	if len(seq.WahaSessions) == 0 {
		seq.WahaSessions = pq.StringArray{}
	}
	if seq.SendWindow == nil {
		seq.SendWindow = models.JSON{}
	}
	if seq.ArchiveMeta == nil {
		seq.ArchiveMeta = json.RawMessage("{}")
	}
	_, err := c.db.Exec(`UPDATE campaigns
		SET name = $2, status = $3, schedule_id = $4, send_window = $5, email_ids = $6, waha_sessions = $7, archive = $8, archive_template_id = $9, archive_slug = $10, archive_meta = $11, updated_at = NOW()
		WHERE id = $1 AND type = 'sequence'`,
		seq.ID, seq.Name, seq.Status, seq.ScheduleID, seq.SendWindow, seq.EmailIDs, seq.WahaSessions, seq.Archive, seq.ArchiveTemplateID, seq.ArchiveSlug, seq.ArchiveMeta)
	if err != nil {
		c.log.Printf("error updating sequence: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}

	targetLists := getFirstIntSlice(listIDs)
	if targetLists != nil {
		if err := c.syncSequenceLists(seq.ID, targetLists, seq.Status); err != nil {
			c.log.Printf("error syncing sequence lists on update: %v", err)
		}
	}

	return c.GetSequence(seq.ID, "")
}

func (c *Core) syncSequenceLists(seqID int, listIDs []int, status string) error {
	_, err := c.db.Exec("DELETE FROM campaign_lists WHERE campaign_id = $1", seqID)
	if err != nil {
		return err
	}
	if len(listIDs) > 0 {
		_, err = c.db.Exec(`INSERT INTO campaign_lists (campaign_id, list_id, list_name)
			SELECT $1, id, name FROM lists WHERE id = ANY($2::INT[])`, seqID, pq.Array(listIDs))
		if err != nil {
			return err
		}
	}

	if (status == models.CampaignStatusRunning || status == models.CampaignStatusScheduled || status == "active") && len(listIDs) > 0 {
		_, err = c.db.Exec(`INSERT INTO campaign_subscribers (campaign_id, subscriber_id, status, current_step, next_send_at)
			SELECT DISTINCT cl.campaign_id, subl.subscriber_id, 'scheduled', 1, NOW()
			FROM campaign_lists cl
			JOIN lists l ON l.id = cl.list_id
			JOIN subscriber_lists subl ON subl.list_id = cl.list_id
				AND (
					(l.optin = 'double' AND subl.status = 'confirmed') OR
					(l.optin != 'double' AND subl.status != 'unsubscribed')
				)
			JOIN subscribers s ON s.id = subl.subscriber_id AND s.status = 'enabled'
			WHERE cl.campaign_id = $1
			ON CONFLICT (campaign_id, subscriber_id) DO UPDATE SET
				status = CASE
					WHEN campaign_subscribers.status = 'opted_out' THEN 'scheduled'
					ELSE campaign_subscribers.status
				END,
				next_send_at = CASE
					WHEN campaign_subscribers.status = 'opted_out' THEN NOW()
					ELSE campaign_subscribers.next_send_at
				END`, seqID)
		if err != nil {
			return err
		}
	}

	_, err = c.db.Exec(`UPDATE campaign_subscribers cs
		SET status = 'opted_out'
		WHERE cs.campaign_id = $1
		  AND cs.status IN ('scheduled', 'in_progress')
		  AND NOT EXISTS (
		      SELECT 1 FROM campaign_lists cl
		      JOIN lists l ON l.id = cl.list_id
		      JOIN subscriber_lists subl ON subl.list_id = cl.list_id
		          AND (
		              (l.optin = 'double' AND subl.status = 'confirmed') OR
		              (l.optin != 'double' AND subl.status != 'unsubscribed')
		          )
		      WHERE cl.campaign_id = cs.campaign_id AND subl.subscriber_id = cs.subscriber_id
		  )`, seqID)
	return err
}

// EnrollListSubscribersInActiveSequences enrolls active subscribers from targeted lists into active sequences.
func EnrollListSubscribersInActiveSequences(c *Core, subIDs []int, listIDs []int, userContext map[string]any) error {
	if userContext == nil {
		return c.EnrollSubscribersByList(subIDs, listIDs)
	}
	return c.EnrollSubscribersByList(subIDs, listIDs, userContext)
}

// DisenrollListSubscribersFromSequences marks subscribers as opted_out when removed from active sequence lists.
func DisenrollListSubscribersFromSequences(c *Core, subIDs []int, listIDs []int) error {
	return c.OptOutSubscribersByList(subIDs, listIDs)
}

// LockSequenceChannelSender locks assigned email account or WAHA session for sequence subscribers.
func LockSequenceChannelSender(c *Core, sequenceID int, subscriberIDs []int, userContext map[string]any) error {
	return c.EnrollCampaignSubscribers(sequenceID, subscriberIDs, userContext)
}

// EnrollSubscribersByList enrolls active subscribers for given list IDs into all active sequence campaigns targeting those lists.
func (c *Core) EnrollSubscribersByList(subIDs []int, listIDs []int, userContext ...map[string]any) error {
	if len(subIDs) == 0 || len(listIDs) == 0 {
		return nil
	}

	var (
		explicitEmailID     null.Int
		explicitWahaSession null.String
	)

	if len(userContext) > 0 && len(userContext[0]) > 0 {
		ctx := userContext[0]
		if rawEID, ok := ctx["email_id"].(float64); ok && rawEID > 0 {
			explicitEmailID = null.IntFrom(int(rawEID))
		} else if rawEIDInt, ok := ctx["email_id"].(int); ok && rawEIDInt > 0 {
			explicitEmailID = null.IntFrom(rawEIDInt)
		}

		if rawWS, ok := ctx["waha_session"].(string); ok && strings.TrimSpace(rawWS) != "" {
			explicitWahaSession = null.StringFrom(strings.TrimSpace(rawWS))
		}

		var uid int
		if rawID, ok := ctx["id"].(float64); ok && rawID > 0 {
			uid = int(rawID)
		} else if rawIDInt, ok := ctx["id"].(int); ok && rawIDInt > 0 {
			uid = rawIDInt
		}

		if uid <= 0 {
			var emailStr, phoneStr string
			if rawEmail, ok := ctx["email"].(string); ok {
				emailStr = strings.TrimSpace(rawEmail)
			}
			if rawPhone, ok := ctx["phone"].(string); ok {
				phoneStr = strings.TrimSpace(rawPhone)
			}
			if emailStr != "" || phoneStr != "" {
				if u, err := c.GetUserByEmailOrPhone(emailStr, phoneStr); err == nil && u.ID > 0 {
					uid = u.ID
				}
			}
		}

		if uid > 0 {
			var u auth.User
			if err := c.db.Get(&u, "SELECT id, email_id, waha_session FROM users WHERE id = $1", uid); err == nil {
				if u.EmailID.Valid && !explicitEmailID.Valid {
					explicitEmailID = u.EmailID
				}
				if u.WahaSession.Valid && u.WahaSession.String != "" && (!explicitWahaSession.Valid || explicitWahaSession.String == "") {
					explicitWahaSession = u.WahaSession
				}
			}
			if !explicitEmailID.Valid {
				var emailAccountID int
				if err := c.db.Get(&emailAccountID, "SELECT id FROM emails WHERE user_id = $1 ORDER BY id ASC LIMIT 1", uid); err == nil && emailAccountID > 0 {
					explicitEmailID = null.IntFrom(emailAccountID)
				}
			}
		}
	}

	var mbVal any
	if explicitEmailID.Valid {
		mbVal = explicitEmailID.Int
	}
	var wsVal any
	if explicitWahaSession.Valid && explicitWahaSession.String != "" {
		wsVal = explicitWahaSession.String
	}

	_, err := c.db.Exec(`INSERT INTO campaign_subscribers (campaign_id, subscriber_id, email_id, waha_session, status, current_step, next_send_at)
		SELECT DISTINCT cl.campaign_id, s.id, $3::INT, $4::TEXT, 'scheduled', 1, NOW()
		FROM subscribers s
		JOIN subscriber_lists subl ON subl.subscriber_id = s.id
		JOIN campaign_lists cl ON cl.list_id = subl.list_id
		JOIN lists l ON l.id = cl.list_id
			AND (
				(l.optin = 'double' AND subl.status = 'confirmed') OR
				(l.optin != 'double' AND subl.status != 'unsubscribed')
			)
		JOIN campaigns camp ON camp.id = cl.campaign_id AND camp.type = 'sequence' AND camp.status = 'running'
		WHERE s.id = ANY($1::INT[]) AND subl.list_id = ANY($2::INT[]) AND s.status = 'enabled'
		ON CONFLICT (campaign_id, subscriber_id) DO UPDATE SET
			status = CASE
				WHEN campaign_subscribers.status = 'opted_out' THEN 'scheduled'
				ELSE campaign_subscribers.status
			END,
			next_send_at = CASE
				WHEN campaign_subscribers.status = 'opted_out' THEN NOW()
				ELSE campaign_subscribers.next_send_at
			END,
			email_id = COALESCE(campaign_subscribers.email_id, EXCLUDED.email_id),
			waha_session = COALESCE(campaign_subscribers.waha_session, EXCLUDED.waha_session)`,
		pq.Array(subIDs), pq.Array(listIDs), mbVal, wsVal)
	if err != nil {
		c.log.Printf("error auto-enrolling subscribers by list into active sequences: %v", err)
		return err
	}
	return nil
}

// OptOutSubscribersByList marks sequence contacts as opted_out if they no longer belong to any active list attached to the sequence campaign.
func (c *Core) OptOutSubscribersByList(subIDs []int, listIDs []int) error {
	if len(subIDs) == 0 || len(listIDs) == 0 {
		return nil
	}
	_, err := c.db.Exec(`UPDATE campaign_subscribers cs
		SET status = 'opted_out'
		WHERE cs.subscriber_id = ANY($1::INT[])
		  AND cs.status IN ('scheduled', 'in_progress')
		  AND cs.campaign_id IN (
		      SELECT cl.campaign_id FROM campaign_lists cl WHERE cl.list_id = ANY($2::INT[])
		  )
		  AND NOT EXISTS (
		      SELECT 1 FROM campaign_lists cl2
		      JOIN lists l2 ON l2.id = cl2.list_id
		      JOIN subscriber_lists subl ON subl.list_id = cl2.list_id
		          AND (
		              (l2.optin = 'double' AND subl.status = 'confirmed') OR
		              (l2.optin != 'double' AND subl.status != 'unsubscribed')
		          )
		      WHERE cl2.campaign_id = cs.campaign_id AND subl.subscriber_id = cs.subscriber_id
		  )`, pq.Array(subIDs), pq.Array(listIDs))
	if err != nil {
		c.log.Printf("error opting out subscribers from sequences for removed lists: %v", err)
		return err
	}
	return nil
}

// UpdateSequenceStatus updates a sequence campaign's status.
func (c *Core) UpdateSequenceStatus(id int, status string) (*models.Campaign, error) {
	switch status {
	case models.CampaignStatusRunning, models.CampaignStatusPaused, models.CampaignStatusFinished, models.CampaignStatusCancelled, "active":
	default:
		return nil, echo.NewHTTPError(http.StatusBadRequest, c.i18n.Ts("globals.messages.invalidFields", "name", "status"))
	}

	dbStatus := status
	if status == "active" {
		dbStatus = models.CampaignStatusRunning
	}

	_, err := c.db.Exec("UPDATE campaigns SET status = $2, updated_at = NOW() WHERE id = $1 AND type = 'sequence'", id, dbStatus)
	if err != nil {
		c.log.Printf("error updating sequence status: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}

	if dbStatus == models.CampaignStatusRunning {
		_, _ = c.db.Exec(`INSERT INTO campaign_subscribers (campaign_id, subscriber_id, status, current_step, next_send_at)
			SELECT DISTINCT cl.campaign_id, subl.subscriber_id, 'scheduled', 1, NOW()
			FROM campaign_lists cl
			JOIN lists l ON l.id = cl.list_id
			JOIN subscriber_lists subl ON subl.list_id = cl.list_id
				AND (
					(l.optin = 'double' AND subl.status = 'confirmed') OR
					(l.optin != 'double' AND subl.status != 'unsubscribed')
				)
			JOIN subscribers s ON s.id = subl.subscriber_id AND s.status = 'enabled'
			WHERE cl.campaign_id = $1
			ON CONFLICT (campaign_id, subscriber_id) DO UPDATE SET
				status = CASE
					WHEN campaign_subscribers.status = 'opted_out' THEN 'scheduled'
					ELSE campaign_subscribers.status
				END,
				next_send_at = CASE
					WHEN campaign_subscribers.status = 'opted_out' THEN NOW()
					ELSE campaign_subscribers.next_send_at
				END`, id)
	}

	res, err := c.GetSequence(id, "")
	if err == nil && res != nil {
		_ = c.DispatchWebhookEvent("sequence.updated", res)
	}
	return res, err
}

// UpdateSequenceArchive updates sequence web archive settings.
func (c *Core) UpdateSequenceArchive(id int, archive bool, templateID null.Int, meta models.JSON, archiveSlug null.String) error {
	if meta == nil {
		meta = models.JSON{}
	}
	_, err := c.db.Exec(`UPDATE campaigns
		SET archive = $2, archive_template_id = $3, archive_meta = $4, archive_slug = $5, updated_at = NOW()
		WHERE id = $1 AND type = 'sequence'`, id, archive, templateID, meta, archiveSlug)
	if err != nil {
		c.log.Printf("error updating sequence archive: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return nil
}

// DeleteSequence deletes a sequence campaign.
func (c *Core) DeleteSequence(id int) error {
	_, err := c.db.Exec("DELETE FROM campaigns WHERE id = $1 AND type = 'sequence'", id)
	if err != nil {
		c.log.Printf("error deleting sequence: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return nil
}

// DeleteSequences bulk deletes sequence campaigns by ID list.
func (c *Core) DeleteSequences(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := c.db.Exec("DELETE FROM campaigns WHERE id = ANY($1) AND type = 'sequence'", pq.Array(ids))
	if err != nil {
		c.log.Printf("error bulk deleting sequences: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return nil
}

// ManageSubscriberSequences modifies subscriber sequence memberships (enroll, disenroll, pause).
func (c *Core) ManageSubscriberSequences(subscriberIDs []int, sequenceIDs []int, action string, status string) error {
	if len(subscriberIDs) == 0 || len(sequenceIDs) == 0 {
		return nil
	}
	switch action {
	case "add", "enroll":
		if status == "" {
			status = models.CampaignSubscriberStatusScheduled
		}
		for _, seqID := range sequenceIDs {
			_, err := c.db.Exec(`INSERT INTO campaign_subscribers (campaign_id, subscriber_id, status, current_step, next_send_at)
				SELECT $1, id, $3, 1, NOW()
				FROM subscribers
				WHERE id = ANY($2)
				ON CONFLICT (campaign_id, subscriber_id) DO UPDATE SET status = EXCLUDED.status`,
				seqID, pq.Array(subscriberIDs), status)
			if err != nil {
				c.log.Printf("error enrolling subscribers into sequence %d: %v", seqID, err)
				return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
			}
		}
	case "remove", "disenroll":
		_, err := c.db.Exec("DELETE FROM campaign_subscribers WHERE subscriber_id = ANY($1) AND campaign_id = ANY($2)",
			pq.Array(subscriberIDs), pq.Array(sequenceIDs))
		if err != nil {
			c.log.Printf("error disenrolling subscribers: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
		}
	case "pause":
		_, err := c.db.Exec("UPDATE campaign_subscribers SET status = 'paused' WHERE subscriber_id = ANY($1) AND campaign_id = ANY($2)",
			pq.Array(subscriberIDs), pq.Array(sequenceIDs))
		if err != nil {
			c.log.Printf("error pausing subscribers in sequence: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
		}
	default:
		return echo.NewHTTPError(http.StatusBadRequest, c.i18n.Ts("globals.messages.invalidFields", "name", "action"))
	}
	return nil
}

// GetSubscriberSequences returns sequence memberships for a given subscriber ID.
func (c *Core) GetSubscriberSequences(subscriberID int) ([]models.CampaignSubscriber, error) {
	var out []models.CampaignSubscriber
	err := c.db.Select(&out, `SELECT campaign_id, subscriber_id, email_id, waha_session, status, current_step, next_send_at, last_read_at, last_clicked_at, last_message_id, created_at
		FROM campaign_subscribers WHERE subscriber_id = $1 ORDER BY campaign_id ASC`, subscriberID)
	if err != nil {
		c.log.Printf("error getting subscriber sequences: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return out, nil
}

// GetCampaignSteps returns the steps for a given campaign sequence.
func (c *Core) GetCampaignSteps(sequenceID int) ([]models.CampaignStep, error) {
	var steps []models.CampaignStep
	query := `SELECT
		s.id, s.campaign_id, s.step_number, s.delay, s.messenger, s.condition,
		s.subject, s.body, COALESCE(s.email_type, '') AS email_type, s.template_id,
		COALESCE(ARRAY_AGG(m.media_id) FILTER (WHERE m.media_id IS NOT NULL), '{}') AS media_ids
	FROM campaign_steps s
	LEFT JOIN campaign_step_media m ON s.id = m.campaign_step_id
	WHERE s.campaign_id = $1
	GROUP BY s.id
	ORDER BY s.step_number ASC`
	err := c.db.Select(&steps, query, sequenceID)
	if err != nil {
		c.log.Printf("error getting sequence steps: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	return steps, nil
}

// GetSequenceSteps alias for GetCampaignSteps.
func (c *Core) GetSequenceSteps(sequenceID int) ([]models.CampaignStep, error) {
	return c.GetCampaignSteps(sequenceID)
}

// SaveCampaignSteps updates or inserts steps for a campaign sequence.
func (c *Core) SaveCampaignSteps(sequenceID int, steps []models.CampaignStep) error {
	tx, err := c.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM campaign_steps WHERE campaign_id = $1", sequenceID); err != nil {
		return err
	}

	for i, s := range steps {
		s.CampaignID = sequenceID
		s.StepNumber = i + 1
		if s.Delay == "" {
			s.Delay = "0s"
		}
		if s.Messenger == "" {
			s.Messenger = "email"
		}
		if s.Condition == "" {
			s.Condition = models.CampaignConditionAlways
		}
		if s.StepNumber == 1 {
			s.EmailType = ""
		}

		var newID int
		err := tx.Get(&newID, `INSERT INTO campaign_steps (campaign_id, step_number, delay, messenger, condition, subject, body, email_type, template_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
			s.CampaignID, s.StepNumber, s.Delay, s.Messenger, s.Condition, s.Subject, s.Body, s.EmailType, s.TemplateID)
		if err != nil {
			return err
		}

		if len(s.MediaIDs) > 0 {
			_, err = tx.Exec(`INSERT INTO campaign_step_media (campaign_step_id, media_id, filename)
				SELECT $1, id, filename FROM media WHERE id = ANY($2::INT[])`,
				newID, s.MediaIDs)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// SaveSequenceSteps alias for SaveCampaignSteps.
func (c *Core) SaveSequenceSteps(sequenceID int, steps []models.CampaignStep) error {
	return c.SaveCampaignSteps(sequenceID, steps)
}

// GetStepAttachments loads attachments for a given list of media IDs using the media store.
func (c *Core) GetStepAttachments(store media.Store, mediaIDs []int64) ([]models.Attachment, error) {
	if len(mediaIDs) == 0 || store == nil {
		return nil, nil
	}
	var atts []models.Attachment
	for _, mid := range mediaIDs {
		m, err := c.GetMedia(int(mid), "", "", store)
		if err != nil {
			c.log.Printf("error fetching media %d: %v", mid, err)
			continue
		}
		b, err := store.GetBlob(m.URL)
		if err != nil {
			c.log.Printf("error fetching blob for media %d (%s): %v", mid, m.Filename, err)
			continue
		}
		atts = append(atts, models.Attachment{
			Name:    m.Filename,
			Content: b,
		})
	}
	return atts, nil
}

// AllocateSendersRoundRobinInt distributes subscriber IDs round-robin across an integer pool (e.g. user IDs or email account IDs) with an optional starting offset.
func AllocateSendersRoundRobinInt(subIDs []int, pool []int64, offset ...int) map[int]null.Int {
	start := 0
	if len(offset) > 0 {
		start = offset[0]
	}
	alloc := make(map[int]null.Int, len(subIDs))
	if len(pool) == 0 {
		for _, id := range subIDs {
			alloc[id] = null.Int{}
		}
		return alloc
	}
	for i, subID := range subIDs {
		alloc[subID] = null.IntFrom(int(pool[(start+i)%len(pool)]))
	}
	return alloc
}

// AllocateSendersRoundRobinString distributes subscriber IDs round-robin across a string pool (e.g. WAHA sessions) with an optional starting offset.
func AllocateSendersRoundRobinString(subIDs []int, pool []string, offset ...int) map[int]null.String {
	start := 0
	if len(offset) > 0 {
		start = offset[0]
	}
	alloc := make(map[int]null.String, len(subIDs))
	if len(pool) == 0 {
		for _, id := range subIDs {
			alloc[id] = null.String{}
		}
		return alloc
	}
	for i, subID := range subIDs {
		alloc[subID] = null.StringFrom(pool[(start+i)%len(pool)])
	}
	return alloc
}

// AllocateSendersCapacityWeighted distributes subscriber IDs based on remaining email account daily capacity.
func AllocateSendersCapacityWeighted(subIDs []int, emails []models.Email) map[int]null.Int {
	alloc := make(map[int]null.Int, len(subIDs))
	if len(emails) == 0 {
		for _, id := range subIDs {
			alloc[id] = null.Int{}
		}
		return alloc
	}

	type capBox struct {
		id        int64
		remaining int
	}

	var active []capBox
	totalRemaining := 0
	for _, m := range emails {
		rem := 0
		if m.MaxSendPerDay > 0 {
			rem = m.MaxSendPerDay - m.SentToday
		} else {
			rem = 10000
		}
		if rem < 0 {
			rem = 0
		}
		if rem > 0 {
			active = append(active, capBox{id: int64(m.ID), remaining: rem})
			totalRemaining += rem
		}
	}

	if totalRemaining == 0 || len(active) == 0 {
		var pool []int64
		for _, m := range emails {
			pool = append(pool, int64(m.ID))
		}
		return AllocateSendersRoundRobinInt(subIDs, pool)
	}

	n := len(subIDs)
	quotas := make(map[int64]int, len(active))
	assignedCount := 0

	for _, b := range active {
		q := (n * b.remaining) / totalRemaining
		quotas[b.id] = q
		assignedCount += q
	}

	remainder := n - assignedCount
	for i := 0; i < remainder; i++ {
		quotas[active[i%len(active)].id]++
	}

	subIdx := 0
	for _, b := range active {
		q := quotas[b.id]
		for j := 0; j < q && subIdx < len(subIDs); j++ {
			alloc[subIDs[subIdx]] = null.IntFrom(int(b.id))
			subIdx++
		}
	}

	for subIdx < len(subIDs) {
		alloc[subIDs[subIdx]] = null.IntFrom(int(active[subIdx%len(active)].id))
		subIdx++
	}

	return alloc
}

// EnrollCampaignSubscribers enrolls subscribers into a sequence campaign.
func (c *Core) EnrollCampaignSubscribers(sequenceID int, subscriberIDs []int, userContext map[string]any) error {
	if len(subscriberIDs) == 0 {
		return nil
	}

	seq, err := c.GetSequence(sequenceID, "")
	if err != nil {
		return err
	}

	var existingCount int
	_ = c.db.Get(&existingCount, "SELECT COUNT(*) FROM campaign_subscribers WHERE campaign_id = $1", sequenceID)

	var userAlloc map[int]null.Int
	if len(seq.UserIDs) > 0 {
		userAlloc = AllocateSendersRoundRobinInt(subscriberIDs, seq.UserIDs, existingCount)
	}

	var emailAlloc map[int]null.Int
	if len(seq.EmailIDs) > 0 {
		emailAlloc = AllocateSendersRoundRobinInt(subscriberIDs, seq.EmailIDs, existingCount)
	}

	var wahaAlloc map[int]null.String
	if len(seq.WahaSessions) > 0 {
		wahaAlloc = AllocateSendersRoundRobinString(subscriberIDs, seq.WahaSessions, existingCount)
	}

	tx, err := c.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Preparex(`INSERT INTO campaign_subscribers (campaign_id, subscriber_id, user_id, email_id, waha_session, status, current_step, next_send_at)
		VALUES ($1, $2, $3, $4, $5, 'scheduled', 1, NOW())
		ON CONFLICT (campaign_id, subscriber_id) DO UPDATE SET
			user_id = COALESCE(campaign_subscribers.user_id, EXCLUDED.user_id),
			email_id = COALESCE(campaign_subscribers.email_id, EXCLUDED.email_id),
			waha_session = COALESCE(campaign_subscribers.waha_session, EXCLUDED.waha_session)`)
	if err != nil {
		c.log.Printf("error preparing sequence enrollment stmt: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
	}
	defer stmt.Close()

	for _, subID := range subscriberIDs {
		var uVal any
		if userAlloc != nil && userAlloc[subID].Valid {
			uVal = userAlloc[subID].Int
		}
		var mbVal any
		if emailAlloc != nil && emailAlloc[subID].Valid {
			mbVal = emailAlloc[subID].Int
		}
		var wsVal any
		if wahaAlloc != nil && wahaAlloc[subID].Valid && wahaAlloc[subID].String != "" {
			wsVal = wahaAlloc[subID].String
		}

		if _, err := stmt.Exec(sequenceID, subID, uVal, mbVal, wsVal); err != nil {
			c.log.Printf("error enrolling subscriber %d into sequence %d: %v", subID, sequenceID, err)
			return echo.NewHTTPError(http.StatusInternalServerError, c.i18n.Ts("public.errorProcessingRequest"))
		}
	}

	return tx.Commit()
}

// GetDueSequenceSubscribers returns sequence subscribers due for sending for active sequence campaigns.
func (c *Core) GetDueSequenceSubscribers(limit int) ([]models.CampaignSubscriber, error) {
	var out []models.CampaignSubscriber
	err := c.db.Select(&out, `SELECT cs.campaign_id, cs.subscriber_id, cs.email_id, cs.waha_session, cs.status, cs.current_step, cs.next_send_at, cs.last_read_at, cs.last_clicked_at, cs.last_message_id, cs.last_thread_msg_id, cs.created_at
		FROM campaign_subscribers cs
		JOIN campaigns c ON c.id = cs.campaign_id
		WHERE c.type = 'sequence' AND c.status = 'running' AND cs.status IN ('scheduled', 'in_progress') AND cs.next_send_at <= NOW()
		LIMIT $1`, limit)
	if err != nil {
		c.log.Printf("error getting due sequence subscribers: %v", err)
		return nil, err
	}
	return out, nil
}

// UpdateSequenceSubscriberStatus updates progress of a subscriber in a sequence campaign.
func (c *Core) UpdateSequenceSubscriberStatus(sequenceID, subID int, status string, currentStep int, nextSendAt null.Time, lastMsgID null.String, lastThreadMsgID null.String) error {
	_, err := c.db.Exec(`UPDATE campaign_subscribers
		SET status = $3, current_step = $4, next_send_at = $5, last_message_id = $6, last_thread_msg_id = $7
		WHERE campaign_id = $1 AND subscriber_id = $2`,
		sequenceID, subID, status, currentStep, nextSendAt, lastMsgID, lastThreadMsgID)
	return err
}

// RecordSequenceRead records an open/read event for a sequence subscriber.
func (c *Core) RecordSequenceRead(sequenceID, subID int) error {
	if c == nil || c.db == nil {
		return nil
	}
	_, err := c.db.Exec(`UPDATE campaign_subscribers SET last_read_at = NOW() WHERE campaign_id = $1 AND subscriber_id = $2`, sequenceID, subID)
	return err
}

// RecordSequenceReadByPhone marks sequence contacts as read matching a phone number or WhatsApp LID.
func (c *Core) RecordSequenceReadByPhone(phone string, lids ...string) error {
	if c == nil || c.db == nil {
		return nil
	}
	lid := ""
	if len(lids) > 0 {
		lid = lids[0]
	}
	cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(phone, "")
	if cleaned == "" && lid == "" {
		return nil
	}
	_, err := c.db.Exec(`UPDATE campaign_subscribers
		SET last_read_at = NOW()
		WHERE subscriber_id IN (
			SELECT id FROM subscribers
			WHERE (CASE WHEN $1 != '' THEN REGEXP_REPLACE(phone, '[^\d]', '', 'g') = $1 OR REGEXP_REPLACE(attribs->>'phone', '[^\d]', '', 'g') = $1 ELSE FALSE END)
			   OR (CASE WHEN $2 != '' THEN attribs->>'whatsapp_lid' = $2 OR attribs->>'lid' = $2 ELSE FALSE END)
		) AND status IN ('scheduled', 'in_progress')`, cleaned, lid)
	return err
}

// RecordSequenceReadByMessageID marks sequence contacts as read matching a last_message_id or stanzaID.
func (c *Core) RecordSequenceReadByMessageID(msgID string, extra ...string) error {
	if c == nil || c.db == nil || msgID == "" {
		return nil
	}
	stanzaID := ""
	lid := ""
	if len(extra) > 0 {
		stanzaID = extra[0]
	}
	if len(extra) > 1 {
		lid = extra[1]
	}

	var subID int
	err := c.db.QueryRow(`UPDATE campaign_subscribers
		SET last_read_at = NOW()
		WHERE (
			last_message_id = $1
			OR last_thread_msg_id = $1
			OR ($2 != '' AND (last_message_id = $2 OR last_thread_msg_id = $2 OR last_message_id LIKE '%' || $2))
		) AND status IN ('scheduled', 'in_progress')
		RETURNING subscriber_id`, msgID, stanzaID).Scan(&subID)

	if err == nil && subID > 0 && lid != "" {
		_ = c.LinkSubscriberLID(subID, lid)
	}
	if err == sql.ErrNoRows {
		return nil
	}
	return err
}

// RecordSequenceClick records a link click event for a sequence subscriber.
func (c *Core) RecordSequenceClick(sequenceID, subID int) error {
	if c == nil || c.db == nil {
		return nil
	}
	_, err := c.db.Exec(`UPDATE campaign_subscribers SET last_clicked_at = NOW() WHERE campaign_id = $1 AND subscriber_id = $2`, sequenceID, subID)
	return err
}

// RecordSequenceClickByPhone marks sequence contacts as clicked matching a phone number or WhatsApp LID.
func (c *Core) RecordSequenceClickByPhone(phone string, lids ...string) error {
	if c == nil || c.db == nil {
		return nil
	}
	lid := ""
	if len(lids) > 0 {
		lid = lids[0]
	}
	cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(phone, "")
	if cleaned == "" && lid == "" {
		return nil
	}
	_, err := c.db.Exec(`UPDATE campaign_subscribers
		SET last_clicked_at = NOW()
		WHERE subscriber_id IN (
			SELECT id FROM subscribers
			WHERE (CASE WHEN $1 != '' THEN REGEXP_REPLACE(phone, '[^\d]', '', 'g') = $1 OR REGEXP_REPLACE(attribs->>'phone', '[^\d]', '', 'g') = $1 ELSE FALSE END)
			   OR (CASE WHEN $2 != '' THEN attribs->>'whatsapp_lid' = $2 OR attribs->>'lid' = $2 ELSE FALSE END)
		) AND status IN ('scheduled', 'in_progress')`, cleaned, lid)
	return err
}

// RecordSequenceReply marks subscriber status as 'replied' by email.
func (c *Core) RecordSequenceReply(email string) error {
	_, err := c.db.Exec(`UPDATE campaign_subscribers
		SET status = 'replied'
		WHERE subscriber_id = (SELECT id FROM subscribers WHERE email = $1 LIMIT 1)
		  AND status IN ('scheduled', 'in_progress')`, email)
	return err
}

// RecordSequenceReplyByPhone marks subscriber sequence status as 'replied' by phone number or WhatsApp LID.
func (c *Core) RecordSequenceReplyByPhone(identifier string, lids ...string) error {
	if c == nil || c.db == nil {
		return nil
	}
	lid := ""
	if len(lids) > 0 {
		lid = lids[0]
	}
	if strings.Contains(identifier, "@lid") && lid == "" {
		lid = identifier
	}
	cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(identifier, "")
	if cleaned == "" && lid == "" {
		return nil
	}
	_, err := c.db.Exec(`UPDATE campaign_subscribers
		SET status = 'replied'
		WHERE subscriber_id IN (
			SELECT id FROM subscribers
			WHERE (CASE WHEN $1 != '' THEN REGEXP_REPLACE(phone, '[^\d]', '', 'g') = $1 OR REGEXP_REPLACE(attribs->>'phone', '[^\d]', '', 'g') = $1 ELSE FALSE END)
			   OR (CASE WHEN $2 != '' THEN attribs->>'whatsapp_lid' = $2 OR attribs->>'lid' = $2 ELSE FALSE END)
		) AND status IN ('scheduled', 'in_progress')`, cleaned, lid)
	return err
}

// CancelSequenceSubscriberForOptOut cancels active sequence subscribers and unsubscribes the subscriber upon explicit opt-out.
func (c *Core) CancelSequenceSubscriberForOptOut(identifier string, isPhone bool, lids ...string) error {
	if identifier == "" {
		return nil
	}

	lid := ""
	if len(lids) > 0 {
		lid = lids[0]
	}
	if strings.Contains(identifier, "@lid") && lid == "" {
		lid = identifier
	}

	if isPhone || lid != "" {
		cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(identifier, "")
		if cleaned == "" && lid == "" {
			return nil
		}
		_, err := c.db.Exec(`UPDATE campaign_subscribers
			SET status = 'cancelled'
			WHERE subscriber_id IN (
				SELECT id FROM subscribers
				WHERE (CASE WHEN $1 != '' THEN REGEXP_REPLACE(phone, '[^\d]', '', 'g') = $1 OR REGEXP_REPLACE(attribs->>'phone', '[^\d]', '', 'g') = $1 ELSE FALSE END)
				   OR (CASE WHEN $2 != '' THEN attribs->>'whatsapp_lid' = $2 OR attribs->>'lid' = $2 ELSE FALSE END)
			) AND status IN ('scheduled', 'in_progress')`, cleaned, lid)
		if err != nil {
			return err
		}
		_, err = c.db.Exec(`UPDATE subscribers SET status = 'unsubscribed'
			WHERE (CASE WHEN $1 != '' THEN REGEXP_REPLACE(phone, '[^\d]', '', 'g') = $1 OR REGEXP_REPLACE(attribs->>'phone', '[^\d]', '', 'g') = $1 ELSE FALSE END)
			   OR (CASE WHEN $2 != '' THEN attribs->>'whatsapp_lid' = $2 OR attribs->>'lid' = $2 ELSE FALSE END)`, cleaned, lid)
		return err
	}

	_, err := c.db.Exec(`UPDATE campaign_subscribers
		SET status = 'cancelled'
		WHERE subscriber_id = (SELECT id FROM subscribers WHERE email = $1 LIMIT 1)
		  AND status IN ('scheduled', 'in_progress')`, identifier)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(`UPDATE subscribers SET status = 'unsubscribed' WHERE email = $1`, identifier)
	return err
}

// DeferSequenceSubscriberOOO defers active sequence subscribers to a future return date for Out-Of-Office replies.
func (c *Core) DeferSequenceSubscriberOOO(identifier string, isPhone bool, returnDate time.Time, lids ...string) error {
	if identifier == "" {
		return nil
	}

	lid := ""
	if len(lids) > 0 {
		lid = lids[0]
	}
	if strings.Contains(identifier, "@lid") && lid == "" {
		lid = identifier
	}

	if isPhone || lid != "" {
		cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(identifier, "")
		if cleaned == "" && lid == "" {
			return nil
		}
		_, err := c.db.Exec(`UPDATE campaign_subscribers
			SET next_send_at = $3, status = 'in_progress'
			WHERE subscriber_id IN (
				SELECT id FROM subscribers
				WHERE (CASE WHEN $1 != '' THEN REGEXP_REPLACE(phone, '[^\d]', '', 'g') = $1 OR REGEXP_REPLACE(attribs->>'phone', '[^\d]', '', 'g') = $1 ELSE FALSE END)
				   OR (CASE WHEN $2 != '' THEN attribs->>'whatsapp_lid' = $2 OR attribs->>'lid' = $2 ELSE FALSE END)
			) AND status IN ('scheduled', 'in_progress')`, cleaned, lid, returnDate)
		return err
	}

	_, err := c.db.Exec(`UPDATE campaign_subscribers
		SET next_send_at = $2, status = 'in_progress'
		WHERE subscriber_id = (SELECT id FROM subscribers WHERE email = $1 LIMIT 1)
		  AND status IN ('scheduled', 'in_progress')`, identifier, returnDate)
	return err
}

// RecordSequenceStepHistory appends a step execution record to a subscriber's sequence_history attribute.
func (c *Core) RecordSequenceStepHistory(subID int, stepNumber int, messenger, subject, body string) error {
	historyRecord := map[string]any{
		"step_number": stepNumber,
		"step":        stepNumber,
		"messenger":   messenger,
		"subject":     subject,
		"content":     body,
		"message":     body,
		"sent_at":     time.Now().Format(time.RFC3339),
	}

	var rawAttribs []byte
	err := c.db.Get(&rawAttribs, `SELECT COALESCE(attribs, '{}'::jsonb) FROM subscribers WHERE id = $1`, subID)
	if err != nil {
		return err
	}

	var attribs map[string]any
	if err := json.Unmarshal(rawAttribs, &attribs); err != nil {
		attribs = make(map[string]any)
	}

	var historyList []any
	if existingHistory, ok := attribs["sequence_history"].([]any); ok {
		historyList = existingHistory
	}
	historyList = append(historyList, historyRecord)
	attribs["sequence_history"] = historyList

	updatedBytes, err := json.Marshal(attribs)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(`UPDATE subscribers SET attribs = $1::jsonb WHERE id = $2`, string(updatedBytes), subID)
	return err
}

// IsNationalHoliday returns true if date falls on standard national holidays.
func IsNationalHoliday(t time.Time) bool {
	month, day, weekday := t.Month(), t.Day(), t.Weekday()

	if month == time.January && day == 1 {
		return true
	}
	if month == time.July && day == 4 {
		return true
	}
	if month == time.December && (day == 24 || day == 25) {
		return true
	}
	if month == time.May && weekday == time.Monday && day >= 25 {
		return true
	}
	if month == time.September && weekday == time.Monday && day <= 7 {
		return true
	}
	if month == time.November && weekday == time.Thursday && day >= 22 && day <= 28 {
		return true
	}
	return false
}

// IsInsideSchedule checks if current time in target timezone falls within the active Schedule window.
func IsInsideSchedule(sched *models.Schedule, contactLoc *time.Location, now time.Time) (bool, time.Time) {
	if sched == nil {
		return true, now
	}

	loc := time.Local
	if sched.UseSubscriberTimezone && contactLoc != nil {
		loc = contactLoc
	} else if sched.Timezone != "" {
		if l, err := time.LoadLocation(sched.Timezone); err == nil {
			loc = l
		}
	}

	localNow := now.In(loc)
	if sched.SkipHolidays && IsNationalHoliday(localNow) {
		nextDay := localNow.AddDate(0, 0, 1)
		return false, time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 8, 0, 0, 0, loc)
	}

	dayKey := strings.ToLower(localNow.Format("mon"))

	var startStr, endStr string

	if len(sched.SendingWindows) > 0 {
		var raw map[string]interface{}
		if b, err := json.Marshal(sched.SendingWindows); err == nil {
			_ = json.Unmarshal(b, &raw)
		}

		if dayVal, exists := raw[dayKey]; exists && dayVal != nil {
			if m, ok := dayVal.(map[string]interface{}); ok {
				if s, ok := m["start"].(string); ok {
					startStr = s
				}
				if e, ok := m["end"].(string); ok {
					endStr = e
				}
			} else if slice, ok := dayVal.([]interface{}); ok && len(slice) > 0 {
				if m, ok := slice[0].(map[string]interface{}); ok {
					if s, ok := m["start"].(string); ok {
						startStr = s
					}
					if e, ok := m["end"].(string); ok {
						endStr = e
					}
				}
			}
		}
	}

	if startStr == "" || endStr == "" {
		nextDay := localNow.AddDate(0, 0, 1)
		return false, time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 8, 0, 0, 0, loc)
	}

	startHour, startMin := 8, 0
	endHour, endMin := 17, 0
	fmt.Sscanf(startStr, "%d:%d", &startHour, &startMin)
	fmt.Sscanf(endStr, "%d:%d", &endHour, &endMin)

	startTimeToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), startHour, startMin, 0, 0, loc)
	endTimeToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), endHour, endMin, 0, 0, loc)

	if (!localNow.Before(startTimeToday)) && localNow.Before(endTimeToday) {
		return true, localNow
	}

	nextDay := localNow.AddDate(0, 0, 1)
	return false, time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 8, 0, 0, 0, loc)
}

// GetSequenceAnalytics calculates real metrics across sequence contacts and sequence steps.
func (c *Core) GetSequenceAnalytics() (*models.CampaignSequenceAnalytics, error) {
	out := &models.CampaignSequenceAnalytics{
		Funnel: []models.CampaignStepFunnel{},
		AggregatedAnalytics: models.CampaignAnalytics{
			Breakdowns: models.CampaignBreakdownStats{
				Devices:   []models.DeviceBreakdown{},
				Locations: []models.LocationBreakdown{},
				Links:     []models.CampaignAnalyticsLink{},
				Variants:  []models.VariantPerformance{},
				Bots: models.CampaignBotStats{
					BotTypeBreakdown: make(map[string]int),
				},
			},
		},
	}

	err := c.db.Get(&out.ActiveSubscribers, `SELECT COALESCE(COUNT(*), 0) FROM campaign_subscribers WHERE status IN ('scheduled', 'in_progress')`)
	if err != nil && err != sql.ErrNoRows {
		c.log.Printf("error querying active sequence subscribers: %v", err)
	}

	err = c.db.Get(&out.StepCompletions, `SELECT COALESCE(SUM(current_step), 0) FROM campaign_subscribers`)
	if err != nil && err != sql.ErrNoRows {
		c.log.Printf("error querying sequence step completions: %v", err)
	}

	var totalEnrolled, totalReplied, totalFinished int
	_ = c.db.Get(&totalEnrolled, `SELECT COALESCE(COUNT(*), 0) FROM campaign_subscribers`)
	_ = c.db.Get(&totalReplied, `SELECT COALESCE(COUNT(*), 0) FROM campaign_subscribers WHERE status = 'replied'`)
	_ = c.db.Get(&totalFinished, `SELECT COALESCE(COUNT(*), 0) FROM campaign_subscribers WHERE status IN ('replied', 'finished')`)

	if totalEnrolled > 0 {
		out.ReplyRate = (float64(totalReplied) / float64(totalEnrolled)) * 100.0
		out.ConversionRate = (float64(totalFinished) / float64(totalEnrolled)) * 100.0
	}

	viewRow := c.db.QueryRowx(`
		SELECT
			COALESCE(COUNT(*), 0) AS total,
			COALESCE(COUNT(DISTINCT subscriber_id), 0) AS unique_views,
			COALESCE(COUNT(*) FILTER (WHERE is_bot = FALSE), 0) AS human_total,
			COALESCE(COUNT(DISTINCT subscriber_id) FILTER (WHERE is_bot = FALSE), 0) AS human_unique,
			COALESCE(COUNT(*) FILTER (WHERE is_bot = TRUE), 0) AS bot_total,
			COALESCE(COUNT(*) FILTER (WHERE is_proxy = TRUE), 0) AS proxy_mpp_total
		FROM campaign_views
		WHERE sequence_step_id IS NOT NULL`)
	_ = viewRow.Scan(
		&out.AggregatedAnalytics.Views.Total,
		&out.AggregatedAnalytics.Views.Unique,
		&out.AggregatedAnalytics.Views.HumanTotal,
		&out.AggregatedAnalytics.Views.HumanUnique,
		&out.AggregatedAnalytics.Views.BotTotal,
		&out.AggregatedAnalytics.Views.ProxyMPPTotal,
	)

	clickRow := c.db.QueryRowx(`
		SELECT
			COALESCE(COUNT(*), 0) AS total,
			COALESCE(COUNT(DISTINCT subscriber_id), 0) AS unique_clicks,
			COALESCE(COUNT(*) FILTER (WHERE is_bot = FALSE), 0) AS human_total,
			COALESCE(COUNT(DISTINCT subscriber_id) FILTER (WHERE is_bot = FALSE), 0) AS human_unique,
			COALESCE(COUNT(*) FILTER (WHERE is_bot = TRUE), 0) AS bot_clicks
		FROM link_clicks
		WHERE sequence_step_id IS NOT NULL`)
	_ = clickRow.Scan(
		&out.AggregatedAnalytics.Clicks.Total,
		&out.AggregatedAnalytics.Clicks.Unique,
		&out.AggregatedAnalytics.Clicks.HumanTotal,
		&out.AggregatedAnalytics.Clicks.HumanUnique,
		&out.AggregatedAnalytics.Clicks.BotClicks,
	)

	if out.AggregatedAnalytics.Views.HumanUnique > 0 {
		out.AggregatedAnalytics.Clicks.CTOR = (float64(out.AggregatedAnalytics.Clicks.HumanUnique) / float64(out.AggregatedAnalytics.Views.HumanUnique)) * 100.0
	}

	rows, err := c.db.Queryx(`
		SELECT
			st.id,
			st.step_number,
			COALESCE(st.subject, '') AS subject,
			COALESCE(st.messenger, 'email') AS messenger,
			COALESCE((SELECT COUNT(*) FROM campaign_subscribers cs WHERE cs.campaign_id = st.campaign_id AND cs.current_step >= st.step_number), 0) AS reached,
			COALESCE((SELECT COUNT(*) FROM campaign_subscribers cs WHERE cs.campaign_id = st.campaign_id AND cs.current_step = st.step_number AND cs.status = 'replied'), 0) AS replied
		FROM campaign_steps st
		ORDER BY st.campaign_id ASC, st.step_number ASC
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var (
				stepID int
				f      models.CampaignStepFunnel
			)
			f.Analytics = models.CampaignAnalytics{
				Breakdowns: models.CampaignBreakdownStats{
					Bots: models.CampaignBotStats{
						BotTypeBreakdown: make(map[string]int),
					},
				},
			}
			if err := rows.Scan(&stepID, &f.StepNumber, &f.Subject, &f.Messenger, &f.Reached, &f.Replied); err == nil {
				_ = c.db.QueryRowx(`
					SELECT
						COALESCE(COUNT(*), 0) AS total,
						COALESCE(COUNT(DISTINCT subscriber_id) FILTER (WHERE is_bot = FALSE), 0) AS human_unique
					FROM campaign_views WHERE sequence_step_id = $1`, stepID).Scan(
					&f.Analytics.Views.Total,
					&f.Analytics.Views.HumanUnique,
				)

				_ = c.db.QueryRowx(`
					SELECT
						COALESCE(COUNT(*), 0) AS total,
						COALESCE(COUNT(DISTINCT subscriber_id) FILTER (WHERE is_bot = FALSE), 0) AS human_unique
					FROM link_clicks WHERE sequence_step_id = $1`, stepID).Scan(
					&f.Analytics.Clicks.Total,
					&f.Analytics.Clicks.HumanUnique,
				)

				out.Funnel = append(out.Funnel, f)
			}
		}
	}

	return out, nil
}
