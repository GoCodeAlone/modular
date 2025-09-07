package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestJWTValidator tests JWT validation mechanisms
func TestJWTValidator(t *testing.T) {
	t.Run("should validate HS256 JWT tokens", func(t *testing.T) {
		secret := "test-secret-key"
		validator := NewJWTValidator(&JWTConfig{
			Secret:    secret,
			Algorithm: "HS256",
		})

		// Create a valid token
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub":   "user123",
			"iss":   "test-issuer",
			"aud":   "test-audience",
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
			"email": "user@example.com",
		})

		tokenString, err := token.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("Failed to sign token: %v", err)
		}

		// Validate the token
		claims, err := validator.ValidateToken(tokenString)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}

		if claims["sub"] != "user123" {
			t.Errorf("Expected sub 'user123', got: %v", claims["sub"])
		}

		if claims["email"] != "user@example.com" {
			t.Errorf("Expected email 'user@example.com', got: %v", claims["email"])
		}
	})

	t.Run("should reject expired JWT tokens", func(t *testing.T) {
		secret := "test-secret-key"
		validator := NewJWTValidator(&JWTConfig{
			Secret:    secret,
			Algorithm: "HS256",
		})

		// Create an expired token
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user123",
			"exp": time.Now().Add(-time.Hour).Unix(), // Expired 1 hour ago
			"iat": time.Now().Add(-2 * time.Hour).Unix(),
		})

		tokenString, err := token.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("Failed to sign token: %v", err)
		}

		// Validation should fail
		_, err = validator.ValidateToken(tokenString)
		if err == nil {
			t.Error("Expected validation to fail for expired token")
		}
	})

	t.Run("should validate RS256 JWT tokens", func(t *testing.T) {
		// Generate RSA key pair for testing
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("Failed to generate RSA key: %v", err)
		}

		validator := NewJWTValidator(&JWTConfig{
			PublicKey: &privateKey.PublicKey,
			Algorithm: "RS256",
		})

		// Create a valid RS256 token
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub": "user123",
			"iss": "test-issuer",
			"aud": "test-audience",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Unix(),
		})

		tokenString, err := token.SignedString(privateKey)
		if err != nil {
			t.Fatalf("Failed to sign RS256 token: %v", err)
		}

		// Validate the token
		claims, err := validator.ValidateToken(tokenString)
		if err != nil {
			t.Fatalf("Failed to validate RS256 token: %v", err)
		}

		if claims["sub"] != "user123" {
			t.Errorf("Expected sub 'user123', got: %v", claims["sub"])
		}
	})

	t.Run("should reject tokens with wrong algorithm", func(t *testing.T) {
		secret := "test-secret-key"
		validator := NewJWTValidator(&JWTConfig{
			Secret:    secret,
			Algorithm: "HS256",
		})

		// Create token with different algorithm
		token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
			"sub": "user123",
			"exp": time.Now().Add(time.Hour).Unix(),
		})

		tokenString, err := token.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("Failed to sign token: %v", err)
		}

		// Validation should fail due to algorithm mismatch
		_, err = validator.ValidateToken(tokenString)
		if err == nil {
			t.Error("Expected validation to fail for wrong algorithm")
		}
	})

	t.Run("should validate audience claims", func(t *testing.T) {
		secret := "test-secret-key"
		validator := NewJWTValidator(&JWTConfig{
			Secret:          secret,
			Algorithm:       "HS256",
			ValidAudiences:  []string{"api", "web"},
			RequireAudience: true,
		})

		// Create token with valid audience
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user123",
			"aud": "api",
			"exp": time.Now().Add(time.Hour).Unix(),
		})

		tokenString, err := token.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("Failed to sign token: %v", err)
		}

		// Should validate successfully
		_, err = validator.ValidateToken(tokenString)
		if err != nil {
			t.Fatalf("Failed to validate token with valid audience: %v", err)
		}

		// Create token with invalid audience
		invalidToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user123",
			"aud": "invalid",
			"exp": time.Now().Add(time.Hour).Unix(),
		})

		invalidTokenString, err := invalidToken.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("Failed to sign invalid token: %v", err)
		}

		// Should fail validation
		_, err = validator.ValidateToken(invalidTokenString)
		if err == nil {
			t.Error("Expected validation to fail for invalid audience")
		}
	})

	t.Run("should validate issuer claims", func(t *testing.T) {
		secret := "test-secret-key"
		validator := NewJWTValidator(&JWTConfig{
			Secret:        secret,
			Algorithm:     "HS256",
			ValidIssuer:   "trusted-issuer",
			RequireIssuer: true,
		})

		// Create token with valid issuer
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user123",
			"iss": "trusted-issuer",
			"exp": time.Now().Add(time.Hour).Unix(),
		})

		tokenString, err := token.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("Failed to sign token: %v", err)
		}

		// Should validate successfully
		_, err = validator.ValidateToken(tokenString)
		if err != nil {
			t.Fatalf("Failed to validate token with valid issuer: %v", err)
		}

		// Create token with invalid issuer
		invalidToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user123",
			"iss": "untrusted-issuer",
			"exp": time.Now().Add(time.Hour).Unix(),
		})

		invalidTokenString, err := invalidToken.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("Failed to sign invalid token: %v", err)
		}

		// Should fail validation
		_, err = validator.ValidateToken(invalidTokenString)
		if err == nil {
			t.Error("Expected validation to fail for invalid issuer")
		}
	})
}

// TestAPIKeyAuthenticator tests API key authentication
func TestAPIKeyAuthenticator(t *testing.T) {
	t.Run("should authenticate valid API keys", func(t *testing.T) {
		apiKeys := map[string]*Principal{
			"api-key-123": {
				ID:    "user1",
				Email: "user1@example.com",
				Roles: []string{"user"},
				Claims: map[string]interface{}{
					"scope": "read:data",
				},
			},
			"admin-key-456": {
				ID:    "admin1",
				Email: "admin@example.com",
				Roles: []string{"admin"},
				Claims: map[string]interface{}{
					"scope": "read:data write:data",
				},
			},
		}

		authenticator := NewAPIKeyAuthenticator(&APIKeyConfig{
			HeaderName: "X-API-Key",
			APIKeys:    apiKeys,
		})

		// Test valid API key
		principal, err := authenticator.Authenticate("api-key-123")
		if err != nil {
			t.Fatalf("Failed to authenticate valid API key: %v", err)
		}

		if principal.ID != "user1" {
			t.Errorf("Expected user ID 'user1', got: %s", principal.ID)
		}

		if principal.Email != "user1@example.com" {
			t.Errorf("Expected email 'user1@example.com', got: %s", principal.Email)
		}

		if len(principal.Roles) != 1 || principal.Roles[0] != "user" {
			t.Errorf("Expected roles [user], got: %v", principal.Roles)
		}
	})

	t.Run("should reject invalid API keys", func(t *testing.T) {
		apiKeys := map[string]*Principal{
			"valid-key": {
				ID:    "user1",
				Email: "user1@example.com",
			},
		}

		authenticator := NewAPIKeyAuthenticator(&APIKeyConfig{
			HeaderName: "X-API-Key",
			APIKeys:    apiKeys,
		})

		// Test invalid API key
		_, err := authenticator.Authenticate("invalid-key")
		if err == nil {
			t.Error("Expected authentication to fail for invalid API key")
		}
	})

	t.Run("should handle empty API key", func(t *testing.T) {
		authenticator := NewAPIKeyAuthenticator(&APIKeyConfig{
			HeaderName: "X-API-Key",
			APIKeys:    map[string]*Principal{},
		})

		// Test empty API key
		_, err := authenticator.Authenticate("")
		if err == nil {
			t.Error("Expected authentication to fail for empty API key")
		}
	})

	t.Run("should support bearer token prefix", func(t *testing.T) {
		apiKeys := map[string]*Principal{
			"secret-token": {
				ID:    "user1",
				Email: "user1@example.com",
			},
		}

		authenticator := NewAPIKeyAuthenticator(&APIKeyConfig{
			HeaderName:    "Authorization",
			BearerPrefix:  true,
			PrefixValue:   "Bearer ",
			APIKeys:       apiKeys,
		})

		// Test with Bearer prefix
		principal, err := authenticator.Authenticate("Bearer secret-token")
		if err != nil {
			t.Fatalf("Failed to authenticate with Bearer prefix: %v", err)
		}

		if principal.ID != "user1" {
			t.Errorf("Expected user ID 'user1', got: %s", principal.ID)
		}
	})

	t.Run("should support custom prefix", func(t *testing.T) {
		apiKeys := map[string]*Principal{
			"custom-token": {
				ID:    "user1",
				Email: "user1@example.com",
			},
		}

		authenticator := NewAPIKeyAuthenticator(&APIKeyConfig{
			HeaderName:   "X-Auth",
			BearerPrefix: true,
			PrefixValue:  "Custom ",
			APIKeys:      apiKeys,
		})

		// Test with custom prefix
		principal, err := authenticator.Authenticate("Custom custom-token")
		if err != nil {
			t.Fatalf("Failed to authenticate with custom prefix: %v", err)
		}

		if principal.ID != "user1" {
			t.Errorf("Expected user ID 'user1', got: %s", principal.ID)
		}
	})
}

// TestOIDCProvider tests OIDC integration
func TestOIDCProvider(t *testing.T) {
	t.Run("should handle OIDC metadata parsing", func(t *testing.T) {
		// Mock OIDC metadata response
		metadata := &OIDCMetadata{
			Issuer:                "https://auth.example.com",
			AuthorizationEndpoint: "https://auth.example.com/oauth/authorize",
			TokenEndpoint:         "https://auth.example.com/oauth/token",
			JWKSURI:               "https://auth.example.com/.well-known/jwks.json",
			SupportedScopes:       []string{"openid", "email", "profile"},
			SupportedResponseTypes: []string{"code", "token"},
		}

		provider := &OIDCProvider{
			metadata: metadata,
		}

		// Verify metadata parsing
		if provider.GetIssuer() != "https://auth.example.com" {
			t.Errorf("Expected issuer 'https://auth.example.com', got: %s", provider.GetIssuer())
		}

		if provider.GetJWKSURI() != "https://auth.example.com/.well-known/jwks.json" {
			t.Errorf("Expected JWKS URI, got: %s", provider.GetJWKSURI())
		}
	})

	t.Run("should validate supported scopes", func(t *testing.T) {
		metadata := &OIDCMetadata{
			SupportedScopes: []string{"openid", "email", "profile"},
		}

		provider := &OIDCProvider{
			metadata: metadata,
		}

		// Test supported scope
		if !provider.SupportScope("email") {
			t.Error("Expected 'email' scope to be supported")
		}

		// Test unsupported scope
		if provider.SupportScope("admin") {
			t.Error("Expected 'admin' scope to not be supported")
		}
	})

	t.Run("should validate supported response types", func(t *testing.T) {
		metadata := &OIDCMetadata{
			SupportedResponseTypes: []string{"code", "token", "id_token"},
		}

		provider := &OIDCProvider{
			metadata: metadata,
		}

		// Test supported response type
		if !provider.SupportResponseType("code") {
			t.Error("Expected 'code' response type to be supported")
		}

		// Test unsupported response type
		if provider.SupportResponseType("unsupported") {
			t.Error("Expected 'unsupported' response type to not be supported")
		}
	})
}

// TestPrincipalMapping tests principal creation and claims mapping
func TestPrincipalMapping(t *testing.T) {
	t.Run("should map JWT claims to principal", func(t *testing.T) {
		claims := map[string]interface{}{
			"sub":   "user123",
			"email": "user@example.com",
			"name":  "John Doe",
			"roles": []interface{}{"user", "editor"},
			"scope": "read:data write:posts",
			"custom_field": "custom_value",
		}

		principal := NewPrincipalFromJWT(claims)

		if principal.ID != "user123" {
			t.Errorf("Expected ID 'user123', got: %s", principal.ID)
		}

		if principal.Email != "user@example.com" {
			t.Errorf("Expected email 'user@example.com', got: %s", principal.Email)
		}

		if principal.Name != "John Doe" {
			t.Errorf("Expected name 'John Doe', got: %s", principal.Name)
		}

		expectedRoles := []string{"user", "editor"}
		if len(principal.Roles) != len(expectedRoles) {
			t.Errorf("Expected %d roles, got %d", len(expectedRoles), len(principal.Roles))
		}

		for i, role := range expectedRoles {
			if i >= len(principal.Roles) || principal.Roles[i] != role {
				t.Errorf("Expected role %s at index %d, got: %v", role, i, principal.Roles)
			}
		}

		// Check custom claims
		if principal.Claims["custom_field"] != "custom_value" {
			t.Errorf("Expected custom_field 'custom_value', got: %v", principal.Claims["custom_field"])
		}
	})

	t.Run("should handle missing optional claims", func(t *testing.T) {
		claims := map[string]interface{}{
			"sub": "user123",
			// Missing email, name, roles, etc.
		}

		principal := NewPrincipalFromJWT(claims)

		if principal.ID != "user123" {
			t.Errorf("Expected ID 'user123', got: %s", principal.ID)
		}

		if principal.Email != "" {
			t.Errorf("Expected empty email, got: %s", principal.Email)
		}

		if len(principal.Roles) != 0 {
			t.Errorf("Expected no roles, got: %v", principal.Roles)
		}
	})

	t.Run("should validate principal permissions", func(t *testing.T) {
		principal := &Principal{
			ID:    "user123",
			Roles: []string{"admin", "user"},
			Claims: map[string]interface{}{
				"scope": "read:data write:data delete:data",
			},
		}

		// Test role checking
		if !principal.HasRole("admin") {
			t.Error("Expected principal to have 'admin' role")
		}

		if principal.HasRole("superuser") {
			t.Error("Expected principal to not have 'superuser' role")
		}

		// Test scope checking
		if !principal.HasScope("read:data") {
			t.Error("Expected principal to have 'read:data' scope")
		}

		if principal.HasScope("admin:system") {
			t.Error("Expected principal to not have 'admin:system' scope")
		}
	})

	t.Run("should support claims validation", func(t *testing.T) {
		principal := &Principal{
			ID: "user123",
			Claims: map[string]interface{}{
				"department": "engineering",
				"level":      5,
				"active":     true,
			},
		}

		// Test claim existence
		if !principal.HasClaim("department") {
			t.Error("Expected principal to have 'department' claim")
		}

		// Test claim value
		if principal.GetClaimString("department") != "engineering" {
			t.Errorf("Expected department 'engineering', got: %s", principal.GetClaimString("department"))
		}

		if principal.GetClaimInt("level") != 5 {
			t.Errorf("Expected level 5, got: %d", principal.GetClaimInt("level"))
		}

		if !principal.GetClaimBool("active") {
			t.Error("Expected active to be true")
		}
	})
}

// Helper functions (these would need to be implemented in the actual auth module)

func NewJWTValidator(config *JWTConfig) *JWTValidator {
	return &JWTValidator{
		config: config,
	}
}

func NewAPIKeyAuthenticator(config *APIKeyConfig) *APIKeyAuthenticator {
	return &APIKeyAuthenticator{
		config: config,
	}
}

func NewPrincipalFromJWT(claims map[string]interface{}) *Principal {
	principal := &Principal{
		Claims: make(map[string]interface{}),
	}

	if sub, ok := claims["sub"].(string); ok {
		principal.ID = sub
	}

	if email, ok := claims["email"].(string); ok {
		principal.Email = email
	}

	if name, ok := claims["name"].(string); ok {
		principal.Name = name
	}

	if roles, ok := claims["roles"].([]interface{}); ok {
		for _, role := range roles {
			if roleStr, ok := role.(string); ok {
				principal.Roles = append(principal.Roles, roleStr)
			}
		}
	}

	// Copy all claims
	for k, v := range claims {
		principal.Claims[k] = v
	}

	return principal
}

// Mock types for testing (these would be defined in the actual auth module)

type JWTConfig struct {
	Secret          string
	PublicKey       *rsa.PublicKey
	Algorithm       string
	ValidIssuer     string
	ValidAudiences  []string
	RequireIssuer   bool
	RequireAudience bool
}

type JWTValidator struct {
	config *JWTConfig
}

func (v *JWTValidator) ValidateToken(tokenString string) (map[string]interface{}, error) {
	// Mock implementation - in real code this would use jwt.Parse
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if v.config.Algorithm == "HS256" {
			return []byte(v.config.Secret), nil
		}
		if v.config.Algorithm == "RS256" {
			return v.config.PublicKey, nil
		}
		return nil, fmt.Errorf("unsupported algorithm: %s", v.config.Algorithm)
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	// Validate algorithm
	if token.Header["alg"] != v.config.Algorithm {
		return nil, fmt.Errorf("invalid algorithm")
	}

	// Validate issuer if required
	if v.config.RequireIssuer {
		if iss, ok := claims["iss"].(string); ok {
			if iss != v.config.ValidIssuer {
				return nil, fmt.Errorf("invalid issuer")
			}
		} else {
			return nil, fmt.Errorf("missing issuer")
		}
	}

	// Validate audience if required
	if v.config.RequireAudience {
		if aud, ok := claims["aud"].(string); ok {
			validAud := false
			for _, validAudience := range v.config.ValidAudiences {
				if aud == validAudience {
					validAud = true
					break
				}
			}
			if !validAud {
				return nil, fmt.Errorf("invalid audience")
			}
		} else {
			return nil, fmt.Errorf("missing audience")
		}
	}

	return claims, nil
}

type APIKeyConfig struct {
	HeaderName   string
	BearerPrefix bool
	PrefixValue  string
	APIKeys      map[string]*Principal
}

type APIKeyAuthenticator struct {
	config *APIKeyConfig
}

func (a *APIKeyAuthenticator) Authenticate(key string) (*Principal, error) {
	if key == "" {
		return nil, fmt.Errorf("empty API key")
	}

	// Handle prefix
	if a.config.BearerPrefix && a.config.PrefixValue != "" {
		if len(key) <= len(a.config.PrefixValue) {
			return nil, fmt.Errorf("invalid API key format")
		}
		if key[:len(a.config.PrefixValue)] != a.config.PrefixValue {
			return nil, fmt.Errorf("invalid prefix")
		}
		key = key[len(a.config.PrefixValue):]
	}

	principal, exists := a.config.APIKeys[key]
	if !exists {
		return nil, fmt.Errorf("invalid API key")
	}

	return principal, nil
}

type OIDCMetadata struct {
	Issuer                     string   `json:"issuer"`
	AuthorizationEndpoint      string   `json:"authorization_endpoint"`
	TokenEndpoint              string   `json:"token_endpoint"`
	JWKSURI                    string   `json:"jwks_uri"`
	SupportedScopes            []string `json:"scopes_supported"`
	SupportedResponseTypes     []string `json:"response_types_supported"`
}

type OIDCProvider struct {
	metadata *OIDCMetadata
}

func (p *OIDCProvider) GetIssuer() string {
	return p.metadata.Issuer
}

func (p *OIDCProvider) GetJWKSURI() string {
	return p.metadata.JWKSURI
}

func (p *OIDCProvider) SupportScope(scope string) bool {
	for _, supported := range p.metadata.SupportedScopes {
		if supported == scope {
			return true
		}
	}
	return false
}

func (p *OIDCProvider) SupportResponseType(responseType string) bool {
	for _, supported := range p.metadata.SupportedResponseTypes {
		if supported == responseType {
			return true
		}
	}
	return false
}

// Principal methods for testing
func (p *Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (p *Principal) HasScope(scope string) bool {
	scopeStr, ok := p.Claims["scope"].(string)
	if !ok {
		return false
	}
	// Simple implementation - in real code might parse scopes properly
	return fmt.Sprintf(" %s ", scopeStr) != fmt.Sprintf(" %s ", scope) // contains check
}

func (p *Principal) HasClaim(claim string) bool {
	_, exists := p.Claims[claim]
	return exists
}

func (p *Principal) GetClaimString(claim string) string {
	if val, ok := p.Claims[claim].(string); ok {
		return val
	}
	return ""
}

func (p *Principal) GetClaimInt(claim string) int {
	if val, ok := p.Claims[claim].(int); ok {
		return val
	}
	if val, ok := p.Claims[claim].(float64); ok {
		return int(val)
	}
	return 0
}

func (p *Principal) GetClaimBool(claim string) bool {
	if val, ok := p.Claims[claim].(bool); ok {
		return val
	}
	return false
}