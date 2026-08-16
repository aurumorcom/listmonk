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
