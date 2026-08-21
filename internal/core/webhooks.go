package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

var (
	webhookWorkerOnce   sync.Once
	webhookWorkerCtx    context.Context
	webhookWorkerCancel context.CancelFunc
	webhookWorkerWG     sync.WaitGroup
	webhookClient       = &http.Client{
		Timeout: 10 * time.Second,
	}
)

// SetWebhookHTTPClient sets a custom HTTP client for testing / VCR transport.
func SetWebhookHTTPClient(client *http.Client) {
	if client != nil {
		webhookClient = client
	}
}

// ComputeHMACSignature computes the HMAC SHA256 signature for a webhook payload.
// Returns header format: t=<timestamp>,v1=<hex_signature>
func ComputeHMACSignature(secret string, timestamp int64, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", timestamp)))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, sig)
}

// StartWebhookWorkers starts background workers to poll and deliver pending webhook logs.
func (c *Core) StartWebhookWorkers(workerCount int) {
	if workerCount <= 0 {
		workerCount = 5
	}
	webhookWorkerOnce.Do(func() {
		webhookWorkerCtx, webhookWorkerCancel = context.WithCancel(context.Background())
		for i := 0; i < workerCount; i++ {
			webhookWorkerWG.Add(1)
			go c.runWebhookWorker(webhookWorkerCtx)
		}
	})
}

// StopWebhookWorkers stops all running webhook worker goroutines and waits for them to exit cleanly.
func (c *Core) StopWebhookWorkers() {
	if webhookWorkerCancel != nil {
		webhookWorkerCancel()
		webhookWorkerWG.Wait()
	}
}

func (c *Core) runWebhookWorker(ctx context.Context) {
	defer webhookWorkerWG.Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.processPendingWebhookLogs()
		}
	}
}

func (c *Core) processPendingWebhookLogs() {
	var logs []models.WebhookLog
	err := c.q.PopPendingWebhookLogs.Select(&logs, 10)
	if err != nil || len(logs) == 0 {
		return
	}

	for _, l := range logs {
		c.deliverWebhookLog(l)
	}
}

func (c *Core) deliverWebhookLog(l models.WebhookLog) {
	var ep models.Webhook
	if err := c.q.GetWebhookByID.Get(&ep, l.WebhookID); err != nil || !ep.Enabled {
		// Endpoint deleted or disabled
		_, _ = c.q.UpdateWebhookLogStatus.Exec("failed", l.Attempts+1, time.Now(), 0, "Endpoint disabled or deleted", l.ID)
		return
	}

	ts := time.Now().Unix()
	signature := ComputeHMACSignature(ep.Secret, ts, l.Payload)

	req, err := http.NewRequest("POST", ep.URL, bytes.NewBuffer(l.Payload))
	if err != nil {
		c.handleWebhookFailure(l, 0, err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Listmonk-Webhook-Engine")
	req.Header.Set("Listmonk-Signature", signature)

	resp, err := webhookClient.Do(req)
	if err != nil {
		c.handleWebhookFailure(l, 0, err.Error())
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	bodyStr := string(bodyBytes)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = c.q.UpdateWebhookLogStatus.Exec("delivered", l.Attempts+1, time.Now(), resp.StatusCode, bodyStr, l.ID)
	} else {
		c.handleWebhookFailure(l, resp.StatusCode, bodyStr)
	}
}

func (c *Core) handleWebhookFailure(l models.WebhookLog, code int, respBody string) {
	attempts := l.Attempts + 1
	if attempts >= l.MaxAttempts {
		_, _ = c.q.UpdateWebhookLogStatus.Exec("failed", attempts, time.Now(), code, respBody, l.ID)
		return
	}

	// Exponential backoff: 2^attempt * 5 seconds (capped at 1 hour)
	delaySec := 5 * (1 << (attempts - 1))
	if delaySec > 3600 {
		delaySec = 3600
	}
	nextRetry := time.Now().Add(time.Duration(delaySec) * time.Second)

	_, _ = c.q.UpdateWebhookLogStatus.Exec("pending", attempts, nextRetry, code, respBody, l.ID)
}

// DispatchWebhookEvent finds all active webhooks subscribed to eventType, creates full state transfer JSON payload, and enqueues delivery logs.
func (c *Core) DispatchWebhookEvent(eventType string, data any) error {
	var endpoints []models.Webhook
	if err := c.q.GetActiveWebhooksForEvent.Select(&endpoints, eventType); err != nil {
		return err
	}
	if len(endpoints) == 0 {
		return nil
	}

	evtID, _ := uuid.NewV4()
	payload := models.Event{
		ID:        "evt_" + evtID.String()[:16],
		Event:     eventType,
		CreatedAt: time.Now().UTC(),
		Data:      data,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	for _, ep := range endpoints {
		var logID int64
		_ = c.q.EnqueueWebhookLog.Get(&logID, ep.ID, eventType, payloadBytes)
	}

	return nil
}

// GetWebhooks fetches all webhooks.
func (c *Core) GetWebhooks() ([]models.Webhook, error) {
	var out []models.Webhook
	if err := c.q.GetWebhooks.Select(&out); err != nil {
		c.log.Printf("error fetching webhooks: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "webhooks", "error", pqErrMsg(err)))
	}
	return out, nil
}

// GetWebhook fetches a single webhook by ID.
func (c *Core) GetWebhook(id int) (models.Webhook, error) {
	var ep models.Webhook
	if err := c.q.GetWebhookByID.Get(&ep, id); err != nil {
		return models.Webhook{}, echo.NewHTTPError(http.StatusNotFound, "Webhook not found")
	}
	return ep, nil
}

// CreateWebhook creates a new webhook.
func (c *Core) CreateWebhook(ep models.Webhook) (models.Webhook, error) {
	if ep.Secret == "" {
		sec, _ := uuid.NewV4()
		ep.Secret = "whsec_" + sec.String()
	}

	var out models.Webhook
	if err := c.q.InsertWebhook.Get(&out, ep.Name, ep.URL, ep.Secret, ep.Events, ep.Enabled); err != nil {
		c.log.Printf("error creating webhook: %v", err)
		return models.Webhook{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorCreating", "name", "webhook", "error", pqErrMsg(err)))
	}

	return out, nil
}

// UpdateWebhook updates an existing webhook.
func (c *Core) UpdateWebhook(id int, ep models.Webhook) (models.Webhook, error) {
	var out models.Webhook
	if err := c.q.UpdateWebhook.Get(&out, ep.Name, ep.URL, ep.Secret, ep.Events, ep.Enabled, id); err != nil {
		c.log.Printf("error updating webhook: %v", err)
		return models.Webhook{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "webhook", "error", pqErrMsg(err)))
	}

	return out, nil
}

// DeleteWebhook deletes a webhook by ID.
func (c *Core) DeleteWebhook(id int) error {
	if _, err := c.q.DeleteWebhook.Exec(id); err != nil {
		c.log.Printf("error deleting webhook: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorDeleting", "name", "webhook", "error", pqErrMsg(err)))
	}
	return nil
}

// GetWebhookLogs fetches paginated webhook logs.
func (c *Core) GetWebhookLogs(offset, limit int) ([]models.WebhookLog, error) {
	var logs []models.WebhookLog
	if err := c.q.GetWebhookLogs.Select(&logs, limit, offset); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "error fetching webhook logs")
	}
	return logs, nil
}

// TestWebhook sends a test payload immediately to a specified target URL.
func (c *Core) TestWebhook(url, secret, eventType string) error {
	if url == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "URL is required for test")
	}
	if eventType == "" {
		eventType = "subscriber.created"
	}
	if secret == "" {
		secret = "test_secret"
	}

	dummyData := map[string]any{
		"test":      true,
		"message":   "This is a test webhook payload from Listmonk",
		"timestamp": time.Now().UTC(),
	}

	payload := models.Event{
		ID:        "evt_test_12345678",
		Event:     eventType,
		CreatedAt: time.Now().UTC(),
		Data:      dummyData,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	ts := time.Now().Unix()
	signature := ComputeHMACSignature(secret, ts, payloadBytes)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Listmonk-Webhook-Engine/Test")
	req.Header.Set("Listmonk-Signature", signature)

	resp, err := webhookClient.Do(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, fmt.Sprintf("Failed to reach endpoint: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Target returned HTTP %d: %s", resp.StatusCode, string(bodyBytes)))
	}

	return nil
}
