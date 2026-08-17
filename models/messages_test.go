//go:build unit || !integration

package models

import (
	htmltpl "html/template"
	"testing"
	txttpl "text/template"
)

func TestTxMessage_Render_Exhaustive(t *testing.T) {
	// Setup compiled template
	htmlTpl, err := htmltpl.New(BaseTpl).Parse("<html><body><h1>Hello {{ .Subscriber.Name }}</h1><p>Order #{{ .Tx.Data.order_id }}</p></body></html>")
	if err != nil {
		t.Fatalf("failed compiling html template: %v", err)
	}

	subjTpl, err := txttpl.New(BaseTpl).Parse("Default Subject {{ .Subscriber.Name }}")
	if err != nil {
		t.Fatalf("failed compiling subject template: %v", err)
	}

	tpl := &Template{
		Name:       "Tx Template",
		Type:       TemplateTypeTx,
		Subject:    "Default Subject {{ .Subscriber.Name }}",
		Tpl:        htmlTpl,
		SubjectTpl: subjTpl,
	}

	sub := Subscriber{
		Name:  "Alice",
		Email: "alice@example.com",
	}

	// 1. Render with message-level templated subject and alt body
	msg1 := &TxMessage{
		Subject: "Custom Subject {{ .Subscriber.Name }}",
		AltBody: "Alt text for {{ .Subscriber.Name }}",
		Data: map[string]any{
			"order_id": 12345,
		},
	}

	funcs := txttpl.FuncMap{}
	if err := msg1.Render(sub, tpl, funcs); err != nil {
		t.Fatalf("unexpected error rendering TxMessage: %v", err)
	}

	if string(msg1.Body) == "" || !testing.Short() && len(msg1.Body) == 0 {
		t.Fatalf("expected non-empty Body")
	}
	if msg1.Subject != "Custom Subject Alice" {
		t.Fatalf("expected rendered subject 'Custom Subject Alice', got %q", msg1.Subject)
	}
	if msg1.AltBody != "Alt text for Alice" {
		t.Fatalf("expected rendered alt body 'Alt text for Alice', got %q", msg1.AltBody)
	}

	// 2. Render with fallback template subject
	msg2 := &TxMessage{
		Data: map[string]any{
			"order_id": 67890,
		},
	}
	if err := msg2.Render(sub, tpl, funcs); err != nil {
		t.Fatalf("unexpected error rendering TxMessage with template subject: %v", err)
	}
	if msg2.Subject != "Default Subject Alice" {
		t.Fatalf("expected rendered subject 'Default Subject Alice', got %q", msg2.Subject)
	}

	// 3. Render error branches
	msgBadAlt := &TxMessage{
		AltBody: "Alt {{ .Subscriber.Name | invalidFunc }}",
	}
	if err := msgBadAlt.Render(sub, tpl, funcs); err == nil {
		t.Fatalf("expected error rendering bad alt body")
	}

	msgBadSubj := &TxMessage{
		Subject: "Subject {{ .Subscriber.Name | invalidFunc }}",
	}
	if err := msgBadSubj.Render(sub, tpl, funcs); err == nil {
		t.Fatalf("expected error rendering bad subject")
	}
}
