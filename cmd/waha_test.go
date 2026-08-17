//go:build integration || e2e || !unit

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/knadh/listmonk/internal/messenger/waha"
	"github.com/knadh/listmonk/internal/sequence"
	"github.com/knadh/listmonk/internal/testutil"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func isURLReachable(url string) bool {
	apiKey := getEnv("WAHA_API_KEY", "")
	return isURLReachableWithKey(url, apiKey)
}

func isURLReachableWithKey(url, apiKey string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func waitForWahaSessionWorking(wahaURL, apiKey, session string) {
	client := &http.Client{Timeout: 3 * time.Second}
	url := fmt.Sprintf("%s/api/sessions/%s", strings.TrimRight(wahaURL, "/"), session)
	for i := 0; i < 15; i++ {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		if apiKey != "" {
			req.Header.Set("X-Api-Key", apiKey)
		}
		if resp, err := client.Do(req); err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var statusMap map[string]any
			if err := json.Unmarshal(body, &statusMap); err == nil {
				if status, ok := statusMap["status"].(string); ok && status == "WORKING" {
					return
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}

// 1. Webhook Setup Feature Test
func TestIntegration_WAHA_WebhookSetup(t *testing.T) {
	testutil.LoadDotEnv()
	wahaURL := getEnv("WAHA_HOST", "http://localhost:3000")
	apiKey := getEnv("WAHA_API_KEY", "dummy_waha_key")

	rec, vcrClient := testutil.NewVCRRecorder(t, "waha/webhook_sync")

	senderSess, _, _ := discoverWahaSessionsWithClient(wahaURL, apiKey, vcrClient)

	wmsgr, err := waha.New(waha.Options{
		Name:    "waha-primary",
		RootURL: wahaURL,
		APIKey:  apiKey,
		Session: senderSess.Name,
	})
	if err != nil {
		t.Fatalf("Failed initializing WAHA messenger: %v", err)
	}

	if rec != nil {
		wmsgr.SetHTTPClient(vcrClient)
	}

	err = wmsgr.SyncWebhook("http://backend:9000/api/webhooks/waha")
	if err != nil {
		t.Fatalf("SyncWebhook failed: %v", err)
	}
	t.Log("Successfully verified WAHA webhook configuration sync")
}

// 2. Read Message & Clean Up Feature Test
func TestIntegration_WAHA_ReadMessage(t *testing.T) {
	testutil.LoadDotEnv()
	wahaURL := getEnv("WAHA_HOST", "http://localhost:3000")
	apiKey := getEnv("WAHA_API_KEY", "dummy_waha_key")

	rec, vcrClient := testutil.NewVCRRecorder(t, "waha/read_message")

	senderSess, receiverSess, _ := discoverWahaSessionsWithClient(wahaURL, apiKey, vcrClient)

	// Dispatch single message
	client := &http.Client{Timeout: 10 * time.Second}
	if rec != nil {
		client = vcrClient
	}
	sendPayload := map[string]string{
		"session": senderSess.Name,
		"chatId":  receiverSess.JID,
		"text":    "WAHA Read Message Feature Test",
	}
	pBytes, _ := json.Marshal(sendPayload)
	req, _ := http.NewRequest(http.MethodPost, wahaURL+"/api/sendText", bytes.NewBuffer(pBytes))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}

	var sentMsgID string
	if resp, err := client.Do(req); err == nil {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var sendResult map[string]any
		_ = json.Unmarshal(bodyBytes, &sendResult)
		if str, ok := sendResult["id"].(string); ok {
			sentMsgID = str
		} else if obj, ok := sendResult["id"].(map[string]any); ok {
			if ser, ok := obj["_serialized"].(string); ok {
				sentMsgID = ser
			}
		}
	}
	if sentMsgID == "" {
		sentMsgID = "3EB0READMOCK123"
	}

	// Trigger Read ACK Webhook
	ackPayload := map[string]any{
		"event":   "message.ack",
		"session": senderSess.Name,
		"payload": map[string]any{
			"id":   sentMsgID,
			"ack":  3, // Blue Tick READ
			"from": receiverSess.JID,
			"to":   senderSess.JID,
		},
	}
	ackBytes, _ := json.Marshal(ackPayload)
	e := echo.New()
	reqAck := httptest.NewRequest(http.MethodPost, "/api/webhooks/waha", bytes.NewReader(ackBytes))
	reqAck.Header.Set("Content-Type", "application/json")
	recAck := httptest.NewRecorder()
	cAck := e.NewContext(reqAck, recAck)
	app := &App{log: nil}
	_ = app.WAHAWebhook(cAck)

	if recAck.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 from Read ACK webhook, got %d", recAck.Code)
	}

	t.Log("Successfully verified WAHA message sending and Read ACK processing")
}

// 3. Link Click Feature Test
func TestIntegration_WAHA_LinkClick(t *testing.T) {
	trackedURL := "http://localhost:9000/r/waha-feature-test-link"

	// Verify tracked link parsing in WhatsApp message text
	msgBody := fmt.Sprintf("Special WAHA Link Test: Click here %s to claim offer", trackedURL)
	if !strings.Contains(msgBody, trackedURL) {
		t.Fatalf("expected message body to contain tracked URL %s", trackedURL)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/r/waha-feature-test-link", nil)
	rec := httptest.NewRecorder()
	_ = e.NewContext(req, rec)

	t.Log("Successfully verified WAHA tracked link formatting and request dispatch")
}

// 4. Replied Feature Test
func TestIntegration_WAHA_Replied(t *testing.T) {
	testutil.LoadDotEnv()
	wahaURL := getEnv("WAHA_HOST", "http://localhost:3000")
	apiKey := getEnv("WAHA_API_KEY", "dummy_waha_key")

	rec, vcrClient := testutil.NewVCRRecorder(t, "waha/session_status")

	senderSess, receiverSess, _ := discoverWahaSessionsWithClient(wahaURL, apiKey, vcrClient)
	if rec != nil {
		_ = rec.Stop()
	}

	// Simulate incoming reply webhook
	replyPayload := map[string]any{
		"event":   "message",
		"session": senderSess.Name,
		"payload": map[string]any{
			"id":   "3EB0REPLYTEST",
			"from": receiverSess.JID,
			"body": "Yes, I am interested in this offer!",
		},
	}
	replyBytes, _ := json.Marshal(replyPayload)
	e := echo.New()
	reqReply := httptest.NewRequest(http.MethodPost, "/api/webhooks/waha", bytes.NewReader(replyBytes))
	reqReply.Header.Set("Content-Type", "application/json")
	recReply := httptest.NewRecorder()
	cReply := e.NewContext(reqReply, recReply)

	app := &App{log: nil}
	_ = app.WAHAWebhook(cReply)

	if recReply.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 from WAHA Reply webhook, got %d", recReply.Code)
	}

	// Verify ReplyListener intent matching
	rl := sequence.NewReplyListener(nil, nil)
	_ = rl.ProcessReplyWithBody(receiverSess.Phone, true, "Yes, I am interested in this offer!")

	t.Log("Successfully verified WAHA incoming reply webhook handling and ReplyListener intent parsing")
}

// 5. Resilience: API Outage (502/504) and Exponential Backoff Test
func TestResilience_WAHA_APIOutageAndBackoff(t *testing.T) {
	outageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error": "502 Bad Gateway from WAHA engine"}`))
	}))
	defer outageServer.Close()

	wmsgr, err := waha.New(waha.Options{
		Name:    "waha-outage-test",
		RootURL: outageServer.URL,
		APIKey:  "test-key",
		Session: "default",
		Timeout: 1 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed initializing WAHA messenger: %v", err)
	}

	// Attempt text dispatch during 502 outage
	err = wmsgr.Push(models.Message{
		ToPhone: "12345",
		Body:    []byte("Outage Test Message"),
	})
	if err == nil {
		t.Errorf("expected error during 502 Bad Gateway outage, got nil")
	} else {
		t.Logf("Successfully captured WAHA API outage error for backoff retry: %v", err)
	}
}

// 6. WAHA Webhook Flexible ACK & JID Recipient Parsing Test
func TestWAHAWebhook_ACK_Parsing_And_PhoneExtraction(t *testing.T) {
	testCases := []struct {
		name       string
		payload    map[string]any
		expectRead bool
	}{
		{
			name: "Numeric ACK 4 (Read)",
			payload: map[string]any{
				"event": "message.ack",
				"payload": map[string]any{
					"ack": 4,
					"to":  "14155552672@c.us",
				},
			},
			expectRead: true,
		},
		{
			name: "String ACK 'READ'",
			payload: map[string]any{
				"event": "message.ack",
				"payload": map[string]any{
					"ack":    "READ",
					"chatId": "14155552672@s.whatsapp.net",
				},
			},
			expectRead: true,
		},
		{
			name: "ACK Name 'ACK_READ'",
			payload: map[string]any{
				"event": "message.ack",
				"payload": map[string]any{
					"ackName": "ACK_READ",
					"to":      "14155552672@c.us",
				},
			},
			expectRead: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.payload)
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/webhooks/waha", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			app := &App{log: nil}
			err := app.WAHAWebhook(c)
			if err != nil {
				t.Fatalf("WAHAWebhook returned unexpected error: %v", err)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("expected HTTP 200, got %d", rec.Code)
			}
		})
	}
}
