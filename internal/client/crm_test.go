package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knadh/listmonk/models"
)

func TestCRMClient_Singleton(t *testing.T) {
	cfg := models.CRMSettings{
		Enabled:   true,
		BaseURL:   "http://localhost:8000",
		APIKey:    "test_key",
		APISecret: "test_secret",
	}

	c1 := CRM(cfg)
	c2 := CRM(cfg)

	if c1 == nil {
		t.Fatal("expected non-nil CRM client singleton")
	}
	if c1 != c2 {
		t.Fatal("expected CRM() to return thread-safe singleton instance")
	}
}

func TestCRMClient_DeepResearch(t *testing.T) {
	var capturedAuth string
	var capturedPayload CRMDeepResearchPayload

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&capturedPayload)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer ts.Close()

	client := NewCRMClient(models.CRMSettings{
		Enabled:   true,
		BaseURL:   ts.URL,
		APIKey:    "my_key",
		APISecret: "my_secret",
	})

	sub := models.Subscriber{Email: "test@example.com"}
	sub.ID = 100
	payload := CRMDeepResearchPayload{
		CampaignID: 42,
		ListIDs:    []int{1, 2},
		Subscriber: sub,
		CampaignSubscriber: models.CampaignSubscriber{
			CampaignID:   42,
			SubscriberID: 100,
			Status:       "waiting",
		},
	}

	err := client.DeepResearch(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected error on DeepResearch: %v", err)
	}

	expectedAuth := "token my_key:my_secret"
	if capturedAuth != expectedAuth {
		t.Errorf("expected Authorization header %q, got %q", expectedAuth, capturedAuth)
	}

	if capturedPayload.CampaignID != 42 || capturedPayload.Subscriber.ID != 100 {
		t.Errorf("expected payload CampaignID=42, SubID=100, got %v", capturedPayload)
	}
}
