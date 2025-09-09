package database

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	_ "modernc.org/sqlite" // Import sqlite driver for testing
)

// TestOnTokenRefresh tests the onTokenRefresh method in service.go
func TestOnTokenRefresh(t *testing.T) {
	// Test early return when db is nil
	t.Run("early_return_when_db_is_nil", func(t *testing.T) {
		service := &databaseServiceImpl{
			config: ConnectionConfig{
				Driver: "sqlite",
				DSN:    "test.db",
			},
			db:     nil, // db is nil
			logger: &MockLogger{},
			ctx:    context.Background(),
		}

		// This should return early and not panic
		service.onTokenRefresh("new-token", "test-endpoint")
		// Test passes if no panic occurs
	})
}

// TestAWSIAMTokenProviderGetToken tests the GetToken method for cached token scenario
func TestAWSIAMTokenProviderGetToken(t *testing.T) {
	t.Run("returns_cached_valid_token", func(t *testing.T) {
		provider := &AWSIAMTokenProvider{
			currentToken: "cached-token",
			tokenExpiry:  time.Now().Add(5 * time.Minute),
		}
		
		token, err := provider.GetToken(context.Background(), "test-endpoint:5432")
		
		assert.NoError(t, err)
		assert.Equal(t, "cached-token", token)
	})
}

// TestAWSIAMTokenProviderBuildDSNWithIAMToken tests the BuildDSNWithIAMToken method  
func TestAWSIAMTokenProviderBuildDSNWithIAMToken(t *testing.T) {
	t.Run("builds_dsn_with_cached_token", func(t *testing.T) {
		provider := &AWSIAMTokenProvider{
			currentToken: "iam-token-12345",
			tokenExpiry:  time.Now().Add(5 * time.Minute),
		}
		
		dsn, err := provider.BuildDSNWithIAMToken(context.Background(), "postgres://user:oldpassword@host.example.com:5432/dbname")
		
		assert.NoError(t, err)
		assert.Equal(t, "postgres://user:iam-token-12345@host.example.com:5432/dbname", dsn)
	})

	t.Run("error_with_invalid_dsn", func(t *testing.T) {
		provider := &AWSIAMTokenProvider{
			currentToken: "iam-token-12345",
			tokenExpiry:  time.Now().Add(5 * time.Minute),
		}
		
		_, err := provider.BuildDSNWithIAMToken(context.Background(), "invalid-dsn-format")
		
		assert.Error(t, err)
	})
}

// TestRefreshTokenWithCallback tests the refreshToken method with callback scenarios
func TestRefreshTokenWithCallback(t *testing.T) {
	tests := []struct {
		name              string
		setupCallback     bool
		expectCallbackRun bool
	}{
		{
			name:              "refresh_token_without_callback",
			setupCallback:     false,
			expectCallbackRun: false,
		},
		{
			name:              "refresh_token_with_callback_success",
			setupCallback:     true,
			expectCallbackRun: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use mock provider for easier testing
			provider := NewMockIAMTokenProviderWithExpiry("initial-token", 1*time.Hour)
			
			var mutex sync.Mutex
			var callbackCalled bool
			var callbackToken string
			
			if tt.setupCallback {
				callback := func(token, endpoint string) {
					mutex.Lock()
					defer mutex.Unlock()
					callbackCalled = true
					callbackToken = token
				}
				provider.SetTokenRefreshCallback(callback)
			}
			
			// Trigger refresh
			err := provider.RefreshToken()
			assert.NoError(t, err)
			
			// Give callback time to run (it's in a goroutine)
			if tt.setupCallback {
				time.Sleep(50 * time.Millisecond)
			}
			
			// Read callback results with mutex protection
			mutex.Lock()
			actualCallbackCalled := callbackCalled
			actualCallbackToken := callbackToken
			mutex.Unlock()
			
			if tt.expectCallbackRun {
				assert.True(t, actualCallbackCalled, "Callback should have been called")
				assert.Equal(t, "refreshed-token-1", actualCallbackToken)
			} else {
				assert.False(t, actualCallbackCalled, "Callback should not have been called")
			}
		})
	}
}