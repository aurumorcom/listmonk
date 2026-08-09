package cadence

import (
	"context"
	"fmt"
	"log"
	"net/textproto"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/knadh/listmonk/internal/core"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/internal/media"
	"github.com/knadh/listmonk/models"
	null "gopkg.in/volatiletech/null.v6"
)

// Manager handles scheduled processing of cadence sequences.
type Manager struct {
	core       *core.Core
	messengers map[string]manager.Messenger
	mediaStore media.Store
	log        *log.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager returns a new Cadence Manager.
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
					m.log.Printf("cadence scheduler error: %v", err)
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
func EvaluateStepCondition(cond string, sub models.CadenceSubscriber) bool {
	switch cond {
	case models.CadenceConditionAlways:
		return true
	case models.CadenceConditionIfRead:
		return sub.LastReadAt.Valid
	case models.CadenceConditionIfNotRead:
		return !sub.LastReadAt.Valid
	case models.CadenceConditionIfClicked:
		return sub.LastClickedAt.Valid
	default:
		return true
	}
}

// ProcessBatch processes due cadence subscribers.
func (m *Manager) ProcessBatch() error {
	subs, err := m.core.GetDueCadenceSubscribers(100)
	if err != nil {
		return err
	}

	for _, sub := range subs {
		steps, err := m.core.GetCadenceSteps(sub.CadenceID)
		if err != nil {
			m.log.Printf("error getting cadence steps for cadence %d: %v", sub.CadenceID, err)
			continue
		}

		if len(steps) == 0 || sub.CurrentStep > len(steps) {
			_ = m.core.UpdateCadenceSubscriberStatus(sub.CadenceID, sub.SubscriberID, models.CadenceSubStatusFinished, sub.CurrentStep, null.Time{}, null.String{})
			continue
		}

		step := steps[sub.CurrentStep-1]

		if !EvaluateStepCondition(step.Condition, sub) {
			// Skip step if condition not met and advance to next step
			nextStep := sub.CurrentStep + 1
			if nextStep > len(steps) {
				_ = m.core.UpdateCadenceSubscriberStatus(sub.CadenceID, sub.SubscriberID, models.CadenceSubStatusFinished, nextStep, null.Time{}, null.String{})
			} else {
				nextSend := null.TimeFrom(time.Now().Add(time.Duration(steps[nextStep-1].DelayDays) * 24 * time.Hour))
				_ = m.core.UpdateCadenceSubscriberStatus(sub.CadenceID, sub.SubscriberID, models.CadenceSubStatusInProgress, nextStep, nextSend, null.String{})
			}
			continue
		}

		// Resolve subscriber details
		subscriber, err := m.core.GetSubscriber(sub.SubscriberID, "", "")
		if err != nil {
			m.log.Printf("error resolving subscriber %d: %v", sub.SubscriberID, err)
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

		msgID := fmt.Sprintf("<cadence-%d-%d-%s@listmonk>", sub.CadenceID, step.StepNumber, uuid.Must(uuid.NewV4()).String())
		msg := models.Message{
			Subscriber: subscriber,
			Subject:    step.Subject,
			Body:       []byte(step.Body),
			Messenger:  msgr.Name(),
		}

		if len(step.MediaIDs) > 0 && m.mediaStore != nil {
			atts, err := m.core.GetStepAttachments(m.mediaStore, step.MediaIDs)
			if err != nil {
				m.log.Printf("error loading attachments for step %d: %v", step.ID, err)
			} else {
				msg.Attachments = atts
			}
		}

		// Threading headers if previous step message exists
		if sub.LastMessageID.Valid {
			msg.Headers = make(textproto.MIMEHeader)
			msg.Headers.Set("In-Reply-To", sub.LastMessageID.String)
			msg.Headers.Set("References", sub.LastMessageID.String)
		}

		if err := msgr.Push(msg); err != nil {
			m.log.Printf("error pushing cadence message for subscriber %d: %v", sub.SubscriberID, err)
			continue
		}

		nextStep := sub.CurrentStep + 1
		var nextSend null.Time
		status := models.CadenceSubStatusInProgress

		if nextStep > len(steps) {
			status = models.CadenceSubStatusFinished
		} else {
			nextSend = null.TimeFrom(time.Now().Add(time.Duration(steps[nextStep-1].DelayDays) * 24 * time.Hour))
		}

		_ = m.core.UpdateCadenceSubscriberStatus(sub.CadenceID, sub.SubscriberID, status, nextStep, nextSend, null.StringFrom(msgID))
	}

	return nil
}
