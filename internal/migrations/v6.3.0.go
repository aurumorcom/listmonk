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

		-- 2. Subscriber Timezone
		ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS tz TEXT NOT NULL DEFAULT '';
		UPDATE subscribers SET tz = COALESCE(NULLIF(attribs->>'tz', ''), NULLIF(attribs->>'timezone', ''), '') WHERE tz = '';
		ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS crm_id TEXT NULL;
		CREATE INDEX IF NOT EXISTS idx_subs_crm_id ON subscribers(crm_id) WHERE crm_id IS NOT NULL;

		ALTER TABLE lists ADD COLUMN IF NOT EXISTS crm_id TEXT NULL;
		CREATE INDEX IF NOT EXISTS idx_lists_crm_id ON lists(crm_id) WHERE crm_id IS NOT NULL;

		-- 3. Unified Campaign Schema Updates
		ALTER TYPE campaign_type ADD VALUE IF NOT EXISTS 'sequence';

		-- Drop unreleased sequence tables if they exist
		DROP TABLE IF EXISTS sequence_step_media CASCADE;
		DROP TABLE IF EXISTS sequence_subscribers CASCADE;
		DROP TABLE IF EXISTS sequence_steps CASCADE;
		DROP TABLE IF EXISTS sequence_lists CASCADE;
		DROP TABLE IF EXISTS sequences CASCADE;

		-- Create Schedules table
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

		ALTER TABLE campaigns
			ADD COLUMN IF NOT EXISTS schedule_id INTEGER NULL REFERENCES schedules(id) ON DELETE SET NULL,
			ADD COLUMN IF NOT EXISTS send_window JSONB NOT NULL DEFAULT '{}',
			ADD COLUMN IF NOT EXISTS email_ids INTEGER[] NOT NULL DEFAULT '{}',
			ADD COLUMN IF NOT EXISTS waha_sessions TEXT[] NOT NULL DEFAULT '{}',
			ADD COLUMN IF NOT EXISTS user_ids INTEGER[] NOT NULL DEFAULT '{}';

		CREATE TABLE IF NOT EXISTS campaign_steps (
			id           SERIAL PRIMARY KEY,
			campaign_id  INTEGER NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
			step_number  INTEGER NOT NULL DEFAULT 1,
			delay        TEXT NOT NULL DEFAULT '0s',
			messenger    TEXT NOT NULL DEFAULT 'email',
			condition    TEXT NOT NULL DEFAULT 'always',
			subject      TEXT NOT NULL DEFAULT '',
			body         TEXT NOT NULL DEFAULT '',
			email_type   TEXT NOT NULL DEFAULT '',
			template_id  INTEGER NULL REFERENCES templates(id) ON DELETE SET NULL,
			created_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_camp_steps_camp_id ON campaign_steps(campaign_id);

		CREATE TABLE IF NOT EXISTS campaign_step_media (
			campaign_step_id INTEGER REFERENCES campaign_steps(id) ON DELETE CASCADE ON UPDATE CASCADE,
			media_id         INTEGER NULL REFERENCES media(id) ON DELETE SET NULL ON UPDATE CASCADE,
			filename         TEXT NOT NULL DEFAULT ''
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_campaign_step_media_id ON campaign_step_media (campaign_step_id, media_id);
		CREATE INDEX IF NOT EXISTS idx_campaign_step_media_step_id ON campaign_step_media(campaign_step_id);

		CREATE TABLE IF NOT EXISTS campaign_subscribers (
			campaign_id        INTEGER NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
			subscriber_id      INTEGER NOT NULL REFERENCES subscribers(id) ON DELETE CASCADE,
			email_id           INTEGER NULL REFERENCES emails(id) ON DELETE SET NULL,
			from_address       TEXT NULL,
			waha_session       TEXT NULL,
			user_id            INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
			status             TEXT NOT NULL DEFAULT 'scheduled',
			current_step       INTEGER NOT NULL DEFAULT 1,
			next_send_at       TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			last_read_at       TIMESTAMP WITH TIME ZONE NULL,
			last_clicked_at    TIMESTAMP WITH TIME ZONE NULL,
			last_message_id    TEXT NULL,
			last_thread_msg_id TEXT NULL,
			created_at         TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			PRIMARY KEY (campaign_id, subscriber_id)
		);
		ALTER TABLE campaign_subscribers ADD COLUMN IF NOT EXISTS user_id INTEGER NULL REFERENCES users(id) ON DELETE SET NULL;
		CREATE INDEX IF NOT EXISTS idx_camp_subscribers_next_send ON campaign_subscribers(status, next_send_at);
		CREATE INDEX IF NOT EXISTS idx_camp_subscribers_sender ON campaign_subscribers(campaign_id, email_id, waha_session);
		CREATE INDEX IF NOT EXISTS idx_camp_subscribers_user_id ON campaign_subscribers(campaign_id, user_id);

		-- 4. Templates
		ALTER TYPE template_type ADD VALUE IF NOT EXISTS 'prompt';
		ALTER TABLE templates ADD COLUMN IF NOT EXISTS parent_template_id INTEGER NULL REFERENCES templates(id) ON DELETE SET NULL;
		ALTER TABLE templates DROP COLUMN IF EXISTS system_prompt;

		-- 5. Users table
		ALTER TABLE users
			ADD COLUMN IF NOT EXISTS email_id INTEGER NULL REFERENCES emails(id) ON DELETE SET NULL,
			ADD COLUMN IF NOT EXISTS waha_session TEXT NULL,
			ADD COLUMN IF NOT EXISTS signature TEXT NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS phone TEXT NULL,
			ADD COLUMN IF NOT EXISTS crm_id TEXT NULL,
			ADD COLUMN IF NOT EXISTS attribs JSONB NOT NULL DEFAULT '{}';
		CREATE INDEX IF NOT EXISTS idx_users_channels ON users(email_id, waha_session);
		CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);
		CREATE INDEX IF NOT EXISTS idx_users_crm_id ON users(crm_id) WHERE crm_id IS NOT NULL;

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
			('webhooks', '[]'),
			('crm', '{"enabled": false, "base_url": "", "api_key": "", "api_secret": ""}')
		ON CONFLICT (key) DO NOTHING;
	`); err != nil {
		return err
	}

	return nil
}
