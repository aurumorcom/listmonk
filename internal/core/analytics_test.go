//go:build unit || !integration

package core

import (
	"testing"
	"time"
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
			wantType: BotTypeSecurityScanner,
		},
		{
			name:     "Proofpoint URLDefense",
			ua:       "Mozilla/5.0 Proofpoint/1.0 (URLDefense/3.0; crawler)",
			wantBot:  true,
			wantType: BotTypeSecurityScanner,
		},
		{
			name:     "Mimecast Email Security",
			ua:       "Mozilla/5.0 (compatible; Mimecast Email Security Bot 1.2)",
			wantBot:  true,
			wantType: BotTypeSecurityScanner,
		},
		{
			name:     "Barracuda Networks Scanner",
			ua:       "Barracuda Networks Link Scanner/1.0",
			wantBot:  true,
			wantType: BotTypeSecurityScanner,
		},
		{
			name:     "Cisco IronPort",
			ua:       "Mozilla/5.0 (compatible; Cisco IronPort AsyncOS 14.0)",
			wantBot:  true,
			wantType: BotTypeSecurityScanner,
		},
		{
			name:     "Sophos Email Security",
			ua:       "Sophos Anti-Spam Link Scanner/2.0",
			wantBot:  true,
			wantType: BotTypeSecurityScanner,
		},
		{
			name:     "Trend Micro Email Security",
			ua:       "TrendMicro Deep Discovery Email Inspector/3.5",
			wantBot:  true,
			wantType: BotTypeSecurityScanner,
		},
		{
			name:     "SonicWALL Scanner",
			ua:       "SonicWALL Email Security Pre-fetcher/5.0",
			wantBot:  true,
			wantType: BotTypeSecurityScanner,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ClassifyBot(tt.ua, nil, 0, false)
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
			wantType: BotTypeAICrawler,
		},
		{
			name:     "OpenAI ChatGPT-User",
			ua:       "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; ChatGPT-User/1.0; +https://openai.com/bot)",
			wantBot:  true,
			wantType: BotTypeAICrawler,
		},
		{
			name:     "Anthropic ClaudeBot",
			ua:       "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; ClaudeBot/1.0; +claudebot@anthropic.com)",
			wantBot:  true,
			wantType: BotTypeAICrawler,
		},
		{
			name:     "PerplexityBot",
			ua:       "Mozilla/5.0 (compatible; PerplexityBot/1.0; +https://perplexity.ai/perplexitybot)",
			wantBot:  true,
			wantType: BotTypeAICrawler,
		},
		{
			name:     "Meta-ExternalAgent",
			ua:       "Mozilla/5.0 (compatible; Meta-ExternalAgent/1.0; +https://developers.facebook.com/docs/sharing/webmasters/crawler)",
			wantBot:  true,
			wantType: BotTypeAICrawler,
		},
		{
			name:     "Google-Extended AI",
			ua:       "Mozilla/5.0 (compatible; Google-Extended; +https://developers.google.com/search/docs/crawling-indexing/overview-google-crawlers)",
			wantBot:  true,
			wantType: BotTypeAICrawler,
		},
		{
			name:     "ByteDance Bytespider",
			ua:       "Mozilla/5.0 (Linux; Android 5.0) AppleWebKit/537.36 (KHTML, like Gecko) Mobile Safari/537.36 Bytespider",
			wantBot:  true,
			wantType: BotTypeAICrawler,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ClassifyBot(tt.ua, nil, 0, false)
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
		res := ClassifyBot(ua, nil, 0, false)
		if !res.IsBot || !res.IsProxy || res.BotType != BotTypeGoogleProxy {
			t.Fatalf("expected Google Image Proxy classification, got %+v", res)
		}
	})

	t.Run("Apple Mail Privacy Protection", func(t *testing.T) {
		ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Apple-Mail/3608.120"
		res := ClassifyBot(ua, nil, 0, false)
		if !res.IsBot || !res.IsProxy || res.BotType != BotTypeProxyMPP {
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
			res := ClassifyBot(s, nil, 0, false)
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
			meta := ParseClientMeta("203.0.113.195", h.ua, nil, 0, false, nil, "", "", nil)
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
		res := ClassifyBot("Mozilla/5.0 Human", nil, 500*time.Millisecond, false)
		if !res.IsBot || res.BotType != BotTypePrefetch {
			t.Fatalf("expected prefetch bot detection for 500ms click delta, got %+v", res)
		}
	})

	t.Run("Honeypot trap hit", func(t *testing.T) {
		res := ClassifyBot("Mozilla/5.0 Human", nil, 0, true)
		if !res.IsBot || res.BotType != BotTypeHoneypot {
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

	meta := ParseClientMeta(
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
}
