package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				JWT: JWTConfig{
					Secret:            "test-secret",
					Expiration:        time.Hour,
					RefreshExpiration: time.Hour * 24,
				},
				Password: PasswordConfig{
					MinLength:  8,
					BcryptCost: 12,
				},
			},
			wantErr: false,
		},
		{
			name: "missing JWT secret",
			config: &Config{
				JWT: JWTConfig{
					Secret:            "",
					Expiration:        time.Hour,
					RefreshExpiration: time.Hour * 24,
				},
				Password: PasswordConfig{
					MinLength:  8,
					BcryptCost: 12,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid JWT expiration",
			config: &Config{
				JWT: JWTConfig{
					Secret:            "test-secret",
					Expiration:        0,
					RefreshExpiration: time.Hour * 24,
				},
				Password: PasswordConfig{
					MinLength:  8,
					BcryptCost: 12,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid password min length",
			config: &Config{
				JWT: JWTConfig{
					Secret:            "test-secret",
					Expiration:        time.Hour,
					RefreshExpiration: time.Hour * 24,
				},
				Password: PasswordConfig{
					MinLength:  0,
					BcryptCost: 12,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid bcrypt cost",
			config: &Config{
				JWT: JWTConfig{
					Secret:            "test-secret",
					Expiration:        time.Hour,
					RefreshExpiration: time.Hour * 24,
				},
				Password: PasswordConfig{
					MinLength:  8,
					BcryptCost: 3, // Too low
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestService_GenerateToken(t *testing.T) {
	config := &Config{
		JWT: JWTConfig{
			Secret:            "test-secret",
			Expiration:        time.Hour,
			RefreshExpiration: time.Hour * 24,
			Issuer:            "test-issuer",
		},
	}

	userStore := NewMemoryUserStore()
	sessionStore := NewMemorySessionStore()
	service := NewService(config, userStore, sessionStore)

	userID := "test-user-123"
	customClaims := map[string]interface{}{
		"email": "test@example.com",
		"roles": []string{"user"},
	}

	tokenPair, err := service.GenerateToken(userID, customClaims)
	require.NoError(t, err)
	require.NotNil(t, tokenPair)

	assert.NotEmpty(t, tokenPair.AccessToken)
	assert.NotEmpty(t, tokenPair.RefreshToken)
	assert.Equal(t, "Bearer", tokenPair.TokenType)
	assert.Equal(t, int64(config.JWT.Expiration.Seconds()), tokenPair.ExpiresIn)
	assert.True(t, time.Now().Before(tokenPair.ExpiresAt))
}

func TestService_ValidateToken(t *testing.T) {
	config := &Config{
		JWT: JWTConfig{
			Secret:            "test-secret",
			Expiration:        time.Hour,
			RefreshExpiration: time.Hour * 24,
			Issuer:            "test-issuer",
		},
	}

	userStore := NewMemoryUserStore()
	sessionStore := NewMemorySessionStore()
	service := NewService(config, userStore, sessionStore)

	userID := "test-user-123"
	customClaims := map[string]interface{}{
		"email":        "test@example.com",
		"roles":        []string{"user", "admin"},
		"permissions":  []string{"read", "write"},
		"custom_field": "custom_value",
	}

	// Generate token
	tokenPair, err := service.GenerateToken(userID, customClaims)
	require.NoError(t, err)

	// Validate token
	claims, err := service.ValidateToken(tokenPair.AccessToken)
	require.NoError(t, err)
	require.NotNil(t, claims)

	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, []string{"user", "admin"}, claims.Roles)
	assert.Equal(t, []string{"read", "write"}, claims.Permissions)
	assert.Equal(t, "test-issuer", claims.Issuer)
	assert.Equal(t, userID, claims.Subject)
	assert.Equal(t, "custom_value", claims.Custom["custom_field"])
}

func TestService_ValidateToken_Invalid(t *testing.T) {
	config := &Config{
		JWT: JWTConfig{
			Secret:            "test-secret",
			Expiration:        time.Hour,
			RefreshExpiration: time.Hour * 24,
		},
	}

	userStore := NewMemoryUserStore()
	sessionStore := NewMemorySessionStore()
	service := NewService(config, userStore, sessionStore)

	tests := []struct {
		name      string
		token     string
		expectErr error
	}{
		{
			name:      "empty token",
			token:     "",
			expectErr: ErrTokenInvalid,
		},
		{
			name:      "malformed token",
			token:     "invalid.token.format",
			expectErr: ErrTokenInvalid,
		},
		{
			name:      "token with wrong secret",
			token:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoidGVzdCIsImV4cCI6OTk5OTk5OTk5OSwidHlwZSI6ImFjY2VzcyJ9.invalid",
			expectErr: ErrTokenInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.ValidateToken(tt.token)
			assert.ErrorIs(t, err, tt.expectErr)
		})
	}
}

func TestService_RefreshToken(t *testing.T) {
	config := &Config{
		JWT: JWTConfig{
			Secret:            "test-secret",
			Expiration:        time.Hour,
			RefreshExpiration: time.Hour * 24,
		},
	}

	userStore := NewMemoryUserStore()
	sessionStore := NewMemorySessionStore()
	service := NewService(config, userStore, sessionStore)

	// Create a test user
	user := &User{
		ID:          "test-user-123",
		Email:       "test@example.com",
		Roles:       []string{"user"},
		Permissions: []string{"read"},
		Active:      true,
	}
	err := userStore.CreateUser(nil, user)
	require.NoError(t, err)

	// Generate initial token pair
	tokenPair, err := service.GenerateToken(user.ID, map[string]interface{}{
		"email": user.Email,
	})
	require.NoError(t, err)

	// Refresh token
	newTokenPair, err := service.RefreshToken(tokenPair.RefreshToken)
	require.NoError(t, err)
	require.NotNil(t, newTokenPair)

	assert.NotEmpty(t, newTokenPair.AccessToken)
	assert.NotEmpty(t, newTokenPair.RefreshToken)
	assert.NotEqual(t, tokenPair.AccessToken, newTokenPair.AccessToken)
	assert.NotEqual(t, tokenPair.RefreshToken, newTokenPair.RefreshToken)

	// Validate new access token contains updated user info
	claims, err := service.ValidateToken(newTokenPair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, user.Email, claims.Email)
	assert.Equal(t, user.Roles, claims.Roles)
	assert.Equal(t, user.Permissions, claims.Permissions)
}

func TestService_HashPassword(t *testing.T) {
	config := &Config{
		Password: PasswordConfig{
			BcryptCost: 4, // Low cost for testing
		},
	}

	userStore := NewMemoryUserStore()
	sessionStore := NewMemorySessionStore()
	service := NewService(config, userStore, sessionStore)

	password := "testpassword123"
	hash, err := service.HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)
}

func TestService_VerifyPassword(t *testing.T) {
	config := &Config{
		Password: PasswordConfig{
			BcryptCost: 4, // Low cost for testing
		},
	}

	userStore := NewMemoryUserStore()
	sessionStore := NewMemorySessionStore()
	service := NewService(config, userStore, sessionStore)

	password := "testpassword123"
	hash, err := service.HashPassword(password)
	require.NoError(t, err)

	// Correct password should verify
	err = service.VerifyPassword(hash, password)
	assert.NoError(t, err)

	// Wrong password should fail
	err = service.VerifyPassword(hash, "wrongpassword")
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestService_ValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		config   PasswordConfig
		password string
		wantErr  bool
	}{
		{
			name: "valid password with all requirements",
			config: PasswordConfig{
				MinLength:      8,
				RequireUpper:   true,
				RequireLower:   true,
				RequireDigit:   true,
				RequireSpecial: true,
			},
			password: "Password123!",
			wantErr:  false,
		},
		{
			name: "password too short",
			config: PasswordConfig{
				MinLength: 10,
			},
			password: "short",
			wantErr:  true,
		},
		{
			name: "missing uppercase",
			config: PasswordConfig{
				MinLength:    8,
				RequireUpper: true,
			},
			password: "password123",
			wantErr:  true,
		},
		{
			name: "missing lowercase",
			config: PasswordConfig{
				MinLength:    8,
				RequireLower: true,
			},
			password: "PASSWORD123",
			wantErr:  true,
		},
		{
			name: "missing digit",
			config: PasswordConfig{
				MinLength:    8,
				RequireDigit: true,
			},
			password: "Password",
			wantErr:  true,
		},
		{
			name: "missing special character",
			config: PasswordConfig{
				MinLength:      8,
				RequireSpecial: true,
			},
			password: "Password123",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Password: tt.config,
			}

			userStore := NewMemoryUserStore()
			sessionStore := NewMemorySessionStore()
			service := NewService(config, userStore, sessionStore)

			err := service.ValidatePasswordStrength(tt.password)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrPasswordTooWeak)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestService_Sessions(t *testing.T) {
	config := &Config{
		Session: SessionConfig{
			MaxAge: time.Hour,
		},
	}

	userStore := NewMemoryUserStore()
	sessionStore := NewMemorySessionStore()
	service := NewService(config, userStore, sessionStore)

	userID := "test-user-123"
	metadata := map[string]interface{}{
		"ip_address": "127.0.0.1",
		"user_agent": "test-browser",
	}

	// Create session
	session, err := service.CreateSession(userID, metadata)
	require.NoError(t, err)
	require.NotNil(t, session)

	assert.NotEmpty(t, session.ID)
	assert.Equal(t, userID, session.UserID)
	assert.True(t, session.Active)
	assert.True(t, time.Now().Before(session.ExpiresAt))
	assert.Equal(t, metadata, session.Metadata)

	// Get session
	retrievedSession, err := service.GetSession(session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, retrievedSession.ID)
	assert.Equal(t, session.UserID, retrievedSession.UserID)

	// Capture original expiration time before refreshing
	originalExpiresAt := session.ExpiresAt

	// Refresh session
	refreshedSession, err := service.RefreshSession(session.ID)
	require.NoError(t, err)
	assert.True(t, refreshedSession.ExpiresAt.After(originalExpiresAt))

	// Delete session
	err = service.DeleteSession(session.ID)
	require.NoError(t, err)

	// Should not be able to get deleted session
	_, err = service.GetSession(session.ID)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestService_OAuth2(t *testing.T) {
	config := &Config{
		OAuth2: OAuth2Config{
			Providers: map[string]OAuth2Provider{
				"google": {
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					RedirectURL:  "http://localhost:8080/auth/google/callback",
					Scopes:       []string{"openid", "email", "profile"},
					AuthURL:      "https://accounts.google.com/o/oauth2/auth",
					TokenURL:     "https://oauth2.googleapis.com/token",
					UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
				},
			},
		},
	}

	userStore := NewMemoryUserStore()
	sessionStore := NewMemorySessionStore()
	service := NewService(config, userStore, sessionStore)

	// Test getting OAuth2 auth URL
	authURL, err := service.GetOAuth2AuthURL("google", "test-state")
	require.NoError(t, err)
	assert.Contains(t, authURL, "accounts.google.com")
	assert.Contains(t, authURL, "test-client-id")
	assert.Contains(t, authURL, "test-state")

	// Test with non-existent provider
	_, err = service.GetOAuth2AuthURL("nonexistent", "test-state")
	assert.ErrorIs(t, err, ErrProviderNotFound)

	// Note: ExchangeOAuth2Code would require actual OAuth2 flow to test properly
	// In a real implementation, this would be tested with mock HTTP clients
}
