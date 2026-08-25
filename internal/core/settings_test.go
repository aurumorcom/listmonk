//go:build unit || !integration

package core

import (
	"encoding/json"
	"testing"

	"github.com/knadh/listmonk/models"
)

func TestSMTPSettings_WAHASettings_UserID_JSON(t *testing.T) {
	settingsJSON := []byte(`{
		"smtp": [
			{
				"name": "email-outreach",
				"host": "smtp.mail.com",
				"port": 587,
				"user_id": 5
			}
		],
		"waha": [
			{
				"name": "whatsapp-outreach",
				"host": "http://waha:3000",
				"session": "default",
				"user_id": 5
			}
		]
	}`)

	var s models.Settings
	if err := json.Unmarshal(settingsJSON, &s); err != nil {
		t.Fatalf("unexpected error unmarshaling settings JSON: %v", err)
	}

	if len(s.SMTP) != 1 {
		t.Fatalf("expected 1 SMTP setting, got %d", len(s.SMTP))
	}
	if !s.SMTP[0].UserID.Valid || s.SMTP[0].UserID.Int != 5 {
		t.Errorf("expected SMTP user_id 5, got %+v", s.SMTP[0].UserID)
	}

	if len(s.WAHASettings) != 1 {
		t.Fatalf("expected 1 WAHA setting, got %d", len(s.WAHASettings))
	}
	if !s.WAHASettings[0].UserID.Valid || s.WAHASettings[0].UserID.Int != 5 {
		t.Errorf("expected WAHA user_id 5, got %+v", s.WAHASettings[0].UserID)
	}

	// Verify serialization back to JSON retains user_id
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("unexpected error marshaling settings struct: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unexpected error parsing raw JSON: %v", err)
	}

	smtpList, ok := raw["smtp"].([]interface{})
	if !ok || len(smtpList) == 0 {
		t.Fatalf("expected non-empty smtp array in serialized JSON")
	}
	smtpItem := smtpList[0].(map[string]interface{})
	if uid, ok := smtpItem["user_id"].(float64); !ok || int(uid) != 5 {
		t.Errorf("expected serialized smtp user_id to be 5, got %v", smtpItem["user_id"])
	}

	wahaList, ok := raw["waha"].([]interface{})
	if !ok || len(wahaList) == 0 {
		t.Fatalf("expected non-empty waha array in serialized JSON")
	}
	wahaItem := wahaList[0].(map[string]interface{})
	if uid, ok := wahaItem["user_id"].(float64); !ok || int(uid) != 5 {
		t.Errorf("expected serialized waha user_id to be 5, got %v", wahaItem["user_id"])
	}
}

func TestSMTPSettings_IMAPCredentials_JSON(t *testing.T) {
	settingsJSON := []byte(`{
		"smtp": [
			{
				"name": "email-outreach",
				"host": "smtp.gmail.com",
				"port": 587,
				"imap_enabled": true,
				"imap_host": "imap.gmail.com",
				"imap_port": 993,
				"imap_username": "user@gmail.com",
				"imap_password": "app_specific_secret_password",
				"imap_folder": "INBOX",
				"imap_auth_protocol": "login"
			}
		]
	}`)

	var s models.Settings
	if err := json.Unmarshal(settingsJSON, &s); err != nil {
		t.Fatalf("unexpected error unmarshaling settings JSON: %v", err)
	}

	if len(s.SMTP) != 1 {
		t.Fatalf("expected 1 SMTP setting, got %d", len(s.SMTP))
	}

	smtp := s.SMTP[0]
	if !smtp.IMAPEnabled {
		t.Errorf("expected IMAPEnabled to be true")
	}
	if smtp.IMAPHost != "imap.gmail.com" {
		t.Errorf("expected IMAPHost 'imap.gmail.com', got %q", smtp.IMAPHost)
	}
	if smtp.IMAPPort != 993 {
		t.Errorf("expected IMAPPort 993, got %d", smtp.IMAPPort)
	}
	if smtp.IMAPUsername != "user@gmail.com" {
		t.Errorf("expected IMAPUsername 'user@gmail.com', got %q", smtp.IMAPUsername)
	}
	if smtp.IMAPPassword != "app_specific_secret_password" {
		t.Errorf("expected IMAPPassword 'app_specific_secret_password', got %q", smtp.IMAPPassword)
	}

	// Test unmarshaling from nested "imap" sub-object
	nestedJSON := []byte(`{
		"smtp": [
			{
				"name": "email-outreach",
				"host": "smtp.gmail.com",
				"port": 587,
				"imap": {
					"enabled": true,
					"host": "imap.mail.yahoo.com",
					"port": 993,
					"username": "user@yahoo.com",
					"password": "yahoo_app_password"
				}
			}
		]
	}`)

	var s2 models.Settings
	if err := json.Unmarshal(nestedJSON, &s2); err != nil {
		t.Fatalf("unexpected error unmarshaling nested IMAP settings: %v", err)
	}

	smtp2 := s2.SMTP[0]
	if smtp2.IMAPHost != "imap.mail.yahoo.com" {
		t.Errorf("expected nested IMAPHost 'imap.mail.yahoo.com', got %q", smtp2.IMAPHost)
	}
	if smtp2.IMAPUsername != "user@yahoo.com" {
		t.Errorf("expected nested IMAPUsername 'user@yahoo.com', got %q", smtp2.IMAPUsername)
	}
	if smtp2.IMAPPassword != "yahoo_app_password" {
		t.Errorf("expected nested IMAPPassword 'yahoo_app_password', got %q", smtp2.IMAPPassword)
	}
}

func TestCRMSettings_JSON(t *testing.T) {
	settingsJSON := []byte(`{
		"crm": {
			"enabled": true,
			"base_url": "https://crm.example.com",
			"api_key": "crm_key_123",
			"api_secret": "crm_secret_456"
		}
	}`)

	var s models.Settings
	if err := json.Unmarshal(settingsJSON, &s); err != nil {
		t.Fatalf("unexpected error unmarshaling CRM settings JSON: %v", err)
	}

	if !s.CRM.Enabled {
		t.Error("expected CRM Enabled=true")
	}
	if s.CRM.BaseURL != "https://crm.example.com" {
		t.Errorf("expected BaseURL 'https://crm.example.com', got %q", s.CRM.BaseURL)
	}
	if s.CRM.APIKey != "crm_key_123" || s.CRM.APISecret != "crm_secret_456" {
		t.Errorf("unexpected API credentials in CRM settings: %+v", s.CRM)
	}

	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("unexpected error marshaling CRM settings: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unexpected error unmarshaling raw map: %v", err)
	}

	crmMap, ok := raw["crm"].(map[string]interface{})
	if !ok || crmMap["base_url"] != "https://crm.example.com" {
		t.Errorf("expected crm.base_url in serialized JSON, got %v", raw["crm"])
	}
}
