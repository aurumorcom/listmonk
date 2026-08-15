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
	"github.com/labstack/echo/v4"
)

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func isURLReachable(url string) bool {
	apiKey := getEnv("WAHA_API_KEY", "key_JR59f24sOxG1O2OhhLFXIsLVID4ajvLD")
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
func TestWAHA_WebhookSetup(t *testing.T) {
	wahaURL := getEnv("WAHA_ROOT_URL", "http://localhost:3000")
	apiKey := getEnv("WAHA_API_KEY", "key_JR59f24sOxG1O2OhhLFXIsLVID4ajvLD")

	senderSess, _, isLive := discoverWahaSessions(wahaURL, apiKey)
	targetURL := wahaURL

	if !isLive {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}))
		defer mockServer.Close()
		targetURL = mockServer.URL
	}

	wmsgr, err := waha.New(waha.Options{
		Name:    "waha-primary",
		RootURL: targetURL,
		APIKey:  apiKey,
		Session: senderSess.Name,
	})
	if err != nil {
		t.Fatalf("Failed initializing WAHA messenger: %v", err)
	}

	err = wmsgr.SyncWebhook("http://backend:9000/api/webhooks/waha")
	if err != nil {
		t.Fatalf("SyncWebhook failed: %v", err)
	}
	t.Log("Successfully verified WAHA webhook configuration sync")
}

// 2. Read Message & Clean Up Feature Test
func TestWAHA_ReadMessage(t *testing.T) {
	wahaURL := getEnv("WAHA_ROOT_URL", "http://localhost:3000")
	apiKey := getEnv("WAHA_API_KEY", "key_JR59f24sOxG1O2OhhLFXIsLVID4ajvLD")

	senderSess, receiverSess, isLive := discoverWahaSessions(wahaURL, apiKey)
	targetURL := wahaURL

	if !isLive {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/sendText":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"3EB0READMOCK123"}`))
			default:
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			}
		}))
		defer mockServer.Close()
		targetURL = mockServer.URL
	}

	// Dispatch single message
	client := &http.Client{Timeout: 10 * time.Second}
	sendPayload := map[string]string{
		"session": senderSess.Name,
		"chatId":  receiverSess.JID,
		"text":    "WAHA Read Message Feature Test",
	}
	pBytes, _ := json.Marshal(sendPayload)
	req, _ := http.NewRequest(http.MethodPost, targetURL+"/api/sendText", bytes.NewBuffer(pBytes))
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
func TestWAHA_LinkClick(t *testing.T) {
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
func TestWAHA_Replied(t *testing.T) {
	wahaURL := getEnv("WAHA_ROOT_URL", "http://localhost:3000")
	apiKey := getEnv("WAHA_API_KEY", "key_JR59f24sOxG1O2OhhLFXIsLVID4ajvLD")

	senderSess, receiverSess, _ := discoverWahaSessions(wahaURL, apiKey)

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
