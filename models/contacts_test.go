package models

import (
	"testing"

	null "gopkg.in/volatiletech/null.v6"
)

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

func TestTimezoneResolutionHierarchy(t *testing.T) {
	seq := Sequence{
		Timezone: "America/New_York",
	}

	// Case 1: Contact specific timezone takes precedence
	c1 := Subscriber{
		Attribs: JSON{"tz": "Asia/Tokyo"},
	}
	if loc := c1.ResolveTimezone(seq); loc.String() != "Asia/Tokyo" {
		t.Errorf("expected Asia/Tokyo, got %s", loc.String())
	}

	// Case 2: Sequence default timezone used when contact tz is missing
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

func TestContactToContactSummary(t *testing.T) {
	c := Contact{
		Base: Base{
			ID: 42,
		},
		UUID:  "12345678-1234-1234-1234-123456789012",
		Name:  "Jane Doe",
		Email: "jane@example.com",
		Phone: null.StringFrom("+15550001111"),
	}

	summary := c.ToContactSummary()
	if summary.ID != 42 || summary.UUID != "12345678-1234-1234-1234-123456789012" || summary.Name != "Jane Doe" || summary.Email != "jane@example.com" || summary.Phone != "+15550001111" {
		t.Errorf("unexpected ContactSummary values: %+v", summary)
	}
}

func TestSettingsStructsMapping(t *testing.T) {
	smtp := SMTPSettings{
		Name:      "Main SMTP",
		Host:      "smtp.example.com",
		Port:      587,
		Signature: "<p>SMTP Sig</p>",
	}
	imap := IMAPSettings{
		Host:   "imap.example.com",
		Port:   993,
		Folder: "INBOX",
	}
	emailSettings := EmailSettings{
		SMTP:   smtp,
		IMAP:   imap,
		UserID: null.IntFrom(1),
		User:   "admin",
	}

	if emailSettings.SMTP.Host != "smtp.example.com" || emailSettings.IMAP.Host != "imap.example.com" || emailSettings.SMTP.Signature != "<p>SMTP Sig</p>" {
		t.Errorf("unexpected EmailSettings mapping: %+v", emailSettings)
	}

	waha := WAHASettings{
		Name:      "Main WhatsApp",
		Host:      "http://localhost:3000",
		Session:   "default",
		Signature: "<p>WAHA Sig</p>",
	}
	waSettings := WhatsappSettings{
		WAHA:   waha,
		UserID: null.IntFrom(1),
		User:   "admin",
	}

	if waSettings.WAHA.Host != "http://localhost:3000" || waSettings.WAHA.Signature != "<p>WAHA Sig</p>" {
		t.Errorf("unexpected WhatsappSettings mapping: %+v", waSettings)
	}
}
