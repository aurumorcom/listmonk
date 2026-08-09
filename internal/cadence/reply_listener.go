package cadence

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/knadh/listmonk/internal/core"
)

// ReplyListener monitors incoming replies to stop active cadence sequences.
type ReplyListener struct {
	core *core.Core
	log  *log.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewReplyListener returns a new ReplyListener.
func NewReplyListener(c *core.Core, l *log.Logger) *ReplyListener {
	ctx, cancel := context.WithCancel(context.Background())
	return &ReplyListener{
		core:   c,
		log:    l,
		ctx:    ctx,
		cancel: cancel,
	}
}

// ProcessReply records a recipient reply by email address, stopping active cadences for that subscriber.
func (r *ReplyListener) ProcessReply(fromEmail string) error {
	if fromEmail == "" {
		return nil
	}
	r.log.Printf("reply detected from %s; cancelling active cadence steps", fromEmail)
	return r.core.RecordCadenceReply(fromEmail)
}

// Start starts background monitoring for replies.
func (r *ReplyListener) Start(interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Minute
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ticker.C:
				// Poll IMAP mailboxes if configured
				mailboxes, err := r.core.GetMailboxes()
				if err != nil {
					continue
				}
				for _, mb := range mailboxes {
					_ = mb // Reserved for IMAP worker polling
				}
			}
		}
	}()
}

// Stop stops the reply listener worker.
func (r *ReplyListener) Stop() {
	r.cancel()
	r.wg.Wait()
}
