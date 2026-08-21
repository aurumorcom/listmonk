//go:build unit || !integration

package models

import (
	"testing"

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
	seq := Sequence{
		Timezone: "America/New_York",
	}

	// Case 1: Subscriber specific timezone takes precedence
	c1 := Subscriber{
		Attribs: JSON{"tz": "Asia/Tokyo"},
	}
	if loc := c1.ResolveTimezone(seq); loc.String() != "Asia/Tokyo" {
		t.Errorf("expected Asia/Tokyo, got %s", loc.String())
	}

	// Case 2: Sequence default timezone used when subscriber tz is missing
	c2 := Subscriber{
		Attribs: JSON{},
	}
	if loc := c2.ResolveTimezone(seq); loc.String() != "America/New_York" {
		t.Errorf("expected America/New_York, got %s", loc.String())
	}

	// Case 3: UTC fallback when both are missing
	emptySeq := Sequence{}
	if loc := c2.ResolveTimezone(emptySeq); loc.String() != "UTC" {
		t.Errorf("expected UTC, got %s", loc.String())
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
