package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

func V6_3_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	// Add sender pool fields to sequences table if not present.
	if _, err := db.Exec(`
		ALTER TABLE sequences
			ADD COLUMN IF NOT EXISTS mailbox_ids INTEGER[] NOT NULL DEFAULT '{}',
			ADD COLUMN IF NOT EXISTS waha_sessions TEXT[] NOT NULL DEFAULT '{}',
			ADD COLUMN IF NOT EXISTS load_balance_mode TEXT NOT NULL DEFAULT 'round_robin';
	`); err != nil {
		return err
	}

	// Add sender lock fields to sequence_contacts table if not present.
	if _, err := db.Exec(`
		ALTER TABLE sequence_contacts
			ADD COLUMN IF NOT EXISTS mailbox_id INTEGER NULL REFERENCES mailboxes(id) ON DELETE SET NULL,
			ADD COLUMN IF NOT EXISTS waha_session TEXT NULL;

		CREATE INDEX IF NOT EXISTS idx_sequence_contacts_sender ON sequence_contacts(sequence_id, mailbox_id, waha_session);
	`); err != nil {
		return err
	}

	return nil
}
