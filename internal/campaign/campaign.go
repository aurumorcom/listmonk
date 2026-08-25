// Package campaign provides campaign and multi-step sequence execution engines.
package campaign

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	htmltpl "html/template"
	"log/slog"
	"maps"
	"net/textproto"
	"strings"
	"sync"
	txttpl "text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/gofrs/uuid/v5"
	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/core"
	"github.com/knadh/listmonk/internal/i18n"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/internal/media"
	"github.com/knadh/listmonk/internal/utils"
	"github.com/knadh/listmonk/models"
	null "gopkg.in/volatiletech/null.v6"
)

var (
	_campaignInstance *Manager
	_campaignOnce     sync.Once
)

// CampaignManager returns the thread-safe singleton instance of Campaign Manager.
func CampaignManager(c *core.Core, msgrs map[string]manager.Messenger, store media.Store, logger *slog.Logger) *Manager {
	_campaignOnce.Do(func() {
		_campaignInstance = NewManager(c, msgrs, store, logger)
	})
	return _campaignInstance
}

// Manager handles scheduled processing of sequences and campaign steps.
type Manager struct {
	core          *core.Core
	messengers    map[string]manager.Messenger
	mediaStore    media.Store
	bifrostClient *manager.BifrostClient
	logger        *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// TemplateFuncs returns template functions to be applied during sequence step rendering.
func (m *Manager) TemplateFuncs() htmltpl.FuncMap {
	return m.TemplateFuncsWithContext("", "")
}

// TemplateFuncsWithContext returns template functions configured with sequence and subscriber tracking context.
func (m *Manager) TemplateFuncsWithContext(seqUUID, subUUID string, stepID ...int) htmltpl.FuncMap {
	var rootURL string
	var i18nInst *i18n.I18n
	if m != nil && m.core != nil {
		i18nInst = m.core.I18n()
		if st, err := m.core.GetSettings(); err == nil {
			rootURL = strings.TrimRight(st.AppRootURL, "/")
		}
	}

	var curStepID int
	if len(stepID) > 0 && stepID[0] > 0 {
		curStepID = stepID[0]
	}

	f := htmltpl.FuncMap{
		"L": func() *i18n.I18n {
			return i18nInst
		},
		"TrackLink": func(url string, args ...any) string {
			if strings.TrimSpace(url) == "" {
				return ""
			}
			url = strings.ReplaceAll(url, "&", "&")
			if seqUUID == "" || subUUID == "" {
				return url
			}

			if m != nil && m.core != nil {
				var seqID, subID int
				if seq, err := m.core.GetSequence(0, seqUUID); err == nil {
					seqID = seq.ID
				}
				if sub, err := m.core.GetSubscriber(0, subUUID, ""); err == nil {
					subID = sub.ID
				}
				if linkID, err := m.core.GetOrCreateLinkID(url); err == nil && linkID > 0 {
					token := utils.EncodeSqidsLink(linkID, true, seqID, subID, curStepID)
					if token != "" {
						if rootURL != "" {
							return fmt.Sprintf("%s/link/%s", strings.TrimRight(rootURL, "/"), token)
						}
						return fmt.Sprintf("/link/%s", token)
					}
				}
			}

			var uu string
			if m != nil && m.core != nil {
				if lUUID, err := m.core.CreateLink(url); err == nil {
					uu = lUUID
				} else if m.logger != nil {
					m.logger.Error("failed creating sequence link tracking record", slog.String("url", url), slog.String("error", err.Error()))
				}
			}
			if uu == "" {
				uu = uuid.Must(uuid.NewV4()).String()
			}
			if rootURL != "" {
				return fmt.Sprintf("%s/link/%s/%s/%s", rootURL, uu, seqUUID, subUUID)
			}
			return fmt.Sprintf("/link/%s/%s/%s", uu, seqUUID, subUUID)
		},
		"TrackView": func(args ...any) htmltpl.HTML {
			if seqUUID == "" || subUUID == "" {
				return htmltpl.HTML("")
			}
			var pxURL string
			if curStepID > 0 {
				pxURL = fmt.Sprintf("%s/campaign/%s/%s/px.png?step_id=%d", rootURL, seqUUID, subUUID, curStepID)
				if rootURL == "" {
					pxURL = fmt.Sprintf("/campaign/%s/%s/px.png?step_id=%d", seqUUID, subUUID, curStepID)
				}
			} else {
				pxURL = fmt.Sprintf("%s/campaign/%s/%s/px.png", rootURL, seqUUID, subUUID)
				if rootURL == "" {
					pxURL = fmt.Sprintf("/campaign/%s/%s/px.png", seqUUID, subUUID)
				}
			}
			return htmltpl.HTML(fmt.Sprintf(`<img src="%s" width="1" height="1" style="display:none;max-height:0;max-width:0;opacity:0" alt="" />`, pxURL))
		},
		"UnsubscribeURL": func(args ...any) string {
			if subUUID == "" {
				return ""
			}
			if rootURL != "" {
				return fmt.Sprintf("%s/subscription/optin/%s", rootURL, subUUID)
			}
			return fmt.Sprintf("/subscription/optin/%s", subUUID)
		},
		"ManageURL": func(args ...any) string {
			if subUUID == "" {
				return ""
			}
			if rootURL != "" {
				return fmt.Sprintf("%s/subscription/optin/%s", rootURL, subUUID)
			}
			return fmt.Sprintf("/subscription/optin/%s", subUUID)
		},
		"OptinURL": func(args ...any) string {
			if subUUID == "" {
				return ""
			}
			if rootURL != "" {
				return fmt.Sprintf("%s/subscription/optin/%s", rootURL, subUUID)
			}
			return fmt.Sprintf("/subscription/optin/%s", subUUID)
		},
		"MessageURL": func(args ...any) string {
			if seqUUID == "" || subUUID == "" {
				return ""
			}
			if rootURL != "" {
				return fmt.Sprintf("%s/campaign/%s/%s", rootURL, seqUUID, subUUID)
			}
			return fmt.Sprintf("/campaign/%s/%s", seqUUID, subUUID)
		},
		"ArchiveURL": func() string {
			if rootURL != "" {
				return fmt.Sprintf("%s/archive", rootURL)
			}
			return "/archive"
		},
		"RootURL": func() string {
			return rootURL
		},
		"Date": func(layout string) string {
			if layout == "" {
				layout = time.ANSIC
			}
			return time.Now().Format(layout)
		},
		"Safe": func(safeHTML string) htmltpl.HTML {
			return htmltpl.HTML(safeHTML)
		},
	}

	sprigFuncs := sprig.GenericFuncMap()
	delete(sprigFuncs, "env")
	delete(sprigFuncs, "expandenv")
	delete(sprigFuncs, "getHostByName")

	maps.Copy(f, sprigFuncs)
	return f
}

// SetBifrostClient sets the Bifrost AI client on the sequence manager.
func (m *Manager) SetBifrostClient(bc *manager.BifrostClient) {
	m.bifrostClient = bc
}

// NewManager returns a new Sequence Manager.
func NewManager(c *core.Core, msgrs map[string]manager.Messenger, store media.Store, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		core:       c,
		messengers: msgrs,
		mediaStore: store,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the background scheduler loop.
func (m *Manager) Start(interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Minute
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				if err := m.ProcessBatch(); err != nil {
					m.logger.Error("sequence scheduler batch processing error", slog.String("error", err.Error()))
				}
			}
		}
	}()
}

// Stop stops the scheduler loop.
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

// ProcessBatch processes due sequence subscribers sequentially based on step delays.
func (m *Manager) ProcessBatch() error {
	subs, err := m.core.GetDueSequenceSubscribers(100)
	if err != nil {
		return err
	}

	m.logger.Debug("executing sequence step processing batch", slog.Int("due_subscribers_count", len(subs)))

	for _, sub := range subs {
		steps, err := m.core.GetSequenceSteps(sub.CampaignID)
		if err != nil {
			m.logger.Error("failed getting sequence steps", slog.Int("campaign_id", sub.CampaignID), slog.String("error", err.Error()))
			continue
		}

		if len(steps) == 0 || sub.CurrentStep > len(steps) {
			_ = m.core.UpdateSequenceSubscriberStatus(sub.CampaignID, sub.SubscriberID, models.CampaignSubscriberStatusFinished, sub.CurrentStep, null.Time{}, sub.LastMessageID, sub.LastThreadMsgID)
			continue
		}

		step := steps[sub.CurrentStep-1]

		subscriber, err := m.core.GetSubscriber(sub.SubscriberID, "", "")
		if err != nil {
			m.logger.Error("failed resolving subscriber for sequence step", slog.Int("subscriber_id", sub.SubscriberID), slog.String("error", err.Error()))
			continue
		}

		if err := m.PrepareAndDispatchStep(sub, subscriber, step, ""); err != nil {
			m.logger.Error("failed to dispatch sequence step", slog.Int("step_id", step.ID), slog.Int("subscriber_id", sub.SubscriberID), slog.String("error", err.Error()))
			continue
		}
		m.logger.Info("dispatched sequence step to subscriber", slog.Int("campaign_id", sub.CampaignID), slog.Int("step_id", step.ID), slog.Int("subscriber_id", sub.SubscriberID))
	}

	return nil
}

// PrepareAndDispatchStep compiles and dispatches a sequence step for a subscriber.
func (m *Manager) PrepareAndDispatchStep(sub models.CampaignSubscriber, subscriber models.Subscriber, step models.CampaignStep, overrideRecipient string) error {
	msgr, ok := m.messengers[step.Messenger]
	if !ok {
		if step.Messenger == "whatsapp" {
			msgr, ok = m.messengers["waha"]
		} else if step.Messenger == "waha" {
			msgr, ok = m.messengers["whatsapp"]
		}
		if !ok {
			isWhatsApp := step.Messenger == "whatsapp" || step.Messenger == "waha" || strings.HasPrefix(step.Messenger, "whatsapp-") || strings.HasPrefix(step.Messenger, "waha-")
			for name, cand := range m.messengers {
				if isWhatsApp && (name == "whatsapp" || name == "waha" || strings.HasPrefix(name, "whatsapp-") || strings.HasPrefix(name, "waha-")) {
					msgr = cand
					ok = true
					break
				} else if !isWhatsApp && (strings.HasPrefix(name, step.Messenger) || strings.HasPrefix(step.Messenger, name)) {
					msgr = cand
					ok = true
					break
				}
			}
		}
		if !ok {
			return fmt.Errorf("no suitable messenger found for messenger type %q", step.Messenger)
		}
	}

	var activeEmail *models.Email
	if step.Messenger == "email" || msgr.Name() == "email" || strings.HasPrefix(msgr.Name(), "email-") {
		if sub.EmailID.Valid && m.core != nil {
			mb, err := m.core.GetEmail(sub.EmailID.Int)
			if err != nil {
				m.logger.Error("failed resolving assigned email account", slog.Int("email_id", sub.EmailID.Int), slog.Int("contact_id", sub.SubscriberID), slog.String("error", err.Error()))
			} else {
				activeEmail = mb
			}
		}
		if activeEmail == nil && m.core != nil {
			if emails, err := m.core.GetEmails(); err == nil && len(emails) > 0 {
				activeEmail = &emails[0]
			}
		}
		if activeEmail == nil && m.core == nil {
			activeEmail = &models.Email{
				Name:  "Test Account",
				Email: "test@example.com",
				SMTPConfig: models.JSON{
					"from_addresses": []any{"test@example.com"},
				},
			}
		}
		if activeEmail == nil {
			return fmt.Errorf("no active email sending account configured for sequence step %d", step.StepNumber)
		}
	}

	var fromEmail string
	if sub.FromAddress.Valid && strings.TrimSpace(sub.FromAddress.String) != "" {
		fromEmail = strings.TrimSpace(sub.FromAddress.String)
	} else if activeEmail != nil {
		fromEmail = activeEmail.FromAddress()
	}

	if activeEmail != nil && overrideRecipient == "" {
		maxPerDay := activeEmail.GetSMTPSettings().MaxSendPerDay
		if maxPerDay == 0 {
			maxPerDay = activeEmail.MaxSendPerDay
		}
		if maxPerDay > 0 {
			sentForAddr := activeEmail.GetAddressSent(fromEmail)
			if sentForAddr >= maxPerDay || activeEmail.GetTotalSent() >= maxPerDay {
				foundAlternative := false
				for _, altAddr := range activeEmail.FromAddresses() {
					if activeEmail.GetAddressSent(altAddr) < maxPerDay {
						fromEmail = altAddr
						foundAlternative = true
						break
					}
				}
				if !foundAlternative {
					m.logger.Warn("mailbox sending threshold approaching", slog.Int("mailbox_id", activeEmail.ID), slog.String("email", activeEmail.Email), slog.String("from", fromEmail), slog.Int("sent_today", sentForAddr), slog.Int("max_per_day", maxPerDay), slog.Int("contact_id", sub.SubscriberID))
					deferSend := null.TimeFrom(time.Now().Add(24 * time.Hour))
					if m.core != nil {
						_ = m.core.UpdateSequenceSubscriberStatus(sub.CampaignID, sub.SubscriberID, sub.Status, sub.CurrentStep, deferSend, sub.LastMessageID, sub.LastThreadMsgID)
					}
					return fmt.Errorf("email account %d address %s reached daily limit", activeEmail.ID, fromEmail)
				}
			}
		}
	}

	isWhatsApp := step.Messenger == "whatsapp" || step.Messenger == "waha" || strings.HasPrefix(step.Messenger, "whatsapp-") || strings.HasPrefix(step.Messenger, "waha-")
	toEmails, toPhone := ResolveTargetRecipient(subscriber, overrideRecipient, isWhatsApp)

	msgID := fmt.Sprintf("<sequence-%d-%d-%s@listmonk>", sub.CampaignID, step.StepNumber, uuid.Must(uuid.NewV4()).String())
	msg := models.Message{
		Subscriber: subscriber,
		Subject:    step.Subject,
		Body:       []byte(step.Body),
		Messenger:  msgr.Name(),
		To:         toEmails,
		ToPhone:    toPhone,
	}

	displayName, assignedUser := ResolveSenderDisplayName(subscriber, activeEmail, overrideRecipient != "", m.core)
	msg.From = FormatSenderFromHeader(displayName, fromEmail)

	var seqUUID string
	if m.core != nil && sub.CampaignID > 0 {
		if seq, err := m.core.GetSequence(sub.CampaignID, ""); err == nil {
			seqUUID = seq.UUID
		}
	}

	scope := manager.ExtractTemplateScopeAdvanced(subscriber, assignedUser)
	if seqUUID != "" {
		if rawCamp, ok := scope["Campaign"].(map[string]any); ok {
			rawCamp["UUID"] = seqUUID
			rawCamp["Subject"] = msg.Subject
			scope["Campaign"] = rawCamp
		}
	}

	if step.TemplateID.Valid && step.TemplateID.Int > 0 && m.core != nil {
		tpl, err := m.core.GetTemplate(step.TemplateID.Int, false)
		if err == nil && tpl.Type == models.TemplateTypePrompt {
			sysPromptStr := tpl.Body
			if st, err := txttpl.New("sys").Parse(sysPromptStr); err == nil {
				var sb bytes.Buffer
				if err := st.Execute(&sb, scope); err == nil {
					sysPromptStr = sb.String()
				}
			}

			userPromptStr := step.Body
			if ut, err := txttpl.New("user").Parse(userPromptStr); err == nil {
				var ub bytes.Buffer
				if err := ut.Execute(&ub, scope); err == nil {
					userPromptStr = ub.String()
				}
			}

			if m.bifrostClient != nil {
				var respFormat *manager.BifrostResponseFormat
				isWhatsApp := step.Messenger == "whatsapp" || step.Messenger == "waha" || msgr.Name() == "whatsapp" || msgr.Name() == "waha" || strings.HasPrefix(msgr.Name(), "whatsapp-") || strings.HasPrefix(msgr.Name(), "waha-")
				if !isWhatsApp {
					respFormat = manager.EmailResponseFormat()
				} else {
					respFormat = manager.MessageResponseFormat()
				}

				ctx, cancel := m.bifrostClient.TimeoutContext()
				aiBody, err := m.bifrostClient.GeneratePromptWithFormat(ctx, sysPromptStr, userPromptStr, respFormat)
				cancel()
				if err != nil {
					if overrideRecipient == "" {
						m.logger.Error("Bifrost AI prompt generation failed for step", slog.Int("step_id", step.ID), slog.Int("subscriber_id", subscriber.ID), slog.String("error", err.Error()))
						deferSend := null.TimeFrom(time.Now().Add(1 * time.Hour))
						_ = m.core.UpdateSequenceSubscriberStatus(sub.CampaignID, sub.SubscriberID, sub.Status, sub.CurrentStep, deferSend, sub.LastMessageID, sub.LastThreadMsgID)
					}
					return fmt.Errorf("bifrost AI generation failed: %w", err)
				}

				cleanBody := manager.CleanJSONResponse(aiBody)
				if isWhatsApp {
					var msgOut manager.MessageStructuredOutput
					if err := json.Unmarshal([]byte(cleanBody), &msgOut); err == nil && msgOut.Message != "" {
						msg.Body = []byte(msgOut.Message)
					} else {
						msg.Body = []byte(aiBody)
					}
				} else {
					var emailOut manager.EmailStructuredOutput
					var finalContent string
					if err := json.Unmarshal([]byte(cleanBody), &emailOut); err == nil && emailOut.Content != "" {
						if emailOut.Subject != "" {
							msg.Subject = emailOut.Subject
						}
						sig := manager.ResolveSignatureAdvanced(manager.SignatureOpts{
							Subscriber: subscriber,
							Email:      activeEmail,
							User:       assignedUser,
						})
						finalContent = manager.FormatPlainTextWithSignature(emailOut.Content, sig)
					} else {
						finalContent = aiBody
					}

					if tpl.ParentTemplateID.Valid && tpl.ParentTemplateID.Int > 0 {
						if parentTpl, err := m.core.GetTemplate(int(tpl.ParentTemplateID.Int), false); err == nil {
							camp := models.Campaign{
								UUID:         uuid.Must(uuid.NewV4()).String(),
								Subject:      msg.Subject,
								TemplateBody: parentTpl.Body,
								Body:         finalContent,
							}
							funcs := m.TemplateFuncsWithContext(seqUUID, subscriber.UUID, step.ID)
							if err := camp.CompileTemplate(funcs); err == nil {
								var buf bytes.Buffer
								if err := camp.Tpl.ExecuteTemplate(&buf, models.BaseTpl, scope); err == nil {
									msg.Body = buf.Bytes()
								} else {
									msg.Body = []byte(finalContent)
								}
							} else {
								msg.Body = []byte(finalContent)
							}
						} else {
							msg.Body = []byte(finalContent)
						}
					} else {
						msg.Body = []byte(finalContent)
					}
				}
			}
		} else if err == nil && tpl.Body != "" {
			isWhatsApp := step.Messenger == "whatsapp" || step.Messenger == "waha" || msgr.Name() == "whatsapp" || msgr.Name() == "waha" || strings.HasPrefix(msgr.Name(), "whatsapp-") || strings.HasPrefix(msgr.Name(), "waha-")
			if isWhatsApp {
				funcs := txttpl.FuncMap(m.TemplateFuncsWithContext(seqUUID, subscriber.UUID, step.ID))
				bodyStr := models.SubstituteTplShorthand(step.Body)
				subjStr := models.SubstituteTplShorthand(msg.Subject)

				if ut, err := txttpl.New("body").Funcs(funcs).Parse(bodyStr); err == nil {
					var ub bytes.Buffer
					if err := ut.Execute(&ub, scope); err == nil {
						msg.Body = []byte(manager.StripHTML(ub.String()))
					} else {
						msg.Body = []byte(manager.StripHTML(step.Body))
					}
				} else {
					msg.Body = []byte(manager.StripHTML(step.Body))
				}
				if st, err := txttpl.New("subj").Funcs(funcs).Parse(subjStr); err == nil {
					var sb bytes.Buffer
					if err := st.Execute(&sb, scope); err == nil {
						msg.Subject = sb.String()
					}
				}
			} else {
				camp := models.Campaign{
					UUID:         uuid.Must(uuid.NewV4()).String(),
					Subject:      msg.Subject,
					TemplateBody: tpl.Body,
					Body:         step.Body,
				}
				funcs := m.TemplateFuncsWithContext(seqUUID, subscriber.UUID, step.ID)
				if err := camp.CompileTemplate(funcs); err == nil {
					var buf bytes.Buffer
					if err := camp.Tpl.ExecuteTemplate(&buf, models.BaseTpl, scope); err == nil {
						msg.Body = buf.Bytes()
					} else if m.logger != nil {
						m.logger.Error("sequence step HTML template execution error", slog.Int("step_id", step.ID), slog.Int("subscriber_id", subscriber.ID), slog.String("error", err.Error()))
					}
				} else if m.logger != nil {
					m.logger.Error("sequence step HTML template compilation error", slog.Int("step_id", step.ID), slog.Int("subscriber_id", subscriber.ID), slog.String("error", err.Error()))
				}
			}
		}
	} else {
		funcs := txttpl.FuncMap(m.TemplateFuncsWithContext(seqUUID, subscriber.UUID, step.ID))
		bodyStr := models.SubstituteTplShorthand(string(msg.Body))
		subjStr := models.SubstituteTplShorthand(msg.Subject)

		if ut, err := txttpl.New("body").Funcs(funcs).Parse(bodyStr); err == nil {
			var ub bytes.Buffer
			if err := ut.Execute(&ub, scope); err == nil {
				rendered := ub.String()
				if step.Messenger == "whatsapp" || step.Messenger == "waha" || msgr.Name() == "whatsapp" || msgr.Name() == "waha" || strings.HasPrefix(msgr.Name(), "whatsapp-") || strings.HasPrefix(msgr.Name(), "waha-") {
					rendered = manager.StripHTML(rendered)
				}
				msg.Body = []byte(rendered)
			} else if m.logger != nil {
				m.logger.Error("sequence step text template execution error", slog.Int("step_id", step.ID), slog.Int("subscriber_id", subscriber.ID), slog.String("error", err.Error()))
			}
		} else if m.logger != nil {
			m.logger.Error("sequence step text template parse error", slog.Int("step_id", step.ID), slog.Int("subscriber_id", subscriber.ID), slog.String("error", err.Error()))
		}
		if st, err := txttpl.New("subj").Funcs(funcs).Parse(subjStr); err == nil {
			var sb bytes.Buffer
			if err := st.Execute(&sb, scope); err == nil {
				msg.Subject = sb.String()
			}
		}
	}

	if (step.Messenger == "whatsapp" || step.Messenger == "waha" || msgr.Name() == "whatsapp" || msgr.Name() == "waha" || strings.HasPrefix(msgr.Name(), "whatsapp-") || strings.HasPrefix(msgr.Name(), "waha-")) && sub.WhatsAppID.Valid && sub.WhatsAppID.String != "" && sub.WhatsAppID.String != "default" {
		msg.MessengerSession = sub.WhatsAppID.String
	}

	if len(step.MediaIDs) > 0 && m.mediaStore != nil {
		atts, err := m.core.GetStepAttachments(m.mediaStore, step.MediaIDs)
		if err != nil {
			m.logger.Error("error loading attachments for step", slog.Int("step_id", step.ID), slog.String("error", err.Error()))
		} else {
			msg.Attachments = atts
		}
	}

	nextLastThreadMsgID := sub.LastThreadMsgID
	if step.Messenger == "email" || msgr.Name() == "email" || strings.HasPrefix(msgr.Name(), "email-") {
		if step.StepNumber == 1 {
			nextLastThreadMsgID = null.StringFrom(msgID)
		} else if strings.EqualFold(step.EmailType, models.EmailTypeNewThread) || step.EmailType == "New Thread" {
			nextLastThreadMsgID = null.StringFrom(msgID)
		} else {
			replyToMsgID := sub.LastThreadMsgID.String
			if replyToMsgID == "" {
				replyToMsgID = sub.LastMessageID.String
			}
			if replyToMsgID != "" {
				msg.Headers = make(textproto.MIMEHeader)
				msg.Headers.Set("In-Reply-To", replyToMsgID)
				msg.Headers.Set("References", replyToMsgID)
			}
			if !nextLastThreadMsgID.Valid && replyToMsgID != "" {
				nextLastThreadMsgID = null.StringFrom(replyToMsgID)
			}
		}
	} else if sub.LastMessageID.Valid {
		msg.Headers = make(textproto.MIMEHeader)
		msg.Headers.Set("In-Reply-To", sub.LastMessageID.String)
		msg.Headers.Set("References", sub.LastMessageID.String)
	}

	if overrideRecipient != "" {
		if strings.Contains(overrideRecipient, "@") {
			msg.To = []string{overrideRecipient}
		} else {
			msg.ToPhone = overrideRecipient
		}
	}

	if err := msgr.Push(msg); err != nil {
		return fmt.Errorf("error pushing sequence message: %w", err)
	}

	if overrideRecipient != "" {
		return nil
	}

	if m.core != nil {
		_ = m.core.RecordSequenceStepHistory(sub.SubscriberID, step.StepNumber, step.Messenger, msg.Subject, string(msg.Body))
	}

	if activeEmail != nil && m.core != nil {
		_ = m.core.IncrementEmailAddressSent(activeEmail.ID, fromEmail)
	}

	if m.core != nil {
		steps, err := m.core.GetSequenceSteps(sub.CampaignID)
		if err == nil {
			nextStep := sub.CurrentStep + 1
			var nextSend null.Time
			status := models.CampaignSubscriberStatusInProgress

			if nextStep > len(steps) {
				status = models.CampaignSubscriberStatusFinished
			} else {
				delayDur, _ := utils.ParseDuration(steps[nextStep-1].Delay)
				nextSend = null.TimeFrom(time.Now().Add(delayDur))
			}

			_ = m.core.UpdateSequenceSubscriberStatus(sub.CampaignID, sub.SubscriberID, status, nextStep, nextSend, null.StringFrom(msgID), nextLastThreadMsgID)
		}
	}

	return nil
}

type coreUserGetter interface {
	GetUser(id int, email, username string) (auth.User, error)
}

// GetAssignedUser resolves the assigned user for a sequence contact/channel.
func GetAssignedUser(contact models.Subscriber, activeEmail *models.Email, store coreUserGetter) *auth.User {
	if activeEmail != nil && activeEmail.UserID.Valid && store != nil {
		if u, err := store.GetUser(activeEmail.UserID.Int, "", ""); err == nil {
			return &u
		}
	}
	return nil
}

// ResolveSenderDisplayName resolves the sender display name according to multi-tiered priority rules:
func ResolveSenderDisplayName(contact models.Subscriber, activeEmail *models.Email, isTest bool, store coreUserGetter) (string, *auth.User) {
	if userMap, ok := contact.Attribs["user"].(map[string]any); ok {
		if name, ok := userMap["name"].(string); ok && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name), nil
		}
	}
	if userMap, ok := contact.Attribs["assigned_user"].(map[string]any); ok {
		if name, ok := userMap["name"].(string); ok && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name), nil
		}
	}

	var assignedUser *auth.User
	if activeEmail != nil && activeEmail.UserID.Valid && store != nil {
		if u, err := store.GetUser(activeEmail.UserID.Int, "", ""); err == nil {
			assignedUser = &u
			if strings.TrimSpace(u.Name) != "" {
				return strings.TrimSpace(u.Name), &u
			}
		}
	}

	if isTest {
		if userMap, ok := contact.Attribs["active_user"].(map[string]any); ok {
			if name, ok := userMap["name"].(string); ok && strings.TrimSpace(name) != "" {
				return strings.TrimSpace(name), assignedUser
			}
		}
	}

	return "", assignedUser
}

// FormatSenderFromHeader formats a display name and base email into an RFC-5322 From header string.
func FormatSenderFromHeader(displayName string, fromEmail string) string {
	displayName = strings.TrimSpace(displayName)
	fromEmail = strings.TrimSpace(fromEmail)
	if fromEmail == "" {
		return ""
	}
	if displayName != "" {
		return fmt.Sprintf("%s <%s>", displayName, fromEmail)
	}
	return fromEmail
}

// ResolveTargetRecipient determines recipient target addresses (To / ToPhone) for Live vs Test dispatches.
func ResolveTargetRecipient(contact models.Subscriber, overrideRecipient string, isWhatsApp bool) ([]string, string) {
	if overrideRecipient != "" {
		if isWhatsApp {
			return nil, overrideRecipient
		}
		return []string{overrideRecipient}, contact.Phone.String
	}
	var to []string
	if contact.Email != "" {
		to = []string{contact.Email}
	}
	return to, contact.Phone.String
}
