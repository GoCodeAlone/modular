package modular

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TenantGuardMode defines the strictness level for tenant isolation enforcement.
// Different modes provide different levels of tenant isolation checking and
// violation handling.
type TenantGuardMode string

const (
	// TenantGuardModeStrict enforces strict tenant isolation.
	// Cross-tenant access attempts will be blocked and result in errors.
	// This provides the highest level of tenant isolation security.
	TenantGuardModeStrict TenantGuardMode = "strict"

	// TenantGuardModeLenient enforces tenant isolation with warnings.
	// Cross-tenant access attempts are logged but allowed to proceed.
	// This provides backward compatibility while monitoring violations.
	TenantGuardModeLenient TenantGuardMode = "lenient"

	// TenantGuardModeDisabled disables tenant isolation enforcement.
	// No tenant checking is performed, essentially single-tenant mode.
	// This is useful for testing or single-tenant deployments.
	TenantGuardModeDisabled TenantGuardMode = "disabled"
)

// String returns the string representation of the tenant guard mode.
func (m TenantGuardMode) String() string {
	return string(m)
}

// IsEnforcing returns true if this mode performs any kind of tenant enforcement.
func (m TenantGuardMode) IsEnforcing() bool {
	return m == TenantGuardModeStrict || m == TenantGuardModeLenient
}

// IsStrict returns true if this mode strictly enforces tenant isolation.
func (m TenantGuardMode) IsStrict() bool {
	return m == TenantGuardModeStrict
}

// ParseTenantGuardMode parses a string into a TenantGuardMode.
func ParseTenantGuardMode(s string) (TenantGuardMode, error) {
	mode := TenantGuardMode(s)
	switch mode {
	case TenantGuardModeStrict, TenantGuardModeLenient, TenantGuardModeDisabled:
		return mode, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidTenantGuardMode, s)
	}
}

// TenantGuardConfig provides configuration options for tenant guard behavior.
type TenantGuardConfig struct {
	// Mode defines the tenant guard enforcement mode
	Mode TenantGuardMode `json:"mode"`

	// EnforceIsolation enables tenant isolation enforcement
	EnforceIsolation bool `json:"enforce_isolation"`

	// AllowCrossTenant allows cross-tenant access (when false, blocks cross-tenant)
	AllowCrossTenant bool `json:"allow_cross_tenant"`

	// ValidationTimeout specifies timeout for tenant validation operations
	ValidationTimeout time.Duration `json:"validation_timeout"`

	// MaxTenantCacheSize limits the size of the tenant cache
	MaxTenantCacheSize int `json:"max_tenant_cache_size"`

	// TenantTTL specifies how long to cache tenant information
	TenantTTL time.Duration `json:"tenant_ttl"`

	// LogViolations enables logging of tenant violations
	LogViolations bool `json:"log_violations"`

	// BlockViolations enables blocking of tenant violations
	BlockViolations bool `json:"block_violations"`

	// CrossTenantWhitelist maps tenants to allowed cross-tenant access targets
	CrossTenantWhitelist map[string][]string `json:"cross_tenant_whitelist,omitempty"`
}

// IsValid validates the tenant guard configuration.
func (c TenantGuardConfig) IsValid() bool {
	// Check if mode is valid
	if c.Mode != TenantGuardModeStrict && c.Mode != TenantGuardModeLenient && c.Mode != TenantGuardModeDisabled {
		return false
	}

	// Validation timeout must be positive
	if c.ValidationTimeout < 0 {
		return false
	}

	// Max cache size cannot be negative
	if c.MaxTenantCacheSize < 0 {
		return false
	}

	// TTL cannot be negative
	if c.TenantTTL < 0 {
		return false
	}

	return true
}

// NewDefaultTenantGuardConfig creates a default tenant guard configuration for the given mode.
func NewDefaultTenantGuardConfig(mode TenantGuardMode) TenantGuardConfig {
	config := TenantGuardConfig{
		Mode:                 mode,
		ValidationTimeout:    5 * time.Second,
		MaxTenantCacheSize:   1000,
		TenantTTL:            10 * time.Minute,
		LogViolations:        true,
		CrossTenantWhitelist: make(map[string][]string),
	}

	switch mode {
	case TenantGuardModeStrict:
		config.EnforceIsolation = true
		config.AllowCrossTenant = false
		config.BlockViolations = true

	case TenantGuardModeLenient:
		config.EnforceIsolation = true
		config.AllowCrossTenant = true // Allow but log
		config.BlockViolations = false

	case TenantGuardModeDisabled:
		config.EnforceIsolation = false
		config.AllowCrossTenant = true
		config.BlockViolations = false
		config.LogViolations = false
	}

	return config
}

// TenantViolationType defines the type of tenant isolation violation.
type TenantViolationType string

const (
	// TenantViolationCrossTenantAccess indicates access across tenant boundaries
	TenantViolationCrossTenantAccess TenantViolationType = "cross_tenant_access"

	// TenantViolationInvalidTenantContext indicates invalid tenant context
	TenantViolationInvalidTenantContext TenantViolationType = "invalid_tenant_context"

	// TenantViolationMissingTenantContext indicates missing tenant context
	TenantViolationMissingTenantContext TenantViolationType = "missing_tenant_context"

	// TenantViolationUnauthorizedOperation indicates unauthorized tenant operation
	TenantViolationUnauthorizedOperation TenantViolationType = "unauthorized_tenant_operation"
)

// TenantViolationSeverity defines the severity level of tenant violations.
type TenantViolationSeverity string

const (
	// TenantViolationSeverityLow indicates low-severity violations
	TenantViolationSeverityLow TenantViolationSeverity = "low"

	// TenantViolationSeverityMedium indicates medium-severity violations
	TenantViolationSeverityMedium TenantViolationSeverity = "medium"

	// TenantViolationSeverityHigh indicates high-severity violations
	TenantViolationSeverityHigh TenantViolationSeverity = "high"

	// TenantViolationSeverityCritical indicates critical-severity violations
	TenantViolationSeverityCritical TenantViolationSeverity = "critical"
)

// TenantViolation represents a tenant isolation violation.
type TenantViolation struct {
	// RequestingTenant is the tenant that initiated the request
	RequestingTenant string `json:"requesting_tenant"`

	// AccessedResource is the resource that was accessed
	AccessedResource string `json:"accessed_resource"`

	// ViolationType classifies the type of violation
	ViolationType TenantViolationType `json:"violation_type"`

	// Timestamp records when the violation occurred
	Timestamp time.Time `json:"timestamp"`

	// Severity indicates the severity level of the violation
	Severity TenantViolationSeverity `json:"severity"`

	// Context provides additional context about the violation
	Context map[string]interface{} `json:"context,omitempty"`
}

// TenantGuard provides tenant isolation enforcement functionality.
type TenantGuard interface {
	// GetMode returns the current tenant guard mode
	GetMode() TenantGuardMode

	// ValidateAccess validates whether a tenant access should be allowed
	ValidateAccess(ctx context.Context, violation *TenantViolation) (bool, error)

	// GetRecentViolations returns recent tenant violations
	GetRecentViolations() []*TenantViolation
}

// WithTenantContext creates a new context with tenant information attached.
func WithTenantContext(ctx context.Context, tenantID string) context.Context {
	return NewTenantContext(ctx, TenantID(tenantID))
}

// scopeContextKeyType is a unique type for scope context keys to avoid collisions
type scopeContextKeyType string

// WithScopeContext creates a new context with scope information for scoped services.
func WithScopeContext(ctx context.Context, scopeKey, scopeValue string) context.Context {
	// Use a consistent key type that can be referenced from other packages
	return context.WithValue(ctx, scopeContextKeyType(scopeKey), scopeValue)
}

// WithTenantGuardMode configures tenant isolation strictness for multi-tenant applications.
// This option configures tenant access validation throughout the framework.
//
// Supported modes:
//   - TenantGuardModeStrict: Fail on cross-tenant access attempts with error
//   - TenantGuardModeLenient: Log warnings but allow access (backward compatibility)
//   - TenantGuardModeDisabled: No tenant checking (single-tenant mode)
//
// Parameters:
//   - mode: The tenant guard mode to use
//
// Example:
//
//	app := NewApplication(
//	    WithTenantGuardMode(TenantGuardModeStrict),
//	)
func WithTenantGuardMode(mode TenantGuardMode) Option {
	return WithTenantGuardModeConfig(NewDefaultTenantGuardConfig(mode))
}

// WithTenantGuardModeConfig configures tenant isolation with detailed configuration.
// This allows fine-tuned control over tenant isolation behavior.
//
// Parameters:
//   - config: Detailed tenant guard configuration
//
// Example:
//
//	config := TenantGuardConfig{
//	    Mode: TenantGuardModeStrict,
//	    EnforceIsolation: true,
//	    ValidationTimeout: 5 * time.Second,
//	}
//	app := NewApplication(
//	    WithTenantGuardModeConfig(config),
//	)
func WithTenantGuardModeConfig(config TenantGuardConfig) Option {
	return func(builder *ApplicationBuilder) error {
		if !config.IsValid() {
			return ErrInvalidTenantGuardConfiguration
		}

		// Create and register a tenant guard service
		tenantGuard := &stdTenantGuard{
			config:     config,
			violations: make([]*TenantViolation, 0),
		}

		// Register the tenant guard as a service
		// In a real implementation, this would integrate with the service registry
		builder.tenantGuard = tenantGuard

		return nil
	}
}

// stdTenantGuard implements the TenantGuard interface
type stdTenantGuard struct {
	config     TenantGuardConfig
	violations []*TenantViolation
	mu         sync.RWMutex // protects violations slice
}

func (g *stdTenantGuard) GetMode() TenantGuardMode {
	return g.config.Mode
}

func (g *stdTenantGuard) ValidateAccess(ctx context.Context, violation *TenantViolation) (bool, error) {
	switch g.config.Mode {
	case TenantGuardModeDisabled:
		return true, nil

	case TenantGuardModeStrict:
		// In strict mode, check for cross-tenant access
		if violation.ViolationType == TenantViolationCrossTenantAccess {
			// Check whitelist
			if g.isWhitelisted(violation.RequestingTenant, violation.AccessedResource) {
				return true, nil
			}
			return false, nil // Block the access
		}
		return true, nil

	case TenantGuardModeLenient:
		// In lenient mode, log but allow access
		if violation.ViolationType == TenantViolationCrossTenantAccess {
			g.logViolation(violation)
		}
		return true, nil

	default:
		return false, fmt.Errorf("%w: %s", ErrUnknownTenantGuardMode, g.config.Mode)
	}
}

func (g *stdTenantGuard) GetRecentViolations() []*TenantViolation {
	g.mu.RLock()
	defer g.mu.RUnlock()
	// Return a shallow copy to avoid callers mutating internal slice
	out := make([]*TenantViolation, len(g.violations))
	copy(out, g.violations)
	return out
}

func (g *stdTenantGuard) isWhitelisted(requestingTenant, accessedResource string) bool {
	if g.config.CrossTenantWhitelist == nil {
		return false
	}

	allowedTargets, exists := g.config.CrossTenantWhitelist[requestingTenant]
	if !exists {
		return false
	}

	// Extract tenant from resource path (simple implementation)
	// In a real system, this would be more sophisticated
	for _, target := range allowedTargets {
		if len(accessedResource) > len(target) && accessedResource[:len(target)+1] == target+"/" {
			return true
		}
	}

	return false
}

func (g *stdTenantGuard) logViolation(violation *TenantViolation) {
	violation.Timestamp = time.Now()
	g.mu.Lock()
	g.violations = append(g.violations, violation)
	g.mu.Unlock()

	// In a real implementation, this would use proper logging
	// For now, we just store it for testing
}

// Extend ApplicationBuilder to support tenant guard
type ApplicationBuilderExtension struct {
	*ApplicationBuilder
	tenantGuard TenantGuard //nolint:unused // reserved for future tenant guard functionality
}

// GetTenantGuard returns the application's tenant guard if configured.
// (GetTenantGuard now defined on StdApplication in application.go to satisfy Application interface)
