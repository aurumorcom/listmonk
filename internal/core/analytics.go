package core

import (
	"database/sql"
	"net"
	"strings"
	"time"

	"github.com/knadh/listmonk/models"
	"github.com/mileusna/useragent"
)

// ClientMeta captures request client metadata, network details, and bot classification.
type ClientMeta struct {
	IPAddress      string            `json:"ip_address"`
	GeoCountry     string            `json:"geo_country"`
	GeoRegion      string            `json:"geo_region"`
	GeoCity        string            `json:"geo_city"`
	GeoASN         string            `json:"geo_asn"`
	UserAgent      string            `json:"user_agent"`
	DeviceType     string            `json:"device_type"`    // "desktop", "mobile", "tablet", "bot", "unknown"
	ClientOS       string            `json:"client_os"`      // "iOS", "Windows", "Android", "macOS", "Linux"
	ClientBrowser  string            `json:"client_browser"` // "Chrome", "Safari", "Firefox", "Edge"
	EmailClient    string            `json:"email_client"`   // "Gmail Web", "Apple Mail", "Outlook", "Webmail"
	IsProxy        bool              `json:"is_proxy"`
	IsBot          bool              `json:"is_bot"`
	BotType        string            `json:"bot_type"`
	IsHuman        bool              `json:"is_human"`
	SequenceStepID *int              `json:"sequence_step_id,omitempty"`
	VariantID      string            `json:"variant_id,omitempty"`
	LinkPosition   string            `json:"link_position,omitempty"`
	UTMParams      map[string]string `json:"utm_params,omitempty"`
}

// ParseClientMeta parses raw request parameters, User-Agent, and headers into a ClientMeta container.
func ParseClientMeta(ip, ua string, headers map[string]string, deltaSinceSend time.Duration, isHoneypot bool, stepID *int, variantID, linkPos string, utms map[string]string) ClientMeta {
	// 1. Clean IP address
	cleanIP := sanitizeIP(ip)

	// 2. Classify Bot
	botInfo := ClassifyBot(ua, headers, deltaSinceSend, isHoneypot)

	// 3. Parse User-Agent using high-performance parser
	parsedUA := useragent.Parse(ua)

	deviceType := "desktop"
	if botInfo.IsBot {
		deviceType = "bot"
	} else if parsedUA.Mobile {
		deviceType = "mobile"
	} else if parsedUA.Tablet {
		deviceType = "tablet"
	} else if parsedUA.Desktop {
		deviceType = "desktop"
	} else if strings.TrimSpace(ua) == "" {
		deviceType = "unknown"
	}

	clientOS := parsedUA.OS
	if clientOS == "" {
		clientOS = detectOSFallback(ua)
	}

	clientBrowser := parsedUA.Name
	if clientBrowser == "" {
		clientBrowser = detectBrowserFallback(ua)
	}

	emailClient := detectEmailClient(ua, headers, botInfo)

	// 4. Resolve Geo Country (fallback based on cloud headers or IP)
	geoCountry, geoRegion, geoCity, geoASN := resolveGeoHeaders(headers, cleanIP)

	return ClientMeta{
		IPAddress:      cleanIP,
		GeoCountry:     geoCountry,
		GeoRegion:      geoRegion,
		GeoCity:        geoCity,
		GeoASN:         geoASN,
		UserAgent:      ua,
		DeviceType:     deviceType,
		ClientOS:       clientOS,
		ClientBrowser:  clientBrowser,
		EmailClient:    emailClient,
		IsProxy:        botInfo.IsProxy,
		IsBot:          botInfo.IsBot,
		BotType:        botInfo.BotType,
		IsHuman:        botInfo.IsHuman,
		SequenceStepID: stepID,
		VariantID:      variantID,
		LinkPosition:   linkPos,
		UTMParams:      utms,
	}
}

func sanitizeIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	parsed := net.ParseIP(ip)
	if parsed != nil {
		return parsed.String()
	}
	return ""
}

func detectOSFallback(ua string) string {
	uaLower := strings.ToLower(ua)
	switch {
	case strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad") || strings.Contains(uaLower, "ios"):
		return "iOS"
	case strings.Contains(uaLower, "android"):
		return "Android"
	case strings.Contains(uaLower, "windows"):
		return "Windows"
	case strings.Contains(uaLower, "macintosh") || strings.Contains(uaLower, "mac os"):
		return "macOS"
	case strings.Contains(uaLower, "linux"):
		return "Linux"
	default:
		return "Unknown"
	}
}

func detectBrowserFallback(ua string) string {
	uaLower := strings.ToLower(ua)
	switch {
	case strings.Contains(uaLower, "edg/"):
		return "Edge"
	case strings.Contains(uaLower, "chrome/"):
		return "Chrome"
	case strings.Contains(uaLower, "safari/") && !strings.Contains(uaLower, "chrome"):
		return "Safari"
	case strings.Contains(uaLower, "firefox/"):
		return "Firefox"
	default:
		return "Other"
	}
}

func detectEmailClient(ua string, headers map[string]string, bot BotClassification) string {
	if bot.IsProxy && bot.BotType == BotTypeGoogleProxy {
		return "Gmail Image Proxy"
	}
	if bot.IsProxy && bot.BotType == BotTypeProxyMPP {
		return "Apple Mail Privacy"
	}

	uaLower := strings.ToLower(ua)
	switch {
	case strings.Contains(uaLower, "outlook"):
		return "Outlook"
	case strings.Contains(uaLower, "thunderbird"):
		return "Thunderbird"
	case strings.Contains(uaLower, "apple-mail") || (strings.Contains(uaLower, "macintosh") && strings.Contains(uaLower, "mail/")):
		return "Apple Mail"
	case strings.Contains(uaLower, "roundcube"):
		return "Roundcube"
	default:
		if headers["referer"] != "" && strings.Contains(strings.ToLower(headers["referer"]), "mail.google.com") {
			return "Gmail Webmail"
		}
		if headers["referer"] != "" && strings.Contains(strings.ToLower(headers["referer"]), "outlook.live.com") {
			return "Outlook Webmail"
		}
		return "Web / Native Client"
	}
}

func resolveGeoHeaders(headers map[string]string, ip string) (country, region, city, asn string) {
	if c := headers["cf-ipcountry"]; c != "" {
		country = strings.ToUpper(c)
	} else if c := headers["x-country-code"]; c != "" {
		country = strings.ToUpper(c)
	} else if c := headers["geoip_country_code"]; c != "" {
		country = strings.ToUpper(c)
	}

	if r := headers["cf-region"]; r != "" {
		region = r
	} else if r := headers["x-region-name"]; r != "" {
		region = r
	}

	if ci := headers["cf-ipcity"]; ci != "" {
		city = ci
	} else if ci := headers["x-city"]; ci != "" {
		city = ci
	}

	if a := headers["cf-ipasn"]; a != "" {
		asn = a
	}

	return country, region, city, asn
}

// GetCampaignAnalytics calculates deep, multidimensional analytics for a campaign.
func (c *Core) GetCampaignAnalytics(campID int, fromDate, toDate string) (*models.CampaignAnalytics, error) {
	out := &models.CampaignAnalytics{
		CampaignID: campID,
		Breakdowns: models.CampaignBreakdownStats{
			Devices:   []models.DeviceBreakdown{},
			Locations: []models.LocationBreakdown{},
			Links:     []models.CampaignAnalyticsLink{},
			Variants:  []models.VariantPerformance{},
			Bots: models.CampaignBotStats{
				BotTypeBreakdown: make(map[string]int),
			},
		},
	}

	// 1. Fetch Campaign Details (Name, UUID, Sent, ToSend, Bounces)
	row := c.db.QueryRowx(`
		SELECT
			c.uuid,
			c.name,
			COALESCE(c.sent, 0) AS sent,
			COALESCE(c.to_send, 0) AS to_send,
			(SELECT COUNT(*) FROM bounces WHERE campaign_id = c.id) AS bounces
		FROM campaigns c
		WHERE c.id = $1`, campID)
	if err := row.Scan(&out.CampaignUUID, &out.CampaignName, &out.Sent, &out.ToSend, &out.Bounces); err != nil && err != sql.ErrNoRows {
		c.log.Printf("error fetching campaign info for analytics: %v", err)
	}

	// 2. Fetch View Stats
	viewRow := c.db.QueryRowx(`
		SELECT
			COALESCE(COUNT(*), 0) AS total,
			COALESCE(COUNT(DISTINCT subscriber_id), 0) AS unique_views,
			COALESCE(COUNT(*) FILTER (WHERE is_bot = FALSE), 0) AS human_total,
			COALESCE(COUNT(DISTINCT subscriber_id) FILTER (WHERE is_bot = FALSE), 0) AS human_unique,
			COALESCE(COUNT(*) FILTER (WHERE is_bot = TRUE), 0) AS bot_total,
			COALESCE(COUNT(*) FILTER (WHERE is_proxy = TRUE), 0) AS proxy_mpp_total
		FROM campaign_views
		WHERE campaign_id = $1`, campID)
	_ = viewRow.Scan(
		&out.Views.Total,
		&out.Views.Unique,
		&out.Views.HumanTotal,
		&out.Views.HumanUnique,
		&out.Views.BotTotal,
		&out.Views.ProxyMPPTotal,
	)

	// 3. Fetch Click Stats
	clickRow := c.db.QueryRowx(`
		SELECT
			COALESCE(COUNT(*), 0) AS total,
			COALESCE(COUNT(DISTINCT subscriber_id), 0) AS unique_clicks,
			COALESCE(COUNT(*) FILTER (WHERE is_bot = FALSE), 0) AS human_total,
			COALESCE(COUNT(DISTINCT subscriber_id) FILTER (WHERE is_bot = FALSE), 0) AS human_unique,
			COALESCE(COUNT(*) FILTER (WHERE is_bot = TRUE), 0) AS bot_clicks
		FROM link_clicks
		WHERE campaign_id = $1`, campID)
	_ = clickRow.Scan(
		&out.Clicks.Total,
		&out.Clicks.Unique,
		&out.Clicks.HumanTotal,
		&out.Clicks.HumanUnique,
		&out.Clicks.BotClicks,
	)

	if out.Views.HumanUnique > 0 {
		out.Clicks.CTOR = (float64(out.Clicks.HumanUnique) / float64(out.Views.HumanUnique)) * 100.0
	}

	// 4. Breakdown: Devices
	devRows, err := c.db.Queryx(`
		SELECT
			COALESCE(device_type, 'unknown') AS device_type,
			COALESCE(client_os, 'unknown') AS os,
			COALESCE(client_browser, 'unknown') AS browser,
			COALESCE(COUNT(*), 0) AS clicks,
			COALESCE(COUNT(DISTINCT subscriber_id), 0) AS unique_clicks
		FROM link_clicks
		WHERE campaign_id = $1
		GROUP BY device_type, client_os, client_browser
		ORDER BY clicks DESC`, campID)
	if err == nil {
		defer devRows.Close()
		for devRows.Next() {
			var d models.DeviceBreakdown
			if err := devRows.Scan(&d.DeviceType, &d.OS, &d.Browser, &d.Clicks, &d.UniqueClicks); err == nil {
				out.Breakdowns.Devices = append(out.Breakdowns.Devices, d)
			}
		}
	}

	// 5. Breakdown: Locations
	locRows, err := c.db.Queryx(`
		SELECT
			COALESCE(geo_country, '') AS country,
			COALESCE(geo_region, '') AS region,
			COALESCE(geo_city, '') AS city,
			COALESCE(geo_asn, '') AS asn,
			COALESCE(COUNT(*), 0) AS clicks,
			COALESCE(COUNT(DISTINCT subscriber_id), 0) AS unique_clicks
		FROM link_clicks
		WHERE campaign_id = $1 AND geo_country IS NOT NULL AND geo_country != ''
		GROUP BY geo_country, geo_region, geo_city, geo_asn
		ORDER BY clicks DESC`, campID)
	if err == nil {
		defer locRows.Close()
		for locRows.Next() {
			var l models.LocationBreakdown
			if err := locRows.Scan(&l.Country, &l.Region, &l.City, &l.ASN, &l.Clicks, &l.UniqueClicks); err == nil {
				out.Breakdowns.Locations = append(out.Breakdowns.Locations, l)
			}
		}
	}

	// 6. Breakdown: Links
	linkRows, err := c.db.Queryx(`
		SELECT
			l.url,
			COALESCE(COUNT(lc.id), 0) AS count
		FROM links l
		JOIN link_clicks lc ON lc.link_id = l.id
		WHERE lc.campaign_id = $1
		GROUP BY l.url
		ORDER BY count DESC`, campID)
	if err == nil {
		defer linkRows.Close()
		for linkRows.Next() {
			var l models.CampaignAnalyticsLink
			if err := linkRows.Scan(&l.URL, &l.Count); err == nil {
				out.Breakdowns.Links = append(out.Breakdowns.Links, l)
			}
		}
	}

	// 7. Breakdown: Variants
	varRows, err := c.db.Queryx(`
		SELECT
			variant_id,
			COALESCE(COUNT(DISTINCT subscriber_id), 0) AS unique_clicks
		FROM link_clicks
		WHERE campaign_id = $1 AND variant_id IS NOT NULL AND variant_id != ''
		GROUP BY variant_id`, campID)
	if err == nil {
		defer varRows.Close()
		for varRows.Next() {
			var v models.VariantPerformance
			if err := varRows.Scan(&v.VariantID, &v.UniqueClicks); err == nil {
				out.Breakdowns.Variants = append(out.Breakdowns.Variants, v)
			}
		}
	}

	// 8. Breakdown: Bots
	botRows, err := c.db.Queryx(`
		SELECT
			COALESCE(bot_type, 'unknown') AS b_type,
			COUNT(*) AS count
		FROM link_clicks
		WHERE campaign_id = $1 AND is_bot = TRUE
		GROUP BY bot_type`, campID)
	if err == nil {
		defer botRows.Close()
		for botRows.Next() {
			var bType string
			var cnt int
			if err := botRows.Scan(&bType, &cnt); err == nil {
				out.Breakdowns.Bots.BotTypeBreakdown[bType] = cnt
				out.Breakdowns.Bots.TotalBotEvents += cnt
				if bType == BotTypeSecurityScanner {
					out.Breakdowns.Bots.ScannersDetected += cnt
				} else if bType == BotTypeHoneypot {
					out.Breakdowns.Bots.HoneypotTriggers += cnt
				}
			}
		}
	}

	return out, nil
}
