package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/knadh/listmonk/models"
)

// BifrostConfig holds settings for Bifrost AI inference.
type BifrostConfig struct {
	APIKey  string
	Endpoint string
	Model   string
	Timeout time.Duration
}

// BifrostClient is a client for performing JIT AI prompt completions via Bifrost.
type BifrostClient struct {
	cfg        BifrostConfig
	httpClient *http.Client
}

// BifrostMessage represents a chat message in the completion request.
type BifrostMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// BifrostRequest is the payload sent to the Bifrost AI endpoint.
type BifrostRequest struct {
	Model    string           `json:"model"`
	Messages []BifrostMessage `json:"messages"`
}

// BifrostResponse represents the response from the Bifrost AI endpoint.
type BifrostResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewBifrostClient initializes a new BifrostClient.
func NewBifrostClient(cfg BifrostConfig) *BifrostClient {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}

	return &BifrostClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// TimeoutContext returns a new context with the configured Bifrost timeout.
func (b *BifrostClient) TimeoutContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), b.cfg.Timeout)
	return ctx
}

// GeneratePrompt performs JIT prompt execution with system and user prompts interpolated against context & user objects.
func (b *BifrostClient) GeneratePrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if b == nil || b.cfg.APIKey == "" {
		return "", fmt.Errorf("bifrost client is not configured")
	}

	endpoint := b.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/chat/completions"
	}

	messages := []BifrostMessage{}
	if systemPrompt != "" {
		messages = append(messages, BifrostMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	messages = append(messages, BifrostMessage{
		Role:    "user",
		Content: userPrompt,
	})

	reqPayload := BifrostRequest{
		Model:    b.cfg.Model,
		Messages: messages,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("error encoding bifrost request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("error creating bifrost http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.cfg.APIKey)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error executing bifrost http call: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading bifrost response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bifrost API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var bifrostResp BifrostResponse
	if err := json.Unmarshal(respBytes, &bifrostResp); err != nil {
		return "", fmt.Errorf("error parsing bifrost response JSON: %w", err)
	}

	if bifrostResp.Error != nil && bifrostResp.Error.Message != "" {
		return "", fmt.Errorf("bifrost error: %s", bifrostResp.Error.Message)
	}

	if len(bifrostResp.Choices) == 0 {
		return "", fmt.Errorf("bifrost returned no choices in response")
	}

	return bifrostResp.Choices[0].Message.Content, nil
}

// ExtractTemplateScope extracts .Context, .User, and .Subscriber or .Contact maps from Subscriber attribs.
func ExtractTemplateScope(sub models.Subscriber) map[string]any {
	var ctxObj any
	var userObj any

	if sub.Attribs != nil {
		ctxObj = sub.Attribs["context"]
		userObj = sub.Attribs["user"]
	}

	if ctxObj == nil {
		ctxObj = map[string]any{}
	}
	if userObj == nil {
		userObj = map[string]any{}
	}

	return map[string]any{
		"Subscriber": sub,
		"Contact":    sub,
		"Context":    ctxObj,
		"User":       userObj,
	}
}
