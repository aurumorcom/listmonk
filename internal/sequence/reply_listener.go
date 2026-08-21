// Package sequence provides drip and automated sequences engine.
package sequence

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/knadh/listmonk/internal/core"
	"github.com/knadh/listmonk/internal/manager"
)

type ChannelType int

const (
	ChannelTypeUnknown ChannelType = iota
	ChannelTypeEmail
	ChannelTypeWhatsApp
)

var (
	_reOptOut     = regexp.MustCompile(`(?i)^\s*(stop|unsubscribe|cancel|quit|end|optout|opt-out|remove|block|take me off)\s*$`)
	_reInterested = regexp.MustCompile(`(?i)^\s*(yes|sure|interested|tell me more|let's talk|lets talk|call me|pricing|demo|sounds good|schedule|schedule a call)\b`)
	_reOOO        = regexp.MustCompile(`(?i)(out of office|automatic reply|auto-reply|autoresponse|on vacation|away from my desk)`)
)

// ReplyListener monitors incoming replies on all channels to stop active sequences or handle opt-out / OOO deferrals.
type ReplyListener struct {
	core          *core.Core
	bifrostClient *manager.BifrostClient
	logger        *slog.Logger
}

// NewReplyListener returns a new ReplyListener.
func NewReplyListener(c *core.Core, logger *slog.Logger) *ReplyListener {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReplyListener{
		core:   c,
		logger: logger,
	}
}

// SetBifrostClient sets the Bifrost AI client on the ReplyListener.
func (r *ReplyListener) SetBifrostClient(bc *manager.BifrostClient) {
	r.bifrostClient = bc
}

// ProcessReply records a recipient reply by email address, stopping active sequence steps for that subscriber.
func (r *ReplyListener) ProcessReply(fromEmail string) error {
	return r.ProcessReplyWithBody(fromEmail, ChannelTypeEmail, "")
}

// ProcessReplyWithContext processes replies while respecting caller context cancellation.
func (r *ReplyListener) ProcessReplyWithContext(ctx context.Context, fromIdentifier string, channel ChannelType, messageBody string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return r.ProcessReplyWithBody(fromIdentifier, channel, messageBody)
	}
}

// ProcessReplyWithQuotedID processes incoming replies across Email and WhatsApp including quoted message IDs.
func (r *ReplyListener) ProcessReplyWithQuotedID(fromIdentifier string, channel ChannelType, messageBody string, quotedMsgID string) error {
	if fromIdentifier == "" || r.core == nil {
		return nil
	}

	isPhone := (channel == ChannelTypeWhatsApp) || strings.Contains(fromIdentifier, "@lid")
	trimmedBody := strings.TrimSpace(messageBody)

	// If a quoted message ID is present, also trigger read matching by message ID
	if quotedMsgID != "" {
		_ = r.core.RecordSequenceReadByMessageID(quotedMsgID)
	}

	// --- Layer 1: Fast-Path Regex Filter ---
	if trimmedBody != "" {
		// 1A. Fast Opt-Out Regex
		if _reOptOut.MatchString(trimmedBody) {
			r.logger.Info("fast-path reply classification matched", slog.String("from", fromIdentifier), slog.String("intent", "opt_out"))
			return r.core.CancelSequenceSubscriberForOptOut(fromIdentifier, isPhone)
		}

		// 1B. Fast Interested Regex
		if _reInterested.MatchString(trimmedBody) {
			r.logger.Info("fast-path reply classification matched", slog.String("from", fromIdentifier), slog.String("intent", "interested"))
			if isPhone {
				return r.core.RecordSequenceReplyByPhone(fromIdentifier)
			}
			return r.core.RecordSequenceReply(fromIdentifier)
		}

		// 1C. Fast OOO Regex
		if _reOOO.MatchString(trimmedBody) {
			r.logger.Info("fast-path reply classification matched", slog.String("from", fromIdentifier), slog.String("intent", "out_of_office"))
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

			return r.core.DeferSequenceSubscriberOOO(fromIdentifier, isPhone, returnDate)
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
				r.logger.Info("Bifrost AI classified reply intent", slog.String("from", fromIdentifier), slog.String("intent", "opt_out"), slog.String("reason", intentResult.Reason))
				return r.core.CancelSequenceSubscriberForOptOut(fromIdentifier, isPhone)

			case "out_of_office":
				var returnDate time.Time
				if intentResult.ReturnDate != "" {
					returnDate, _ = time.Parse(time.RFC3339, intentResult.ReturnDate)
				}
				if returnDate.IsZero() || returnDate.Before(time.Now()) {
					returnDate = time.Now().Add(72 * time.Hour)
				}
				r.logger.Info("Bifrost AI classified reply intent", slog.String("from", fromIdentifier), slog.String("intent", "out_of_office"), slog.Time("return_date", returnDate))
				return r.core.DeferSequenceSubscriberOOO(fromIdentifier, isPhone, returnDate)

			case "interested", "other":
				r.logger.Info("Bifrost AI classified reply intent", slog.String("from", fromIdentifier), slog.String("intent", intentResult.Intent))
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

// ProcessReplyWithBody processes incoming replies across Email and WhatsApp using Layer 1 Regex and Layer 2 Bifrost AI.
func (r *ReplyListener) ProcessReplyWithBody(fromIdentifier string, channel ChannelType, messageBody string) error {
	return r.ProcessReplyWithQuotedID(fromIdentifier, channel, messageBody, "")
}
