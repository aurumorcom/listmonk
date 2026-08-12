package sequence

import (
	"context"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/knadh/listmonk/internal/core"
	"github.com/knadh/listmonk/internal/manager"
)

var (
	reOptOut     = regexp.MustCompile(`(?i)^\s*(stop|unsubscribe|cancel|quit|end|optout|opt-out|remove|block|take me off)\s*$`)
	reInterested = regexp.MustCompile(`(?i)^\s*(yes|sure|interested|tell me more|let's talk|lets talk|call me|pricing|demo|sounds good|schedule|schedule a call)\b`)
	reOOO        = regexp.MustCompile(`(?i)(out of office|automatic reply|auto-reply|autoresponse|on vacation|away from my desk)`)
)

// ReplyListener monitors incoming replies on all channels to stop active sequences or handle opt-out / OOO deferrals.
type ReplyListener struct {
	core          *core.Core
	bifrostClient *manager.BifrostClient
	log           *log.Logger
}

// NewReplyListener returns a new ReplyListener.
func NewReplyListener(c *core.Core, l *log.Logger) *ReplyListener {
	return &ReplyListener{
		core: c,
		log:  l,
	}
}

// SetBifrostClient sets the Bifrost AI client on the ReplyListener.
func (r *ReplyListener) SetBifrostClient(bc *manager.BifrostClient) {
	r.bifrostClient = bc
}

// ProcessReply records a recipient reply by email address, stopping active sequence steps for that subscriber.
func (r *ReplyListener) ProcessReply(fromEmail string) error {
	return r.ProcessReplyWithBody(fromEmail, false, "")
}

// ProcessReplyWithBody processes incoming replies across Email and WhatsApp using Layer 1 Regex and Layer 2 Bifrost AI.
func (r *ReplyListener) ProcessReplyWithBody(fromIdentifier string, isPhone bool, messageBody string) error {
	if fromIdentifier == "" || r.core == nil {
		return nil
	}

	trimmedBody := strings.TrimSpace(messageBody)

	// --- Layer 1: Fast-Path Regex Filter ---
	if trimmedBody != "" {
		// 1A. Fast Opt-Out Regex
		if reOptOut.MatchString(trimmedBody) {
			if r.log != nil {
				r.log.Printf("fast-path opt-out regex match from %s; cancelling active sequence & blocklisting", fromIdentifier)
			}
			return r.core.CancelSequenceContactForOptOut(fromIdentifier, isPhone)
		}

		// 1B. Fast Interested Regex
		if reInterested.MatchString(trimmedBody) {
			if r.log != nil {
				r.log.Printf("fast-path interested regex match from %s; marking sequence as replied", fromIdentifier)
			}
			if isPhone {
				return r.core.RecordSequenceReplyByPhone(fromIdentifier)
			}
			return r.core.RecordSequenceReply(fromIdentifier)
		}

		// 1C. Fast OOO Regex
		if reOOO.MatchString(trimmedBody) {
			if r.log != nil {
				r.log.Printf("fast-path OOO regex match from %s; parsing return date via AI", fromIdentifier)
			}
			nowStr := time.Now().Format(time.RFC3339)
			var returnDate time.Time
			var err error

			if r.bifrostClient != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				returnDate, err = r.bifrostClient.ExtractOOOReturnDate(ctx, trimmedBody, nowStr)
			}

			if err != nil || returnDate.IsZero() || returnDate.Before(time.Now()) {
				// Fallback OOO deferral: 72 hours (3 days)
				returnDate = time.Now().Add(72 * time.Hour)
			}

			return r.core.DeferSequenceContactOOO(fromIdentifier, isPhone, returnDate)
		}
	}

	// --- Layer 2: Bifrost AI Full Intent Classification ---
	if trimmedBody != "" && r.bifrostClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		nowStr := time.Now().Format(time.RFC3339)
		intentResult, err := r.bifrostClient.ClassifyReplyIntent(ctx, trimmedBody, nowStr)
		if err == nil && intentResult != nil {
			switch intentResult.Intent {
			case "opt_out":
				if r.log != nil {
					r.log.Printf("Bifrost AI classified opt_out from %s (%s); cancelling sequence & blocklisting", fromIdentifier, intentResult.Reason)
				}
				return r.core.CancelSequenceContactForOptOut(fromIdentifier, isPhone)

			case "out_of_office":
				var returnDate time.Time
				if intentResult.ReturnDate != "" {
					returnDate, _ = time.Parse(time.RFC3339, intentResult.ReturnDate)
				}
				if returnDate.IsZero() || returnDate.Before(time.Now()) {
					returnDate = time.Now().Add(72 * time.Hour)
				}
				if r.log != nil {
					r.log.Printf("Bifrost AI classified out_of_office from %s; deferring to %v", fromIdentifier, returnDate)
				}
				return r.core.DeferSequenceContactOOO(fromIdentifier, isPhone, returnDate)

			case "interested", "other":
				if r.log != nil {
					r.log.Printf("Bifrost AI classified %s from %s; marking sequence as replied", intentResult.Intent, fromIdentifier)
				}
				if isPhone {
					return r.core.RecordSequenceReplyByPhone(fromIdentifier)
				}
				return r.core.RecordSequenceReply(fromIdentifier)
			}
		}
	}

	// Fallback Default: Mark sequence as replied
	if isPhone {
		return r.core.RecordSequenceReplyByPhone(fromIdentifier)
	}
	return r.core.RecordSequenceReply(fromIdentifier)
}
