package core

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/listmonk/internal/utils"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
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
		o.BodySource)
	if err != nil {
		c.log.Printf("error updating campaign: %v", err)
		return models.Campaign{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.campaign}", "error", pqErrMsg(err)))
	}

	out, err := c.GetCampaign(id, "", "")
	if err != nil {
		return models.Campaign{}, err
	}

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
