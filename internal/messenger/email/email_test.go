package email

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEmailer_GetPool_RFC5322_DisplayAddress(t *testing.T) {
	e := &Emailer{
		name:  "email",
		pools: make(map[string][]*Server),
	}

	serverAryan := &Server{Name: "smtp-aryan", FromAddresses: []string{"aryan.singh@capybaara.com"}}
	serverDefault := &Server{Name: "smtp-default"}

	e.pools[""] = []*Server{serverDefault}
	e.pools["aryan.singh@capybaara.com"] = []*Server{serverAryan}

	// Test 1: Full RFC 5322 display name
	matched := e.getPool("Aryan Singh <aryan.singh@capybaara.com>")
	if len(matched) == 0 || matched[0].Name != "smtp-aryan" {
		t.Fatalf("expected to resolve smtp-aryan pool, got %+v", matched)
	}

	// Test 2: Bare address
	matchedBare := e.getPool("aryan.singh@capybaara.com")
	if len(matchedBare) == 0 || matchedBare[0].Name != "smtp-aryan" {
		t.Fatalf("expected to resolve smtp-aryan pool for bare address, got %+v", matchedBare)
	}
}

func TestEmailer_GetPool_DomainFallback(t *testing.T) {
	e := &Emailer{
		name:  "email",
		pools: make(map[string][]*Server),
	}

	serverDomain := &Server{Name: "smtp-domain", FromAddresses: []string{"capybaara.com"}}
	serverDefault := &Server{Name: "smtp-default"}

	e.pools[""] = []*Server{serverDefault}
	e.pools["capybaara.com"] = []*Server{serverDomain}

	// Test: Different user on same domain
	matched := e.getPool("Support Team <support@capybaara.com>")
	if len(matched) == 0 || matched[0].Name != "smtp-domain" {
		t.Fatalf("expected to resolve domain pool smtp-domain, got %+v", matched)
	}
}

func TestEmailer_MalformedFromAddress(t *testing.T) {
	e := &Emailer{
		name:  "email",
		pools: make(map[string][]*Server),
	}
	serverDefault := &Server{Name: "smtp-default"}
	e.pools[""] = []*Server{serverDefault}

	// Should not panic and should return nil (so Push falls back to default pool e.pools[""])
	matched := e.getPool("@@invalid-address@@")
	if matched != nil {
		t.Fatalf("expected nil pool for malformed address, got %+v", matched)
	}
}

func TestServer_UnmarshalJSON_NestedOpt(t *testing.T) {
	raw := []byte(`{
		"name": "email-aquiveal",
		"auth_protocol": "login",
		"tls_type": "TLS",
		"opt": {
			"host": "smtp.gmail.com",
			"port": 465,
			"username": "aquiveal@gmail.com",
			"password": "secretpassword",
			"max_conns": 10,
			"max_msg_retries": 2,
			"idle_timeout": "15s"
		}
	}`)

	var s Server
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if s.Name != "email-aquiveal" {
		t.Errorf("expected Name 'email-aquiveal', got '%s'", s.Name)
	}
	if s.Host != "smtp.gmail.com" {
		t.Errorf("expected Host 'smtp.gmail.com', got '%s'", s.Host)
	}
	if s.Port != 465 {
		t.Errorf("expected Port 465, got %d", s.Port)
	}
	if s.Username != "aquiveal@gmail.com" {
		t.Errorf("expected Username 'aquiveal@gmail.com', got '%s'", s.Username)
	}
	if s.Password != "secretpassword" {
		t.Errorf("expected Password 'secretpassword', got '%s'", s.Password)
	}
	if s.MaxConns != 10 {
		t.Errorf("expected MaxConns 10, got %d", s.MaxConns)
	}
	if s.IdleTimeout != 15*time.Second {
		t.Errorf("expected IdleTimeout 15s, got %v", s.IdleTimeout)
	}
}

func TestServer_UnmarshalJSON_MissingMaxConnsFallback(t *testing.T) {
	raw := []byte(`{
		"name": "email-no-conns",
		"opt": {
			"host": "smtp.gmail.com",
			"port": 465
		}
	}`)

	var s Server
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if s.MaxConns != 10 {
		t.Errorf("expected MaxConns to default to 10, got %d", s.MaxConns)
	}

	msgr, err := New(s.Name, s)
	if err != nil {
		t.Fatalf("expected New() to succeed with defaulted MaxConns, got error: %v", err)
	}
	if msgr == nil {
		t.Fatal("expected non-nil Emailer")
	}
}
