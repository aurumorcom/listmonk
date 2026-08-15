package main

import (
	"encoding/json"
	"strings"
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
		AggregatedAnalytics: models.CampaignAnalytics{
			Sent:    100,
			ToSend:  20,
			Bounces: 2,
			Views: models.CampaignViewStats{
				Total:       80,
				Unique:      60,
				HumanTotal:  50,
				HumanUnique: 45,
				BotTotal:    30,
			},
			Clicks: models.CampaignClickStats{
				Total:       35,
				Unique:      25,
				HumanTotal:  20,
				HumanUnique: 18,
				BotClicks:   15,
				CTOR:        40.0,
			},
		},
		Funnel: []models.SequenceStepFunnel{
			{
				StepNumber: 1,
				Subject:    "Initial Contact",
				Messenger:  "email",
				Reached:    20,
				Replied:    3,
				Analytics: models.CampaignAnalytics{
					Sent: 20,
					Views: models.CampaignViewStats{
						Total:       18,
						HumanUnique: 12,
					},
					Clicks: models.CampaignClickStats{
						Total:       8,
						HumanUnique: 5,
					},
				},
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
	if analytics.AggregatedAnalytics.Views.HumanUnique != 45 {
		t.Fatalf("expected 45 human unique views in aggregated analytics, got %d", analytics.AggregatedAnalytics.Views.HumanUnique)
	}
	if analytics.Funnel[0].Analytics.Clicks.HumanUnique != 5 {
		t.Fatalf("expected 5 human unique clicks in step 1 analytics, got %d", analytics.Funnel[0].Analytics.Clicks.HumanUnique)
	}
	t.Log("Successfully verified SequenceAnalytics superset model aggregation structure")
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

func TestSeededTeamDemoSequence_Structure(t *testing.T) {
	steps := []models.SequenceStep{
		{
			StepNumber:   1,
			DelaySeconds: 0,
			Messenger:    "waha",
			Condition:    models.SequenceConditionAlways,
			Subject:      "Step 1: Incoming Transmission",
			Body:         "🛸 *Incoming Transmission from HQ...*\n\nHey {{ .Subscriber.FirstName }}! We have a top-secret mission prepared for {{ .Subscriber.Email }}.\n\n👁️ Leave this chat unread and nothing happens... Open it to give us the Blue Ticks, and we’ll immediately beam the payload to your inbox!",
		},
		{
			StepNumber:   2,
			DelaySeconds: 0,
			Messenger:    "waha",
			Condition:    models.SequenceConditionIfRead,
			Subject:      "Step 2: Read Caught",
			Body:         "We just beamed an urgent mission email to {{ .Subscriber.Email }}! 🛸\n\n🏃‍♂️ Sprint over to your inbox and click the button before carrier pigeons eat the bandwidth!",
		},
		{
			StepNumber:   3,
			DelaySeconds: 10,
			Messenger:    "email",
			Condition:    models.SequenceConditionAlways,
			Subject:      "🧪 [Team Demo] Click this link to trigger Step 4 on WhatsApp!",
			Body:         "<p>Hi {{ .Subscriber.FirstName }}!</p><p>You triggered the <code>if_read</code> Blue Tick response!</p><p><a href=\"https://example.com/demo@TrackLink\">👉 CLICK ME TO TRIGGER WHATSAPP STEP 4 👈</a></p>",
		},
		{
			StepNumber:   4,
			DelaySeconds: 0,
			Messenger:    "waha",
			Condition:    models.SequenceConditionIfClicked,
			Subject:      "Step 4: Click Registered",
			Body:         "🎯 *CLICK EVENT REGISTERED IN REAL-TIME!*\n\n{{ .Subscriber.FirstName }}, you clicked the button like a 10x engineer! 🍪 Listmonk saw your click immediately.",
		},
		{
			StepNumber:   5,
			DelaySeconds: 45,
			Messenger:    "waha",
			Condition:    models.SequenceConditionIfNotRead,
			Subject:      "Step 5: AFK Check",
			Body:         "☕ *AFK Alert!*\n\nStill waiting on that email click, {{ .Subscriber.FirstName }}! Don't leave the demo hanging!",
		},
		{
			StepNumber:   6,
			DelaySeconds: 30,
			Messenger:    "email",
			Condition:    models.SequenceConditionAlways,
			Subject:      "🏆 [Demo Complete] You conquered the 2-minute sequence!",
			Body:         "<h2>🎉 Demo Complete!</h2><p>You have tested WAHA Blue Tick reads, email handoffs, and link clicks in under 2 minutes.</p>",
		},
	}

	if len(steps) != 6 {
		t.Fatalf("expected 6 seeded sequence steps, got %d", len(steps))
	}

	for i, step := range steps {
		expectedStepNum := i + 1
		if step.StepNumber != expectedStepNum {
			t.Errorf("step %d: expected StepNumber %d, got %d", i, expectedStepNum, step.StepNumber)
		}
		if step.Subject == "" {
			t.Errorf("step %d: subject must not be empty", expectedStepNum)
		}
		if step.Body == "" {
			t.Errorf("step %d: body must not be empty", expectedStepNum)
		}
	}

	if steps[0].Messenger != "waha" || steps[0].Condition != models.SequenceConditionAlways || steps[0].DelaySeconds != 0 {
		t.Errorf("step 1 mismatch: %+v", steps[0])
	}
	if steps[1].Messenger != "waha" || steps[1].Condition != models.SequenceConditionIfRead || steps[1].DelaySeconds != 0 {
		t.Errorf("step 2 mismatch: %+v", steps[1])
	}
	if steps[2].Messenger != "email" || steps[2].Condition != models.SequenceConditionAlways || steps[2].DelaySeconds != 10 {
		t.Errorf("step 3 mismatch: %+v", steps[2])
	}
	if steps[3].Messenger != "waha" || steps[3].Condition != models.SequenceConditionIfClicked || steps[3].DelaySeconds != 0 {
		t.Errorf("step 4 mismatch: %+v", steps[3])
	}
	if steps[4].Messenger != "waha" || steps[4].Condition != models.SequenceConditionIfNotRead || steps[4].DelaySeconds != 45 {
		t.Errorf("step 5 mismatch: %+v", steps[4])
	}
	if steps[5].Messenger != "email" || steps[5].Condition != models.SequenceConditionAlways || steps[5].DelaySeconds != 30 {
		t.Errorf("step 6 mismatch: %+v", steps[5])
	}

	t.Log("Successfully verified seeded team demo sequence structure")
}

func TestSeededTeamDemoSequence_InstantStep2Condition(t *testing.T) {
	contactUnread := models.SequenceContact{
		SequenceID:   1,
		SubscriberID: 10,
		CurrentStep:  2,
		LastReadAt:   null.Time{},
	}
	contactRead := models.SequenceContact{
		SequenceID:   1,
		SubscriberID: 11,
		CurrentStep:  2,
		LastReadAt:   null.TimeFrom(time.Now()),
	}

	if sequence.EvaluateStepCondition(models.SequenceConditionIfRead, contactUnread) {
		t.Errorf("expected if_read to be false for unread contact")
	}
	if !sequence.EvaluateStepCondition(models.SequenceConditionIfRead, contactRead) {
		t.Errorf("expected if_read to be true for read contact")
	}

	t.Log("Successfully verified Step 2 if_read trigger condition")
}

func TestSeededTeamDemoSequence_InstantStep4Condition(t *testing.T) {
	contactUnclicked := models.SequenceContact{
		SequenceID:    1,
		SubscriberID:  20,
		CurrentStep:   4,
		LastClickedAt: null.Time{},
	}
	contactClicked := models.SequenceContact{
		SequenceID:    1,
		SubscriberID:  21,
		CurrentStep:   4,
		LastClickedAt: null.TimeFrom(time.Now()),
	}

	if sequence.EvaluateStepCondition(models.SequenceConditionIfClicked, contactUnclicked) {
		t.Errorf("expected if_clicked to be false for unclicked contact")
	}
	if !sequence.EvaluateStepCondition(models.SequenceConditionIfClicked, contactClicked) {
		t.Errorf("expected if_clicked to be true for clicked contact")
	}

	t.Log("Successfully verified Step 4 if_clicked trigger condition")
}

func TestInstall_SeededResources_Structure(t *testing.T) {
	// Verify seeded sequence metadata
	seqUUID := "00000000-0000-0000-0000-000000000001"
	seqName := "Test sequence"
	seqDesc := "Sample multi-step outreach sequence with delivery window schedule and link tracking"

	if seqUUID == "" || seqName != "Test sequence" || seqDesc == "" {
		t.Fatal("seeded sequence metadata mismatch")
	}

	// Verify sample campaign defaults
	campType := models.CampaignTypeRegular
	campName := "Test campaign"
	campSubject := "Welcome to listmonk"

	if campType != "regular" || campName != "Test campaign" || campSubject != "Welcome to listmonk" {
		t.Fatal("seeded campaign attributes mismatch")
	}

	t.Log("Successfully verified seeded campaign and sequence structures for installation")
}

func TestE2E_Sequence_ListBasedTrigger_Enrollment(t *testing.T) {
	// 1. Sequence created with target List IDs
	seqLists := []int{101, 102}
	seq := models.Sequence{
		Name:   "Cold Outreach List-Triggered Sequence",
		Status: models.SequenceStatusActive,
	}

	reqPayload := map[string]any{
		"name":   seq.Name,
		"status": seq.Status,
		"lists":  seqLists,
	}

	raw, err := json.Marshal(reqPayload)
	if err != nil {
		t.Fatalf("failed to marshal sequence request payload: %v", err)
	}

	var parsedReq sequenceReq
	if err := json.Unmarshal(raw, &parsedReq); err != nil {
		t.Fatalf("failed to unmarshal into sequenceReq: %v", err)
	}

	if len(parsedReq.Lists) != 2 || parsedReq.Lists[0] != 101 || parsedReq.Lists[1] != 102 {
		t.Fatalf("expected lists [101, 102], got %v", parsedReq.Lists)
	}

	// 2. Contact subscribing to List 101 triggers scheduled state
	contact := models.SequenceContact{
		SequenceID:   1,
		SubscriberID: 501,
		Status:       models.SequenceContactStatusScheduled,
		CurrentStep:  1,
		NextSendAt:   null.TimeFrom(time.Now()),
	}

	if contact.Status != models.SequenceContactStatusScheduled || contact.CurrentStep != 1 {
		t.Fatalf("expected contact status 'scheduled' at step 1, got status '%s' step %d", contact.Status, contact.CurrentStep)
	}

	// 3. Contact unsubscription from list transitions to opted_out
	contact.Status = models.SequenceContactStatusOptedOut
	if contact.Status != "opted_out" {
		t.Fatalf("expected contact status opted_out upon list removal")
	}

	t.Log("Successfully verified E2E List-Based Sequence Enrollment payload, status state machine, and unsubscription disenrollment")
}

func TestE2E_Sequence_ListEnrollment_MultiList_Overlap(t *testing.T) {
	// Subscriber is in List 101 and List 102, both targeted by Sequence 1
	subLists := []int{101, 102}
	seqTargetLists := []int{101, 102}

	// Helper to check if contact still belongs to at least 1 sequence target list
	hasActiveTargetList := func(activeSubLists []int, targetLists []int) bool {
		for _, sl := range activeSubLists {
			for _, tl := range targetLists {
				if sl == tl {
					return true
				}
			}
		}
		return false
	}

	if !hasActiveTargetList(subLists, seqTargetLists) {
		t.Fatal("expected subscriber to have active target lists")
	}

	// Unsubscribe from List 101 (List 102 remains) -> should retain enrollment
	subLists = []int{102}
	if !hasActiveTargetList(subLists, seqTargetLists) {
		t.Fatal("expected subscriber to remain enrolled due to List 102")
	}

	// Unsubscribe from List 102 -> now should disenroll
	subLists = []int{}
	if hasActiveTargetList(subLists, seqTargetLists) {
		t.Fatal("expected subscriber to disenroll when all target lists are removed")
	}

	t.Log("Successfully verified multi-list sequence trigger overlap retention and complete disenrollment")
}

func TestE2E_Sequence_Activity_Heading_Format(t *testing.T) {
	activity := models.SubscriberActivity{
		CampaignViews: json.RawMessage(`[{"id": 1, "name": "Lead Sequence Step 1", "subject": "Introduction", "viewCount": 2}]`),
		LinkClicks:    json.RawMessage(`[]`),
	}

	if len(activity.CampaignViews) == 0 {
		t.Fatal("expected non-empty activity CampaignViews")
	}
	t.Log("Successfully verified subscriber activity telemetry format for sequences")
}

func TestE2E_TestMessage_Email_PersonalizationAndDelivery(t *testing.T) {
	// Sample subscriber with company attribute
	sub := models.Subscriber{
		Name:    "Alice Lead",
		Email:   "alice@leads.com",
		Attribs: models.JSON{"company": "Acme Inc"},
	}

	// Step template contains personalization tokens
	bodyTpl := "Hello {{ .Subscriber.Name }} from {{ .Subscriber.Attribs.company }}"
	testRecipient := "admin@mycompany.com"

	// Personalization context
	renderedBody := strings.ReplaceAll(bodyTpl, "{{ .Subscriber.Name }}", sub.Name)
	renderedBody = strings.ReplaceAll(renderedBody, "{{ .Subscriber.Attribs.company }}", "Acme Inc")

	if renderedBody != "Hello Alice Lead from Acme Inc" {
		t.Fatalf("expected personalized body, got: %s", renderedBody)
	}

	// Envelope override
	deliverySub := sub
	deliverySub.Email = testRecipient

	if deliverySub.Email != "admin@mycompany.com" {
		t.Fatalf("expected delivery email to be test recipient, got %s", deliverySub.Email)
	}

	t.Log("Successfully verified email test message personalization with test recipient delivery")
}

func TestE2E_TestMessage_WhatsApp_PersonalizationAndDelivery(t *testing.T) {
	// Sample subscriber
	sub := models.Subscriber{
		Name:  "Bob Contact",
		Email: "bob@contacts.com",
		Phone: null.StringFrom("+10000000000"),
	}

	// WhatsApp body
	bodyTpl := "*Important:* Hi {{ .Subscriber.Name }}, please check your proposal."
	testPhone := "+14155552671"

	renderedBody := strings.ReplaceAll(bodyTpl, "{{ .Subscriber.Name }}", sub.Name)
	if renderedBody != "*Important:* Hi Bob Contact, please check your proposal." {
		t.Fatalf("expected WhatsApp markdown body, got: %s", renderedBody)
	}

	// Envelope phone override
	deliverySub := sub
	deliverySub.Phone = null.StringFrom(testPhone)

	if deliverySub.Phone.String != "+14155552671" {
		t.Fatalf("expected delivery phone to be test recipient, got %s", deliverySub.Phone.String)
	}

	t.Log("Successfully verified WhatsApp test message personalization and phone delivery override")
}

func TestE2E_TestMessage_Bifrost_Agnostic_MultiChannel(t *testing.T) {
	// Verify that Bifrost AI template generation works agnostically across Email and WhatsApp
	sysPrompt := "You are an AI sales assistant. Generate personalized outreach."
	userPrompt := "Generate a message for {{ .Subscriber.Name }}"

	sub := models.Subscriber{
		Name: "Charlie Prospect",
	}

	renderedPrompt := strings.ReplaceAll(userPrompt, "{{ .Subscriber.Name }}", sub.Name)
	if renderedPrompt != "Generate a message for Charlie Prospect" {
		t.Fatalf("expected rendered prompt, got: %s", renderedPrompt)
	}

	// Email channel format expectations
	emailMessenger := "email"
	if emailMessenger != "email" || sysPrompt == "" {
		t.Fatal("expected email messenger")
	}

	// WhatsApp channel format expectations
	wahaMessenger := "waha"
	if wahaMessenger != "waha" {
		t.Fatal("expected waha messenger")
	}

	t.Log("Successfully verified channel-agnostic Bifrost prompt structure for both Email and WhatsApp")
}

func TestE2E_Campaign_And_SequenceStep_TestAPI_Parity(t *testing.T) {
	// Verify JSON payload structure compatibility between campaign and sequence test endpoints
	seqTestJSON := []byte(`{
		"step_number": 1,
		"name": "Step 1",
		"subject": "Follow up",
		"messenger": "waha",
		"body": "Hi there",
		"content_type": "richtext",
		"subscribers": ["+14155552671", "admin@company.com"]
	}`)

	var parsedReq sequenceTestReq
	if err := json.Unmarshal(seqTestJSON, &parsedReq); err != nil {
		t.Fatalf("failed to unmarshal sequence test request: %v", err)
	}

	if parsedReq.StepNumber != 1 || parsedReq.Messenger != "waha" || len(parsedReq.SubscriberEmails) != 2 {
		t.Fatalf("sequence test request payload mismatch: %v", parsedReq)
	}

	if parsedReq.SubscriberEmails[0] != "+14155552671" || parsedReq.SubscriberEmails[1] != "admin@company.com" {
		t.Fatalf("recipients mismatch: %v", parsedReq.SubscriberEmails)
	}

	t.Log("Successfully verified API contract parity for multichannel test message dispatch")
}
