package models

import "testing"
import null "gopkg.in/volatiletech/null.v6"

func TestContactAliasEquivalence(t *testing.T) {
	sub := Subscriber{
		Email: "test@example.com",
		Name:  "Test User",
		Phone: null.StringFrom("+15551234567"),
	}

	var contact Contact = sub

	if contact.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", contact.Email)
	}

	if contact.Name != "Test User" {
		t.Errorf("expected name Test User, got %s", contact.Name)
	}

	var subs Subscribers = Subscribers{sub}
	var contacts Contacts = subs

	if len(contacts) != 1 {
		t.Errorf("expected 1 contact, got %d", len(contacts))
	}
}

func TestContactPrimaryIdentifier(t *testing.T) {
	cEmail := Contact{
		Email: "user@example.com",
		Phone: null.StringFrom("+15559876543"),
	}
	if id := cEmail.PrimaryIdentifier(); id != "user@example.com" {
		t.Errorf("expected user@example.com, got %s", id)
	}

	cPhoneOnly := Contact{
		Email: "",
		Phone: null.StringFrom("+15559876543"),
	}
	if id := cPhoneOnly.PrimaryIdentifier(); id != "+15559876543" {
		t.Errorf("expected +15559876543, got %s", id)
	}
}

func TestContactChannelAddress(t *testing.T) {
	c := Contact{
		Email: "user@example.com",
		Phone: null.StringFrom("+15559876543"),
	}

	if addr := c.ChannelAddress("whatsapp"); addr != "+15559876543" {
		t.Errorf("expected +15559876543 for whatsapp, got %s", addr)
	}

	if addr := c.ChannelAddress("email"); addr != "user@example.com" {
		t.Errorf("expected user@example.com for email, got %s", addr)
	}

	cNoPhone := Contact{
		Email: "user@example.com",
		Phone: null.String{},
	}

	if addr := cNoPhone.ChannelAddress("whatsapp"); addr != "user@example.com" {
		t.Errorf("expected user@example.com fallback for whatsapp without phone, got %s", addr)
	}
}

func TestContactCompany(t *testing.T) {
	cWithCompany := Contact{
		Attribs: JSON{
			"company": "Acme Corp",
		},
	}

	if company := cWithCompany.Company(); company != "Acme Corp" {
		t.Errorf("expected Acme Corp, got %s", company)
	}

	cWithoutCompany := Contact{
		Attribs: JSON{},
	}

	if company := cWithoutCompany.Company(); company != "" {
		t.Errorf("expected empty string, got %s", company)
	}
}
