package sequence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	htmltpl "html/template"
	"log"
	"net/textproto"
	"strings"
	"sync"
	txttpl "text/template"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/core"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/internal/media"
	"github.com/knadh/listmonk/internal/utils"
	"github.com/knadh/listmonk/models"
	null "gopkg.in/volatiletech/null.v6"
)

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
		if sub.EmailID.Valid {
			mb, err := m.core.GetEmail(sub.EmailID.Int)
			if err != nil {
				m.log.Printf("error resolving assigned email account %d for contact %d: %v", sub.EmailID.Int, sub.SubscriberID, err)
			} else {
				if overrideRecipient == "" && mb.MaxSendPerDay > 0 && mb.SentToday >= mb.MaxSendPerDay {
					m.log.Printf("email account %d (%s) reached daily limit (%d/%d), deferring sequence step for contact %d", mb.ID, mb.Email, mb.SentToday, mb.MaxSendPerDay, sub.SubscriberID)
					deferSend := null.TimeFrom(time.Now().Add(24 * time.Hour))
					_ = m.core.UpdateSequenceContactStatus(sub.SequenceID, sub.SubscriberID, sub.Status, sub.CurrentStep, deferSend, sub.LastMessageID, sub.LastThreadMsgID)
					return fmt.Errorf("email account %d reached daily limit", mb.ID)
				}
				activeEmail = mb
			}
		}
		if activeEmail == nil && m.core != nil {
			if emails, err := m.core.GetEmails(); err == nil && len(emails) > 0 {
				activeEmail = &emails[0]
			}
		}
	}

	msgID := fmt.Sprintf("<sequence-%d-%d-%s@listmonk>", sub.SequenceID, step.StepNumber, uuid.Must(uuid.NewV4()).String())
	msg := models.Message{
		Subscriber: contact,
		Subject:    step.Subject,
		Body:       []byte(step.Body),
		Messenger:  msgr.Name(),
	}

	// Resolve Sender Display Name and Persona
	var userName string
	var userEmail string
	if userMap, ok := contact.Attribs["user"].(map[string]any); ok {
		if name, ok := userMap["name"].(string); ok && strings.TrimSpace(name) != "" {
			userName = strings.TrimSpace(name)
		}
		if em, ok := userMap["email"].(string); ok && strings.TrimSpace(em) != "" {
			userEmail = strings.TrimSpace(em)
		}
	}

	var assignedUser *auth.User
	if activeEmail != nil && activeEmail.UserID.Valid && m.core != nil {
		if u, err := m.core.GetUser(activeEmail.UserID.Int, "", ""); err == nil {
			assignedUser = &u
			if userName == "" && u.Name != "" {
				userName = u.Name
			}
		}
	}

	if activeEmail != nil {
		fromEmail := activeEmail.Email
		if userName != "" {
			msg.From = fmt.Sprintf("%s <%s>", userName, fromEmail)
		} else if activeEmail.Name != "" {
			msg.From = fmt.Sprintf("%s <%s>", activeEmail.Name, fromEmail)
		} else {
			msg.From = fromEmail
		}
	} else if userEmail != "" {
		if userName != "" {
			msg.From = fmt.Sprintf("%s <%s>", userName, userEmail)
		} else {
			msg.From = userEmail
		}
	} else {
		if userName != "" {
			msg.From = fmt.Sprintf("%s <noreply@listmonk.app>", userName)
		} else {
			msg.From = "noreply@listmonk.app"
		}
	}

	scope := manager.ExtractTemplateScope(contact)

	if step.TemplateID.Valid && step.TemplateID.Int > 0 {
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
							if err := camp.CompileTemplate(htmltpl.FuncMap{}); err == nil {
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
			// Standard campaign/tx template assigned to step
			camp := models.Campaign{
				UUID:         uuid.Must(uuid.NewV4()).String(),
				Subject:      msg.Subject,
				TemplateBody: tpl.Body,
				Body:         step.Body,
			}
			if err := camp.CompileTemplate(htmltpl.FuncMap{}); err == nil {
				var buf bytes.Buffer
				if err := camp.Tpl.ExecuteTemplate(&buf, models.BaseTpl, scope); err == nil {
					msg.Body = buf.Bytes()
				}
			}
		}
	} else {
		// Plain text / standard template interpolation
		if ut, err := txttpl.New("body").Parse(string(msg.Body)); err == nil {
			var ub bytes.Buffer
			if err := ut.Execute(&ub, scope); err == nil {
				msg.Body = ub.Bytes()
			}
		}
		if st, err := txttpl.New("subj").Parse(msg.Subject); err == nil {
			var sb bytes.Buffer
			if err := st.Execute(&sb, scope); err == nil {
				msg.Subject = sb.String()
			}
		}
	}

	if (step.Messenger == "whatsapp" || step.Messenger == "waha" || msgr.Name() == "whatsapp" || msgr.Name() == "waha" || strings.HasPrefix(msgr.Name(), "whatsapp-") || strings.HasPrefix(msgr.Name(), "waha-")) && sub.WahaSession.Valid {
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
			msg.Subscriber.Email = overrideRecipient
		} else {
			msg.ToPhone = overrideRecipient
			msg.Subscriber.Phone = null.StringFrom(overrideRecipient)
		}
	}

	if err := msgr.Push(msg); err != nil {
		return fmt.Errorf("error pushing sequence message: %w", err)
	}

	// If running in test mode, do not mutate sequence contacts or step history
	if overrideRecipient != "" {
		return nil
	}

	_ = m.core.RecordSequenceStepHistory(sub.SubscriberID, step.StepNumber, step.Messenger, msg.Subject, string(msg.Body))

	if activeEmail != nil {
		_ = m.core.IncrementEmailSent(activeEmail.ID)
	}

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

	return nil
}
