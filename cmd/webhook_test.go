package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/knadh/listmonk/internal/core"
)

func TestE2E_WebhookTestEndpoint(t *testing.T) {
	// Launch local httptest receiver
	received := make(chan bool, 1)
	var recHeader string
	var recBody []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recHeader = r.Header.Get("Listmonk-Signature")
		recBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		received <- true
	}))
	defer ts.Close()

	// Execute test webhook delivery directly via core method
	c := &core.Core{}
	err := c.TestWebhookEndpoint(ts.URL, "secret_123", "contact.created")
	if err != nil {
		t.Fatalf("unexpected TestWebhookEndpoint error: %v", err)
	}

	select {
	case <-received:
		if recHeader == "" {
			t.Fatalf("expected Listmonk-Signature header in test webhook receiver")
		}

		var payload map[string]any
		if err := json.Unmarshal(recBody, &payload); err != nil {
			t.Fatalf("failed to unmarshal test webhook payload: %v", err)
		}

		if payload["event"] != "contact.created" {
			t.Fatalf("expected event contact.created, got: %v", payload["event"])
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("receiver did not receive test webhook request in time")
	}
}
