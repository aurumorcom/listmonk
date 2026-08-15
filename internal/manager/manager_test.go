package manager

import (
	"testing"

	"github.com/knadh/listmonk/models"
)

type dummyMessenger struct {
	name string
}

func (d *dummyMessenger) Name() string {
	return d.name
}

func (d *dummyMessenger) Push(m models.Message) error {
	return nil
}

func (d *dummyMessenger) Flush() error {
	return nil
}

func (d *dummyMessenger) Close() error {
	return nil
}

func TestResolveMessenger_PoolFallback(t *testing.T) {
	mgr := &Manager{
		messengers: make(map[string]Messenger),
	}

	// 1. Initially empty
	if _, err := mgr.resolveMessenger("whatsapp"); err == nil {
		t.Fatal("expected error for empty messenger map, got nil")
	}

	// 2. Register specific whatsapp account: "whatsapp-aquiveal"
	wahaMsgr := &dummyMessenger{name: "whatsapp-aquiveal"}
	mgr.messengers["whatsapp-aquiveal"] = wahaMsgr

	// Resolve "whatsapp" (pooled default) -> should resolve to "whatsapp-aquiveal"
	resolved, err := mgr.resolveMessenger("whatsapp")
	if err != nil {
		t.Fatalf("expected fallback to whatsapp-aquiveal, got err: %v", err)
	}
	if resolved.Name() != "whatsapp-aquiveal" {
		t.Fatalf("expected resolved name 'whatsapp-aquiveal', got '%s'", resolved.Name())
	}

	// Resolve "waha" -> should also fallback to "whatsapp-aquiveal"
	resolvedWaha, err := mgr.resolveMessenger("waha")
	if err != nil {
		t.Fatalf("expected fallback to whatsapp-aquiveal for 'waha', got err: %v", err)
	}
	if resolvedWaha.Name() != "whatsapp-aquiveal" {
		t.Fatalf("expected resolved name 'whatsapp-aquiveal', got '%s'", resolvedWaha.Name())
	}

	// Resolve exact match "whatsapp-aquiveal"
	resolvedExact, err := mgr.resolveMessenger("whatsapp-aquiveal")
	if err != nil {
		t.Fatalf("expected exact resolution for 'whatsapp-aquiveal', got err: %v", err)
	}
	if resolvedExact.Name() != "whatsapp-aquiveal" {
		t.Fatalf("expected resolved name 'whatsapp-aquiveal', got '%s'", resolvedExact.Name())
	}

	// 3. Register specific email account: "email-aquiveal"
	emailMsgr := &dummyMessenger{name: "email-aquiveal"}
	mgr.messengers["email-aquiveal"] = emailMsgr

	// Resolve "email" -> should fallback to "email-aquiveal"
	resolvedEmail, err := mgr.resolveMessenger("email")
	if err != nil {
		t.Fatalf("expected fallback to email-aquiveal, got err: %v", err)
	}
	if resolvedEmail.Name() != "email-aquiveal" {
		t.Fatalf("expected resolved name 'email-aquiveal', got '%s'", resolvedEmail.Name())
	}
}
