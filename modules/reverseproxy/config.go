// Package reverseproxy provides configuration structures for the reverse proxy module.
package reverseproxy

import "time"

// ReverseProxyConfig provides configuration options for the ReverseProxyModule.
type ReverseProxyConfig struct {
	BackendServices        map[string]string               `json:"backend_services" yaml:"backend_services"`
	Routes                 map[string]string               `json:"routes" yaml:"routes"`
	DefaultBackend         string                          `json:"default_backend" yaml:"default_backend"`
	CircuitBreakerConfig   CircuitBreakerConfig            `json:"circuit_breaker" yaml:"circuit_breaker"`
	BackendCircuitBreakers map[string]CircuitBreakerConfig `json:"backend_circuit_breakers" yaml:"backend_circuit_breakers"`
	CompositeRoutes        map[string]CompositeRoute       `json:"composite_routes" yaml:"composite_routes"`
	TenantIDHeader         string                          `json:"tenant_id_header" yaml:"tenant_id_header"`
	RequireTenantID        bool                            `json:"require_tenant_id" yaml:"require_tenant_id"`
	CacheEnabled           bool                            `json:"cache_enabled" yaml:"cache_enabled"`
	CacheTTL               time.Duration                   `json:"cache_ttl" yaml:"cache_ttl"`
	RequestTimeout         time.Duration                   `json:"request_timeout" yaml:"request_timeout"`
	MetricsEnabled         bool                            `json:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsPath            string                          `json:"metrics_path" yaml:"metrics_path"`
	MetricsEndpoint        string                          `json:"metrics_endpoint" yaml:"metrics_endpoint"`
}

// CompositeRoute defines a route that combines responses from multiple backends.
type CompositeRoute struct {
	Pattern  string   `json:"pattern" yaml:"pattern"`
	Backends []string `json:"backends" yaml:"backends"`
	Strategy string   `json:"strategy" yaml:"strategy"`
}

// Config provides configuration options for the ReverseProxyModule.
// This is the original Config struct which is being phased out in favor of ReverseProxyConfig.
type Config struct {
	Backends       map[string]BackendConfig `json:"backends" yaml:"backends"`
	PrefixMapping  map[string]string        `json:"prefix_mapping" yaml:"prefix_mapping"`
	ExactMapping   map[string]string        `json:"exact_mapping" yaml:"exact_mapping"`
	MetricsEnabled bool                     `json:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsPath    string                   `json:"metrics_path" yaml:"metrics_path"`
	CircuitBreaker CircuitBreakerConfig     `json:"circuit_breaker" yaml:"circuit_breaker"`
	Retry          RetryConfig              `json:"retry" yaml:"retry"`
}

// BackendConfig provides configuration for a backend server.
type BackendConfig struct {
	URL                 string                `json:"url" yaml:"url"`
	Timeout             time.Duration         `json:"timeout" yaml:"timeout"`
	MaxIdleConns        int                   `json:"max_idle_conns" yaml:"max_idle_conns"`
	MaxIdleConnsPerHost int                   `json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`
	MaxConnsPerHost     int                   `json:"max_conns_per_host" yaml:"max_conns_per_host"`
	IdleConnTimeout     time.Duration         `json:"idle_conn_timeout" yaml:"idle_conn_timeout"`
	TLSSkipVerify       bool                  `json:"tls_skip_verify" yaml:"tls_skip_verify"`
	CircuitBreaker      *CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker"`
	Retry               *RetryConfig          `json:"retry" yaml:"retry"`
}

// CircuitBreakerConfig provides configuration for the circuit breaker.
type CircuitBreakerConfig struct {
	Enabled                 bool          `json:"enabled" yaml:"enabled"`
	FailureThreshold        int           `json:"failure_threshold" yaml:"failure_threshold"`
	SuccessThreshold        int           `json:"success_threshold" yaml:"success_threshold"`
	OpenTimeout             time.Duration `json:"open_timeout" yaml:"open_timeout"`
	HalfOpenAllowedRequests int           `json:"half_open_allowed_requests" yaml:"half_open_allowed_requests"`
	WindowSize              int           `json:"window_size" yaml:"window_size"`
	SuccessRateThreshold    float64       `json:"success_rate_threshold" yaml:"success_rate_threshold"`
}

// RetryConfig provides configuration for the retry policy.
type RetryConfig struct {
	Enabled              bool          `json:"enabled" yaml:"enabled"`
	MaxRetries           int           `json:"max_retries" yaml:"max_retries"`
	BaseDelay            time.Duration `json:"base_delay" yaml:"base_delay"`
	MaxDelay             time.Duration `json:"max_delay" yaml:"max_delay"`
	Jitter               float64       `json:"jitter" yaml:"jitter"`
	Timeout              time.Duration `json:"timeout" yaml:"timeout"`
	RetryableStatusCodes []int         `json:"retryable_status_codes" yaml:"retryable_status_codes"`
}
