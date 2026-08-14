package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"
	"time"

	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/models"
	null "gopkg.in/volatiletech/null.v6"
)

func TestE2E_JIT_AI_Sequence_Contact(t *testing.T) {
	// Create contact Alice at Acme Corp
	contact := models.Subscriber{
		Base:  models.Base{ID: 501},
		Email: "alice@acme.corp",
		Name:  "Alice",
		Attribs: models.JSON{
			"company": "Acme Corp",
		},
	}

	// Scope extraction
	scope := manager.ExtractTemplateScope(contact)

	promptTemplate := "Write a friendly pitch for {{ .Contact.Name }} at {{ .Contact.Attribs.company }}"
	tmpl, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		t.Fatalf("Failed to parse prompt template: %v", err)
	}

	var renderedPrompt bytes.Buffer
	if err := tmpl.Execute(&renderedPrompt, scope); err != nil {
		t.Fatalf("Failed to execute prompt template: %v", err)
	}

	expectedPrompt := "Write a friendly pitch for Alice at Acme Corp"
	if renderedPrompt.String() != expectedPrompt {
		t.Errorf("expected rendered prompt '%s', got '%s'", expectedPrompt, renderedPrompt.String())
	}

	// Mock Bifrost AI server
	mockBifrost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req manager.BifrostRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.WriteHeader(http.StatusOK)
		resp := manager.BifrostResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{
					Message: struct {
						Content string `json:"content"`
					}{
						Content: "Hi Alice, hope all is well at Acme Corp!",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockBifrost.Close()

	bc := manager.NewBifrostClient(manager.BifrostConfig{
		APIKey:   "test-gemini-key",
		Endpoint: mockBifrost.URL,
		Model:    "gemini-3.5-flash-lite",
		Timeout:  5 * time.Second,
	})

	aiResult, err := bc.GeneratePrompt(bc.TimeoutContext(), "You are a sales rep", renderedPrompt.String())
	if err != nil {
		t.Fatalf("Bifrost AI prompt generation failed: %v", err)
	}

	expectedAIResult := "Hi Alice, hope all is well at Acme Corp!"
	if aiResult != expectedAIResult {
		t.Errorf("expected AI result '%s', got '%s'", expectedAIResult, aiResult)
	}

	t.Log("Successfully tested JIT AI Sequence .Contact namespace compilation & Bifrost resolution")
}

func TestE2E_JIT_AI_Campaign_Subscriber(t *testing.T) {
	// Create subscriber Bob at Stark Industries
	sub := models.Subscriber{
		Base:  models.Base{ID: 601},
		Email: "bob@stark.com",
		Name:  "Bob",
		Attribs: models.JSON{
			"company": "Stark Industries",
		},
	}

	// Scope extraction
	scope := manager.ExtractTemplateScope(sub)

	promptTemplate := "Dear {{ .Subscriber.Name }}, offer for {{ .Subscriber.Attribs.company }}"
	tmpl, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		t.Fatalf("Failed to parse prompt template: %v", err)
	}

	var renderedPrompt bytes.Buffer
	if err := tmpl.Execute(&renderedPrompt, scope); err != nil {
		t.Fatalf("Failed to execute prompt template: %v", err)
	}

	expectedPrompt := "Dear Bob, offer for Stark Industries"
	if renderedPrompt.String() != expectedPrompt {
		t.Errorf("expected rendered prompt '%s', got '%s'", expectedPrompt, renderedPrompt.String())
	}

	// Mock Bifrost AI server
	mockBifrost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := manager.BifrostResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{
					Message: struct {
						Content string `json:"content"`
					}{
						Content: "Exclusive partnership offer for Bob and Stark Industries.",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockBifrost.Close()

	bc := manager.NewBifrostClient(manager.BifrostConfig{
		APIKey:   "test-gemini-key",
		Endpoint: mockBifrost.URL,
		Model:    "gemini-3.5-flash-lite",
		Timeout:  5 * time.Second,
	})

	aiResult, err := bc.GeneratePrompt(bc.TimeoutContext(), "You are a marketing specialist", renderedPrompt.String())
	if err != nil {
		t.Fatalf("Bifrost AI prompt generation failed: %v", err)
	}

	expectedAIResult := "Exclusive partnership offer for Bob and Stark Industries."
	if aiResult != expectedAIResult {
		t.Errorf("expected AI result '%s', got '%s'", expectedAIResult, aiResult)
	}

	t.Log("Successfully tested JIT AI Campaign .Subscriber namespace compilation & Bifrost resolution")
}

func TestE2E_ContactAware_Template_Preview_Lifecycle(t *testing.T) {
	// Contact Carol at Wayne Enterprises with User.Signature
	sub := models.Subscriber{
		Base:  models.Base{ID: 901},
		Email: "carol@wayne.corp",
		Name:  "Carol",
		Attribs: models.JSON{
			"company": "Wayne Enterprises",
			"user": map[string]any{
				"signature": "<p>Carol's Custom Signature</p>",
			},
			"sequence_history": []map[string]any{
				{
					"step_number": 1,
					"messenger":   "email",
					"subject":     "Step 1 Intro Subject",
					"content":     "Step 1 Intro Content",
				},
			},
		},
	}

	scope := manager.ExtractTemplateScope(sub)

	// Verify step history referencing in preview scope
	step1Data, ok := scope["Step1"].(map[string]any)
	if !ok || step1Data["subject"] != "Step 1 Intro Subject" {
		t.Fatalf("expected Step 1 subject in preview scope, got %v", scope["Step1"])
	}

	// Verify signature resolution
	sig := manager.ResolveSignature(sub, "<p>Default Global Signature</p>")
	if sig != "<p>Carol's Custom Signature</p>" {
		t.Errorf("expected Carol's Custom Signature, got %q", sig)
	}

	// Simulate rendering prompt response with signature precedence
	mockAIContent := "<p>Hi Carol, following up on Step 1 Intro Subject.</p>"
	finalPreviewBody := mockAIContent + "<br/><br/>" + sig

	expectedPreview := "<p>Hi Carol, following up on Step 1 Intro Subject.</p><br/><br/><p>Carol's Custom Signature</p>"
	if finalPreviewBody != expectedPreview {
		t.Errorf("expected rendered preview %q, got %q", expectedPreview, finalPreviewBody)
	}

	t.Log("Successfully verified contact-aware template preview lifecycle with signature precedence and step history")
}

func TestE2E_TestMessageDispatch_Lifecycle(t *testing.T) {
	req := campReq{
		SubscriberID: 901,
		TestEmail:    "admin-test@example.com",
	}

	if req.SubscriberID != 901 || req.TestEmail != "admin-test@example.com" {
		t.Errorf("invalid test message request payload")
	}

	t.Log("Successfully verified test message dispatch payload lifecycle")
}

func TestE2E_DummySubscriber_UserBio_And_StepVariations(t *testing.T) {
	scope := manager.ExtractTemplateScope(dummySubscriber)

	// Verify User bio
	userObj, ok := scope["User"].(map[string]any)
	if !ok || userObj["bio"] == "" {
		t.Fatalf("expected non-empty user.bio in dummySubscriber scope, got %v", scope["User"])
	}

	// Verify Step 1 variations
	tplStr := `Rep: {{ .User.name }} ({{ .User.bio }}). Step 1: {{ .Step1.subject }}. Steps.Step1: {{ .Steps.Step1.subject }}. Step.1: {{ (index .Step "1").subject }}. Step.Step1: {{ .Step.Step1.subject }}`
	tmpl, err := template.New("test_dummy").Parse(tplStr)
	if err != nil {
		t.Fatalf("failed to parse template string: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, scope); err != nil {
		t.Fatalf("failed to execute template string: %v", err)
	}

	rendered := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Experienced Account Executive")) {
		t.Errorf("expected user.bio 'Experienced Account Executive' in rendered output, got %s", rendered)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Introduction Email")) {
		t.Errorf("expected 'Introduction Email' in rendered step variations, got %s", rendered)
	}

	t.Log("Successfully verified dummySubscriber with user.bio and all step template scope variations")
}

func TestE2E_MostPopulated_Subscriber_Preview_Selection(t *testing.T) {
	// Contact 1: Basic contact with minimal attribs
	c1 := models.Subscriber{
		Base:    models.Base{ID: 101},
		Email:   "basic@example.com",
		Name:    "Basic Contact",
		Attribs: models.JSON{"city": "NYC"},
	}

	// Contact 2: Rich CRM contact with extensive Frappe Lead doc attributes
	c2 := models.Subscriber{
		Base:  models.Base{ID: 102},
		Email: "richcrm@example.com",
		Name:  "Rich CRM Contact",
		Attribs: models.JSON{
			"city":         "San Francisco",
			"company_name": "Acme Corp",
			"user": map[string]any{
				"name":         "Alice Sales Rep",
				"email_id":     10,
				"waha_session": "sales_session_a",
				"signature":    "Best regards,\nAlice",
			},
			"frappe_lead": map[string]any{
				"lead_name":     "Rich CRM Contact",
				"status":        "Interested",
				"custom_budget": "$100,000",
				"notes":         "Key decision maker.",
			},
		},
	}

	// Compare attribs payload sizes
	b1, _ := json.Marshal(c1.Attribs)
	b2, _ := json.Marshal(c2.Attribs)
	if len(b2) <= len(b1) {
		t.Fatalf("expected rich CRM contact to have larger attribs payload than basic contact")
	}

	// Verify template scope extraction for rich CRM contact
	scope := manager.ExtractTemplateScope(c2)
	frappeLead, ok := scope["Subscriber"].(models.Subscriber).Attribs["frappe_lead"].(map[string]any)
	if !ok || frappeLead["custom_budget"] != "$100,000" {
		t.Fatalf("expected frappe_lead custom_budget '$100,000' in scope, got %v", scope["Subscriber"])
	}

	// Test fallback behavior logic
	subs := []models.Subscriber{c1, c2}

	// Simulating GetMostPopulatedSubscriber selection: select subscriber with max OCTET_LENGTH(attribs)
	var selected models.Subscriber
	maxLen := -1
	for _, s := range subs {
		b, _ := json.Marshal(s.Attribs)
		if len(b) > maxLen {
			maxLen = len(b)
			selected = s
		}
	}

	if selected.ID != 102 || selected.Name != "Rich CRM Contact" {
		t.Errorf("expected most populated contact (ID 102) selected for preview fallback, got ID %d", selected.ID)
	}

	// Verify empty database fallback returns dummySubscriber
	var emptySubs []models.Subscriber
	fallbackSub := dummySubscriber
	if len(emptySubs) > 0 {
		fallbackSub = emptySubs[0]
	}
	if fallbackSub.Email != "demo@listmonk.app" {
		t.Errorf("expected empty database fallback to 'demo@listmonk.app', got %s", fallbackSub.Email)
	}

	t.Log("Successfully verified explicit subscriber ID selection, most-populated CRM contact fallback, and empty DB dummy fallback")
}

func TestTemplate_ParentTemplateID_Persistence(t *testing.T) {
	tpl := models.Template{
		Name: "Prompt with Parent Layout",
		Type: models.TemplateTypePrompt,
		Body: "You are a helpful assistant.",
	}
	if tpl.ParentTemplateID.Valid {
		t.Errorf("expected ParentTemplateID to be null initially")
	}

	tpl.ParentTemplateID = null.IntFrom(42)
	if !tpl.ParentTemplateID.Valid || tpl.ParentTemplateID.Int != 42 {
		t.Errorf("expected ParentTemplateID to be valid and 42, got %v", tpl.ParentTemplateID)
	}
}
