package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

// Service implements the AuthService interface
type Service struct {
	config        *Config
	userStore     UserStore
	sessionStore  SessionStore
	oauth2Configs map[string]*oauth2.Config
	tokenCounter  int64 // Add counter to ensure unique tokens
}

// NewService creates a new authentication service
func NewService(config *Config, userStore UserStore, sessionStore SessionStore) *Service {
	s := &Service{
		config:        config,
		userStore:     userStore,
		sessionStore:  sessionStore,
		oauth2Configs: make(map[string]*oauth2.Config),
	}

	// Initialize OAuth2 configurations if providers exist
	if len(config.OAuth2.Providers) > 0 {
		for name, provider := range config.OAuth2.Providers {
			s.oauth2Configs[name] = &oauth2.Config{
				ClientID:     provider.ClientID,
				ClientSecret: provider.ClientSecret,
				RedirectURL:  provider.RedirectURL,
				Scopes:       provider.Scopes,
				Endpoint: oauth2.Endpoint{
					AuthURL:  provider.AuthURL,
					TokenURL: provider.TokenURL,
				},
			}
		}
	}

	return s
}

// GenerateToken creates a new JWT token pair
func (s *Service) GenerateToken(userID string, customClaims map[string]interface{}) (*TokenPair, error) {
	now := time.Now()
	// Add atomic counter to ensure uniqueness
	counter := atomic.AddInt64(&s.tokenCounter, 1)

	// Generate access token
	accessClaims := jwt.MapClaims{
		"user_id": userID,
		"type":    "access",
		"iat":     now.Unix(),
		"exp":     now.Add(s.config.JWT.Expiration).Unix(),
		"counter": counter, // Add counter to make tokens unique
	}

	if s.config.JWT.Issuer != "" {
		accessClaims["iss"] = s.config.JWT.Issuer
	}
	accessClaims["sub"] = userID

	// Add custom claims
	for key, value := range customClaims {
		accessClaims[key] = value
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.config.JWT.Secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Generate refresh token with different counter
	refreshCounter := atomic.AddInt64(&s.tokenCounter, 1)
	refreshClaims := jwt.MapClaims{
		"user_id": userID,
		"type":    "refresh",
		"iat":     now.Unix(),
		"exp":     now.Add(s.config.JWT.RefreshExpiration).Unix(),
		"counter": refreshCounter, // Different counter for refresh token
	}

	if s.config.JWT.Issuer != "" {
		refreshClaims["iss"] = s.config.JWT.Issuer
	}
	refreshClaims["sub"] = userID

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(s.config.JWT.Secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	expiresAt := now.Add(s.config.JWT.Expiration)

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.config.JWT.Expiration.Seconds()),
		ExpiresAt:    expiresAt,
	}, nil
}

// ValidateToken validates a JWT token and returns the claims
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWT.Secret), nil
	})

	if err != nil {
		if strings.Contains(err.Error(), "token is expired") {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	if !token.Valid {
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrTokenMalformed
	}

	// Check token type
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "access" {
		return nil, ErrTokenInvalid
	}

	// Extract claims
	userID, _ := claims["user_id"].(string)
	email, _ := claims["email"].(string)
	issuer, _ := claims["iss"].(string)
	subject, _ := claims["sub"].(string)

	var roles []string
	if rolesInterface, exists := claims["roles"]; exists {
		if rolesList, ok := rolesInterface.([]interface{}); ok {
			for _, role := range rolesList {
				if roleStr, ok := role.(string); ok {
					roles = append(roles, roleStr)
				}
			}
		}
	}

	var permissions []string
	if permsInterface, exists := claims["permissions"]; exists {
		if permsList, ok := permsInterface.([]interface{}); ok {
			for _, perm := range permsList {
				if permStr, ok := perm.(string); ok {
					permissions = append(permissions, permStr)
				}
			}
		}
	}

	issuedAt := time.Unix(int64(claims["iat"].(float64)), 0)
	expiresAt := time.Unix(int64(claims["exp"].(float64)), 0)

	// Extract custom claims
	custom := make(map[string]interface{})
	standardClaims := map[string]bool{
		"user_id": true, "email": true, "roles": true, "permissions": true,
		"iat": true, "exp": true, "iss": true, "sub": true, "type": true,
	}
	for k, v := range claims {
		if !standardClaims[k] {
			custom[k] = v
		}
	}

	return &Claims{
		UserID:      userID,
		Email:       email,
		Roles:       roles,
		Permissions: permissions,
		IssuedAt:    issuedAt,
		ExpiresAt:   expiresAt,
		Issuer:      issuer,
		Subject:     subject,
		Custom:      custom,
	}, nil
}

// RefreshToken creates a new token pair using a refresh token
func (s *Service) RefreshToken(refreshTokenString string) (*TokenPair, error) {
	token, err := jwt.Parse(refreshTokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWT.Secret), nil
	})

	if err != nil {
		if strings.Contains(err.Error(), "token is expired") {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	if !token.Valid {
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrTokenMalformed
	}

	// Check token type
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return nil, ErrTokenInvalid
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return nil, ErrTokenMalformed
	}

	// Get user to include current roles and permissions
	user, err := s.userStore.GetUser(context.Background(), userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if !user.Active {
		return nil, ErrInvalidCredentials
	}

	// Add a small delay to ensure different timestamps for new tokens
	time.Sleep(time.Millisecond)

	// Create new token pair with current user data
	customClaims := map[string]interface{}{
		"email":       user.Email,
		"roles":       user.Roles,
		"permissions": user.Permissions,
	}

	return s.GenerateToken(userID, customClaims)
}

// HashPassword hashes a password using bcrypt
func (s *Service) HashPassword(password string) (string, error) {
	cost := s.config.Password.BcryptCost
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword verifies a password against its hash
func (s *Service) VerifyPassword(hashedPassword, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// ValidatePasswordStrength validates password against configured requirements
func (s *Service) ValidatePasswordStrength(password string) error {
	if len(password) < s.config.Password.MinLength {
		return ErrPasswordTooWeak
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if s.config.Password.RequireUpper && !hasUpper {
		return ErrPasswordTooWeak
	}
	if s.config.Password.RequireLower && !hasLower {
		return ErrPasswordTooWeak
	}
	if s.config.Password.RequireDigit && !hasDigit {
		return ErrPasswordTooWeak
	}
	if s.config.Password.RequireSpecial && !hasSpecial {
		return ErrPasswordTooWeak
	}

	return nil
}

// CreateSession creates a new user session
func (s *Service) CreateSession(userID string, metadata map[string]interface{}) (*Session, error) {
	sessionID, err := generateRandomID(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	now := time.Now()
	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(s.config.Session.MaxAge),
		Active:    true,
		Metadata:  metadata,
	}

	err = s.sessionStore.Store(context.Background(), session)
	if err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	return session, nil
}

// GetSession retrieves a session by ID
func (s *Service) GetSession(sessionID string) (*Session, error) {
	session, err := s.sessionStore.Get(context.Background(), sessionID)
	if err != nil {
		return nil, ErrSessionNotFound
	}

	if time.Now().After(session.ExpiresAt) {
		_ = s.sessionStore.Delete(context.Background(), sessionID) // Ignore error for expired session cleanup
		return nil, ErrSessionExpired
	}

	if !session.Active {
		return nil, ErrSessionNotFound
	}

	return session, nil
}

// DeleteSession removes a session
func (s *Service) DeleteSession(sessionID string) error {
	return s.sessionStore.Delete(context.Background(), sessionID)
}

// RefreshSession extends a session's expiration time
func (s *Service) RefreshSession(sessionID string) (*Session, error) {
	session, err := s.sessionStore.Get(context.Background(), sessionID)
	if err != nil {
		return nil, err
	}

	if !session.Active {
		return nil, ErrSessionExpired
	}

	// Capture the original expiration time before modifying
	originalExpiresAt := session.ExpiresAt

	// Add a small delay to ensure the new expiration time is detectably later
	time.Sleep(time.Millisecond)

	// Update expiration time to extend the session
	newExpiresAt := time.Now().Add(s.config.Session.MaxAge)
	session.ExpiresAt = newExpiresAt

	// Ensure the new expiration is actually later than the original
	if !newExpiresAt.After(originalExpiresAt) {
		// If for some reason it's not later, add extra time
		session.ExpiresAt = originalExpiresAt.Add(time.Millisecond)
	}

	err = s.sessionStore.Store(context.Background(), session)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// GetOAuth2AuthURL returns the OAuth2 authorization URL for a provider
func (s *Service) GetOAuth2AuthURL(provider, state string) (string, error) {
	config, exists := s.oauth2Configs[provider]
	if !exists {
		return "", ErrProviderNotFound
	}

	return config.AuthCodeURL(state), nil
}

// ExchangeOAuth2Code exchanges an OAuth2 authorization code for user info
func (s *Service) ExchangeOAuth2Code(provider, code, state string) (*OAuth2Result, error) {
	config, exists := s.oauth2Configs[provider]
	if !exists {
		return nil, ErrProviderNotFound
	}

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOAuth2Failed, err)
	}

	// Get user info from provider
	userInfo, err := s.fetchOAuth2UserInfo(provider, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	return &OAuth2Result{
		Provider:     provider,
		UserInfo:     userInfo,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.Expiry,
	}, nil
}

// fetchOAuth2UserInfo fetches user information from OAuth2 provider
func (s *Service) fetchOAuth2UserInfo(provider, accessToken string) (map[string]interface{}, error) {
	providerConfig, exists := s.config.OAuth2.Providers[provider]
	if !exists {
		return nil, ErrProviderNotFound
	}

	if providerConfig.UserInfoURL == "" {
		return nil, fmt.Errorf("user info URL not configured for provider %s", provider)
	}

	// This is a simplified implementation - in practice, you'd make an HTTP request
	// to the provider's user info endpoint using the access token
	userInfo := map[string]interface{}{
		"provider": provider,
		"token":    accessToken,
	}

	return userInfo, nil
}

// generateRandomID generates a random hex-encoded ID
func generateRandomID(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
