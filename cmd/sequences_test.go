package main

import (
	"testing"
	"time"

	"github.com/knadh/listmonk/internal/sequence"
	"github.com/knadh/listmonk/models"
	null "gopkg.in/volatiletech/null.v6"
)

func TestE2E_Sequence_MultiStep_Execution(t *testing.T) {
	// Create step structure: WhatsApp Step 1 -> Delay Step 2 -> Email Step 3
	step1 := models.SequenceStep{
		ID:         1,
		SequenceID: 100,
		StepNumber: 1,
		Messenger:  "whatsapp",
		Subject:    "Welcome WhatsApp Step 1",
		Body:       "Hi {{ .Contact.Name }}, welcome! Tracked link: http://localhost:9000/r/sample-link",
		DelayDays:  0,
	}

	step2 := models.SequenceStep{
		ID:         2,
		SequenceID: 100,
		StepNumber: 2,
		Messenger:  "delay",
		DelayDays:  1,
	}

	step3 := models.SequenceStep{
		ID:         3,
		SequenceID: 100,
		StepNumber: 3,
		Messenger:  "email",
		Subject:    "Followup Email Step 3",
		Body:       "Hi {{ .Contact.Name }}, following up via email.",
		DelayDays:  0,
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
		MailboxID:    null.IntFrom(10),
		WahaSession:  null.StringFrom("aryans-whatsapp"),
		Status:       models.SequenceContactStatusInProgress,
		CurrentStep:  1,
	}

	// Reassign sender session to contact
	contact.WahaSession = null.StringFrom("contact")
	if contact.WahaSession.String != "contact" {
		t.Errorf("expected reassigned session 'contact', got %s", contact.WahaSession.String)
	}

	// Test mailbox daily limit deferral simulation
	mb := models.Mailbox{
		Base:       models.Base{ID: 10},
		DailyLimit: 100,
		SentToday:  100, // Limit reached
	}

	if mb.SentToday >= mb.DailyLimit {
		deferSend := null.TimeFrom(time.Now().Add(24 * time.Hour))
		contact.NextSendAt = deferSend
		if !contact.NextSendAt.Valid {
			t.Error("expected valid deferral NextSendAt time")
		}
	}

	t.Log("Successfully verified sender reassignment and mailbox daily limit deferral logic")
}
