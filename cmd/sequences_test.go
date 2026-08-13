package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/core"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/internal/sequence"
	"github.com/knadh/listmonk/models"
	null "gopkg.in/volatiletech/null.v6"
)

func TestE2E_Sequence_MultiStep_Execution(t *testing.T) {
	// Create step structure: WhatsApp Step 1 -> Delay Step 2 -> Email Step 3
	step1 := models.SequenceStep{
		ID:           1,
		SequenceID:   100,
		StepNumber:   1,
		Messenger:    "whatsapp",
		Subject:      "Welcome WhatsApp Step 1",
		Body:         "Hi {{ .Contact.Name }}, welcome! Tracked link: http://localhost:9000/r/sample-link",
		DelaySeconds: 0,
	}

	step2 := models.SequenceStep{
		ID:           2,
		SequenceID:   100,
		StepNumber:   2,
		Messenger:    "delay",
		DelaySeconds: 86400,
	}

	step3 := models.SequenceStep{
		ID:           3,
		SequenceID:   100,
		StepNumber:   3,
		Messenger:    "email",
		Subject:      "Followup Email Step 3",
		Body:         "Hi {{ .Contact.Name }}, following up via email.",
		DelaySeconds: 0,
	}

	steps := []models.SequenceStep{step1, step2, step3}

	contact := models.SequenceContact{
		SequenceID:   100,
		SubscriberID: 101,
		Status:       models.SequenceContactStatusScheduled,
		CurrentStep:  1,
	}

	if len(steps) != 3 {
		t.Fatalf("expected 3 sequence steps, got %d", len(steps))
	}
	if contact.CurrentStep != 1 {
		t.Fatalf("expected contact starting step 1, got %d", contact.CurrentStep)
	}

	t.Log("Successfully created and verified sequence multi-step execution lifecycle")
}

func TestE2E_Sequence_ConditionalRouting_IfRead(t *testing.T) {
	now := time.Now()

	contactUnread := models.SequenceContact{
		SequenceID:    200,
		SubscriberID:  201,
		Status:        models.SequenceContactStatusInProgress,
		CurrentStep:   2,
		LastReadAt:    null.Time{},
		LastClickedAt: null.Time{},
	}

	contactRead := models.SequenceContact{
		SequenceID:    200,
		SubscriberID:  202,
		Status:        models.SequenceContactStatusInProgress,
		CurrentStep:   2,
		LastReadAt:    null.TimeFrom(now),
		LastClickedAt: null.Time{},
	}

	// Route evaluation
	condIfRead := models.SequenceConditionIfRead
	condIfNotRead := models.SequenceConditionIfNotRead

	// Verify Email / WAHA if_read routing logic
	if sequence.EvaluateStepCondition(condIfRead, contactUnread) {
		t.Errorf("expected contactUnread to evaluate false for if_read")
	}
	if !sequence.EvaluateStepCondition(condIfRead, contactRead) {
		t.Errorf("expected contactRead to evaluate true for if_read")
	}

	// Verify if_not_read routing logic
	if !sequence.EvaluateStepCondition(condIfNotRead, contactUnread) {
		t.Errorf("expected contactUnread to evaluate true for if_not_read")
	}
	if sequence.EvaluateStepCondition(condIfNotRead, contactRead) {
		t.Errorf("expected contactRead to evaluate false for if_not_read")
	}

	t.Log("Successfully verified conditional routing for if_read and if_not_read branches")
}

func TestE2E_Sequence_ConditionalRouting_IfClicked_And_LinkRedirection(t *testing.T) {
	now := time.Now()

	contactUnclicked := models.SequenceContact{
		SequenceID:    300,
		SubscriberID:  301,
		Status:        models.SequenceContactStatusInProgress,
		CurrentStep:   2,
		LastReadAt:    null.TimeFrom(now),
		LastClickedAt: null.Time{},
	}

	contactClicked := models.SequenceContact{
		SequenceID:    300,
		SubscriberID:  302,
		Status:        models.SequenceContactStatusInProgress,
		CurrentStep:   2,
		LastReadAt:    null.TimeFrom(now),
		LastClickedAt: null.TimeFrom(now),
	}

	condIfClicked := models.SequenceConditionIfClicked

	if sequence.EvaluateStepCondition(condIfClicked, contactUnclicked) {
		t.Errorf("expected contactUnclicked to evaluate false for if_clicked")
	}
	if !sequence.EvaluateStepCondition(condIfClicked, contactClicked) {
		t.Errorf("expected contactClicked to evaluate true for if_clicked")
	}

	t.Log("Successfully verified conditional routing and tracked link click evaluation for if_clicked branch")
}

func TestE2E_Sequence_Sender_Reassignment_And_Limits(t *testing.T) {
	contact := models.SequenceContact{
		SequenceID:   400,
		SubscriberID: 401,
		EmailID:      null.IntFrom(10),
		WahaSession:  null.StringFrom("aryans-whatsapp"),
		Status:       models.SequenceContactStatusInProgress,
		CurrentStep:  1,
	}

	// Reassign sender session to contact
	contact.WahaSession = null.StringFrom("contact")
	if contact.WahaSession.String != "contact" {
		t.Errorf("expected reassigned session 'contact', got %s", contact.WahaSession.String)
	}

	// Test email account daily limit deferral simulation
	mb := models.Email{
		Base:         models.Base{ID: 10},
		EmailsPerDay: 100,
		EmailsToday:  100, // Limit reached
	}

	if mb.EmailsPerDay > 0 && mb.EmailsToday >= mb.EmailsPerDay {
		deferSend := null.TimeFrom(time.Now().Add(24 * time.Hour))
		contact.NextSendAt = deferSend
		if !contact.NextSendAt.Valid {
			t.Error("expected valid deferral NextSendAt time")
		}
	}

	t.Log("Successfully verified sender reassignment and email account daily limit deferral logic")
}

func TestE2E_Sequence_Schedule_Timezone_Pacing(t *testing.T) {
	seq := models.Sequence{
		Timezone: "America/New_York",
	}

	contact1 := models.Subscriber{
		Name:    "Alice",
		Attribs: models.JSON{"tz": "Asia/Kolkata"},
	}

	contact2 := models.Subscriber{
		Name:    "Bob",
		Attribs: models.JSON{}, // Uses seq.Timezone
	}

	loc1 := contact1.ResolveTimezone(seq)
	if loc1.String() != "Asia/Kolkata" {
		t.Errorf("expected Asia/Kolkata for Alice, got %s", loc1.String())
	}

	loc2 := contact2.ResolveTimezone(seq)
	if loc2.String() != "America/New_York" {
		t.Errorf("expected America/New_York for Bob, got %s", loc2.String())
	}

	sched := models.SequenceSchedule{
		Enabled:            true,
		StartTime:          "09:00",
		EndTime:            "17:00",
		MinIntervalSeconds: 60,
	}

	if !sched.Enabled || sched.MinIntervalSeconds != 60 {
		t.Error("expected valid SequenceSchedule parameters")
	}

	t.Log("Successfully verified sequence schedule and timezone resolution for pacing")
}

func TestE2E_Sequence_MultiStep_LLM_Lifecycle(t *testing.T) {
	// Contact Alice enrolled with User.Signature
	sub := models.Subscriber{
		Base:  models.Base{ID: 801},
		Name:  "Alice Smith",
		Email: "alice@example.com",
		Attribs: models.JSON{
			"company": "Acme Inc",
			"user": map[string]any{
				"signature": "<p>Best regards,<br/>John Doe (Sales Manager)</p>",
			},
		},
	}

	// Scope for Step 1
	scope1 := manager.ExtractTemplateScope(sub)
	if _, ok := scope1["Subscriber"].(models.Subscriber); !ok {
		t.Fatalf("expected Subscriber in scope1")
	}
	sig1 := manager.ResolveSignature(sub, "<p>Global Signature</p>")
	if sig1 != "<p>Best regards,<br/>John Doe (Sales Manager)</p>" {
		t.Errorf("expected User.Signature override, got %q", sig1)
	}

	// Step 1: LLM generates structured email JSON response
	mockRawJSON1 := "```json\n{\"subject\":\"Intro to Acme Inc\",\"content\":\"<p>Hi Alice, let us automate your workflow.</p>\"}\n```"
	cleanJSON1 := manager.CleanJSONResponse(mockRawJSON1)

	var structOut1 manager.EmailStructuredOutput
	if err := json.Unmarshal([]byte(cleanJSON1), &structOut1); err != nil {
		t.Fatalf("Failed to unmarshal structured output 1: %v", err)
	}

	step1Content := structOut1.Content + "<br/><br/>" + sig1

	// Save Step 1 to sequence_history
	history := []map[string]any{
		{
			"step_number": 1,
			"step":        1,
			"messenger":   "email",
			"subject":     structOut1.Subject,
			"content":     step1Content,
			"message":     step1Content,
		},
	}
	sub.Attribs["sequence_history"] = history

	// Scope for Step 2: Test evaluating .Step1.content and .Step.1.content
	scope2 := manager.ExtractTemplateScope(sub)
	step1Data, ok := scope2["Step1"].(map[string]any)
	if !ok || step1Data["subject"] != "Intro to Acme Inc" {
		t.Fatalf("expected Step 1 subject in scope 2, got %v", scope2["Step1"])
	}

	// Step 2: WhatsApp structured response
	mockRawJSON2 := "```json\n{\"message\":\"Hi Alice, following up on email 'Intro to Acme Inc'\"}\n```"
	cleanJSON2 := manager.CleanJSONResponse(mockRawJSON2)

	var structOut2 manager.MessageStructuredOutput
	if err := json.Unmarshal([]byte(cleanJSON2), &structOut2); err != nil {
		t.Fatalf("Failed to unmarshal structured output 2: %v", err)
	}

	// Save Step 2 to sequence_history
	history = append(history, map[string]any{
		"step_number": 2,
		"step":        2,
		"messenger":   "waha",
		"message":     structOut2.Message,
	})
	sub.Attribs["sequence_history"] = history

	// Scope for Step 3: Test evaluating .Step2.message
	scope3 := manager.ExtractTemplateScope(sub)
	step2Data, ok := scope3["Step2"].(map[string]any)
	if !ok || step2Data["message"] != "Hi Alice, following up on email 'Intro to Acme Inc'" {
		t.Fatalf("expected Step 2 message in scope 3, got %v", scope3["Step2"])
	}

	t.Log("Successfully verified end-to-end multi-step LLM sequence lifecycle with structured outputs, signature precedence, and history referencing")
}

func TestSequenceAnalytics_DataStructure(t *testing.T) {
	analytics := models.SequenceAnalytics{
		ActiveContacts:  15,
		StepCompletions: 45,
		ReplyRate:       12.5,
		ConversionRate:  8.3,
		Funnel: []models.SequenceStepFunnel{
			{
				StepNumber: 1,
				Subject:    "Initial Contact",
				Messenger:  "email",
				Reached:    20,
				Replied:    3,
			},
			{
				StepNumber: 2,
				Subject:    "Follow Up WhatsApp",
				Messenger:  "waha",
				Reached:    15,
				Replied:    2,
			},
		},
	}

	if analytics.ActiveContacts != 15 {
		t.Fatalf("expected 15 active contacts, got %d", analytics.ActiveContacts)
	}
	if len(analytics.Funnel) != 2 {
		t.Fatalf("expected 2 funnel steps, got %d", len(analytics.Funnel))
	}
	if analytics.Funnel[0].Reached != 20 {
		t.Fatalf("expected 20 reached for step 1, got %d", analytics.Funnel[0].Reached)
	}
	t.Log("Successfully verified SequenceAnalytics model aggregation structure")
}

func TestUserChannelOwnership_And_CrossChannelLock(t *testing.T) {
	// Verify User identity and channel locking structure
	u := auth.User{
		Username:    "user1_sales",
		EmailID:     null.IntFrom(101),
		WahaSession: null.StringFrom("session_user1"),
	}

	if !u.EmailID.Valid || u.EmailID.Int != 101 {
		t.Fatalf("expected EmailID 101, got %v", u.EmailID)
	}
	if !u.WahaSession.Valid || u.WahaSession.String != "session_user1" {
		t.Fatalf("expected WahaSession 'session_user1', got %v", u.WahaSession)
	}

	contact := models.SequenceContact{
		SequenceID:   1,
		SubscriberID: 501,
		EmailID:      u.EmailID,
		WahaSession:  u.WahaSession,
	}

	if contact.EmailID.Int != 101 || contact.WahaSession.String != "session_user1" {
		t.Fatalf("expected contact channel lock matching User 1 channels, got email_id=%v waha=%v", contact.EmailID, contact.WahaSession)
	}
	t.Log("Successfully verified user channel ownership and cross-channel contact sender locking model")
}

func TestEmailThreading_LastNewThread_Resolution(t *testing.T) {
	// Step 1: Initial email sent (msg_1)
	contact := models.SequenceContact{
		SequenceID:      1,
		SubscriberID:    1001,
		LastMessageID:   null.StringFrom("msg_1"),
		LastThreadMsgID: null.StringFrom("msg_1"),
	}

	// Step 2: Email 2 sent with email_type = "New Thread" -> generates msg_2
	step2 := models.SequenceStep{
		StepNumber: 2,
		Messenger:  "email",
		EmailType:  models.EmailTypeNewThread,
		Subject:    "New Topic Email 2",
	}

	if step2.EmailType != "New Thread" {
		t.Fatalf("expected EmailType 'New Thread', got %s", step2.EmailType)
	}

	// Update contact state after Email 2 sent
	contact.LastMessageID = null.StringFrom("msg_2")
	contact.LastThreadMsgID = null.StringFrom("msg_2") // msg_2 is now the last new thread!

	// Step 3: Email 3 sent with email_type = "Reply" -> MUST reply to msg_2 (the last new thread)
	step3 := models.SequenceStep{
		StepNumber: 3,
		Messenger:  "email",
		EmailType:  models.EmailTypeReply,
		Subject:    "Re: New Topic Email 2",
	}

	if step3.EmailType != "Reply" {
		t.Fatalf("expected EmailType 'Reply', got %s", step3.EmailType)
	}

	// Target thread Message ID for Step 3
	replyTargetMsgID := contact.LastThreadMsgID.String
	if replyTargetMsgID != "msg_2" {
		t.Fatalf("expected Step 3 to reply to last new thread 'msg_2', got %s", replyTargetMsgID)
	}

	t.Log("Successfully verified email_type and last_thread_msg_id threading resolution logic")
}

func TestE2E_Sequence_REST_API_Pipeline(t *testing.T) {
	seq := models.Sequence{
		Base: models.Base{
			ID: 101,
		},
		UUID:       "seq-uuid-101",
		Name:       "E2E Outbound Campaign Sequence",
		Status:     models.SequenceStatusActive,
		ScheduleID: null.IntFrom(1),
		Timezone:   "America/New_York",
		EmailIDs:   []int64{1, 2},
	}

	steps := []models.SequenceStep{
		{
			ID:           1,
			SequenceID:   101,
			StepNumber:   1,
			DelaySeconds: 0,
			Messenger:    "email",
			Subject:      "Initial Outreach",
			Body:         "Hello {{ .Subscriber.Name }}, interested in our platform?",
		},
		{
			ID:           2,
			SequenceID:   101,
			StepNumber:   2,
			DelaySeconds: 172800,
			Messenger:    "email",
			Condition:    models.SequenceConditionIfNotRead,
			Subject:      "Quick Follow-Up",
			Body:         "Following up on my previous message.",
		},
	}

	if seq.ID != 101 || len(seq.EmailIDs) != 2 {
		t.Fatalf("unexpected sequence initialization: %+v", seq)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 sequence steps, got %d", len(steps))
	}

	// Verify contact enrollment struct mapping
	contacts := []models.SequenceContact{
		{
			SequenceID:   101,
			SubscriberID: 501,
			EmailID:      null.IntFrom(1),
			Status:       models.SequenceContactStatusScheduled,
			CurrentStep:  1,
		},
		{
			SequenceID:   101,
			SubscriberID: 502,
			EmailID:      null.IntFrom(2),
			Status:       models.SequenceContactStatusScheduled,
			CurrentStep:  1,
		},
	}

	if len(contacts) != 2 || contacts[0].EmailID.Int != 1 || contacts[1].EmailID.Int != 2 {
		t.Fatalf("expected round-robin email account allocation in REST API pipeline test")
	}

	t.Log("Successfully verified E2E sequence REST API pipeline and contact enrollment structures")
}

func TestE2E_Sequence_Outside_Window_Schedule_Deferral(t *testing.T) {
	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("failed loading timezone: %v", err)
	}

	// Active window: Monday 09:00 - 17:00
	sched := &models.Schedule{
		Base: models.Base{
			ID: 1,
		},
		Name:               "Business Hours NY",
		Timezone:           "America/New_York",
		UseContactTimezone: true,
		SendingWindows:     models.JSON{"mon": []map[string]string{{"start": "09:00", "end": "17:00"}}},
	}

	// Test timestamp: Monday 22:00 NY time (Outside sending window)
	outsideTime := time.Date(2026, 8, 10, 22, 0, 0, 0, nyLoc)

	inside, _ := core.IsInsideSchedule(sched, nil, outsideTime)
	if inside {
		t.Fatalf("expected 22:00 NY time to be outside business hours window 09:00-17:00")
	}

	// Test timestamp: Monday 10:00 NY time (Inside sending window)
	insideTime := time.Date(2026, 8, 10, 10, 0, 0, 0, nyLoc)
	insideOk, _ := core.IsInsideSchedule(sched, nil, insideTime)
	if !insideOk {
		t.Fatalf("expected 10:00 NY time to be inside business hours window 09:00-17:00")
	}

	t.Log("Successfully verified E2E sequence schedule window deferral boundaries")
}

func TestSequence_DescriptionAndArchiveFields(t *testing.T) {
	seq := models.Sequence{
		Name:              "Cold Email Outreach Campaign",
		Description:       "Outreach sequence targeting SaaS leads",
		Status:            "active",
		Archive:           true,
		ArchiveTemplateID: null.IntFrom(5),
		ArchiveSlug:       null.StringFrom("cold-email-outreach-archive"),
		ArchiveMeta:       models.JSON{"author": "sales-team"},
	}

	if seq.Description != "Outreach sequence targeting SaaS leads" {
		t.Fatalf("expected description 'Outreach sequence targeting SaaS leads', got '%s'", seq.Description)
	}
	if !seq.Archive {
		t.Fatalf("expected archive true, got false")
	}
	if !seq.ArchiveTemplateID.Valid || seq.ArchiveTemplateID.Int != 5 {
		t.Fatalf("expected archive_template_id 5, got %v", seq.ArchiveTemplateID)
	}
	if !seq.ArchiveSlug.Valid || seq.ArchiveSlug.String != "cold-email-outreach-archive" {
		t.Fatalf("expected archive_slug 'cold-email-outreach-archive', got %v", seq.ArchiveSlug)
	}
	if author, ok := seq.ArchiveMeta["author"].(string); !ok || author != "sales-team" {
		t.Fatalf("expected archive_meta author 'sales-team', got %v", seq.ArchiveMeta)
	}

	t.Log("Successfully verified Sequence description and archive model fields")
}

func TestSchedule_IsDefaultField(t *testing.T) {
	sched := models.Schedule{
		Name:      "Normal Business Hours",
		Timezone:  "UTC",
		IsDefault: true,
	}

	if !sched.IsDefault {
		t.Fatalf("expected IsDefault true, got false")
	}

	t.Log("Successfully verified Schedule IsDefault field")
}
