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
