package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V6_3_0 performs consolidated DB migrations for v6.3.0.
func V6_3_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	lo.Printf("running consolidated migration v6.3.0")

	if _, err := db.Exec(`
		-- 1. Emails table
		CREATE TABLE IF NOT EXISTS emails (
			id               SERIAL PRIMARY KEY,
			name             TEXT NOT NULL,
			email            TEXT NOT NULL UNIQUE,
			smtp_config      JSONB NOT NULL DEFAULT '{}',
			imap_config      JSONB NOT NULL DEFAULT '{}',
			max_send_per_day INTEGER NOT NULL DEFAULT 0,
			sent_today       INTEGER NOT NULL DEFAULT 0,
			user_id          INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
			signature        TEXT NOT NULL DEFAULT '',
			created_at       TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at       TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_emails_user_id ON emails(user_id);

		-- 2. Sequences & Sequence Subscribers
		ALTER TABLE sequences
			ADD COLUMN IF NOT EXISTS email_ids INTEGER[] NOT NULL DEFAULT '{}',
			ADD COLUMN IF NOT EXISTS waha_sessions TEXT[] NOT NULL DEFAULT '{}',
			ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS archive BOOLEAN NOT NULL DEFAULT false,
			ADD COLUMN IF NOT EXISTS archive_template_id INTEGER NULL REFERENCES templates(id) ON DELETE SET NULL,
			ADD COLUMN IF NOT EXISTS archive_slug TEXT NULL,
			ADD COLUMN IF NOT EXISTS archive_meta JSONB NOT NULL DEFAULT '{}';
		ALTER TABLE sequences DROP COLUMN IF EXISTS load_balance_mode;

		ALTER TABLE sequence_subscribers
			ADD COLUMN IF NOT EXISTS email_id INTEGER NULL REFERENCES emails(id) ON DELETE SET NULL,
			ADD COLUMN IF NOT EXISTS from_address TEXT NULL,
			ADD COLUMN IF NOT EXISTS waha_session TEXT NULL,
			ADD COLUMN IF NOT EXISTS last_thread_msg_id TEXT NULL;
		CREATE INDEX IF NOT EXISTS idx_seq_subscribers_sender ON sequence_subscribers(sequence_id, email_id, waha_session);

		-- 3. Templates & Sequence Steps
		ALTER TYPE template_type ADD VALUE IF NOT EXISTS 'prompt';
		ALTER TABLE templates ADD COLUMN IF NOT EXISTS parent_template_id INTEGER NULL REFERENCES templates(id) ON DELETE SET NULL;
		ALTER TABLE templates DROP COLUMN IF EXISTS system_prompt;
		ALTER TABLE sequence_steps ADD COLUMN IF NOT EXISTS email_type TEXT NOT NULL DEFAULT '';

		-- 4. Schedules Table
		CREATE TABLE IF NOT EXISTS schedules (
			id                      SERIAL PRIMARY KEY,
			uuid                    UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
			name                    TEXT NOT NULL,
			timezone                TEXT NOT NULL DEFAULT 'UTC',
			use_subscriber_timezone BOOLEAN NOT NULL DEFAULT TRUE,
			skip_holidays           BOOLEAN NOT NULL DEFAULT TRUE,
			sending_windows         JSONB NOT NULL DEFAULT '{}',
			is_default              BOOLEAN NOT NULL DEFAULT false,
			created_at              TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at              TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_schedules_default ON schedules (is_default) WHERE is_default = true;
		ALTER TABLE sequences ADD COLUMN IF NOT EXISTS schedule_id INTEGER NULL REFERENCES schedules(id) ON DELETE SET NULL;

		-- 5. Users table
		ALTER TABLE users
			ADD COLUMN IF NOT EXISTS email_id INTEGER NULL REFERENCES emails(id) ON DELETE SET NULL,
			ADD COLUMN IF NOT EXISTS waha_session TEXT NULL,
			ADD COLUMN IF NOT EXISTS signature TEXT NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS phone TEXT NULL;
		CREATE INDEX IF NOT EXISTS idx_users_channels ON users(email_id, waha_session);
		CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);

		-- 6. Webhooks & Webhook Logs
		DO $$
		BEGIN
			IF EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'webhook_endpoints') THEN
				ALTER TABLE webhook_endpoints RENAME TO webhooks;
			END IF;
			IF EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'webhook_logs' AND column_name = 'endpoint_id') THEN
				ALTER TABLE webhook_logs RENAME COLUMN endpoint_id TO webhook_id;
			END IF;
		END $$;

		CREATE TABLE IF NOT EXISTS webhooks (
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
			webhook_id    INT REFERENCES webhooks(id) ON DELETE CASCADE,
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

		-- 7. Seed Settings Rows
		INSERT INTO settings (key, value) VALUES
			('waha', '[]'),
			('webhooks', '[]')
		ON CONFLICT (key) DO NOTHING;
	`); err != nil {
		return err
	}

	return nil
}
