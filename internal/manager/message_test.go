//go:build unit || !integration

package manager

import (
	"fmt"
	"sync"
	"testing"

	"github.com/knadh/listmonk/models"
)

func TestScopeContextIsolationConcurrent(t *testing.T) {
	const count = 10
	var wg sync.WaitGroup

	for i := 1; i <= count; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			companyName := fmt.Sprintf("Company_%d", id)
			userName := fmt.Sprintf("User_%d", id)

			attribs := models.JSON{
				"context": map[string]any{"company": companyName},
				"user":    map[string]any{"name": userName},
			}

			sub := models.Subscriber{
				Base:    models.Base{ID: id},
				Email:   fmt.Sprintf("user%d@example.com", id),
				Attribs: attribs,
			}

			scope := ExtractTemplateScope(sub)

			ctxMap, ok := scope["Context"].(map[string]any)
			if !ok || ctxMap["company"] != companyName {
				t.Errorf("worker %d: expected company %s, got %v", id, companyName, scope["Context"])
			}

			userMap, ok := scope["User"].(map[string]any)
			if !ok || userMap["name"] != userName {
				t.Errorf("worker %d: expected user name %s, got %v", id, userName, scope["User"])
			}
		}(i)
	}

	wg.Wait()
}

func TestCampaignMessage_FromEmail_RFC5322_And_BareFormat(t *testing.T) {
	mgr := &Manager{
		cfg: Config{UnsubURL: "http://localhost:9000/unsub/%s/%s"},
	}

	// 1. RFC 5322 Display Name format
	campRFC := &models.Campaign{
		Subject:   "RFC 5322 Subject",
		FromEmail: "Marketing Lead <newsletter@brand.com>",
		Body:      "Hello world",
	}
	_ = campRFC.CompileTemplate(nil)
	sub := models.Subscriber{Email: "recipient@example.com"}

	msgRFC, err := mgr.NewCampaignMessage(campRFC, sub)
	if err != nil {
		t.Fatalf("failed to create campaign message: %v", err)
	}
	if msgRFC.from != "Marketing Lead <newsletter@brand.com>" {
		t.Errorf("expected from 'Marketing Lead <newsletter@brand.com>', got '%s'", msgRFC.from)
	}

	// 2. Bare format
	campBare := &models.Campaign{
		Subject:   "Bare Subject",
		FromEmail: "newsletter@brand.com",
		Body:      "Hello world",
	}
	_ = campBare.CompileTemplate(nil)
	msgBare, err := mgr.NewCampaignMessage(campBare, sub)
	if err != nil {
		t.Fatalf("failed to create bare campaign message: %v", err)
	}
	if msgBare.from != "newsletter@brand.com" {
		t.Errorf("expected from 'newsletter@brand.com', got '%s'", msgBare.from)
	}
}

func TestCampaignMessage_StandardTemplate_Compilation(t *testing.T) {
	mgr := &Manager{
		cfg: Config{UnsubURL: "http://localhost:9000/unsub/%s/%s"},
	}

	camp := &models.Campaign{
		Subject:   "Hello {{ .Subscriber.Name }}",
		FromEmail: "team@listmonk.app",
		Body:      "<p>Welcome {{ .Subscriber.Name }} at {{ .Subscriber.Attribs.company }}!</p>",
	}
	if err := camp.CompileTemplate(nil); err != nil {
		t.Fatalf("failed to compile template: %v", err)
	}

	sub := models.Subscriber{
		Name:    "Alice",
		Email:   "alice@example.com",
		Attribs: models.JSON{"company": "Acme Corp"},
	}

	msg, err := mgr.NewCampaignMessage(camp, sub)
	if err != nil {
		t.Fatalf("failed to create campaign message: %v", err)
	}

	if msg.Subject() != "Hello Alice" {
		t.Errorf("expected compiled subject 'Hello Alice', got '%s'", msg.Subject())
	}
	if string(msg.Body()) != "<p>Welcome Alice at Acme Corp!</p>" {
		t.Errorf("expected compiled body '<p>Welcome Alice at Acme Corp!</p>', got '%s'", string(msg.Body()))
	}
}

func TestCampaignMessage_OverrideTo_PreservesFromAndBody(t *testing.T) {
	mgr := &Manager{
		cfg: Config{UnsubURL: "http://localhost:9000/unsub/%s/%s"},
	}

	camp := &models.Campaign{
		Subject:   "Exclusive Deals",
		FromEmail: "Sales Dept <sales@listmonk.app>",
		Body:      "<p>Body Content</p>",
	}
	_ = camp.CompileTemplate(nil)

	sub := models.Subscriber{
		Email: "alice@original.com",
	}

	msg, err := mgr.NewCampaignMessage(camp, sub)
	if err != nil {
		t.Fatalf("failed to create message: %v", err)
	}

	msg.OverrideTo("tester@preview.com", "+14155552671")

	if msg.to != "tester@preview.com" {
		t.Errorf("expected overridden to 'tester@preview.com', got '%s'", msg.to)
	}
	if msg.toPhone != "+14155552671" {
		t.Errorf("expected overridden toPhone '+14155552671', got '%s'", msg.toPhone)
	}
	if msg.from != "Sales Dept <sales@listmonk.app>" {
		t.Errorf("expected preserved from 'Sales Dept <sales@listmonk.app>', got '%s'", msg.from)
	}
	if string(msg.Body()) != "<p>Body Content</p>" {
		t.Errorf("expected preserved body '<p>Body Content</p>', got '%s'", string(msg.Body()))
	}
}
