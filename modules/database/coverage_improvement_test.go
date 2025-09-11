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
			currentToken: "cached-token",
			tokenExpiry:  time.Now().Add(5 * time.Minute),
		}
		
		dsn, err := provider.BuildDSNWithIAMToken(context.Background(), "postgres://user:password@localhost:5432/db")
		
		assert.NoError(t, err)
		assert.Contains(t, dsn, "cached-token")
	})
}

// TestAWSIAMTokenProviderSetTokenRefreshCallback tests the SetTokenRefreshCallback method
func TestAWSIAMTokenProviderSetTokenRefreshCallback(t *testing.T) {
	provider := &AWSIAMTokenProvider{}
	
	callbackCalled := false
	callback := func(token, endpoint string) {
		callbackCalled = true
	}
	
	provider.SetTokenRefreshCallback(callback)
	
	// Test that callback is stored
	assert.NotNil(t, provider.tokenRefreshCallback)
	
	// Test that callback can be called
	provider.tokenRefreshCallback("test-token", "test-endpoint")
	assert.True(t, callbackCalled)
}

// TestAWSIAMTokenProviderStopTokenRefresh tests the StopTokenRefresh method
func TestAWSIAMTokenProviderStopTokenRefresh(t *testing.T) {
	t.Run("stops_token_refresh_when_not_started", func(t *testing.T) {
		provider := &AWSIAMTokenProvider{
			refreshStarted: false,
		}
		
		// This should return early and not block
		provider.StopTokenRefresh()
		// Test passes if method returns without blocking
	})
}

// TestDatabaseServiceImplDB tests the DB method with connection mutex
func TestDatabaseServiceImplDB(t *testing.T) {
	service := &databaseServiceImpl{
		connMutex: sync.RWMutex{},
		db:        nil,
	}
	
	// This should not panic even when db is nil
	db := service.DB()
	assert.Nil(t, db)
}

// TestReplacesDSNPasswordFunction tests replaceDSNPassword function edge cases
func TestReplacesDSNPasswordFunction(t *testing.T) {
	t.Run("handles_url_style_dsn_without_userinfo", func(t *testing.T) {
		dsn := "postgres://localhost:5432/database"
		_, err := replaceDSNPassword(dsn, "new-token")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no user information in DSN")
	})
	
	t.Run("adds_password_to_key_value_dsn_when_missing", func(t *testing.T) {
		dsn := "host=localhost port=5432 user=testuser dbname=testdb"
		newDSN, err := replaceDSNPassword(dsn, "new-token")
		
		assert.NoError(t, err)
		assert.Contains(t, newDSN, "password=new-token")
	})
}

// TestLooksLikeHostnameFunction tests looksLikeHostname function edge cases
func TestLooksLikeHostnameFunction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty_string", "", false},
		{"valid_hostname_with_port", "localhost:5432", true},
		{"valid_hostname_with_dot", "db.example.com", true},
		{"localhost_only", "localhost", true},
		{"invalid_with_special_chars", "host!@#$", false},
		{"hostname_with_path", "db.example.com/path", true},
		{"hostname_with_query", "db.example.com?param=value", true},
		{"starts_with_number", "127.0.0.1", true},
		{"starts_with_special_char", "!invalid", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := looksLikeHostname(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsHexDigitFunction tests isHexDigit function
func TestIsHexDigitFunction(t *testing.T) {
	tests := []struct {
		name     string
		input    byte
		expected bool
	}{
		{"digit_0", '0', true},
		{"digit_9", '9', true},
		{"uppercase_A", 'A', true},
		{"uppercase_F", 'F', true},
		{"lowercase_a", 'a', true},
		{"lowercase_f", 'f', true},
		{"invalid_G", 'G', false},
		{"invalid_g", 'g', false},
		{"invalid_special", '!', false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHexDigit(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPreprocessDSNForParsingFunction tests preprocessDSNForParsing function edge cases  
func TestPreprocessDSNForParsingFunction(t *testing.T) {
	t.Run("returns_non_url_dsn_unchanged", func(t *testing.T) {
		dsn := "host=localhost port=5432"
		result, err := preprocessDSNForParsing(dsn)
		
		assert.NoError(t, err)
		assert.Equal(t, dsn, result)
	})
	
	t.Run("returns_dsn_without_credentials_unchanged", func(t *testing.T) {
		dsn := "postgres://localhost:5432/database"
		result, err := preprocessDSNForParsing(dsn)
		
		assert.NoError(t, err)
		assert.Equal(t, dsn, result)
	})
	
	t.Run("returns_dsn_without_password_unchanged", func(t *testing.T) {
		dsn := "postgres://username@localhost:5432/database"
		result, err := preprocessDSNForParsing(dsn)
		
		assert.NoError(t, err)
		assert.Equal(t, dsn, result)
	})
	
	t.Run("returns_already_encoded_dsn_unchanged", func(t *testing.T) {
		dsn := "postgres://username:password%21@localhost:5432/database"
		result, err := preprocessDSNForParsing(dsn)
		
		assert.NoError(t, err)
		assert.Equal(t, dsn, result)
	})
}