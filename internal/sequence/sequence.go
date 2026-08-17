package sequence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	htmltpl "html/template"
	"log"
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
	sequenceInstance *Manager
	sequenceOnce     sync.Once
)

// GetSequenceManager returns the thread-safe singleton instance of Sequence Manager.
func GetSequenceManager(c *core.Core, msgrs map[string]manager.Messenger, store media.Store, l *log.Logger) *Manager {
	sequenceOnce.Do(func() {
		sequenceInstance = NewManager(c, msgrs, store, l)
	})
	return sequenceInstance
}

// Manager handles scheduled processing of sequences.
type Manager struct {
	core          *core.Core
	messengers    map[string]manager.Messenger
	mediaStore    media.Store
	bifrostClient *manager.BifrostClient
	log           *log.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// TemplateFuncs returns template functions to be applied during sequence step rendering.
func (m *Manager) TemplateFuncs() htmltpl.FuncMap {
	return m.TemplateFuncsWithContext("", "")
}

// TemplateFuncsWithContext returns template functions configured with sequence and subscriber tracking context.
func (m *Manager) TemplateFuncsWithContext(seqUUID, subUUID string) htmltpl.FuncMap {
	var rootURL string
	var i18nInst *i18n.I18n
	if m != nil && m.core != nil {
		i18nInst = m.core.I18n()
		if st, err := m.core.GetSettings(); err == nil {
			rootURL = strings.TrimRight(st.AppRootURL, "/")
		}
	}

	f := htmltpl.FuncMap{
		"L": func() *i18n.I18n {
			return i18nInst
		},
		"TrackLink": func(url string, args ...any) string {
			if strings.TrimSpace(url) == "" {
				return ""
			}
			url = strings.ReplaceAll(url, "&amp;", "&")
			if seqUUID == "" || subUUID == "" {
				return url
			}
			var uu string
			if m != nil && m.core != nil {
				if lUUID, err := m.core.CreateLink(url); err == nil {
					uu = lUUID
				} else if m.log != nil {
					m.log.Printf("error creating sequence link tracking record for %s: %v", url, err)
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
			pxURL := fmt.Sprintf("%s/campaign/%s/%s/px.png", rootURL, seqUUID, subUUID)
			if rootURL == "" {
				pxURL = fmt.Sprintf("/campaign/%s/%s/px.png", seqUUID, subUUID)
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
func NewManager(c *core.Core, msgrs map[string]manager.Messenger, store media.Store, l *log.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		core:       c,
		messengers: msgrs,
		mediaStore: store,
		log:        l,
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
					m.log.Printf("sequence scheduler error: %v", err)
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

// EvaluateStepCondition evaluates whether a step condition is satisfied.
func EvaluateStepCondition(cond string, sub models.SequenceContact) bool {
	switch cond {
	case models.SequenceConditionAlways:
		return true
	case models.SequenceConditionIfRead:
		return sub.LastReadAt.Valid
	case models.SequenceConditionIfNotRead:
		return !sub.LastReadAt.Valid
	case models.SequenceConditionIfClicked:
		return sub.LastClickedAt.Valid
	default:
		return true
	}
}

// ProcessBatch processes due sequence contacts.
func (m *Manager) ProcessBatch() error {
	subs, err := m.core.GetDueSequenceContacts(100)
	if err != nil {
		return err
	}

	for _, sub := range subs {
		steps, err := m.core.GetSequenceSteps(sub.SequenceID)
		if err != nil {
			m.log.Printf("error getting sequence steps for sequence %d: %v", sub.SequenceID, err)
			continue
		}

		if len(steps) == 0 || sub.CurrentStep > len(steps) {
			_ = m.core.UpdateSequenceContactStatus(sub.SequenceID, sub.SubscriberID, models.SequenceContactStatusFinished, sub.CurrentStep, null.Time{}, null.String{}, sub.LastThreadMsgID)
			continue
		}

		step := steps[sub.CurrentStep-1]

		if !EvaluateStepCondition(step.Condition, sub) {
			// Skip step if condition not met and advance to next step
			nextStep := sub.CurrentStep + 1
			if nextStep > len(steps) {
				_ = m.core.UpdateSequenceContactStatus(sub.SequenceID, sub.SubscriberID, models.SequenceContactStatusFinished, nextStep, null.Time{}, null.String{}, sub.LastThreadMsgID)
			} else {
				delayDur, _ := utils.ParseDuration(steps[nextStep-1].Delay)
				nextSend := null.TimeFrom(time.Now().Add(delayDur))
				_ = m.core.UpdateSequenceContactStatus(sub.SequenceID, sub.SubscriberID, models.SequenceContactStatusInProgress, nextStep, nextSend, null.String{}, sub.LastThreadMsgID)
			}
			continue
		}

		// Resolve contact details
		contact, err := m.core.GetContact(sub.SubscriberID, "", "")
		if err != nil {
			m.log.Printf("error resolving contact %d: %v", sub.SubscriberID, err)
			continue
		}

		if err := m.PrepareAndDispatchStep(sub, contact, step, ""); err != nil {
			m.log.Printf("error dispatching sequence step %d for subscriber %d: %v", step.ID, sub.SubscriberID, err)
			continue
		}
	}

	return nil
}

// PrepareAndDispatchStep compiles and dispatches a sequence step for a contact.
// If overrideRecipient is non-empty, it runs in test mode (dispatches with overridden recipient
// and avoids mutating contact state or recording history).
func (m *Manager) PrepareAndDispatchStep(sub models.SequenceContact, contact models.Subscriber, step models.SequenceStep, overrideRecipient string) error {
	// Pick messenger
	msgr, ok := m.messengers[step.Messenger]
	if !ok {
		if step.Messenger == "whatsapp" {
			msgr, ok = m.messengers["waha"]
		} else if step.Messenger == "waha" {
			msgr, ok = m.messengers["whatsapp"]
		}
		if !ok {
			// Try finding any messenger matching prefix for the target messenger type
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
				m.log.Printf("error resolving assigned email account %d for contact %d: %v", sub.EmailID.Int, sub.SubscriberID, err)
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
			// Standalone unit test environment fallback
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

	// Resolve From Address and Check Daily Send Limits
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
				// Address exhausted, attempt failover to alternate fromAddress
				foundAlternative := false
				for _, altAddr := range activeEmail.FromAddresses() {
					if activeEmail.GetAddressSent(altAddr) < maxPerDay {
						fromEmail = altAddr
						foundAlternative = true
						break
					}
				}
				if !foundAlternative {
					m.log.Printf("email account %d (%s) / address %s reached daily limit (%d/%d), deferring step for contact %d", activeEmail.ID, activeEmail.Email, fromEmail, sentForAddr, maxPerDay, sub.SubscriberID)
					deferSend := null.TimeFrom(time.Now().Add(24 * time.Hour))
					if m.core != nil {
						_ = m.core.UpdateSequenceContactStatus(sub.SequenceID, sub.SubscriberID, sub.Status, sub.CurrentStep, deferSend, sub.LastMessageID, sub.LastThreadMsgID)
					}
					return fmt.Errorf("email account %d address %s reached daily limit", activeEmail.ID, fromEmail)
				}
			}
		}
	}

	isWhatsApp := step.Messenger == "whatsapp" || step.Messenger == "waha" || strings.HasPrefix(step.Messenger, "whatsapp-") || strings.HasPrefix(step.Messenger, "waha-")
	toEmails, toPhone := ResolveTargetRecipient(contact, overrideRecipient, isWhatsApp)

	msgID := fmt.Sprintf("<sequence-%d-%d-%s@listmonk>", sub.SequenceID, step.StepNumber, uuid.Must(uuid.NewV4()).String())
	msg := models.Message{
		Subscriber: contact,
		Subject:    step.Subject,
		Body:       []byte(step.Body),
		Messenger:  msgr.Name(),
		To:         toEmails,
		ToPhone:    toPhone,
	}

	// Resolve Sender Display Name & From Header via modular helper functions
	displayName, assignedUser := ResolveSenderDisplayName(contact, activeEmail, overrideRecipient != "", m.core)
	msg.From = FormatSenderFromHeader(displayName, fromEmail)

	var seqUUID string
	if m.core != nil && sub.SequenceID > 0 {
		if seq, err := m.core.GetSequence(sub.SequenceID, ""); err == nil {
			seqUUID = seq.UUID
		}
	}

	scope := manager.ExtractTemplateScope(contact)
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
				}

				ctx, cancel := m.bifrostClient.TimeoutContext()
				aiBody, err := m.bifrostClient.GeneratePromptWithFormat(ctx, sysPromptStr, userPromptStr, respFormat)
				cancel()
				if err != nil {
					if overrideRecipient == "" {
						m.log.Printf("Bifrost AI prompt generation failed for step %d, contact %d: %v", step.ID, contact.ID, err)
						deferSend := null.TimeFrom(time.Now().Add(1 * time.Hour))
						_ = m.core.UpdateSequenceContactStatus(sub.SequenceID, sub.SubscriberID, sub.Status, sub.CurrentStep, deferSend, sub.LastMessageID, sub.LastThreadMsgID)
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
							Subscriber: contact,
							Email:      activeEmail,
							User:       assignedUser,
						})
						finalContent = manager.FormatPlainTextWithSignature(emailOut.Content, sig)
					} else {
						finalContent = aiBody
					}

					// If prompt template specifies a parent HTML layout wrapper, wrap content in parent template
					if tpl.ParentTemplateID.Valid && tpl.ParentTemplateID.Int > 0 {
						if parentTpl, err := m.core.GetTemplate(int(tpl.ParentTemplateID.Int), false); err == nil {
							camp := models.Campaign{
								UUID:         uuid.Must(uuid.NewV4()).String(),
								Subject:      msg.Subject,
								TemplateBody: parentTpl.Body,
								Body:         finalContent,
							}
							funcs := m.TemplateFuncsWithContext(seqUUID, contact.UUID)
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
				// WhatsApp steps bypass HTML email layout templates. Render step body directly and sanitize HTML/CSS.
				funcs := txttpl.FuncMap(m.TemplateFuncsWithContext(seqUUID, contact.UUID))
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
				// Standard campaign/tx template assigned to email step
				camp := models.Campaign{
					UUID:         uuid.Must(uuid.NewV4()).String(),
					Subject:      msg.Subject,
					TemplateBody: tpl.Body,
					Body:         step.Body,
				}
				funcs := m.TemplateFuncsWithContext(seqUUID, contact.UUID)
				if err := camp.CompileTemplate(funcs); err == nil {
					var buf bytes.Buffer
					if err := camp.Tpl.ExecuteTemplate(&buf, models.BaseTpl, scope); err == nil {
						msg.Body = buf.Bytes()
					} else if m.log != nil {
						m.log.Printf("sequence step %d HTML template execution error for subscriber %d: %v", step.ID, contact.ID, err)
					}
				} else if m.log != nil {
					m.log.Printf("sequence step %d HTML template compilation error for subscriber %d: %v", step.ID, contact.ID, err)
				}
			}
		}
	} else {
		// Plain text / standard template interpolation with full FuncMap and shorthand tags replacement
		funcs := txttpl.FuncMap(m.TemplateFuncsWithContext(seqUUID, contact.UUID))
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
			} else if m.log != nil {
				m.log.Printf("sequence step %d text template execution error for subscriber %d: %v", step.ID, contact.ID, err)
			}
		} else if m.log != nil {
			m.log.Printf("sequence step %d text template parse error for subscriber %d: %v", step.ID, contact.ID, err)
		}
		if st, err := txttpl.New("subj").Funcs(funcs).Parse(subjStr); err == nil {
			var sb bytes.Buffer
			if err := st.Execute(&sb, scope); err == nil {
				msg.Subject = sb.String()
			}
		}
	}

	if (step.Messenger == "whatsapp" || step.Messenger == "waha" || msgr.Name() == "whatsapp" || msgr.Name() == "waha" || strings.HasPrefix(msgr.Name(), "whatsapp-") || strings.HasPrefix(msgr.Name(), "waha-")) && sub.WahaSession.Valid && sub.WahaSession.String != "" && sub.WahaSession.String != "default" {
		msg.MessengerSession = sub.WahaSession.String
	}

	if len(step.MediaIDs) > 0 && m.mediaStore != nil {
		atts, err := m.core.GetStepAttachments(m.mediaStore, step.MediaIDs)
		if err != nil {
			m.log.Printf("error loading attachments for step %d: %v", step.ID, err)
		} else {
			msg.Attachments = atts
		}
	}

	// Threading headers resolution based on email_type and last_thread_msg_id
	nextLastThreadMsgID := sub.LastThreadMsgID
	if step.Messenger == "email" || msgr.Name() == "email" || strings.HasPrefix(msgr.Name(), "email-") {
		if step.StepNumber == 1 {
			// Email 1 starts initial thread root
			nextLastThreadMsgID = null.StringFrom(msgID)
		} else if strings.EqualFold(step.EmailType, models.EmailTypeNewThread) || step.EmailType == "New Thread" {
			// Email step is explicit "New Thread": start clean thread without In-Reply-To
			nextLastThreadMsgID = null.StringFrom(msgID)
		} else {
			// Email step is "Reply" or default: reply to the last new thread root
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
		// Non-email messengers fallback
		msg.Headers = make(textproto.MIMEHeader)
		msg.Headers.Set("In-Reply-To", sub.LastMessageID.String)
		msg.Headers.Set("References", sub.LastMessageID.String)
	}

	// Apply recipient override for test dispatches
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

	// If running in test mode, do not mutate sequence contacts or step history
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
		steps, err := m.core.GetSequenceSteps(sub.SequenceID)
		if err == nil {
			nextStep := sub.CurrentStep + 1
			var nextSend null.Time
			status := models.SequenceContactStatusInProgress

			if nextStep > len(steps) {
				status = models.SequenceContactStatusFinished
			} else {
				delayDur, _ := utils.ParseDuration(steps[nextStep-1].Delay)
				nextSend = null.TimeFrom(time.Now().Add(delayDur))
			}

			_ = m.core.UpdateSequenceContactStatus(sub.SequenceID, sub.SubscriberID, status, nextStep, nextSend, null.StringFrom(msgID), nextLastThreadMsgID)
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
// Tier 1: Contact Assigned User (contact.Attribs["user"] or contact.Attribs["assigned_user"])
// Tier 2: Messenger Assigned User (activeEmail.UserID)
// Tier 3: Active Triggering User (extra fallback for test messages)
// Zero Account Name Fallback: Never uses Email Account Name (models.Email.Name).
func ResolveSenderDisplayName(contact models.Subscriber, activeEmail *models.Email, isTest bool, store coreUserGetter) (string, *auth.User) {
	// Tier 1: Contact Assigned User (stored in contact.Attribs["user"] or contact.Attribs["assigned_user"])
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

	// Tier 2: Messenger Assigned User
	var assignedUser *auth.User
	if activeEmail != nil && activeEmail.UserID.Valid && store != nil {
		if u, err := store.GetUser(activeEmail.UserID.Int, "", ""); err == nil {
			assignedUser = &u
			if strings.TrimSpace(u.Name) != "" {
				return strings.TrimSpace(u.Name), &u
			}
		}
	}

	// Tier 3: Active Triggering User (Extra fallback for preview test messages)
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
