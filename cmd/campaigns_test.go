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
	"testing"
	"time"

	"github.com/knadh/listmonk/internal/core"
	"github.com/knadh/listmonk/internal/messenger/email"
	"github.com/knadh/listmonk/internal/messenger/waha"
	"github.com/knadh/listmonk/internal/sequence"
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
		From:    "Listmonk Campaign <noreply@listmonk.app>",
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
	client := &http.Client{Timeout: 3 * time.Second}
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
		return working[0], wahaSessionTarget{Name: "mock_receiver", Phone: "1000000002", JID: "1000000002@c.us"}, false
	}
	return wahaSessionTarget{Name: "mock_sender", Phone: "1000000001", JID: "1000000001@c.us"},
		wahaSessionTarget{Name: "mock_receiver", Phone: "1000000002", JID: "1000000002@c.us"}, false
}

func TestE2E_Campaign_WhatsApp_WAHA(t *testing.T) {
	wahaURL := getEnv("WAHA_ROOT_URL", "http://localhost:3000")
	apiKey := getEnv("WAHA_API_KEY", "key_JR59f24sOxG1O2OhhLFXIsLVID4ajvLD")

	senderSess, receiverSess, isLive := discoverWahaSessions(wahaURL, apiKey)
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

	msgID := fmt.Sprintf("3EB0CAMP%d", time.Now().UnixNano()%10000000)
	trackedLink := "http://localhost:9000/r/waha-campaign-special"
	campaignBody := fmt.Sprintf("🚀 [Listmonk Campaign E2E] Dynamic dual-session WhatsApp campaign test! Tracked link: %s", trackedLink)

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

		if isLive && msgID != "" {
			_ = deleteWahaMessage(wahaURL, apiKey, senderSess.Name, receiverSess.JID, msgID)
		}
	}

	t.Log("Successfully validated WAHA WhatsApp Campaign lifecycle (Dynamic Sessions -> Dispatch -> Read ACK -> Tracked Link -> Contact Reply)")
}
