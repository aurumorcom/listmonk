//go:build unit || !integration

package models

import (
	"testing"
	"time"

	null "gopkg.in/volatiletech/null.v6"
)

func TestSubscriberPrimaryIdentifier(t *testing.T) {
	cEmail := Subscriber{
		Email: "user@example.com",
		Phone: null.StringFrom("+15559876543"),
	}
	if id := cEmail.PrimaryIdentifier(); id != "user@example.com" {
		t.Errorf("expected user@example.com, got %s", id)
	}

	cPhoneOnly := Subscriber{
		Email: "",
		Phone: null.StringFrom("+15559876543"),
	}
	if id := cPhoneOnly.PrimaryIdentifier(); id != "+15559876543" {
		t.Errorf("expected +15559876543, got %s", id)
	}
}

func TestSubscriberChannelAddress(t *testing.T) {
	c := Subscriber{
		Email: "user@example.com",
		Phone: null.StringFrom("+15559876543"),
	}

	if addr := c.ChannelAddress("whatsapp"); addr != "+15559876543" {
		t.Errorf("expected +15559876543 for whatsapp, got %s", addr)
	}

	if addr := c.ChannelAddress("email"); addr != "user@example.com" {
		t.Errorf("expected user@example.com for email, got %s", addr)
	}

	cNoPhone := Subscriber{
		Email: "user@example.com",
		Phone: null.String{},
	}

	if addr := cNoPhone.ChannelAddress("whatsapp"); addr != "user@example.com" {
		t.Errorf("expected user@example.com fallback for whatsapp without phone, got %s", addr)
	}
}

func TestSubscriberCompany(t *testing.T) {
	cWithCompany := Subscriber{
		Attribs: JSON{
			"company": "Acme Corp",
		},
	}

	if company := cWithCompany.Company(); company != "Acme Corp" {
		t.Errorf("expected Acme Corp, got %s", company)
	}

	cWithoutCompany := Subscriber{
		Attribs: JSON{},
	}

	if company := cWithoutCompany.Company(); company != "" {
		t.Errorf("expected empty string, got %s", company)
	}
}

func TestTimezoneResolutionHierarchy(t *testing.T) {
	// Case 1: Subscriber dedicated TZ takes precedence
	c1 := Subscriber{
		TZ:      "Asia/Tokyo",
		Attribs: JSON{"tz": "Europe/London"},
	}
	if loc := c1.ResolveTimezone(); loc.String() != "Asia/Tokyo" {
		t.Errorf("expected Asia/Tokyo, got %s", loc.String())
	}

	// Case 2: Subscriber specific timezone attribute used when TZ column empty
	c2 := Subscriber{
		Attribs: JSON{"tz": "Europe/London"},
	}
	if loc := c2.ResolveTimezone(); loc.String() != "Europe/London" {
		t.Errorf("expected Europe/London, got %s", loc.String())
	}

	// Case 3: Server local timezone fallback when subscriber tz is missing
	c3 := Subscriber{
		Attribs: JSON{},
	}
	if loc := c3.ResolveTimezone(); loc.String() != time.Local.String() {
		t.Errorf("expected %s, got %s", time.Local.String(), loc.String())
	}
}

func TestSubscriberToSubscriberSummary(t *testing.T) {
	c := Subscriber{
		Base: Base{
			ID: 42,
		},
		UUID:  "12345678-1234-1234-1234-123456789012",
		Name:  "Jane Doe",
		Email: "jane@example.com",
		Phone: null.StringFrom("+15550001111"),
	}

	summary := c.ToSubscriberSummary()
	if summary.ID != 42 || summary.UUID != "12345678-1234-1234-1234-123456789012" || summary.Name != "Jane Doe" || summary.Email != "jane@example.com" || summary.Phone != "+15550001111" {
		t.Errorf("unexpected SubscriberSummary values: %+v", summary)
	}
}
