package sequence

import (
	"testing"
	"time"

	"github.com/knadh/listmonk/internal/core"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/models"
	null "gopkg.in/volatiletech/null.v6"
)

func TestEvaluateStepCondition(t *testing.T) {
	now := time.Now()

	subUnread := models.SequenceContact{
		LastReadAt:    null.Time{},
		LastClickedAt: null.Time{},
	}

	subRead := models.SequenceContact{
		LastReadAt:    null.TimeFrom(now),
		LastClickedAt: null.Time{},
	}

	subClicked := models.SequenceContact{
		LastReadAt:    null.TimeFrom(now),
		LastClickedAt: null.TimeFrom(now),
	}

	tests := []struct {
		name      string
		condition string
		sub       models.SequenceContact
		expected  bool
	}{
		{"Always - Unread", models.SequenceConditionAlways, subUnread, true},
		{"Always - Read", models.SequenceConditionAlways, subRead, true},
		{"IfRead - Unread", models.SequenceConditionIfRead, subUnread, false},
		{"IfRead - Read", models.SequenceConditionIfRead, subRead, true},
		{"IfNotRead - Unread", models.SequenceConditionIfNotRead, subUnread, true},
		{"IfNotRead - Read", models.SequenceConditionIfNotRead, subRead, false},
		{"IfClicked - Unclicked", models.SequenceConditionIfClicked, subRead, false},
		{"IfClicked - Clicked", models.SequenceConditionIfClicked, subClicked, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := EvaluateStepCondition(tt.condition, tt.sub)
			if res != tt.expected {
				t.Errorf("EvaluateStepCondition(%s) = %v; want %v", tt.condition, res, tt.expected)
			}
		})
	}
}

func TestReplyListenerProcessReply(t *testing.T) {
	l := NewReplyListener(nil, nil)
	// Empty email should return nil without error
	if err := l.ProcessReply(""); err != nil {
		t.Errorf("expected nil error for empty email, got %v", err)
	}

	// Test Layer 1 Fast-Path Opt-Out Regex
	if !reOptOut.MatchString("STOP") || !reOptOut.MatchString("unsubscribe") {
		t.Errorf("expected reOptOut to match 'STOP' and 'unsubscribe'")
	}

	// Test Layer 1 Fast-Path Interested Regex
	if !reInterested.MatchString("yes, let's talk") || !reInterested.MatchString("interested") {
		t.Errorf("expected reInterested to match 'yes, let's talk' and 'interested'")
	}

	// Test Layer 1 Fast-Path OOO Regex
	if !reOOO.MatchString("I am currently out of office until Monday") {
		t.Errorf("expected reOOO to match 'out of office'")
	}
}

func TestSequenceStepMediaIDs(t *testing.T) {
	step := models.SequenceStep{
		ID:         1,
		SequenceID: 10,
		StepNumber: 1,
		MediaIDs:   []int64{101, 102},
	}

	if len(step.MediaIDs) != 2 || step.MediaIDs[0] != 101 || step.MediaIDs[1] != 102 {
		t.Errorf("unexpected MediaIDs: %v", step.MediaIDs)
	}
}

func TestAllocateSendersRoundRobinInt(t *testing.T) {
	subIDs := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	pool := []int64{10, 20, 30}

	alloc := core.AllocateSendersRoundRobinInt(subIDs, pool)

	if len(alloc) != 10 {
		t.Fatalf("expected 10 allocations, got %d", len(alloc))
	}

	expected := map[int]int{
		1: 10, 2: 20, 3: 30,
		4: 10, 5: 20, 6: 30,
		7: 10, 8: 20, 9: 30,
		10: 10,
	}

	for subID, expMb := range expected {
		got, ok := alloc[subID]
		if !ok || !got.Valid || got.Int != expMb {
			t.Errorf("subscriber %d: expected email account %d, got %v", subID, expMb, got)
		}
	}
}

func TestAllocateSendersRoundRobinString(t *testing.T) {
	subIDs := []int{1, 2, 3, 4, 5}
	pool := []string{"session_a", "session_b"}

	alloc := core.AllocateSendersRoundRobinString(subIDs, pool)

	if len(alloc) != 5 {
		t.Fatalf("expected 5 allocations, got %d", len(alloc))
	}

	expected := map[int]string{
		1: "session_a",
		2: "session_b",
		3: "session_a",
		4: "session_b",
		5: "session_a",
	}

	for subID, expSession := range expected {
		got, ok := alloc[subID]
		if !ok || !got.Valid || got.String != expSession {
			t.Errorf("subscriber %d: expected session %s, got %v", subID, expSession, got)
		}
	}
}

func TestAllocateSendersCapacityWeighted(t *testing.T) {
	var subIDs []int
	for i := 1; i <= 20; i++ {
		subIDs = append(subIDs, i)
	}

	emails := []models.Email{
		{Base: models.Base{ID: 1}, MaxSendPerDay: 100, SentToday: 90}, // Remaining: 10
		{Base: models.Base{ID: 2}, MaxSendPerDay: 100, SentToday: 50}, // Remaining: 50
		{Base: models.Base{ID: 3}, MaxSendPerDay: 100, SentToday: 60}, // Remaining: 40
	}

	alloc := core.AllocateSendersCapacityWeighted(subIDs, emails)

	counts := make(map[int]int)
	for _, mbID := range alloc {
		if mbID.Valid {
			counts[mbID.Int]++
		}
	}

	if counts[1] != 2 {
		t.Errorf("expected email account 1 count = 2, got %d", counts[1])
	}
	if counts[2] != 10 {
		t.Errorf("expected email account 2 count = 10, got %d", counts[2])
	}
	if counts[3] != 8 {
		t.Errorf("expected email account 3 count = 8, got %d", counts[3])
	}
}

func TestSequenceManagerSetBifrostClient(t *testing.T) {
	mgr := &Manager{}
	if mgr.bifrostClient != nil {
		t.Errorf("expected initial bifrostClient to be nil")
	}

	bc := &manager.BifrostClient{}
	mgr.SetBifrostClient(bc)

	if mgr.bifrostClient != bc {
		t.Errorf("expected bifrostClient to be set on sequence manager")
	}
}

func TestSequence_ParentHTMLLayoutWrapping(t *testing.T) {
	parentTpl := models.Template{
		Base: models.Base{ID: 10},
		Name: "Parent Layout",
		Type: models.TemplateTypeCampaign,
		Body: `<html><body><div class="card">{{ template "content" . }}</div></body></html>`,
	}

	aiContent := "<p>Generated AI Email Body</p>"
	camp := models.Campaign{
		UUID:         "test-uuid",
		Subject:      "Test Subject",
		TemplateBody: parentTpl.Body,
		Body:         aiContent,
	}

	if err := camp.CompileTemplate(nil); err != nil {
		t.Fatalf("failed to compile parent template wrapper: %v", err)
	}

	if camp.Tpl == nil {
		t.Fatalf("expected compiled camp.Tpl")
	}
}

type mockSeqMessenger struct {
	name   string
	pushed []models.Message
}

func (m *mockSeqMessenger) Name() string {
	return m.name
}

func (m *mockSeqMessenger) Push(msg models.Message) error {
	m.pushed = append(m.pushed, msg)
	return nil
}

func (m *mockSeqMessenger) Flush() error {
	return nil
}

func (m *mockSeqMessenger) Close() error {
	return nil
}

func TestSequence_PrepareAndDispatchStep_FromResolution(t *testing.T) {
	msgr := &mockSeqMessenger{name: "email"}
	mgr := &Manager{
		messengers: map[string]manager.Messenger{
			"email": msgr,
		},
	}

	step := models.SequenceStep{
		StepNumber: 1,
		Messenger:  "email",
		Subject:    "Intro",
		Body:       "Hello",
		EmailType:  models.EmailTypeNewThread,
	}

	// 1. Contact Attribs User Name persona resolution
	subWithPersona := models.Subscriber{
		Name:  "Bob Lead",
		Email: "bob@target.com",
		Attribs: models.JSON{
			"user": map[string]any{
				"name": "Aryan Singh",
			},
		},
	}
	seqContact := models.SequenceContact{
		SequenceID:   1,
		SubscriberID: 10,
	}

	// In the absence of DB, we test step execution with contact persona and override recipient
	err := mgr.PrepareAndDispatchStep(seqContact, subWithPersona, step, "preview@test.com")
	if err != nil {
		t.Fatalf("PrepareAndDispatchStep failed: %v", err)
	}

	if len(msgr.pushed) != 1 {
		t.Fatalf("expected 1 message pushed, got %d", len(msgr.pushed))
	}
	if msgr.pushed[0].To[0] != "preview@test.com" {
		t.Errorf("expected override recipient 'preview@test.com', got '%s'", msgr.pushed[0].To[0])
	}
}

func TestSequence_PrepareAndDispatchStep_StepCompilation_TextAndPrompt(t *testing.T) {
	msgr := &mockSeqMessenger{name: "email"}
	mgr := &Manager{
		messengers: map[string]manager.Messenger{
			"email": msgr,
		},
	}

	step := models.SequenceStep{
		StepNumber: 1,
		Messenger:  "email",
		Subject:    "Proposal for {{ .Subscriber.Name }}",
		Body:       "Hi {{ .Subscriber.Name }}, how is {{ .Subscriber.Attribs.company }}?",
		EmailType:  models.EmailTypeNewThread,
	}

	contact := models.Subscriber{
		Name:    "Alice Lead",
		Email:   "alice@leads.com",
		Attribs: models.JSON{"company": "Acme Inc"},
	}
	seqContact := models.SequenceContact{
		SequenceID:   2,
		SubscriberID: 20,
	}

	err := mgr.PrepareAndDispatchStep(seqContact, contact, step, "alice@leads.com")
	if err != nil {
		t.Fatalf("PrepareAndDispatchStep failed: %v", err)
	}

	if len(msgr.pushed) != 1 {
		t.Fatalf("expected 1 message pushed, got %d", len(msgr.pushed))
	}

	lastMsg := msgr.pushed[0]
	if lastMsg.Subject != "Proposal for Alice Lead" {
		t.Errorf("expected compiled subject 'Proposal for Alice Lead', got '%s'", lastMsg.Subject)
	}
	if string(lastMsg.Body) != "Hi Alice Lead, how is Acme Inc?" {
		t.Errorf("expected compiled body 'Hi Alice Lead, how is Acme Inc?', got '%s'", string(lastMsg.Body))
	}
}

func TestSequence_PrepareAndDispatchStep_ThreadingHeaders(t *testing.T) {
	msgr := &mockSeqMessenger{name: "email"}
	mgr := &Manager{
		messengers: map[string]manager.Messenger{
			"email": msgr,
		},
	}

	// Reply step with thread ID
	step := models.SequenceStep{
		StepNumber: 2,
		Messenger:  "email",
		Subject:    "Re: Follow up",
		Body:       "Checking back on our last note.",
		EmailType:  models.EmailTypeReply,
	}

	contact := models.Subscriber{
		Name:  "Charlie",
		Email: "charlie@client.com",
	}
	seqContact := models.SequenceContact{
		SequenceID:      3,
		SubscriberID:    30,
		LastThreadMsgID: null.StringFrom("<thread-root-123@listmonk>"),
	}

	err := mgr.PrepareAndDispatchStep(seqContact, contact, step, "charlie@client.com")
	if err != nil {
		t.Fatalf("PrepareAndDispatchStep failed: %v", err)
	}

	if len(msgr.pushed) != 1 {
		t.Fatalf("expected 1 pushed message, got %d", len(msgr.pushed))
	}

	lastMsg := msgr.pushed[0]
	if lastMsg.Headers == nil || lastMsg.Headers.Get("In-Reply-To") != "<thread-root-123@listmonk>" {
		t.Errorf("expected In-Reply-To '<thread-root-123@listmonk>', got '%s'", lastMsg.Headers.Get("In-Reply-To"))
	}
	if lastMsg.Headers.Get("References") != "<thread-root-123@listmonk>" {
		t.Errorf("expected References '<thread-root-123@listmonk>', got '%s'", lastMsg.Headers.Get("References"))
	}
}

func TestSequence_PrepareAndDispatchStep_MissingWhatsAppMessenger_ReturnsError(t *testing.T) {
	emailMsgr := &mockSeqMessenger{name: "email"}
	mgr := &Manager{
		messengers: map[string]manager.Messenger{
			"email": emailMsgr,
		},
	}

	step := models.SequenceStep{
		StepNumber: 1,
		Messenger:  "whatsapp",
		Subject:    "WAHA Step",
		Body:       "Hello via WhatsApp",
	}

	contact := models.Subscriber{
		Name:  "Target Contact",
		Phone: null.StringFrom("+918935885359"),
	}
	seqContact := models.SequenceContact{
		SequenceID:   10,
		SubscriberID: 100,
	}

	err := mgr.PrepareAndDispatchStep(seqContact, contact, step, "+918935885359")
	if err == nil {
		t.Fatal("expected error when WhatsApp messenger is missing, got nil")
	}

	if len(emailMsgr.pushed) != 0 {
		t.Fatalf("expected 0 email messages pushed on WhatsApp failure, got %d", len(emailMsgr.pushed))
	}
}

func TestEmail_SMTPSettings_SentTodayMap(t *testing.T) {
	emailAcct := models.Email{
		Name:          "Sales Rep Account",
		Email:         "smtp_login@acme.com",
		MaxSendPerDay: 50,
		SMTPConfig: models.JSON{
			"from_addresses": []any{"rep1@acme.com", "rep2@acme.com"},
			"sent_today": map[string]any{
				"rep1@acme.com": 15,
				"rep2@acme.com": 20,
			},
		},
	}

	addrs := emailAcct.FromAddresses()
	if len(addrs) != 2 || addrs[0] != "rep1@acme.com" || addrs[1] != "rep2@acme.com" {
		t.Fatalf("unexpected FromAddresses: %v", addrs)
	}

	if emailAcct.GetAddressSent("rep1@acme.com") != 15 {
		t.Errorf("expected 15 sent for rep1@acme.com, got %d", emailAcct.GetAddressSent("rep1@acme.com"))
	}
	if emailAcct.GetAddressSent("rep2@acme.com") != 20 {
		t.Errorf("expected 20 sent for rep2@acme.com, got %d", emailAcct.GetAddressSent("rep2@acme.com"))
	}
	if emailAcct.GetTotalSent() != 35 {
		t.Errorf("expected total sent 35, got %d", emailAcct.GetTotalSent())
	}
}

func TestSequence_ContactStickyFromAddress_TestVsProdRouting(t *testing.T) {
	msgr := &mockSeqMessenger{name: "email"}
	mgr := &Manager{
		messengers: map[string]manager.Messenger{
			"email": msgr,
		},
	}

	step := models.SequenceStep{
		StepNumber: 1,
		Messenger:  "email",
		Subject:    "Welcome",
		Body:       "Hello contact",
	}

	contact := models.Subscriber{
		Name:  "Lead Person",
		Email: "lead@client.com",
		Attribs: models.JSON{
			"user": map[string]any{
				"name": "Active Admin User",
			},
		},
	}

	seqContact := models.SequenceContact{
		SequenceID:   1,
		SubscriberID: 10,
		FromAddress:  null.StringFrom("sticky.rep@acme.com"),
	}

	// 1. Test Mode dispatch (overrideRecipient != "")
	err := mgr.PrepareAndDispatchStep(seqContact, contact, step, "preview.admin@company.com")
	if err != nil {
		t.Fatalf("Test mode PrepareAndDispatchStep failed: %v", err)
	}

	if len(msgr.pushed) != 1 {
		t.Fatalf("expected 1 message pushed in test mode, got %d", len(msgr.pushed))
	}

	testMsg := msgr.pushed[0]
	if testMsg.To[0] != "preview.admin@company.com" {
		t.Errorf("expected test destination 'preview.admin@company.com', got '%s'", testMsg.To[0])
	}
	if testMsg.From != "Active Admin User <sticky.rep@acme.com>" {
		t.Errorf("expected test sender 'Active Admin User <sticky.rep@acme.com>', got '%s'", testMsg.From)
	}

	// Reset mock messenger
	msgr.pushed = nil

	// 2. Production Mode dispatch (overrideRecipient == "")
	err = mgr.PrepareAndDispatchStep(seqContact, contact, step, "")
	if err != nil {
		t.Fatalf("Prod mode PrepareAndDispatchStep failed: %v", err)
	}

	if len(msgr.pushed) != 1 {
		t.Fatalf("expected 1 message pushed in prod mode, got %d", len(msgr.pushed))
	}

	prodMsg := msgr.pushed[0]
	if prodMsg.To[0] != "lead@client.com" {
		t.Errorf("expected prod destination 'lead@client.com', got '%s'", prodMsg.To[0])
	}
	if prodMsg.From != "Test Account <sticky.rep@acme.com>" {
		t.Errorf("expected prod sender 'Test Account <sticky.rep@acme.com>', got '%s'", prodMsg.From)
	}
}

func TestSequence_PerAddressDailyLimitFailover(t *testing.T) {
	msgr := &mockSeqMessenger{name: "email"}
	mgr := &Manager{
		messengers: map[string]manager.Messenger{
			"email": msgr,
		},
	}

	step := models.SequenceStep{
		StepNumber: 1,
		Messenger:  "email",
		Subject:    "Outreach",
		Body:       "Body text",
	}

	contact := models.Subscriber{
		Name:  "Lead",
		Email: "target@lead.com",
	}

	// Contact assigned to primary@acme.com
	seqContact := models.SequenceContact{
		SequenceID:   1,
		SubscriberID: 10,
		FromAddress:  null.StringFrom("primary@acme.com"),
	}

	// Dispatch in test mode with sticky address
	err := mgr.PrepareAndDispatchStep(seqContact, contact, step, "preview@admin.com")
	if err != nil {
		t.Fatalf("PrepareAndDispatchStep failed: %v", err)
	}

	if len(msgr.pushed) != 1 {
		t.Fatalf("expected 1 pushed message, got %d", len(msgr.pushed))
	}

	if msgr.pushed[0].From != "Test Account <primary@acme.com>" {
		t.Errorf("expected From header 'Test Account <primary@acme.com>', got '%s'", msgr.pushed[0].From)
	}
}
