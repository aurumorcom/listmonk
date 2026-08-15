package models

import "gopkg.in/volatiletech/null.v6"

// Settings represents the app settings stored in the DB.
type Settings struct {
	AppSiteName                   string   `json:"app.site_name"`
	AppRootURL                    string   `json:"app.root_url"`
	AppLogoURL                    string   `json:"app.logo_url"`
	AppFaviconURL                 string   `json:"app.favicon_url"`
	AppFromEmail                  string   `json:"app.from_email"`
	AppGlobalSignature            string   `json:"app.global_signature"`
	AppNotifyEmails               []string `json:"app.notify_emails"`
	EnablePublicSubPage           bool     `json:"app.enable_public_subscription_page"`
	EnablePublicArchive           bool     `json:"app.enable_public_archive"`
	EnablePublicArchiveRSSContent bool     `json:"app.enable_public_archive_rss_content"`
	ShowOptinPage                 bool     `json:"app.show_optin_page"`
	SendOptinConfirmation         bool     `json:"app.send_optin_confirmation"`
	CheckUpdates                  bool     `json:"app.check_updates"`
	AppLang                       string   `json:"app.lang"`

	AppBatchSize             int    `json:"app.batch_size"`
	AppConcurrency           int    `json:"app.concurrency"`
	AppMaxSendErrors         int    `json:"app.max_send_errors"`
	AppMessageRate           int    `json:"app.message_rate"`
	CacheSlowQueries         bool   `json:"app.cache_slow_queries"`
	CacheSlowQueriesInterval string `json:"app.cache_slow_queries_interval"`

	AppMessageSlidingWindow         bool   `json:"app.message_sliding_window"`
	AppMessageSlidingWindowDuration string `json:"app.message_sliding_window_duration"`
	AppMessageSlidingWindowRate     int    `json:"app.message_sliding_window_rate"`

	PrivacyIndividualTracking bool     `json:"privacy.individual_tracking"`
	PrivacyDisableTracking    bool     `json:"privacy.disable_tracking"`
	PrivacyUnsubHeader        bool     `json:"privacy.unsubscribe_header"`
	PrivacyAllowBlocklist     bool     `json:"privacy.allow_blocklist"`
	PrivacyAllowPreferences   bool     `json:"privacy.allow_preferences"`
	PrivacyAllowExport        bool     `json:"privacy.allow_export"`
	PrivacyAllowWipe          bool     `json:"privacy.allow_wipe"`
	PrivacyExportable         []string `json:"privacy.exportable"`
	PrivacyRecordOptinIP      bool     `json:"privacy.record_optin_ip"`
	DomainBlocklist           []string `json:"privacy.domain_blocklist"`
	DomainAllowlist           []string `json:"privacy.domain_allowlist"`

	SecurityCaptcha struct {
		Altcha struct {
			Enabled    bool `json:"enabled"`
			Complexity int  `json:"complexity"`
		} `json:"altcha"`
		HCaptcha struct {
			Enabled bool   `json:"enabled"`
			Key     string `json:"key"`
			Secret  string `json:"secret"`
		} `json:"hcaptcha"`
	} `json:"security.captcha"`

	OIDC struct {
		Enabled           bool     `json:"enabled"`
		ProviderURL       string   `json:"provider_url"`
		ProviderName      string   `json:"provider_name"`
		ClientID          string   `json:"client_id"`
		ClientSecret      string   `json:"client_secret"`
		AutoCreateUsers   bool     `json:"auto_create_users"`
		DefaultUserRoleID null.Int `json:"default_user_role_id"`
		DefaultListRoleID null.Int `json:"default_list_role_id"`
	} `json:"security.oidc"`

	SecurityTrustedURLs []string `json:"security.trusted_urls"`

	UploadProvider             string   `json:"upload.provider"`
	UploadExtensions           []string `json:"upload.extensions"`
	UploadFilesystemUploadPath string   `json:"upload.filesystem.upload_path"`
	UploadFilesystemUploadURI  string   `json:"upload.filesystem.upload_uri"`
	UploadS3URL                string   `json:"upload.s3.url"`
	UploadS3PublicURL          string   `json:"upload.s3.public_url"`
	UploadS3AwsAccessKeyID     string   `json:"upload.s3.aws_access_key_id"`
	UploadS3AwsDefaultRegion   string   `json:"upload.s3.aws_default_region"`
	UploadS3AwsSecretAccessKey string   `json:"upload.s3.aws_secret_access_key,omitempty"`
	UploadS3Bucket             string   `json:"upload.s3.bucket"`
	UploadS3BucketDomain       string   `json:"upload.s3.bucket_domain"`
	UploadS3BucketPath         string   `json:"upload.s3.bucket_path"`
	UploadS3BucketType         string   `json:"upload.s3.bucket_type"`
	UploadS3Expiry             string   `json:"upload.s3.expiry"`

	SMTP []struct {
		Name          string              `json:"name"`
		User          string              `json:"user"`
		UserID        null.Int            `json:"user_id"`
		Signature     string              `json:"signature"`
		UUID          string              `json:"uuid"`
		Enabled       bool                `json:"enabled"`
		Host          string              `json:"host"`
		HelloHostname string              `json:"hello_hostname"`
		Port          int                 `json:"port"`
		AuthProtocol  string              `json:"auth_protocol"`
		Username      string              `json:"username"`
		Password      string              `json:"password,omitempty"`
		EmailHeaders  []map[string]string `json:"email_headers"`
		MaxConns      int                 `json:"max_conns"`
		MaxMsgRetries int                 `json:"max_msg_retries"`
		MsgRetryDelay string              `json:"msg_retry_delay"`
		IdleTimeout   string              `json:"idle_timeout"`
		WaitTimeout   string              `json:"wait_timeout"`
		TLSType       string              `json:"tls_type"`
		TLSSkipVerify bool                `json:"tls_skip_verify"`
		FromAddresses []string            `json:"from_addresses"`
		EmailsPerDay  int                 `json:"emails_per_day"`
		EmailsPerHour int                 `json:"emails_per_hour"`

		// IMAP settings
		IMAPEnabled       bool   `json:"imap_enabled"`
		IMAPHost          string `json:"imap_host"`
		IMAPPort          int    `json:"imap_port"`
		IMAPAuthProtocol  string `json:"imap_auth_protocol"`
		IMAPUsername      string `json:"imap_username"`
		IMAPPassword      string `json:"imap_password,omitempty"`
		IMAPTLSType       string `json:"imap_tls_type"`
		IMAPTLSSkipVerify bool   `json:"imap_tls_skip_verify"`
		IMAPFolder        string `json:"imap_folder"`
		IMAPInterval      string `json:"imap_interval"`
		IMAPMaxConns      int    `json:"imap_max_conns"`
		IMAPIdleTimeout   string `json:"imap_idle_timeout"`
		IMAPWaitTimeout   string `json:"imap_wait_timeout"`
		IMAPMaxRetries    int    `json:"imap_max_retries"`
		IMAPRetryDelay    string `json:"imap_retry_delay"`
	} `json:"smtp"`

	Messengers []struct {
		UUID          string `json:"uuid"`
		Enabled       bool   `json:"enabled"`
		Name          string `json:"name"`
		RootURL       string `json:"root_url"`
		Username      string `json:"username"`
		Password      string `json:"password,omitempty"`
		MaxConns      int    `json:"max_conns"`
		Timeout       string `json:"timeout"`
		MaxMsgRetries int    `json:"max_msg_retries"`
	} `json:"messengers"`

	WAHAMessengers []WAHAMessenger `json:"waha_messengers"`

	BounceEnabled        bool `json:"bounce.enabled"`
	BounceEnableWebhooks bool `json:"bounce.webhooks_enabled"`
	BounceActions        map[string]struct {
		Count  int    `json:"count"`
		Action string `json:"action"`
	} `json:"bounce.actions"`
	SESEnabled      bool   `json:"bounce.ses_enabled"`
	SendgridEnabled bool   `json:"bounce.sendgrid_enabled"`
	SendgridKey     string `json:"bounce.sendgrid_key"`
	BounceAzure     struct {
		Enabled            bool   `json:"enabled"`
		SharedSecret       string `json:"shared_secret"`
		SharedSecretHeader string `json:"shared_secret_header"`
	} `json:"bounce.azure"`
	BouncePostmark struct {
		Enabled  bool   `json:"enabled"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"bounce.postmark"`
	BounceForwardEmail struct {
		Enabled bool   `json:"enabled"`
		Key     string `json:"key"`
	} `json:"bounce.forwardemail"`
	BounceLettermint struct {
		Enabled bool   `json:"enabled"`
		Key     string `json:"key"`
	} `json:"bounce.lettermint"`
	BounceBoxes []struct {
		UUID          string `json:"uuid"`
		Enabled       bool   `json:"enabled"`
		Type          string `json:"type"`
		Host          string `json:"host"`
		Port          int    `json:"port"`
		AuthProtocol  string `json:"auth_protocol"`
		ReturnPath    string `json:"return_path"`
		Username      string `json:"username"`
		Password      string `json:"password,omitempty"`
		TLSEnabled    bool   `json:"tls_enabled"`
		TLSSkipVerify bool   `json:"tls_skip_verify"`
		ScanInterval  string `json:"scan_interval"`
	} `json:"bounce.mailboxes"`

	MaintenanceDB struct {
		Vacuum         bool   `json:"vacuum"`
		VacuumInterval string `json:"vacuum_cron_interval"`
	} `json:"maintenance.db"`

	AdminCustomCSS  string `json:"appearance.admin.custom_css"`
	AdminCustomJS   string `json:"appearance.admin.custom_js"`
	PublicCustomCSS string `json:"appearance.public.custom_css"`
	PublicCustomJS  string `json:"appearance.public.custom_js"`
}

// WAHAMessenger represents individual WAHA messenger configuration settings.
type WAHAMessenger struct {
	UUID              string   `json:"uuid"`
	Enabled           bool     `json:"enabled"`
	Name              string   `json:"name"`
	RootURL           string   `json:"root_url"`
	APIKey            string   `json:"api_key,omitempty"`
	Session           string   `json:"session"`
	PhoneAttribute    string   `json:"phone_attribute"`
	TypingDelayMs     int      `json:"typing_delay_ms"`
	TargetWPM         int      `json:"target_wpm"`
	WPMStd            float64  `json:"wpm_std"`
	KeyboardLayout    string   `json:"keyboard_layout"`
	TypingMode        string   `json:"typing_mode"`
	MaxTypingDelaySec int      `json:"max_typing_delay_sec"`
	MaxConns          int      `json:"max_conns"`
	Timeout           string   `json:"timeout"`
	MaxMsgRetries     int      `json:"max_msg_retries"`
	MessagesPerDay    int      `json:"messages_per_day"`
	MessagesPerHour   int      `json:"messages_per_hour"`
	UserID            null.Int `json:"user_id"`
	User              string   `json:"user"`
	Signature         string   `json:"signature"`
}
