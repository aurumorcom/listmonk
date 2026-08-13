package models

import null "gopkg.in/volatiletech/null.v6"

// Emails represents a slice of Email accounts.
type Emails []Email

// Email represents a sending email account pool for cold outreach sequences.
type Email struct {
	Base

	Name          string   `db:"name" json:"name"`
	Email         string   `db:"email" json:"email"`
	SMTPConfig    JSON     `db:"smtp_config" json:"smtp_config"`
	IMAPConfig    JSON     `db:"imap_config" json:"imap_config"`
	EmailsPerDay  int      `db:"emails_per_day" json:"emails_per_day"`
	EmailsPerHour int      `db:"emails_per_hour" json:"emails_per_hour"`
	EmailsToday   int      `db:"emails_today" json:"emails_today"`
	UserID        null.Int `db:"user_id" json:"user_id"`
	Signature     string   `db:"signature" json:"signature"`
}
