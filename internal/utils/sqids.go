// Package utils provides utility functions across listmonk packages.
package utils

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/sqids/sqids-go"
)

var (
	_defaultSqidsOnce sync.Once
	_defaultSqids     *sqids.Sqids

	// ErrInvalidShortToken is returned when a short link token fails decoding.
	ErrInvalidShortToken = errors.New("invalid short link token")
)

// ShortLinkPayload contains the unpacked contextual parameters for a tracked link.
type ShortLinkPayload struct {
	LinkID       int  `json:"link_id"`
	IsSequence   bool `json:"is_sequence"`
	EntityID     int  `json:"entity_id"`
	SubscriberID int  `json:"subscriber_id"`
	StepID       int  `json:"step_id,omitempty"`
}

func initDefaultSqids() {
	s, err := sqids.New(sqids.Options{
		MinLength: 10,
	})
	if err != nil {
		slog.Error("failed to initialize Sqids instance", slog.String("error", err.Error()))
		return
	}
	_defaultSqids = s
}

// Sqids returns the singleton Sqids instance with minimum length 10.
func Sqids() *sqids.Sqids {
	_defaultSqidsOnce.Do(initDefaultSqids)
	return _defaultSqids
}

// EncodeSqidsLink encodes linkID, isSequence, entityID, subscriberID, and optional stepID into a compact Sqids token.
func EncodeSqidsLink(linkID int, isSequence bool, entityID int, subscriberID int, stepID ...int) string {
	s := Sqids()
	if s == nil {
		slog.Error("sqids instance is uninitialized during link encoding")
		return ""
	}
	var isSeqNum uint64
	if isSequence {
		isSeqNum = 1
	}
	nums := []uint64{uint64(linkID), isSeqNum, uint64(entityID), uint64(subscriberID)}
	if len(stepID) > 0 && stepID[0] > 0 {
		nums = append(nums, uint64(stepID[0]))
	}
	token, err := s.Encode(nums)
	if err != nil {
		slog.Error("failed to encode sqids link", slog.Int("link_id", linkID), slog.String("error", err.Error()))
		return ""
	}
	return token
}

// DecodeSqidsLink decodes a Sqids token back into its constituent integers.
func DecodeSqidsLink(token string) (ShortLinkPayload, error) {
	if token == "" {
		slog.Warn("empty short link token received for decoding")
		return ShortLinkPayload{}, ErrInvalidShortToken
	}
	s := Sqids()
	if s == nil {
		slog.Error("sqids instance is uninitialized during link decoding")
		return ShortLinkPayload{}, ErrInvalidShortToken
	}
	nums := s.Decode(token)
	if len(nums) < 4 {
		slog.Warn("malformed short link token received for decoding", slog.String("token", token), slog.Int("decoded_nums_count", len(nums)))
		return ShortLinkPayload{}, ErrInvalidShortToken
	}
	var stepID int
	if len(nums) >= 5 {
		stepID = int(nums[4])
	}
	slog.Debug("successfully decoded sqids link token", slog.String("token", token), slog.Int("link_id", int(nums[0])), slog.Bool("is_sequence", nums[1] == 1), slog.Int("subscriber_id", int(nums[3])))
	return ShortLinkPayload{
		LinkID:       int(nums[0]),
		IsSequence:   nums[1] == 1,
		EntityID:     int(nums[2]),
		SubscriberID: int(nums[3]),
		StepID:       stepID,
	}, nil
}
