//go:build unit || !integration

package models

import (
	"encoding/json"
	"testing"

	null "gopkg.in/volatiletech/null.v6"
)

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

func TestSettings_UserID_JSON_Unmarshal(t *testing.T) {
	jsonBlob := []byte(`{
		"smtp": [
			{
				"name": "email-1",
				"host": "smtp.example.com",
				"user_id": 42
			}
		],
		"waha": [
			{
				"name": "whatsapp-1",
				"host": "http://waha:3000",
				"user_id": 42
			}
		]
	}`)

	var settings Settings
	if err := json.Unmarshal(jsonBlob, &settings); err != nil {
		t.Fatalf("failed to unmarshal settings: %v", err)
	}

	if len(settings.SMTP) != 1 || !settings.SMTP[0].UserID.Valid || settings.SMTP[0].UserID.Int != 42 {
		t.Errorf("expected SMTP UserID 42, got: %+v", settings.SMTP)
	}

	if len(settings.WAHASettings) != 1 || !settings.WAHASettings[0].UserID.Valid || settings.WAHASettings[0].UserID.Int != 42 {
		t.Errorf("expected WAHASettings UserID 42, got: %+v", settings.WAHASettings)
	}
}
