package models

import (
	"encoding/json"
	"strings"

	null "gopkg.in/volatiletech/null.v6"
)

// Emails represents a slice of Email accounts.
type Emails []Email

// Email represents a sending email account pool for cold outreach sequences.
type Email struct {
	Base

	Name          string   `db:"name" json:"name"`
	Email         string   `db:"email" json:"email"` // Primary SMTP login email value
	SMTPConfig    JSON     `db:"smtp_config" json:"smtp_config"`
	IMAPConfig    JSON     `db:"imap_config" json:"imap_config"`
	MaxSendPerDay int      `db:"max_send_per_day" json:"max_send_per_day"`
	SentToday     int      `db:"sent_today" json:"sent_today"`
	UserID        null.Int `db:"user_id" json:"user_id"`
	Signature     string   `db:"signature" json:"signature"`
}

// GetSMTPSettings parses and returns SMTPSettings from SMTPConfig.
func (e *Email) GetSMTPSettings() SMTPSettings {
	var s SMTPSettings
	if len(e.SMTPConfig) > 0 {
		if b, err := json.Marshal(e.SMTPConfig); err == nil {
			_ = json.Unmarshal(b, &s)
		}
	}
	if s.MaxSendPerDay == 0 && e.MaxSendPerDay > 0 {
		s.MaxSendPerDay = e.MaxSendPerDay
	}
	return s
}

// FromAddresses returns all configured sender addresses in SMTPSettings.fromAddresses,
// falling back to Email (the primary SMTP login email) if unconfigured.
func (e *Email) FromAddresses() []string {
	smtp := e.GetSMTPSettings()
	var addrs []string
	for _, a := range smtp.FromAddresses {
		if strings.TrimSpace(a) != "" {
			addrs = append(addrs, strings.TrimSpace(a))
		}
	}
	if len(addrs) == 0 && e.Email != "" {
		addrs = []string{e.Email}
	}
	return addrs
}

// FromAddress returns the primary sender email address for this account (SMTPSettings.fromAddresses[0]),
// falling back to Email if from_addresses is empty or unset.
func (e *Email) FromAddress() string {
	addrs := e.FromAddresses()
	if len(addrs) > 0 {
		return addrs[0]
	}
	return e.Email
}

// GetAddressSent returns daily emails sent for a specific address from SMTPSettings.SentToday.
func (e *Email) GetAddressSent(addr string) int {
	smtp := e.GetSMTPSettings()
	if smtp.SentToday != nil {
		return smtp.SentToday[addr]
	}
	return 0
}

// GetTotalSent returns total emails sent today across all addresses in SMTPSettings.SentToday,
// falling back to Email.SentToday if the map is empty.
func (e *Email) GetTotalSent() int {
	smtp := e.GetSMTPSettings()
	if len(smtp.SentToday) > 0 {
		total := 0
		for _, c := range smtp.SentToday {
			total += c
		}
		return total
	}
	return e.SentToday
}
