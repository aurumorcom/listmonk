package cadence

import (
	"testing"
	"time"

	"github.com/knadh/listmonk/models"
	null "gopkg.in/volatiletech/null.v6"
)

func TestEvaluateStepCondition(t *testing.T) {
	now := time.Now()

	subUnread := models.CadenceContact{
		LastReadAt:    null.Time{},
		LastClickedAt: null.Time{},
	}

	subRead := models.CadenceContact{
		LastReadAt:    null.TimeFrom(now),
		LastClickedAt: null.Time{},
	}

	subClicked := models.CadenceContact{
		LastReadAt:    null.TimeFrom(now),
		LastClickedAt: null.TimeFrom(now),
	}

	tests := []struct {
		name      string
		condition string
		sub       models.CadenceContact
		expected  bool
	}{
		{"Always - Unread", models.CadenceConditionAlways, subUnread, true},
		{"Always - Read", models.CadenceConditionAlways, subRead, true},
		{"IfRead - Unread", models.CadenceConditionIfRead, subUnread, false},
		{"IfRead - Read", models.CadenceConditionIfRead, subRead, true},
		{"IfNotRead - Unread", models.CadenceConditionIfNotRead, subUnread, true},
		{"IfNotRead - Read", models.CadenceConditionIfNotRead, subRead, false},
		{"IfClicked - Unclicked", models.CadenceConditionIfClicked, subRead, false},
		{"IfClicked - Clicked", models.CadenceConditionIfClicked, subClicked, true},
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
}

func TestCadenceStepMediaIDs(t *testing.T) {
	step := models.CadenceStep{
		ID:         1,
		CadenceID:  10,
		StepNumber: 1,
		MediaIDs:   []int64{101, 102},
	}

	if len(step.MediaIDs) != 2 || step.MediaIDs[0] != 101 || step.MediaIDs[1] != 102 {
		t.Errorf("unexpected MediaIDs: %v", step.MediaIDs)
	}
}
