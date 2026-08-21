package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"strings"
	txttpl "text/template"

	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/types"
	"github.com/lib/pq"
	null "gopkg.in/volatiletech/null.v6"
)

const (
	CampaignStatusDraft         = "draft"
	CampaignStatusScheduled     = "scheduled"
	CampaignStatusRunning       = "running"
	CampaignStatusPaused        = "paused"
	CampaignStatusFinished      = "finished"
	CampaignStatusCancelled     = "cancelled"
	CampaignTypeRegular         = "regular"
	CampaignTypeOptin           = "optin"
	CampaignTypeSequence        = "sequence"
	CampaignContentTypeRichtext = "richtext"
	CampaignContentTypeHTML     = "html"
	CampaignContentTypeMarkdown = "markdown"
	CampaignContentTypePlain    = "plain"
	CampaignContentTypeVisual   = "visual"

	CampaignSubscriberStatusScheduled  = "scheduled"
	CampaignSubscriberStatusInProgress = "in_progress"
	CampaignSubscriberStatusReplied    = "replied"
	CampaignSubscriberStatusFinished   = "finished"
	CampaignSubscriberStatusOptedOut   = "opted_out"

	CampaignConditionAlways    = "always"
	CampaignConditionIfRead    = "if_read"
	CampaignConditionIfNotRead = "if_not_read"
	CampaignConditionIfClicked = "if_clicked"

	EmailTypeNewThread = "New Thread"
	EmailTypeReply     = "Reply"
)

// Campaigns represents a slice of Campaigns.
type Campaigns []Campaign

// Campaign represents an e-mail campaign.
type Campaign struct {
	Base
	CampaignMeta

	UUID              string          `db:"uuid" json:"uuid"`
	Type              string          `db:"type" json:"type"`
	Name              string          `db:"name" json:"name"`
	Subject           string          `db:"subject" json:"subject"`
	FromEmail         string          `db:"from_email" json:"from_email"`
	Body              string          `db:"body" json:"body"`
	BodySource        null.String     `db:"body_source" json:"body_source"`
	AltBody           null.String     `db:"altbody" json:"altbody"`
	SendAt            null.Time       `db:"send_at" json:"send_at"`
	Status            string          `db:"status" json:"status"`
	ContentType       string          `db:"content_type" json:"content_type"`
	Tags              pq.StringArray  `db:"tags" json:"tags"`
	Headers           Headers         `db:"headers" json:"headers"`
	Attribs           JSON            `db:"attribs" json:"attribs"`
	TemplateID        null.Int        `db:"template_id" json:"template_id"`
	Messenger         string          `db:"messenger" json:"messenger"`
	Archive           bool            `db:"archive" json:"archive"`
	ArchiveSlug       null.String     `db:"archive_slug" json:"archive_slug"`
	ArchiveTemplateID null.Int        `db:"archive_template_id" json:"archive_template_id"`
	ArchiveMeta       json.RawMessage `db:"archive_meta" json:"archive_meta"`

	ScheduleID   null.Int       `db:"schedule_id" json:"schedule_id"`
	Schedule     *Schedule      `db:"-" json:"schedule,omitempty"`
	SendWindow   JSON           `db:"send_window" json:"send_window"`
	EmailIDs     pq.Int64Array  `db:"email_ids" json:"email_ids"`
	WahaSessions pq.StringArray `db:"waha_sessions" json:"waha_sessions"`
	Steps        []CampaignStep `db:"-" json:"steps,omitempty"`

	// TemplateBody is joined in from templates by the next-campaigns query.
	TemplateBody        string             `db:"template_body" json:"-"`
	ArchiveTemplateBody string             `db:"archive_template_body" json:"-"`
	TemplateType        string             `db:"template_type" json:"-"`
	Tpl                 *template.Template `json:"-"`
	SubjectTpl          *txttpl.Template   `json:"-"`
	SystemPromptTpl     *txttpl.Template   `json:"-"`
	AltBodyTpl          *template.Template `json:"-"`

	// HeaderTpls is holds optionally {{ templated }} campaign headers.
	HeaderTpls []map[string]*txttpl.Template `json:"-"`

	// List of media (attachment) IDs obtained from the next-campaign query
	// while sending a campaign.
	MediaIDs pq.Int64Array `json:"-" db:"media_id"`

	// Fetched bodies of the attachments.
	Attachments []Attachment `json:"-" db:"-"`

	// Pseudofield for getting the total number of subscribers
	// in searches and queries.
	Total int `db:"total" json:"-"`
}

// CampaignMeta contains fields tracking a campaign's progress.
type CampaignMeta struct {
	CampaignID int `db:"campaign_id" json:"-"`
	Views      int `db:"views" json:"views"`
	Clicks     int `db:"clicks" json:"clicks"`
	Bounces    int `db:"bounces" json:"bounces"`

	// This is a list of {list_id, name} pairs unlike Subscriber.Lists[]
	// because lists can be deleted after a campaign is finished, resulting
	// in null lists data to be returned. For that reason, campaign_lists maintains
	// campaign-list associations with a historical record of id + name that persist
	// even after a list is deleted.
	Lists types.JSONText `db:"lists" json:"lists"`
	Media types.JSONText `db:"media" json:"media"`

	StartedAt null.Time `db:"started_at" json:"started_at"`
	ToSend    int       `db:"to_send" json:"to_send"`
	Sent      int       `db:"sent" json:"sent"`
}

// CampaignViewStats aggregates detailed view/open telemetry separating human from bot traffic.
type CampaignViewStats struct {
	Total         int `json:"total"`
	Unique        int `json:"unique"`
	HumanTotal    int `json:"human_total"`
	HumanUnique   int `json:"human_unique"`
	BotTotal      int `json:"bot_total"`
	ProxyMPPTotal int `json:"proxy_mpp_total"`
}

// CampaignClickStats aggregates detailed click telemetry separating human from bot traffic.
type CampaignClickStats struct {
	Total       int     `json:"total"`
	Unique      int     `json:"unique"`
	HumanTotal  int     `json:"human_total"`
	HumanUnique int     `json:"human_unique"`
	BotClicks   int     `json:"bot_clicks"`
	CTOR        float64 `json:"ctor"` // Click-to-Open Rate: (HumanUniqueClicks / HumanUniqueViews) * 100
}

// CampaignBotStats aggregates bot-detected telemetry.
type CampaignBotStats struct {
	TotalBotEvents   int            `json:"total_bot_events"`
	ScannersDetected int            `json:"scanners_detected"`
	HoneypotTriggers int            `json:"honeypot_triggers"`
	BotTypeBreakdown map[string]int `json:"bot_type_breakdown"`
}

// DeviceBreakdown provides dimensional slicing by device type, OS, and browser.
type DeviceBreakdown struct {
	DeviceType   string `json:"device_type"`
	OS           string `json:"os"`
	Browser      string `json:"browser"`
	Clicks       int    `json:"clicks"`
	UniqueClicks int    `json:"unique_clicks"`
	Views        int    `json:"views"`
	UniqueViews  int    `json:"unique_views"`
}

// LocationBreakdown provides dimensional slicing by country, region, city, and ASN network.
type LocationBreakdown struct {
	Country      string `json:"country"`
	Region       string `json:"region"`
	City         string `json:"city"`
	ASN          string `json:"asn"`
	Clicks       int    `json:"clicks"`
	UniqueClicks int    `json:"unique_clicks"`
	Views        int    `json:"views"`
	UniqueViews  int    `json:"unique_views"`
}

// VariantPerformance tracks A/B test variant metrics.
type VariantPerformance struct {
	VariantID    string  `json:"variant_id"`
	Sent         int     `json:"sent"`
	UniqueOpens  int     `json:"unique_opens"`
	UniqueClicks int     `json:"unique_clicks"`
	CTOR         float64 `json:"ctor"`
}

// CampaignBreakdownStats aggregates multidimensional slices for a campaign.
type CampaignBreakdownStats struct {
	Devices   []DeviceBreakdown       `json:"devices,omitempty"`
	Locations []LocationBreakdown     `json:"locations,omitempty"`
	Links     []CampaignAnalyticsLink `json:"links,omitempty"`
	Variants  []VariantPerformance    `json:"variants,omitempty"`
	Bots      CampaignBotStats        `json:"bots"`
}

// CampaignAnalytics is the canonical analytics model across campaigns and sequences.
type CampaignAnalytics struct {
	CampaignID   int                    `json:"campaign_id,omitempty"`
	CampaignUUID string                 `json:"campaign_uuid,omitempty"`
	CampaignName string                 `json:"campaign_name,omitempty"`
	Sent         int                    `json:"sent"`
	ToSend       int                    `json:"to_send"`
	Bounces      int                    `json:"bounces"`
	Views        CampaignViewStats      `json:"views"`
	Clicks       CampaignClickStats     `json:"clicks"`
	Breakdowns   CampaignBreakdownStats `json:"breakdowns"`
}

// GetIDs returns the list of campaign IDs.
func (camps Campaigns) GetIDs() []int {
	IDs := make([]int, len(camps))
	for i, c := range camps {
		IDs[i] = c.ID
	}

	return IDs
}

// LoadStats lazy loads campaign stats onto a list of campaigns.
func (camps Campaigns) LoadStats(stmt *sqlx.Stmt) error {
	var meta []CampaignMeta
	if err := stmt.Select(&meta, pq.Array(camps.GetIDs())); err != nil {
		return err
	}

	if len(camps) != len(meta) {
		return errors.New("campaign stats count does not match")
	}

	for i, c := range meta {
		if c.CampaignID == camps[i].ID {
			camps[i].Lists = c.Lists
			camps[i].Views = c.Views
			camps[i].Clicks = c.Clicks
			camps[i].Bounces = c.Bounces
			camps[i].Media = c.Media
		}
	}

	return nil
}

// CompileTemplate compiles a campaign body template into its base
// template and sets the resultant template to Campaign.Tpl.
func compileCampaignSubject(c *Campaign, f template.FuncMap) error {
	if !hasTplExpr(c.Subject) {
		return nil
	}
	subj := c.Subject
	for _, r := range regTplFuncs {
		subj = r.regExp.ReplaceAllString(subj, r.replace)
	}

	var txtFuncs map[string]any = f
	subjTpl, err := txttpl.New(ContentTpl).Funcs(txtFuncs).Parse(subj)
	if err != nil {
		return fmt.Errorf("error compiling subject: %v", err)
	}
	c.SubjectTpl = subjTpl
	return nil
}

func compileCampaignSystemPrompt(c *Campaign, f template.FuncMap) error {
	if c.TemplateType != TemplateTypePrompt || c.TemplateBody == "" || !hasTplExpr(c.TemplateBody) {
		return nil
	}
	sysPrompt := c.TemplateBody
	for _, r := range regTplFuncs {
		sysPrompt = r.regExp.ReplaceAllString(sysPrompt, r.replace)
	}

	var txtFuncs map[string]any = f
	sysTpl, err := txttpl.New(ContentTpl).Funcs(txtFuncs).Parse(sysPrompt)
	if err != nil {
		return fmt.Errorf("error compiling system prompt: %v", err)
	}
	c.SystemPromptTpl = sysTpl
	return nil
}

func compileCampaignBaseAndBody(c *Campaign, f template.FuncMap) error {
	body := c.TemplateBody
	if body == "" || c.ContentType == CampaignContentTypeVisual {
		body = `{{ template "content" . }}`
	}

	for _, r := range regTplFuncs {
		body = r.regExp.ReplaceAllString(body, r.replace)
	}

	baseTPL, err := template.New(BaseTpl).Funcs(f).Parse(body)
	if err != nil {
		return fmt.Errorf("error compiling base template: %v", err)
	}

	if c.ContentType == CampaignContentTypeMarkdown {
		var b bytes.Buffer
		if err := markdown.Convert([]byte(c.Body), &b); err != nil {
			return err
		}
		body = b.String()
	} else {
		body = c.Body
	}

	for _, r := range regTplFuncs {
		body = r.regExp.ReplaceAllString(body, r.replace)
	}

	msgTpl, err := template.New(ContentTpl).Funcs(f).Parse(body)
	if err != nil {
		return fmt.Errorf("error compiling message: %v", err)
	}

	out, err := baseTPL.AddParseTree(ContentTpl, msgTpl.Tree)
	if err != nil {
		return fmt.Errorf("error inserting child template: %v", err)
	}
	c.Tpl = out
	return nil
}

func compileCampaignAltBody(c *Campaign, f template.FuncMap) error {
	if !hasTplExpr(c.AltBody.String) {
		return nil
	}
	b := c.AltBody.String
	for _, r := range regTplFuncs {
		b = r.regExp.ReplaceAllString(b, r.replace)
	}
	bTpl, err := template.New(ContentTpl).Funcs(f).Parse(b)
	if err != nil {
		return fmt.Errorf("error compiling alt plaintext message: %v", err)
	}
	c.AltBodyTpl = bTpl
	return nil
}

func compileCampaignHeaderTemplates(c *Campaign, f template.FuncMap) error {
	hasHdrExpr := false
	for _, set := range c.Headers {
		for _, val := range set {
			if hasTplExpr(val) {
				hasHdrExpr = true
				break
			}
		}
		if hasHdrExpr {
			break
		}
	}
	if !hasHdrExpr {
		return nil
	}

	c.HeaderTpls = make([]map[string]*txttpl.Template, len(c.Headers))
	var txtFuncs map[string]any = f
	for i, set := range c.Headers {
		c.HeaderTpls[i] = make(map[string]*txttpl.Template, len(set))
		for hdr, val := range set {
			if !hasTplExpr(val) {
				continue
			}
			tpl, err := txttpl.New(ContentTpl).Funcs(txtFuncs).Parse(val)
			if err != nil {
				return fmt.Errorf("error compiling header %q: %v", hdr, err)
			}
			c.HeaderTpls[i][hdr] = tpl
		}
	}
	return nil
}

// CompileTemplate compiles a campaign body template into its base template.
func (c *Campaign) CompileTemplate(f template.FuncMap) error {
	if err := compileCampaignSubject(c, f); err != nil {
		return err
	}
	if err := compileCampaignSystemPrompt(c, f); err != nil {
		return err
	}
	if err := compileCampaignBaseAndBody(c, f); err != nil {
		return err
	}
	if err := compileCampaignAltBody(c, f); err != nil {
		return err
	}
	if err := compileCampaignHeaderTemplates(c, f); err != nil {
		return err
	}
	return nil
}

// hasTplExpr checks whether a given string has a Go template expression with {{ and  }}.
func hasTplExpr(s string) bool {
	_, after, ok := strings.Cut(s, "{{")
	return ok && strings.Contains(after, "}}")
}

// ConvertContent converts a campaign's body from one format to another,
// for example, Markdown to HTML.
func (c *Campaign) ConvertContent(from, to string) (string, error) {
	body := c.Body
	for _, r := range regTplFuncs {
		body = r.regExp.ReplaceAllString(body, r.replace)
	}

	// If the format is markdown, convert Markdown to HTML.
	var out string
	if from == CampaignContentTypeMarkdown &&
		(to == CampaignContentTypeHTML || to == CampaignContentTypeRichtext) {
		var b bytes.Buffer
		if err := markdown.Convert([]byte(c.Body), &b); err != nil {
			return out, err
		}
		out = b.String()
	} else {
		return out, errors.New("unknown formats to convert")
	}

	return out, nil
}

// CampaignSteps represents a slice of CampaignStep.
type CampaignSteps []CampaignStep

// CampaignStep represents an individual step in a sequence campaign.
type CampaignStep struct {
	ID         int           `db:"id" json:"id"`
	CampaignID int           `db:"campaign_id" json:"campaign_id"`
	StepNumber int           `db:"step_number" json:"step_number"`
	Delay      string        `db:"delay" json:"delay"`
	Messenger  string        `db:"messenger" json:"messenger"`
	Condition  string        `db:"condition" json:"condition"`
	Subject    string        `db:"subject" json:"subject"`
	Body       string        `db:"body" json:"body"`
	EmailType  string        `db:"email_type" json:"email_type"`
	TemplateID null.Int      `db:"template_id" json:"template_id"`
	MediaIDs   pq.Int64Array `db:"media_ids" json:"media_ids"`
}

// CampaignSubscribers represents a slice of CampaignSubscriber.
type CampaignSubscribers []CampaignSubscriber

// CampaignSubscriber tracks the state machine position of a subscriber within a sequence campaign.
type CampaignSubscriber struct {
	CampaignID      int         `db:"campaign_id" json:"campaign_id"`
	SubscriberID    int         `db:"subscriber_id" json:"subscriber_id"`
	EmailID         null.Int    `db:"email_id" json:"email_id"`
	FromAddress     null.String `db:"from_address" json:"from_address"`
	WahaSession     null.String `db:"waha_session" json:"waha_session"`
	Status          string      `db:"status" json:"status"`
	CurrentStep     int         `db:"current_step" json:"current_step"`
	NextSendAt      null.Time   `db:"next_send_at" json:"next_send_at"`
	LastReadAt      null.Time   `db:"last_read_at" json:"last_read_at"`
	LastClickedAt   null.Time   `db:"last_clicked_at" json:"last_clicked_at"`
	LastMessageID   null.String `db:"last_message_id" json:"last_message_id"`
	LastThreadMsgID null.String `db:"last_thread_msg_id" json:"last_thread_msg_id"`
	CreatedAt       null.Time   `db:"created_at" json:"created_at"`
}

// CampaignStepFunnel represents metrics for an individual sequence step in the conversion funnel.
type CampaignStepFunnel struct {
	StepNumber int               `json:"step_number"`
	Subject    string            `json:"subject"`
	Messenger  string            `json:"messenger"`
	Reached    int               `json:"reached"`
	Replied    int               `json:"replied"`
	Analytics  CampaignAnalytics `json:"analytics"`
}

// CampaignSequenceAnalytics aggregates metrics across sequence campaigns.
type CampaignSequenceAnalytics struct {
	ActiveSubscribers   int                  `json:"active_subscribers"`
	StepCompletions     int                  `json:"step_completions"`
	ReplyRate           float64              `json:"reply_rate"`
	ConversionRate      float64              `json:"conversion_rate"`
	AggregatedAnalytics CampaignAnalytics    `json:"aggregated_analytics"`
	Funnel              []CampaignStepFunnel `json:"funnel"`
}
