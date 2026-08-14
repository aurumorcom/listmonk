package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/gofrs/uuid/v5"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/utils"
	"github.com/knadh/listmonk/models"
	"github.com/knadh/stuffbin"
	"github.com/lib/pq"
	null "gopkg.in/volatiletech/null.v6"
)

// install runs the first time setup of setting up the database.
func install(lastVer string, db *sqlx.DB, fs stuffbin.FileSystem, prompt, idempotent bool) {
	qMap := readQueries(queryFilePath, fs)

	fmt.Println("")
	if !idempotent {
		fmt.Println("** first time installation **")
		fmt.Printf("** IMPORTANT: This will wipe existing listmonk tables and types in the DB '%s' **",
			ko.String("db.database"))
	} else {
		fmt.Println("** first time (idempotent) installation **")
	}
	fmt.Println("")

	if prompt {
		var ok string
		fmt.Print("continue (y/N)?  ")
		if _, err := fmt.Scanf("%s", &ok); err != nil {
			lo.Fatalf("error reading value from terminal: %v", err)
		}
		if strings.ToLower(ok) != "y" {
			fmt.Println("install cancelled.")
			return
		}
	}

	// If idempotence is on, check if the DB is already setup.
	if idempotent {
		if _, err := db.Exec("SELECT count(*) FROM settings"); err != nil {
			// If "settings" doesn't exist, assume it's a fresh install.
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code != "42P01" {
				lo.Fatalf("error checking existing DB schema: %v", err)
			}
		} else {
			lo.Println("skipping install as database appears to be already setup")
			os.Exit(0)
		}
	}

	// Migrate the tables.
	if err := installSchema(lastVer, db, fs); err != nil {
		lo.Fatalf("error migrating DB schema: %v", err)
	}

	// Load the queries.
	q := prepareQueries(qMap, db, ko)

	// Sample list.
	defList, optinList := installLists(q)

	// Sample subscribers.
	installSubs(defList, optinList, q)

	// Templates.
	campTplID, archiveTplID := installTemplates(q)

	// Sample campaign.
	installCampaign(defList, campTplID, archiveTplID, q)

	// Sample sequence.
	installSequence(campTplID, archiveTplID, q)

	// Setup admin user optionally.
	var (
		user     = os.Getenv("LISTMONK_ADMIN_USER")
		password = os.Getenv("LISTMONK_ADMIN_PASSWORD")
		apiUser  = os.Getenv("LISTMONK_ADMIN_API_USER")

		hasUser = false
	)

	// Admin user.
	if user != "" && password != "" {
		if len(user) < 3 || len(password) < 8 {
			lo.Fatal("LISTMONK_ADMIN_USER should be min 3 chars and LISTMONK_ADMIN_PASSWORD should be min 8 chars")
		}

		lo.Printf("creating superadmin user '%s'", user)
		hasUser = true
	} else {
		lo.Printf("no superadmin user created. Visit webpage to create user.")
	}

	// API User.
	if apiUser != "" {
		if !hasUser {
			lo.Fatal("LISTMONK_ADMIN_API_USER requires LISTMONK_ADMIN_USER and LISTMONK_ADMIN_PASSWORD to be set")
		}

		if len(apiUser) < 3 {
			lo.Fatal("LISTMONK_ADMIN_API_USER should be min 3 chars")
		}

		lo.Printf("creating superadmin API user '%s'", apiUser)
	}

	if hasUser {
		installUser(user, password, apiUser, q)
	}

	lo.Printf("setup complete")
	lo.Printf(`run the program and access the dashboard at %s`, ko.MustString("app.address"))
}

// installSchema executes the SQL schema and creates the necessary tables and types.
func installSchema(curVer string, db *sqlx.DB, fs stuffbin.FileSystem) error {
	q, err := fs.Read("/schema.sql")
	if err != nil {
		return err
	}

	if _, err := db.Exec(string(q)); err != nil {
		return err
	}

	// Insert the current migration version.
	return recordMigrationVersion(curVer, db)
}

func installLists(q *models.Queries) (int, int) {
	var (
		defList   int
		optinList int
	)
	if err := q.CreateList.Get(&defList,
		uuid.Must(uuid.NewV4()),
		"Default list",
		models.ListTypePrivate,
		models.ListOptinSingle,
		models.ListStatusActive,
		pq.StringArray{"test"},
		"",
	); err != nil {
		lo.Fatalf("error creating list: %v", err)
	}

	if err := q.CreateList.Get(&optinList, uuid.Must(uuid.NewV4()),
		"Opt-in list",
		models.ListTypePublic,
		models.ListOptinDouble,
		models.ListStatusActive,
		pq.StringArray{"test"},
		"",
	); err != nil {
		lo.Fatalf("error creating list: %v", err)
	}

	return defList, optinList
}

func installSubs(defListID, optinListID int, q *models.Queries) {
	// Sample subscriber.
	if _, err := q.UpsertSubscriber.Exec(
		uuid.Must(uuid.NewV4()),
		"john@example.com",
		"John Doe",
		`{"type": "known", "good": true, "city": "Bengaluru"}`,
		pq.Int64Array{int64(defListID)},
		models.SubscriptionStatusUnconfirmed,
		true, true); err != nil {
		lo.Fatalf("Error creating subscriber: %v", err)
	}
	if _, err := q.UpsertSubscriber.Exec(
		uuid.Must(uuid.NewV4()),
		"anon@example.com",
		"Anon Doe",
		`{"type": "unknown", "good": true, "city": "Bengaluru"}`,
		pq.Int64Array{int64(optinListID)},
		models.SubscriptionStatusUnconfirmed,
		true, true); err != nil {
		lo.Fatalf("error creating subscriber: %v", err)
	}
}

func installTemplates(q *models.Queries) (int, int) {
	// Default campaign template.
	campTpl, err := fs.Get("/static/email-templates/default.tpl")
	if err != nil {
		lo.Fatalf("error reading default e-mail template: %v", err)
	}

	var campTplID int
	if err := q.CreateTemplate.Get(&campTplID, "Default campaign template", models.TemplateTypeCampaign, "", campTpl.ReadBytes(), nil, nil); err != nil {
		lo.Fatalf("error creating default campaign template: %v", err)
	}
	if _, err := q.SetDefaultTemplate.Exec(campTplID); err != nil {
		lo.Fatalf("error setting default template: %v", err)
	}

	// Default campaign archive template.
	archiveTpl, err := fs.Get("/static/email-templates/default-archive.tpl")
	if err != nil {
		lo.Fatalf("error reading default archive template: %v", err)
	}

	var archiveTplID int
	if err := q.CreateTemplate.Get(&archiveTplID, "Default archive template", models.TemplateTypeCampaign, "", archiveTpl.ReadBytes(), nil, nil); err != nil {
		lo.Fatalf("error creating default campaign template: %v", err)
	}

	// Sample tx template.
	txTpl, err := fs.Get("/static/email-templates/sample-tx.tpl")
	if err != nil {
		lo.Fatalf("error reading default e-mail template: %v", err)
	}

	if _, err := q.CreateTemplate.Exec("Sample transactional template", models.TemplateTypeTx, "Welcome {{ .Subscriber.Name }}", txTpl.ReadBytes(), nil, nil); err != nil {
		lo.Fatalf("error creating sample transactional template: %v", err)
	}

	// Sample visual campaign template.
	visualTpl, err := fs.Get("/static/email-templates/default-visual.tpl")
	if err != nil {
		lo.Fatalf("error reading default visual template: %v", err)
	}
	visualSrc, err := fs.Get("/static/email-templates/default-visual.json")
	if err != nil {
		lo.Fatalf("error reading default visual template json: %v", err)
	}

	var visualTplID int
	if err := q.CreateTemplate.Get(&visualTplID, "Sample visual template", models.TemplateTypeCampaignVisual, "", visualTpl.ReadBytes(), visualSrc.ReadBytes(), nil); err != nil {
		lo.Fatalf("error creating default visual campaign template: %v", err)
	}

	return campTplID, archiveTplID
}

func installCampaign(defListID, campTplID, archiveTplID int, q *models.Queries) {
	// Sample campaign.
	var campID int
	if err := q.CreateCampaign.Get(&campID, uuid.Must(uuid.NewV4()),
		models.CampaignTypeRegular,
		"Test campaign",
		"Welcome to listmonk",
		"No Reply <noreply@yoursite.com>",
		`<h3>Hi {{ .Subscriber.FirstName }}!</h3>
		<p>This is a test e-mail campaign. Your second name is {{ .Subscriber.LastName }} and you are from {{ .Subscriber.Attribs.city }}.</p>
		<p>Here is a <a href="https://listmonk.app@TrackLink">tracked link</a>.</p>
		<p>Use the link icon in the editor toolbar or when writing raw HTML or Markdown,
			simply suffix @TrackLink to the end of a URL to turn it into a tracking link. Example:</p>
		<pre>&lt;a href=&quot;https:/&zwnj;/listmonk.app&#064;TrackLink&quot;&gt;&lt;/a&gt;</pre>
		<p>For help, refer to the <a href="https://listmonk.app/docs">documentation</a>.</p>
		`,
		"",
		"richtext",
		nil,
		json.RawMessage("[]"),
		json.RawMessage("{}"),
		pq.StringArray{"test-campaign"},
		emailMsgr,
		campTplID,
		pq.Int64Array{int64(defListID)},
		false,
		"welcome-to-listmonk",
		archiveTplID,
		json.RawMessage(`{"name": "Subscriber"}`),
		pq.Int64Array{},
		nil,
	); err != nil {
		lo.Fatalf("error creating sample campaign: %v", err)
	}
}

func installSequence(campTplID, archiveTplID int, q *models.Queries) {
	var seq models.Sequence
	if err := q.CreateSequence.Get(&seq,
		uuid.Must(uuid.NewV4()).String(),
		"Internal Team Demo (WhatsApp + Email in 2 Mins)",
		"Rapid 6-step interactive sequence demonstrating instant WAHA read receipts, email handoffs, and link clicks",
		models.SequenceStatusActive,
		nil,
		json.RawMessage(`{"days": ["mon","tue","wed","thu","fri","sat","sun"], "start_time": "00:00", "end_time": "23:59"}`),
		pq.Int64Array{},
		pq.StringArray{"default"},
		false,
		archiveTplID,
		nil,
		json.RawMessage("{}"),
	); err != nil {
		lo.Fatalf("error creating sample sequence: %v", err)
	}

	steps := []models.SequenceStep{
		{
			StepNumber:   1,
			DelaySeconds: 0,
			Messenger:    "waha",
			Condition:    models.SequenceConditionAlways,
			Subject:      "Step 1: Incoming Transmission",
			Body:         "🛸 *Incoming Transmission from HQ...*\n\nHey {{ .Subscriber.FirstName }}! We have a top-secret mission prepared for {{ .Subscriber.Email }}.\n\n👁️ Leave this chat unread and nothing happens... Open it to give us the Blue Ticks, and we’ll immediately beam the payload to your inbox!",
		},
		{
			StepNumber:   2,
			DelaySeconds: 0,
			Messenger:    "waha",
			Condition:    models.SequenceConditionIfRead,
			Subject:      "Step 2: Read Caught",
			Body:         "We just beamed an urgent mission email to {{ .Subscriber.Email }}! 🛸\n\n🏃‍♂️ Sprint over to your inbox and click the button before carrier pigeons eat the bandwidth!",
		},
		{
			StepNumber:   3,
			DelaySeconds: 10,
			Messenger:    "email",
			Condition:    models.SequenceConditionAlways,
			Subject:      "🧪 [Team Demo] Click this link to trigger Step 4 on WhatsApp!",
			Body:         "<p>Hi {{ .Subscriber.FirstName }}!</p><p>You triggered the <code>if_read</code> Blue Tick response!</p><p><a href=\"https://example.com/demo@TrackLink\">👉 CLICK ME TO TRIGGER WHATSAPP STEP 4 👈</a></p>",
		},
		{
			StepNumber:   4,
			DelaySeconds: 0,
			Messenger:    "waha",
			Condition:    models.SequenceConditionIfClicked,
			Subject:      "Step 4: Click Registered",
			Body:         "🎯 *CLICK EVENT REGISTERED IN REAL-TIME!*\n\n{{ .Subscriber.FirstName }}, you clicked the button like a 10x engineer! 🍪 Listmonk saw your click immediately.",
		},
		{
			StepNumber:   5,
			DelaySeconds: 45,
			Messenger:    "waha",
			Condition:    models.SequenceConditionIfNotRead,
			Subject:      "Step 5: AFK Check",
			Body:         "☕ *AFK Alert!*\n\nStill waiting on that email click, {{ .Subscriber.FirstName }}! Don't leave the demo hanging!",
		},
		{
			StepNumber:   6,
			DelaySeconds: 30,
			Messenger:    "email",
			Condition:    models.SequenceConditionAlways,
			Subject:      "🏆 [Demo Complete] You conquered the 2-minute sequence!",
			Body:         "<h2>🎉 Demo Complete!</h2><p>You have tested WAHA Blue Tick reads, email handoffs, and link clicks in under 2 minutes.</p>",
		},
	}

	for _, s := range steps {
		if _, err := q.CreateSequenceStep.Exec(seq.ID, s.StepNumber, s.DelaySeconds, s.Messenger, s.Condition, s.Subject, s.Body, models.EmailTypeNewThread, campTplID); err != nil {
			lo.Fatalf("error creating sample sequence step %d: %v", s.StepNumber, err)
		}
	}
}

// recordMigrationVersion inserts the given version (of DB migration) into the
// `migrations` array in the settings table.
func recordMigrationVersion(ver string, db *sqlx.DB) error {
	_, err := db.Exec(fmt.Sprintf(`INSERT INTO settings (key, value)
	VALUES('migrations', '["%s"]'::JSONB)
	ON CONFLICT (key) DO UPDATE SET value = settings.value || EXCLUDED.value`, ver))
	return err
}

func newConfigFile(path string) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("error creating %s: %v", path, err)
	}

	// Initialize the static file system into which all
	// required static assets (.sql, .js files etc.) are loaded.
	fs := initFS(appDir, "", "", "")
	b, err := fs.Read("config.toml.sample")
	if err != nil {
		return fmt.Errorf("error reading sample config (is binary stuffed?): %v", err)
	}

	return os.WriteFile(path, b, 0644)
}

// checkSchema checks if the DB schema is installed.
func checkSchema(db *sqlx.DB) (bool, error) {
	if _, err := db.Exec(`SELECT id FROM templates LIMIT 1`); err != nil {
		if isTableNotExistErr(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func installUser(username, password, apiUsername string, q *models.Queries) {
	consts := initConstConfig(ko)

	// Super Admin role gets all permissions.
	perms := []string{}
	for p := range consts.Permissions {
		perms = append(perms, p)
	}

	// Create the Super Admin role in the DB.
	var role auth.Role
	if err := q.CreateRole.Get(&role, "Super Admin", auth.RoleTypeUser, pq.Array(perms)); err != nil {
		lo.Fatalf("error creating super admin role: %v", err)
	}

	// Create the admin user.
	if _, err := q.CreateUser.Exec(username, true, password, username+"@listmonk", username, auth.RoleTypeUser, role.ID, nil, auth.UserStatusEnabled); err != nil {
		lo.Fatalf("error creating superadmin user: %v", err)
	}

	// Create the admin API user.
	if apiUsername != "" {
		// Generate a random API token.
		tk, err := utils.GenerateRandomString(32)
		if err != nil {
			lo.Fatalf("error generating API token: %v", err)
		}

		var (
			email    = null.String{String: apiUsername + "@api", Valid: true}
			password = null.String{String: auth.HashAPIToken(tk), Valid: true}
		)

		if _, err := q.CreateUser.Exec(apiUsername, false, password, email, apiUsername, auth.UserTypeAPI, role.ID, nil, auth.UserStatusEnabled); err != nil {
			lo.Fatalf("error creating superadmin API user: %v", err)
		}

		// Print the token to stdout so that it can be grepped out.
		lo.Println("writing API token LISTMONK_ADMIN_API_TOKEN to stderr")
		fmt.Fprintf(os.Stderr, "export LISTMONK_ADMIN_API_TOKEN=\"%s\"\n", tk)
	}
}
