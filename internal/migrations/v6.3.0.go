package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V6_3_0 performs consolidated DB migrations for v6.3.0 (sequences, prompt templates, schedules, emails table, threading & channel locking).
func V6_3_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	lo.Printf("running consolidated migration v6.3.0")

	// 1. Create emails table
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS emails (
			id              SERIAL PRIMARY KEY,
			name            TEXT NOT NULL,
			email           TEXT NOT NULL UNIQUE,
			smtp_config     JSONB NOT NULL DEFAULT '{}',
			imap_config     JSONB NOT NULL DEFAULT '{}',
			max_send_per_day INTEGER NOT NULL DEFAULT 0,
			sent_today      INTEGER NOT NULL DEFAULT 0,
			user_id         INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
			signature       TEXT NOT NULL DEFAULT '',
			created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);

		ALTER TABLE emails
			ADD COLUMN IF NOT EXISTS user_id INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
			ADD COLUMN IF NOT EXISTS signature TEXT NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS max_send_per_day INTEGER NOT NULL DEFAULT 0,
			ADD COLUMN IF NOT EXISTS sent_today INTEGER NOT NULL DEFAULT 0;
		CREATE INDEX IF NOT EXISTS idx_emails_user_id ON emails(user_id);
	`); err != nil {
		return err
	}

	// 2. Add sender pool fields to sequences table if not present.
	if _, err := db.Exec(`
		ALTER TABLE sequences
			ADD COLUMN IF NOT EXISTS email_ids INTEGER[] NOT NULL DEFAULT '{}',
			ADD COLUMN IF NOT EXISTS waha_sessions TEXT[] NOT NULL DEFAULT '{}',
			ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS archive BOOLEAN NOT NULL DEFAULT false,
			ADD COLUMN IF NOT EXISTS archive_template_id INTEGER NULL REFERENCES templates(id) ON DELETE SET NULL,
			ADD COLUMN IF NOT EXISTS archive_slug TEXT NULL,
			ADD COLUMN IF NOT EXISTS archive_meta JSONB NOT NULL DEFAULT '{}';

		ALTER TABLE sequences DROP COLUMN IF EXISTS load_balance_mode;
	`); err != nil {
		return err
	}

	// 3. Add sender lock fields to sequence_contacts table if not present.
	if _, err := db.Exec(`
		ALTER TABLE sequence_contacts
			ADD COLUMN IF NOT EXISTS email_id INTEGER NULL REFERENCES emails(id) ON DELETE SET NULL,
			ADD COLUMN IF NOT EXISTS waha_session TEXT NULL,
			ADD COLUMN IF NOT EXISTS last_thread_msg_id TEXT NULL;

		CREATE INDEX IF NOT EXISTS idx_sequence_contacts_sender ON sequence_contacts(sequence_id, email_id, waha_session);
	`); err != nil {
		return err
	}

	// 4. Prompt Template & Bifrost AI support + Parent HTML Layout
	if _, err := db.Exec(`
		ALTER TYPE template_type ADD VALUE IF NOT EXISTS 'prompt';
		ALTER TABLE templates
			ADD COLUMN IF NOT EXISTS parent_template_id INTEGER NULL REFERENCES templates(id) ON DELETE SET NULL;
		ALTER TABLE templates
			DROP COLUMN IF EXISTS system_prompt;
	`); err != nil {
		return err
	}

	// 5. Schedules table and sequences schedule_id
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schedules (
			id                   SERIAL PRIMARY KEY,
			uuid                 UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
			name                 TEXT NOT NULL,
			timezone             TEXT NOT NULL DEFAULT 'UTC',
			use_contact_timezone BOOLEAN NOT NULL DEFAULT TRUE,
			skip_holidays        BOOLEAN NOT NULL DEFAULT TRUE,
			sending_windows      JSONB NOT NULL DEFAULT '{}',
			is_default           BOOLEAN NOT NULL DEFAULT false,
			created_at           TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at           TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);

		ALTER TABLE schedules
			ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false;

		CREATE UNIQUE INDEX IF NOT EXISTS schedules_is_default_idx ON schedules (is_default) WHERE is_default = true;

		ALTER TABLE sequences
			ADD COLUMN IF NOT EXISTS schedule_id INTEGER NULL REFERENCES schedules(id) ON DELETE SET NULL;
	`); err != nil {
		return err
	}

	// Seed default schedule if none exists
	var count int
	if err := db.Get(&count, "SELECT COUNT(*) FROM schedules"); err == nil && count == 0 {
		defaultWindows := `{"mon":{"start":"08:00","end":"17:00"},"tue":{"start":"08:00","end":"17:00"},"wed":{"start":"08:00","end":"17:00"},"thu":{"start":"08:00","end":"17:00"},"fri":{"start":"08:00","end":"17:00"},"sat":{},"sun":{}}`
		var defaultID int
		err := db.Get(&defaultID, `
			INSERT INTO schedules (name, timezone, use_contact_timezone, skip_holidays, sending_windows, is_default)
			VALUES ($1, $2, $3, $4, $5, true)
			RETURNING id`,
			"Normal Business Hours", "UTC", true, true, defaultWindows)
		if err == nil {
			_, _ = db.Exec("UPDATE sequences SET schedule_id = $1 WHERE schedule_id IS NULL", defaultID)
		}
	}

	// 6. Sequence steps email_type
	if _, err := db.Exec(`
		ALTER TABLE sequence_steps
			ADD COLUMN IF NOT EXISTS email_type TEXT NOT NULL DEFAULT '';
	`); err != nil {
		return err
	}

	// 7. Users table channel bindings & phone
	if _, err := db.Exec(`
		ALTER TABLE users
			ADD COLUMN IF NOT EXISTS email_id INTEGER NULL REFERENCES emails(id) ON DELETE SET NULL,
			ADD COLUMN IF NOT EXISTS waha_session TEXT NULL,
			ADD COLUMN IF NOT EXISTS signature TEXT NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS phone TEXT NULL;
		CREATE INDEX IF NOT EXISTS idx_users_channels ON users(email_id, waha_session);
		CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);
	`); err != nil {
		return err
	}

	// 8. Webhook Endpoints & Delivery Logs tables
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS webhook_endpoints (
			id          SERIAL PRIMARY KEY,
			name        TEXT NOT NULL,
			url         TEXT NOT NULL,
			secret      TEXT NOT NULL,
			events      TEXT[] NOT NULL DEFAULT '{}',
			enabled     BOOLEAN NOT NULL DEFAULT TRUE,
			created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS webhook_logs (
			id            BIGSERIAL PRIMARY KEY,
			endpoint_id   INT REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
			event_type    TEXT NOT NULL,
			payload       JSONB NOT NULL,
			status        TEXT NOT NULL DEFAULT 'pending',
			attempts      INT NOT NULL DEFAULT 0,
			max_attempts  INT NOT NULL DEFAULT 5,
			next_retry_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			response_code INT NOT NULL DEFAULT 0,
			response_body TEXT NOT NULL DEFAULT '',
			created_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_webhook_logs_pending ON webhook_logs(status, next_retry_at) WHERE status = 'pending';
	`); err != nil {
		return err
	}

	// 9. Ensure waha and webhooks seed rows exist in settings table
	if _, err := db.Exec(`
		INSERT INTO settings (key, value) VALUES
			('waha', '[]'),
			('webhooks', '[]')
		ON CONFLICT (key) DO NOTHING;
	`); err != nil {
		return err
	}

	return nil
}
