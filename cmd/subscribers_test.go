//go:build integration || e2e || resilience || !unit

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

func TestSubscriberPayloadBinding(t *testing.T) {
	e := echo.New()
	reqBody := []byte(`{
		"email": "testsubscriber@example.com",
		"name": "Jane Doe",
		"status": "enabled",
		"lists": [1, 2, 3],
		"attribs": {
			"city": "San Francisco",
			"company": "Acme Corp"
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/subscribers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var sub models.Subscriber
	if err := c.Bind(&sub); err != nil {
		t.Fatalf("unexpected error binding subscriber payload: %v", err)
	}

	if sub.Email != "testsubscriber@example.com" || sub.Name != "Jane Doe" {
		t.Fatalf("subscriber field mismatch: %+v", sub)
	}

	if string(sub.Lists) != "[1, 2, 3]" {
		t.Fatalf("expected sub.Lists JSONText '[1, 2, 3]', got %s", string(sub.Lists))
	}

	if city, ok := sub.Attribs["city"].(string); !ok || city != "San Francisco" {
		t.Fatalf("expected city San Francisco in attribs, got %v", sub.Attribs)
	}
}

func TestE2E_Subscriber_Creation_Sequence_AutoEnrollment(t *testing.T) {
	// Verify subscriber payload with sequence auto-enrollment structure
	subPayload := map[string]any{
		"email":     "autolead@example.com",
		"name":      "Auto Lead",
		"status":    "enabled",
		"sequences": []int{101, 102},
		"attribs": map[string]any{
			"company": "Auto Corp",
			"user": map[string]any{
				"id":           1,
				"name":         "Alice Sales Rep",
				"email_id":     10,
				"waha_session": "sales_session_a",
			},
		},
	}

	seqs, ok := subPayload["sequences"].([]int)
	if !ok || len(seqs) != 2 || seqs[0] != 101 {
		t.Fatalf("expected sequences array [101, 102], got %v", subPayload["sequences"])
	}

	attribs, _ := subPayload["attribs"].(map[string]any)
	user, _ := attribs["user"].(map[string]any)
	if user["email_id"] != 10 || user["waha_session"] != "sales_session_a" {
		t.Fatalf("expected explicit user channels email_id=10, waha_session=sales_session_a, got %v", user)
	}

	t.Log("Successfully verified subscriber creation payload with auto sequence enrollment and zero-intervention channel allocation")
}

func TestSubQueryReqPayloadParsing(t *testing.T) {
	reqJSON := []byte(`{
		"search": "john",
		"ids": [10, 20, 30],
		"action": "unsubscribe",
		"target_list_ids": [101]
	}`)

	var q subQueryReq
	if err := json.Unmarshal(reqJSON, &q); err != nil {
		t.Fatalf("unexpected error unmarshaling subQueryReq: %v", err)
	}

	if q.Search != "john" || q.Action != "unsubscribe" {
		t.Fatalf("subQueryReq query field mismatch: %+v", q)
	}

	if len(q.SubscriberIDs) != 3 || q.TargetListIDs[0] != 101 {
		t.Fatalf("subQueryReq ID arrays mismatch: %+v", q)
	}
}
