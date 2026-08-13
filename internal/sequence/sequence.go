package sequence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
				nextSend := null.TimeFrom(time.Now().Add(time.Duration(steps[nextStep-1].DelaySeconds) * time.Second))
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

		// Pick messenger
		msgr, ok := m.messengers[step.Messenger]
		if !ok {
			msgr, ok = m.messengers["email"]
			if !ok {
				m.log.Printf("no suitable messenger found for step %d (%s)", step.ID, step.Messenger)
				continue
			}
		}

		var activeEmail *models.Email
		if step.Messenger == "email" || msgr.Name() == "email" {
			if sub.EmailID.Valid {
				mb, err := m.core.GetEmail(sub.EmailID.Int)
				if err != nil {
					m.log.Printf("error resolving assigned email account %d for contact %d: %v", sub.EmailID.Int, sub.SubscriberID, err)
				} else {
					if mb.EmailsPerDay > 0 && mb.EmailsToday >= mb.EmailsPerDay {
						m.log.Printf("email account %d (%s) reached daily limit (%d/%d), deferring sequence step for contact %d", mb.ID, mb.Email, mb.EmailsToday, mb.EmailsPerDay, sub.SubscriberID)
						deferSend := null.TimeFrom(time.Now().Add(24 * time.Hour))
						_ = m.core.UpdateSequenceContactStatus(sub.SequenceID, sub.SubscriberID, sub.Status, sub.CurrentStep, deferSend, sub.LastMessageID, sub.LastThreadMsgID)
						continue
					}
					activeEmail = mb
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

		if step.TemplateID.Valid && step.TemplateID.Int > 0 {
			tpl, err := m.core.GetTemplate(step.TemplateID.Int, false)
			if err == nil && tpl.Type == models.TemplateTypePrompt {
				scope := manager.ExtractTemplateScope(contact)

				sysPromptStr := tpl.SystemPrompt
				if tpl.SystemPromptTpl != nil {
					var sb bytes.Buffer
					if err := tpl.SystemPromptTpl.Execute(&sb, scope); err == nil {
						sysPromptStr = sb.String()
					}
				} else if sysPromptStr != "" {
					if st, err := txttpl.New("sys").Parse(sysPromptStr); err == nil {
						var sb bytes.Buffer
						if err := st.Execute(&sb, scope); err == nil {
							sysPromptStr = sb.String()
						}
					}
				}

				userPromptStr := step.Body
				if userPromptStr == "" {
					userPromptStr = tpl.Body
				}
				if tpl.UserPromptTpl != nil {
					var ub bytes.Buffer
					if err := tpl.UserPromptTpl.Execute(&ub, scope); err == nil {
						userPromptStr = ub.String()
					}
				} else if tpl.Tpl != nil {
					var ub bytes.Buffer
					if err := tpl.Tpl.Execute(&ub, scope); err == nil {
						userPromptStr = ub.String()
					}
				} else if userPromptStr != "" {
					if ut, err := txttpl.New("user").Parse(userPromptStr); err == nil {
						var ub bytes.Buffer
						if err := ut.Execute(&ub, scope); err == nil {
							userPromptStr = ub.String()
						}
					}
				}

				if m.bifrostClient != nil {
					var respFormat *manager.BifrostResponseFormat
					if step.Messenger != "waha" && msgr.Name() != "waha" {
						respFormat = manager.EmailResponseFormat()
					}

					aiBody, err := m.bifrostClient.GeneratePromptWithFormat(m.bifrostClient.TimeoutContext(), sysPromptStr, userPromptStr, respFormat)
					if err != nil {
						m.log.Printf("Bifrost AI prompt generation failed for step %d, contact %d: %v", step.ID, contact.ID, err)
						deferSend := null.TimeFrom(time.Now().Add(1 * time.Hour))
						_ = m.core.UpdateSequenceContactStatus(sub.SequenceID, sub.SubscriberID, sub.Status, sub.CurrentStep, deferSend, sub.LastMessageID, sub.LastThreadMsgID)
						continue
					}

					cleanBody := manager.CleanJSONResponse(aiBody)
					if step.Messenger == "waha" || msgr.Name() == "waha" {
						var msgOut manager.MessageStructuredOutput
						if err := json.Unmarshal([]byte(cleanBody), &msgOut); err == nil && msgOut.Message != "" {
							msg.Body = []byte(msgOut.Message)
						} else {
							msg.Body = []byte(aiBody)
						}
					} else {
						var emailOut manager.EmailStructuredOutput
						if err := json.Unmarshal([]byte(cleanBody), &emailOut); err == nil && emailOut.Content != "" {
							if emailOut.Subject != "" {
								msg.Subject = emailOut.Subject
							}
							var assignedUser *auth.User
							if activeEmail != nil && activeEmail.UserID.Valid {
								if u, err := m.core.GetUser(activeEmail.UserID.Int, "", ""); err == nil {
									assignedUser = &u
								}
							}
							sig := manager.ResolveSignatureAdvanced(manager.SignatureOpts{
								Subscriber: contact,
								Email:      activeEmail,
								User:       assignedUser,
							})
							finalContent := manager.FormatPlainTextWithSignature(emailOut.Content, sig)
							msg.Body = []byte(finalContent)
						} else {
							msg.Body = []byte(aiBody)
						}
					}
				}
			}
		}

		if activeEmail != nil {
			msg.From = activeEmail.Email
		}

		if (step.Messenger == "waha" || msgr.Name() == "waha") && sub.WahaSession.Valid {
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
		if step.Messenger == "email" || msgr.Name() == "email" {
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

		if err := msgr.Push(msg); err != nil {
			m.log.Printf("error pushing sequence message for subscriber %d: %v", sub.SubscriberID, err)
			continue
		}

		_ = m.core.RecordSequenceStepHistory(sub.SubscriberID, step.StepNumber, step.Messenger, msg.Subject, string(msg.Body))

		if activeEmail != nil {
			_ = m.core.IncrementEmailSent(activeEmail.ID)
		}

		nextStep := sub.CurrentStep + 1
		var nextSend null.Time
		status := models.SequenceContactStatusInProgress

		if nextStep > len(steps) {
			status = models.SequenceContactStatusFinished
		} else {
			nextSend = null.TimeFrom(time.Now().Add(time.Duration(steps[nextStep-1].DelaySeconds) * time.Second))
		}

		_ = m.core.UpdateSequenceContactStatus(sub.SequenceID, sub.SubscriberID, status, nextStep, nextSend, null.StringFrom(msgID), nextLastThreadMsgID)
	}

	return nil
}
