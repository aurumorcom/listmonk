package core

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/knadh/listmonk/models"
)

func TestComputeHMACSignature(t *testing.T) {
	secret := "test_secret_123"
	timestamp := int64(1700000000)
	payload := []byte(`{"event":"contact.created","data":{"id":1,"email":"test@example.com"}}`)

	sig := ComputeHMACSignature(secret, timestamp, payload)

	if sig == "" {
		t.Fatalf("expected non-empty HMAC signature")
	}

	// Verify header format t=<ts>,v1=<sig>
	expectedPrefix := "t=1700000000,v1="
	if len(sig) <= len(expectedPrefix) || sig[:len(expectedPrefix)] != expectedPrefix {
		t.Fatalf("unexpected signature format: %s, expected prefix: %s", sig, expectedPrefix)
	}

	// Signature for same inputs must be deterministic
	sig2 := ComputeHMACSignature(secret, timestamp, payload)
	if sig != sig2 {
		t.Fatalf("HMAC signature not deterministic: %s != %s", sig, sig2)
	}
}

func TestEventMarshalling(t *testing.T) {
	evt := models.Event{
		ID:        "evt_12345678",
		Event:     "contact.created",
		CreatedAt: time.Unix(1700000000, 0).UTC(),
		Data: map[string]any{
			"id":    101,
			"email": "user@example.com",
			"name":  "Test User",
		},
	}

	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	var unmarshalled models.Event
	if err := json.Unmarshal(b, &unmarshalled); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if unmarshalled.ID != "evt_12345678" || unmarshalled.Event != "contact.created" {
		t.Fatalf("unmarshalled values mismatch: %+v", unmarshalled)
	}
}
