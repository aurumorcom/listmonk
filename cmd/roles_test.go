//go:build integration || e2e || resilience || !unit

package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/labstack/echo/v4"
)

func TestRolePayloadBinding(t *testing.T) {
	e := echo.New()
	reqBody := []byte(`{
		"name": "Campaign Manager",
		"type": "user",
		"permissions": ["campaigns:get", "campaigns:manage", "subscribers:get"]
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/roles", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var role auth.Role
	if err := c.Bind(&role); err != nil {
		t.Fatalf("unexpected error binding role payload: %v", err)
	}

	if !role.Name.Valid || role.Name.String != "Campaign Manager" {
		t.Fatalf("role name mismatch: %+v", role.Name)
	}

	if len(role.Permissions) != 3 || role.Permissions[0] != "campaigns:get" {
		t.Fatalf("expected permissions ['campaigns:get', ...], got %v", role.Permissions)
	}
}
