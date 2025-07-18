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
	HealthCheck            HealthCheckConfig               `json:"health_check" yaml:"health_check" toml:"health_check"`
	// BackendConfigs defines per-backend configurations including path rewriting and header rewriting
	BackendConfigs map[string]BackendServiceConfig `json:"backend_configs" yaml:"backend_configs" toml:"backend_configs"`
}

// CompositeRoute defines a route that combines responses from multiple backends.
type CompositeRoute struct {
	Pattern  string   `json:"pattern" yaml:"pattern" toml:"pattern" env:"PATTERN"`
	Backends []string `json:"backends" yaml:"backends" toml:"backends" env:"BACKENDS"`
	Strategy string   `json:"strategy" yaml:"strategy" toml:"strategy" env:"STRATEGY"`
}

// PathRewritingConfig defines configuration for path rewriting rules.
type PathRewritingConfig struct {
	// StripBasePath removes the specified base path from all requests before forwarding to backends
	StripBasePath string `json:"strip_base_path" yaml:"strip_base_path" toml:"strip_base_path" env:"STRIP_BASE_PATH"`

	// BasePathRewrite replaces the base path with a new path for all requests
	BasePathRewrite string `json:"base_path_rewrite" yaml:"base_path_rewrite" toml:"base_path_rewrite" env:"BASE_PATH_REWRITE"`

	// EndpointRewrites defines per-endpoint path rewriting rules
	EndpointRewrites map[string]EndpointRewriteRule `json:"endpoint_rewrites" yaml:"endpoint_rewrites" toml:"endpoint_rewrites"`
}

// EndpointRewriteRule defines a rewrite rule for a specific endpoint pattern.
type EndpointRewriteRule struct {
	// Pattern is the incoming request pattern to match (e.g., "/api/v1/users")
	Pattern string `json:"pattern" yaml:"pattern" toml:"pattern" env:"PATTERN"`

	// Replacement is the new path to use when forwarding to backend (e.g., "/users")
	Replacement string `json:"replacement" yaml:"replacement" toml:"replacement" env:"REPLACEMENT"`

	// Backend specifies which backend this rule applies to (optional, applies to all if empty)
	Backend string `json:"backend" yaml:"backend" toml:"backend" env:"BACKEND"`

	// StripQueryParams removes query parameters from the request when forwarding
	StripQueryParams bool `json:"strip_query_params" yaml:"strip_query_params" toml:"strip_query_params" env:"STRIP_QUERY_PARAMS"`
}

// BackendServiceConfig defines configuration for a specific backend service.
type BackendServiceConfig struct {
	// URL is the base URL for the backend service
	URL string `json:"url" yaml:"url" toml:"url" env:"URL"`

	// PathRewriting defines path rewriting rules specific to this backend
	PathRewriting PathRewritingConfig `json:"path_rewriting" yaml:"path_rewriting" toml:"path_rewriting"`

	// HeaderRewriting defines header rewriting rules specific to this backend
	HeaderRewriting HeaderRewritingConfig `json:"header_rewriting" yaml:"header_rewriting" toml:"header_rewriting"`

	// Endpoints defines endpoint-specific configurations
	Endpoints map[string]EndpointConfig `json:"endpoints" yaml:"endpoints" toml:"endpoints"`
}

// EndpointConfig defines configuration for a specific endpoint within a backend service.
type EndpointConfig struct {
	// Pattern is the URL pattern that this endpoint matches (e.g., "/api/v1/users/*")
	Pattern string `json:"pattern" yaml:"pattern" toml:"pattern" env:"PATTERN"`

	// PathRewriting defines path rewriting rules specific to this endpoint
	PathRewriting PathRewritingConfig `json:"path_rewriting" yaml:"path_rewriting" toml:"path_rewriting"`

	// HeaderRewriting defines header rewriting rules specific to this endpoint
	HeaderRewriting HeaderRewritingConfig `json:"header_rewriting" yaml:"header_rewriting" toml:"header_rewriting"`
}

// HeaderRewritingConfig defines configuration for header rewriting rules.
type HeaderRewritingConfig struct {
	// HostnameHandling controls how the Host header is handled
	HostnameHandling HostnameHandlingMode `json:"hostname_handling" yaml:"hostname_handling" toml:"hostname_handling" env:"HOSTNAME_HANDLING"`

	// CustomHostname sets a custom hostname to use instead of the original or backend hostname
	CustomHostname string `json:"custom_hostname" yaml:"custom_hostname" toml:"custom_hostname" env:"CUSTOM_HOSTNAME"`

	// SetHeaders defines headers to set or override on the request
	SetHeaders map[string]string `json:"set_headers" yaml:"set_headers" toml:"set_headers"`

	// RemoveHeaders defines headers to remove from the request
	RemoveHeaders []string `json:"remove_headers" yaml:"remove_headers" toml:"remove_headers"`
}

// HostnameHandlingMode defines how the Host header should be handled when forwarding requests.
type HostnameHandlingMode string

const (
	// HostnamePreserveOriginal preserves the original client's Host header (default)
	HostnamePreserveOriginal HostnameHandlingMode = "preserve_original"

	// HostnameUseBackend uses the backend service's hostname
	HostnameUseBackend HostnameHandlingMode = "use_backend"

	// HostnameUseCustom uses a custom hostname specified in CustomHostname
	HostnameUseCustom HostnameHandlingMode = "use_custom"
)

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

// HealthCheckConfig provides configuration for backend health checking.
type HealthCheckConfig struct {
	Enabled                  bool                           `json:"enabled" yaml:"enabled" toml:"enabled" env:"ENABLED" default:"false" desc:"Enable health checking for backend services"`
	Interval                 time.Duration                  `json:"interval" yaml:"interval" toml:"interval" env:"INTERVAL" default:"30s" desc:"Interval between health checks"`
	Timeout                  time.Duration                  `json:"timeout" yaml:"timeout" toml:"timeout" env:"TIMEOUT" default:"5s" desc:"Timeout for health check requests"`
	RecentRequestThreshold   time.Duration                  `json:"recent_request_threshold" yaml:"recent_request_threshold" toml:"recent_request_threshold" env:"RECENT_REQUEST_THRESHOLD" default:"60s" desc:"Skip health check if a request to the backend occurred within this time"`
	HealthEndpoints          map[string]string              `json:"health_endpoints" yaml:"health_endpoints" toml:"health_endpoints" env:"HEALTH_ENDPOINTS" desc:"Custom health check endpoints for specific backends (defaults to base URL)"`
	ExpectedStatusCodes      []int                          `json:"expected_status_codes" yaml:"expected_status_codes" toml:"expected_status_codes" env:"EXPECTED_STATUS_CODES" default:"[200]" desc:"HTTP status codes considered healthy"`
	BackendHealthCheckConfig map[string]BackendHealthConfig `json:"backend_health_check_config" yaml:"backend_health_check_config" toml:"backend_health_check_config" desc:"Per-backend health check configuration"`
}

// BackendHealthConfig provides per-backend health check configuration.
type BackendHealthConfig struct {
	Enabled             bool          `json:"enabled" yaml:"enabled" toml:"enabled" env:"ENABLED" default:"true" desc:"Enable health checking for this backend"`
	Endpoint            string        `json:"endpoint" yaml:"endpoint" toml:"endpoint" env:"ENDPOINT" desc:"Custom health check endpoint (defaults to base URL)"`
	Interval            time.Duration `json:"interval" yaml:"interval" toml:"interval" env:"INTERVAL" desc:"Override global interval for this backend"`
	Timeout             time.Duration `json:"timeout" yaml:"timeout" toml:"timeout" env:"TIMEOUT" desc:"Override global timeout for this backend"`
	ExpectedStatusCodes []int         `json:"expected_status_codes" yaml:"expected_status_codes" toml:"expected_status_codes" env:"EXPECTED_STATUS_CODES" desc:"Override global expected status codes for this backend"`
}
