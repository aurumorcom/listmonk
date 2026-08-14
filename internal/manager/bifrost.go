package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
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

var reHTMLTags = regexp.MustCompile(`<[^>]*>`)
var reBlockTags = regexp.MustCompile(`(?i)</p>|</div>|</h[1-6]>`)
var reBreakTags = regexp.MustCompile(`(?i)<br\s*/?>`)
var reMultipleNewlines = regexp.MustCompile(`\n{3,}`)

// EmailResponseFormat returns the json_schema response format guide for email prompt completions.
func EmailResponseFormat() *BifrostResponseFormat {
	return &BifrostResponseFormat{
		Type: "json_schema",
		JSONSchema: map[string]any{
			"name":   "email_prompt_output",
			"strict": true,
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"subject": map[string]any{
						"type":        "string",
						"description": "The email subject line. Short, compelling, and relevant.",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "The main body of the email in pure plain text. MUST NOT contain the subject line. MUST NOT contain signatures or sign-offs (e.g. 'Best regards', 'Sincerely', or closing names), as signatures are dynamically appended by the system.",
					},
				},
				"required":             []string{"subject", "content"},
				"additionalProperties": false,
			},
		},
	}
}

// StripHTML converts HTML block end tags to \n\n and line breaks to \n, strips remaining HTML tags, and unescapes HTML entities.
func StripHTML(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	s := reBlockTags.ReplaceAllString(input, "\n\n")
	s = reBreakTags.ReplaceAllString(s, "\n")
	s = reHTMLTags.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return NormalizePlainTextLineBreaks(s)
}

// NormalizePlainTextLineBreaks converts Windows newlines (\r\n) to \n, trims space, and collapses 3+ consecutive newlines into \n\n.
func NormalizePlainTextLineBreaks(input string) string {
	s := strings.ReplaceAll(input, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSpace(s)
	s = reMultipleNewlines.ReplaceAllString(s, "\n\n")
	return s
}

// FormatPlainTextWithSignature normalizes the content and signature to pure plain text and joins them with double newlines.
func FormatPlainTextWithSignature(content string, sig string) string {
	cleanContent := NormalizePlainTextLineBreaks(StripHTML(content))
	cleanSig := NormalizePlainTextLineBreaks(StripHTML(sig))

	if cleanSig == "" {
		return cleanContent
	}
	if cleanContent == "" {
		return cleanSig
	}

	return fmt.Sprintf("%s\n\n%s", cleanContent, cleanSig)
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

// ReplyIntentResult represents structured intent classification output from LLM analysis.
type ReplyIntentResult struct {
	Intent     string `json:"intent"`      // "opt_out", "interested", "out_of_office", "other"
	Reason     string `json:"reason"`      // Explanation
	ReturnDate string `json:"return_date"` // ISO 8601 UTC string if OOO, else ""
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

// TimeoutContext returns a new context with the configured Bifrost timeout and its cancel function.
func (b *BifrostClient) TimeoutContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), b.cfg.Timeout)
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

// ClassifyReplyIntent classifies an incoming message body into intent categories ("opt_out", "interested", "out_of_office", "other").
func (b *BifrostClient) ClassifyReplyIntent(ctx context.Context, messageBody string, nowStr string) (*ReplyIntentResult, error) {
	if b == nil || b.cfg.APIKey == "" {
		return nil, fmt.Errorf("bifrost client is not configured")
	}

	systemPrompt := fmt.Sprintf(`You are an AI assistant analyzing incoming email/WhatsApp replies for sales outreach sequences.
Analyze the recipient's message and determine their intent.
Today's date and time is %s.

Rules for intent classification:
- "opt_out": Recipient requests to stop receiving messages, unsubscribe, remove email, or expresses explicit unwillingness (e.g., "STOP", "Unsubscribe", "Take me off your list", "Don't email me").
- "interested": Recipient expresses interest, asks for pricing, requests a call/demo, or asks a follow-up question.
- "out_of_office": Recipient is away, on vacation, or an automatic out-of-office reply. If a return date is stated, extract it into return_date as ISO 8601 UTC timestamp.
- "other": Generic replies or unclassified text.

Output JSON conforming to the requested schema.`, nowStr)

	respFormat := &BifrostResponseFormat{
		Type: "json_object",
	}

	raw, err := b.GeneratePromptWithFormat(ctx, systemPrompt, messageBody, respFormat)
	if err != nil {
		return nil, err
	}

	clean := CleanJSONResponse(raw)
	var res ReplyIntentResult
	if err := json.Unmarshal([]byte(clean), &res); err != nil {
		return nil, fmt.Errorf("error unmarshaling reply intent response: %w", err)
	}

	return &res, nil
}

// ExtractOOOReturnDate parses an Out-Of-Office message text and extracts the exact return timestamp if present.
func (b *BifrostClient) ExtractOOOReturnDate(ctx context.Context, messageBody string, nowStr string) (time.Time, error) {
	if b == nil || b.cfg.APIKey == "" {
		return time.Time{}, fmt.Errorf("bifrost client is not configured")
	}

	systemPrompt := fmt.Sprintf(`You are an AI assistant parsing Out-Of-Office auto-reply messages.
Your goal is to extract the date the recipient will return to work. Today's date is %s.
Return JSON in format: {"return_date": "YYYY-MM-DDTHH:MM:SSZ"} if a date is mentioned, or {"return_date": ""} if no clear return date is stated.`, nowStr)

	respFormat := &BifrostResponseFormat{
		Type: "json_object",
	}

	raw, err := b.GeneratePromptWithFormat(ctx, systemPrompt, messageBody, respFormat)
	if err != nil {
		return time.Time{}, err
	}

	clean := CleanJSONResponse(raw)
	var res struct {
		ReturnDate string `json:"return_date"`
	}
	if err := json.Unmarshal([]byte(clean), &res); err != nil || res.ReturnDate == "" {
		return time.Time{}, fmt.Errorf("no return date parsed")
	}

	t, err := time.Parse(time.RFC3339, res.ReturnDate)
	if err != nil {
		// Fallback parse YYYY-MM-DD
		if tShort, err2 := time.Parse("2006-01-02", res.ReturnDate); err2 == nil {
			return tShort, nil
		}
		return time.Time{}, err
	}

	return t, nil
}
