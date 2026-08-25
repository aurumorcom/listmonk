package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/knadh/listmonk/models"
)

var (
	_crmInstance *CRMClient
	_crmOnce     sync.Once
)

// CRM returns the thread-safe singleton CRMClient.
func CRM(cfg models.CRMSettings) *CRMClient {
	_crmOnce.Do(func() {
		_crmInstance = NewCRMClient(cfg)
	})
	return _crmInstance
}

type CRMClient struct {
	cfg        models.CRMSettings
	httpClient *http.Client
}

type CRMDeepResearchPayload struct {
	CampaignSubscriber models.CampaignSubscriber `json:"campaign_subscriber"`
	Subscriber         models.Subscriber         `json:"subscriber"`
	CampaignID         int                       `json:"campaign_id"`
	ListIDs            []int                     `json:"list_ids"`
}

func NewCRMClient(cfg models.CRMSettings) *CRMClient {
	return &CRMClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CRMClient) DeepResearch(ctx context.Context, payload CRMDeepResearchPayload) error {
	if c == nil || !c.cfg.Enabled || c.cfg.BaseURL == "" {
		return nil
	}
	endpoint := fmt.Sprintf("%s/api/method/frappe_listmonk.deep_research.get", c.cfg.BaseURL)
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", c.cfg.APIKey, c.cfg.APISecret))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("crm deep research returned status %d", resp.StatusCode)
	}
	return nil
}
