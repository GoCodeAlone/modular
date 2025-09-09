package database

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIAMTokenRefreshBugFix validates that the bug described in the issue is fixed
// Issue: "an application that was running fine, suddenly stopped being able to communicate with the database"
func TestIAMTokenRefreshBugFix(t *testing.T) {
	// This test simulates the exact scenario described in the GitHub issue:
	// 1. Application starts up and works fine with IAM auth
	// 2. Time passes and token expires/gets refreshed
	// 3. Application should continue working (not suddenly stop communicating)

	// Create a mock token provider that simulates token rotation
	mockProvider := NewMockIAMTokenProviderWithExpiry("startup-token", 3*time.Second)

	ctx := context.Background()

	// Step 1: Application startup - build initial DSN with valid IAM token
	config := ConnectionConfig{
		Driver: "postgres",
		DSN:    "postgres://user:password@host.example.com:5432/testdb",
		AWSIAMAuth: &AWSIAMAuthConfig{
			Enabled:              true,
			Region:               "us-east-1",
			DBUser:               "testuser",
			TokenRefreshInterval: 300,
		},
	}

	initialDSN, err := mockProvider.BuildDSNWithIAMToken(ctx, config.DSN)
	require.NoError(t, err, "Should build initial DSN with startup token")
	assert.Contains(t, initialDSN, "startup-token", "Initial DSN should contain startup token")

	// Step 2: Application is working fine initially
	token1, err := mockProvider.GetToken(ctx, "host.example.com:5432")
	require.NoError(t, err, "Should get initial token")
	assert.Equal(t, "startup-token", token1, "Should get startup token")

	// Step 3: Wait for token to expire, then test automatic refresh behavior
	// Sleep to let the token expire naturally
	time.Sleep(4 * time.Second) // Token expires after 3 seconds

	// Now the token should be expired
	_, err = mockProvider.GetToken(ctx, "host.example.com:5432")
	assert.Error(t, err, "Token should be expired after waiting")
	assert.Contains(t, err.Error(), "token expired", "Error should indicate token expiration")

	// Trigger refresh (simulating what the background goroutine would do)
	err = mockProvider.RefreshToken()
	require.NoError(t, err, "Token refresh should succeed")

	// Verify token has been refreshed
	assert.Equal(t, 1, mockProvider.GetRefreshCount(), "Token should have been refreshed once")

	// Get new token to verify it's different and valid
	token2, err := mockProvider.GetToken(ctx, "host.example.com:5432")
	require.NoError(t, err, "Should get refreshed token")
	assert.Equal(t, "refreshed-token-1", token2, "Should get refreshed token")
	assert.NotEqual(t, token1, token2, "Tokens should be different after refresh")

	// Get new DSN to verify it contains the new token
	newDSN, err := mockProvider.BuildDSNWithIAMToken(ctx, config.DSN)
	require.NoError(t, err, "Should build new DSN with refreshed token")
	assert.Contains(t, newDSN, "refreshed-token-1", "New DSN should use refreshed token")
	assert.NotEqual(t, initialDSN, newDSN, "DSN should be different after token refresh")

	// Step 4: Verify multiple token refreshes work with realistic timing
	for i := 0; i < 3; i++ {
		err = mockProvider.RefreshToken()
		require.NoError(t, err, "Multiple token refreshes should work")

		tokenN, err := mockProvider.GetToken(ctx, "host.example.com:5432")
		require.NoError(t, err, "Should get token after each refresh")
		assert.Contains(t, tokenN, fmt.Sprintf("refreshed-token-%d", i+2), "Should get correct refreshed token")
	}

	// Final verification
	assert.Equal(t, 4, mockProvider.GetRefreshCount(), "Should have refreshed 4 times total")

	// This test demonstrates that our token provider correctly:
	// 1. Maintains separate tokens per refresh cycle
	// 2. Updates DSNs with new tokens
	// 3. Allows applications to continue working after token refresh
	// In real usage, the database service callback will recreate connections
	// when the AWSIAMTokenProvider calls the refresh callback
}

// TestTokenRefreshCallbackNotifiesService validates that token refresh properly notifies the service
func TestTokenRefreshCallbackNotifiesService(t *testing.T) {
	// This test validates that our callback mechanism is properly set up

	// Create a mock provider
	mockProvider := NewMockIAMTokenProviderWithExpiry("initial-token", 3*time.Second)

	// Test that the callback mechanism exists and can be tested
	// We don't need to actually connect to test the callback setup

	// Use sync.Mutex to avoid data races
	var mu sync.Mutex
	var callbackReceived bool
	var receivedToken string
	var receivedEndpoint string

	// Test the callback directly
	callback := func(newToken string, endpoint string) {
		mu.Lock()
		defer mu.Unlock()
		callbackReceived = true
		receivedToken = newToken
		receivedEndpoint = endpoint
	}

	// Set callback on mock provider
	mockProvider.SetTokenRefreshCallback(callback)

	// Set endpoint for callback testing
	mockProvider.StartTokenRefresh(context.Background(), "test-endpoint")

	// Trigger a token refresh which should call the callback
	err := mockProvider.RefreshToken()
	require.NoError(t, err, "Token refresh should succeed")

	// Give callback time to execute (it runs in a goroutine)
	time.Sleep(100 * time.Millisecond)

	// Verify callback mechanism works with proper synchronization
	mu.Lock()
	assert.True(t, callbackReceived, "Callback should be received")
	assert.Equal(t, "refreshed-token-1", receivedToken, "Should receive correct token")
	assert.Equal(t, "test-endpoint", receivedEndpoint, "Should receive correct endpoint")
	mu.Unlock()

	// This test demonstrates that the callback mechanism is functional
	// In real usage with AWSIAMTokenProvider, the refreshToken method
	// will call this callback when tokens are refreshed, notifying the
	// database service to recreate connections with the new token
}
