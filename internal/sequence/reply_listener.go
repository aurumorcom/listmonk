package sequence

import (
	"log"

	"github.com/knadh/listmonk/internal/core"
)

// ReplyListener monitors incoming replies to stop active sequences.
type ReplyListener struct {
	core *core.Core
	log  *log.Logger
}

// NewReplyListener returns a new ReplyListener.
func NewReplyListener(c *core.Core, l *log.Logger) *ReplyListener {
	return &ReplyListener{
		core: c,
		log:  l,
	}
}

// ProcessReply records a recipient reply by email address, stopping active sequence steps for that subscriber.
func (r *ReplyListener) ProcessReply(fromEmail string) error {
	if fromEmail == "" || r.core == nil {
		return nil
	}
	if r.log != nil {
		r.log.Printf("reply detected from %s; cancelling active sequence steps", fromEmail)
	}
	return r.core.RecordSequenceReply(fromEmail)
}
