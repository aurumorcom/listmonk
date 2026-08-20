//go:build unit || !integration

package tmptokens

import (
	"context"
	"testing"
	"time"
)

func TestSetAndGet_SingleUse(t *testing.T) {
	id := "test_token_123"
	data := map[string]string{"user_id": "42"}

	Set(id, 5*time.Minute, data)

	// First Get should succeed and retrieve data
	res, err := Get(id)
	if err != nil {
		t.Fatalf("expected successful Get for valid token, got: %v", err)
	}

	resMap, ok := res.(map[string]string)
	if !ok || resMap["user_id"] != "42" {
		t.Fatalf("retrieved data mismatch, got %+v", res)
	}

	// Second Get should fail (token was consumed)
	_, err = Get(id)
	if err == nil {
		t.Fatalf("expected error on second Get for consumed token, got nil")
	}
}

func TestCheck_MaxTriesRateLimiting(t *testing.T) {
	id := "test_rate_limited_token"
	Set(id, 5*time.Minute, "secret_data")

	// Call Check up to maxTries (15)
	for i := 1; i <= maxTries; i++ {
		data, err := Check(id)
		if err != nil {
			t.Fatalf("expected Check #%d to succeed, got error: %v", i, err)
		}
		if data != "secret_data" {
			t.Fatalf("expected 'secret_data', got %v", data)
		}
	}

	// 16th Check should exceed maxTries and delete token
	_, err := Check(id)
	if err == nil {
		t.Fatalf("expected error on Check exceeding maxTries, got nil")
	}

	// Verify token is deleted
	_, err = Get(id)
	if err == nil {
		t.Fatalf("expected token to be deleted after exceeding maxTries")
	}
}

func TestTTL_Expiration(t *testing.T) {
	id := "test_expired_token"
	// Set TTL to -1 second so it is instantly expired
	Set(id, -1*time.Second, "expired_data")

	_, err := Check(id)
	if err == nil {
		t.Fatalf("expected Check to fail for expired token")
	}

	Set(id, -1*time.Second, "expired_data")
	_, err = Get(id)
	if err == nil {
		t.Fatalf("expected Get to fail for expired token")
	}
}

func TestDeleteAndClean(t *testing.T) {
	id1 := "token_keep"
	id2 := "token_delete"
	id3 := "token_expired"

	Set(id1, 10*time.Minute, "valid")
	Set(id2, 10*time.Minute, "to_delete")
	Set(id3, -10*time.Second, "expired")

	Delete(id2)

	_, err := Get(id2)
	if err == nil {
		t.Fatalf("expected Get to fail for deleted token")
	}

	purged := Clean()
	if purged != 1 {
		t.Fatalf("expected 1 token to be purged, got %d", purged)
	}

	// Expired token should be purged
	_, err = Get(id3)
	if err == nil {
		t.Fatalf("expected Get to fail for cleaned expired token")
	}

	// Valid token should still exist
	val, err := Get(id1)
	if err != nil || val != "valid" {
		t.Fatalf("expected valid token to persist after Clean(), got %v, err: %v", val, err)
	}
}

func TestStartCleanup_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	StartCleanup(ctx, 10*time.Millisecond)

	Set("cancel_test_token", -1*time.Second, "expired")

	// Allow one ticker run
	time.Sleep(25 * time.Millisecond)

	_, err := Get("cancel_test_token")
	if err == nil {
		t.Fatalf("expected token to be cleaned up by background worker")
	}

	// Cancel context to stop cleanup
	cancel()
	time.Sleep(10 * time.Millisecond)
}
