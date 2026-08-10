package models

// Mailboxes represents a slice of Mailbox.
type Mailboxes []Mailbox

// Mailbox represents a sending email account pool for cold outreach sequences.
type Mailbox struct {
	Base

	Name       string `db:"name" json:"name"`
	Email      string `db:"email" json:"email"`
	SMTPConfig JSON   `db:"smtp_config" json:"smtp_config"`
	IMAPConfig JSON   `db:"imap_config" json:"imap_config"`
	DailyLimit int    `db:"daily_limit" json:"daily_limit"`
	SentToday  int    `db:"sent_today" json:"sent_today"`
}
