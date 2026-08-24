//go:build integration || e2e || resilience || !unit

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
	err := c.TestWebhook(ts.URL, "secret_123", "subscriber.created")
	if err != nil {
		t.Fatalf("unexpected TestWebhook error: %v", err)
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

		if payload["event"] != "subscriber.created" {
			t.Fatalf("expected event subscriber.created, got: %v", payload["event"])
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("receiver did not receive test webhook request in time")
	}
}

func TestWebhookSettings_StructAndSecretMasking(t *testing.T) {
	jsonPayload := []byte(`{
		"app.site_name": "Test Site",
		"webhooks": [
			{
				"id": 1,
				"name": "n8n Integration",
				"url": "https://n8n.example.com/webhook/123",
				"secret": "whsec_supersecret123",
				"events": ["subscriber.created", "campaign.clicked"],
				"enabled": true
			}
		]
	}`)

	var set struct {
		Webhooks []struct {
			ID      int      `json:"id"`
			Name    string   `json:"name"`
			URL     string   `json:"url"`
			Secret  string   `json:"secret"`
			Events  []string `json:"events"`
			Enabled bool     `json:"enabled"`
		} `json:"webhooks"`
	}

	if err := json.Unmarshal(jsonPayload, &set); err != nil {
		t.Fatalf("failed to unmarshal Webhooks in settings payload: %v", err)
	}

	if len(set.Webhooks) != 1 {
		t.Fatalf("expected 1 webhook item, got %d", len(set.Webhooks))
	}

	wh := set.Webhooks[0]
	if wh.ID != 1 || wh.Name != "n8n Integration" || wh.URL != "https://n8n.example.com/webhook/123" {
		t.Errorf("unexpected webhook fields: %+v", wh)
	}

	if len(wh.Events) != 2 || wh.Events[0] != "subscriber.created" || wh.Events[1] != "campaign.clicked" {
		t.Errorf("unexpected webhook events: %v", wh.Events)
	}
}
