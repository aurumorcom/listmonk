package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/gdgvda/cron"
	"github.com/gofrs/uuid/v5"
	"github.com/jmoiron/sqlx/types"
	koanfjson "github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/internal/messenger/email"
	"github.com/knadh/listmonk/internal/messenger/waha"
	"github.com/knadh/listmonk/internal/notifs"
	"github.com/knadh/listmonk/internal/sequence"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

const pwdMask = "•"

type aboutHost struct {
	OS       string `json:"os"`
	Machine  string `json:"arch"`
	Hostname string `json:"hostname"`
}

type aboutSystem struct {
	NumCPU  int    `json:"num_cpu"`
	AllocMB uint64 `json:"memory_alloc_mb"`
	OSMB    uint64 `json:"memory_from_os_mb"`
}

type about struct {
	Version   string         `json:"version"`
	Build     string         `json:"build"`
	GoVersion string         `json:"go_version"`
	GoArch    string         `json:"go_arch"`
	Database  types.JSONText `json:"database"`
	System    aboutSystem    `json:"system"`
	Host      aboutHost      `json:"host"`
}

var (
	reAlphaNum = regexp.MustCompile(`[^a-z0-9\-]`)
)

// GetSettings returns settings from the DB.
func (a *App) GetSettings(c echo.Context) error {
	s, err := a.core.GetSettings()
	if err != nil {
		return err
	}

	// Empty out passwords and prepopulate empty duration defaults.
	for i := range s.SMTP {
		s.SMTP[i].Password = strings.Repeat(pwdMask, utf8.RuneCountInString(s.SMTP[i].Password))
		if strings.TrimSpace(s.SMTP[i].IdleTimeout) == "" {
			s.SMTP[i].IdleTimeout = "15s"
		}
		if strings.TrimSpace(s.SMTP[i].WaitTimeout) == "" {
			s.SMTP[i].WaitTimeout = "5s"
		}
		if strings.TrimSpace(s.SMTP[i].MsgRetryDelay) == "" {
			s.SMTP[i].MsgRetryDelay = "0s"
		}
	}
	for i := range s.BounceBoxes {
		s.BounceBoxes[i].Password = strings.Repeat(pwdMask, utf8.RuneCountInString(s.BounceBoxes[i].Password))
	}
	for i := range s.Messengers {
		s.Messengers[i].Password = strings.Repeat(pwdMask, utf8.RuneCountInString(s.Messengers[i].Password))
	}
	for i := range s.WAHASettings {
		s.WAHASettings[i].APIKey = strings.Repeat(pwdMask, utf8.RuneCountInString(s.WAHASettings[i].APIKey))
	}

	s.UploadS3AwsSecretAccessKey = strings.Repeat(pwdMask, utf8.RuneCountInString(s.UploadS3AwsSecretAccessKey))
	s.SendgridKey = strings.Repeat(pwdMask, utf8.RuneCountInString(s.SendgridKey))
	s.BounceAzure.SharedSecret = strings.Repeat(pwdMask, utf8.RuneCountInString(s.BounceAzure.SharedSecret))
	s.BouncePostmark.Password = strings.Repeat(pwdMask, utf8.RuneCountInString(s.BouncePostmark.Password))
	s.BounceForwardEmail.Key = strings.Repeat(pwdMask, utf8.RuneCountInString(s.BounceForwardEmail.Key))
	s.BounceLettermint.Key = strings.Repeat(pwdMask, utf8.RuneCountInString(s.BounceLettermint.Key))
	s.SecurityCaptcha.HCaptcha.Secret = strings.Repeat(pwdMask, utf8.RuneCountInString(s.SecurityCaptcha.HCaptcha.Secret))
	s.OIDC.ClientSecret = strings.Repeat(pwdMask, utf8.RuneCountInString(s.OIDC.ClientSecret))

	return c.JSON(http.StatusOK, okResp{s})
}

// UpdateSettings returns settings from the DB.
func (a *App) UpdateSettings(c echo.Context) error {
	// Unmarshal and marshal the fields once to sanitize the settings blob.
	var set models.Settings
	if err := c.Bind(&set); err != nil {
		return err
	}

	// Get the existing settings.
	cur, err := a.core.GetSettings()
	if err != nil {
		return err
	}

	// Validate and sanitize postback Messenger names along with SMTP names
	// (where each SMTP is also considered as a standalone messenger).
	// Duplicates are disallowed and "email" is a reserved name.
	names := map[string]bool{emailMsgr: true}

	// There should be at least one SMTP block that's enabled.
	has := false
	for i, s := range set.SMTP {
		if s.Enabled {
			has = true
		}

		// Sanitize and normalize the SMTP server name.
		name := reAlphaNum.ReplaceAllString(strings.ToLower(strings.TrimSpace(s.Name)), "-")
		if name != "" {
			if !strings.HasPrefix(name, "email-") {
				name = "email-" + name
			}

			if _, ok := names[name]; ok {
				return echo.NewHTTPError(http.StatusBadRequest,
					a.i18n.Ts("settings.duplicateMessengerName", "name", name))
			}

			names[name] = true
		}
		set.SMTP[i].Name = name

		// Assign a UUID. The frontend only sends a password when the user explicitly
		// changes the password. In other cases, the existing password in the DB
		// is copied while updating the settings and the UUID is used to match
		// the incoming array of SMTP blocks with the array in the DB.
		if s.UUID == "" {
			set.SMTP[i].UUID = uuid.Must(uuid.NewV4()).String()
		}

		// Ensure the HOST is trimmed of any whitespace.
		// This is a common mistake when copy-pasting SMTP settings.
		set.SMTP[i].Host = strings.TrimSpace(s.Host)

		// Sanitize & default SMTP duration fields if empty.
		if strings.TrimSpace(s.IdleTimeout) == "" {
			set.SMTP[i].IdleTimeout = "15s"
		} else if _, err := time.ParseDuration(s.IdleTimeout); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("settings.invalidDuration", "field", "idle_timeout"))
		}

		if strings.TrimSpace(s.WaitTimeout) == "" {
			set.SMTP[i].WaitTimeout = "5s"
		} else if _, err := time.ParseDuration(s.WaitTimeout); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("settings.invalidDuration", "field", "wait_timeout"))
		}

		if strings.TrimSpace(s.MsgRetryDelay) == "" {
			set.SMTP[i].MsgRetryDelay = "0s"
		} else if _, err := time.ParseDuration(s.MsgRetryDelay); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("settings.invalidDuration", "field", "msg_retry_delay"))
		}

		// If there's no password coming in from the frontend or it's masked, copy the existing
		// password by matching UUID, Name, or Username.
		if s.Password == "" || strings.Contains(s.Password, pwdMask) {
			for _, c := range cur.SMTP {
				if s.UUID == c.UUID || (s.Name != "" && s.Name == c.Name) || (s.Username != "" && s.Username == c.Username) {
					set.SMTP[i].Password = c.Password
					break
				}
			}
		}
	}
	if !has {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("settings.errorNoSMTP"))
	}

	// Normalize `from_addresses``. Values are either an e-mail address
	// or an FQDN. Duplicate domains across server blocks are allowed
	// (they get round-robin'd while sending).
	for i, s := range set.SMTP {
		if !s.Enabled {
			continue
		}

		addrs := make([]string, 0, len(s.FromAddresses))
		for _, addr := range s.FromAddresses {
			if k := email.NormalizeAddr(addr); k != "" {
				addrs = append(addrs, k)
			}
		}
		set.SMTP[i].FromAddresses = addrs
	}

	// Always remove the trailing slash from the app root URL.
	set.AppRootURL = strings.TrimRight(set.AppRootURL, "/")

	// Bounce boxes.
	for i, s := range set.BounceBoxes {
		// Assign a UUID. The frontend only sends a password when the user explicitly
		// changes the password. In other cases, the existing password in the DB
		// is copied while updating the settings and the UUID is used to match
		// the incoming array of blocks with the array in the DB.
		if s.UUID == "" {
			set.BounceBoxes[i].UUID = uuid.Must(uuid.NewV4()).String()
		}

		// Ensure the HOST is trimmed of any whitespace.
		// This is a common mistake when copy-pasting SMTP settings.
		set.BounceBoxes[i].Host = strings.TrimSpace(s.Host)

		if d, _ := time.ParseDuration(s.ScanInterval); d.Minutes() < 1 {
			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("settings.bounces.invalidScanInterval"))
		}

		// If there's no password coming in from the frontend, copy the existing
		// password by matching the UUID.
		if s.Password == "" {
			for _, c := range cur.BounceBoxes {
				if s.UUID == c.UUID {
					set.BounceBoxes[i].Password = c.Password
				}
			}
		}
	}

	for i, m := range set.Messengers {
		// UUID to keep track of password changes similar to the SMTP logic above.
		if m.UUID == "" {
			set.Messengers[i].UUID = uuid.Must(uuid.NewV4()).String()
		}

		if m.Password == "" {
			for _, c := range cur.Messengers {
				if m.UUID == c.UUID {
					set.Messengers[i].Password = c.Password
				}
			}
		}

		name := reAlphaNum.ReplaceAllString(strings.ToLower(m.Name), "")
		if _, ok := names[name]; ok {
			return echo.NewHTTPError(http.StatusBadRequest,
				a.i18n.Ts("settings.duplicateMessengerName", "name", name))
		}
		if len(name) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("settings.invalidMessengerName"))
		}

		set.Messengers[i].Name = name
		names[name] = true
	}

	for i, m := range set.WAHASettings {
		if m.UUID == "" {
			set.WAHASettings[i].UUID = uuid.Must(uuid.NewV4()).String()
		}

		if m.APIKey == "" {
			for _, c := range cur.WAHASettings {
				if m.UUID == c.UUID {
					set.WAHASettings[i].APIKey = c.APIKey
				}
			}
		}

		rawName := strings.TrimSpace(m.Name)
		if rawName == "" {
			if m.Session != "" && m.Session != "default" {
				rawName = "whatsapp-" + m.Session
			} else {
				rawName = "whatsapp"
			}
		}

		name := reAlphaNum.ReplaceAllString(strings.ToLower(rawName), "-")
		if name == "" {
			name = "whatsapp"
		}
		if _, ok := names[name]; ok {
			return echo.NewHTTPError(http.StatusBadRequest,
				a.i18n.Ts("settings.duplicateMessengerName", "name", name))
		}

		set.WAHASettings[i].Name = name
		names[name] = true
	}

	// S3 password?
	if set.UploadS3AwsSecretAccessKey == "" {
		set.UploadS3AwsSecretAccessKey = cur.UploadS3AwsSecretAccessKey
	}
	if set.SendgridKey == "" {
		set.SendgridKey = cur.SendgridKey
	}
	if set.BounceAzure.SharedSecret == "" {
		set.BounceAzure.SharedSecret = cur.BounceAzure.SharedSecret
	}
	if set.BouncePostmark.Password == "" {
		set.BouncePostmark.Password = cur.BouncePostmark.Password
	}
	if set.BounceForwardEmail.Key == "" {
		set.BounceForwardEmail.Key = cur.BounceForwardEmail.Key
	}
	if set.BounceLettermint.Key == "" {
		set.BounceLettermint.Key = cur.BounceLettermint.Key
	}
	if set.SecurityCaptcha.HCaptcha.Secret == "" {
		set.SecurityCaptcha.HCaptcha.Secret = cur.SecurityCaptcha.HCaptcha.Secret
	}
	if set.OIDC.ClientSecret == "" {
		set.OIDC.ClientSecret = cur.OIDC.ClientSecret
	}

	// OIDC user auto-creation is enabled. Validate.
	if set.OIDC.AutoCreateUsers {
		if set.OIDC.DefaultUserRoleID.Int < auth.SuperAdminRoleID {
			return echo.NewHTTPError(http.StatusBadRequest,
				a.i18n.Ts("globals.messages.invalidFields", "name", a.i18n.T("settings.security.OIDCDefaultRole")))
		}
	}

	for n, v := range set.UploadExtensions {
		set.UploadExtensions[n] = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(v), "."))
	}

	// Domain blocklist / allowlist.
	doms := make([]string, 0, len(set.DomainBlocklist))
	for _, d := range set.DomainBlocklist {
		if d = strings.TrimSpace(strings.ToLower(d)); d != "" {
			doms = append(doms, d)
		}
	}
	set.DomainBlocklist = doms

	doms = make([]string, 0, len(set.DomainAllowlist))
	for _, d := range set.DomainAllowlist {
		if d = strings.TrimSpace(strings.ToLower(d)); d != "" {
			doms = append(doms, d)
		}
	}
	set.DomainAllowlist = doms

	// Validate and clean trusted URLs.
	urls := make([]string, 0, len(set.SecurityTrustedURLs))
	for _, d := range set.SecurityTrustedURLs {
		if d = strings.TrimSpace(d); d != "" {
			if d == "*" {
				urls = append(urls, d)
				continue
			}

			// Parse and validate the URL.
			u, err := url.Parse(d)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return echo.NewHTTPError(http.StatusBadRequest,
					a.i18n.Ts("globals.messages.invalidData")+": invalid trusted URL: "+d)
			}
			urls = append(urls, d)
		}
	}
	set.SecurityTrustedURLs = urls

	// Validate slow query caching cron.
	if set.CacheSlowQueries {
		if _, err := cron.ParseStandard(set.CacheSlowQueriesInterval); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidData")+": slow query cron: "+err.Error())
		}
	}

	// Update the settings in the DB and synchronize email accounts.
	if err := a.core.UpdateSettings(set); err != nil {
		return err
	}

	// Dynamically refresh messengers with updated accounts.
	a.messengers = append(append(initSMTPMessengers(a.core), initPostbackMessengers(ko)...), initWAHAMessengers(a.core, ko)...)
	for _, m := range a.messengers {
		if m.Name() == "email" {
			if em, ok := m.(*email.Emailer); ok {
				a.emailMsgr = em
			}
		}
	}
	if a.manager != nil {
		for _, m := range a.messengers {
			a.manager.AddMessenger(m)
		}
	}
	if a.seqManager != nil {
		a.seqManager.Stop()
		msgrMap := make(map[string]manager.Messenger)
		for _, m := range a.messengers {
			msgrMap[m.Name()] = m
		}
		a.seqManager = sequence.NewManager(a.core, msgrMap, a.media, a.log)
		if a.manager != nil && a.manager.HasRunningCampaigns() {
			go a.seqManager.Start(1 * time.Minute)
		}
	}

	// Sync WAHA webhooks for any enabled WAHA messengers.
	rootURL := set.AppRootURL
	if rootURL == "" && a.urlCfg != nil {
		rootURL = a.urlCfg.RootURL
	}
	for i := range set.WAHASettings {
		if set.WAHASettings[i].Host == "" && set.WAHASettings[i].RootURL != "" {
			set.WAHASettings[i].Host = set.WAHASettings[i].RootURL
		}
		if set.WAHASettings[i].RootURL == "" && set.WAHASettings[i].Host != "" {
			set.WAHASettings[i].RootURL = set.WAHASettings[i].Host
		}
	}

	for _, wm := range set.WAHASettings {
		if wm.Enabled {
			wHost := wm.Host
			if wHost == "" {
				wHost = wm.RootURL
			}
			dur, _ := time.ParseDuration(wm.Timeout)
			o := waha.Options{
				Name:              wm.Name,
				Host:              wHost,
				RootURL:           wHost,
				APIKey:            wm.APIKey,
				Session:           wm.Session,
				PhoneAttribute:    wm.PhoneAttribute,
				TypingDelayMs:     wm.TypingDelayMs,
				TargetWPM:         wm.TargetWPM,
				WPMStd:            wm.WPMStd,
				KeyboardLayout:    wm.KeyboardLayout,
				TypingMode:        wm.TypingMode,
				MaxTypingDelaySec: wm.MaxTypingDelaySec,
				MaxConns:          wm.MaxConns,
				Timeout:           dur,
			}
			if w, err := waha.New(o); err == nil {
				_ = w.SyncWebhook(rootURL)
			}
		}
	}

	return a.handleSettingsRestart(c)
}

// UpdateSettingsByKey updates a single setting key-value in the DB.
func (a *App) UpdateSettingsByKey(c echo.Context) error {
	key := c.Param("key")
	if key == "" {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidData"))
	}

	// Read the raw JSON body as the value.
	var b json.RawMessage
	if err := c.Bind(&b); err != nil {
		return err
	}

	// Update the value in the DB.
	if err := a.core.UpdateSettingsByKey(key, b); err != nil {
		return err
	}

	return a.handleSettingsRestart(c)
}

// handleSettingsRestart checks for running campaigns and either triggers an
// immediate app restart or marks the app as needing a restart.
func (a *App) handleSettingsRestart(c echo.Context) error {
	// If there are any active campaigns, don't do an auto reload and
	// warn the user on the frontend.
	if a.manager.HasRunningCampaigns() {
		a.Lock()
		a.needsRestart = true
		a.Unlock()

		return c.JSON(http.StatusOK, okResp{struct {
			NeedsRestart bool `json:"needs_restart"`
		}{true}})
	}

	// No running campaigns. Reload the app.
	go func() {
		<-time.After(time.Millisecond * 500)
		a.chReload <- syscall.SIGHUP
	}()

	return c.JSON(http.StatusOK, okResp{true})
}

// GetLogs returns the log entries stored in the log buffer.
func (a *App) GetLogs(c echo.Context) error {
	return c.JSON(http.StatusOK, okResp{a.bufLog.Lines()})
}

// TestSMTPSettings returns the log entries stored in the log buffer.
func (a *App) TestSMTPSettings(c echo.Context) error {
	// Copy the raw JSON post body.
	reqBody, err := io.ReadAll(c.Request().Body)
	if err != nil {
		a.log.Printf("error reading SMTP test: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.internalError"))
	}

	// Load the JSON into koanf to parse SMTP settings properly including timestrings.
	ko := koanf.New(".")
	if err := ko.Load(rawbytes.Provider(reqBody), koanfjson.Parser()); err != nil {
		a.log.Printf("error unmarshalling SMTP test request: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.internalError"))
	}

	req := email.Server{}
	if err := ko.UnmarshalWithConf("", &req, koanf.UnmarshalConf{Tag: "json"}); err != nil {
		a.log.Printf("error scanning SMTP test request: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.internalError"))
	}

	to := ko.String("email")
	if to == "" {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.missingFields", "name", "email"))
	}

	// Initialize a new SMTP pool.
	req.MaxConns = 1
	req.IdleTimeout = time.Second * 2
	req.PoolWaitTimeout = time.Second * 2
	msgr, err := email.New("", req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("globals.messages.errorCreating", "name", "SMTP", "error", err.Error()))
	}

	// Render the test email template body.
	var b bytes.Buffer
	if err := notifs.Tpls.ExecuteTemplate(&b, "smtp-test", nil); err != nil {
		a.log.Printf("error compiling notification template '%s': %v", "smtp-test", err)
		return err
	}

	m := models.Message{}
	m.From = a.cfg.FromEmail
	m.To = []string{to}
	m.Subject = a.i18n.T("settings.smtp.testConnection")
	m.Body = b.Bytes()
	if err := msgr.Push(m); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, okResp{a.bufLog.Lines()})
}

// TestWAHASettings sends a test WhatsApp message using the provided WAHA server configuration.
func (a *App) TestWAHASettings(c echo.Context) error {
	var req struct {
		waha.Options
		Phone string `json:"phone"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.invalidReq"))
	}
	if strings.TrimSpace(req.Phone) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.missingFields", "name", "phone"))
	}

	if req.Host == "" && req.RootURL != "" {
		req.Host = req.RootURL
	}
	if req.RootURL == "" && req.Host != "" {
		req.RootURL = req.Host
	}

	w, err := waha.New(req.Options)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.Ts("globals.messages.errorCreating", "name", "WAHA", "error", err.Error()))
	}

	msg := models.Message{
		ToPhone: strings.TrimSpace(req.Phone),
		Body:    []byte(a.i18n.T("settings.smtp.testConnection")),
	}
	if err := w.Push(msg); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, okResp{a.bufLog.Lines()})
}

func (a *App) GetAboutInfo(c echo.Context) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	out := a.about
	out.System.AllocMB = mem.Alloc / 1024 / 1024
	out.System.OSMB = mem.Sys / 1024 / 1024

	return c.JSON(http.StatusOK, out)
}
