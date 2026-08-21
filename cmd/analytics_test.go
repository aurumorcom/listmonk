//go:build integration || e2e || resilience || !unit

package main

import (
	"encoding/json"
	"testing"

	"github.com/knadh/listmonk/models"
)

func TestCampaignAndSequenceAnalytics_SupersetJSON(t *testing.T) {
	campAnalytics := models.CampaignAnalytics{
		CampaignID:   42,
		CampaignUUID: "ca908234-7128-482a-a82f-293812039841",
		CampaignName: "Spring Launch",
		Sent:         1000,
		ToSend:       50,
		Bounces:      12,
		Views: models.CampaignViewStats{
			Total:         600,
			Unique:        450,
			HumanTotal:    420,
			HumanUnique:   380,
			BotTotal:      180,
			ProxyMPPTotal: 150,
		},
		Clicks: models.CampaignClickStats{
			Total:       200,
			Unique:      140,
			HumanTotal:  160,
			HumanUnique: 120,
			BotClicks:   40,
			CTOR:        31.57,
		},
		Breakdowns: models.CampaignBreakdownStats{
			Devices: []models.DeviceBreakdown{
				{DeviceType: "mobile", OS: "iOS", Browser: "Safari", Clicks: 90, UniqueClicks: 70},
			},
			Locations: []models.LocationBreakdown{
				{Country: "US", Region: "CA", City: "Los Angeles", ASN: "AS7018", Clicks: 50, UniqueClicks: 40},
			},
			Bots: models.CampaignBotStats{
				TotalBotEvents:   220,
				ScannersDetected: 40,
				HoneypotTriggers: 2,
				BotTypeBreakdown: map[string]int{
					"security_scanner": 40,
					"proxy_mpp":        150,
				},
			},
		},
	}

	seqAnalytics := models.CampaignSequenceAnalytics{
		ActiveSubscribers:   45,
		StepCompletions:     120,
		ReplyRate:           18.5,
		ConversionRate:      12.0,
		AggregatedAnalytics: campAnalytics,
		Funnel: []models.CampaignStepFunnel{
			{
				StepNumber: 1,
				Subject:    "Intro Cold Outreach",
				Messenger:  "email",
				Reached:    50,
				Replied:    9,
				Analytics:  campAnalytics,
			},
		},
	}

	// Verify JSON marshaling & unmarshaling fidelity
	data, err := json.Marshal(seqAnalytics)
	if err != nil {
		t.Fatalf("failed to marshal CampaignSequenceAnalytics: %v", err)
	}

	var parsed models.CampaignSequenceAnalytics
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal SequenceAnalytics: %v", err)
	}

	if parsed.AggregatedAnalytics.Clicks.CTOR != 31.57 {
		t.Fatalf("expected CTOR 31.57 in unmarshaled superset, got %f", parsed.AggregatedAnalytics.Clicks.CTOR)
	}
	if parsed.Funnel[0].Analytics.Views.ProxyMPPTotal != 150 {
		t.Fatalf("expected ProxyMPPTotal 150 in step analytics, got %d", parsed.Funnel[0].Analytics.Views.ProxyMPPTotal)
	}
	t.Log("Successfully verified CampaignAnalytics & SequenceAnalytics superset JSON marshaling")
}
