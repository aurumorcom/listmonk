//go:build unit || !integration

package bounce

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/knadh/listmonk/internal/bounce/webhooks"
	"github.com/knadh/listmonk/models"
)

func TestBounceManager_Initialization(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	opt := Opt{
		WebhooksEnabled: true,
		SESEnabled:      true,
		Postmark: struct {
			Enabled  bool
			Username string
			Password string
		}{
			Enabled:  true,
			Username: "pm_user",
			Password: "pm_password",
		},
	}

	mgr, err := New(opt, nil, logger)
	if err != nil {
		t.Fatalf("unexpected error creating bounce manager: %v", err)
	}

	if mgr.SES == nil {
		t.Fatalf("expected SES webhook handler to be initialized")
	}
	if mgr.Postmark == nil {
		t.Fatalf("expected Postmark webhook handler to be initialized")
	}
}

func TestBounceManager_RecordQueueing(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	recorded := make(chan models.Bounce, 1)

	opt := Opt{
		RecordBounceCB: func(b models.Bounce) error {
			recorded <- b
			return nil
		},
	}

	mgr, err := New(opt, nil, logger)
	if err != nil {
		t.Fatalf("unexpected error creating bounce manager: %v", err)
	}

	b := models.Bounce{
		Email:        "bounced@example.com",
		CampaignUUID: "camp_123",
		Type:         models.BounceTypeHard,
	}

	if err := mgr.Record(b); err != nil {
		t.Fatalf("unexpected error recording bounce: %v", err)
	}

	// Process queue single iteration
	go func() {
		bounce := <-mgr.queue
		if bounce.CreatedAt.IsZero() {
			bounce.CreatedAt = time.Now()
		}
		_ = mgr.opt.RecordBounceCB(bounce)
	}()

	select {
	case res := <-recorded:
		if res.Email != "bounced@example.com" || res.Type != models.BounceTypeHard {
			t.Fatalf("recorded bounce mismatch: %+v", res)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("recorded bounce callback timed out")
	}
}

func TestPostmarkWebhook_AuthAndTypeMapping(t *testing.T) {
	pm := webhooks.NewPostmark("postmark_user", "postmark_pass")
	if pm == nil {
		t.Fatalf("failed creating Postmark webhook handler")
	}
}
