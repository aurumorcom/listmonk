package email

import (
	"testing"
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
