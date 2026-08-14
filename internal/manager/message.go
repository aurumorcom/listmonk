package manager

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/knadh/listmonk/models"
)

// NewCampaignMessage creates and returns a CampaignMessage that is made available
// to message templates while they're compiled. It represents a message from
// a campaign that's bound to a single Subscriber.
func (m *Manager) NewCampaignMessage(c *models.Campaign, s models.Subscriber) (CampaignMessage, error) {
	msg := CampaignMessage{
		Campaign:   c,
		Subscriber: s,

		subject:  c.Subject,
		from:     c.FromEmail,
		to:       s.Email,
		unsubURL: fmt.Sprintf(m.cfg.UnsubURL, c.UUID, s.UUID),
	}

	if err := msg.render(); err != nil {
		return msg, err
	}

	return msg, nil
}

// render takes a Message, executes its pre-compiled Campaign.Tpl
// and applies the resultant bytes to Message.body to be used in messages.
func (m *CampaignMessage) render() error {
	out := bytes.Buffer{}

	// If this is a Prompt template type and Bifrost client is configured, run JIT AI generation.
	if m.pipe != nil && m.pipe.m != nil && m.pipe.m.bifrostClient != nil && m.Campaign != nil {
		scope := ExtractTemplateScope(m.Subscriber)

		sysPromptStr := m.Campaign.TemplateBody
		if m.Campaign.SystemPromptTpl != nil {
			var sb bytes.Buffer
			if err := m.Campaign.SystemPromptTpl.Execute(&sb, scope); err == nil {
				sysPromptStr = sb.String()
			}
		}

		var userBuf bytes.Buffer
		if m.Campaign.Tpl != nil {
			if err := m.Campaign.Tpl.Execute(&userBuf, scope); err == nil {
				userPromptStr := userBuf.String()

				// Run JIT generation via Bifrost SDK with EmailResponseFormat guide
				ctx, cancel := m.pipe.m.bifrostClient.TimeoutContext()
				aiBody, err := m.pipe.m.bifrostClient.GeneratePromptWithFormat(ctx, sysPromptStr, userPromptStr, EmailResponseFormat())
				cancel()
				if err == nil && aiBody != "" {
					cleanBody := CleanJSONResponse(aiBody)
					var structOut EmailStructuredOutput
					if err := json.Unmarshal([]byte(cleanBody), &structOut); err == nil && structOut.Content != "" {
						if structOut.Subject != "" {
							m.subject = structOut.Subject
						}
						sig := ResolveSignatureAdvanced(SignatureOpts{
							Subscriber: m.Subscriber,
						})
						finalContent := FormatPlainTextWithSignature(structOut.Content, sig)
						m.body = []byte(finalContent)
						return nil
					}

					m.body = []byte(aiBody)
					return nil
				}
			}
		}
	}

	// Render the subject if it's a template.
	if m.Campaign.SubjectTpl != nil {
		if err := m.Campaign.SubjectTpl.ExecuteTemplate(&out, models.ContentTpl, m); err != nil {
			return err
		}
		m.subject = out.String()
		out.Reset()
	}

	// Compile the main template.
	if err := m.Campaign.Tpl.ExecuteTemplate(&out, models.BaseTpl, m); err != nil {
		return err
	}
	m.body = out.Bytes()

	// Is there an alt body?
	if m.Campaign.ContentType != models.CampaignContentTypePlain && m.Campaign.AltBody.Valid {
		if m.Campaign.AltBodyTpl != nil {
			b := bytes.Buffer{}
			if err := m.Campaign.AltBodyTpl.ExecuteTemplate(&b, models.ContentTpl, m); err != nil {
				return err
			}
			m.altBody = b.Bytes()
		} else {
			m.altBody = []byte(m.Campaign.AltBody.String)
		}
	}

	// If there are templated headers, compile them.
	if m.Campaign.HeaderTpls == nil {
		m.headers = m.Campaign.Headers
	} else {
		hdrOut := bytes.Buffer{}
		m.headers = make(models.Headers, len(m.Campaign.Headers))
		for i, set := range m.Campaign.Headers {
			m.headers[i] = make(map[string]string, len(set))
			for hdr, val := range set {
				tpl := m.Campaign.HeaderTpls[i][hdr]
				if tpl == nil {
					m.headers[i][hdr] = val
					continue
				}
				hdrOut.Reset()
				if err := tpl.ExecuteTemplate(&hdrOut, models.ContentTpl, m); err != nil {
					return fmt.Errorf("error rendering header %q: %v", hdr, err)
				}
				m.headers[i][hdr] = hdrOut.String()
			}
		}
	}

	return nil
}

// Subject returns a copy of the message subject
func (m *CampaignMessage) Subject() string {
	return m.subject
}

// Body returns a copy of the message body.
func (m *CampaignMessage) Body() []byte {
	out := make([]byte, len(m.body))
	copy(out, m.body)
	return out
}

// AltBody returns a copy of the message's alt body.
func (m *CampaignMessage) AltBody() []byte {
	out := make([]byte, len(m.altBody))
	copy(out, m.altBody)
	return out
}
