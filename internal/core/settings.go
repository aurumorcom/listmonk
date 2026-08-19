package core

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jmoiron/sqlx/types"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

// GetSettings returns settings from the DB and enriches with emails table data.
func (c *Core) GetSettings() (models.Settings, error) {
	var (
		b   types.JSONText
		out models.Settings
	)

	if err := c.q.GetSettings.Get(&b); err != nil {
		return out, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching",
				"name", "{globals.terms.settings}", "error", pqErrMsg(err)))
	}

	// Unmarshal the settings.
	if err := json.Unmarshal([]byte(b), &out); err != nil {
		return out, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("settings.errorEncoding", "error", err.Error()))
	}

	// Enrich/Populate SMTP settings from the emails table if present
	emailAccounts, err := c.GetEmails()
	if err == nil && len(emailAccounts) > 0 {
		smtpList := make([]models.SMTPSettings, 0, len(emailAccounts))

		for _, em := range emailAccounts {
			var smtpMap map[string]any
			if len(em.SMTPConfig) > 0 {
				smtpMap = em.SMTPConfig
			}

			optMap, _ := smtpMap["opt"].(map[string]any)

			getString := func(m map[string]any, k string, fallback string) string {
				if m == nil {
					return fallback
				}
				if v, ok := m[k].(string); ok {
					return v
				}
				return fallback
			}
			getInt := func(m map[string]any, k string, fallback int) int {
				if m == nil {
					return fallback
				}
				if v, ok := m[k].(float64); ok {
					return int(v)
				}
				if v, ok := m[k].(int); ok {
					return v
				}
				return fallback
			}
			getBool := func(m map[string]any, k string, fallback bool) bool {
				if m == nil {
					return fallback
				}
				if v, ok := m[k].(bool); ok {
					return v
				}
				return fallback
			}

			host := getString(optMap, "host", getString(smtpMap, "host", ""))
			port := getInt(optMap, "port", getInt(smtpMap, "port", 25))
			username := getString(optMap, "username", getString(smtpMap, "username", ""))
			password := getString(optMap, "password", getString(smtpMap, "password", ""))
			helloHost := getString(optMap, "hello_hostname", getString(smtpMap, "hello_hostname", ""))
			tlsSkip := getBool(optMap, "tls_skip_verify", getBool(smtpMap, "tls_skip_verify", false))
			maxConns := getInt(optMap, "max_conns", getInt(smtpMap, "max_conns", 10))
			maxRetries := getInt(optMap, "max_msg_retries", getInt(smtpMap, "max_msg_retries", 2))
			retryDelay := getString(optMap, "msg_retry_delay", getString(smtpMap, "msg_retry_delay", "0s"))

			var fromAddrs []string
			if rawAddrs, ok := smtpMap["from_addresses"].([]any); ok {
				for _, a := range rawAddrs {
					if str, ok := a.(string); ok && str != "" {
						fromAddrs = append(fromAddrs, str)
					}
				}
			}
			if len(fromAddrs) == 0 && em.Email != "" {
				fromAddrs = []string{em.Email}
			}

			entry := models.SMTPSettings{
				Name:          em.Name,
				Signature:     em.Signature,
				UserID:        em.UserID,
				UUID:          getString(smtpMap, "uuid", ""),
				Enabled:       true,
				Host:          host,
				Port:          port,
				AuthProtocol:  getString(smtpMap, "auth_protocol", "login"),
				Username:      username,
				Password:      password,
				HelloHostname: helloHost,
				TLSType:       getString(smtpMap, "tls_type", "STARTTLS"),
				TLSSkipVerify: tlsSkip,
				MaxConns:      maxConns,
				MaxMsgRetries: maxRetries,
				MsgRetryDelay: retryDelay,
				FromAddresses: fromAddrs,
				MaxSendPerDay: em.MaxSendPerDay,
			}

			smtpList = append(smtpList, entry)
		}
		out.SMTP = smtpList
	}

	// Populate Webhooks settings from the webhooks table
	if webhooks, err := c.GetWebhooks(); err == nil {
		if webhooks == nil {
			webhooks = make([]models.Webhook, 0)
		}
		out.Webhooks = webhooks
	} else {
		out.Webhooks = make([]models.Webhook, 0)
	}

	return out, nil
}

// UpdateSettings updates settings and synchronizes email accounts to the emails table.
func (c *Core) UpdateSettings(s models.Settings) error {
	// Marshal settings.
	b, err := json.Marshal(s)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("settings.errorEncoding", "error", err.Error()))
	}

	// Update the settings in the DB.
	if _, err := c.q.UpdateSettings.Exec(b); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.settings}", "error", pqErrMsg(err)))
	}

	// Synchronize s.SMTP with the emails table
	existingEmails, err := c.GetEmails()
	if err == nil {
		existingByEmail := make(map[string]models.Email)
		for _, e := range existingEmails {
			existingByEmail[strings.ToLower(e.Email)] = e
		}

		activeEmails := make(map[string]bool)

		for _, item := range s.SMTP {
			if !item.Enabled {
				continue
			}

			emailAddr := item.Username
			if len(item.FromAddresses) > 0 && item.FromAddresses[0] != "" {
				emailAddr = item.FromAddresses[0]
			}
			emailAddr = strings.TrimSpace(emailAddr)
			if emailAddr == "" {
				continue
			}

			activeEmails[strings.ToLower(emailAddr)] = true

			smtpCfg := models.JSON{
				"name":           item.Name,
				"uuid":           item.UUID,
				"auth_protocol":  item.AuthProtocol,
				"tls_type":       item.TLSType,
				"from_addresses": item.FromAddresses,
				"email_headers":  item.EmailHeaders,
				"opt": map[string]any{
					"host":            item.Host,
					"port":            item.Port,
					"username":        item.Username,
					"password":        item.Password,
					"hello_hostname":  item.HelloHostname,
					"tls_skip_verify": item.TLSSkipVerify,
					"max_conns":       item.MaxConns,
					"max_msg_retries": item.MaxMsgRetries,
					"msg_retry_delay": item.MsgRetryDelay,
				},
			}

			if existing, ok := existingByEmail[strings.ToLower(emailAddr)]; ok {
				// Update existing
				existing.Name = item.Name
				existing.SMTPConfig = smtpCfg
				existing.MaxSendPerDay = item.MaxSendPerDay
				existing.Signature = item.Signature
				existing.UserID = item.UserID
				_, _ = c.UpdateEmail(existing)
			} else {
				// Create new
				newEmail := models.Email{
					Name:          item.Name,
					Email:         emailAddr,
					SMTPConfig:    smtpCfg,
					MaxSendPerDay: item.MaxSendPerDay,
					Signature:     item.Signature,
					UserID:        item.UserID,
				}
				_, _ = c.CreateEmail(newEmail)
			}
		}

		// Delete obsolete emails
		for _, e := range existingEmails {
			if !activeEmails[strings.ToLower(e.Email)] {
				_ = c.DeleteEmail(e.ID)
			}
		}
	}

	// Synchronize s.Webhooks with the webhooks table
	existingWebhooks, err := c.GetWebhooks()
	if err == nil {
		activeIDs := make(map[int]bool)

		for _, item := range s.Webhooks {
			if item.Events == nil {
				item.Events = make([]string, 0)
			}

			if item.ID > 0 {
				activeIDs[item.ID] = true
				if _, err := c.UpdateWebhook(item.ID, item); err != nil {
					c.log.Printf("error updating webhook %d in settings sync: %v", item.ID, err)
				}
			} else {
				// New webhook endpoint created in settings UI
				created, err := c.CreateWebhook(item)
				if err == nil {
					activeIDs[created.ID] = true
				} else {
					c.log.Printf("error creating webhook in settings sync: %v", err)
				}
			}
		}

		// Delete webhooks removed from the settings UI
		for _, w := range existingWebhooks {
			if !activeIDs[w.ID] {
				if err := c.DeleteWebhook(w.ID); err != nil {
					c.log.Printf("error deleting webhook %d in settings sync: %v", w.ID, err)
				}
			}
		}
	}

	return nil
}

// UpdateSettingsByKey updates a single setting by key.
func (c *Core) UpdateSettingsByKey(key string, value json.RawMessage) error {
	if _, err := c.q.UpdateSettingsByKey.Exec(key, value); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.settings}", "error", pqErrMsg(err)))
	}

	return nil
}
