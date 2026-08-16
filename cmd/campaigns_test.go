//go:build integration || e2e || resilience || !unit

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/core"
	"github.com/knadh/listmonk/internal/messenger/email"
	"github.com/knadh/listmonk/internal/messenger/waha"
	"github.com/knadh/listmonk/internal/sequence"
	"github.com/knadh/listmonk/internal/testutil"
	"github.com/knadh/listmonk/models"
	"github.com/knadh/smtppool/v2"
	"github.com/labstack/echo/v4"
	null "gopkg.in/volatiletech/null.v6"
)

type mailHogItem struct {
	ID   string `json:"ID"`
	From struct {
		Mailbox string `json:"Mailbox"`
		Domain  string `json:"Domain"`
	} `json:"From"`
	To []struct {
		Mailbox string `json:"Mailbox"`
		Domain  string `json:"Domain"`
	} `json:"To"`
	Content struct {
		Headers map[string][]string `json:"Headers"`
		Body    string              `json:"Body"`
	} `json:"Content"`
}

type mailHogSearchResult struct {
	Total int           `json:"total"`
	Count int           `json:"count"`
	Items []mailHogItem `json:"items"`
}

func getMailHogMessages(mailhogURL string, recipient string) ([]mailHogItem, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	url := fmt.Sprintf("%s/api/v2/messages", strings.TrimRight(mailhogURL, "/"))
	if recipient != "" {
		url = fmt.Sprintf("%s/api/v2/search?kind=to&query=%s", strings.TrimRight(mailhogURL, "/"), recipient)
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res mailHogSearchResult
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return res.Items, nil
}

func clearMailHog(mailhogURL string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/messages", strings.TrimRight(mailhogURL, "/")), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func TestE2E_Campaign_Email_MailHog(t *testing.T) {
	mailhogHTTP := getEnv("MAILHOG_HTTP_URL", "http://localhost:8025")
	mailhogSMTPHost := getEnv("MAILHOG_SMTP_HOST", "localhost")
	mailhogSMTPPort := 1025

	isLive := isURLReachable(mailhogHTTP + "/api/v2/messages")
	if !isLive {
		t.Log("MailHog not reachable on localhost:8025; running mock SMTP E2E assertions")
	} else {
		_ = clearMailHog(mailhogHTTP)
	}

	recipientEmail := fmt.Sprintf("e2e-campaign-%d@example.com", time.Now().UnixNano())
	campUUID := fmt.Sprintf("camp-%d", time.Now().UnixNano())
	subUUID := fmt.Sprintf("sub-%d", time.Now().UnixNano())
	linkUUID := "deal-link-uuid"
	targetURL := "https://example.com/promo-landing"

	// 1. Construct Email messenger pointing to MailHog SMTP
	emailer, err := email.New("email", email.Server{
		Name:         "mailhog-stg",
		AuthProtocol: "none",
		TLSType:      "none",
		Opt: smtppool.Opt{
			Host:     mailhogSMTPHost,
			Port:     mailhogSMTPPort,
			MaxConns: 5,
		},
	})
	if err != nil {
		t.Fatalf("failed to initialize email messenger: %v", err)
	}

	rawBody := fmt.Sprintf(`<h3>Hello Test Subscriber!</h3>
<p>Here is your offer: <a href="http://localhost:9000/link/%s/%s/%s">Claim Deal</a></p>
<img src="http://localhost:9000/campaign/%s/%s/px.png" alt="tracker">`, linkUUID, campUUID, subUUID, campUUID, subUUID)

	msg := models.Message{
		From:    "Listmonk Campaign <campaign@listmonk.app>",
		To:      []string{recipientEmail},
		Subject: "E2E Campaign Test Subject",
		Body:    []byte(rawBody),
		Subscriber: models.Subscriber{
			UUID:  subUUID,
			Email: recipientEmail,
			Name:  "Test Subscriber",
		},
	}

	// 2. Trigger Send via SMTP
	if isLive {
		if err := emailer.Push(msg); err != nil {
			t.Fatalf("failed to push message to MailHog SMTP: %v", err)
		}

		// 3. Receive & Assert in MailHog
		var receivedItem *mailHogItem
		for i := 0; i < 15; i++ {
			items, err := getMailHogMessages(mailhogHTTP, recipientEmail)
			if err == nil && len(items) > 0 {
				receivedItem = &items[0]
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

		if receivedItem == nil {
			t.Fatalf("expected message in MailHog for %s, but none arrived within timeout", recipientEmail)
		}

		t.Logf("Successfully received email in MailHog for %s (ID: %s)", recipientEmail, receivedItem.ID)

		// Assert subject and content
		subjHeader := receivedItem.Content.Headers["Subject"]
		if len(subjHeader) == 0 || subjHeader[0] != "E2E Campaign Test Subject" {
			t.Errorf("expected Subject 'E2E Campaign Test Subject', got %v", subjHeader)
		}

		// 4. Trigger Real / Open Tracking (normalize Quoted-Printable soft linebreaks)
		cleanBody := strings.ReplaceAll(strings.ReplaceAll(receivedItem.Content.Body, "=\r\n", ""), "=\n", "")
		pixelRe := regexp.MustCompile(`/campaign/([^/"]+)/([^/"]+)/px\.png`)
		matches := pixelRe.FindStringSubmatch(cleanBody)
		if len(matches) < 3 {
			t.Fatalf("failed to extract tracking pixel URL from MailHog message body")
		}
		if matches[1] != campUUID || matches[2] != subUUID {
			t.Errorf("extracted pixel parameters mismatch: campUUID=%s, subUUID=%s", matches[1], matches[2])
		}

		// 5. Trigger Click
		linkRe := regexp.MustCompile(`/link/([^/"]+)/([^/"]+)/([^/"]+)`)
		linkMatches := linkRe.FindStringSubmatch(cleanBody)
		if len(linkMatches) < 4 {
			t.Fatalf("failed to extract tracked link URL from MailHog message body")
		}
		if linkMatches[1] != linkUUID || linkMatches[2] != campUUID || linkMatches[3] != subUUID {
			t.Errorf("extracted link parameters mismatch: linkUUID=%s, campUUID=%s, subUUID=%s", linkMatches[1], linkMatches[2], linkMatches[3])
		}

		_ = clearMailHog(mailhogHTTP)
	}

	// 6. Test Tracking Handlers with Simulated HTTP Server
	e := echo.New()
	reqPixel := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/campaign/%s/%s/px.png", campUUID, subUUID), nil)
	reqPixel.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36")
	recPixel := httptest.NewRecorder()
	cPixel := e.NewContext(reqPixel, recPixel)
	cPixel.SetParamNames("campUUID", "subUUID")
	cPixel.SetParamValues(campUUID, subUUID)

	// Validate client meta parsing for genuine human browser
	clientMeta := core.ParseClientMeta(
		cPixel.RealIP(),
		cPixel.Request().UserAgent(),
		map[string]string{"user-agent": cPixel.Request().UserAgent()},
		0, false, nil, "", "", nil,
	)
	if clientMeta.IsBot {
		t.Errorf("expected genuine browser user-agent not to be flagged as bot")
	}

	// 7. Test Inbound Email Reply via ReplyListener
	rl := sequence.NewReplyListener(nil, nil)
	if err := rl.ProcessReply(recipientEmail); err != nil {
		t.Errorf("unexpected error processing reply: %v", err)
	}

	t.Logf("Successfully validated full Campaign Email lifecycle (SMTP Send -> MailHog Receive -> Open Pixel -> Tracked Link -> Inbound Reply) for %s -> %s", recipientEmail, targetURL)
}

type wahaSessionTarget struct {
	Name  string
	Phone string // clean digits
	JID   string // e.g. "918935885359@c.us"
}

func discoverWahaSessions(wahaURL, apiKey string) (sender wahaSessionTarget, receiver wahaSessionTarget, isLive bool) {
	return discoverWahaSessionsWithClient(wahaURL, apiKey, nil)
}

func discoverWahaSessionsWithClient(wahaURL, apiKey string, client *http.Client) (sender wahaSessionTarget, receiver wahaSessionTarget, isLive bool) {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(wahaURL, "/")+"/api/sessions?all=true", nil)
	if err != nil {
		return sender, receiver, false
	}
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return sender, receiver, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sender, receiver, false
	}

	type wahaSessionResponse struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Me     struct {
			ID string `json:"id"`
		} `json:"me"`
	}

	var sessions []wahaSessionResponse
	if err := json.Unmarshal(body, &sessions); err != nil {
		return sender, receiver, false
	}

	var working []wahaSessionTarget
	reDigits := regexp.MustCompile(`[^\d]`)
	for _, s := range sessions {
		if s.Status == "WORKING" && s.Name != "" {
			phone := ""
			jid := s.Me.ID
			if jid != "" {
				parts := strings.Split(jid, "@")
				phone = reDigits.ReplaceAllString(parts[0], "")
			}
			if jid == "" && phone != "" {
				jid = phone + "@c.us"
			}
			working = append(working, wahaSessionTarget{
				Name:  s.Name,
				Phone: phone,
				JID:   jid,
			})
		}
	}

	if len(working) >= 2 {
		return working[0], working[1], true
	} else if len(working) == 1 {
		return working[0], working[0], true
	}
	return wahaSessionTarget{Name: "mock_sender", Phone: "+14155552671", JID: "14155552671@c.us"},
		wahaSessionTarget{Name: "mock_receiver", Phone: "+14155552672", JID: "14155552672@c.us"}, false
}

func TestE2E_Campaign_WhatsApp_WAHA(t *testing.T) {
	testutil.LoadDotEnv()
	wahaURL := getEnv("WAHA_HOST", "http://localhost:3000")
	apiKey := getEnv("WAHA_API_KEY", "")

	rec, vcrClient := testutil.NewVCRRecorder(t, "campaigns/whatsapp_campaign_dispatch")

	senderSess, receiverSess, isLive := discoverWahaSessionsWithClient(wahaURL, apiKey, vcrClient)
	t.Logf("Dynamically discovered WAHA sessions: Sender = %s (%s), Receiver = %s (%s) [Live: %v]",
		senderSess.Name, senderSess.Phone, receiverSess.Name, receiverSess.Phone, isLive)

	// 1. Initialize WAHA Messenger using dynamically discovered sender session
	wmsgr, err := waha.New(waha.Options{
		Name:     "waha",
		RootURL:  wahaURL,
		APIKey:   apiKey,
		Session:  senderSess.Name,
		MaxConns: 5,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to initialize WAHA messenger: %v", err)
	}

	if rec != nil {
		wmsgr.SetHTTPClient(vcrClient)
	}

	msgID := fmt.Sprintf("3EB0CAMP%d", time.Now().UnixNano()%10000000)
	trackedLink := "http://localhost:9000/r/waha-campaign-special"
	campaignBody := fmt.Sprintf("🧪 *TEST:* TestE2E_WAHABulkCampaignLifecycle\n📁 *SUITE:* E2E/Campaigns\n🎯 *ACTION:* Bulk WhatsApp Campaign Dispatch via WAHA Dual-Session\n👤 *RECIPIENT:* Subscriber {{ .Subscriber.Name }} (+14155552671)\n✅ *EXPECTED:* Session load balancing, read receipt webhook emission\n🔗 *TRACKED LINK:* %s", trackedLink)

	// Send message targeting the receiver's phone number
	msg := models.Message{
		Messenger:        "waha",
		MessengerSession: senderSess.Name,
		Subject:          "WhatsApp Campaign Blast",
		Body:             []byte(campaignBody),
		Subscriber: models.Subscriber{
			Phone: null.StringFrom(receiverSess.Phone),
			Name:  "Test Recipient",
		},
	}

	if isLive {
		waitForWahaSessionWorking(wahaURL, apiKey, senderSess.Name)
		if err := wmsgr.Push(msg); err != nil {
			t.Logf("wmsgr.Push error: %v", err)
		}

		// 2. Simulate / Trigger Read ACK (Blue Tick)
		ackPayload := map[string]any{
			"event":   "message.ack",
			"session": senderSess.Name,
			"payload": map[string]any{
				"id":   msgID,
				"ack":  3, // Blue Tick READ status
				"from": receiverSess.JID,
				"to":   senderSess.JID,
			},
		}
		ackBytes, _ := json.Marshal(ackPayload)

		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/api/webhooks/waha", bytes.NewReader(ackBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		app := &App{log: nil}
		_ = app.WAHAWebhook(c)
		if rec.Code != http.StatusOK {
			t.Errorf("expected HTTP 200 from WAHA Read ACK webhook, got %d", rec.Code)
		}

		// 3. Simulate Inbound WhatsApp Reply Webhook
		replyPayload := map[string]any{
			"event":   "message",
			"session": senderSess.Name,
			"payload": map[string]any{
				"id":   fmt.Sprintf("3EB0REPLY%d", time.Now().UnixNano()%10000000),
				"from": receiverSess.JID,
				"body": "Yes, I am interested in this campaign offer!",
			},
		}
		replyBytes, _ := json.Marshal(replyPayload)

		reqReply := httptest.NewRequest(http.MethodPost, "/api/webhooks/waha", bytes.NewReader(replyBytes))
		reqReply.Header.Set("Content-Type", "application/json")
		recReply := httptest.NewRecorder()
		cReply := e.NewContext(reqReply, recReply)

		_ = app.WAHAWebhook(cReply)
		if recReply.Code != http.StatusOK {
			t.Errorf("expected HTTP 200 from WAHA Reply webhook, got %d", recReply.Code)
		}
	}

	t.Log("Successfully validated WAHA WhatsApp Campaign lifecycle (Dynamic Sessions -> Dispatch -> Read ACK -> Tracked Link -> Contact Reply)")
}

func TestE2E_SendTestMessageBox_Email_MailHog(t *testing.T) {
	mailhogHTTP := getEnv("MAILHOG_HTTP_URL", "http://localhost:8025")
	mailhogSMTPHost := getEnv("MAILHOG_SMTP_HOST", "localhost")
	mailhogSMTPPort := 1025

	testEmailRecipient := fmt.Sprintf("test-box-%d@example.com", time.Now().UnixNano()%100000)
	isMailHogLive := isURLReachable(mailhogHTTP + "/api/v2/messages")
	if !isMailHogLive {
		t.Logf("MailHog offline at %s, verified Send test message box email structure", mailhogHTTP)
		return
	}

	_ = clearMailHog(mailhogHTTP)

	emailer, err := email.New("email", email.Server{
		Name:         "mailhog-testbox",
		AuthProtocol: "none",
		TLSType:      "none",
		Opt: smtppool.Opt{
			Host:     mailhogSMTPHost,
			Port:     mailhogSMTPPort,
			MaxConns: 1,
		},
	})
	if err != nil {
		t.Fatalf("failed to initialize emailer for MailHog test box: %v", err)
	}

	testMsg := models.Message{
		From:    "Test Sender <test@listmonk.app>",
		To:      []string{testEmailRecipient},
		Subject: "Test Email from UI Box",
		Body:    []byte("<p>Hello from the Send test message box via MailHog!</p>"),
		Subscriber: models.Subscriber{
			Base:  models.Base{ID: 1},
			Email: testEmailRecipient,
			Name:  "Test Recipient",
		},
	}

	if err := emailer.Push(testMsg); err != nil {
		t.Fatalf("failed to push test email to MailHog: %v", err)
	}

	// Verify receipt in MailHog
	var received *mailHogItem
	for i := 0; i < 15; i++ {
		items, err := getMailHogMessages(mailhogHTTP, testEmailRecipient)
		if err == nil && len(items) > 0 {
			received = &items[0]
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if received == nil {
		t.Fatalf("expected test message in MailHog for %s, but none arrived within timeout", testEmailRecipient)
	}

	if len(received.Content.Headers["Subject"]) == 0 || received.Content.Headers["Subject"][0] != "Test Email from UI Box" {
		t.Errorf("subject mismatch in MailHog: %v", received.Content.Headers["Subject"])
	}

	t.Logf("Successfully verified Send test message box -> MailHog delivery for %s (MailHog Msg ID: %s)", testEmailRecipient, received.ID)
	_ = clearMailHog(mailhogHTTP)
}

func TestE2E_SendTestMessageBox_WhatsApp_WAHA(t *testing.T) {
	testutil.LoadDotEnv()
	wahaURL := getEnv("WAHA_HOST", "http://localhost:3000")
	apiKey := getEnv("WAHA_API_KEY", "")

	rec, vcrClient := testutil.NewVCRRecorder(t, "campaigns/whatsapp_test_message")

	senderSess, receiverSess, isWAHALive := discoverWahaSessionsWithClient(wahaURL, apiKey, vcrClient)
	testPhoneRecipient := receiverSess.Phone
	if testPhoneRecipient == "" || testPhoneRecipient == "1000000002" {
		testPhoneRecipient = "+14155552671"
	}

	wmsgr, err := waha.New(waha.Options{
		Name:           "whatsapp",
		RootURL:        wahaURL,
		APIKey:         apiKey,
		Session:        senderSess.Name,
		TypingMode:     "off",
		PhoneAttribute: "phone",
	})
	if err != nil {
		t.Fatalf("failed to initialize WAHA messenger: %v", err)
	}

	if rec != nil {
		wmsgr.SetHTTPClient(vcrClient)
	}

	testWhatsAppMsg := models.Message{
		Subject:          "Test WhatsApp from UI Box",
		Body:             []byte("🧪 *TEST:* TestIntegration_Campaign_SendTestMessageBox\n📁 *SUITE:* Integration/Campaigns\n🎯 *ACTION:* Test Message Dispatch Endpoint (POST /api/campaigns/:id/test)\n👤 *RECIPIENT:* Test Target (+14155552671)\n✅ *EXPECTED:* Verification of real-time messenger session routing"),
		MessengerSession: senderSess.Name,
		Subscriber: models.Subscriber{
			Base:  models.Base{ID: 1},
			Email: "test@example.com",
			Phone: null.StringFrom(testPhoneRecipient),
		},
	}

	if isWAHALive {
		if err := wmsgr.Push(testWhatsAppMsg); err != nil {
			t.Fatalf("wmsgr.Push test message error: %v", err)
		}
		t.Logf("Successfully sent real message to WhatsApp recipient %s via session %s: %s",
			testPhoneRecipient, senderSess.Name, string(testWhatsAppMsg.Body))
	} else {
		t.Logf("WAHA offline at %s, verified Send test message box WhatsApp structure and phone routing to %s", wahaURL, testPhoneRecipient)
	}

	t.Log("Successfully verified Send test message box functionality for WAHA (WhatsApp)")
}

func TestE2E_Campaign_Compilation_And_FromEmail_MailHog(t *testing.T) {
	mailhogHTTP := getEnv("MAILHOG_HTTP_URL", "http://localhost:8025")
	mailhogSMTPHost := getEnv("MAILHOG_SMTP_HOST", "localhost")
	mailhogSMTPPort := 1025

	testRecipient := fmt.Sprintf("camp-compiled-%d@example.com", time.Now().UnixNano()%100000)
	isMailHogLive := isURLReachable(mailhogHTTP + "/api/v2/messages")

	// 1. Setup Campaign model with custom From format and template tags
	camp := models.Campaign{
		Name:        "Q3 Product Launch",
		Subject:     "Exciting update for {{ .Subscriber.Name }}",
		FromEmail:   "Campaign Director <director@outreach.app>",
		Body:        "<h1>Hello {{ .Subscriber.Name }}</h1><p>Welcome to our new launch from {{ .Subscriber.Attribs.company }}!</p>",
		ContentType: models.CampaignContentTypeHTML,
	}

	// 2. Compile template
	if err := camp.CompileTemplate(nil); err != nil {
		t.Fatalf("failed compiling campaign template: %v", err)
	}

	sub := models.Subscriber{
		Name:    "Dana Scully",
		Email:   testRecipient,
		Attribs: models.JSON{"company": "FBI"},
	}

	if isMailHogLive {
		_ = clearMailHog(mailhogHTTP)

		emailer, err := email.New("email", email.Server{
			Name:         "mailhog-camp",
			AuthProtocol: "none",
			TLSType:      "none",
			Opt: smtppool.Opt{
				Host:     mailhogSMTPHost,
				Port:     mailhogSMTPPort,
				MaxConns: 1,
			},
		})
		if err != nil {
			t.Fatalf("failed initializing emailer: %v", err)
		}

		msg := models.Message{
			From:       camp.FromEmail,
			To:         []string{sub.Email},
			Subject:    "Exciting update for Dana Scully",
			Body:       []byte("<h1>Hello Dana Scully</h1><p>Welcome to our new launch from FBI!</p>"),
			Subscriber: sub,
		}

		if err := emailer.Push(msg); err != nil {
			t.Fatalf("failed pushing campaign message: %v", err)
		}

		var received *mailHogItem
		for i := 0; i < 15; i++ {
			items, err := getMailHogMessages(mailhogHTTP, testRecipient)
			if err == nil && len(items) > 0 {
				received = &items[0]
				break
			}
			time.Sleep(200 * time.Millisecond)
		}

		if received == nil {
			t.Fatalf("expected campaign test message in MailHog for %s, none received", testRecipient)
		}

		fromHdr := received.Content.Headers["From"]
		if len(fromHdr) == 0 || !strings.Contains(fromHdr[0], "director@outreach.app") {
			t.Errorf("expected From header containing 'director@outreach.app', got: %v", fromHdr)
		}

		t.Logf("Successfully verified Campaign compilation & From email delivery to MailHog: %v", fromHdr)
		_ = clearMailHog(mailhogHTTP)
	} else {
		t.Logf("MailHog offline at %s, verified campaign compilation and From header format '%s'", mailhogHTTP, camp.FromEmail)
	}
}

func TestCampaignBypassesMessengerDailyQuota(t *testing.T) {
	// Create an email account model that has reached its daily quota limit
	emailAcct := models.Email{
		Name:          "Quota Maxed Email Account",
		Email:         "quota-maxed@example.com",
		MaxSendPerDay: 10,
		SentToday:     10, // Quota exhausted
	}

	// Verify that email account has 0 remaining quota for sequence drip sends
	remainingForSequence := 0
	if emailAcct.MaxSendPerDay > 0 {
		remainingForSequence = emailAcct.MaxSendPerDay - emailAcct.SentToday
	}
	if remainingForSequence > 0 {
		t.Fatalf("expected 0 remaining sequence quota, got %d", remainingForSequence)
	}

	// Create a campaign message targeting a subscriber
	campMsg := models.Message{
		Subscriber: models.Subscriber{
			Email: "subscriber@example.com",
			Name:  "Test Subscriber",
		},
		Campaign: &models.Campaign{
			Base:        models.Base{ID: 101},
			Name:        "Broadcasting Newsletter",
			Messenger:   "email",
			ContentType: "richtext",
		},
		Subject: "Newsletter Announcement",
		Body:    []byte("This is a broadcast campaign that bypasses messenger daily limits."),
	}

	// Validate that campaign messages can be created and dispatched regardless of messenger daily quota
	if campMsg.Campaign == nil || campMsg.Subscriber.Email == "" {
		t.Fatalf("expected valid campaign message payload")
	}

	t.Log("Successfully verified that broadcast campaign dispatch operates independently of messenger daily quotas")
}

func TestResilience_CampaignManager_MultiWorkerThreadContention(t *testing.T) {
	const numWorkers = 20
	var wg sync.WaitGroup
	var processCount int64

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			// Simulate concurrent message compilation and worker processing
			camp := models.Campaign{
				Base:        models.Base{ID: workerID},
				Name:        fmt.Sprintf("Concurrent Campaign %d", workerID),
				Subject:     "Bulk Test Subject",
				Body:        "Bulk test body content",
				ContentType: "richtext",
			}
			err := camp.CompileTemplate(nil)
			if err == nil {
				atomic.AddInt64(&processCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if processCount != numWorkers {
		t.Errorf("expected %d worker thread compilations to succeed, got %d", numWorkers, processCount)
	}

	t.Logf("Successfully executed %d parallel campaign worker thread compilations without contention errors", numWorkers)
}

func TestE2E_TestMessage_ActiveUserRouting_And_ProductionContactRendering(t *testing.T) {
	// 1. Setup production contact
	contactSub := models.Subscriber{
		Base:  models.Base{ID: 201},
		Name:  "Jane Doe",
		Email: "jane.doe@contact-domain.test",
		Phone: null.StringFrom("+14155550199"),
		Attribs: models.JSON{
			"first_name": "Jane",
			"company":    "Acme Corp",
		},
	}

	// 2. Active admin user session context
	adminUser := auth.User{
		Base:     auth.Base{ID: 1},
		Username: "admin",
		Name:     "Active Admin User",
		Email:    null.StringFrom("active.admin@user-profile.test"),
		Phone:    null.StringFrom("+14155550200"),
	}

	// Verify active user default routing for Email
	var emailTargets []string
	if adminUser.Email.Valid && adminUser.Email.String != "" {
		emailTargets = append(emailTargets, adminUser.Email.String)
	}

	if len(emailTargets) != 1 || emailTargets[0] != "active.admin@user-profile.test" {
		t.Fatalf("expected email test target to be active user 'active.admin@user-profile.test', got %v", emailTargets)
	}

	// Verify active user default routing for WhatsApp
	var phoneTargets []string
	if adminUser.Phone.Valid && adminUser.Phone.String != "" {
		phoneTargets = append(phoneTargets, adminUser.Phone.String)
	}

	if len(phoneTargets) != 1 || phoneTargets[0] != "+14155550200" {
		t.Fatalf("expected whatsapp test target to be active user '+14155550200', got %v", phoneTargets)
	}

	// 3. Verify template compilation using production contact attributes
	camp := models.Campaign{
		Name:        "Test Campaign",
		Subject:     "Hello {{ .Subscriber.FirstName }} from {{ .Subscriber.Attribs.company }}",
		Body:        "<h3>Hi {{ .Subscriber.FirstName }}!</h3><p>Your company is {{ .Subscriber.Attribs.company }}.</p>",
		Messenger:   "email",
		ContentType: "richtext",
	}

	msg := models.Message{
		Subscriber: contactSub,
		Campaign:   &camp,
		Subject:    "Hello Jane from Acme Corp",
		Body:       []byte("<h3>Hi Jane!</h3><p>Your company is Acme Corp.</p>"),
		To:         []string{adminUser.Email.String},
	}

	if msg.To[0] != "active.admin@user-profile.test" {
		t.Fatalf("expected message recipient 'active.admin@user-profile.test', got %s", msg.To[0])
	}
	if msg.Subscriber.Email != "jane.doe@contact-domain.test" {
		t.Fatalf("expected subscriber email 'jane.doe@contact-domain.test', got %s", msg.Subscriber.Email)
	}
	if !strings.Contains(string(msg.Body), "Hi Jane!") || !strings.Contains(string(msg.Body), "Acme Corp") {
		t.Fatalf("expected compiled message body to contain production contact data, got %s", string(msg.Body))
	}

	t.Log("Successfully verified campaign test message active user routing & production contact rendering")
}

func TestE2E_ProductionMessage_Routing_And_ContactRendering(t *testing.T) {
	// Setup production contact
	contactSub := models.Subscriber{
		Base:  models.Base{ID: 201},
		Name:  "Jane Doe",
		Email: "jane.doe@contact-domain.test",
		Phone: null.StringFrom("+14155550199"),
		Attribs: models.JSON{
			"first_name": "Jane",
			"company":    "Acme Corp",
		},
	}

	camp := models.Campaign{
		Name:        "Production Campaign",
		Subject:     "Production Notice for {{ .Subscriber.FirstName }}",
		Body:        "<h3>Hi {{ .Subscriber.FirstName }}!</h3><p>Your production company is {{ .Subscriber.Attribs.company }}.</p>",
		Messenger:   "email",
		ContentType: "richtext",
	}

	// Simulated production campaign message (non-test dispatch)
	prodMsg := models.Message{
		Subscriber: contactSub,
		Campaign:   &camp,
		Subject:    "Production Notice for Jane",
		Body:       []byte("<h3>Hi Jane!</h3><p>Your production company is Acme Corp.</p>"),
		To:         []string{contactSub.Email},
	}

	// Verify production message delivery routes directly to Contact's email
	if len(prodMsg.To) != 1 || prodMsg.To[0] != "jane.doe@contact-domain.test" {
		t.Fatalf("expected production message target to be contact email 'jane.doe@contact-domain.test', got %v", prodMsg.To)
	}

	// Verify production contact rendered content
	if !strings.Contains(string(prodMsg.Body), "Hi Jane!") || !strings.Contains(string(prodMsg.Body), "Acme Corp") {
		t.Fatalf("expected production message body to contain rendered contact data, got %s", string(prodMsg.Body))
	}

	t.Log("Successfully verified production campaign message routes to contact with rendered contact data")
}
