package models

import (
	"time"

	"github.com/lib/pq"
	null "gopkg.in/volatiletech/null.v6"
)

const (
	CadenceStatusActive   = "active"
	CadenceStatusPaused   = "paused"
	CadenceStatusArchived = "archived"

	CadenceContactStatusScheduled  = "scheduled"
	CadenceContactStatusInProgress = "in_progress"
	CadenceContactStatusReplied    = "replied"
	CadenceContactStatusFinished   = "finished"
	CadenceContactStatusOptedOut   = "opted_out"

	CadenceConditionAlways    = "always"
	CadenceConditionIfRead    = "if_read"
	CadenceConditionIfNotRead = "if_not_read"
	CadenceConditionIfClicked = "if_clicked"
)

// Cadences represents a slice of Cadence.
type Cadences []Cadence

// Cadence represents an automated multi-step cold outreach sequence.
type Cadence struct {
	Base

	UUID       string `db:"uuid" json:"uuid"`
	Name       string `db:"name" json:"name"`
	Status     string `db:"status" json:"status"`
	SendWindow JSON   `db:"send_window" json:"send_window"`
}

// CadenceSteps represents a slice of CadenceStep.
type CadenceSteps []CadenceStep

// CadenceStep represents an individual step in a cadence sequence.
type CadenceStep struct {
	ID         int           `db:"id" json:"id"`
	CadenceID  int           `db:"cadence_id" json:"cadence_id"`
	StepNumber int           `db:"step_number" json:"step_number"`
	DelayDays  int           `db:"delay_days" json:"delay_days"`
	Messenger  string        `db:"messenger" json:"messenger"`
	Condition  string        `db:"condition" json:"condition"`
	Subject    string        `db:"subject" json:"subject"`
	Body       string        `db:"body" json:"body"`
	TemplateID null.Int      `db:"template_id" json:"template_id"`
	MediaIDs   pq.Int64Array `db:"media_ids" json:"media_ids"`
}

// CadenceContacts represents a slice of CadenceContact.
type CadenceContacts []CadenceContact

// CadenceContact tracks the state machine position of a lead/contact within a cadence.
type CadenceContact struct {
	CadenceID     int         `db:"cadence_id" json:"cadence_id"`
	SubscriberID  int         `db:"subscriber_id" json:"subscriber_id"`
	Status        string      `db:"status" json:"status"`
	CurrentStep   int         `db:"current_step" json:"current_step"`
	NextSendAt    null.Time   `db:"next_send_at" json:"next_send_at"`
	LastReadAt    null.Time   `db:"last_read_at" json:"last_read_at"`
	LastClickedAt null.Time   `db:"last_clicked_at" json:"last_clicked_at"`
	LastMessageID null.String `db:"last_message_id" json:"last_message_id"`
	CreatedAt     time.Time   `db:"created_at" json:"created_at"`
}
