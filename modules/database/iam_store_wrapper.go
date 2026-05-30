package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GoCodeAlone/go-db-credential-refresh/driver"
)

// IAMTokenTTL is the lifetime of an AWS RDS IAM authentication token
// AWS documentation states tokens are valid for 15 minutes
const IAMTokenTTL = 15 * time.Minute

// TokenRefreshBuffer is the buffer time before token expiration to refresh
// We refresh 1 minute before expiration to avoid edge cases
const TokenRefreshBuffer = 1 * time.Minute

// EffectiveTokenLifetime is the actual time we'll cache a token before refreshing
// This ensures we refresh before the token expires
const EffectiveTokenLifetime = IAMTokenTTL - TokenRefreshBuffer // 14 minutes

// TTLStore wraps a driver.Store and adds TTL-based caching to prevent
// using expired IAM authentication tokens.
//
// The Problem:
// The awsrds.Store implementation caches credentials indefinitely, which causes
// PAM authentication failures after 15 minutes when tokens expire.
//
// The Solution:
// This wrapper adds a TTL to the cached credentials. When Get() is called:
//  1. If no credentials are cached, call Refresh() to get new ones
//  2. If credentials exist but are expired (>14 min old), call Refresh()
//  3. If credentials exist and are fresh (<14 min old), return cached ones
//
// This ensures tokens are always refreshed before they expire, preventing
// authentication failures while still benefiting from caching.
type TTLStore struct {
	wrapped       driver.Store
	mu            sync.RWMutex
	cachedCreds   driver.Credentials
	cachedAt      time.Time
	tokenLifetime time.Duration
}

// NewTTLStore creates a new TTL-aware store wrapper
func NewTTLStore(wrapped driver.Store) *TTLStore {
	return &TTLStore{
		wrapped:       wrapped,
		tokenLifetime: EffectiveTokenLifetime,
	}
}

// NewTTLStoreWithLifetime creates a TTL store with a custom token lifetime
// This is useful for testing with shorter lifetimes
func NewTTLStoreWithLifetime(wrapped driver.Store, lifetime time.Duration) *TTLStore {
	return &TTLStore{
		wrapped:       wrapped,
		tokenLifetime: lifetime,
	}
}

// Get implements driver.Store interface with TTL-based caching
func (s *TTLStore) Get(ctx context.Context) (driver.Credentials, error) {
	s.mu.RLock()
	creds := s.cachedCreds
	cachedAt := s.cachedAt
	s.mu.RUnlock()

	// If we have cached credentials and they're still fresh, return them
	if creds != nil && time.Since(cachedAt) < s.tokenLifetime {
		return creds, nil
	}

	// Credentials are missing or expired, refresh them
	return s.Refresh(ctx)
}

// Refresh implements driver.Store interface, always getting fresh credentials
func (s *TTLStore) Refresh(ctx context.Context) (driver.Credentials, error) {
	// Call the wrapped store's Refresh to get fresh credentials
	creds, err := s.wrapped.Refresh(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh credentials: %w", err)
	}

	// Cache the credentials with current timestamp
	s.mu.Lock()
	s.cachedCreds = creds
	s.cachedAt = time.Now()
	s.mu.Unlock()

	return creds, nil
}

// GetTokenAge returns how old the current cached token is
// Returns 0 if no token is cached
func (s *TTLStore) GetTokenAge() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.cachedCreds == nil {
		return 0
	}

	return time.Since(s.cachedAt)
}

// IsTokenFresh returns true if cached token is still within TTL
func (s *TTLStore) IsTokenFresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.cachedCreds == nil {
		return false
	}

	return time.Since(s.cachedAt) < s.tokenLifetime
}
