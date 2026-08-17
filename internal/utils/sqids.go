package utils

import (
	"errors"
	"sync"

	"github.com/sqids/sqids-go"
)

var (
	defaultSqidsOnce sync.Once
	defaultSqids     *sqids.Sqids

	// ErrInvalidShortToken is returned when a short link token fails decoding.
	ErrInvalidShortToken = errors.New("invalid short link token")
)

// ShortLinkPayload contains the unpacked contextual parameters for a tracked link.
type ShortLinkPayload struct {
	LinkID       int
	IsSequence   bool
	EntityID     int
	SubscriberID int
	StepID       int
}

func initDefaultSqids() {
	s, err := sqids.New(sqids.Options{
		MinLength: 10,
	})
	if err != nil {
		panic(err)
	}
	defaultSqids = s
}

// GetDefaultSqids returns the singleton Sqids instance with minimum length 10.
func GetDefaultSqids() *sqids.Sqids {
	defaultSqidsOnce.Do(initDefaultSqids)
	return defaultSqids
}

// EncodeSqidsLink encodes linkID, isSequence, entityID, subscriberID, and optional stepID into a compact Sqids token.
func EncodeSqidsLink(linkID int, isSequence bool, entityID int, subscriberID int, stepID ...int) string {
	s := GetDefaultSqids()
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
		return ""
	}
	return token
}

// DecodeSqidsLink decodes a Sqids token back into its constituent integers.
func DecodeSqidsLink(token string) (ShortLinkPayload, error) {
	if token == "" {
		return ShortLinkPayload{}, ErrInvalidShortToken
	}
	s := GetDefaultSqids()
	nums := s.Decode(token)
	if len(nums) < 4 {
		return ShortLinkPayload{}, ErrInvalidShortToken
	}
	var stepID int
	if len(nums) >= 5 {
		stepID = int(nums[4])
	}
	return ShortLinkPayload{
		LinkID:       int(nums[0]),
		IsSequence:   nums[1] == 1,
		EntityID:     int(nums[2]),
		SubscriberID: int(nums[3]),
		StepID:       stepID,
	}, nil
}
