// Package reverseproxy provides a flexible reverse proxy module with support for multiple backends,
// composite responses, and tenant awareness.
package reverseproxy

import (
	"net/http"
)

// ReverseProxyConfig defines the configuration for a reverse proxy module instance.
type ReverseProxyConfig struct {
	// BackendServices maps backend IDs to their service URLs.
	BackendServices map[string]string `yaml:"backend_services"`

	// DefaultBackend is the ID of the default backend to use.
	DefaultBackend string `yaml:"default_backend"`

	// Routes maps URL patterns to backend IDs.
	// These are used for direct proxying to a single backend.
	Routes map[string]string `yaml:"routes"`

	// CompositeRoutes maps URL patterns to composite route configurations.
	// These are used for routes that combine responses from multiple backends.
	CompositeRoutes map[string]CompositeRoute `yaml:"composite_routes"`

	// TenantIDHeader is the name of the HTTP header containing the tenant ID.
	TenantIDHeader string `yaml:"tenant_id_header"`

	// RequireTenantID indicates if requests must include a tenant ID.
	RequireTenantID bool `yaml:"require_tenant_id"`

	// CacheEnabled enables HTTP response caching for composite routes
	CacheEnabled bool `yaml:"cache_enabled"`

	// CacheTTL defines how long cached responses remain valid (in seconds)
	CacheTTL int `yaml:"cache_ttl"`

	// CircuitBreakerConfig holds the global circuit breaker configuration
	CircuitBreakerConfig CircuitBreakerConfig `yaml:"circuit_breaker"`

	// BackendCircuitBreakers defines per-backend circuit breaker configurations,
	// overriding the global settings for specific backends
	BackendCircuitBreakers map[string]CircuitBreakerConfig `yaml:"backend_circuit_breakers"`

	// PreProxyTransformations defines functions that can modify requests before they are sent to the backend
	PreProxyTransformations []func(*http.Request) error `yaml:"-"`
}

// CircuitBreakerConfig holds configuration options for a circuit breaker
type CircuitBreakerConfig struct {
	// Enabled indicates if the circuit breaker is active
	Enabled bool `yaml:"enabled"`

	// FailureThreshold is the number of failures before opening the circuit
	FailureThreshold int `yaml:"failure_threshold"`

	// ResetTimeoutSeconds is the number of seconds to wait before trying a request
	// when the circuit is open
	ResetTimeoutSeconds int `yaml:"reset_timeout_seconds"`
}

// CompositeRoute represents a route that combines responses from multiple backends.
// This allows for creating composite APIs that aggregate data from multiple sources.
type CompositeRoute struct {
	// Pattern is the URL path pattern to match.
	// This uses the router's pattern matching syntax (typically chi router syntax).
	Pattern string `json:"pattern" yaml:"pattern"`

	// Backends is a list of backend identifiers to route to.
	// These should correspond to keys in the BackendServices map.
	Backends []string `json:"backends" yaml:"backends"`

	// Strategy determines how to combine responses from multiple backends.
	// Supported values include:
	// - "merge" - Merge JSON responses at the top level
	// - "select" - Select a specific backend's response
	// - "compare" - Compare responses from multiple backends
	// - "custom" - Use a custom response transformer
	Strategy string `json:"strategy" yaml:"strategy"`
}

// Validate implements the modular.ConfigValidator interface to ensure the
// configuration is valid before the module is initialized.
func (c *ReverseProxyConfig) Validate() error {
	// Initialize maps if they're nil
	if c.BackendServices == nil {
		c.BackendServices = make(map[string]string)
	}

	// Make sure default backend is valid
	if c.DefaultBackend == "" && len(c.BackendServices) > 0 {
		// Set first available backend as default
		for backendID := range c.BackendServices {
			c.DefaultBackend = backendID
			break
		}
	}

	// Set defaults for global circuit breaker config if not specified
	if c.CircuitBreakerConfig.FailureThreshold == 0 {
		c.CircuitBreakerConfig.FailureThreshold = 5 // Default 5 failures
	}
	if c.CircuitBreakerConfig.ResetTimeoutSeconds == 0 {
		c.CircuitBreakerConfig.ResetTimeoutSeconds = 30 // Default 30 seconds
	}
	if !c.CircuitBreakerConfig.Enabled {
		c.CircuitBreakerConfig.Enabled = true // Enabled by default
	}

	// Initialize backend circuit breakers map if nil
	if c.BackendCircuitBreakers == nil {
		c.BackendCircuitBreakers = make(map[string]CircuitBreakerConfig)
	}

	return nil
}
