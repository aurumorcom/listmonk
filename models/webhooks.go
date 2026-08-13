package models

import (
	"time"

	"github.com/lib/pq"
)

// Webhook represents a registered webhook receiver endpoint.
type Webhook struct {
	ID        int            `db:"id" json:"id"`
	Name      string         `db:"name" json:"name"`
	URL       string         `db:"url" json:"url"`
	Secret    string         `db:"secret" json:"secret"`
	Events    pq.StringArray `db:"events" json:"events"`
	Enabled   bool           `db:"enabled" json:"enabled"`
	CreatedAt time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt time.Time      `db:"updated_at" json:"updated_at"`
}

// WebhookLog represents an outbound webhook delivery queue item and log record.
type WebhookLog struct {
	ID           int64     `db:"id" json:"id"`
	EndpointID   int       `db:"endpoint_id" json:"endpoint_id"`
	EventType    string    `db:"event_type" json:"event_type"`
	Payload      []byte    `db:"payload" json:"payload"`
	Status       string    `db:"status" json:"status"`
	Attempts     int       `db:"attempts" json:"attempts"`
	MaxAttempts  int       `db:"max_attempts" json:"max_attempts"`
	NextRetryAt  time.Time `db:"next_retry_at" json:"next_retry_at"`
	ResponseCode int       `db:"response_code" json:"response_code"`
	ResponseBody string    `db:"response_body" json:"response_body"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// Event represents the JSON body sent to webhook targets (Event-Carried State Transfer).
type Event struct {
	ID        string    `json:"id"`
	Event     string    `json:"event"`
	CreatedAt time.Time `json:"created_at"`
	Data      any       `json:"data"`
}
