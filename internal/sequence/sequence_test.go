package sequence

import (
	"testing"
	"time"

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
