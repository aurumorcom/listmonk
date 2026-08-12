package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/models"
)

// BifrostConfig holds settings for Bifrost AI inference.
type BifrostConfig struct {
	APIKey   string
	Endpoint string
	Model    string
	Timeout  time.Duration
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

// BifrostResponseFormat defines structured output schema settings.
type BifrostResponseFormat struct {
	Type       string `json:"type"` // "json_object" or "json_schema"
	JSONSchema any    `json:"json_schema,omitempty"`
}

// EmailStructuredOutput represents structured JSON response for email prompts.
type EmailStructuredOutput struct {
	Subject string `json:"subject"`
	Content string `json:"content"`
}

// MessageStructuredOutput represents structured JSON response for messaging prompts (WhatsApp/SMS).
type MessageStructuredOutput struct {
	Message string `json:"message"`
}

// BifrostRequest is the payload sent to the Bifrost AI endpoint.
type BifrostRequest struct {
	Model          string                 `json:"model"`
	Messages       []BifrostMessage       `json:"messages"`
	ResponseFormat *BifrostResponseFormat `json:"response_format,omitempty"`
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

// CleanJSONResponse strips markdown code block fences (e.g. ```json ... ```) from LLM output.
func CleanJSONResponse(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// GeneratePrompt performs JIT prompt execution with system and user prompts interpolated against context & user objects.
func (b *BifrostClient) GeneratePrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return b.GeneratePromptWithFormat(ctx, systemPrompt, userPrompt, nil)
}

// GeneratePromptWithFormat performs JIT prompt execution with an optional response format parameter.
func (b *BifrostClient) GeneratePromptWithFormat(ctx context.Context, systemPrompt, userPrompt string, respFormat *BifrostResponseFormat) (string, error) {
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
		Model:          b.cfg.Model,
		Messages:       messages,
		ResponseFormat: respFormat,
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

// ExtractTemplateScope extracts .Context, .User, .Subscriber/.Contact, and step history maps (.Steps, .Step1, .Step) from Subscriber attribs.
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

	scope := map[string]any{
		"Subscriber": sub,
		"Contact":    sub,
		"Context":    ctxObj,
		"User":       userObj,
	}

	stepsNamed := map[string]any{}
	stepsIndexed := map[string]any{}

	if sub.Attribs != nil {
		if rawHistory, ok := sub.Attribs["sequence_history"]; ok {
			var historyList []map[string]any
			if b, err := json.Marshal(rawHistory); err == nil {
				_ = json.Unmarshal(b, &historyList)
			}
			for _, item := range historyList {
				stepNumVal, exists := item["step_number"]
				if !exists {
					stepNumVal = item["step"]
				}
				if stepNumVal != nil {
					stepStr := fmt.Sprintf("%v", stepNumVal)
					stepKey := "Step" + stepStr
					stepsNamed[stepKey] = item
					stepsIndexed[stepStr] = item
					stepsIndexed[stepKey] = item
					scope[stepKey] = item
				}
			}
		} else if rawSteps, ok := sub.Attribs["steps"]; ok {
			var stepsMap map[string]map[string]any
			if b, err := json.Marshal(rawSteps); err == nil {
				_ = json.Unmarshal(b, &stepsMap)
			}
			for k, v := range stepsMap {
				cleanNum := strings.TrimPrefix(k, "Step")
				stepKey := "Step" + cleanNum
				stepsNamed[stepKey] = v
				stepsIndexed[cleanNum] = v
				stepsIndexed[stepKey] = v
				scope[stepKey] = v
			}
		}
	}

	scope["Steps"] = stepsNamed
	scope["Step"] = stepsIndexed

	return scope
}

// SignatureOpts defines parameters for 3-tier signature resolution.
type SignatureOpts struct {
	Subscriber    models.Subscriber
	Email         *models.Email
	WAHAMessenger *models.WAHAMessenger
	User          *auth.User
	GlobalSig     string
}

// ResolveSignatureAdvanced resolves the signature using the 3-tier hierarchy:
// 1. Sequence / Enrollment Signature (Most Specific)
// 2. Email / WhatsApp Signature
// 3. User Signature (Most General / Default)
// Fallback: Global Signature
func ResolveSignatureAdvanced(opts SignatureOpts) string {
	// Tier 1: Sequence / Enrollment Signature (from subscriber.Attribs)
	if opts.Subscriber.Attribs != nil {
		if sig, ok := opts.Subscriber.Attribs["sequence_signature"].(string); ok && strings.TrimSpace(sig) != "" {
			return strings.TrimSpace(sig)
		}
		if sig, ok := opts.Subscriber.Attribs["enrollment_signature"].(string); ok && strings.TrimSpace(sig) != "" {
			return strings.TrimSpace(sig)
		}
		if userMap, ok := opts.Subscriber.Attribs["user"].(map[string]any); ok {
			if sig, ok := userMap["signature"].(string); ok && strings.TrimSpace(sig) != "" {
				return strings.TrimSpace(sig)
			}
		} else if userMap, ok := opts.Subscriber.Attribs["user"].(models.JSON); ok {
			if sig, ok := userMap["signature"].(string); ok && strings.TrimSpace(sig) != "" {
				return strings.TrimSpace(sig)
			}
		}
		if sig, ok := opts.Subscriber.Attribs["user_signature"].(string); ok && strings.TrimSpace(sig) != "" {
			return strings.TrimSpace(sig)
		}
		if sig, ok := opts.Subscriber.Attribs["signature"].(string); ok && strings.TrimSpace(sig) != "" {
			return strings.TrimSpace(sig)
		}
	}

	// Tier 2: Email / WhatsApp Channel Signature
	if opts.Email != nil && strings.TrimSpace(opts.Email.Signature) != "" {
		return strings.TrimSpace(opts.Email.Signature)
	}
	if opts.WAHAMessenger != nil && strings.TrimSpace(opts.WAHAMessenger.Signature) != "" {
		return strings.TrimSpace(opts.WAHAMessenger.Signature)
	}

	// Tier 3: User Signature
	if opts.User != nil && strings.TrimSpace(opts.User.Signature) != "" {
		return strings.TrimSpace(opts.User.Signature)
	}

	// Ultimate Fallback: System Global Signature setting
	return strings.TrimSpace(opts.GlobalSig)
}

// ResolveSignature resolves the signature using default options for backward compatibility.
func ResolveSignature(sub models.Subscriber, globalSig string) string {
	return ResolveSignatureAdvanced(SignatureOpts{
		Subscriber: sub,
		GlobalSig:  globalSig,
	})
}
