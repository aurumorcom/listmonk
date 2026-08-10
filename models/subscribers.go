package models

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/types"
	"github.com/lib/pq"
	null "gopkg.in/volatiletech/null.v6"
)

const (
	SubscriberStatusEnabled     = "enabled"
	SubscriberStatusDisabled    = "disabled"
	SubscriberStatusBlockListed = "blocklisted"

	SubscriptionStatusUnconfirmed  = "unconfirmed"
	SubscriptionStatusConfirmed    = "confirmed"
	SubscriptionStatusUnsubscribed = "unsubscribed"
)

// Subscribers represents a slice of Subscriber.
type Subscribers []Subscriber

// Subscriber represents an e-mail subscriber.
type Subscriber struct {
	Base

	UUID    string      `db:"uuid" json:"uuid"`
	Email   string      `db:"email" json:"email" form:"email"`
	Name    string      `db:"name" json:"name" form:"name"`
	Phone   null.String `db:"phone" json:"phone" form:"phone"`
	Attribs JSON        `db:"attribs" json:"attribs"`
	Status  string      `db:"status" json:"status"`
	Lists   types.JSONText `db:"lists" json:"lists"`
}

type subLists struct {
	SubscriberID int            `db:"subscriber_id"`
	Lists        types.JSONText `db:"lists"`
}

// GetIDs returns the list of subscriber IDs.
func (subs Subscribers) GetIDs() []int {
	IDs := make([]int, len(subs))
	for i, c := range subs {
		IDs[i] = c.ID
	}

	return IDs
}

// LoadLists lazy loads the lists for all the subscribers
// in the Subscribers slice and attaches them to their []Lists property.
func (subs Subscribers) LoadLists(stmt *sqlx.Stmt) error {
	var sl []subLists
	err := stmt.Select(&sl, pq.Array(subs.GetIDs()))
	if err != nil {
		return err
	}

	if len(subs) != len(sl) {
		return errors.New("campaign stats count does not match")
	}

	for i, s := range sl {
		if s.SubscriberID == subs[i].ID {
			subs[i].Lists = s.Lists
		}
	}

	return nil
}

// FirstName splits the name by spaces and returns the first chunk
// of the name that's greater than 2 characters in length, assuming
// that it is the subscriber's first name.
func (s Subscriber) FirstName() string {
	for _, s := range strings.Split(s.Name, " ") {
		if len(s) > 2 {
			return s
		}
	}

	return s.Name
}

// LastName splits the name by spaces and returns the last chunk
// of the name that's greater than 2 characters in length, assuming
// that it is the subscriber's last name.
func (s Subscriber) LastName() string {
	chunks := strings.Split(s.Name, " ")
	for i := len(chunks) - 1; i >= 0; i-- {
		chunk := chunks[i]
		if len(chunk) > 2 {
			return chunk
		}
	}

	return s.Name
}

// ResolveTimezone resolves the subscriber's timezone location using a 3-tier hierarchy:
// 1. Contact specific attribute (`Attribs["tz"]` or `Attribs["timezone"]`)
// 2. User-configured sequence default timezone (`seq.Timezone`)
// 3. Fallback to UTC (`time.UTC`)
func (s Subscriber) ResolveTimezone(seq Sequence) *time.Location {
	if s.Attribs != nil {
		if tzVal, ok := s.Attribs["tz"].(string); ok && strings.TrimSpace(tzVal) != "" {
			if loc, err := time.LoadLocation(strings.TrimSpace(tzVal)); err == nil {
				return loc
			}
		}
		if tzVal, ok := s.Attribs["timezone"].(string); ok && strings.TrimSpace(tzVal) != "" {
			if loc, err := time.LoadLocation(strings.TrimSpace(tzVal)); err == nil {
				return loc
			}
		}
	}

	if strings.TrimSpace(seq.Timezone) != "" {
		if loc, err := time.LoadLocation(strings.TrimSpace(seq.Timezone)); err == nil {
			return loc
		}
	}

	return time.UTC
}

// Subscription represents a list attached to a subscriber.
type Subscription struct {
	List
	SubscriptionStatus    null.String     `db:"subscription_status" json:"subscription_status"`
	SubscriptionCreatedAt null.String     `db:"subscription_created_at" json:"subscription_created_at"`
	Meta                  json.RawMessage `db:"meta" json:"meta"`
}

// SubscriberExport represents a subscriber record that is exported to raw data.
type SubscriberExport struct {
	Base

	UUID    string `db:"uuid" json:"uuid"`
	Email   string `db:"email" json:"email"`
	Name    string `db:"name" json:"name"`
	Attribs string `db:"attribs" json:"attribs"`
	Status  string `db:"status" json:"status"`
}

// SubscriberExportProfile represents a subscriber's collated data in JSON for export.
type SubscriberExportProfile struct {
	Email         string          `db:"email" json:"-"`
	Profile       json.RawMessage `db:"profile" json:"profile,omitempty"`
	Subscriptions json.RawMessage `db:"subscriptions" json:"subscriptions,omitempty"`
	CampaignViews json.RawMessage `db:"campaign_views" json:"campaign_views,omitempty"`
	LinkClicks    json.RawMessage `db:"link_clicks" json:"link_clicks,omitempty"`
}

// SubscriberActivity represents a subscriber's campaign views and link clicks for the Activity tab.
type SubscriberActivity struct {
	CampaignViews json.RawMessage `db:"campaign_views" json:"campaign_views"`
	LinkClicks    json.RawMessage `db:"link_clicks" json:"link_clicks"`
}
