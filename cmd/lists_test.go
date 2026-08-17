//go:build integration || e2e || resilience || !unit

package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
)

func TestListPayloadBinding(t *testing.T) {
	e := echo.New()
	reqBody := []byte(`{
		"name": "Weekly Tech Newsletter",
		"type": "public",
		"optin": "double",
		"tags": ["newsletter", "tech"]
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/lists", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var list models.List
	if err := c.Bind(&list); err != nil {
		t.Fatalf("unexpected error binding list payload: %v", err)
	}

	if list.Name != "Weekly Tech Newsletter" || list.Type != "public" || list.Optin != "double" {
		t.Fatalf("list field mismatch: %+v", list)
	}

	if len(list.Tags) != 2 || list.Tags[0] != "newsletter" {
		t.Fatalf("expected tags ['newsletter', 'tech'], got %v", list.Tags)
	}
}
