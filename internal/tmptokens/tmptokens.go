// Package tmptokens provides a simple in memory store for one time temp tokens with TTL.
// This can be used for creating throwaway tokens for flows like password reset, 2FA verification, etc.
// Tokens are automatically deleted when retrieved or when they expire.
package tmptokens

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

const (
	// maxTries is the maximum number of verification attempts allowed for a token.
	// After this many failed checks, the token is automatically deleted.
	maxTries = 15
)

// Token represents a temporary token with TTL and arbitrary data.
type Token struct {
	TTL       time.Duration
	CreatedAt time.Time
	Count     int
	Data      any
}

var (
	Err = errors.New("token was not found or has expired")

	_tokens = make(map[string]Token)
	_mu     sync.RWMutex
)

// StartCleanup starts a lifecycle-managed background ticker that periodically purges expired tokens.
func StartCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("stopping temporary tokens cleanup worker")
				return
			case <-ticker.C:
				purged := Clean()
				if purged > 0 {
					slog.Debug("purged expired temporary tokens", slog.Int("purged_count", purged))
				}
			}
		}
	}()
}

// Set stores a token with the given ID, TTL, and data.
// If a token with the same ID already exists, it will be overwritten silently.
func Set(id string, ttl time.Duration, data any) {
	_mu.Lock()
	defer _mu.Unlock()

	_tokens[id] = Token{
		TTL:       ttl,
		Data:      data,
		CreatedAt: time.Now(),
	}
}

// Check retrieves a token by ID without deleting it.
// An error is returned if the token doesn't exist or has expired.
// Unlike Get(), this method does not consume/delete the token.
// It also increments the check counter and deletes the token if maxTries is exceeded,
// acting as a rate limiter.
func Check(id string) (any, error) {
	_mu.Lock()
	defer _mu.Unlock()

	token, exists := _tokens[id]
	if !exists {
		return nil, Err
	}

	// Check if token has expired.
	if time.Since(token.CreatedAt) > token.TTL {
		delete(_tokens, id)
		return nil, Err
	}

	// Increment the rate limit counter.
	token.Count++

	// Check if max attempts exceeded.
	if token.Count > maxTries {
		delete(_tokens, id)
		return nil, Err
	}

	// Update the token with the new count.
	_tokens[id] = token

	return token.Data, nil
}

// Get retrieves a token by ID and automatically deletes it (after one time use).
// An error is returned if the token doesn't exist or has expired.
func Get(id string) (any, error) {
	_mu.Lock()
	defer _mu.Unlock()

	token, exists := _tokens[id]
	if !exists {
		return nil, Err
	}

	// Check if token has expired.
	if time.Since(token.CreatedAt) > token.TTL {
		delete(_tokens, id)
		return nil, Err
	}

	// Delete the token.
	delete(_tokens, id)

	return token.Data, nil
}

// Delete deletes a token by ID.
func Delete(id string) {
	_mu.Lock()
	defer _mu.Unlock()

	delete(_tokens, id)
}

// Clean deletes all expired tokens. This can be called periodically
// to purge unused and expired tokens.
func Clean() int {
	_mu.Lock()
	defer _mu.Unlock()

	var purged int
	now := time.Now()
	for id, token := range _tokens {
		if now.Sub(token.CreatedAt) > token.TTL {
			delete(_tokens, id)
			purged++
		}
	}
	return purged
}
