// Package auth provides authentication and authorization services
package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// Static errors for OIDC
var (
	ErrMetadataFetchFailed     = errors.New("failed to fetch OIDC metadata")
	ErrJWKSFetchFailed         = errors.New("failed to fetch JWKS")
	ErrKeyNotFound             = errors.New("signing key not found in JWKS")
	ErrInvalidKeyFormat        = errors.New("invalid key format in JWKS")
	ErrOIDCConfigurationFailed = errors.New("OIDC configuration failed")
)

// OIDCMetadata represents OpenID Connect discovery metadata
type OIDCMetadata struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	UserInfoEndpoint      string   `json:"userinfo_endpoint"`
	JWKSUri               string   `json:"jwks_uri"`
	ScopesSupported       []string `json:"scopes_supported"`
	ResponseTypesSupported []string `json:"response_types_supported"`
	IdTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	SubjectTypesSupported []string `json:"subject_types_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

// JWKSResponse represents a JSON Web Key Set response
type JWKSResponse struct {
	Keys []JSONWebKey `json:"keys"`
}

// JSONWebKey represents a single key in a JWKS
type JSONWebKey struct {
	KeyType   string `json:"kty"`
	Use       string `json:"use,omitempty"`
	KeyID     string `json:"kid,omitempty"`
	Algorithm string `json:"alg,omitempty"`
	
	// RSA key parameters
	Modulus  string `json:"n,omitempty"`
	Exponent string `json:"e,omitempty"`
}

// OIDCProvider manages OIDC metadata and JWKS
type OIDCProvider struct {
	mu                sync.RWMutex
	issuerURL         string
	metadata          *OIDCMetadata
	jwks              *JWKSResponse
	signingKeys       map[string]*rsa.PublicKey
	lastMetadataFetch time.Time
	lastJWKSFetch     time.Time
	refreshInterval   time.Duration
	httpClient        *http.Client
}

// OIDCConfig configures the OIDC provider
type OIDCConfig struct {
	IssuerURL       string        `json:"issuer_url"`
	RefreshInterval time.Duration `json:"refresh_interval"`
	HTTPTimeout     time.Duration `json:"http_timeout"`
}

// NewOIDCProvider creates a new OIDC provider
func NewOIDCProvider(config *OIDCConfig) *OIDCProvider {
	refreshInterval := config.RefreshInterval
	if refreshInterval == 0 {
		refreshInterval = 1 * time.Hour // Default refresh interval
	}

	httpTimeout := config.HTTPTimeout
	if httpTimeout == 0 {
		httpTimeout = 30 * time.Second // Default HTTP timeout
	}

	return &OIDCProvider{
		issuerURL:       config.IssuerURL,
		refreshInterval: refreshInterval,
		signingKeys:     make(map[string]*rsa.PublicKey),
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

// FetchMetadata fetches OIDC discovery metadata from the issuer
func (p *OIDCProvider) FetchMetadata(ctx context.Context) error {
	metadataURL := p.issuerURL + "/.well-known/openid_configuration"
	
	req, err := http.NewRequestWithContext(ctx, "GET", metadataURL, nil)
	if err != nil {
		return fmt.Errorf("%w: failed to create request: %v", ErrMetadataFetchFailed, err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: HTTP request failed: %v", ErrMetadataFetchFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: HTTP %d", ErrMetadataFetchFailed, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%w: failed to read response: %v", ErrMetadataFetchFailed, err)
	}

	var metadata OIDCMetadata
	err = json.Unmarshal(body, &metadata)
	if err != nil {
		return fmt.Errorf("%w: failed to parse metadata: %v", ErrMetadataFetchFailed, err)
	}

	p.mu.Lock()
	p.metadata = &metadata
	p.lastMetadataFetch = time.Now()
	p.mu.Unlock()

	return nil
}

// FetchJWKS fetches the JSON Web Key Set from the OIDC provider
func (p *OIDCProvider) FetchJWKS(ctx context.Context) error {
	p.mu.RLock()
	metadata := p.metadata
	p.mu.RUnlock()

	if metadata == nil || metadata.JWKSUri == "" {
		return fmt.Errorf("%w: no JWKS URI available", ErrJWKSFetchFailed)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", metadata.JWKSUri, nil)
	if err != nil {
		return fmt.Errorf("%w: failed to create request: %v", ErrJWKSFetchFailed, err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: HTTP request failed: %v", ErrJWKSFetchFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: HTTP %d", ErrJWKSFetchFailed, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%w: failed to read response: %v", ErrJWKSFetchFailed, err)
	}

	var jwks JWKSResponse
	err = json.Unmarshal(body, &jwks)
	if err != nil {
		return fmt.Errorf("%w: failed to parse JWKS: %v", ErrJWKSFetchFailed, err)
	}

	// Convert JWK to RSA public keys
	signingKeys := make(map[string]*rsa.PublicKey)
	for _, key := range jwks.Keys {
		if key.KeyType == "RSA" && (key.Use == "sig" || key.Use == "") {
			rsaKey, err := p.jwkToRSAPublicKey(&key)
			if err != nil {
				// Log error but continue with other keys
				continue
			}
			signingKeys[key.KeyID] = rsaKey
		}
	}

	p.mu.Lock()
	p.jwks = &jwks
	p.signingKeys = signingKeys
	p.lastJWKSFetch = time.Now()
	p.mu.Unlock()

	return nil
}

// GetSigningKey returns the RSA public key for the given key ID
func (p *OIDCProvider) GetSigningKey(keyID string) (*rsa.PublicKey, error) {
	p.mu.RLock()
	key, exists := p.signingKeys[keyID]
	lastFetch := p.lastJWKSFetch
	p.mu.RUnlock()

	if !exists {
		// Try refreshing JWKS if it's been a while
		if time.Since(lastFetch) > p.refreshInterval {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			
			_ = p.FetchJWKS(ctx) // Ignore error, try with existing keys
			
			p.mu.RLock()
			key, exists = p.signingKeys[keyID]
			p.mu.RUnlock()
		}
		
		if !exists {
			return nil, fmt.Errorf("%w: key ID %s", ErrKeyNotFound, keyID)
		}
	}

	return key, nil
}

// RefreshMetadata refreshes both metadata and JWKS if needed
func (p *OIDCProvider) RefreshMetadata(ctx context.Context) error {
	p.mu.RLock()
	lastMetadataFetch := p.lastMetadataFetch
	lastJWKSFetch := p.lastJWKSFetch
	p.mu.RUnlock()

	// Refresh metadata if it's stale
	if time.Since(lastMetadataFetch) > p.refreshInterval {
		err := p.FetchMetadata(ctx)
		if err != nil {
			return err
		}
	}

	// Refresh JWKS if it's stale
	if time.Since(lastJWKSFetch) > p.refreshInterval {
		err := p.FetchJWKS(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetMetadata returns the current OIDC metadata
func (p *OIDCProvider) GetMetadata() *OIDCMetadata {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metadata
}

// IsReady returns true if metadata and JWKS have been fetched
func (p *OIDCProvider) IsReady() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metadata != nil && p.jwks != nil
}

// jwkToRSAPublicKey converts a JWK to an RSA public key
func (p *OIDCProvider) jwkToRSAPublicKey(jwk *JSONWebKey) (*rsa.PublicKey, error) {
	if jwk.KeyType != "RSA" {
		return nil, ErrInvalidKeyFormat
	}

	// Decode modulus
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.Modulus)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode modulus: %v", ErrInvalidKeyFormat, err)
	}

	// Decode exponent
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.Exponent)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode exponent: %v", ErrInvalidKeyFormat, err)
	}

	// Convert to big integers
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	// Create RSA public key
	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}

// StartAutoRefresh starts automatic background refresh of metadata and JWKS
func (p *OIDCProvider) StartAutoRefresh(ctx context.Context) {
	ticker := time.NewTicker(p.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = p.RefreshMetadata(ctx) // Log errors in real implementation
		}
	}
}