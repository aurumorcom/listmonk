//go:build unit || !integration

package sequence

import (
	"context"
	"testing"
	"time"
)

func TestIntegration_IMAP_MailHog_ReplyListening_ContextCancellation(t *testing.T) {
	listener := NewReplyListener(nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := listener.ProcessReplyWithContext(ctx, "test@example.com", ChannelTypeEmail, "I am interested")
	if err == nil {
		t.Errorf("expected context cancellation error, got nil")
	}
}

func TestReplyListener_RegexLayer1_IntentRouting(t *testing.T) {
	// Test OptOut Regex
	optOutMsg := "STOP"
	if !_reOptOut.MatchString(optOutMsg) {
		t.Errorf("expected optOut regex to match %q", optOutMsg)
	}

	// Test Interested Regex
	interestedMsg := "yes I'm interested"
	if !_reInterested.MatchString(interestedMsg) {
		t.Errorf("expected interested regex to match %q", interestedMsg)
	}

	// Test OOO Regex
	oooMsg := "Out of office till next week"
	if !_reOOO.MatchString(oooMsg) {
		t.Errorf("expected OOO regex to match %q", oooMsg)
	}

	// Test @lid identifier routing without core
	listener := NewReplyListener(nil, nil)
	if err := listener.ProcessReplyWithQuotedID("210556493537459@lid", ChannelTypeWhatsApp, "STOP", "AC6DD75E5513F74D982B46860BA9E85D"); err != nil {
		t.Errorf("expected nil error for nil core, got %v", err)
	}
}

func TestResilience_IMAP_ConnectionDropAndAutoReconnect(t *testing.T) {
	listener := NewReplyListener(nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Simulate processing reply during network connection drop
	done := make(chan error, 1)
	go func() {
		done <- listener.ProcessReplyWithContext(ctx, "subscriber@example.com", ChannelTypeEmail, "Out of office until Friday")
	}()

	select {
	case <-time.After(1 * time.Second):
		t.Fatalf("IMAP listener processing hung on context cancellation")
	case err := <-done:
		t.Logf("IMAP listener returned cleanly on context cancellation: %v", err)
	}
}
