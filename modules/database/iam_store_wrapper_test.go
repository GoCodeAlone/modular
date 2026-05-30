package database

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/GoCodeAlone/go-db-credential-refresh/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCredentials implements driver.Credentials for testing
type MockCredentials struct {
	username string
	password string
}

func (m *MockCredentials) GetUsername() string {
	return m.username
}

func (m *MockCredentials) GetPassword() string {
	return m.password
}

// MockStore implements driver.Store for testing
type MockStore struct {
	mu           sync.Mutex
	getCalls     int
	refreshCalls int
	password     string
}

func (m *MockStore) Get(ctx context.Context) (driver.Credentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getCalls++
	return &MockCredentials{
		username: "testuser",
		password: m.password,
	}, nil
}

func (m *MockStore) Refresh(ctx context.Context) (driver.Credentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.refreshCalls++
	// Generate a new token to simulate AWS behavior
	m.password = "token-" + time.Now().Format("20060102-150405.000000000")
	return &MockCredentials{
		username: "testuser",
		password: m.password,
	}, nil
}

func TestTTLStore_InitialGet(t *testing.T) {
	ctx := context.Background()
	mockStore := &MockStore{}
	ttlStore := NewTTLStore(mockStore)

	// First Get should call Refresh since no cached credentials
	creds, err := ttlStore.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, creds)

	assert.Equal(t, "testuser", creds.GetUsername())
	assert.NotEmpty(t, creds.GetPassword())
	assert.Equal(t, 1, mockStore.refreshCalls, "First Get should trigger Refresh")
	assert.Equal(t, 0, mockStore.getCalls, "TTLStore.Get should not call wrapped.Get()")
}

func TestTTLStore_CachedGet(t *testing.T) {
	ctx := context.Background()
	mockStore := &MockStore{}
	ttlStore := NewTTLStore(mockStore)

	// First call to cache credentials
	creds1, err := ttlStore.Get(ctx)
	require.NoError(t, err)
	password1 := creds1.GetPassword()

	// Second call within TTL should return cached credentials
	creds2, err := ttlStore.Get(ctx)
	require.NoError(t, err)
	password2 := creds2.GetPassword()

	assert.Equal(t, password1, password2, "Should return same cached password")
	assert.Equal(t, 1, mockStore.refreshCalls, "Should only call Refresh once")
	assert.True(t, ttlStore.IsTokenFresh(), "Token should be fresh")
}

func TestTTLStore_ExpiredToken(t *testing.T) {
	ctx := context.Background()
	mockStore := &MockStore{}

	// Create TTL store with very short lifetime for testing
	ttlStore := NewTTLStoreWithLifetime(mockStore, 100*time.Millisecond)

	// First call to cache credentials
	creds1, err := ttlStore.Get(ctx)
	require.NoError(t, err)
	password1 := creds1.GetPassword()

	// Check token is initially fresh
	assert.True(t, ttlStore.IsTokenFresh(), "Token should be fresh initially")

	// Wait for token to expire
	time.Sleep(150 * time.Millisecond)

	// Token should now be stale
	assert.False(t, ttlStore.IsTokenFresh(), "Token should be stale after TTL")

	// Second call after expiration should refresh
	creds2, err := ttlStore.Get(ctx)
	require.NoError(t, err)
	password2 := creds2.GetPassword()

	assert.NotEqual(t, password1, password2, "Should get new password after expiration")
	assert.Equal(t, 2, mockStore.refreshCalls, "Should call Refresh twice (initial + after expiration)")
	assert.True(t, ttlStore.IsTokenFresh(), "New token should be fresh after refresh")
}

func TestTTLStore_ManualRefresh(t *testing.T) {
	ctx := context.Background()
	mockStore := &MockStore{}
	ttlStore := NewTTLStore(mockStore)

	// Get initial credentials
	creds1, err := ttlStore.Get(ctx)
	require.NoError(t, err)
	password1 := creds1.GetPassword()

	// Manually refresh
	creds2, err := ttlStore.Refresh(ctx)
	require.NoError(t, err)
	password2 := creds2.GetPassword()

	assert.NotEqual(t, password1, password2, "Manual refresh should get new password")
	assert.Equal(t, 2, mockStore.refreshCalls, "Should call Refresh twice")

	// Next Get should return the manually refreshed credentials
	creds3, err := ttlStore.Get(ctx)
	require.NoError(t, err)
	password3 := creds3.GetPassword()

	assert.Equal(t, password2, password3, "Get should return manually refreshed credentials")
}

func TestTTLStore_GetTokenAge(t *testing.T) {
	ctx := context.Background()
	mockStore := &MockStore{}
	ttlStore := NewTTLStore(mockStore)

	// Initially, no token cached
	assert.Equal(t, time.Duration(0), ttlStore.GetTokenAge(), "No token should have age 0")

	// Get credentials
	_, err := ttlStore.Get(ctx)
	require.NoError(t, err)

	// Token should have minimal age
	age := ttlStore.GetTokenAge()
	assert.Greater(t, age, time.Duration(0), "Token should have positive age")
	assert.Less(t, age, 100*time.Millisecond, "Token should be very fresh")

	// Wait and check age increases
	time.Sleep(50 * time.Millisecond)
	age2 := ttlStore.GetTokenAge()
	assert.Greater(t, age2, age, "Token age should increase over time")
}

func TestTTLStore_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	mockStore := &MockStore{}
	ttlStore := NewTTLStore(mockStore)

	// Launch multiple concurrent Gets
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, err := ttlStore.Get(ctx)
			assert.NoError(t, err)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// All goroutines should have succeeded
	// The number of Refresh calls may vary due to timing, but should be small
	assert.Greater(t, mockStore.refreshCalls, 0, "Should have called Refresh")
	assert.LessOrEqual(t, mockStore.refreshCalls, numGoroutines, "Should not refresh more than goroutines")
}

func TestTTLStore_RealWorldScenario(t *testing.T) {
	// This test simulates a real-world scenario with the default 14-minute TTL
	ctx := context.Background()
	mockStore := &MockStore{}

	// Use a shorter TTL for testing (2 seconds instead of 14 minutes)
	ttlStore := NewTTLStoreWithLifetime(mockStore, 2*time.Second)

	t.Log("Simulating connection pool behavior with token rotation")

	// Simulate queries happening over time
	queries := []struct {
		delay         time.Duration
		expectRefresh bool
		description   string
	}{
		{0, true, "Initial connection - should generate first token"},
		{500 * time.Millisecond, false, "Query within TTL - should use cached token"},
		{500 * time.Millisecond, false, "Another query within TTL - should use cached token (1s total)"},
		{1500 * time.Millisecond, true, "Query after TTL expires - should refresh token (2.5s total, > 2s TTL)"},
		{500 * time.Millisecond, false, "Query with new token - should use cached token"},
	}

	refreshCallsBefore := 0
	for i, q := range queries {
		time.Sleep(q.delay)

		creds, err := ttlStore.Get(ctx)
		require.NoError(t, err, "Query %d failed", i)

		refreshCallsNow := mockStore.refreshCalls
		didRefresh := refreshCallsNow > refreshCallsBefore

		t.Logf("Query %d: %s (age: %v, fresh: %v, refreshed: %v)",
			i, q.description, ttlStore.GetTokenAge(), ttlStore.IsTokenFresh(), didRefresh)

		if q.expectRefresh {
			assert.True(t, didRefresh, "Query %d: %s - expected token refresh", i, q.description)
		} else {
			assert.False(t, didRefresh, "Query %d: %s - should use cached token", i, q.description)
		}

		assert.NotNil(t, creds)
		refreshCallsBefore = refreshCallsNow
	}

	t.Logf("Total refresh calls: %d (expected: 2)", mockStore.refreshCalls)
	assert.Equal(t, 2, mockStore.refreshCalls, "Should refresh exactly twice in this scenario (initial + after TTL)")
}

func TestTTLStore_DefaultLifetime(t *testing.T) {
	mockStore := &MockStore{}
	ttlStore := NewTTLStore(mockStore)

	// Verify default lifetime is 14 minutes (15 min - 1 min buffer)
	assert.Equal(t, EffectiveTokenLifetime, ttlStore.tokenLifetime)
	assert.Equal(t, 14*time.Minute, ttlStore.tokenLifetime)
}

func TestTTLStore_CustomLifetime(t *testing.T) {
	mockStore := &MockStore{}
	customLifetime := 5 * time.Minute
	ttlStore := NewTTLStoreWithLifetime(mockStore, customLifetime)

	// Verify custom lifetime is used
	assert.Equal(t, customLifetime, ttlStore.tokenLifetime)
}

// TestTTLStore_ExplainsTheFix documents why the fix works
func TestTTLStore_ExplainsTheFix(t *testing.T) {
	t.Log("=== Why Token Rotation Was Failing ===")
	t.Log("")
	t.Log("PROBLEM: awsrds.Store.Get() caches credentials indefinitely")
	t.Log("")
	t.Log("Without TTLStore:")
	t.Log("  T+0:00  - Store.Refresh() generates token (valid until T+15:00)")
	t.Log("  T+0:00  - Store caches token in v.creds")
	t.Log("  T+14:00 - Connection pool creates new connection")
	t.Log("  T+14:00 - Connector.Connect() calls Store.Get()")
	t.Log("  T+14:00 - Store.Get() returns CACHED token from T+0:00 (14 min old)")
	t.Log("  T+14:00 - Connection succeeds (token still valid for 1 more minute)")
	t.Log("  T+15:05 - Query arrives, uses same connection")
	t.Log("  T+15:05 - Token has expired → ❌ PAM authentication failure")
	t.Log("")
	t.Log("With TTLStore:")
	t.Log("  T+0:00  - TTLStore.Get() calls wrapped.Refresh()")
	t.Log("  T+0:00  - Token generated, cached with timestamp")
	t.Log("  T+14:00 - Connection pool creates new connection")
	t.Log("  T+14:00 - Connector.Connect() calls TTLStore.Get()")
	t.Log("  T+14:00 - TTLStore checks: time.Since(cachedAt) = 14min >= 14min TTL")
	t.Log("  T+14:00 - TTLStore calls wrapped.Refresh() for FRESH token")
	t.Log("  T+14:00 - New token generated (valid until T+29:00)")
	t.Log("  T+14:00 - Connection succeeds with fresh token")
	t.Log("  T+15:05 - Query succeeds (token valid until T+29:00) ✓")
	t.Log("")
	t.Log("The fix ensures tokens are always refreshed BEFORE expiration,")
	t.Log("preventing PAM authentication failures.")
}
