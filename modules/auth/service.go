package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/CrisisTextLine/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

// EventEmitter interface for emitting auth events
type EventEmitter interface {
	EmitEvent(ctx context.Context, event cloudevents.Event) error
}

// Service implements the AuthService interface
type Service struct {
	config        *Config
	userStore     UserStore
	sessionStore  SessionStore
	oauth2Configs map[string]*oauth2.Config
	tokenCounter  int64        // Add counter to ensure unique tokens
	eventEmitter  EventEmitter // For emitting events
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

// SetEventEmitter sets the event emitter for this service
func (s *Service) SetEventEmitter(emitter EventEmitter) {
	s.eventEmitter = emitter
}

// emitEvent is a helper method to emit events if an emitter is available
func (s *Service) emitEvent(ctx context.Context, eventType string, data interface{}, metadata map[string]interface{}) {
	if s.eventEmitter != nil {
		// Use the modular framework's NewCloudEvent to ensure proper CloudEvent format
		event := modular.NewCloudEvent(eventType, "auth-service", data, metadata)
		_ = s.eventEmitter.EmitEvent(ctx, event)
	}
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
		"exp":     now.Add(s.config.JWT.GetJWTExpiration()).Unix(),
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
		"exp":     now.Add(s.config.JWT.GetJWTRefreshExpiration()).Unix(),
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

	expiresAt := now.Add(s.config.JWT.GetJWTExpiration())

	tokenPair := &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.config.JWT.GetJWTExpiration().Seconds()),
		ExpiresAt:    expiresAt,
	}

	// Emit token generated event
	s.emitEvent(context.Background(), EventTypeTokenGenerated, map[string]interface{}{
		"userID":    userID,
		"expiresAt": expiresAt,
	}, map[string]interface{}{
		"counter": counter,
	})

	return tokenPair, nil
}

// ValidateToken validates a JWT token and returns the claims
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: %v", ErrUnexpectedSigningMethod, token.Header["alg"])
		}
		return []byte(s.config.JWT.Secret), nil
	})

	if err != nil {
		if strings.Contains(err.Error(), "token is expired") {
			// Emit token expired event
			tokenPrefix := tokenString
			if len(tokenString) > 20 {
				tokenPrefix = tokenString[:20] + "..."
			}
			s.emitEvent(context.Background(), EventTypeTokenExpired, map[string]interface{}{
				"tokenString": tokenPrefix, // Only log prefix for security
			}, nil)
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

	claimsResult := &Claims{
		UserID:      userID,
		Email:       email,
		Roles:       roles,
		Permissions: permissions,
		IssuedAt:    issuedAt,
		ExpiresAt:   expiresAt,
		Issuer:      issuer,
		Subject:     subject,
		Custom:      custom,
	}

	// Emit token validated event
	s.emitEvent(context.Background(), EventTypeTokenValidated, map[string]interface{}{
		"userID":    userID,
		"tokenType": claims["type"],
	}, nil)

	return claimsResult, nil
}

// RefreshToken creates a new token pair using a refresh token
func (s *Service) RefreshToken(refreshTokenString string) (*TokenPair, error) {
	token, err := jwt.Parse(refreshTokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: %v", ErrUnexpectedSigningMethod, token.Header["alg"])
		}
		return []byte(s.config.JWT.Secret), nil
	})

	if err != nil {
		if strings.Contains(err.Error(), "token is expired") {
			// Emit token expired event for refresh token
			tokenPrefix := refreshTokenString
			if len(refreshTokenString) > 10 {
				tokenPrefix = refreshTokenString[:10] + "..."
			}
			s.emitEvent(context.Background(), EventTypeTokenExpired, map[string]interface{}{
				"token":     tokenPrefix, // Only show first 10 chars for security
				"tokenType": "refresh",
			}, nil)
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

	newTokenPair, err := s.GenerateToken(userID, customClaims)
	if err != nil {
		return nil, err
	}

	// Emit token refreshed event
	s.emitEvent(context.Background(), EventTypeTokenRefreshed, map[string]interface{}{
		"userID":    userID,
		"expiresAt": newTokenPair.ExpiresAt,
	}, nil)

	return newTokenPair, nil
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
		ExpiresAt: now.Add(s.config.Session.GetSessionMaxAge()),
		Active:    true,
		Metadata:  metadata,
	}

	err = s.sessionStore.Store(context.Background(), session)
	if err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	// Emit session created event
	s.emitEvent(context.Background(), EventTypeSessionCreated, map[string]interface{}{
		"sessionID": sessionID,
		"userID":    userID,
		"expiresAt": session.ExpiresAt,
		"metadata":  metadata, // Include metadata in data instead of extensions
	}, nil)

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

		// Emit session expired event
		s.emitEvent(context.Background(), EventTypeSessionExpired, map[string]interface{}{
			"sessionID": sessionID,
			"userID":    session.UserID,
		}, nil)

		return nil, ErrSessionExpired
	}

	if !session.Active {
		return nil, ErrSessionNotFound
	}

	// Emit session accessed event
	s.emitEvent(context.Background(), EventTypeSessionAccessed, map[string]interface{}{
		"sessionID": sessionID,
		"userID":    session.UserID,
	}, nil)

	return session, nil
}

// DeleteSession removes a session
func (s *Service) DeleteSession(sessionID string) error {
	// Get session first to get userID for event
	session, err := s.sessionStore.Get(context.Background(), sessionID)
	var userID string
	if err == nil && session != nil {
		userID = session.UserID
	}

	err = s.sessionStore.Delete(context.Background(), sessionID)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}

	// Emit session destroyed event
	s.emitEvent(context.Background(), EventTypeSessionDestroyed, map[string]interface{}{
		"sessionID": sessionID,
		"userID":    userID,
	}, nil)

	return nil
}

// RefreshSession extends a session's expiration time
func (s *Service) RefreshSession(sessionID string) (*Session, error) {
	session, err := s.sessionStore.Get(context.Background(), sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting session for refresh: %w", err)
	}

	if !session.Active {
		return nil, ErrSessionExpired
	}

	// Capture the original expiration time before modifying
	originalExpiresAt := session.ExpiresAt

	// Add a small delay to ensure the new expiration time is detectably later
	time.Sleep(time.Millisecond)

	// Update expiration time to extend the session
	newExpiresAt := time.Now().Add(s.config.Session.GetSessionMaxAge())
	session.ExpiresAt = newExpiresAt

	// Ensure the new expiration is actually later than the original
	if !newExpiresAt.After(originalExpiresAt) {
		// If for some reason it's not later, add extra time
		session.ExpiresAt = originalExpiresAt.Add(time.Millisecond)
	}

	err = s.sessionStore.Store(context.Background(), session)
	if err != nil {
		return nil, fmt.Errorf("storing refreshed session: %w", err)
	}

	return session, nil
}

// GetOAuth2AuthURL returns the OAuth2 authorization URL for a provider
func (s *Service) GetOAuth2AuthURL(provider, state string) (string, error) {
	config, exists := s.oauth2Configs[provider]
	if !exists {
		return "", ErrProviderNotFound
	}

	authURL := config.AuthCodeURL(state)

	// Emit OAuth2 auth URL generated event
	s.emitEvent(context.Background(), EventTypeOAuth2AuthURL, map[string]interface{}{
		"provider": provider,
		"state":    state,
	}, nil)

	return authURL, nil
}

// ExchangeOAuth2Code exchanges an OAuth2 authorization code for user info
func (s *Service) ExchangeOAuth2Code(provider, code, state string) (*OAuth2Result, error) {
	config, exists := s.oauth2Configs[provider]
	if !exists {
		return nil, ErrProviderNotFound
	}

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOAuth2Failed, err)
	}

	// Get user info from provider
	userInfo, err := s.fetchOAuth2UserInfo(provider, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	result := &OAuth2Result{
		Provider:     provider,
		UserInfo:     userInfo,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.Expiry,
	}

	// Emit OAuth2 exchange successful event
	s.emitEvent(context.Background(), EventTypeOAuth2Exchange, map[string]interface{}{
		"provider": provider,
		"userInfo": userInfo,
	}, map[string]interface{}{
		"expiresAt": token.Expiry,
	})

	return result, nil
}

// fetchOAuth2UserInfo fetches user information from OAuth2 provider
func (s *Service) fetchOAuth2UserInfo(provider, accessToken string) (map[string]interface{}, error) {
	providerConfig, exists := s.config.OAuth2.Providers[provider]
	if !exists {
		return nil, ErrProviderNotFound
	}

	if providerConfig.UserInfoURL == "" {
		return nil, fmt.Errorf("%w: %s", ErrUserInfoURLNotConfigured, provider)
	}

	// Create HTTP request to fetch user info from OAuth2 provider
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", providerConfig.UserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating user info request: %w", err)
	}

	// Set authorization header with the access token
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	// Use a reusable HTTP client with appropriate timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching user info from provider %s: %w", provider, err)
	}
	defer resp.Body.Close()

	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user info request failed with status %d: %w", resp.StatusCode, &UserInfoError{StatusCode: resp.StatusCode, Body: string(body)})
	}

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading user info response: %w", err)
	}

	var userInfo map[string]interface{}
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("parsing user info JSON: %w", err)
	}

	// Add provider information to the user info
	userInfo["provider"] = provider

	return userInfo, nil
}

// generateRandomID generates a random hex-encoded ID
func generateRandomID(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
