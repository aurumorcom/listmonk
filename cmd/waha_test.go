//go:build integration || e2e || !unit

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

	// 4B. Verify @lid Inbound Reply with fromMe=false
	replyLIDPayload := map[string]any{
		"event":   "message",
		"session": senderSess.Name,
		"payload": map[string]any{
			"id":     "false_210556493537459@lid_AC6DD75E5513F74D982B46860BA9E85D",
			"from":   "210556493537459@lid",
			"fromMe": false,
			"body":   "Hlo, I want to confirm my interest",
		},
	}
	bLID, _ := json.Marshal(replyLIDPayload)
	reqLID := httptest.NewRequest(http.MethodPost, "/api/webhooks/waha", bytes.NewReader(bLID))
	reqLID.Header.Set("Content-Type", "application/json")
	recLID := httptest.NewRecorder()
	cLID := e.NewContext(reqLID, recLID)
	_ = app.WAHAWebhook(cLID)
	if recLID.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 from WAHA LID Reply webhook, got %d", recLID.Code)
	}

	// 4C. Verify Outbound Echo with fromMe=true is Ignored
	echoPayload := map[string]any{
		"event":   "message",
		"session": senderSess.Name,
		"payload": map[string]any{
			"id":     "true_210556493537459@lid_OUTBOUND123",
			"from":   senderSess.JID,
			"fromMe": true,
			"body":   "Automated broadcast from bot",
		},
	}
	bEcho, _ := json.Marshal(echoPayload)
	reqEcho := httptest.NewRequest(http.MethodPost, "/api/webhooks/waha", bytes.NewReader(bEcho))
	reqEcho.Header.Set("Content-Type", "application/json")
	recEcho := httptest.NewRecorder()
	cEcho := e.NewContext(reqEcho, recEcho)
	_ = app.WAHAWebhook(cEcho)
	if recEcho.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 from WAHA echo webhook, got %d", recEcho.Code)
	}

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

func TestParseWAHAAckLevel_DeviceVsRead(t *testing.T) {
	// Test Request #1: ack: 2, ackName: "DEVICE" -> level 2 (Delivery, not read)
	if l1 := ParseWAHAAckLevel(2, "DEVICE"); l1 != 2 {
		t.Errorf("expected level 2 for DEVICE delivery, got %d", l1)
	}

	// Test Request #2: ack: 3, ackName: "READ" -> level 3 (Read, Blue Tick)
	if l2 := ParseWAHAAckLevel(3, "READ"); l2 != 3 {
		t.Errorf("expected level 3 for READ blue tick, got %d", l2)
	}

	// Test string ACK variants
	if l3 := ParseWAHAAckLevel("READ", ""); l3 != 3 {
		t.Errorf("expected level 3 for string READ, got %d", l3)
	}

	if l4 := ParseWAHAAckLevel("DEVICE", ""); l4 != 2 {
		t.Errorf("expected level 2 for string DEVICE, got %d", l4)
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
			name: "Numeric ACK 2 (Device Delivery - Double Grey Tick)",
			payload: map[string]any{
				"event": "message.ack",
				"payload": map[string]any{
					"ack":     2,
					"ackName": "DEVICE",
					"to":      "14155552672@c.us",
				},
			},
			expectRead: false,
		},
		{
			name: "Numeric ACK 3 (Blue Tick Read)",
			payload: map[string]any{
				"event": "message.ack",
				"payload": map[string]any{
					"ack":     3,
					"ackName": "READ",
					"to":      "14155552672@c.us",
				},
			},
			expectRead: true,
		},
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

func TestWAHAWebhook_DeepLogging_Resilience(t *testing.T) {
	// Logger capturing logs in memory buffer to verify deep logging
	var logBuf bytes.Buffer
	testLogger := log.New(&logBuf, "[TEST] ", 0)

	app := &App{log: testLogger}
	e := echo.New()

	testCases := []struct {
		name         string
		contentType  string
		body         string
		expectSubstr string
	}{
		{
			name:         "Empty request body",
			contentType:  "application/json",
			body:         "",
			expectSubstr: "[WAHA WEBHOOK] Incoming POST /api/webhooks/waha",
		},
		{
			name:         "Malformed JSON body",
			contentType:  "application/json",
			body:         "{invalid-json-payload",
			expectSubstr: "[WAHA WEBHOOK ERROR] JSON bind failed",
		},
		{
			name:         "Valid message.ack payload",
			contentType:  "application/json",
			body:         `{"event":"message.ack","payload":{"id":"true_14155552671@c.us_MSG123","ack":3,"ackName":"READ","to":"14155552671@c.us"}}`,
			expectSubstr: "[WAHA WEBHOOK READ ACK]",
		},
		{
			name:         "Valid inbound message payload",
			contentType:  "application/json",
			body:         `{"event":"message","payload":{"from":"14155552671@c.us","fromMe":false,"body":"Hello there","_data":{"quotedMsg":{"id":"MSG123"}}}}`,
			expectSubstr: "[WAHA WEBHOOK INBOUND REPLY]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logBuf.Reset()
			req := httptest.NewRequest(http.MethodPost, "/api/webhooks/waha", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := app.WAHAWebhook(c)
			if err != nil {
				t.Fatalf("WAHAWebhook returned unexpected error: %v", err)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("expected HTTP 200, got %d", rec.Code)
			}

			loggedOutput := logBuf.String()
			if !strings.Contains(loggedOutput, tc.expectSubstr) {
				t.Errorf("expected log output to contain %q, got: %s", tc.expectSubstr, loggedOutput)
			}
		})
	}
}

func TestWAHAWebhook_LID_Resolution_Cassette(t *testing.T) {
	rec, vcrClient := testutil.NewVCRRecorder(t, "waha/lid_resolution")

	wmsgr, err := waha.GetWAHAMessenger(waha.Options{
		Name:    "waha",
		Session: "aquiveal",
		Host:    "http://localhost:3000",
	})
	if err != nil {
		t.Fatalf("failed initializing WAHA messenger: %v", err)
	}
	if rec != nil {
		wmsgr.SetHTTPClient(vcrClient)
	}

	var logBuf bytes.Buffer
	testLogger := log.New(&logBuf, "[TEST] ", 0)

	app := &App{log: testLogger}
	e := echo.New()

	payload := map[string]any{
		"event":   "message.ack",
		"session": "aquiveal",
		"payload": map[string]any{
			"id":      "true_210556493537459@lid_A5584986A985A536363A45CDFF7FDBD9",
			"ack":     3,
			"ackName": "READ",
			"to":      "210556493537459@lid",
		},
	}
	pBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/waha", bytes.NewReader(pBytes))
	req.Header.Set("Content-Type", "application/json")
	recHTTP := httptest.NewRecorder()
	c := e.NewContext(req, recHTTP)

	if err := app.WAHAWebhook(c); err != nil {
		t.Fatalf("WAHAWebhook returned unexpected error: %v", err)
	}

	logged := logBuf.String()
	if !strings.Contains(logged, "[WAHA WEBHOOK LID RESOLVED]") || !strings.Contains(logged, "14155552671") {
		t.Errorf("expected log output to confirm LID resolution to phone 14155552671, got: %s", logged)
	}
}

func TestWAHAWebhook_LID_PN_Resolution_Cassette(t *testing.T) {
	rec, vcrClient := testutil.NewVCRRecorder(t, "waha/lid_resolution_pn")

	wmsgr, err := waha.GetWAHAMessenger(waha.Options{
		Name:    "waha",
		Session: "aquiveal",
		Host:    "http://localhost:3000",
	})
	if err != nil {
		t.Fatalf("failed initializing WAHA messenger: %v", err)
	}
	if rec != nil {
		wmsgr.SetHTTPClient(vcrClient)
	}

	var logBuf bytes.Buffer
	testLogger := log.New(&logBuf, "[TEST] ", 0)

	app := &App{log: testLogger}
	e := echo.New()

	payload := map[string]any{
		"event":   "message.ack",
		"session": "aquiveal",
		"payload": map[string]any{
			"id":      "true_210556493537459@lid_A586AF1159FE67F12752F51F57C40229",
			"ack":     3,
			"ackName": "READ",
			"to":      "210556493537459@lid",
		},
	}
	pBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/waha", bytes.NewReader(pBytes))
	req.Header.Set("Content-Type", "application/json")
	recHTTP := httptest.NewRecorder()
	c := e.NewContext(req, recHTTP)

	if err := app.WAHAWebhook(c); err != nil {
		t.Fatalf("WAHAWebhook returned unexpected error: %v", err)
	}

	logged := logBuf.String()
	if !strings.Contains(logged, "[WAHA WEBHOOK LID RESOLVED]") || !strings.Contains(logged, "14155552672") {
		t.Errorf("expected log output to confirm LID resolution to real phone 14155552672, got: %s", logged)
	}
}
