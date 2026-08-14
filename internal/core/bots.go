package core

import (
	"strings"
	"time"

	"github.com/mileusna/useragent"
)

const (
	BotTypeSecurityScanner = "security_scanner"
	BotTypeAICrawler       = "ai_crawler"
	BotTypeWebCrawler      = "web_crawler"
	BotTypeProxyMPP        = "proxy_mpp"
	BotTypeGoogleProxy     = "google_proxy"
	BotTypePrefetch        = "prefetch"
	BotTypeHoneypot        = "honeypot"
	BotTypeHeadless        = "headless"
	BotTypeScript          = "script"
)

var (
	// Enterprise security gateways and anti-phishing pre-click scanners
	securityScannerTokens = []string{
		"safelinks",
		"ms-office",
		"outlook-ios",
		"outlook-android",
		"proofpoint",
		"pp_url_defense",
		"urldefense",
		"mimecast",
		"barracuda",
		"bariguard",
		"ironport",
		"cisco-talos",
		"fortimail",
		"sophos",
		"trendmicro",
		"sonicwall",
		"kaspersky",
		"eset",
		"avast",
		"fireeye",
		"trellix",
		"crowdstrike",
		"spamexperts",
		"mailcontrol",
		"forcepoint",
		"symantec",
	}

	// Modern Generative AI and LLM scrapers/indexers
	aiCrawlerTokens = []string{
		"gptbot",
		"chatgpt-user",
		"oai-searchbot",
		"claudebot",
		"claude-web",
		"anthropic-ai",
		"perplexitybot",
		"meta-externalagent",
		"facebookbot",
		"google-extended",
		"google-cloudvertexbot",
		"googleother",
		"bytespider",
		"ccbot",
		"diffbot",
		"cohere-ai",
		"mistralai",
		"amazonbot",
		"applebot-extended",
	}

	// Search engine indexers and social preview scrapers
	webCrawlerTokens = []string{
		"googlebot",
		"bingbot",
		"yandexbot",
		"baiduspider",
		"duckduckbot",
		"twitterbot",
		"facebookexternalhit",
		"linkedinbot",
		"slackbot",
		"telegrambot",
		"discordbot",
		"whatsapp",
		"pinterest",
		"yahoo! slurp",
		"sogou",
		"exabot",
		"seznambot",
		"semrushbot",
		"ahrefsbot",
		"mj12bot",
	}

	// Automation and headless HTTP test frameworks
	headlessTokens = []string{
		"headlesschrome",
		"phantomjs",
		"playwright",
		"puppeteer",
		"selenium",
		"cypress",
	}

	// Scripted HTTP libraries
	scriptTokens = []string{
		"curl",
		"python-requests",
		"aiohttp",
		"urllib",
		"go-http-client",
		"postmanruntime",
		"apache-httpclient",
		"wget",
		"httpx",
		"node-fetch",
		"axios",
		"httpclient",
		"libwww-perl",
		"okhttp",
	}
)

// BotClassification contains the evaluation outcome of request headers and user agent.
type BotClassification struct {
	IsBot   bool   `json:"is_bot"`
	BotType string `json:"bot_type"`
	IsProxy bool   `json:"is_proxy"`
	IsHuman bool   `json:"is_human"`
}

// ClassifyBot evaluates User-Agent, custom proxy headers, click delta, and honeypots to detect automated bots.
func ClassifyBot(ua string, headers map[string]string, deltaSinceSend time.Duration, isHoneypot bool) BotClassification {
	// 1. Check honeypot parameter
	if isHoneypot {
		return BotClassification{
			IsBot:   true,
			BotType: BotTypeHoneypot,
			IsProxy: false,
			IsHuman: false,
		}
	}

	// 2. Check timing prefetch delta (< 1500ms from campaign send)
	if deltaSinceSend > 0 && deltaSinceSend < 1500*time.Millisecond {
		return BotClassification{
			IsBot:   true,
			BotType: BotTypePrefetch,
			IsProxy: false,
			IsHuman: false,
		}
	}

	uaLower := strings.ToLower(ua)

	// 3. Check Mail Privacy & Image Proxies
	if strings.Contains(uaLower, "googleimageproxy") {
		return BotClassification{
			IsBot:   true,
			BotType: BotTypeGoogleProxy,
			IsProxy: true,
			IsHuman: false,
		}
	}

	if strings.Contains(uaLower, "apple-mail") || strings.Contains(uaLower, "apple-cloud-privacy-relay") || headers["x-apple-mpp"] != "" {
		return BotClassification{
			IsBot:   true,
			BotType: BotTypeProxyMPP,
			IsProxy: true,
			IsHuman: false,
		}
	}

	if strings.Contains(uaLower, "yahoomailproxy") {
		return BotClassification{
			IsBot:   true,
			BotType: BotTypeProxyMPP,
			IsProxy: true,
			IsHuman: false,
		}
	}

	// 4. Check Enterprise Security Gateways
	for _, tok := range securityScannerTokens {
		if strings.Contains(uaLower, tok) {
			return BotClassification{
				IsBot:   true,
				BotType: BotTypeSecurityScanner,
				IsProxy: false,
				IsHuman: false,
			}
		}
	}

	// 5. Check Generative AI & LLM Crawlers
	for _, tok := range aiCrawlerTokens {
		if strings.Contains(uaLower, tok) {
			return BotClassification{
				IsBot:   true,
				BotType: BotTypeAICrawler,
				IsProxy: false,
				IsHuman: false,
			}
		}
	}

	// 6. Check Web Crawlers & Social Scrapers
	for _, tok := range webCrawlerTokens {
		if strings.Contains(uaLower, tok) {
			return BotClassification{
				IsBot:   true,
				BotType: BotTypeWebCrawler,
				IsProxy: false,
				IsHuman: false,
			}
		}
	}

	// 7. Check Headless Automation
	for _, tok := range headlessTokens {
		if strings.Contains(uaLower, tok) {
			return BotClassification{
				IsBot:   true,
				BotType: BotTypeHeadless,
				IsProxy: false,
				IsHuman: false,
			}
		}
	}

	// 8. Check Scripted HTTP Clients
	for _, tok := range scriptTokens {
		if strings.Contains(uaLower, tok) {
			return BotClassification{
				IsBot:   true,
				BotType: BotTypeScript,
				IsProxy: false,
				IsHuman: false,
			}
		}
	}

	// 9. Check standard useragent library Bot detector
	if ua != "" {
		parsed := useragent.Parse(ua)
		if parsed.Bot {
			return BotClassification{
				IsBot:   true,
				BotType: BotTypeWebCrawler,
				IsProxy: false,
				IsHuman: false,
			}
		}
	}

	// Empty UA is flagged as script/bot
	if strings.TrimSpace(ua) == "" {
		return BotClassification{
			IsBot:   true,
			BotType: BotTypeScript,
			IsProxy: false,
			IsHuman: false,
		}
	}

	// Genuine Human
	return BotClassification{
		IsBot:   false,
		BotType: "",
		IsProxy: false,
		IsHuman: true,
	}
}
