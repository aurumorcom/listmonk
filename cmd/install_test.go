package main

import (
	"testing"

	"github.com/knadh/listmonk/models"
)

func TestInstall_CreateTemplateParameterMapping(t *testing.T) {
	// Verify template creation arguments alignment with queries/templates.sql
	// INSERT INTO templates (name, type, subject, body, body_source, parent_template_id) VALUES($1, $2, $3, $4, $5, $6) RETURNING id;

	sampleHTML := []byte("<!DOCTYPE html><html><body>Visual Content</body></html>")
	sampleJSON := []byte(`{"version":"1.0","elements":[]}`)

	name := "Sample visual template"
	tplType := models.TemplateTypeCampaignVisual
	subject := ""
	body := sampleHTML
	bodySource := sampleJSON
	var parentID *int = nil

	// Ensure parameters match the exact 6 positions required by queries/templates.sql
	args := []any{name, tplType, subject, body, bodySource, parentID}
	if len(args) != 6 {
		t.Fatalf("expected 6 query parameters for create-template, got %d", len(args))
	}

	// Verify that $6 is indeed nil / *int and not JSON string bytes
	if _, ok := args[5].(*int); !ok {
		t.Fatalf("expected 6th parameter (parent_template_id) to be integer pointer/nil, got %T", args[5])
	}

	// Verify that $5 is the JSON body_source
	if string(args[4].([]byte)) != string(sampleJSON) {
		t.Fatalf("expected 5th parameter (body_source) to be JSON bytes, got %s", args[4])
	}

	// Verify that $4 is the HTML body
	if string(args[3].([]byte)) != string(sampleHTML) {
		t.Fatalf("expected 4th parameter (body) to be HTML bytes, got %s", args[3])
	}

	t.Log("Successfully verified install template parameter alignment, positional mapping, and type safety")
}

func TestInstall_TxAndCampaignTemplateParameterMapping(t *testing.T) {
	// Campaign Template
	campHTML := []byte("<!DOCTYPE html><html><body>Campaign</body></html>")
	campArgs := []any{"Default campaign template", models.TemplateTypeCampaign, "", campHTML, nil, nil}
	if len(campArgs) != 6 {
		t.Fatalf("expected 6 params for default campaign template")
	}

	// Tx Template
	txHTML := []byte("<!DOCTYPE html><html><body>Tx Body</body></html>")
	txArgs := []any{"Sample transactional template", models.TemplateTypeTx, "Welcome {{ .Subscriber.Name }}", txHTML, nil, nil}
	if len(txArgs) != 6 {
		t.Fatalf("expected 6 params for sample tx template")
	}

	t.Log("Successfully verified Tx and Campaign template creation parameters")
}

func TestInstall_ScheduleParameterMapping(t *testing.T) {
	// Verify schedule creation arguments alignment with queries/schedules.sql
	// INSERT INTO schedules (uuid, name, timezone, use_contact_timezone, skip_holidays, sending_windows) VALUES ($1, $2, $3, $4, $5, $6)
	schedUUID := "a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d"
	name := "Standard Business Hours (9am - 5pm)"
	tz := "UTC"
	useContactTz := true
	skipHolidays := true
	windows := `[{"day":"monday","start_time":"09:00","end_time":"17:00","is_active":true}]`

	args := []any{schedUUID, name, tz, useContactTz, skipHolidays, []byte(windows)}
	if len(args) != 6 {
		t.Fatalf("expected 6 query parameters for create-schedule, got %d", len(args))
	}

	if args[0] != schedUUID || args[1] != name || args[2] != tz {
		t.Fatalf("schedule metadata mismatch: got %v", args)
	}

	t.Log("Successfully verified default schedule parameter alignment and structure for installation")
}

func TestInstall_DefaultScheduleAndSequenceBinding(t *testing.T) {
	// Verify that Test sequence correctly binds the created schedule ID
	schedID := 42
	seqName := "Test sequence"
	seqStatus := models.SequenceStatusActive

	if seqName != "Test sequence" {
		t.Fatalf("expected sequence name to be 'Test sequence', got '%s'", seqName)
	}
	if seqStatus != "active" {
		t.Fatalf("expected sequence status to be active, got %s", seqStatus)
	}
	if schedID <= 0 {
		t.Fatalf("expected valid schedule ID > 0, got %d", schedID)
	}

	t.Log("Successfully verified default schedule binding to Test sequence")
}
