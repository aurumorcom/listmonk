package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/knadh/listmonk/internal/core"
	"github.com/knadh/listmonk/models"
)

func TestBotClassification_EnterpriseScanners(t *testing.T) {
	tests := []struct {
		name     string
		ua       string
		wantBot  bool
		wantType string
	}{
		{
			name:     "Microsoft SafeLinks Scanner",
			ua:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.85 Safari/537.36 SafeLinks",
			wantBot:  true,
			wantType: core.BotTypeSecurityScanner,
		},
		{
			name:     "Proofpoint URLDefense",
			ua:       "Mozilla/5.0 Proofpoint/1.0 (URLDefense/3.0; crawler)",
			wantBot:  true,
			wantType: core.BotTypeSecurityScanner,
		},
		{
			name:     "Mimecast Email Security",
			ua:       "Mozilla/5.0 (compatible; Mimecast Email Security Bot 1.2)",
			wantBot:  true,
			wantType: core.BotTypeSecurityScanner,
		},
		{
			name:     "Barracuda Networks Scanner",
			ua:       "Barracuda Networks Link Scanner/1.0",
			wantBot:  true,
			wantType: core.BotTypeSecurityScanner,
		},
		{
			name:     "Cisco IronPort",
			ua:       "Mozilla/5.0 (compatible; Cisco IronPort AsyncOS 14.0)",
			wantBot:  true,
			wantType: core.BotTypeSecurityScanner,
		},
		{
			name:     "Sophos Email Security",
			ua:       "Sophos Anti-Spam Link Scanner/2.0",
			wantBot:  true,
			wantType: core.BotTypeSecurityScanner,
		},
		{
			name:     "Trend Micro Email Security",
			ua:       "TrendMicro Deep Discovery Email Inspector/3.5",
			wantBot:  true,
			wantType: core.BotTypeSecurityScanner,
		},
		{
			name:     "SonicWALL Scanner",
			ua:       "SonicWALL Email Security Pre-fetcher/5.0",
			wantBot:  true,
			wantType: core.BotTypeSecurityScanner,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := core.ClassifyBot(tt.ua, nil, 0, false)
			if res.IsBot != tt.wantBot {
				t.Fatalf("expected is_bot=%v, got %v", tt.wantBot, res.IsBot)
			}
			if res.BotType != tt.wantType {
				t.Fatalf("expected bot_type=%s, got %s", tt.wantType, res.BotType)
			}
		})
	}
}

func TestBotClassification_AICrawlers(t *testing.T) {
	tests := []struct {
		name     string
		ua       string
		wantBot  bool
		wantType string
	}{
		{
			name:     "OpenAI GPTBot",
			ua:       "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; GPTBot/1.0; +https://openai.com/gptbot)",
			wantBot:  true,
			wantType: core.BotTypeAICrawler,
		},
		{
			name:     "OpenAI ChatGPT-User",
			ua:       "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; ChatGPT-User/1.0; +https://openai.com/bot)",
			wantBot:  true,
			wantType: core.BotTypeAICrawler,
		},
		{
			name:     "Anthropic ClaudeBot",
			ua:       "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; ClaudeBot/1.0; +claudebot@anthropic.com)",
			wantBot:  true,
			wantType: core.BotTypeAICrawler,
		},
		{
			name:     "PerplexityBot",
			ua:       "Mozilla/5.0 (compatible; PerplexityBot/1.0; +https://perplexity.ai/perplexitybot)",
			wantBot:  true,
			wantType: core.BotTypeAICrawler,
		},
		{
			name:     "Meta-ExternalAgent",
			ua:       "Mozilla/5.0 (compatible; Meta-ExternalAgent/1.0; +https://developers.facebook.com/docs/sharing/webmasters/crawler)",
			wantBot:  true,
			wantType: core.BotTypeAICrawler,
		},
		{
			name:     "Google-Extended AI",
			ua:       "Mozilla/5.0 (compatible; Google-Extended; +https://developers.google.com/search/docs/crawling-indexing/overview-google-crawlers)",
			wantBot:  true,
			wantType: core.BotTypeAICrawler,
		},
		{
			name:     "ByteDance Bytespider",
			ua:       "Mozilla/5.0 (Linux; Android 5.0) AppleWebKit/537.36 (KHTML, like Gecko) Mobile Safari/537.36 Bytespider",
			wantBot:  true,
			wantType: core.BotTypeAICrawler,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := core.ClassifyBot(tt.ua, nil, 0, false)
			if res.IsBot != tt.wantBot {
				t.Fatalf("expected is_bot=%v, got %v", tt.wantBot, res.IsBot)
			}
			if res.BotType != tt.wantType {
				t.Fatalf("expected bot_type=%s, got %s", tt.wantType, res.BotType)
			}
		})
	}
}

func TestBotClassification_Proxies(t *testing.T) {
	t.Run("Google Image Proxy", func(t *testing.T) {
		ua := "Mozilla/5.0 (Windows NT 5.1; rv:11.0) Gecko/Firefox/11.0 (via ggpht.com GoogleImageProxy)"
		res := core.ClassifyBot(ua, nil, 0, false)
		if !res.IsBot || !res.IsProxy || res.BotType != core.BotTypeGoogleProxy {
			t.Fatalf("expected Google Image Proxy classification, got %+v", res)
		}
	})

	t.Run("Apple Mail Privacy Protection", func(t *testing.T) {
		ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Apple-Mail/3608.120"
		res := core.ClassifyBot(ua, nil, 0, false)
		if !res.IsBot || !res.IsProxy || res.BotType != core.BotTypeProxyMPP {
			t.Fatalf("expected Apple MPP classification, got %+v", res)
		}
	})
}

func TestBotClassification_HeadlessAndScripts(t *testing.T) {
	scripts := []string{
		"HeadlessChrome/112.0.5615.49",
		"curl/7.88.1",
		"python-requests/2.31.0",
		"Go-http-client/1.1",
		"PostmanRuntime/7.32.3",
	}

	for _, s := range scripts {
		t.Run(s, func(t *testing.T) {
			res := core.ClassifyBot(s, nil, 0, false)
			if !res.IsBot {
				t.Fatalf("expected bot detection for script/headless %s, got false", s)
			}
		})
	}
}

func TestBotClassification_HumanUserAgents(t *testing.T) {
	humans := []struct {
		name    string
		ua      string
		wantOS  string
		wantDev string
	}{
		{
			name:    "iPhone Safari",
			ua:      "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
			wantOS:  "iOS",
			wantDev: "mobile",
		},
		{
			name:    "Android Chrome",
			ua:      "Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.6312.80 Mobile Safari/537.36",
			wantOS:  "Android",
			wantDev: "mobile",
		},
		{
			name:    "Windows Chrome",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
			wantOS:  "Windows",
			wantDev: "desktop",
		},
		{
			name:    "macOS Safari",
			ua:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
			wantOS:  "macOS",
			wantDev: "desktop",
		},
	}

	for _, h := range humans {
		t.Run(h.name, func(t *testing.T) {
			meta := core.ParseClientMeta("203.0.113.195", h.ua, nil, 0, false, nil, "", "", nil)
			if meta.IsBot {
				t.Fatalf("expected human user-agent not to be tagged as bot: %s", h.ua)
			}
			if !meta.IsHuman {
				t.Fatalf("expected is_human=true for: %s", h.name)
			}
			if meta.DeviceType != h.wantDev {
				t.Fatalf("expected device_type=%s, got %s", h.wantDev, meta.DeviceType)
			}
		})
	}
}

func TestBotClassification_PrefetchAndHoneypot(t *testing.T) {
	t.Run("Speed-of-click prefetch anomaly", func(t *testing.T) {
		res := core.ClassifyBot("Mozilla/5.0 Human", nil, 500*time.Millisecond, false)
		if !res.IsBot || res.BotType != core.BotTypePrefetch {
			t.Fatalf("expected prefetch bot detection for 500ms click delta, got %+v", res)
		}
	})

	t.Run("Honeypot trap hit", func(t *testing.T) {
		res := core.ClassifyBot("Mozilla/5.0 Human", nil, 0, true)
		if !res.IsBot || res.BotType != core.BotTypeHoneypot {
			t.Fatalf("expected honeypot bot detection, got %+v", res)
		}
	})
}

func TestClientMetaParsing(t *testing.T) {
	headers := map[string]string{
		"cf-ipcountry": "US",
		"cf-region":    "California",
		"cf-ipcity":    "San Francisco",
		"cf-ipasn":     "AS13335 Cloudflare",
		"referer":      "https://mail.google.com/mail/u/0/",
	}

	utms := map[string]string{
		"utm_source":   "newsletter",
		"utm_campaign": "spring_sale",
	}

	meta := core.ParseClientMeta(
		"198.51.100.42:54321",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		headers,
		0,
		false,
		nil,
		"variant_b",
		"hero_button",
		utms,
	)

	if meta.IPAddress != "198.51.100.42" {
		t.Fatalf("expected sanitized IP 198.51.100.42, got %s", meta.IPAddress)
	}
	if meta.GeoCountry != "US" || meta.GeoCity != "San Francisco" {
		t.Fatalf("expected US / San Francisco, got %s / %s", meta.GeoCountry, meta.GeoCity)
	}
	if meta.EmailClient != "Gmail Webmail" {
		t.Fatalf("expected Gmail Webmail, got %s", meta.EmailClient)
	}
	if meta.VariantID != "variant_b" || meta.LinkPosition != "hero_button" {
		t.Fatalf("expected variant_b / hero_button, got %s / %s", meta.VariantID, meta.LinkPosition)
	}
	if meta.UTMParams["utm_campaign"] != "spring_sale" {
		t.Fatalf("expected spring_sale UTM, got %s", meta.UTMParams["utm_campaign"])
	}
	t.Log("Successfully verified ClientMeta extraction and parsing")
}

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

	seqAnalytics := models.SequenceAnalytics{
		ActiveContacts:      45,
		StepCompletions:     120,
		ReplyRate:           18.5,
		ConversionRate:      12.0,
		AggregatedAnalytics: campAnalytics,
		Funnel: []models.SequenceStepFunnel{
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
		t.Fatalf("failed to marshal SequenceAnalytics: %v", err)
	}

	var parsed models.SequenceAnalytics
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
