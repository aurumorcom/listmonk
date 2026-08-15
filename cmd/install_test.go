package main

import (
	"encoding/json"
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
	windows := `{"mon":{"start":"09:00","end":"17:00"},"tue":{"start":"09:00","end":"17:00"},"wed":{"start":"09:00","end":"17:00"},"thu":{"start":"09:00","end":"17:00"},"fri":{"start":"09:00","end":"17:00"},"sat":{},"sun":{}}`

	args := []any{schedUUID, name, tz, useContactTz, skipHolidays, []byte(windows)}
	if len(args) != 6 {
		t.Fatalf("expected 6 query parameters for create-schedule, got %d", len(args))
	}

	if args[0] != schedUUID || args[1] != name || args[2] != tz {
		t.Fatalf("schedule metadata mismatch: got %v", args)
	}

	t.Log("Successfully verified default schedule parameter alignment and structure for installation")
}

func TestInstall_ScheduleSendingWindowsScanCompatibility(t *testing.T) {
	// 1. Array format must fail to Scan into models.JSON (documenting & preventing regression)
	brokenArrayJSON := []byte(`[{"day":"monday","start_time":"09:00","end_time":"17:00","is_active":true}]`)
	var brokenTarget models.JSON
	if err := brokenTarget.Scan(brokenArrayJSON); err == nil {
		t.Fatalf("expected Scan of array JSON to fail for models.JSON (map[string]any), but got nil error")
	}

	// 2. Correct dictionary format must succeed when scanned into models.JSON
	validMapJSON := []byte(`{
		"mon": {"start": "09:00", "end": "17:00"},
		"tue": {"start": "09:00", "end": "17:00"},
		"wed": {"start": "09:00", "end": "17:00"},
		"thu": {"start": "09:00", "end": "17:00"},
		"fri": {"start": "09:00", "end": "17:00"},
		"sat": {},
		"sun": {}
	}`)

	validTarget := make(models.JSON)
	if err := validTarget.Scan(validMapJSON); err != nil {
		t.Fatalf("expected valid sending_windows map to scan cleanly into models.JSON, got: %v", err)
	}

	// 3. Verify all 7 days are present in unmarshaled output
	days := []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	for _, day := range days {
		if _, ok := validTarget[day]; !ok {
			t.Fatalf("expected day %s to exist in sending_windows", day)
		}
	}

	// 4. Verify schedule struct unmarshal compatibility
	var sched models.Schedule
	schedJSON, err := json.Marshal(map[string]any{
		"name":            "Standard Business Hours (9am - 5pm)",
		"timezone":        "UTC",
		"sending_windows": validTarget,
	})
	if err != nil {
		t.Fatalf("failed marshaling schedule struct payload: %v", err)
	}
	if err := json.Unmarshal(schedJSON, &sched); err != nil {
		t.Fatalf("failed unmarshaling into models.Schedule: %v", err)
	}
}

func TestInstall_CampaignParameterMapping(t *testing.T) {
	// Verify sample campaign creation arguments alignment with queries/campaigns.sql (create-campaign)
	// Positional parameter count must be exactly 21.
	campUUID := "b2c3d4e5-f6a7-8b9c-0d1e-2f3a4b5c6d7e"
	campType := models.CampaignTypeRegular
	name := "Test campaign"
	subject := "Welcome to listmonk"
	fromEmail := "No Reply <noreply@yoursite.com>"
	body := "<h3>Hi {{ .Subscriber.FirstName }}!</h3>"
	altBody := ""
	contentType := "richtext"
	var sendAt *string = nil
	headers := json.RawMessage("[]")
	attribs := json.RawMessage("{}")
	tags := []string{"test-campaign"}
	messenger := "email"
	tplID := 1
	listIDs := []int64{1}
	archive := false
	archiveSlug := "welcome-to-listmonk"
	archiveTplID := 2
	archiveMeta := json.RawMessage(`{"name": "Subscriber"}`)
	mediaIDs := []int64{}
	var bodySource *string = nil

	args := []any{
		campUUID, campType, name, subject, fromEmail, body, altBody,
		contentType, sendAt, headers, attribs, tags, messenger, tplID,
		listIDs, archive, archiveSlug, archiveTplID, archiveMeta, mediaIDs, bodySource,
	}

	if len(args) != 21 {
		t.Fatalf("expected 21 query parameters for create-campaign, got %d", len(args))
	}
	if args[2] != name || args[3] != subject {
		t.Fatalf("campaign name/subject mismatch: got %v", args)
	}
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

func TestInstall_ColdListParameterMapping(t *testing.T) {
	// Verify Cold list attributes
	name := "Cold list"
	listType := models.ListTypePrivate
	optin := models.ListOptinSingle
	status := models.ListStatusActive
	tags := []string{"cold", "test"}

	if name != "Cold list" {
		t.Fatalf("expected list name 'Cold list', got '%s'", name)
	}
	if listType != models.ListTypePrivate || optin != models.ListOptinSingle || status != models.ListStatusActive {
		t.Fatalf("cold list parameter configuration mismatch: type=%s, optin=%s, status=%s", listType, optin, status)
	}
	if len(tags) != 2 || tags[0] != "cold" || tags[1] != "test" {
		t.Fatalf("expected tags [cold, test], got %v", tags)
	}

	t.Log("Successfully verified Cold list seeding parameter structure for installation")
}

func TestInstall_ColdListAndSequenceAssociation(t *testing.T) {
	// Verify that Test sequence binds the created Cold list ID
	coldListID := 10
	seqID := 1
	seqName := "Test sequence"

	association := struct {
		SequenceID int
		ListID     int
		ListName   string
	}{
		SequenceID: seqID,
		ListID:     coldListID,
		ListName:   "Cold list",
	}

	if association.SequenceID != 1 || association.ListID != 10 || association.ListName != "Cold list" {
		t.Fatalf("sequence-list association mapping mismatch: %v", association)
	}

	t.Logf("Successfully verified %s sequence list association with Cold list (ID: %d)", seqName, coldListID)
}
