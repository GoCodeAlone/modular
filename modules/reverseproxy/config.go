// Package reverseproxy provides configuration structures for the reverse proxy module.
package reverseproxy

import "time"

// ReverseProxyConfig provides configuration options for the ReverseProxyModule.
type ReverseProxyConfig struct {
	BackendServices        map[string]string               `json:"backend_services" yaml:"backend_services" toml:"backend_services" env:"BACKEND_SERVICES"`
	Routes                 map[string]string               `json:"routes" yaml:"routes" toml:"routes" env:"ROUTES"`
	DefaultBackend         string                          `json:"default_backend" yaml:"default_backend" toml:"default_backend" env:"DEFAULT_BACKEND"`
	CircuitBreakerConfig   CircuitBreakerConfig            `json:"circuit_breaker" yaml:"circuit_breaker" toml:"circuit_breaker"`
	BackendCircuitBreakers map[string]CircuitBreakerConfig `json:"backend_circuit_breakers" yaml:"backend_circuit_breakers" toml:"backend_circuit_breakers"`
	CompositeRoutes        map[string]CompositeRoute       `json:"composite_routes" yaml:"composite_routes" toml:"composite_routes"`
	TenantIDHeader         string                          `json:"tenant_id_header" yaml:"tenant_id_header" toml:"tenant_id_header" env:"TENANT_ID_HEADER"`
	RequireTenantID        bool                            `json:"require_tenant_id" yaml:"require_tenant_id" toml:"require_tenant_id" env:"REQUIRE_TENANT_ID"`
	CacheEnabled           bool                            `json:"cache_enabled" yaml:"cache_enabled" toml:"cache_enabled" env:"CACHE_ENABLED"`
	CacheTTL               time.Duration                   `json:"cache_ttl" yaml:"cache_ttl" toml:"cache_ttl" env:"CACHE_TTL"`
	RequestTimeout         time.Duration                   `json:"request_timeout" yaml:"request_timeout" toml:"request_timeout" env:"REQUEST_TIMEOUT"`
	MetricsEnabled         bool                            `json:"metrics_enabled" yaml:"metrics_enabled" toml:"metrics_enabled" env:"METRICS_ENABLED"`
	MetricsPath            string                          `json:"metrics_path" yaml:"metrics_path" toml:"metrics_path" env:"METRICS_PATH"`
	MetricsEndpoint        string                          `json:"metrics_endpoint" yaml:"metrics_endpoint" toml:"metrics_endpoint" env:"METRICS_ENDPOINT"`
}

// CompositeRoute defines a route that combines responses from multiple backends.
type CompositeRoute struct {
	Pattern  string   `json:"pattern" yaml:"pattern" toml:"pattern" env:"PATTERN"`
	Backends []string `json:"backends" yaml:"backends" toml:"backends" env:"BACKENDS"`
	Strategy string   `json:"strategy" yaml:"strategy" toml:"strategy" env:"STRATEGY"`
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
	Enabled                 bool          `json:"enabled" yaml:"enabled" toml:"enabled" env:"ENABLED"`
	FailureThreshold        int           `json:"failure_threshold" yaml:"failure_threshold" toml:"failure_threshold" env:"FAILURE_THRESHOLD"`
	SuccessThreshold        int           `json:"success_threshold" yaml:"success_threshold" toml:"success_threshold" env:"SUCCESS_THRESHOLD"`
	OpenTimeout             time.Duration `json:"open_timeout" yaml:"open_timeout" toml:"open_timeout" env:"OPEN_TIMEOUT"`
	HalfOpenAllowedRequests int           `json:"half_open_allowed_requests" yaml:"half_open_allowed_requests" toml:"half_open_allowed_requests" env:"HALF_OPEN_ALLOWED_REQUESTS"`
	WindowSize              int           `json:"window_size" yaml:"window_size" toml:"window_size" env:"WINDOW_SIZE"`
	SuccessRateThreshold    float64       `json:"success_rate_threshold" yaml:"success_rate_threshold" toml:"success_rate_threshold" env:"SUCCESS_RATE_THRESHOLD"`
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
