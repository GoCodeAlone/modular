// Package auth provides authentication and authorization services
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Static errors for principal and claims mapping
var (
	ErrPrincipalNotFound     = errors.New("principal not found")
	ErrInvalidClaims         = errors.New("invalid claims structure")
	ErrMissingRequiredClaim  = errors.New("missing required claim")
	ErrClaimMappingFailed    = errors.New("claim mapping failed")
	ErrUnauthorizedAccess    = errors.New("unauthorized access")
	ErrInsufficientRole      = errors.New("insufficient role for operation")
)

// Principal represents an authenticated entity (user, service, etc.)
type Principal struct {
	// Core identity fields
	ID       string `json:"id"`        // Unique identifier (subject)
	Type     string `json:"type"`      // e.g., "user", "service", "api-key"
	Name     string `json:"name"`      // Display name
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`
	
	// Authentication context
	AuthMethod     string     `json:"auth_method"`      // e.g., "jwt", "api-key", "oauth2"
	AuthTime       time.Time  `json:"auth_time"`        // When authentication occurred
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	Issuer         string     `json:"issuer,omitempty"` // Token issuer
	Audience       string     `json:"audience,omitempty"`
	
	// Authorization information
	Roles       []string          `json:"roles,omitempty"`
	Permissions []string          `json:"permissions,omitempty"`
	Scopes      []string          `json:"scopes,omitempty"`
	Groups      []string          `json:"groups,omitempty"`
	
	// Custom attributes and metadata
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	Metadata   map[string]string      `json:"metadata,omitempty"`
	
	// Tenant context for multi-tenant applications
	TenantID     string   `json:"tenant_id,omitempty"`
	TenantRoles  []string `json:"tenant_roles,omitempty"`
	
	// Session information
	SessionID    string    `json:"session_id,omitempty"`
	IPAddress    string    `json:"ip_address,omitempty"`
	UserAgent    string    `json:"user_agent,omitempty"`
	
	// API Key specific information (if applicable)
	APIKeyID   string `json:"api_key_id,omitempty"`
	APIKeyName string `json:"api_key_name,omitempty"`
}

// ClaimsMapper defines the interface for mapping claims to a Principal
type ClaimsMapper interface {
	// MapJWTClaims maps JWT claims to a Principal
	MapJWTClaims(ctx context.Context, claims *JWTClaims) (*Principal, error)
	
	// MapAPIKeyClaims maps API key information to a Principal
	MapAPIKeyClaims(ctx context.Context, keyInfo *APIKeyInfo) (*Principal, error)
	
	// MapCustomClaims maps custom claims to a Principal
	MapCustomClaims(ctx context.Context, claims map[string]interface{}) (*Principal, error)
}

// ClaimsMappingConfig configures how claims are mapped to Principal fields
type ClaimsMappingConfig struct {
	// JWT claim mappings
	SubjectClaim    string `json:"subject_claim"`     // Default: "sub"
	NameClaim       string `json:"name_claim"`        // Default: "name"
	EmailClaim      string `json:"email_claim"`       // Default: "email"
	UsernameClaim   string `json:"username_claim"`    // Default: "preferred_username"
	RolesClaim      string `json:"roles_claim"`       // Default: "roles"
	GroupsClaim     string `json:"groups_claim"`      // Default: "groups"
	ScopesClaim     string `json:"scopes_claim"`      // Default: "scope"
	TenantClaim     string `json:"tenant_claim"`      // Default: "tenant_id"
	
	// Custom attribute mappings
	AttributeMappings map[string]string `json:"attribute_mappings,omitempty"`
	
	// Required claims
	RequiredClaims []string `json:"required_claims,omitempty"`
	
	// Default values
	DefaultType     string            `json:"default_type"`      // Default: "user"
	DefaultRoles    []string          `json:"default_roles,omitempty"`
	DefaultMetadata map[string]string `json:"default_metadata,omitempty"`
}

// DefaultClaimsMapper provides a configurable implementation of ClaimsMapper
type DefaultClaimsMapper struct {
	config *ClaimsMappingConfig
}

// NewDefaultClaimsMapper creates a new claims mapper with the given configuration
func NewDefaultClaimsMapper(config *ClaimsMappingConfig) *DefaultClaimsMapper {
	// Set defaults
	if config.SubjectClaim == "" {
		config.SubjectClaim = "sub"
	}
	if config.NameClaim == "" {
		config.NameClaim = "name"
	}
	if config.EmailClaim == "" {
		config.EmailClaim = "email"
	}
	if config.UsernameClaim == "" {
		config.UsernameClaim = "preferred_username"
	}
	if config.RolesClaim == "" {
		config.RolesClaim = "roles"
	}
	if config.GroupsClaim == "" {
		config.GroupsClaim = "groups"
	}
	if config.ScopesClaim == "" {
		config.ScopesClaim = "scope"
	}
	if config.TenantClaim == "" {
		config.TenantClaim = "tenant_id"
	}
	if config.DefaultType == "" {
		config.DefaultType = "user"
	}

	return &DefaultClaimsMapper{
		config: config,
	}
}

// MapJWTClaims maps JWT claims to a Principal
func (m *DefaultClaimsMapper) MapJWTClaims(ctx context.Context, claims *JWTClaims) (*Principal, error) {
	if claims == nil {
		return nil, ErrInvalidClaims
	}

	principal := &Principal{
		AuthMethod:  "jwt",
		AuthTime:    time.Now(),
		Type:        m.config.DefaultType,
		Issuer:      claims.Issuer,
		Audience:    claims.Audience,
		Attributes:  make(map[string]interface{}),
		Metadata:    make(map[string]string),
	}

	// Set expiration
	if claims.ExpiresAt > 0 {
		expiresAt := time.Unix(claims.ExpiresAt, 0)
		principal.ExpiresAt = &expiresAt
	}

	// Map standard claims
	principal.ID = claims.Subject
	
	if name, ok := claims.Custom[m.config.NameClaim].(string); ok {
		principal.Name = name
	}
	
	if email, ok := claims.Custom[m.config.EmailClaim].(string); ok {
		principal.Email = email
	}
	
	if username, ok := claims.Custom[m.config.UsernameClaim].(string); ok {
		principal.Username = username
	}

	// Map roles
	if rolesValue, ok := claims.Custom[m.config.RolesClaim]; ok {
		principal.Roles = m.extractStringSlice(rolesValue)
	}
	if len(principal.Roles) == 0 {
		principal.Roles = m.config.DefaultRoles
	}

	// Map groups
	if groupsValue, ok := claims.Custom[m.config.GroupsClaim]; ok {
		principal.Groups = m.extractStringSlice(groupsValue)
	}

	// Map scopes
	if scopesValue, ok := claims.Custom[m.config.ScopesClaim]; ok {
		principal.Scopes = m.extractStringSlice(scopesValue)
	}

	// Map tenant information
	if tenantID, ok := claims.Custom[m.config.TenantClaim].(string); ok {
		principal.TenantID = tenantID
	}

	// Map custom attributes
	for claimKey, principalKey := range m.config.AttributeMappings {
		if value, exists := claims.Custom[claimKey]; exists {
			principal.Attributes[principalKey] = value
		}
	}

	// Copy all unmapped custom claims as attributes
	for key, value := range claims.Custom {
		if _, mapped := m.config.AttributeMappings[key]; !mapped &&
			key != m.config.NameClaim &&
			key != m.config.EmailClaim &&
			key != m.config.UsernameClaim &&
			key != m.config.RolesClaim &&
			key != m.config.GroupsClaim &&
			key != m.config.ScopesClaim &&
			key != m.config.TenantClaim {
			principal.Attributes[key] = value
		}
	}

	// Apply default metadata
	for key, value := range m.config.DefaultMetadata {
		principal.Metadata[key] = value
	}

	// Validate required claims
	for _, requiredClaim := range m.config.RequiredClaims {
		if _, exists := claims.Custom[requiredClaim]; !exists {
			return nil, fmt.Errorf("%w: %s", ErrMissingRequiredClaim, requiredClaim)
		}
	}

	return principal, nil
}

// MapAPIKeyClaims maps API key information to a Principal
func (m *DefaultClaimsMapper) MapAPIKeyClaims(ctx context.Context, keyInfo *APIKeyInfo) (*Principal, error) {
	if keyInfo == nil {
		return nil, ErrInvalidClaims
	}

	principal := &Principal{
		ID:          keyInfo.KeyID,
		Type:        "api-key",
		Name:        keyInfo.Name,
		AuthMethod:  "api-key",
		AuthTime:    time.Now(),
		APIKeyID:    keyInfo.KeyID,
		APIKeyName:  keyInfo.Name,
		Scopes:      keyInfo.Scopes,
		ExpiresAt:   keyInfo.ExpiresAt,
		Attributes:  make(map[string]interface{}),
		Metadata:    make(map[string]string),
	}

	// Copy API key metadata to principal metadata
	for key, value := range keyInfo.Metadata {
		principal.Metadata[key] = value
	}

	// Apply default metadata
	for key, value := range m.config.DefaultMetadata {
		if _, exists := principal.Metadata[key]; !exists {
			principal.Metadata[key] = value
		}
	}

	// Use default roles if not specified
	principal.Roles = m.config.DefaultRoles

	return principal, nil
}

// MapCustomClaims maps custom claims to a Principal
func (m *DefaultClaimsMapper) MapCustomClaims(ctx context.Context, claims map[string]interface{}) (*Principal, error) {
	if claims == nil {
		return nil, ErrInvalidClaims
	}

	principal := &Principal{
		AuthMethod: "custom",
		AuthTime:   time.Now(),
		Type:       m.config.DefaultType,
		Attributes: make(map[string]interface{}),
		Metadata:   make(map[string]string),
	}

	// Map standard fields using configured claim names
	if id, ok := claims[m.config.SubjectClaim].(string); ok {
		principal.ID = id
	}
	
	if name, ok := claims[m.config.NameClaim].(string); ok {
		principal.Name = name
	}
	
	if email, ok := claims[m.config.EmailClaim].(string); ok {
		principal.Email = email
	}
	
	if username, ok := claims[m.config.UsernameClaim].(string); ok {
		principal.Username = username
	}

	// Map roles, groups, and scopes
	if rolesValue, ok := claims[m.config.RolesClaim]; ok {
		principal.Roles = m.extractStringSlice(rolesValue)
	}
	
	if groupsValue, ok := claims[m.config.GroupsClaim]; ok {
		principal.Groups = m.extractStringSlice(groupsValue)
	}
	
	if scopesValue, ok := claims[m.config.ScopesClaim]; ok {
		principal.Scopes = m.extractStringSlice(scopesValue)
	}

	// Apply defaults
	if len(principal.Roles) == 0 {
		principal.Roles = m.config.DefaultRoles
	}

	// Map custom attributes
	for claimKey, principalKey := range m.config.AttributeMappings {
		if value, exists := claims[claimKey]; exists {
			principal.Attributes[principalKey] = value
		}
	}

	// Apply default metadata
	for key, value := range m.config.DefaultMetadata {
		principal.Metadata[key] = value
	}

	return principal, nil
}

// extractStringSlice converts various types to a string slice
func (m *DefaultClaimsMapper) extractStringSlice(value interface{}) []string {
	switch v := value.(type) {
	case string:
		// Handle space-separated string (common for scopes)
		return strings.Fields(v)
	case []string:
		return v
	case []interface{}:
		var result []string
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		return nil
	}
}

// HasRole checks if the principal has a specific role
func (p *Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the principal has any of the specified roles
func (p *Principal) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if p.HasRole(role) {
			return true
		}
	}
	return false
}

// HasPermission checks if the principal has a specific permission
func (p *Principal) HasPermission(permission string) bool {
	for _, perm := range p.Permissions {
		if perm == permission {
			return true
		}
	}
	return false
}

// HasScope checks if the principal has a specific scope
func (p *Principal) HasScope(scope string) bool {
	for _, s := range p.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// IsExpired checks if the principal's authentication has expired
func (p *Principal) IsExpired() bool {
	return p.ExpiresAt != nil && time.Now().After(*p.ExpiresAt)
}

// GetAttribute returns a custom attribute value
func (p *Principal) GetAttribute(key string) (interface{}, bool) {
	value, exists := p.Attributes[key]
	return value, exists
}

// GetStringAttribute returns a custom attribute as a string
func (p *Principal) GetStringAttribute(key string) (string, bool) {
	value, exists := p.Attributes[key]
	if !exists {
		return "", false
	}
	if str, ok := value.(string); ok {
		return str, true
	}
	return "", false
}