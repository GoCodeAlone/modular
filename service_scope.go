package modular

import (
	"errors"
	"fmt"
)

// ServiceScope defines the lifecycle and instantiation behavior of services
// within the dependency injection container.
//
// The scope determines:
//   - How many instances of a service can exist
//   - When instances are created and destroyed
//   - How long instances are cached
//   - Whether instances are shared across requests
type ServiceScope string

const (
	// ServiceScopeSingleton creates a single instance that is shared across
	// the entire application lifetime. The instance is created on first access
	// and reused for all subsequent requests. This is the most memory-efficient
	// scope for stateless services.
	ServiceScopeSingleton ServiceScope = "singleton"

	// ServiceScopeTransient creates a new instance every time the service
	// is requested. No caching is performed, and each instance is independent.
	// This is useful for stateful services or when you need fresh instances.
	ServiceScopeTransient ServiceScope = "transient"

	// ServiceScopeScoped creates one instance per logical scope (e.g., per HTTP request,
	// per tenant, per transaction). The instance is cached within the scope
	// and reused for all requests within that scope. This balances memory efficiency
	// with instance isolation.
	ServiceScopeScoped ServiceScope = "scoped"

	// ServiceScopeFactory provides a factory function that creates instances
	// on demand. The factory itself is typically a singleton, but it can create
	// instances with any desired behavior. This provides maximum flexibility
	// for complex instantiation scenarios.
	ServiceScopeFactory ServiceScope = "factory"
)

// String returns the string representation of the service scope.
func (s ServiceScope) String() string {
	return string(s)
}

// IsValid returns true if the service scope is one of the defined constants.
func (s ServiceScope) IsValid() bool {
	switch s {
	case ServiceScopeSingleton, ServiceScopeTransient, ServiceScopeScoped, ServiceScopeFactory:
		return true
	default:
		return false
	}
}

// ParseServiceScope parses a string into a ServiceScope, returning an error
// if the string is not a valid service scope.
func ParseServiceScope(s string) (ServiceScope, error) {
	scope := ServiceScope(s)
	if !scope.IsValid() {
		return "", fmt.Errorf("invalid service scope: %s", s)
	}
	return scope, nil
}

// GetDefaultServiceScope returns the default service scope used when
// no explicit scope is specified.
func GetDefaultServiceScope() ServiceScope {
	return ServiceScopeSingleton
}

// AllowsMultipleInstances returns true if this scope allows multiple instances
// to exist simultaneously.
func (s ServiceScope) AllowsMultipleInstances() bool {
	switch s {
	case ServiceScopeSingleton:
		return false // Only one instance across the entire application
	case ServiceScopeTransient:
		return true // New instance every time
	case ServiceScopeScoped:
		return true // Multiple instances, one per scope
	case ServiceScopeFactory:
		return true // Factory can create multiple instances
	default:
		return false
	}
}

// IsCacheable returns true if instances of this scope should be cached
// and reused rather than recreated each time.
func (s ServiceScope) IsCacheable() bool {
	switch s {
	case ServiceScopeSingleton:
		return true // Cache for the entire application lifetime
	case ServiceScopeTransient:
		return false // Never cache, always create new
	case ServiceScopeScoped:
		return true // Cache within the scope boundary
	case ServiceScopeFactory:
		return false // Factory decides its own caching strategy
	default:
		return false
	}
}

// Description returns a brief description of the service scope behavior.
func (s ServiceScope) Description() string {
	switch s {
	case ServiceScopeSingleton:
		return "Single instance shared across the application"
	case ServiceScopeTransient:
		return "New instance created for each request"
	case ServiceScopeScoped:
		return "Single instance per scope (e.g., request, session)"
	case ServiceScopeFactory:
		return "Factory method called for each request"
	default:
		return "Unknown scope behavior"
	}
}

// DetailedDescription returns a detailed explanation of the service scope.
func (s ServiceScope) DetailedDescription() string {
	switch s {
	case ServiceScopeSingleton:
		return "One instance is created and reused for all requests"
	case ServiceScopeTransient:
		return "A new instance is created every time the service is requested"
	case ServiceScopeScoped:
		return "One instance per defined scope boundary"
	case ServiceScopeFactory:
		return "A factory function is invoked to create instances"
	default:
		return "Unknown service scope with undefined behavior"
	}
}

// Equals checks if two service scopes are the same.
func (s ServiceScope) Equals(other ServiceScope) bool {
	return s == other
}

// IsCompatibleWith checks if this scope is compatible with another scope
// for dependency injection purposes.
func (s ServiceScope) IsCompatibleWith(other ServiceScope) bool {
	// This method checks if 's' can depend on 'other'
	// Generally, longer-lived scopes can depend on shorter-lived ones
	switch s {
	case ServiceScopeSingleton:
		// Singleton can depend on anything (including transient)
		return true
	case ServiceScopeScoped:
		// Scoped can depend on anything (including transient and singleton)
		return true
	case ServiceScopeTransient:
		// Transient should not depend on longer-lived scopes like singleton
		// to avoid unexpected behavior (transient expecting fresh instances)
		return other != ServiceScopeSingleton
	case ServiceScopeFactory:
		// Factory scope is flexible and can depend on anything
		return true
	default:
		return false
	}
}

// ServiceScopeConfig provides configuration options for service scope behavior.
type ServiceScopeConfig struct {
	// Scope defines the service scope type
	Scope ServiceScope

	// ScopeKey is the key used to identify the scope boundary (for scoped services)
	ScopeKey string

	// MaxInstances limits the number of instances that can be created
	MaxInstances int

	// InstanceTimeout specifies how long instances should be cached
	InstanceTimeout string

	// EnableCaching determines if caching is enabled for cacheable scopes
	EnableCaching bool

	// EnableMetrics determines if scope-related metrics should be collected
	EnableMetrics bool
}

// IsValid returns true if the service scope configuration is valid.
func (c ServiceScopeConfig) IsValid() bool {
	// Basic validation rules
	if !c.Scope.IsValid() {
		return false
	}

	if c.MaxInstances < 0 {
		return false
	}

	if c.Scope == ServiceScopeScoped && c.ScopeKey == "" {
		return false // Scoped services need a scope key
	}

	return true
}

// OrderScopesByLifetime orders service scopes by their lifetime, from longest to shortest.
// This is useful for dependency resolution and initialization ordering.
func OrderScopesByLifetime(scopes []ServiceScope) []ServiceScope {
	// Create a copy to avoid modifying the original slice
	ordered := make([]ServiceScope, len(scopes))
	copy(ordered, scopes)

	// Define lifetime ordering (longer lifetime = lower number)
	lifetimeOrder := map[ServiceScope]int{
		ServiceScopeSingleton: 0, // Longest lifetime
		ServiceScopeScoped:    1, // Medium lifetime
		ServiceScopeTransient: 2, // Short lifetime
		ServiceScopeFactory:   2, // Short lifetime (same as transient)
	}

	// Sort by lifetime order
	for i := 0; i < len(ordered)-1; i++ {
		for j := i + 1; j < len(ordered); j++ {
			orderI := lifetimeOrder[ordered[i]]
			orderJ := lifetimeOrder[ordered[j]]
			if orderI > orderJ {
				ordered[i], ordered[j] = ordered[j], ordered[i]
			}
		}
	}

	return ordered
}

// GetDefaultScopeConfig returns the default configuration for a specific service scope.
func GetDefaultScopeConfig(scope ServiceScope) ServiceScopeConfig {
	config := ServiceScopeConfig{
		Scope:         scope,
		EnableCaching: true,
		EnableMetrics: false,
	}

	switch scope {
	case ServiceScopeSingleton:
		config.MaxInstances = 1
		config.InstanceTimeout = "0" // Never expires
		config.ScopeKey = ""
	case ServiceScopeTransient:
		config.MaxInstances = 1000   // Allow many instances
		config.InstanceTimeout = "0" // No caching
		config.ScopeKey = ""
	case ServiceScopeScoped:
		config.MaxInstances = 100
		config.InstanceTimeout = "5m"
		config.ScopeKey = "default"
	case ServiceScopeFactory:
		config.MaxInstances = 1000 // Factory can create many
		config.InstanceTimeout = "0"
		config.ScopeKey = ""
	default:
		config.MaxInstances = 1
		config.InstanceTimeout = "0"
		config.ScopeKey = ""
	}

	return config
}

// Errors related to service scope validation
var (
	// ErrInvalidServiceScope indicates that an invalid service scope was provided
	ErrInvalidServiceScope = errors.New("invalid service scope")
)
