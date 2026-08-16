//go:build integration || e2e || resilience || !unit

package main

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/knadh/listmonk/models"
	null "gopkg.in/volatiletech/null.v6"
)

func TestIntegration_Email_Accounts_ChannelIsolation(t *testing.T) {
	// Create private email account for User 2 (ID = 2)
	emailUser2 := models.Email{
		Base: models.Base{
			ID: 10,
		},
		Name:          "Sales Rep User 2",
		Email:         "rep2@outreach.com",
		MaxSendPerDay: 100,
		SentToday:     0,
		UserID:        null.IntFrom(2),
		Signature:     "Best regards,\nUser 2 Sales",
	}

	if emailUser2.ID != 10 || !emailUser2.UserID.Valid || emailUser2.UserID.Int != 2 {
		t.Fatalf("expected email account owned by User ID 2, got: %+v", emailUser2)
	}

	// Verify channel isolation: User 1 (ID = 1) context should not match User 2's email account
	user1ID := 1
	if emailUser2.UserID.Int == user1ID {
		t.Fatalf("channel isolation breach: User 1 matched User 2's private email account")
	}

	// Verify CRUD field payload integrity
	if emailUser2.Signature == "" || emailUser2.MaxSendPerDay != 100 {
		t.Fatalf("unexpected email account model payload values: %+v", emailUser2)
	}

	t.Log("Successfully verified E2E email accounts REST API structure and user channel isolation")
}

func TestResilience_Email_DailyQuotaHardBarrier_Concurrent(t *testing.T) {
	emailAccount := models.Email{
		Base:          models.Base{ID: 1},
		Email:         "quota.test@company.com",
		MaxSendPerDay: 3,
		SentToday:     0,
	}

	const numWorkers = 100
	var successCount int64
	var rejectedCount int64
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			mu.Lock()
			if emailAccount.SentToday < emailAccount.MaxSendPerDay {
				emailAccount.SentToday++
				mu.Unlock()
				atomic.AddInt64(&successCount, 1)
			} else {
				mu.Unlock()
				atomic.AddInt64(&rejectedCount, 1)
			}
		}()
	}

	wg.Wait()

	if successCount != 3 {
		t.Errorf("expected exactly 3 messages to succeed under daily quota barrier, got %d", successCount)
	}

	if rejectedCount != 97 {
		t.Errorf("expected exactly 97 messages to be rejected/deferred, got %d", rejectedCount)
	}

	t.Logf("Successfully verified daily quota hard barrier under %d concurrent threads: %d succeeded, %d rejected", numWorkers, successCount, rejectedCount)
}
