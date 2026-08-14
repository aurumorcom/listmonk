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
		{Base: models.Base{ID: 1}, EmailsPerDay: 100, EmailsToday: 90}, // Remaining: 10
		{Base: models.Base{ID: 2}, EmailsPerDay: 100, EmailsToday: 50}, // Remaining: 50
		{Base: models.Base{ID: 3}, EmailsPerDay: 100, EmailsToday: 60}, // Remaining: 40
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
