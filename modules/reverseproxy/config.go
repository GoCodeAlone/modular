// Package reverseproxy provides configuration structures for the reverse proxy module.
package reverseproxy

import "time"

// ReverseProxyConfig provides configuration options for the ReverseProxyModule.
type ReverseProxyConfig struct {
	BackendServices        map[string]string               `json:"backend_services" yaml:"backend_services" toml:"backend_services" env:"BACKEND_SERVICES"`
	Routes                 map[string]string               `json:"routes" yaml:"routes" toml:"routes" env:"ROUTES"`
	RouteConfigs           map[string]RouteConfig          `json:"route_configs" yaml:"route_configs" toml:"route_configs"`
	DefaultBackend         string                          `json:"default_backend" yaml:"default_backend" toml:"default_backend" env:"DEFAULT_BACKEND"`
	CircuitBreakerConfig   CircuitBreakerConfig            `json:"circuit_breaker" yaml:"circuit_breaker" toml:"circuit_breaker"`
	BackendCircuitBreakers map[string]CircuitBreakerConfig `json:"backend_circuit_breakers" yaml:"backend_circuit_breakers" toml:"backend_circuit_breakers"`
	CompositeRoutes        map[string]CompositeRoute       `json:"composite_routes" yaml:"composite_routes" toml:"composite_routes"`
	TenantIDHeader         string                          `json:"tenant_id_header" yaml:"tenant_id_header" toml:"tenant_id_header" env:"TENANT_ID_HEADER" default:"X-Tenant-ID"`
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

	// Debug endpoints configuration
	DebugEndpoints DebugEndpointsConfig `json:"debug_endpoints" yaml:"debug_endpoints" toml:"debug_endpoints"`

	// Dry-run configuration
	DryRun DryRunConfig `json:"dry_run" yaml:"dry_run" toml:"dry_run"`

	// Feature flag configuration
	FeatureFlags FeatureFlagsConfig `json:"feature_flags" yaml:"feature_flags" toml:"feature_flags"`

	// Global timeout configuration
	GlobalTimeout time.Duration `json:"global_timeout" yaml:"global_timeout" toml:"global_timeout" env:"GLOBAL_TIMEOUT"`

	// Metrics configuration
	MetricsConfig MetricsConfig `json:"metrics_config" yaml:"metrics_config" toml:"metrics_config"`

	// Debug configuration
	DebugConfig DebugConfig `json:"debug_config" yaml:"debug_config" toml:"debug_config"`

	// Dry run configuration
	DryRunConfig DryRunConfig `json:"dry_run_config" yaml:"dry_run_config" toml:"dry_run_config"`

	// Header management
	HeaderConfig HeaderConfig `json:"header_config" yaml:"header_config" toml:"header_config"`

	// Response header management
	ResponseHeaderConfig ResponseHeaderRewritingConfig `json:"response_header_config" yaml:"response_header_config" toml:"response_header_config"`

	// Error handling configuration
	ErrorHandling ErrorHandlingConfig `json:"error_handling" yaml:"error_handling" toml:"error_handling"`
}

// RouteConfig defines feature flag-controlled routing configuration for specific routes.
// This allows routes to be dynamically controlled by feature flags, with fallback to alternative backends.
type RouteConfig struct {
	// FeatureFlagID is the ID of the feature flag that controls whether this route uses the primary backend
	// If specified and the feature flag evaluates to false, requests will be routed to the alternative backend
	FeatureFlagID string `json:"feature_flag_id" yaml:"feature_flag_id" toml:"feature_flag_id" env:"FEATURE_FLAG_ID"`

	// FeatureFlag is an alternative name for FeatureFlagID
	FeatureFlag string `json:"feature_flag" yaml:"feature_flag" toml:"feature_flag" env:"FEATURE_FLAG"`

	// AlternativeBackend specifies the backend to use when the feature flag is disabled
	// If FeatureFlagID is specified and evaluates to false, requests will be routed to this backend instead
	AlternativeBackend string `json:"alternative_backend" yaml:"alternative_backend" toml:"alternative_backend" env:"ALTERNATIVE_BACKEND"`

	// AlternativeBackends is a list of alternative backends
	AlternativeBackends []string `json:"alternative_backends" yaml:"alternative_backends" toml:"alternative_backends" env:"ALTERNATIVE_BACKENDS"`

	// CompositeBackends defines multiple backends for composite responses
	CompositeBackends []string `json:"composite_backends" yaml:"composite_backends" toml:"composite_backends" env:"COMPOSITE_BACKENDS"`

	// PathRewrite defines path rewriting for this route
	PathRewrite string `json:"path_rewrite" yaml:"path_rewrite" toml:"path_rewrite" env:"PATH_REWRITE"`

	// Timeout defines a custom timeout for this route
	Timeout time.Duration `json:"timeout" yaml:"timeout" toml:"timeout" env:"TIMEOUT"`

	// DryRun enables dry-run mode for this route, sending requests to both backends and comparing responses
	// When true, requests are sent to both the primary and alternative backends, but only the alternative backend's response is returned
	DryRun bool `json:"dry_run" yaml:"dry_run" toml:"dry_run" env:"DRY_RUN"`

	// DryRunBackend specifies the backend to compare against in dry-run mode
	// If not specified, uses the AlternativeBackend for comparison
	DryRunBackend string `json:"dry_run_backend" yaml:"dry_run_backend" toml:"dry_run_backend" env:"DRY_RUN_BACKEND"`
}

// CompositeRoute defines a route that combines responses from multiple backends.
type CompositeRoute struct {
	Pattern  string   `json:"pattern" yaml:"pattern" toml:"pattern" env:"PATTERN"`
	Backends []string `json:"backends" yaml:"backends" toml:"backends" env:"BACKENDS"`
	Strategy string   `json:"strategy" yaml:"strategy" toml:"strategy" env:"STRATEGY"`

	// EmptyPolicy defines how empty backend responses are handled.
	// Valid values: "allow-empty" (default), "skip-empty", "fail-on-empty".
	// This is used by pipeline and fan-out-merge strategies.
	EmptyPolicy string `json:"empty_policy" yaml:"empty_policy" toml:"empty_policy" env:"EMPTY_POLICY"`

	// FeatureFlagID is the ID of the feature flag that controls whether this composite route is enabled
	// If specified and the feature flag evaluates to false, this route will return 404
	FeatureFlagID string `json:"feature_flag_id" yaml:"feature_flag_id" toml:"feature_flag_id" env:"FEATURE_FLAG_ID"`

	// AlternativeBackend specifies an alternative single backend to use when the feature flag is disabled
	// If FeatureFlagID is specified and evaluates to false, requests will be routed to this backend instead
	AlternativeBackend string `json:"alternative_backend" yaml:"alternative_backend" toml:"alternative_backend" env:"ALTERNATIVE_BACKEND"`
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

	// Simple path rewrite for backwards compatibility
	PathRewrite string `json:"path_rewrite" yaml:"path_rewrite" toml:"path_rewrite" env:"PATH_REWRITE"`

	// HeaderRewriting defines header rewriting rules specific to this backend
	HeaderRewriting HeaderRewritingConfig `json:"header_rewriting" yaml:"header_rewriting" toml:"header_rewriting"`

	// ResponseHeaderRewriting defines response header rewriting rules specific to this backend
	ResponseHeaderRewriting ResponseHeaderRewritingConfig `json:"response_header_rewriting" yaml:"response_header_rewriting" toml:"response_header_rewriting"`

	// Hostname handling mode for this backend
	HostnameHandling string `json:"hostname_handling" yaml:"hostname_handling" toml:"hostname_handling" env:"HOSTNAME_HANDLING"`

	// Custom hostname to use for this backend
	CustomHostname string `json:"custom_hostname" yaml:"custom_hostname" toml:"custom_hostname" env:"CUSTOM_HOSTNAME"`

	// Endpoints defines endpoint-specific configurations
	Endpoints map[string]EndpointConfig `json:"endpoints" yaml:"endpoints" toml:"endpoints"`

	// FeatureFlagID is the ID of the feature flag that controls whether this backend is enabled
	// If specified and the feature flag evaluates to false, requests to this backend will fail or use alternative
	FeatureFlagID string `json:"feature_flag_id" yaml:"feature_flag_id" toml:"feature_flag_id" env:"FEATURE_FLAG_ID"`

	// FeatureFlag is an alternative name for FeatureFlagID
	FeatureFlag string `json:"feature_flag" yaml:"feature_flag" toml:"feature_flag" env:"FEATURE_FLAG"`

	// AlternativeBackend specifies an alternative backend to use when the feature flag is disabled
	// If FeatureFlagID is specified and evaluates to false, requests will be routed to this backend instead
	AlternativeBackend string `json:"alternative_backend" yaml:"alternative_backend" toml:"alternative_backend" env:"ALTERNATIVE_BACKEND"`

	// AlternativeBackends is a list of alternative backends
	AlternativeBackends []string `json:"alternative_backends" yaml:"alternative_backends" toml:"alternative_backends" env:"ALTERNATIVE_BACKENDS"`

	// Health check configuration
	HealthCheck    BackendHealthCheckConfig `json:"health_check" yaml:"health_check" toml:"health_check"`
	HealthEndpoint string                   `json:"health_endpoint" yaml:"health_endpoint" toml:"health_endpoint" env:"HEALTH_ENDPOINT"`

	// Circuit breaker configuration
	CircuitBreaker BackendCircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker" toml:"circuit_breaker"`

	// Retry configuration
	MaxRetries int           `json:"max_retries" yaml:"max_retries" toml:"max_retries" env:"MAX_RETRIES"`
	RetryDelay time.Duration `json:"retry_delay" yaml:"retry_delay" toml:"retry_delay" env:"RETRY_DELAY"`

	// Connection pool configuration
	MaxConnections    int           `json:"max_connections" yaml:"max_connections" toml:"max_connections" env:"MAX_CONNECTIONS"`
	ConnectionTimeout time.Duration `json:"connection_timeout" yaml:"connection_timeout" toml:"connection_timeout" env:"CONNECTION_TIMEOUT"`
	IdleTimeout       time.Duration `json:"idle_timeout" yaml:"idle_timeout" toml:"idle_timeout" env:"IDLE_TIMEOUT"`

	// Queue configuration
	QueueSize    int           `json:"queue_size" yaml:"queue_size" toml:"queue_size" env:"QUEUE_SIZE"`
	QueueTimeout time.Duration `json:"queue_timeout" yaml:"queue_timeout" toml:"queue_timeout" env:"QUEUE_TIMEOUT"`
}

// EndpointConfig defines configuration for a specific endpoint within a backend service.
type EndpointConfig struct {
	// Pattern is the URL pattern that this endpoint matches (e.g., "/api/v1/users/*")
	Pattern string `json:"pattern" yaml:"pattern" toml:"pattern" env:"PATTERN"`

	// PathRewriting defines path rewriting rules specific to this endpoint
	PathRewriting PathRewritingConfig `json:"path_rewriting" yaml:"path_rewriting" toml:"path_rewriting"`

	// HeaderRewriting defines header rewriting rules specific to this endpoint
	HeaderRewriting HeaderRewritingConfig `json:"header_rewriting" yaml:"header_rewriting" toml:"header_rewriting"`

	// ResponseHeaderRewriting defines response header rewriting rules specific to this endpoint
	ResponseHeaderRewriting ResponseHeaderRewritingConfig `json:"response_header_rewriting" yaml:"response_header_rewriting" toml:"response_header_rewriting"`

	// FeatureFlagID is the ID of the feature flag that controls whether this endpoint is enabled
	// If specified and the feature flag evaluates to false, this endpoint will be skipped
	FeatureFlagID string `json:"feature_flag_id" yaml:"feature_flag_id" toml:"feature_flag_id" env:"FEATURE_FLAG_ID"`

	// AlternativeBackend specifies an alternative backend to use when the feature flag is disabled
	// If FeatureFlagID is specified and evaluates to false, requests will be routed to this backend instead
	AlternativeBackend string `json:"alternative_backend" yaml:"alternative_backend" toml:"alternative_backend" env:"ALTERNATIVE_BACKEND"`
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

// ResponseHeaderRewritingConfig defines configuration for response header rewriting rules.
type ResponseHeaderRewritingConfig struct {
	// SetHeaders defines headers to set or override on the response
	SetHeaders map[string]string `json:"set_headers" yaml:"set_headers" toml:"set_headers"`

	// RemoveHeaders defines headers to remove from the response
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

// CircuitBreakerConfig provides configuration for the circuit breaker.
type CircuitBreakerConfig struct {
	Enabled                 bool          `json:"enabled" yaml:"enabled" toml:"enabled" env:"ENABLED"`
	FailureThreshold        int           `json:"failure_threshold" yaml:"failure_threshold" toml:"failure_threshold" env:"FAILURE_THRESHOLD"`
	SuccessThreshold        int           `json:"success_threshold" yaml:"success_threshold" toml:"success_threshold" env:"SUCCESS_THRESHOLD"`
	OpenTimeout             time.Duration `json:"open_timeout" yaml:"open_timeout" toml:"open_timeout" env:"OPEN_TIMEOUT"`
	RequestTimeout          time.Duration `json:"request_timeout" yaml:"request_timeout" toml:"request_timeout" env:"REQUEST_TIMEOUT"`
	HalfOpenAllowedRequests int           `json:"half_open_allowed_requests" yaml:"half_open_allowed_requests" toml:"half_open_allowed_requests" env:"HALF_OPEN_ALLOWED_REQUESTS"`
	WindowSize              int           `json:"window_size" yaml:"window_size" toml:"window_size" env:"WINDOW_SIZE"`
	SuccessRateThreshold    float64       `json:"success_rate_threshold" yaml:"success_rate_threshold" toml:"success_rate_threshold" env:"SUCCESS_RATE_THRESHOLD"`
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

// FeatureFlagsConfig provides configuration for the built-in feature flag evaluator.
type FeatureFlagsConfig struct {
	// Enabled determines whether to create and expose the built-in FileBasedFeatureFlagEvaluator service
	Enabled bool `json:"enabled" yaml:"enabled" toml:"enabled" env:"ENABLED" default:"false" desc:"Enable the built-in file-based feature flag evaluator service"`

	// Flags defines default values for feature flags. Tenant-specific overrides come from tenant config files.
	Flags map[string]bool `json:"flags" yaml:"flags" toml:"flags" desc:"Default values for feature flags"`
}

// MetricsConfig provides configuration for metrics collection.
type MetricsConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled" toml:"enabled" env:"ENABLED" default:"false" desc:"Enable metrics collection"`
	Endpoint string `json:"endpoint" yaml:"endpoint" toml:"endpoint" env:"ENDPOINT" default:"/metrics" desc:"Metrics endpoint path"`
}

// DebugConfig provides configuration for debug endpoints.
type DebugConfig struct {
	Enabled                 bool   `json:"enabled" yaml:"enabled" toml:"enabled" env:"ENABLED" default:"false" desc:"Enable debug endpoints"`
	InfoEndpoint            string `json:"info_endpoint" yaml:"info_endpoint" toml:"info_endpoint" env:"INFO_ENDPOINT" default:"/debug/info" desc:"Debug info endpoint path"`
	BackendsEndpoint        string `json:"backends_endpoint" yaml:"backends_endpoint" toml:"backends_endpoint" env:"BACKENDS_ENDPOINT" default:"/debug/backends" desc:"Debug backends endpoint path"`
	FlagsEndpoint           string `json:"flags_endpoint" yaml:"flags_endpoint" toml:"flags_endpoint" env:"FLAGS_ENDPOINT" default:"/debug/flags" desc:"Debug feature flags endpoint path"`
	CircuitBreakersEndpoint string `json:"circuit_breakers_endpoint" yaml:"circuit_breakers_endpoint" toml:"circuit_breakers_endpoint" env:"CIRCUIT_BREAKERS_ENDPOINT" default:"/debug/circuit-breakers" desc:"Debug circuit breakers endpoint path"`
	HealthChecksEndpoint    string `json:"health_checks_endpoint" yaml:"health_checks_endpoint" toml:"health_checks_endpoint" env:"HEALTH_CHECKS_ENDPOINT" default:"/debug/health-checks" desc:"Debug health checks endpoint path"`
}

// HeaderConfig provides configuration for header management.
type HeaderConfig struct {
	SetHeaders    map[string]string `json:"set_headers" yaml:"set_headers" toml:"set_headers" desc:"Headers to set on requests"`
	RemoveHeaders []string          `json:"remove_headers" yaml:"remove_headers" toml:"remove_headers" desc:"Headers to remove from requests"`
}

// ErrorHandlingConfig provides configuration for error handling.
type ErrorHandlingConfig struct {
	EnableCustomPages bool          `json:"enable_custom_pages" yaml:"enable_custom_pages" toml:"enable_custom_pages" env:"ENABLE_CUSTOM_PAGES" default:"false" desc:"Enable custom error pages"`
	RetryAttempts     int           `json:"retry_attempts" yaml:"retry_attempts" toml:"retry_attempts" env:"RETRY_ATTEMPTS" default:"0" desc:"Number of retry attempts for failed requests"`
	ConnectionRetries int           `json:"connection_retries" yaml:"connection_retries" toml:"connection_retries" env:"CONNECTION_RETRIES" default:"0" desc:"Number of connection retry attempts"`
	RetryDelay        time.Duration `json:"retry_delay" yaml:"retry_delay" toml:"retry_delay" env:"RETRY_DELAY" default:"1s" desc:"Delay between retry attempts"`
}

// BackendHealthCheckConfig provides per-backend health check configuration.
type BackendHealthCheckConfig struct {
	Enabled             bool          `json:"enabled" yaml:"enabled" toml:"enabled" env:"ENABLED" default:"true" desc:"Enable health checking for this backend"`
	Interval            time.Duration `json:"interval" yaml:"interval" toml:"interval" env:"INTERVAL" desc:"Health check interval"`
	Timeout             time.Duration `json:"timeout" yaml:"timeout" toml:"timeout" env:"TIMEOUT" desc:"Health check timeout"`
	ExpectedStatusCodes []int         `json:"expected_status_codes" yaml:"expected_status_codes" toml:"expected_status_codes" env:"EXPECTED_STATUS_CODES" desc:"Expected status codes for health check"`
}

// BackendCircuitBreakerConfig provides per-backend circuit breaker configuration.
type BackendCircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled" yaml:"enabled" toml:"enabled" env:"ENABLED" default:"false" desc:"Enable circuit breaker for this backend"`
	FailureThreshold int           `json:"failure_threshold" yaml:"failure_threshold" toml:"failure_threshold" env:"FAILURE_THRESHOLD" default:"5" desc:"Number of failures before opening circuit"`
	RecoveryTimeout  time.Duration `json:"recovery_timeout" yaml:"recovery_timeout" toml:"recovery_timeout" env:"RECOVERY_TIMEOUT" default:"60s" desc:"Time to wait before attempting recovery"`
}
