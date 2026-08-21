package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/messenger/waha"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

// GetBounce handles retrieval of a specific bounce record by ID.
func (a *App) GetBounce(c echo.Context) error {
	// Fetch one bounce from the DB.
	id := getID(c)
	out, err := a.core.GetBounce(id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// GetBounces handles retrieval of bounce records.
func (a *App) GetBounces(c echo.Context) error {
	// campaign_id is optional; default to 0 on missing or invalid param
	campID, _ := strconv.Atoi(c.QueryParam("campaign_id"))
	var (
		source  = c.FormValue("source")
		orderBy = c.FormValue("order_by")
		order   = c.FormValue("order")

		pg = a.pg.NewFromURL(c.Request().URL.Query())
	)

	// Query and fetch bounces from the DB.
	res, total, err := a.core.QueryBounces(campID, 0, source, orderBy, order, pg.Offset, pg.Limit)
	if err != nil {
		return err
	}

	// No results.
	if len(res) == 0 {
		var emptyBounces []models.Bounce
		return c.JSON(http.StatusOK, okResp{models.PageResults{Results: emptyBounces}})
	}

	out := models.PageResults{
		Results: res,
		Total:   total,
		Page:    pg.Page,
		PerPage: pg.PerPage,
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// GetSubscriberBounces retrieves a subscriber's bounce records.
func (a *App) GetSubscriberBounces(c echo.Context) error {
	subID := getID(c)

	// Check if the user has access to at least one of the lists on the subscriber.
	if err := a.hasSubPerm(auth.GetUser(c), []int{subID}); err != nil {
		return err
	}

	// Query and fetch bounces from the DB.
	out, _, err := a.core.QueryBounces(0, subID, "", "", "", 0, 1000)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// DeleteBounces handles bounce deletion of a list.
func (a *App) DeleteBounces(c echo.Context) error {
	all, _ := strconv.ParseBool(c.QueryParam("all"))

	var ids []int
	if !all {
		// There are multiple IDs in the query string.
		res, err := parseStringIDs(c.Request().URL.Query()["id"])
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidID", "error", err.Error()))
		}
		if len(res) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidID"))
		}

		ids = res
	}

	// Delete bounces from the DB.
	if err := a.core.DeleteBounces(ids, all); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{true})
}

// DeleteBounce handles bounce deletion of a single bounce record.
func (a *App) DeleteBounce(c echo.Context) error {
	// Delete bounces from the DB.
	id := getID(c)
	if err := a.core.DeleteBounces([]int{id}, false); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{true})
}

// BlocklistBouncedSubscribers handles blocklisting of all bounced subscribers.
func (a *App) BlocklistBouncedSubscribers(c echo.Context) error {
	if err := a.core.BlocklistBouncedSubscribers(); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{true})
}

// BounceWebhook handles incoming bounce webhook notifications from various providers.
func (a *App) BounceWebhook(c echo.Context) error {
	// If bounce processing is disabled, a.bounce will be nil.
	// Return early to prevent nil pointer dereference.
	if a.bounce == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable,
			a.i18n.Ts("globals.messages.internalError"))
	}

	// Read the request body instead of using c.Bind() to read to save the entire raw request as meta.
	rawReq, err := io.ReadAll(c.Request().Body)
	if err != nil {
		a.log.Printf("error reading ses notification body: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.internalError"))
	}

	var (
		service = c.Param("service")

		bounces []models.Bounce
	)
	switch true {
	// Native internal webhook.
	case service == "":
		var b models.Bounce
		if err := json.Unmarshal(rawReq, &b); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidData")+":"+err.Error())
		}

		if bv, err := a.validateBounceFields(b); err != nil {
			return err
		} else {
			b = bv
		}

		if len(b.Meta) == 0 {
			b.Meta = json.RawMessage("{}")
		}

		if b.CreatedAt.Year() == 0 {
			b.CreatedAt = time.Now()
		}

		bounces = append(bounces, b)

	// Amazon SES.
	case service == "ses" && a.bounce.SES != nil:
		switch c.Request().Header.Get("X-Amz-Sns-Message-Type") {
		// SNS webhook registration confirmation. Only after these are processed will the endpoint
		// start getting bounce notifications.
		case "SubscriptionConfirmation", "UnsubscribeConfirmation":
			if err := a.bounce.SES.ProcessSubscription(rawReq); err != nil {
				a.log.Printf("error processing SNS (SES) subscription: %v", err)
				return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidData"))
			}

		// Bounce notification.
		case "Notification":
			b, err := a.bounce.SES.ProcessBounce(rawReq)
			if err != nil {
				a.log.Printf("error processing SES notification: %v", err)
				return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidData"))
			}
			bounces = append(bounces, b)

		default:
			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidData"))
		}

	// Azure ACS through Event Grid.
	case service == "azure" && a.bounce.Azure != nil:
		switch c.Request().Header.Get("aeg-event-type") {
		// Event Grid webhook registration validation.
		case "SubscriptionValidation", "SubscriptionValidationEvent":
			res, err := a.bounce.Azure.ProcessSubscription(rawReq)
			if err != nil {
				a.log.Printf("error processing Azure Event Grid subscription validation: %v", err)
				return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidData"))
			}
			return c.JSONBlob(http.StatusOK, res)

		// Regular event delivery.
		case "", "Notification":
			bs, err := a.bounce.Azure.ProcessBounce(c.Request(), rawReq)
			if err != nil {
				a.log.Printf("error processing Azure Event Grid notification: %v", err)
				return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidData"))
			}
			bounces = append(bounces, bs...)

		default:
			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidData"))
		}

	// SendGrid.
	case service == "sendgrid" && a.bounce.Sendgrid != nil:
		var (
			sig = c.Request().Header.Get("X-Twilio-Email-Event-Webhook-Signature")
			ts  = c.Request().Header.Get("X-Twilio-Email-Event-Webhook-Timestamp")
		)

		// Sendgrid sends multiple bounces.
		bs, err := a.bounce.Sendgrid.ProcessBounce(sig, ts, rawReq)
		if err != nil {
			a.log.Printf("error processing sendgrid notification: %v", err)
			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidData"))
		}
		bounces = append(bounces, bs...)

	// Postmark.
	case service == "postmark" && a.bounce.Postmark != nil:
		bs, err := a.bounce.Postmark.ProcessBounce(rawReq, c)
		if err != nil {
			a.log.Printf("error processing postmark notification: %v", err)
			if _, ok := err.(*echo.HTTPError); ok {
				return err
			}

			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidData"))
		}
		bounces = append(bounces, bs...)

	// ForwardEmail.
	case service == "forwardemail" && a.bounce.Forwardemail != nil:
		var (
			sig = c.Request().Header.Get("X-Webhook-Signature")
		)

		bs, err := a.bounce.Forwardemail.ProcessBounce(sig, rawReq)
		if err != nil {
			a.log.Printf("error processing forwardemail notification: %v", err)
			if _, ok := err.(*echo.HTTPError); ok {
				return err
			}

			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidData"))
		}
		bounces = append(bounces, bs...)

	// Lettermint.
	case service == "lettermint" && a.bounce.Lettermint != nil:
		sig := c.Request().Header.Get("X-Lettermint-Signature")
		bs, err := a.bounce.Lettermint.ProcessBounce(sig, rawReq)
		if err != nil {
			a.log.Printf("error processing lettermint notification: %v", err)
			if _, ok := err.(*echo.HTTPError); ok {
				return err
			}

			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidData"))
		}
		bounces = append(bounces, bs...)

	default:
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("bounces.unknownService"))
	}

	// Insert bounces into the DB.
	for _, b := range bounces {
		if err := a.bounce.Record(b); err != nil {
			a.log.Printf("error recording bounce: %v", err)
		}
	}

	return c.JSON(http.StatusOK, okResp{true})
}

func (a *App) validateBounceFields(b models.Bounce) (models.Bounce, error) {
	if b.Email == "" && b.SubscriberUUID == "" {
		return b, echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidFields", "name", "email / subscriber_uuid"))
	}

	if b.SubscriberUUID != "" && !reUUID.MatchString(b.SubscriberUUID) {
		return b, echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidFields", "name", "subscriber_uuid"))
	}

	if b.Email != "" {
		em, err := a.importer.SanitizeEmail(b.Email)
		if err != nil {
			return b, echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		b.Email = em
	}

	if b.Type != models.BounceTypeHard && b.Type != models.BounceTypeSoft && b.Type != models.BounceTypeComplaint {
		return b, echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidFields", "name", "type"))
	}

	return b, nil
}

// ParseWAHAAckLevel extracts numeric ACK level from WAHA payload (2=DEVICE delivery, 3=READ blue tick, 4=PLAYED).
func ParseWAHAAckLevel(ack any, ackName string) int {
	ackLevel := 0
	if ack != nil {
		switch v := ack.(type) {
		case float64:
			ackLevel = int(v)
		case int:
			ackLevel = v
		case string:
			s := strings.ToUpper(strings.TrimSpace(v))
			if strings.Contains(s, "READ") || strings.Contains(s, "PLAYED") || strings.Contains(s, "ACK_READ") {
				ackLevel = 3
			} else if strings.Contains(s, "DEVICE") || strings.Contains(s, "DELIVERED") {
				ackLevel = 2
			} else if s == "-1" || strings.Contains(s, "ERR") {
				ackLevel = -1
			}
		}
	}
	if ackLevel == 0 && ackName != "" {
		s := strings.ToUpper(strings.TrimSpace(ackName))
		if strings.Contains(s, "READ") || strings.Contains(s, "PLAYED") || strings.Contains(s, "ACK_READ") {
			ackLevel = 3
		} else if strings.Contains(s, "DEVICE") || strings.Contains(s, "DELIVERED") {
			ackLevel = 2
		} else if s == "-1" || strings.Contains(s, "ERR") {
			ackLevel = -1
		}
	}
	return ackLevel
}

// WAHAWebhook handles delivery status webhooks from WAHA.
func (a *App) WAHAWebhook(c echo.Context) error {
	if a.log != nil {
		a.log.Printf("[WAHA WEBHOOK] Incoming POST /api/webhooks/waha")
	}

	type wahaPayload struct {
		Event   string `json:"event"`
		Session string `json:"session"`
		Payload struct {
			ID          string `json:"id"`
			FromMe      bool   `json:"fromMe"`
			Ack         any    `json:"ack"`
			AckName     string `json:"ackName"`
			ChatID      string `json:"chatId"`
			From        string `json:"from"`
			To          string `json:"to"`
			Participant string `json:"participant"`
			Error       string `json:"error"`
			Body        string `json:"body"`
			Data        struct {
				ID struct {
					Serialized string `json:"_serialized"`
					ID         string `json:"id"`
				} `json:"id"`
				QuotedMsg struct {
					ID string `json:"id"`
				} `json:"quotedMsg"`
			} `json:"_data"`
		} `json:"payload"`
	}

	// Read raw request body safely without consuming reader destructively
	var rawBody []byte
	if c.Request() != nil && c.Request().Body != nil {
		rawBody, _ = io.ReadAll(c.Request().Body)
		c.Request().Body = io.NopCloser(bytes.NewReader(rawBody))
	}

	var req wahaPayload
	if err := c.Bind(&req); err != nil {
		if a.log != nil {
			a.log.Printf("[WAHA WEBHOOK ERROR] JSON bind failed: %v", err)
		}
		return c.NoContent(http.StatusOK)
	}

	// Parse ACK status into numeric level
	ackLevel := ParseWAHAAckLevel(req.Payload.Ack, req.Payload.AckName)

	msgID := req.Payload.ID
	if msgID == "" {
		msgID = req.Payload.Data.ID.Serialized
	}
	stanzaID := req.Payload.Data.ID.ID
	if stanzaID == "" && msgID != "" {
		if idx := strings.LastIndex(msgID, "_"); idx >= 0 && idx < len(msgID)-1 {
			stanzaID = msgID[idx+1:]
		} else {
			stanzaID = msgID
		}
	}

	rawRecipient := req.Payload.To
	if rawRecipient == "" {
		rawRecipient = req.Payload.ChatID
	}
	if rawRecipient == "" {
		rawRecipient = req.Payload.Participant
	}
	if rawRecipient == "" {
		rawRecipient = req.Payload.From
	}

	lid := ""
	if strings.Contains(rawRecipient, "@lid") {
		lid = rawRecipient
	} else if strings.Contains(req.Payload.From, "@lid") {
		lid = req.Payload.From
	}

	if req.Event == "message.ack" && ackLevel == -1 {
		if a.log != nil {
			target := rawRecipient
			a.log.Printf("WAHA delivery failure for %s: %s", target, req.Payload.Error)
		}
	} else if req.Event == "message.ack" && ackLevel >= 3 {
		targetPhone := rawRecipient
		if targetPhone != "" {
			targetPhone = strings.TrimSuffix(targetPhone, "@c.us")
			targetPhone = strings.TrimSuffix(targetPhone, "@s.whatsapp.net")
			if idx := strings.Index(targetPhone, ":"); idx >= 0 {
				targetPhone = targetPhone[:idx]
			}
		}

		// Attempt WAHA API LID resolution to convert @lid JID to real phone number
		if lid != "" {
			if resolved, err := waha.ResolveLID(req.Session, lid); err == nil && resolved != "" {
				if a.log != nil {
					a.log.Printf("[WAHA WEBHOOK LID RESOLVED] Resolved LID %s -> Phone %s", lid, resolved)
				}
				if a.core != nil {
					_ = a.core.LinkSubscriberLIDByPhone(resolved, lid)
				}
				if targetPhone == "" || strings.Contains(targetPhone, "@lid") {
					targetPhone = resolved
				}
			}
		}

		if a.log != nil {
			a.log.Printf("[WAHA WEBHOOK READ ACK] Blue Tick received (ackLevel=%d) for msgID=%s targetPhone=%s", ackLevel, msgID, targetPhone)
		}

		if a.core != nil {
			if msgID != "" || stanzaID != "" {
				_ = a.core.RecordSequenceReadByMessageID(msgID, stanzaID, lid)
			}
			if targetPhone != "" || lid != "" {
				_ = a.core.RecordSequenceReadByPhone(targetPhone, lid)
			}
		}
	} else if req.Event == "message" && !req.Payload.FromMe && req.Payload.From != "" {
		fromIdentifier := req.Payload.From
		quotedID := req.Payload.Data.QuotedMsg.ID

		// Attempt WAHA API LID resolution to convert @lid JID to real phone number
		if lid != "" {
			if resolved, err := waha.ResolveLID(req.Session, lid); err == nil && resolved != "" {
				if a.log != nil {
					a.log.Printf("[WAHA WEBHOOK LID RESOLVED] Resolved LID %s -> Phone %s", lid, resolved)
				}
				if a.core != nil {
					_ = a.core.LinkSubscriberLIDByPhone(resolved, lid)
				}
				fromIdentifier = resolved
			}
		}

		if a.log != nil {
			a.log.Printf("[WAHA WEBHOOK INBOUND REPLY] Incoming message from %s (quotedMsgID: %s)", fromIdentifier, quotedID)
		}
		if a.core != nil {
			_ = a.core.RecordSequenceReplyByPhone(fromIdentifier)
		}
	}

	return c.NoContent(http.StatusOK)
}
