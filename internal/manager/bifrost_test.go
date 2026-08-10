package manager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/knadh/listmonk/models"
)

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
