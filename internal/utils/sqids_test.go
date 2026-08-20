package utils

import (
	"testing"
)

func TestDefaultSqids(t *testing.T) {
	s := DefaultSqids()
	if s == nil {
		t.Fatalf("expected DefaultSqids() to return non-nil sqids instance")
	}
}

func TestSqidsCodec_Roundtrip(t *testing.T) {
	testCases := []struct {
		name       string
		linkID     int
		isSequence bool
		entityID   int
		subID      int
		stepID     int
	}{
		{"Campaign Link", 10, false, 42, 1001, 0},
		{"Sequence Link without Step", 25, true, 5, 2002, 0},
		{"Sequence Link with StepID", 25, true, 5, 2002, 12},
		{"Zero SubID (Anonymous)", 99, false, 1, 0, 0},
		{"Large IDs", 15000000, true, 30000, 15000000, 99999},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var token string
			if tc.stepID > 0 {
				token = EncodeSqidsLink(tc.linkID, tc.isSequence, tc.entityID, tc.subID, tc.stepID)
			} else {
				token = EncodeSqidsLink(tc.linkID, tc.isSequence, tc.entityID, tc.subID)
			}
			if len(token) < 10 {
				t.Fatalf("expected token length >= 10, got %d ('%s')", len(token), token)
			}

			payload, err := DecodeSqidsLink(token)
			if err != nil {
				t.Fatalf("unexpected decode error: %v", err)
			}

			if payload.LinkID != tc.linkID {
				t.Errorf("LinkID mismatch: want %d, got %d", tc.linkID, payload.LinkID)
			}
			if payload.IsSequence != tc.isSequence {
				t.Errorf("IsSequence mismatch: want %v, got %v", tc.isSequence, payload.IsSequence)
			}
			if payload.EntityID != tc.entityID {
				t.Errorf("EntityID mismatch: want %d, got %d", tc.entityID, payload.EntityID)
			}
			if payload.SubscriberID != tc.subID {
				t.Errorf("SubscriberID mismatch: want %d, got %d", tc.subID, payload.SubscriberID)
			}
			if payload.StepID != tc.stepID {
				t.Errorf("StepID mismatch: want %d, got %d", tc.stepID, payload.StepID)
			}
		})
	}
}

func TestSqidsCodec_InvalidToken(t *testing.T) {
	_, err := DecodeSqidsLink("")
	if err == nil {
		t.Errorf("expected error for empty token, got nil")
	}

	_, err = DecodeSqidsLink("short")
	if err == nil {
		t.Errorf("expected error for short/malformed token, got nil")
	}
}
