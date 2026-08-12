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

	EmailTypeNewThread = "New Thread"
	EmailTypeReply     = "Reply"

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
	ScheduleID      null.Int       `db:"schedule_id" json:"schedule_id"`
	Schedule        *Schedule      `db:"-" json:"schedule,omitempty"`
	Timezone        string         `db:"timezone" json:"timezone"`
	SendSchedule    JSON           `db:"send_schedule" json:"send_schedule"`
	SendWindow      JSON           `db:"send_window" json:"send_window"`
	EmailIDs        pq.Int64Array  `db:"email_ids" json:"email_ids"`
	WahaSessions    pq.StringArray `db:"waha_sessions" json:"waha_sessions"`
	LoadBalanceMode string         `db:"load_balance_mode" json:"load_balance_mode"`
}

// SequenceSchedule defines daily sending time windows and rate pacing settings.
type SequenceSchedule struct {
	Enabled            bool     `json:"enabled"`
	StartTime          string   `json:"start_time"`           // e.g. "09:00"
	EndTime            string   `json:"end_time"`             // e.g. "17:00"
	Days               []string `json:"days"`                 // e.g. ["mon", "tue", "wed", "thu", "fri"]
	MinIntervalSeconds int      `json:"min_interval_seconds"` // e.g. 30
	JitterSeconds      int      `json:"jitter_seconds"`       // e.g. 15
}

// SequenceSteps represents a slice of SequenceStep.
type SequenceSteps []SequenceStep

// SequenceStep represents an individual step in a sequence.
type SequenceStep struct {
	ID           int           `db:"id" json:"id"`
	SequenceID   int           `db:"sequence_id" json:"sequence_id"`
	StepNumber   int           `db:"step_number" json:"step_number"`
	DelaySeconds int           `db:"delay_seconds" json:"delay_seconds"`
	Messenger    string        `db:"messenger" json:"messenger"`
	Condition    string        `db:"condition" json:"condition"`
	Subject      string        `db:"subject" json:"subject"`
	Body         string        `db:"body" json:"body"`
	EmailType    string        `db:"email_type" json:"email_type"`
	TemplateID   null.Int      `db:"template_id" json:"template_id"`
	MediaIDs     pq.Int64Array `db:"media_ids" json:"media_ids"`
}

// SequenceContacts represents a slice of SequenceContact.
type SequenceContacts []SequenceContact

// SequenceContact tracks the state machine position of a lead/contact within a sequence.
type SequenceContact struct {
	SequenceID      int         `db:"sequence_id" json:"sequence_id"`
	SubscriberID    int         `db:"subscriber_id" json:"subscriber_id"`
	EmailID         null.Int    `db:"email_id" json:"email_id"`
	WahaSession     null.String `db:"waha_session" json:"waha_session"`
	Status          string      `db:"status" json:"status"`
	CurrentStep     int         `db:"current_step" json:"current_step"`
	NextSendAt      null.Time   `db:"next_send_at" json:"next_send_at"`
	LastReadAt      null.Time   `db:"last_read_at" json:"last_read_at"`
	LastClickedAt   null.Time   `db:"last_clicked_at" json:"last_clicked_at"`
	LastMessageID   null.String `db:"last_message_id" json:"last_message_id"`
	LastThreadMsgID null.String `db:"last_thread_msg_id" json:"last_thread_msg_id"`
	CreatedAt       time.Time   `db:"created_at" json:"created_at"`
}

// SequenceStepFunnel represents metrics for an individual sequence step in the conversion funnel.
type SequenceStepFunnel struct {
	StepNumber int    `json:"step_number"`
	Subject    string `json:"subject"`
	Messenger  string `json:"messenger"`
	Reached    int    `json:"reached"`
	Replied    int    `json:"replied"`
}

// SequenceAnalytics aggregates metrics across cold outreach sequences.
type SequenceAnalytics struct {
	ActiveContacts  int                  `json:"active_contacts"`
	StepCompletions int                  `json:"step_completions"`
	ReplyRate       float64              `json:"reply_rate"`
	ConversionRate  float64              `json:"conversion_rate"`
	Funnel          []SequenceStepFunnel `json:"funnel"`
}
