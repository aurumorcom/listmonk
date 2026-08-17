//go:build unit || !integration

package auth

import (
	"encoding/base64"
	"io"
	"log"
	"testing"

	null "gopkg.in/volatiletech/null.v6"
)

func TestHashAPIToken(t *testing.T) {
	token := "my_secret_api_token"
	hash1 := HashAPIToken(token)
	hash2 := HashAPIToken(token)

	if hash1 == "" {
		t.Fatalf("expected non-empty hash string")
	}

	if hash1 != hash2 {
		t.Fatalf("expected HashAPIToken to be deterministic, got %s != %s", hash1, hash2)
	}

	if len(hash1) != 64 { // SHA-256 hex string length
		t.Fatalf("expected 64-char hex string, got length %d", len(hash1))
	}
}

func TestParseAuthHeader(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantUser  string
		wantToken string
		wantErr   bool
	}{
		{
			name:      "Token scheme",
			header:    "token admin:secret123",
			wantUser:  "admin",
			wantToken: "secret123",
			wantErr:   false,
		},
		{
			name:      "Basic Auth scheme",
			header:    "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret123")),
			wantUser:  "admin",
			wantToken: "secret123",
			wantErr:   false,
		},
		{
			name:    "Unknown Scheme",
			header:  "Bearer 12345",
			wantErr: true,
		},
		{
			name:    "Invalid Base64 in Basic Auth",
			header:  "Basic !!!!invalid_base64!!!",
			wantErr: true,
		},
		{
			name:    "Missing Delimiter",
			header:  "token admin_no_colon",
			wantErr: true,
		},
		{
			name:    "Empty Username or Token",
			header:  "token :secret123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, token, err := parseAuthHeader(tt.header)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseAuthHeader() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if user != tt.wantUser || token != tt.wantToken {
					t.Fatalf("expected (%s, %s), got (%s, %s)", tt.wantUser, tt.wantToken, user, token)
				}
			}
		})
	}
}

func TestCacheAPIUsersAndGetAPIToken(t *testing.T) {
	a := &Auth{
		apiUsers: make(map[string]User),
	}

	secretToken := "live_token_777"
	hashedToken := HashAPIToken(secretToken)

	u1 := User{
		Base:     Base{ID: 1},
		Username: "user1",
		Password: null.StringFrom(hashedToken),
	}
	u2 := User{
		Base:     Base{ID: 2},
		Username: "user2",
		Password: null.StringFrom(HashAPIToken("other_token")),
	}

	// Cache multiple users
	a.CacheAPIUsers([]User{u1, u2})

	// Retrieve with correct token
	retrieved, ok := a.GetAPIToken("user1", secretToken)
	if !ok {
		t.Fatalf("expected GetAPIToken to succeed for user1")
	}
	if retrieved.ID != 1 {
		t.Fatalf("expected user ID 1, got %d", retrieved.ID)
	}

	// Retrieve with invalid token
	_, ok = a.GetAPIToken("user1", "wrong_token")
	if ok {
		t.Fatalf("expected GetAPIToken to fail with incorrect token")
	}

	// Retrieve non-existent user
	_, ok = a.GetAPIToken("non_existent", secretToken)
	if ok {
		t.Fatalf("expected GetAPIToken to fail for non-existent user")
	}

	// Cache single user update
	u3 := User{
		Base:     Base{ID: 3},
		Username: "user3",
		Password: null.StringFrom(HashAPIToken("user3_token")),
	}
	a.CacheAPIUser(u3)

	retrieved3, ok := a.GetAPIToken("user3", "user3_token")
	if !ok || retrieved3.ID != 3 {
		t.Fatalf("expected user3 retrieval to succeed after CacheAPIUser")
	}
}

func TestUserPermissionsAndSuperAdmin(t *testing.T) {
	// Super Admin bypass
	superUser := User{
		UserRoleID: SuperAdminRoleID,
	}
	if !superUser.HasPerm(PermListManageAll) {
		t.Fatalf("expected Super Admin to have PermListManageAll permission")
	}

	// Standard User with specific permissions map
	standardUser := User{
		UserRoleID: 2,
		PermissionsMap: map[string]struct{}{
			PermSubscribersGet: {},
		},
	}
	if !standardUser.HasPerm(PermSubscribersGet) {
		t.Fatalf("expected standard user to have PermSubscribersGet")
	}
	if standardUser.HasPerm(PermSubscribersManage) {
		t.Fatalf("expected standard user NOT to have PermSubscribersManage")
	}
}

func TestGetOIDCAuthURL_Unconfigured(t *testing.T) {
	a := &Auth{
		log: log.New(io.Discard, "", 0),
		cfg: Config{
			OIDC: OIDCConfig{Enabled: false},
		},
	}

	url := a.GetOIDCAuthURL("state123", "nonce456")
	if url != "" {
		t.Fatalf("expected empty OIDC Auth URL when OIDC is disabled, got %s", url)
	}
}
