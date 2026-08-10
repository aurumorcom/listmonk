package models

import (
	"fmt"
	"html/template"
	txttpl "text/template"
	"time"

	null "gopkg.in/volatiletech/null.v6"
)

const (
	BaseTpl                    = "base"
	ContentTpl                 = "content"
	TemplateTypeCampaign       = "campaign"
	TemplateTypeCampaignVisual = "campaign_visual"
	TemplateTypeTx             = "tx"
	TemplateTypePrompt         = "prompt"
)

// Template represents a reusable e-mail or prompt template.
type Template struct {
	Base

	Name string `db:"name" json:"name"`
	// Subject is for type=tx or type=prompt.
	Subject      string      `db:"subject" json:"subject"`
	SystemPrompt string      `db:"system_prompt" json:"system_prompt,omitempty"`
	Type         string      `db:"type" json:"type"`
	Body         string      `db:"body" json:"body,omitempty"`
	BodySource   null.String `db:"body_source" json:"body_source,omitempty"`
	IsDefault    bool        `db:"is_default" json:"is_default"`

	// Only relevant to tx (transactional) and prompt templates.
	SubjectTpl      *txttpl.Template   `json:"-"`
	SystemPromptTpl *txttpl.Template   `json:"-"`
	Tpl             *template.Template `json:"-"`
	Attachments     []Attachment       `json:"-"`
}

// Compile compiles a template body and subject (only for tx templates) and
// caches the template references to be executed later.
func (t *Template) Compile(f template.FuncMap) error {
	tpl, err := template.New(BaseTpl).Funcs(f).Parse(t.Body)
	if err != nil {
		return fmt.Errorf("error compiling template: %v", err)
	}
	t.Tpl = tpl

	// If the subject line has a template string, compile it.
	if hasTplExpr(t.Subject) {
		subj := t.Subject

		subjTpl, err := txttpl.New(BaseTpl).Funcs(txttpl.FuncMap(f)).Parse(subj)
		if err != nil {
			return fmt.Errorf("error compiling subject: %v", err)
		}
		t.SubjectTpl = subjTpl
	}

	// If the system prompt has a template string, compile it.
	if t.SystemPrompt != "" && hasTplExpr(t.SystemPrompt) {
		sysTpl, err := txttpl.New(BaseTpl).Funcs(txttpl.FuncMap(f)).Parse(t.SystemPrompt)
		if err != nil {
			return fmt.Errorf("error compiling system prompt: %v", err)
		}
		t.SystemPromptTpl = sysTpl
	}

	return nil
}

type CampaignStats struct {
	ID        int       `db:"id" json:"id"`
	Status    string    `db:"status" json:"status"`
	ToSend    int       `db:"to_send" json:"to_send"`
	Sent      int       `db:"sent" json:"sent"`
	Started   null.Time `db:"started_at" json:"started_at"`
	UpdatedAt null.Time `db:"updated_at" json:"updated_at"`
	Rate      int       `json:"rate"`
	NetRate   int       `json:"net_rate"`
}

type CampaignAnalyticsCount struct {
	CampaignID int       `db:"campaign_id" json:"campaign_id"`
	Count      int       `db:"count" json:"count"`
	Timestamp  time.Time `db:"timestamp" json:"timestamp"`
}

type CampaignAnalyticsLink struct {
	URL   string `db:"url" json:"url"`
	Count int    `db:"count" json:"count"`
}

type CampaignViewExport struct {
	CampaignID     int       `db:"campaign_id"`
	CampaignUUID   string    `db:"campaign_uuid"`
	CampaignName   string    `db:"campaign_name"`
	SubscriberID   int       `db:"subscriber_id"`
	SubscriberUUID string    `db:"subscriber_uuid"`
	Email          string    `db:"email"`
	SubscriberName string    `db:"subscriber_name"`
	CreatedAt      time.Time `db:"created_at"`
}

type CampaignClickExport struct {
	CampaignID     int       `db:"campaign_id"`
	CampaignUUID   string    `db:"campaign_uuid"`
	CampaignName   string    `db:"campaign_name"`
	SubscriberID   int       `db:"subscriber_id"`
	SubscriberUUID string    `db:"subscriber_uuid"`
	Email          string    `db:"email"`
	SubscriberName string    `db:"subscriber_name"`
	URL            string    `db:"url"`
	CreatedAt      time.Time `db:"created_at"`
}
