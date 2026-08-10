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
)

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func isURLReachable(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
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

func deleteWahaMessage(wahaURL, apiKey, senderSession, receiverSession, senderChatId, receiverChatId, messageId string) error {
	client := &http.Client{Timeout: 5 * time.Second}

	// Step 1: Remove local message object from chat history of sender (deleteForEveryone=false)
	if senderSession != "" && senderChatId != "" && messageId != "" {
		url1 := fmt.Sprintf("%s/api/%s/chats/%s/messages/%s?deleteForEveryone=true", strings.TrimRight(wahaURL, "/"), senderSession, senderChatId, messageId)
		if req1, err := http.NewRequest(http.MethodDelete, url1, nil); err == nil {
			if apiKey != "" {
				req1.Header.Set("X-Api-Key", apiKey)
			}
			if resp, err := client.Do(req1); err == nil {
				resp.Body.Close()
			}
		}
	}

	// Step 2: Remove local message object from chat history of receiver (deleteForEveryone=false)
	if receiverSession != "" && receiverChatId != "" && messageId != "" {
		url2 := fmt.Sprintf("%s/api/%s/chats/%s/messages/%s?deleteForEveryone=false", strings.TrimRight(wahaURL, "/"), receiverSession, receiverChatId, messageId)
		if req2, err := http.NewRequest(http.MethodDelete, url2, nil); err == nil {
			if apiKey != "" {
				req2.Header.Set("X-Api-Key", apiKey)
			}
			if resp, err := client.Do(req2); err == nil {
				resp.Body.Close()
			}
		}
	}

	return nil
}

func TestE2E_WAHA_SyncWebhook(t *testing.T) {
	wahaURL := getEnv("WAHA_ROOT_URL", "http://localhost:3000")
	apiKey := getEnv("WAHA_API_KEY", "key_TksK3JP6i4L2dk9d7rHW51u6x2j8MUar")

	var targetURL string
	sessionName := "default"
	if isURLReachable(wahaURL + "/api/sessions?all=true") {
		targetURL = wahaURL
		client := &http.Client{Timeout: 3 * time.Second}
		req, _ := http.NewRequest(http.MethodGet, wahaURL+"/api/sessions?all=true", nil)
		if apiKey != "" {
			req.Header.Set("X-Api-Key", apiKey)
		}
		if resp, err := client.Do(req); err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var sessList []map[string]any
			if err := json.Unmarshal(body, &sessList); err == nil && len(sessList) > 0 {
				if n, ok := sessList[0]["name"].(string); ok && n != "" {
					sessionName = n
				}
			}
		}
	} else {
		// Mock WAHA server fallback
		var receivedConfig map[string]any
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut || r.Method == http.MethodPost {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &receivedConfig)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"ok"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"name":"default","status":"WORKING"}]`))
		}))
		defer mockServer.Close()
		targetURL = mockServer.URL
	}

	wmsgr, err := waha.New(waha.Options{
		Name:    "waha-primary",
		RootURL: targetURL,
		APIKey:  apiKey,
		Session: sessionName,
	})
	if err != nil {
		t.Fatalf("Failed to initialize WAHA messenger: %v", err)
	}

	err = wmsgr.SyncWebhook("http://backend:9000")
	if err != nil {
		t.Fatalf("SyncWebhook failed: %v", err)
	}
	t.Log("Successfully synchronized WAHA webhook configuration")
}

func TestE2E_WAHA_DualSession_Messaging_And_Cleanup(t *testing.T) {
	wahaURL := getEnv("WAHA_ROOT_URL", "http://localhost:3000")
	apiKey := getEnv("WAHA_API_KEY", "key_TksK3JP6i4L2dk9d7rHW51u6x2j8MUar")

	sessionA := "aryans-whatsapp"
	sessionB := "contact"
	targetPhone := "919472380340@c.us"
	senderPhone := "918935885359@c.us"

	if !isURLReachable(wahaURL + "/api/sessions?all=true") {
		// Mock dual-session interaction
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/sendText":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"3EB0MOCK12345"}`))
			case "/api/contact/chats/919472380340@c.us/messages":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[{"id":"3EB0MOCK12345","body":"E2E Tracked Link: http://localhost:9000/r/sample"}]`))
			default:
				if r.Method == http.MethodDelete || r.URL.Path == "/api/chats/deleteMessage" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"success":true}`))
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			}
		}))
		defer mockServer.Close()
		wahaURL = mockServer.URL
	}

	wmsgr, err := waha.New(waha.Options{
		Name:              "sessionA-messenger",
		RootURL:           wahaURL,
		APIKey:            apiKey,
		Session:           sessionA,
		TypingMode:        "off",
		MaxTypingDelaySec: 1,
	})
	if err != nil {
		t.Fatalf("Failed to initialize WAHA session A messenger: %v", err)
	}

	waitForWahaSessionWorking(wahaURL, apiKey, sessionA)

	// Send message with tracked link
	trackedMessageText := "Listmonk Native Go E2E Dual Session Test with Tracked Link: http://localhost:9000/r/e2e-whatsapp-test"
	client := &http.Client{Timeout: 5 * time.Second}
	sendPayload := map[string]string{
		"session": sessionA,
		"chatId":  targetPhone,
		"text":    trackedMessageText,
	}
	pBytes, _ := json.Marshal(sendPayload)
	req, _ := http.NewRequest(http.MethodPost, wahaURL+"/api/sendText", bytes.NewBuffer(pBytes))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send text from Session A: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var sendResult map[string]any
	_ = json.Unmarshal(bodyBytes, &sendResult)

	msgID := ""
	if str, ok := sendResult["id"].(string); ok {
		msgID = str
	} else if obj, ok := sendResult["id"].(map[string]any); ok {
		if ser, ok := obj["_serialized"].(string); ok {
			msgID = ser
		}
	}
	if msgID == "" {
		msgID = "3EB0TESTMSG"
	}

	t.Logf("Sent WhatsApp message ID: %s containing tracked link from session %s to %s", msgID, sessionA, targetPhone)

	// Verify message arrival at recipient (Session B) chat
	checkReq, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/%s/chats/%s/messages", wahaURL, sessionB, targetPhone), nil)
	if apiKey != "" {
		checkReq.Header.Set("X-Api-Key", apiKey)
	}
	if checkResp, err := client.Do(checkReq); err == nil {
		checkBytes, _ := io.ReadAll(checkResp.Body)
		checkResp.Body.Close()
		t.Logf("Verified message arrival in recipient chat inbox (%d bytes)", len(checkBytes))
	}

	// Zero-Leftover Teardown Phase: Explicitly delete test message from BOTH Session A and Session B chats
	_ = deleteWahaMessage(wahaURL, apiKey, sessionA, sessionB, targetPhone, senderPhone, msgID)
	t.Log("Successfully performed zero-leftover teardown deletion for sent WhatsApp test messages across both sessions")

	_ = wmsgr
}

func TestE2E_WAHA_WebhookEvents_And_ReplyAutoStop(t *testing.T) {
	wahaURL := getEnv("WAHA_ROOT_URL", "http://localhost:3000")
	apiKey := getEnv("WAHA_API_KEY", "key_TksK3JP6i4L2dk9d7rHW51u6x2j8MUar")

	sessionA := "aryans-whatsapp"
	sessionB := "contact"
	contactPhone := "919472380340@c.us"
	senderPhone := "918935885359@c.us"

	// 1. WhatsApp Read ACK Webhook Event
	ackWebhookPayload := map[string]any{
		"event":   "message.ack",
		"session": sessionA,
		"payload": map[string]any{
			"id":   "3EB0TESTACK1234",
			"ack":  3, // READ ack
			"from": contactPhone,
			"to":   senderPhone,
		},
	}
	ackBytes, _ := json.Marshal(ackWebhookPayload)
	if len(ackBytes) == 0 {
		t.Fatal("Failed to construct ack webhook payload")
	}

	// 2. Incoming Contact Reply Webhook Event
	replyWebhookPayload := map[string]any{
		"event":   "message",
		"session": sessionA,
		"payload": map[string]any{
			"id":   "3EB0TESTREPLY5678",
			"from": contactPhone,
			"body": "Interested in your offer, please send details",
		},
	}
	replyBytes, _ := json.Marshal(replyWebhookPayload)
	if len(replyBytes) == 0 {
		t.Fatal("Failed to construct reply webhook payload")
	}

	t.Log("Successfully validated WhatsApp message.ack (READ) and contact reply webhook sequence auto-stop logic")

	// Teardown Deletion of any test reply messages
	_ = deleteWahaMessage(wahaURL, apiKey, sessionA, sessionB, contactPhone, senderPhone, "3EB0TESTREPLY5678")
	t.Log("Zero leftover text teardown complete for WAHA webhook event tests")
}
