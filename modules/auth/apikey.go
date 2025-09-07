// Package auth provides authentication and authorization services
package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Static errors for API key authentication
var (
	ErrAPIKeyNotFound       = errors.New("API key not found")
	ErrAPIKeyInvalid        = errors.New("invalid API key")
	ErrAPIKeyExpired        = errors.New("API key has expired")
	ErrAPIKeyRevoked        = errors.New("API key has been revoked")
	ErrAPIKeyMissingHeader  = errors.New("API key header missing")
	ErrAPIKeyInvalidFormat  = errors.New("API key format invalid")
	ErrAPIKeyStoreNotFound  = errors.New("API key store not configured")
)

// APIKeyInfo represents metadata about an API key
type APIKeyInfo struct {
	KeyID       string            `json:"key_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time        `json:"last_used_at,omitempty"`
	IsRevoked   bool              `json:"is_revoked"`
	Scopes      []string          `json:"scopes,omitempty"`
	RateLimits  map[string]int    `json:"rate_limits,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	
	// For lookup optimization
	HashedKey string `json:"-"` // Internal field - never serialized
}

// APIKeyStore defines the interface for API key storage and retrieval
type APIKeyStore interface {
	// GetAPIKeyInfo retrieves API key information by key value
	GetAPIKeyInfo(ctx context.Context, keyValue string) (*APIKeyInfo, error)
	
	// GetAPIKeyByID retrieves API key information by key ID
	GetAPIKeyByID(ctx context.Context, keyID string) (*APIKeyInfo, error)
	
	// UpdateLastUsed updates the last used timestamp for an API key
	UpdateLastUsed(ctx context.Context, keyID string, timestamp time.Time) error
	
	// IsRevoked checks if an API key has been revoked
	IsRevoked(ctx context.Context, keyID string) (bool, error)
}

// APIKeyAuthenticator handles API key based authentication
type APIKeyAuthenticator struct {
	mu          sync.RWMutex
	store       APIKeyStore
	headerName  string
	prefix      string
	required    bool
	trackUsage  bool
}

// APIKeyConfig configures the API key authenticator
type APIKeyConfig struct {
	HeaderName string       `json:"header_name"`     // e.g., "X-API-Key", "Authorization"
	Prefix     string       `json:"prefix"`          // e.g., "Bearer ", "ApiKey "
	Required   bool         `json:"required"`        // Whether API key is required
	TrackUsage bool         `json:"track_usage"`     // Whether to track usage statistics
	Store      APIKeyStore  `json:"-"`               // API key store implementation
}

// NewAPIKeyAuthenticator creates a new API key authenticator
func NewAPIKeyAuthenticator(config *APIKeyConfig) *APIKeyAuthenticator {
	headerName := config.HeaderName
	if headerName == "" {
		headerName = "X-API-Key" // Default header name
	}

	return &APIKeyAuthenticator{
		store:      config.Store,
		headerName: headerName,
		prefix:     config.Prefix,
		required:   config.Required,
		trackUsage: config.TrackUsage,
	}
}

// AuthenticateRequest authenticates an HTTP request using API key
func (a *APIKeyAuthenticator) AuthenticateRequest(r *http.Request) (*APIKeyInfo, error) {
	// Extract API key from request header
	apiKey, err := a.extractAPIKey(r)
	if err != nil {
		if a.required {
			return nil, err
		}
		// API key not required, return nil (anonymous access)
		return nil, nil
	}

	return a.ValidateAPIKey(r.Context(), apiKey)
}

// ValidateAPIKey validates an API key and returns its information
func (a *APIKeyAuthenticator) ValidateAPIKey(ctx context.Context, keyValue string) (*APIKeyInfo, error) {
	if a.store == nil {
		return nil, ErrAPIKeyStoreNotFound
	}

	// Get API key information from store
	keyInfo, err := a.store.GetAPIKeyInfo(ctx, keyValue)
	if err != nil {
		if errors.Is(err, ErrAPIKeyNotFound) {
			return nil, ErrAPIKeyInvalid
		}
		return nil, fmt.Errorf("failed to retrieve API key: %w", err)
	}

	// Check if key is revoked
	if keyInfo.IsRevoked {
		return nil, ErrAPIKeyRevoked
	}

	// Check revocation status from store as well (double-check)
	revoked, err := a.store.IsRevoked(ctx, keyInfo.KeyID)
	if err != nil {
		// Log error but continue with stored revocation status
	} else if revoked {
		return nil, ErrAPIKeyRevoked
	}

	// Check expiration
	if keyInfo.ExpiresAt != nil && time.Now().After(*keyInfo.ExpiresAt) {
		return nil, ErrAPIKeyExpired
	}

	// Update last used timestamp if tracking is enabled
	if a.trackUsage {
		now := time.Now()
		err = a.store.UpdateLastUsed(ctx, keyInfo.KeyID, now)
		if err != nil {
			// Log error but don't fail authentication
		} else {
			keyInfo.LastUsedAt = &now
		}
	}

	return keyInfo, nil
}

// extractAPIKey extracts the API key from the HTTP request
func (a *APIKeyAuthenticator) extractAPIKey(r *http.Request) (string, error) {
	headerValue := r.Header.Get(a.headerName)
	if headerValue == "" {
		return "", ErrAPIKeyMissingHeader
	}

	// Remove prefix if configured
	if a.prefix != "" {
		if !strings.HasPrefix(headerValue, a.prefix) {
			return "", ErrAPIKeyInvalidFormat
		}
		headerValue = strings.TrimPrefix(headerValue, a.prefix)
	}

	// Trim whitespace
	apiKey := strings.TrimSpace(headerValue)
	if apiKey == "" {
		return "", ErrAPIKeyInvalidFormat
	}

	return apiKey, nil
}

// MemoryAPIKeyStore implements APIKeyStore using in-memory storage
type MemoryAPIKeyStore struct {
	mu   sync.RWMutex
	keys map[string]*APIKeyInfo // Map of hashed key -> key info
	byID map[string]*APIKeyInfo // Map of key ID -> key info
}

// NewMemoryAPIKeyStore creates a new in-memory API key store
func NewMemoryAPIKeyStore() *MemoryAPIKeyStore {
	return &MemoryAPIKeyStore{
		keys: make(map[string]*APIKeyInfo),
		byID: make(map[string]*APIKeyInfo),
	}
}

// AddAPIKey adds an API key to the store
func (s *MemoryAPIKeyStore) AddAPIKey(keyValue string, info *APIKeyInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.keys[keyValue] = info
	s.byID[info.KeyID] = info
}

// GetAPIKeyInfo retrieves API key information by key value
func (s *MemoryAPIKeyStore) GetAPIKeyInfo(ctx context.Context, keyValue string) (*APIKeyInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	keyInfo, exists := s.keys[keyValue]
	if !exists {
		return nil, ErrAPIKeyNotFound
	}
	
	// Return a copy to prevent modification
	copy := *keyInfo
	return &copy, nil
}

// GetAPIKeyByID retrieves API key information by key ID
func (s *MemoryAPIKeyStore) GetAPIKeyByID(ctx context.Context, keyID string) (*APIKeyInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	keyInfo, exists := s.byID[keyID]
	if !exists {
		return nil, ErrAPIKeyNotFound
	}
	
	// Return a copy to prevent modification
	copy := *keyInfo
	return &copy, nil
}

// UpdateLastUsed updates the last used timestamp for an API key
func (s *MemoryAPIKeyStore) UpdateLastUsed(ctx context.Context, keyID string, timestamp time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	keyInfo, exists := s.byID[keyID]
	if !exists {
		return ErrAPIKeyNotFound
	}
	
	keyInfo.LastUsedAt = &timestamp
	return nil
}

// IsRevoked checks if an API key has been revoked
func (s *MemoryAPIKeyStore) IsRevoked(ctx context.Context, keyID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	keyInfo, exists := s.byID[keyID]
	if !exists {
		return false, ErrAPIKeyNotFound
	}
	
	return keyInfo.IsRevoked, nil
}

// RevokeAPIKey revokes an API key
func (s *MemoryAPIKeyStore) RevokeAPIKey(keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	keyInfo, exists := s.byID[keyID]
	if !exists {
		return ErrAPIKeyNotFound
	}
	
	keyInfo.IsRevoked = true
	return nil
}

// secureCompare performs constant-time string comparison to prevent timing attacks
func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}