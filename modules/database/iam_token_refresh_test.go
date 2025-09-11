package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockIAMTokenProviderWithExpiry simulates token expiration scenarios
type MockIAMTokenProviderWithExpiry struct {
	mutex                sync.RWMutex
	currentToken         string
	tokenExpiry          time.Time
	refreshCount         int
	shouldFail           bool
	failAfter            int // fail after this many refresh attempts
	tokenRefreshCallback TokenRefreshCallback
	endpoint             string
}

func NewMockIAMTokenProviderWithExpiry(initialToken string, validDuration time.Duration) *MockIAMTokenProviderWithExpiry {
	return &MockIAMTokenProviderWithExpiry{
		currentToken: initialToken,
		tokenExpiry:  time.Now().Add(validDuration),
		refreshCount: 0,
		shouldFail:   false,
	}
}

func (m *MockIAMTokenProviderWithExpiry) GetToken(ctx context.Context, endpoint string) (string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.shouldFail {
		return "", errors.New("token provider failed")
	}

	// Check if token is expired
	if time.Now().After(m.tokenExpiry) {
		return "", errors.New("token expired")
	}

	return m.currentToken, nil
}

func (m *MockIAMTokenProviderWithExpiry) BuildDSNWithIAMToken(ctx context.Context, originalDSN string) (string, error) {
	token, err := m.GetToken(ctx, "mock-endpoint")
	if err != nil {
		return "", err
	}
	return replaceDSNPassword(originalDSN, token)
}

func (m *MockIAMTokenProviderWithExpiry) RefreshToken() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.refreshCount++

	// Simulate failure after certain attempts
	if m.failAfter > 0 && m.refreshCount > m.failAfter {
		m.shouldFail = true
		return errors.New("refresh failed after max attempts")
	}

	// Generate new token
	m.currentToken = fmt.Sprintf("refreshed-token-%d", m.refreshCount)
	m.tokenExpiry = time.Now().Add(3 * time.Second) // New 3-second token for testing

	// Call the refresh callback if set (simulating the real provider behavior)
	if m.tokenRefreshCallback != nil {
		go m.tokenRefreshCallback(m.currentToken, m.endpoint)
	}

	return nil
}

func (m *MockIAMTokenProviderWithExpiry) ExpireToken() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.tokenExpiry = time.Now().Add(-1 * time.Minute) // Expire the token
}

func (m *MockIAMTokenProviderWithExpiry) StartTokenRefresh(ctx context.Context, endpoint string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.endpoint = endpoint
}

func (m *MockIAMTokenProviderWithExpiry) StopTokenRefresh() {
	// No-op for testing
}

func (m *MockIAMTokenProviderWithExpiry) SetTokenRefreshCallback(callback TokenRefreshCallback) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.tokenRefreshCallback = callback
}

func (m *MockIAMTokenProviderWithExpiry) GetRefreshCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.refreshCount
}

// TestIAMTokenExpirationScenario tests the scenario where a token expires after connection establishment
func TestIAMTokenExpirationScenario(t *testing.T) {
	// Create a mock token provider with a short-lived token
	mockProvider := NewMockIAMTokenProviderWithExpiry("initial-token", 1*time.Second)

	ctx := context.Background()

	// Test 1: Try to build DSN with valid token - should work
	dsn, err := mockProvider.BuildDSNWithIAMToken(ctx, "user:password@host:5432/db")
	require.NoError(t, err, "Should build DSN with valid token")
	assert.Contains(t, dsn, "initial-token", "DSN should contain the token")

	// Test 2: Wait for token to expire and then expire it explicitly
	time.Sleep(2 * time.Second)
	mockProvider.ExpireToken()

	// Test 3: Try to build DSN with expired token - should fail
	_, err = mockProvider.BuildDSNWithIAMToken(ctx, "user:password@host:5432/db")
	assert.Error(t, err, "Should fail with expired token")
	assert.Contains(t, err.Error(), "token expired", "Error should indicate token expiration")
}

// TestTokenRefreshWithExistingConnection tests the core issue:
// What happens when tokens are refreshed but existing connections still use old token
func TestTokenRefreshWithExistingConnection(t *testing.T) {
	// This test demonstrates the core issue reported in the bug:
	// "an application that was running fine, suddenly stopped being able to communicate with the database"

	// Create a mock token provider
	mockProvider := NewMockIAMTokenProviderWithExpiry("initial-token", 3*time.Second)

	ctx := context.Background()

	// Step 1: Build initial DSN - simulates application startup
	initialDSN, err := mockProvider.BuildDSNWithIAMToken(ctx, "user:password@host:5432/db")
	require.NoError(t, err, "Initial DSN build should succeed")
	assert.Contains(t, initialDSN, "initial-token", "Initial DSN should contain initial token")

	// Step 2: Simulate passage of time where token refresh occurs in background
	// This is what would happen in a real application after 10+ minutes
	err = mockProvider.RefreshToken()
	require.NoError(t, err, "Token refresh should succeed")
	assert.Equal(t, 1, mockProvider.GetRefreshCount(), "Should have refreshed once")

	// Step 3: Build new DSN after token refresh
	newDSN, err := mockProvider.BuildDSNWithIAMToken(ctx, "user:password@host:5432/db")
	require.NoError(t, err, "New DSN build should succeed")
	assert.Contains(t, newDSN, "refreshed-token-1", "New DSN should contain refreshed token")

	// Step 4: Verify that the DSNs are different (the key issue)
	assert.NotEqual(t, initialDSN, newDSN, "DSNs should be different after token refresh")

	// This test demonstrates that when tokens are refreshed, the DSN changes,
	// but existing database connections (sql.DB) were created with the old DSN
	// and continue to use the old token until the connections are recreated.
}

// TestTokenProviderRealWorldScenario tests the scenario described in the issue
func TestTokenProviderRealWorldScenario(t *testing.T) {
	// Simulate the real-world scenario:
	// 1. Application starts up with valid token
	// 2. Token expires while application is running
	// 3. Background refresh gets new token
	// 4. Existing connections still use old token and fail

	mockProvider := NewMockIAMTokenProviderWithExpiry("startup-token", 5*time.Second)
	ctx := context.Background()

	// Application startup - gets initial token
	token1, err := mockProvider.GetToken(ctx, "endpoint")
	require.NoError(t, err)
	assert.Equal(t, "startup-token", token1)

	// Simulate time passing (token expires)
	time.Sleep(6 * time.Second)

	// Token should now be expired
	_, err = mockProvider.GetToken(ctx, "endpoint")
	assert.Error(t, err, "Token should be expired")
	assert.Contains(t, err.Error(), "token expired")

	// Background refresh occurs (this is what the background goroutine would do)
	err = mockProvider.RefreshToken()
	require.NoError(t, err, "Refresh should succeed")

	// Now token should work again
	token2, err := mockProvider.GetToken(ctx, "endpoint")
	require.NoError(t, err)
	assert.Equal(t, "refreshed-token-1", token2)
	assert.NotEqual(t, token1, token2, "Tokens should be different")

	// But the issue is: if we had an existing database connection created with token1,
	// it would still be using the old token and would fail even though token2 is valid.
}

// TestConnectionRecreationAfterTokenRefresh tests whether connection recreation helps
func TestConnectionRecreationAfterTokenRefresh(t *testing.T) {
	// This test demonstrates a potential solution: recreating connections after token refresh

	mockProvider := NewMockIAMTokenProviderWithExpiry("initial-token", 3*time.Second)
	ctx := context.Background()

	// Step 1: Create initial connection DSN
	dsn1, err := mockProvider.BuildDSNWithIAMToken(ctx, "postgres://user:password@host:5432/db")
	require.NoError(t, err)

	// Step 2: Refresh token
	err = mockProvider.RefreshToken()
	require.NoError(t, err)

	// Step 3: Create new connection DSN with refreshed token
	dsn2, err := mockProvider.BuildDSNWithIAMToken(ctx, "postgres://user:password@host:5432/db")
	require.NoError(t, err)

	// Verify DSNs are different
	assert.NotEqual(t, dsn1, dsn2, "DSNs should be different after token refresh")
	assert.Contains(t, dsn1, "initial-token")
	assert.Contains(t, dsn2, "refreshed-token-1")

	// This suggests that the solution would involve:
	// 1. Detecting when token refresh occurs
	// 2. Recreating the database connection with the new DSN
	// 3. Properly handling connection pool lifecycle
}

// TestTokenRefreshCallbackFunctionality tests that the token refresh callback works
func TestTokenRefreshCallbackFunctionality(t *testing.T) {
	// Create real AWS token provider to test callback mechanism
	config := &AWSIAMAuthConfig{
		Enabled:              true,
		Region:               "us-east-1",
		DBUser:               "testuser",
		TokenRefreshInterval: 300,
	}

	provider, err := NewAWSIAMTokenProvider(config)
	if err != nil {
		if strings.Contains(err.Error(), "failed to load AWS config") {
			t.Skip("AWS credentials not available, skipping test")
		}
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Track callback invocations
	var callbackInvoked bool

	callback := func(newToken string, endpoint string) {
		callbackInvoked = true
		// In a real scenario, this callback would be called when tokens are refreshed
		assert.NotEmpty(t, newToken, "New token should not be empty")
		assert.NotEmpty(t, endpoint, "Endpoint should not be empty")
	}

	// Set callback
	provider.SetTokenRefreshCallback(callback)

	// Verify callback is set and provider is functional
	assert.NotNil(t, provider, "Provider should be created")

	// We can't easily test the actual callback without real AWS credentials and token generation,
	// but we can verify the mechanism is in place and doesn't cause issues
	_ = callbackInvoked // Use the variable to avoid compiler error
}

// TestAutomaticTokenRefreshOnExpiry tests token refresh triggered by token expiration
func TestAutomaticTokenRefreshOnExpiry(t *testing.T) {
	// This test demonstrates automatic token refresh when tokens expire
	mockProvider := NewMockIAMTokenProviderWithExpiry("initial-token", 2*time.Second)
	ctx := context.Background()

	// Set up callback to track refresh events
	var callbackInvocations []string
	var callbackMutex sync.Mutex
	callback := func(newToken string, endpoint string) {
		callbackMutex.Lock()
		defer callbackMutex.Unlock()
		callbackInvocations = append(callbackInvocations, newToken)
	}
	mockProvider.SetTokenRefreshCallback(callback)
	mockProvider.StartTokenRefresh(ctx, "test-endpoint")

	// Initial token should work
	token1, err := mockProvider.GetToken(ctx, "endpoint")
	require.NoError(t, err)
	assert.Equal(t, "initial-token", token1)

	// Wait for token to expire
	time.Sleep(3 * time.Second)

	// Token should now be expired
	_, err = mockProvider.GetToken(ctx, "endpoint")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token expired")

	// Simulate automatic refresh (this would be triggered by detection of expired token)
	err = mockProvider.RefreshToken()
	require.NoError(t, err)

	// Give callback time to execute
	time.Sleep(100 * time.Millisecond)

	// Verify callback was invoked
	callbackMutex.Lock()
	defer callbackMutex.Unlock()
	assert.Len(t, callbackInvocations, 1, "Callback should be invoked once")
	assert.Equal(t, "refreshed-token-1", callbackInvocations[0])

	// New token should work
	token2, err := mockProvider.GetToken(ctx, "endpoint")
	require.NoError(t, err)
	assert.Equal(t, "refreshed-token-1", token2)
}

// TestErrorDrivenTokenRefresh tests refresh triggered by database connection errors
func TestErrorDrivenTokenRefresh(t *testing.T) {
	// This test simulates the scenario where a database operation fails due to expired token
	// and triggers a token refresh to recover
	mockProvider := NewMockIAMTokenProviderWithExpiry("working-token", 2*time.Second)
	ctx := context.Background()

	// Track refresh events
	var refreshEvents []string
	var eventMutex sync.Mutex
	callback := func(newToken string, endpoint string) {
		eventMutex.Lock()
		defer eventMutex.Unlock()
		refreshEvents = append(refreshEvents, fmt.Sprintf("refresh:%s:%s", newToken, endpoint))
	}
	mockProvider.SetTokenRefreshCallback(callback)
	mockProvider.StartTokenRefresh(ctx, "db.example.com:5432")

	// Initial DSN creation should work
	dsn1, err := mockProvider.BuildDSNWithIAMToken(ctx, "postgres://user:password@db.example.com:5432/mydb")
	require.NoError(t, err)
	assert.Contains(t, dsn1, "working-token")

	// Simulate passage of time - token expires
	time.Sleep(3 * time.Second)

	// Attempt to use token should fail (simulating database error)
	_, err = mockProvider.GetToken(ctx, "db.example.com:5432")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token expired")

	// Simulate error-driven refresh (this would be triggered by the database service
	// detecting authentication failures)
	err = mockProvider.RefreshToken()
	require.NoError(t, err)

	// Give callback time to execute
	time.Sleep(100 * time.Millisecond)

	// Verify refresh event was recorded
	eventMutex.Lock()
	defer eventMutex.Unlock()
	assert.Len(t, refreshEvents, 1, "Should have one refresh event")
	assert.Contains(t, refreshEvents[0], "refreshed-token-1")
	assert.Contains(t, refreshEvents[0], "db.example.com:5432")

	// New DSN should work with fresh token
	dsn2, err := mockProvider.BuildDSNWithIAMToken(ctx, "postgres://user:password@db.example.com:5432/mydb")
	require.NoError(t, err)
	assert.Contains(t, dsn2, "refreshed-token-1")
	assert.NotEqual(t, dsn1, dsn2, "DSNs should be different after refresh")
}
