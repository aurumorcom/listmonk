package models

import (
	"time"

	"github.com/lib/pq"
	null "gopkg.in/volatiletech/null.v6"
)

const (
	SequenceStatusActive   = "active"
	SequenceStatusPaused   = "paused"
	SequenceStatusArchived = "archived"

	SequenceContactStatusScheduled  = "scheduled"
	SequenceContactStatusInProgress = "in_progress"
	SequenceContactStatusReplied    = "replied"
	SequenceContactStatusFinished   = "finished"
	SequenceContactStatusOptedOut   = "opted_out"

	SequenceConditionAlways    = "always"
	SequenceConditionIfRead    = "if_read"
	SequenceConditionIfNotRead = "if_not_read"
	SequenceConditionIfClicked = "if_clicked"

	LoadBalanceModeRoundRobin       = "round_robin"
	LoadBalanceModeCapacityWeighted = "capacity_weighted"
)

// Sequences represents a slice of Sequence.
type Sequences []Sequence

// Sequence represents an automated multi-step cold outreach sequence.
type Sequence struct {
	Base

	UUID            string         `db:"uuid" json:"uuid"`
	Name            string         `db:"name" json:"name"`
	Status          string         `db:"status" json:"status"`
	SendWindow      JSON           `db:"send_window" json:"send_window"`
	MailboxIDs      pq.Int64Array  `db:"mailbox_ids" json:"mailbox_ids"`
	WahaSessions    pq.StringArray `db:"waha_sessions" json:"waha_sessions"`
	LoadBalanceMode string         `db:"load_balance_mode" json:"load_balance_mode"`
}

// SequenceSteps represents a slice of SequenceStep.
type SequenceSteps []SequenceStep

// SequenceStep represents an individual step in a sequence.
type SequenceStep struct {
	ID         int           `db:"id" json:"id"`
	SequenceID int           `db:"sequence_id" json:"sequence_id"`
	StepNumber int           `db:"step_number" json:"step_number"`
	DelayDays  int           `db:"delay_days" json:"delay_days"`
	Messenger  string        `db:"messenger" json:"messenger"`
	Condition  string        `db:"condition" json:"condition"`
	Subject    string        `db:"subject" json:"subject"`
	Body       string        `db:"body" json:"body"`
	TemplateID null.Int      `db:"template_id" json:"template_id"`
	MediaIDs   pq.Int64Array `db:"media_ids" json:"media_ids"`
}

// SequenceContacts represents a slice of SequenceContact.
type SequenceContacts []SequenceContact

// SequenceContact tracks the state machine position of a lead/contact within a sequence.
type SequenceContact struct {
	SequenceID    int         `db:"sequence_id" json:"sequence_id"`
	SubscriberID  int         `db:"subscriber_id" json:"subscriber_id"`
	MailboxID     null.Int    `db:"mailbox_id" json:"mailbox_id"`
	WahaSession   null.String `db:"waha_session" json:"waha_session"`
	Status        string      `db:"status" json:"status"`
	CurrentStep   int         `db:"current_step" json:"current_step"`
	NextSendAt    null.Time   `db:"next_send_at" json:"next_send_at"`
	LastReadAt    null.Time   `db:"last_read_at" json:"last_read_at"`
	LastClickedAt null.Time   `db:"last_clicked_at" json:"last_clicked_at"`
	LastMessageID null.String `db:"last_message_id" json:"last_message_id"`
	CreatedAt     time.Time   `db:"created_at" json:"created_at"`
}
