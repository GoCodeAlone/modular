package auth

import (
	"errors"
	"fmt"
	"sync"
)

// Static errors for OIDC provider registry operations (avoid dynamic errors per err113).
// Reuse existing ErrProviderNotFound from errors.go for consistency with rest of auth package.
var (
	ErrProviderNameEmpty      = errors.New("oidc: provider name cannot be empty")
	ErrProviderNil            = errors.New("oidc: provider cannot be nil")
	ErrTokenEmpty             = errors.New("oidc: token cannot be empty")
	ErrProviderMetadataAbsent = errors.New("oidc: provider metadata not available")
	ErrAuthorizationCodeEmpty = errors.New("oidc: authorization code cannot be empty")
)

// OIDCProvider defines the interface for OIDC provider implementations
type OIDCProvider interface {
	GetProviderName() string
	GetClientID() string
	GetIssuerURL() string
	ValidateToken(token string) (interface{}, error)
	GetUserInfo(token string) (interface{}, error)
	GetAuthURL(state string, scopes []string) (string, error)
	ExchangeCode(code string, state string) (interface{}, error)
}

// OIDCProviderRegistry manages multiple OIDC provider implementations
type OIDCProviderRegistry interface {
	RegisterProvider(name string, provider OIDCProvider) error
	GetProvider(name string) (OIDCProvider, error)
	ListProviders() ([]string, error)
	RemoveProvider(name string) error
}

// ProviderMetadata contains OIDC provider discovery information
type ProviderMetadata struct {
	Issuer                 string   `json:"issuer"`
	AuthorizationEndpoint  string   `json:"authorization_endpoint"`
	TokenEndpoint          string   `json:"token_endpoint"`
	UserInfoEndpoint       string   `json:"userinfo_endpoint"`
	JWKsURI                string   `json:"jwks_uri"`
	ScopesSupported        []string `json:"scopes_supported"`
	ResponseTypesSupported []string `json:"response_types_supported"`
}

// TokenSet represents a set of tokens returned from an OIDC provider
type TokenSet struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// defaultOIDCProviderRegistry is the default implementation of OIDCProviderRegistry
type defaultOIDCProviderRegistry struct {
	providers map[string]OIDCProvider
	mutex     sync.RWMutex
}

// NewOIDCProviderRegistry creates a new OIDC provider registry
func NewOIDCProviderRegistry() OIDCProviderRegistry {
	return &defaultOIDCProviderRegistry{
		providers: make(map[string]OIDCProvider),
	}
}

// RegisterProvider registers a new OIDC provider
func (r *defaultOIDCProviderRegistry) RegisterProvider(name string, provider OIDCProvider) error {
	if name == "" {
		return ErrProviderNameEmpty
	}
	if provider == nil {
		return ErrProviderNil
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.providers[name] = provider
	return nil
}

// GetProvider retrieves an OIDC provider by name
func (r *defaultOIDCProviderRegistry) GetProvider(name string) (OIDCProvider, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}

	return provider, nil
}

// ListProviders returns a list of all registered provider names
func (r *defaultOIDCProviderRegistry) ListProviders() ([]string, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names, nil
}

// RemoveProvider removes an OIDC provider from the registry
func (r *defaultOIDCProviderRegistry) RemoveProvider(name string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.providers[name]; !exists {
		return fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}

	delete(r.providers, name)
	return nil
}

// BasicOIDCProvider provides a basic implementation of OIDCProvider
type BasicOIDCProvider struct {
	providerName string
	clientID     string
	issuerURL    string
	metadata     *ProviderMetadata
}

// NewBasicOIDCProvider creates a new basic OIDC provider
func NewBasicOIDCProvider(name, clientID, issuerURL string) *BasicOIDCProvider {
	return &BasicOIDCProvider{
		providerName: name,
		clientID:     clientID,
		issuerURL:    issuerURL,
	}
}

// GetProviderName returns the provider name
func (p *BasicOIDCProvider) GetProviderName() string {
	return p.providerName
}

// GetClientID returns the client ID
func (p *BasicOIDCProvider) GetClientID() string {
	return p.clientID
}

// GetIssuerURL returns the issuer URL
func (p *BasicOIDCProvider) GetIssuerURL() string {
	return p.issuerURL
}

// ValidateToken validates an OIDC token
func (p *BasicOIDCProvider) ValidateToken(token string) (interface{}, error) {
	// Basic implementation - real implementation would validate JWT signature and claims
	if token == "" {
		return nil, ErrTokenEmpty
	}

	return map[string]interface{}{
		"valid": true,
		"sub":   "user123",
		"iss":   p.issuerURL,
	}, nil
}

// GetUserInfo retrieves user information using an access token
func (p *BasicOIDCProvider) GetUserInfo(token string) (interface{}, error) {
	// Basic implementation - real implementation would make HTTP request to userinfo endpoint
	if token == "" {
		return nil, ErrTokenEmpty
	}

	return map[string]interface{}{
		"sub":   "user123",
		"name":  "Test User",
		"email": "test@example.com",
	}, nil
}

// GetAuthURL generates an authorization URL for the provider
func (p *BasicOIDCProvider) GetAuthURL(state string, scopes []string) (string, error) {
	if p.metadata == nil {
		return "", ErrProviderMetadataAbsent
	}

	// Basic implementation - real implementation would build proper OAuth2/OIDC auth URL
	authURL := fmt.Sprintf("%s?client_id=%s&response_type=code&state=%s",
		p.metadata.AuthorizationEndpoint, p.clientID, state)

	if len(scopes) > 0 {
		// Add scopes to URL
		authURL += "&scope=openid"
		for _, scope := range scopes {
			authURL += "+" + scope
		}
	}

	return authURL, nil
}

// ExchangeCode exchanges an authorization code for tokens
func (p *BasicOIDCProvider) ExchangeCode(code string, state string) (interface{}, error) {
	if code == "" {
		return nil, ErrAuthorizationCodeEmpty
	}

	// Basic implementation - real implementation would make HTTP request to token endpoint
	return &TokenSet{
		AccessToken:  "access_token_" + code,
		RefreshToken: "refresh_token_" + code,
		IDToken:      "id_token_" + code,
		TokenType:    "Bearer",
		ExpiresIn:    3600,
	}, nil
}

// Discover performs OIDC discovery for the provider
func (p *BasicOIDCProvider) Discover() (*ProviderMetadata, error) {
	// Basic implementation - real implementation would fetch .well-known/openid_configuration
	p.metadata = &ProviderMetadata{
		Issuer:                 p.issuerURL,
		AuthorizationEndpoint:  p.issuerURL + "/auth",
		TokenEndpoint:          p.issuerURL + "/token",
		UserInfoEndpoint:       p.issuerURL + "/userinfo",
		JWKsURI:                p.issuerURL + "/jwks",
		ScopesSupported:        []string{"openid", "profile", "email"},
		ResponseTypesSupported: []string{"code", "id_token", "code id_token"},
	}

	return p.metadata, nil
}

// SetMetadata sets the provider metadata (for testing or manual configuration)
func (p *BasicOIDCProvider) SetMetadata(metadata *ProviderMetadata) {
	p.metadata = metadata
}
