package core

import (
	"testing"

	"github.com/knadh/listmonk/models"
)

func TestAllocateUsers(t *testing.T) {
	subIDs := []int{10, 20, 30}
	userIDs := []int64{1, 2}

	// First batch with 0 offset
	alloc1 := AllocateSendersRoundRobinInt(subIDs, userIDs, 0)
	if alloc1[10].Int != 1 || alloc1[20].Int != 2 || alloc1[30].Int != 1 {
		t.Errorf("unexpected allocation batch 1: %v", alloc1)
	}

	// Second batch (3 existing subscribers -> offset 3)
	subIDs2 := []int{40, 50, 60}
	alloc2 := AllocateSendersRoundRobinInt(subIDs2, userIDs, 3)
	// (3+0)%2 = 1 -> User 2
	// (3+1)%2 = 0 -> User 1
	// (3+2)%2 = 1 -> User 2
	if alloc2[40].Int != 2 || alloc2[50].Int != 1 || alloc2[60].Int != 2 {
		t.Errorf("unexpected additive allocation batch 2: %v", alloc2)
	}
}

func TestAllocateEmails(t *testing.T) {
	subIDs := []int{100, 200, 300}
	emails := []models.Email{
		{
			Base:          models.Base{ID: 1},
			MaxSendPerDay: 100,
		},
		{
			Base:          models.Base{ID: 2},
			MaxSendPerDay: 100,
		},
	}

	alloc := AllocateSendersCapacityWeighted(subIDs, emails)
	if len(alloc) != 3 {
		t.Fatalf("expected 3 allocations, got %d", len(alloc))
	}
	if !alloc[100].Valid || !alloc[200].Valid || !alloc[300].Valid {
		t.Errorf("expected all subscribers to receive email ID allocation, got %v", alloc)
	}
}

func TestAllocateWhatsApp(t *testing.T) {
	subIDs := []int{1, 2, 3, 4}
	waIDs := []string{"sess_1", "sess_2"}

	alloc := AllocateSendersRoundRobinString(subIDs, waIDs, 0)
	if alloc[1].String != "sess_1" || alloc[2].String != "sess_2" || alloc[3].String != "sess_1" || alloc[4].String != "sess_2" {
		t.Errorf("unexpected WhatsApp allocation: %v", alloc)
	}
}
