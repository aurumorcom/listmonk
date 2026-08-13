package waha

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/knadh/listmonk/models"
)

func TestFormatChatID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"+1 (555) 019-2834", "15550192834@c.us"},
		{"15550192834@c.us", "15550192834@c.us"},
		{"555-123-4567", "5551234567@c.us"},
	}

	for _, tt := range tests {
		res := formatChatID(tt.input)
		if res != tt.expected {
			t.Errorf("formatChatID(%s) = %s; want %s", tt.input, res, tt.expected)
		}
	}
}

func TestExtractPhone(t *testing.T) {
	attribs := models.JSON{"phone": "+15550192834"}
	res := extractPhone(attribs, "phone")
	if res != "+15550192834" {
		t.Errorf("extractPhone() = %s; want +15550192834", res)
	}

	missing := extractPhone(attribs, "nonexistent")
	if missing != "" {
		t.Errorf("extractPhone() for missing key = %s; want empty string", missing)
	}
}

func TestKeyboardLayout(t *testing.T) {
	qwerty := newKeyboardLayout("qwerty")
	if !qwerty.hasKey('a') || !qwerty.hasKey('q') {
		t.Errorf("expected QWERTY to contain 'a' and 'q'")
	}

	distAQ := qwerty.getDistance('a', 'q')
	if distAQ <= 0 || distAQ > 2.0 {
		t.Errorf("expected distance between 'a' and 'q' to be close (~1.0), got %f", distAQ)
	}

	distAP := qwerty.getDistance('a', 'p')
	if distAP < 4.0 {
		t.Errorf("expected distance between 'a' and 'p' to be far (>4.0), got %f", distAP)
	}

	azerty := newKeyboardLayout("azerty")
	if !azerty.hasKey('a') || !azerty.hasKey('z') {
		t.Errorf("expected AZERTY to contain 'a' and 'z'")
	}
}

func TestLinguisticClassifier(t *testing.T) {
	if getWordDifficulty("the") != "common" {
		t.Errorf("expected 'the' to be common word")
	}
	if getWordDifficulty("unquestionably") != "complex" {
		t.Errorf("expected 'unquestionably' to be complex word")
	}
	if getWordDifficulty("hello") != "normal" {
		t.Errorf("expected 'hello' to be normal word")
	}

	if !isCommonBigram('t', 'h') {
		t.Errorf("expected 'th' to be common bigram")
	}
}

func TestCalculateHumanTypingDelay(t *testing.T) {
	o := Options{
		TargetWPM:         60,
		WPMStd:            10.0,
		KeyboardLayout:    "qwerty",
		TypingMode:        "human",
		MaxTypingDelaySec: 5,
	}

	// Short text
	delayShort := calculateHumanTypingDelay([]byte("Hello"), o)
	if delayShort < 1*time.Second || delayShort > 5*time.Second {
		t.Errorf("calculateHumanTypingDelay(short) out of bounds: %v", delayShort)
	}

	// Long text capped by MaxTypingDelaySec
	delayLong := calculateHumanTypingDelay([]byte("This is a much longer text message that tests the maximum delay cap setting in the human typing simulation model."), o)
	if delayLong < 1*time.Second || delayLong > 5*time.Second {
		t.Errorf("calculateHumanTypingDelay(long) out of bounds: %v", delayLong)
	}
}

func TestMarkovTyperSimulation(t *testing.T) {
	typer := newMarkovTyper("The quick brown fox jumps over the lazy dog.", 60, 10, "qwerty")
	totalSec := typer.run()
	if totalSec <= 0 {
		t.Errorf("expected positive simulated time, got %f", totalSec)
	}
	if string(typer.state.currentText) != "The quick brown fox jumps over the lazy dog." {
		t.Errorf("expected final simulated text to match target text, got %s", string(typer.state.currentText))
	}
}

func TestWahaPushSequence(t *testing.T) {
	var mu sync.Mutex
	var calls []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.URL.Path)
		mu.Unlock()

		if r.Header.Get("X-Api-Key") != "test-api-key" {
			t.Errorf("expected X-Api-Key header 'test-api-key', got %s", r.Header.Get("X-Api-Key"))
		}

		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Errorf("expected non-empty body for %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result": true}`))
	}))
	defer server.Close()

	w, err := New(Options{
		Name:              "waha-test",
		RootURL:           server.URL,
		APIKey:            "test-api-key",
		Session:           "default",
		PhoneAttribute:    "phone",
		TargetWPM:         200, // fast speed to run unit test quickly
		MaxTypingDelaySec: 2,
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create WAHA messenger: %v", err)
	}

	attribs := models.JSON{"phone": "+15550192834"}
	msg := models.Message{
		Subscriber: models.Subscriber{
			UUID:    "test-uuid",
			Attribs: attribs,
		},
		Body: []byte("Hello from Listmonk!"),
	}

	if err := w.Push(msg); err != nil {
		t.Fatalf("w.Push failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(calls) < 3 {
		t.Fatalf("expected at least 3 API calls (startTyping, stopTyping, sendText), got %d: %v", len(calls), calls)
	}

	if calls[0] != "/api/startTyping" {
		t.Errorf("expected first call to be /api/startTyping, got %s", calls[0])
	}
	if calls[len(calls)-2] != "/api/stopTyping" {
		t.Errorf("expected second-to-last call to be /api/stopTyping, got %s", calls[len(calls)-2])
	}
	if calls[len(calls)-1] != "/api/sendText" {
		t.Errorf("expected last call to be /api/sendText, got %s", calls[len(calls)-1])
	}
}

func TestWahaSyncWebhook(t *testing.T) {
	var receivedPayload sessionConfigPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/sessions/default" {
			t.Errorf("expected path /api/sessions/default, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT method, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedPayload)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	w, err := New(Options{
		Name:    "waha-test",
		RootURL: server.URL,
		Session: "default",
	})
	if err != nil {
		t.Fatalf("failed to create WAHA messenger: %v", err)
	}

	if err := w.SyncWebhook("http://localhost:9000"); err != nil {
		t.Fatalf("SyncWebhook failed: %v", err)
	}

	if len(receivedPayload.Config.Webhooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(receivedPayload.Config.Webhooks))
	}

	wh := receivedPayload.Config.Webhooks[0]
	if wh.URL != "http://localhost:9000/api/webhooks/waha" {
		t.Errorf("expected webhook URL 'http://localhost:9000/api/webhooks/waha', got %s", wh.URL)
	}
	if len(wh.Events) != 2 || wh.Events[0] != "message.ack" || wh.Events[1] != "message" {
		t.Errorf("expected events ['message.ack', 'message'], got %v", wh.Events)
	}
}
