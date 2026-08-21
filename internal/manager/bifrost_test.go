//go:build unit || !integration

package manager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/testutil"
	"github.com/knadh/listmonk/models"
)

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func TestBifrostThreadSafety(t *testing.T) {
	cfg := BifrostConfig{
		APIKey:   "test_key",
		Endpoint: "http://localhost:8080",
		Model:    "test_model",
	}

	const goroutines = 50
	var wg sync.WaitGroup
	instances := make([]*BifrostClient, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			instances[idx] = Bifrost(cfg)
		}(i)
	}
	wg.Wait()

	firstInst := instances[0]
	if firstInst == nil {
		t.Fatalf("expected non-nil BifrostClient instance")
	}

	for i := 1; i < goroutines; i++ {
		if instances[i] != firstInst {
			t.Fatalf("expected all instances to point to same singleton pointer, instance[%d] differs", i)
		}
	}
}

func TestIntegration_Bifrost_LiveAI_Completion(t *testing.T) {
	testutil.LoadDotEnv()
	endpoint := getEnv("BIFROST_ENDPOINT", "https://litellm.aurumor.com")
	apiKey := getEnv("BIFROST_API_KEY", "dummy_key_for_vcr_replay")

	cfg := BifrostConfig{
		APIKey:   apiKey,
		Endpoint: endpoint,
		Model:    getEnv("BIFROST_MODEL", "gemini-3.5-flash-lite"),
		Timeout:  15 * time.Second,
	}

	client := NewBifrostClient(cfg)

	rec, vcrClient := testutil.NewVCRRecorder(t, "bifrost/prompt_completion")
	if rec != nil {
		client.SetHTTPClient(vcrClient)
	}

	ctx, cancel := client.TimeoutContext()
	defer cancel()

	systemPrompt := "You are a professional B2B outreach manager writing compelling outreach emails."
	userPrompt := "Write a quick 2-sentence introduction email to Alice at Acme Corp."

	out, err := client.GeneratePromptWithFormat(ctx, systemPrompt, userPrompt, EmailResponseFormat())
	if err != nil {
		t.Fatalf("Bifrost Live AI prompt completion failed: %v", err)
	}

	if strings.TrimSpace(out) == "" {
		t.Errorf("expected non-empty AI generated text, got empty")
	}

	t.Logf("Generated AI Response:\n%s", out)
}

func TestExtractTemplateScope(t *testing.T) {
	attribsMap := models.JSON{
		"context": map[string]any{"company": "Acme Inc", "industry": "Software"},
		"user":    map[string]any{"name": "Alice Sender", "title": "Account Executive"},
	}

	sub := models.Subscriber{
		Base:    models.Base{ID: 101},
		Email:   "test@example.com",
		Name:    "Bob Contact",
		Attribs: attribsMap,
	}

	scope := ExtractTemplateScope(sub)

	if subObj, ok := scope["Subscriber"].(models.Subscriber); !ok || subObj.ID != 101 {
		t.Errorf("expected Subscriber ID 101, got %v", scope["Subscriber"])
	}

	if contactObj, ok := scope["Contact"].(models.Subscriber); !ok || contactObj.ID != 101 {
		t.Errorf("expected Contact ID 101, got %v", scope["Contact"])
	}

	ctxMap, ok := scope["Context"].(map[string]any)
	if !ok || ctxMap["company"] != "Acme Inc" {
		t.Errorf("expected Context company 'Acme Inc', got %v", scope["Context"])
	}

	userMap, ok := scope["User"].(map[string]any)
	if !ok || userMap["name"] != "Alice Sender" {
		t.Errorf("expected User name 'Alice Sender', got %v", scope["User"])
	}
}

func TestCleanJSONResponse(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"```json\n{\"subject\":\"Hello\",\"content\":\"World\"}\n```", "{\"subject\":\"Hello\",\"content\":\"World\"}"},
		{"```JSON\n{\"message\":\"Hi\"}\n```", "{\"message\":\"Hi\"}"},
		{"```\n{\"content\":\"Plain\"}\n```", "{\"content\":\"Plain\"}"},
		{"{\"subject\":\"Direct\"}", "{\"subject\":\"Direct\"}"},
	}

	for _, tt := range tests {
		got := CleanJSONResponse(tt.input)
		if got != tt.expected {
			t.Errorf("CleanJSONResponse(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestResolveSignature(t *testing.T) {
	// Test Case 1: Sequence/Enrollment Signature (Tier 1) overrides all
	sub1 := models.Subscriber{
		Attribs: models.JSON{
			"sequence_signature": "<p>Sequence Sig</p>",
		},
	}
	email1 := &models.Email{Signature: "<p>Email Sig</p>"}
	user1 := &auth.User{Signature: "<p>User Sig</p>"}
	sig1 := ResolveSignatureAdvanced(SignatureOpts{
		Subscriber: sub1,
		Email:      email1,
		User:       user1,
		GlobalSig:  "<p>Global Sig</p>",
	})
	if sig1 != "<p>Sequence Sig</p>" {
		t.Errorf("expected Tier 1 Sequence Sig, got %q", sig1)
	}

	// Test Case 2: Email Channel Signature (Tier 2) overrides User Sig & Global Sig
	sub2 := models.Subscriber{Attribs: models.JSON{}}
	email2 := &models.Email{Signature: "<p>Email Sig</p>"}
	user2 := &auth.User{Signature: "<p>User Sig</p>"}
	sig2 := ResolveSignatureAdvanced(SignatureOpts{
		Subscriber: sub2,
		Email:      email2,
		User:       user2,
		GlobalSig:  "<p>Global Sig</p>",
	})
	if sig2 != "<p>Email Sig</p>" {
		t.Errorf("expected Tier 2 Email Sig, got %q", sig2)
	}

	// Test Case 3: WhatsApp Messenger Signature (Tier 2) overrides User Sig & Global Sig
	waha3 := &models.WAHASettings{Signature: "<p>WhatsApp Sig</p>"}
	sig3 := ResolveSignatureAdvanced(SignatureOpts{
		Subscriber:   sub2,
		WAHASettings: waha3,
		User:         user2,
		GlobalSig:    "<p>Global Sig</p>",
	})
	if sig3 != "<p>WhatsApp Sig</p>" {
		t.Errorf("expected Tier 2 WhatsApp Sig, got %q", sig3)
	}

	// Test Case 4: User Signature (Tier 3) overrides Global Sig when no Tier 1 or Tier 2
	user4 := &auth.User{Signature: "<p>User Sig</p>"}
	sig4 := ResolveSignatureAdvanced(SignatureOpts{
		Subscriber: sub2,
		User:       user4,
		GlobalSig:  "<p>Global Sig</p>",
	})
	if sig4 != "<p>User Sig</p>" {
		t.Errorf("expected Tier 3 User Sig, got %q", sig4)
	}

	// Test Case 5: Fallback to Global Sig when Tiers 1-3 empty
	sig5 := ResolveSignatureAdvanced(SignatureOpts{
		Subscriber: sub2,
		GlobalSig:  "<p>Global Sig</p>",
	})
	if sig5 != "<p>Global Sig</p>" {
		t.Errorf("expected Global Sig fallback, got %q", sig5)
	}
}

func TestExtractTemplateScope_MultiPatternStepHistory(t *testing.T) {
	history := []map[string]any{
		{
			"step_number": 1,
			"messenger":   "email",
			"subject":     "Intro Subject",
			"content":     "Intro Body",
		},
		{
			"step_number": 2,
			"messenger":   "waha",
			"message":     "WhatsApp Message Text",
		},
	}

	sub := models.Subscriber{
		Base: models.Base{ID: 202},
		Attribs: models.JSON{
			"sequence_history": history,
		},
	}

	scope := ExtractTemplateScope(sub)

	// Pattern 1: scope["Steps"]["Step1"]
	stepsMap, ok := scope["Steps"].(map[string]any)
	if !ok {
		t.Fatalf("expected scope[\"Steps\"] map")
	}
	step1Obj, ok := stepsMap["Step1"].(map[string]any)
	if !ok || step1Obj["subject"] != "Intro Subject" {
		t.Errorf("Pattern 1 (.Steps.Step1.subject) failed: got %v", step1Obj)
	}

	// Pattern 2: scope["Step1"]
	topStep1, ok := scope["Step1"].(map[string]any)
	if !ok || topStep1["content"] != "Intro Body" {
		t.Errorf("Pattern 2 (.Step1.content) failed: got %v", topStep1)
	}

	// Pattern 3: scope["Step"]["1"] and scope["Step"]["2"]
	stepIndexed, ok := scope["Step"].(map[string]any)
	if !ok {
		t.Fatalf("expected scope[\"Step\"] map")
	}
	step1Idx, ok := stepIndexed["1"].(map[string]any)
	if !ok || step1Idx["subject"] != "Intro Subject" {
		t.Errorf("Pattern 3 (.Step.1.subject) failed: got %v", step1Idx)
	}
	step2Idx, ok := stepIndexed["2"].(map[string]any)
	if !ok || step2Idx["message"] != "WhatsApp Message Text" {
		t.Errorf("Pattern 3 (.Step.2.message) failed: got %v", step2Idx)
	}
}

func TestBifrostClientGeneratePrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req BifrostRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := BifrostResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{
					Message: struct {
						Content string `json:"content"`
					}{
						Content: "AI generated message for " + req.Messages[len(req.Messages)-1].Content,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBifrostClient(BifrostConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "test-model",
		Timeout:  2 * time.Second,
	})

	out, err := client.GeneratePrompt(context.Background(), "System prompt instruction", "User prompt detail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "AI generated message for User prompt detail"
	if out != expected {
		t.Errorf("expected '%s', got '%s'", expected, out)
	}
}

func TestBifrostClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"Delayed response"}}]}`))
	}))
	defer server.Close()

	client := NewBifrostClient(BifrostConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "test-model",
		Timeout:  30 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, err := client.GeneratePrompt(ctx, "System", "User")
	if err == nil {
		t.Errorf("expected timeout error, got nil")
	}
}

func TestBifrostWorkerPoolStressTest(t *testing.T) {
	var requestCount int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		time.Sleep(5 * time.Millisecond)
		resp := BifrostResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{
					Message: struct {
						Content string `json:"content"`
					}{
						Content: "Generated response",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBifrostClient(BifrostConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "test-model",
		Timeout:  2 * time.Second,
	})

	const numWorkers = 20
	const requestsPerWorker = 25
	var wg sync.WaitGroup

	start := time.Now()
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				_, err := client.GeneratePrompt(context.Background(), "System prompt", "User prompt")
				if err != nil {
					t.Errorf("concurrent prompt generation failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	totalExpected := int64(numWorkers * requestsPerWorker)
	if atomic.LoadInt64(&requestCount) != totalExpected {
		t.Errorf("expected %d requests, got %d", totalExpected, requestCount)
	}

	t.Logf("Completed %d concurrent JIT AI prompt generations in %v", totalExpected, duration)
}

func TestIntegration_Bifrost_LiveAI_ReplyClassification(t *testing.T) {
	testutil.LoadDotEnv()
	endpoint := getEnv("BIFROST_ENDPOINT", "https://litellm.aurumor.com")
	apiKey := getEnv("BIFROST_API_KEY", "dummy_key_for_vcr_replay")

	cfg := BifrostConfig{
		APIKey:   apiKey,
		Endpoint: endpoint,
		Model:    getEnv("BIFROST_MODEL", "gemini-3.5-flash-lite"),
		Timeout:  15 * time.Second,
	}

	client := NewBifrostClient(cfg)

	rec, vcrClient := testutil.NewVCRRecorder(t, "bifrost/reply_classification")
	if rec != nil {
		client.SetHTTPClient(vcrClient)
	}

	ctx, cancel := client.TimeoutContext()
	defer cancel()

	res, err := client.ClassifyReplyIntent(ctx, "I would love to learn more and see a demo!", time.Now().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("unexpected error classifying reply intent via Bifrost Live AI: %v", err)
	}

	t.Logf("Live AI Reply Intent Classification: Intent=%s, Reason=%s", res.Intent, res.Reason)
}

func TestBifrostClassifyReplyIntentAndExtractOOODate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req BifrostRequest
		json.NewDecoder(r.Body).Decode(&req)

		var respContent string
		if strings.Contains(req.Messages[0].Content, "Out-Of-Office") {
			respContent = `{"return_date": "2026-08-25T09:00:00Z"}`
		} else {
			respContent = `{"intent": "opt_out", "reason": "Explicit unsubscribe request", "return_date": ""}`
		}

		resp := BifrostResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{
					Message: struct {
						Content string `json:"content"`
					}{
						Content: respContent,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBifrostClient(BifrostConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
	})

	ctx := context.Background()

	// Test Intent Classification
	res, err := client.ClassifyReplyIntent(ctx, "Please stop emailing me", "2026-08-10T10:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error classifying reply intent: %v", err)
	}
	if res.Intent != "opt_out" {
		t.Errorf("expected intent 'opt_out', got %s", res.Intent)
	}

	// Test OOO Return Date Extraction
	oooDate, err := client.ExtractOOOReturnDate(ctx, "Out of office until Aug 25", "2026-08-10T10:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error extracting OOO return date: %v", err)
	}
	if oooDate.Year() != 2026 || oooDate.Month() != 8 || oooDate.Day() != 25 {
		t.Errorf("unexpected return date extracted: %v", oooDate)
	}
}

func TestPlainTextFormattingUtilities(t *testing.T) {
	// 1. Test StripHTML
	htmlInput := "<p>Hi Alex,<br/><br/>Thanks for connecting.</p><div>Best,<br/>John & Team</div>"
	expectedStripped := "Hi Alex,\n\nThanks for connecting.\n\nBest,\nJohn & Team"
	gotStripped := StripHTML(htmlInput)
	if gotStripped != expectedStripped {
		t.Errorf("StripHTML() = %q, expected %q", gotStripped, expectedStripped)
	}

	// 2. Test NormalizePlainTextLineBreaks
	winInput := "Line 1\r\n\r\nLine 2\r\n\r\n\r\n\r\nLine 3   "
	expectedNormalized := "Line 1\n\nLine 2\n\nLine 3"
	gotNormalized := NormalizePlainTextLineBreaks(winInput)
	if gotNormalized != expectedNormalized {
		t.Errorf("NormalizePlainTextLineBreaks() = %q, expected %q", gotNormalized, expectedNormalized)
	}

	// 3. Test FormatPlainTextWithSignature
	body := "Hi there,\n\nI hope you are doing well."
	sig := "<p>Best regards,<br/><strong>Alice Developer</strong></p>"
	formatted := FormatPlainTextWithSignature(body, sig)
	expectedFormatted := "Hi there,\n\nI hope you are doing well.\n\nBest regards,\nAlice Developer"
	if formatted != expectedFormatted {
		t.Errorf("FormatPlainTextWithSignature() = %q, expected %q", formatted, expectedFormatted)
	}

	// 4. Test EmailResponseFormat schema
	rf := EmailResponseFormat()
	if rf.Type != "json_schema" {
		t.Errorf("expected EmailResponseFormat type 'json_schema', got %q", rf.Type)
	}
}
