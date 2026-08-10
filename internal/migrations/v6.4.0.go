package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V6_4_0 performs DB migrations for v6.4.0 (Prompt Template & Bifrost AI support).
func V6_4_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	// Add 'prompt' value to template_type enum if not already present.
	if _, err := db.Exec(`ALTER TYPE template_type ADD VALUE IF NOT EXISTS 'prompt'`); err != nil {
		return err
	}

	// Add system_prompt column to templates table.
	if _, err := db.Exec(`ALTER TABLE templates ADD COLUMN IF NOT EXISTS system_prompt TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}

	return nil
}
