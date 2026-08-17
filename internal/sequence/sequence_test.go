package sequence

import (
	"bytes"
	"encoding/json"
	htmltpl "html/template"
	"strings"
	"testing"
	txttpl "text/template"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/core"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/internal/testutil"
	"github.com/knadh/listmonk/models"
	null "gopkg.in/volatiletech/null.v6"
)

func TestGetSequenceManager(t *testing.T) {
	m1 := GetSequenceManager(nil, nil, nil, nil)
	if m1 == nil {
		t.Fatalf("expected non-nil SequenceManager instance")
	}

	m2 := GetSequenceManager(nil, nil, nil, nil)
	if m2 == nil {
		t.Fatalf("expected non-nil SequenceManager instance on second call")
	}

	if m1 != m2 {
		t.Fatalf("expected GetSequenceManager to return identical singleton pointer")
	}
}

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
	if prodMsg.From != "Active Admin User <sticky.rep@acme.com>" {
		t.Errorf("expected prod sender 'Active Admin User <sticky.rep@acme.com>', got '%s'", prodMsg.From)
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

	if msgr.pushed[0].From != "primary@acme.com" {
		t.Errorf("expected From header 'primary@acme.com', got '%s'", msgr.pushed[0].From)
	}
}

func TestSequence_TemplateScope_Interpolation(t *testing.T) {
	msgr := &mockSeqMessenger{name: "email"}
	mgr := NewManager(nil, map[string]manager.Messenger{"email": msgr}, nil, nil)

	sub := models.Subscriber{
		UUID:  "sub-uuid-123",
		Name:  "Alice Smith",
		Email: "alice@example.com",
		Phone: null.StringFrom("+14155552672"),
	}

	step := models.SequenceStep{
		ID:         1,
		StepNumber: 1,
		Messenger:  "email",
		Subject:    "Welcome {{ .Subscriber.FirstName }}",
		Body:       "Hey {{ .Subscriber.FirstName }}! Email: {{ .Subscriber.Email }}, Phone: {{ .contact.phone }}",
	}

	seqContact := models.SequenceContact{
		SequenceID:   1,
		SubscriberID: 10,
	}

	err := mgr.PrepareAndDispatchStep(seqContact, sub, step, "alice@example.com")
	if err != nil {
		t.Fatalf("PrepareAndDispatchStep failed: %v", err)
	}

	if len(msgr.pushed) != 1 {
		t.Fatalf("expected 1 message pushed, got %d", len(msgr.pushed))
	}

	msg := msgr.pushed[0]
	if msg.Subject != "Welcome Alice" {
		t.Errorf("expected Subject 'Welcome Alice', got '%s'", msg.Subject)
	}
	expectedBody := "Hey Alice! Email: alice@example.com, Phone: +14155552672"
	if string(msg.Body) != expectedBody {
		t.Errorf("expected Body '%s', got '%s'", expectedBody, string(msg.Body))
	}
}

func TestSequence_TrackLink_And_TrackView(t *testing.T) {
	mgr := NewManager(nil, nil, nil, nil)

	funcs := mgr.TemplateFuncsWithContext("seq-uuid-456", "sub-uuid-789")

	trackLinkFn, ok := funcs["TrackLink"].(func(string, ...any) string)
	if !ok {
		t.Fatal("expected TrackLink function in TemplateFuncs")
	}

	rawURL := "https://example.com/offer?a=1&amp;b=2"
	formatted := trackLinkFn(rawURL)
	if !strings.Contains(formatted, "/link/") || !strings.Contains(formatted, "seq-uuid-456") || !strings.Contains(formatted, "sub-uuid-789") {
		t.Errorf("expected TrackLink to output tracking URL with sequence and sub UUID, got '%s'", formatted)
	}

	trackViewFn, ok := funcs["TrackView"].(func(...any) htmltpl.HTML)
	if !ok {
		t.Fatal("expected TrackView function in TemplateFuncs")
	}

	pixelHTML := string(trackViewFn())
	if !strings.Contains(pixelHTML, "/campaign/seq-uuid-456/sub-uuid-789/px.png") {
		t.Errorf("expected TrackView to output pixel tracking image tag, got '%s'", pixelHTML)
	}
}

func TestE2E_Sequence_PromptTemplate_LiveLiteLLM(t *testing.T) {
	testutil.LoadDotEnv()
	endpoint := "https://litellm.aurumor.com"
	apiKey := "sk-LJ0gQPPGu5NLMQYMEpCMgw"

	rec, vcrClient := testutil.NewVCRRecorder(t, "sequences/bifrost_litellm_prompt_template")
	if rec != nil {
		defer rec.Stop()
	}

	bc := manager.NewBifrostClient(manager.BifrostConfig{
		APIKey:   apiKey,
		Endpoint: endpoint,
		Model:    "gpt-4o-mini",
		Timeout:  15 * time.Second,
	})
	bc.SetHTTPClient(vcrClient)

	sub := models.Subscriber{
		UUID:    "sub-uuid-llm-101",
		Name:    "Alice Prospect",
		Email:   "alice.prospect@acme.org",
		Attribs: models.JSON{"first_name": "Alice"},
	}

	scope := manager.ExtractTemplateScope(sub)

	ctx, cancel := bc.TimeoutContext()
	defer cancel()

	sysPrompt := "You are a professional AI sales assistant writing outreach emails. Output strict JSON with subject and content fields."
	userPrompt := "Invite {{ .Subscriber.FirstName }} at {{ .Subscriber.Email }} to a 15-minute product demo."

	// Compile user prompt template
	var userPromptStr string
	if ut, err := txttpl.New("user").Parse(userPrompt); err == nil {
		var ub bytes.Buffer
		if err := ut.Execute(&ub, scope); err == nil {
			userPromptStr = ub.String()
		}
	}
	if userPromptStr == "" {
		userPromptStr = userPrompt
	}

	rawAI, err := bc.GeneratePromptWithFormat(ctx, sysPrompt, userPromptStr, manager.EmailResponseFormat())
	if err != nil {
		t.Fatalf("Live LiteLLM AI prompt completion failed: %v", err)
	}

	cleanJSON := manager.CleanJSONResponse(rawAI)
	var emailOut manager.EmailStructuredOutput
	if err := json.Unmarshal([]byte(cleanJSON), &emailOut); err != nil || emailOut.Content == "" {
		t.Fatalf("failed unmarshaling AI response JSON '%s': %v", cleanJSON, err)
	}

	if strings.TrimSpace(emailOut.Subject) == "" || strings.TrimSpace(emailOut.Content) == "" {
		t.Fatalf("expected non-empty AI subject and content, got subject='%s', content='%s'", emailOut.Subject, emailOut.Content)
	}

	// 1. Without Parent Template Verification: Content is plain text with signature
	sig := "Best regards,\nSales Team"
	finalPlainText := manager.FormatPlainTextWithSignature(emailOut.Content, sig)

	if !strings.Contains(finalPlainText, sig) {
		t.Errorf("expected final plain text to contain signature, got: %s", finalPlainText)
	}
	if strings.Contains(finalPlainText, "<html>") || strings.Contains(finalPlainText, "<div class=\"wrap\">") {
		t.Errorf("expected plain text without parent template HTML wrapper, got: %s", finalPlainText)
	}

	// 2. With Parent Template Verification: Content is wrapped inside Parent HTML Template layout
	parentTpl := models.Template{
		Body: `<!doctype html><html><body><div class="wrap">{{ template "content" . }}</div></body></html>`,
	}

	camp := models.Campaign{
		UUID:         uuid.Must(uuid.NewV4()).String(),
		Subject:      emailOut.Subject,
		TemplateBody: parentTpl.Body,
		Body:         finalPlainText,
	}

	if err := camp.CompileTemplate(htmltpl.FuncMap{}); err != nil {
		t.Fatalf("failed compiling parent HTML template: %v", err)
	}

	var buf bytes.Buffer
	if err := camp.Tpl.ExecuteTemplate(&buf, models.BaseTpl, scope); err == nil {
		htmlOutput := buf.String()
		if !strings.Contains(htmlOutput, "<div class=\"wrap\">") || !strings.Contains(htmlOutput, "</body>") {
			t.Errorf("expected HTML output to contain parent template layout structure, got: %s", htmlOutput)
		}
	} else {
		t.Fatalf("failed executing parent HTML template: %v", err)
	}

	t.Log("Successfully verified live LiteLLM AI prompt completion both WITH and WITHOUT parent HTML template wrapping")
}

type mockUserStore struct {
	user auth.User
	err  error
}

func (m *mockUserStore) GetUser(id int, email, username string) (auth.User, error) {
	if m.err != nil {
		return auth.User{}, m.err
	}
	return m.user, nil
}

func TestResolveSenderDisplayName_Tier1_ContactAssignedUser(t *testing.T) {
	contact := models.Subscriber{
		Attribs: models.JSON{
			"user": map[string]any{
				"name": "Contact Agent",
			},
		},
	}
	email := &models.Email{
		UserID: null.IntFrom(10),
		Name:   "Account Name",
	}
	store := &mockUserStore{user: auth.User{Name: "Messenger User"}}

	displayName, _ := ResolveSenderDisplayName(contact, email, false, store)
	if displayName != "Contact Agent" {
		t.Errorf("expected Tier 1 Contact Agent, got '%s'", displayName)
	}
}

func TestResolveSenderDisplayName_Tier2_MessengerAssignedUser(t *testing.T) {
	contact := models.Subscriber{
		Attribs: models.JSON{},
	}
	email := &models.Email{
		UserID: null.IntFrom(10),
		Name:   "Company Email Account",
	}
	store := &mockUserStore{user: auth.User{Name: "Alex Rep"}}

	displayName, assignedUser := ResolveSenderDisplayName(contact, email, false, store)
	if displayName != "Alex Rep" {
		t.Errorf("expected Tier 2 Messenger User 'Alex Rep', got '%s'", displayName)
	}
	if assignedUser == nil || assignedUser.Name != "Alex Rep" {
		t.Errorf("expected assignedUser to be returned, got %+v", assignedUser)
	}
}

func TestResolveSenderDisplayName_Tier3_ActiveUser_TestOnly(t *testing.T) {
	contact := models.Subscriber{
		Attribs: models.JSON{
			"active_user": map[string]any{
				"name": "Active Admin",
			},
		},
	}
	email := &models.Email{
		UserID: null.Int{},
		Name:   "Company SMTP",
	}
	store := &mockUserStore{}

	testDisplayName, _ := ResolveSenderDisplayName(contact, email, true, store)
	if testDisplayName != "Active Admin" {
		t.Errorf("expected Tier 3 Active Admin in test mode, got '%s'", testDisplayName)
	}

	liveDisplayName, _ := ResolveSenderDisplayName(contact, email, false, store)
	if liveDisplayName != "" {
		t.Errorf("expected empty display name in live mode when no assigned user exists, got '%s'", liveDisplayName)
	}
}

func TestResolveSenderDisplayName_ZeroAccountNameFallback(t *testing.T) {
	contact := models.Subscriber{Attribs: models.JSON{}}
	email := &models.Email{
		UserID: null.Int{},
		Name:   "Sales Account Name",
	}
	store := &mockUserStore{}

	displayName, _ := ResolveSenderDisplayName(contact, email, false, store)
	if displayName != "" {
		t.Errorf("expected zero account name fallback (empty string), got '%s'", displayName)
	}
}

func TestFormatSenderFromHeader(t *testing.T) {
	if res := FormatSenderFromHeader("John Doe", "john@acme.com"); res != "John Doe <john@acme.com>" {
		t.Errorf("expected 'John Doe <john@acme.com>', got '%s'", res)
	}
	if res := FormatSenderFromHeader("", "john@acme.com"); res != "john@acme.com" {
		t.Errorf("expected 'john@acme.com', got '%s'", res)
	}
	if res := FormatSenderFromHeader("John Doe", ""); res != "" {
		t.Errorf("expected empty string for empty fromEmail, got '%s'", res)
	}
}

func TestResolveTargetRecipient_LiveVsTest(t *testing.T) {
	contact := models.Subscriber{
		Email: "lead@client.com",
		Phone: null.StringFrom("+15551234567"),
	}

	to, phone := ResolveTargetRecipient(contact, "", false)
	if len(to) != 1 || to[0] != "lead@client.com" || phone != "+15551234567" {
		t.Errorf("unexpected live mode recipient resolution: to=%v, phone=%s", to, phone)
	}

	toTest, _ := ResolveTargetRecipient(contact, "tester@company.com", false)
	if len(toTest) != 1 || toTest[0] != "tester@company.com" {
		t.Errorf("unexpected test mode email recipient resolution: to=%v", toTest)
	}

	_, phoneTest := ResolveTargetRecipient(contact, "+15559998888", true)
	if phoneTest != "+15559998888" {
		t.Errorf("unexpected test mode whatsapp recipient resolution: phone=%s", phoneTest)
	}
}
