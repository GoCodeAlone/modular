// Package reverseproxy provides a flexible reverse proxy module with support for multiple backends,
// composite responses, and tenant awareness.
package reverseproxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"path"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/CrisisTextLine/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/gobwas/glob"
)

// ReverseProxyModule provides a modular reverse proxy implementation with support for
// multiple backends, composite routes that combine responses from different backends,
// and tenant-specific routing configurations.
//
// The module implements the following interfaces:
//   - modular.Module: Basic module lifecycle
//   - modular.Configurable: Configuration management
//   - modular.ServiceAware: Service dependency management
//   - modular.TenantAwareModule: Tenant lifecycle management
//   - modular.Startable: Startup logic
//   - modular.Stoppable: Shutdown logic
//
// Key features include:
//   - Multi-backend proxy routing with health checks
//   - Composite responses combining multiple backend calls
//   - Circuit breakers for fault tolerance
//   - Response caching for performance optimization
//   - Tenant-aware routing and configuration
//   - Request/response transformation pipelines
//   - Comprehensive metrics collection
//   - Path-based and header-based routing rules
type ReverseProxyModule struct {
	config          *ReverseProxyConfig
	router          routerService
	httpClient      *http.Client
	backendProxies  map[string]*httputil.ReverseProxy
	backendRoutes   map[string]map[string]http.HandlerFunc
	compositeRoutes map[string]http.HandlerFunc
	defaultBackend  string
	app             modular.Application
	tenantApp       modular.TenantApplication
	responseCache   *responseCache
	circuitBreakers map[string]*CircuitBreaker
	directorFactory func(backend string, tenant modular.TenantID) func(*http.Request)

	tenants              map[modular.TenantID]*ReverseProxyConfig
	tenantBackendProxies map[modular.TenantID]map[string]*httputil.ReverseProxy
	preProxyTransforms   map[string]func(*http.Request)

	// Response header modification callback
	responseHeaderModifier func(*http.Response, string, modular.TenantID) error

	// Response transformers for composite routes (keyed by route pattern)
	responseTransformers map[string]ResponseTransformer

	// Pipeline configurations for composite routes (keyed by route pattern)
	pipelineConfigs map[string]*PipelineConfig

	// Fan-out merger functions for composite routes (keyed by route pattern)
	fanOutMergers map[string]FanOutMerger

	// Empty response policies for composite routes (keyed by route pattern)
	emptyResponsePolicies map[string]EmptyResponsePolicy

	// Metrics collection
	metrics       *MetricsCollector
	enableMetrics bool

	// Health checking
	healthChecker *HealthChecker

	// Feature flag evaluation
	featureFlagEvaluator FeatureFlagEvaluator
	// Track whether the evaluator was provided externally or created internally
	featureFlagEvaluatorProvided bool

	// Dry run handling
	dryRunHandler *DryRunHandler

	// Event observation
	subject modular.Subject

	// Load balancing (simple round-robin) support
	loadBalanceCounters map[string]int // key: backend group spec string (comma-separated)
	loadBalanceMutex    sync.Mutex

	// Synchronization for concurrent map access
	backendProxiesMutex sync.RWMutex
	tenantProxiesMutex  sync.RWMutex

	// Tracks whether Init has completed; used to suppress backend.added events during initial load
	initialized bool
}

// Compile-time assertions to ensure interface compliance
var _ modular.Module = (*ReverseProxyModule)(nil)
var _ modular.Constructable = (*ReverseProxyModule)(nil)
var _ modular.ServiceAware = (*ReverseProxyModule)(nil)
var _ modular.TenantAwareModule = (*ReverseProxyModule)(nil)
var _ modular.Startable = (*ReverseProxyModule)(nil)
var _ modular.Stoppable = (*ReverseProxyModule)(nil)

// singleValueHTTPHeaders defines HTTP headers that should only have a single value
// based on HTTP specifications and common practice. When copying response headers
// from backends, duplicate values for these headers will be merged using the last value.
//
// This prevents issues where backends incorrectly send duplicate header values,
// particularly for CORS headers which can cause browser errors when duplicated.
//
// Multi-value headers like Set-Cookie and Link are intentionally excluded from this
// list and will preserve all values when copied.
var singleValueHTTPHeaders = map[string]bool{
	// CORS headers (W3C CORS Specification, RFC 6454)
	// Browsers reject responses with duplicate CORS headers
	"Access-Control-Allow-Origin":      true,
	"Access-Control-Allow-Methods":     true,
	"Access-Control-Allow-Headers":     true,
	"Access-Control-Allow-Credentials": true,
	"Access-Control-Expose-Headers":    true,
	"Access-Control-Max-Age":           true,

	// Content headers (RFC 7231 - HTTP/1.1 Semantics and Content)
	// These define the representation and should be unique
	"Content-Type":     true,
	"Content-Length":   true,
	"Content-Encoding": true,
	"Content-Language": true,

	// Navigation and caching (RFC 7231, RFC 7234)
	"Location":      true, // Redirect target
	"Server":        true, // Server identification
	"Date":          true, // Message origination date
	"Etag":          true, // Entity tag for caching
	"Last-Modified": true, // Last modification date
	"Expires":       true, // Expiration date
	"Age":           true, // Cache age

	// Custom cache headers
	"X-Cache":      true, // Cache hit/miss indicator
	"X-Cache-Hits": true, // Cache hit counter
}

// NewModule creates a new ReverseProxyModule with default settings.
// This is the primary constructor for the reverseproxy module and should be used
// when registering the module with the application.
//
// The module initializes with:
//   - Optimized HTTP client with connection pooling
//   - Circuit breakers for each backend
//   - Response caching infrastructure
//   - Metrics collection (if enabled)
//   - Thread-safe data structures for concurrent access
//
// Example:
//
//	app.RegisterModule(reverseproxy.NewModule())
func NewModule() *ReverseProxyModule {
	// We'll initialize with a nil client and create it later
	// either in Constructor (if httpclient service is available)
	// or in Init (with default settings)
	module := &ReverseProxyModule{
		httpClient:            nil,
		backendProxies:        make(map[string]*httputil.ReverseProxy),
		backendRoutes:         make(map[string]map[string]http.HandlerFunc),
		compositeRoutes:       make(map[string]http.HandlerFunc),
		tenants:               make(map[modular.TenantID]*ReverseProxyConfig),
		tenantBackendProxies:  make(map[modular.TenantID]map[string]*httputil.ReverseProxy),
		preProxyTransforms:    make(map[string]func(*http.Request)),
		circuitBreakers:       make(map[string]*CircuitBreaker),
		enableMetrics:         true,
		loadBalanceCounters:   make(map[string]int),
		responseTransformers:  make(map[string]ResponseTransformer),
		pipelineConfigs:       make(map[string]*PipelineConfig),
		fanOutMergers:         make(map[string]FanOutMerger),
		emptyResponsePolicies: make(map[string]EmptyResponsePolicy),
	}

	return module
}

// ProvideConfig creates a new default configuration for the reverseproxy module.
// This is used by the modular framework to register the configuration.
func ProvideConfig() interface{} {
	return &ReverseProxyConfig{}
}

// Name returns the name of the module.
// This is used by the modular framework to identify the module.
func (m *ReverseProxyModule) Name() string {
	return "reverseproxy"
}

// RegisterConfig registers the module's configuration with the application.
// It also stores the provided app as a TenantApplication for later use with
// tenant-specific functionality.
func (m *ReverseProxyModule) RegisterConfig(app modular.Application) error {
	// Always store the application reference
	m.app = app

	// Store tenant application if it implements the interface
	if ta, ok := app.(modular.TenantApplication); ok {
		m.tenantApp = ta
	}

	// Bind subject early for events that may be emitted during Init
	if subj, ok := any(app).(modular.Subject); ok {
		m.subject = subj
	}

	// Register the config section only if it doesn't already exist (for BDD tests)
	if _, err := app.GetConfigSection(m.Name()); err != nil {
		// Config section doesn't exist, register a default one
		app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(&ReverseProxyConfig{}))
	}

	return nil
}

// Init initializes the module with the provided application.
// It retrieves the module's configuration and sets up the internal data structures
// for each configured backend, including tenant-specific configurations.
func (m *ReverseProxyModule) Init(app modular.Application) error {
	// Store both interfaces - broader Application for Subject interface, TenantApplication for specific methods
	m.app = app
	if ta, ok := app.(modular.TenantApplication); ok {
		m.tenantApp = ta
	}

	// If observable, opportunistically bind subject for early Init events
	if subj, ok := app.(modular.Subject); ok {
		m.subject = subj
	}

	// Get the config section
	cfg, err := app.GetConfigSection(m.Name())
	if err != nil {
		return fmt.Errorf("failed to get config section '%s': %w", m.Name(), err)
	}

	// Handle both value and pointer types
	configValue := cfg.GetConfig()
	switch v := configValue.(type) {
	case *ReverseProxyConfig:
		m.config = v
	case ReverseProxyConfig:
		m.config = &v
	default:
		return fmt.Errorf("%w: %T", ErrUnexpectedConfigType, v)
	}

	// Validate configuration values
	if err := m.validateConfig(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Initialize metrics collector
	if m.enableMetrics {
		m.metrics = NewMetricsCollector()
		app.Logger().Info("Metrics collection enabled for reverseproxy module")
	}

	// If no HTTP client was set via the httpclient service in Constructor,
	// create a default one here
	if m.httpClient == nil {
		// Create a customized transport with connection pooling settings
		transport := &http.Transport{
			MaxIdleConns:        100,              // Maximum number of idle connections across all hosts
			MaxIdleConnsPerHost: 10,               // Maximum number of idle connections per host
			IdleConnTimeout:     90 * time.Second, // How long to keep idle connections alive
			TLSHandshakeTimeout: 10 * time.Second, // Maximum time for TLS handshake
			DisableCompression:  false,            // Enable compression by default
		}

		// Configure the HTTP client with the transport
		// Note: No client-level timeout is set here. Instead, we use per-request
		// context timeouts which allow different routes to have different timeout values.
		m.httpClient = &http.Client{
			Transport: transport,
		}

		app.Logger().Debug("Using default HTTP client (no httpclient service available)")
	} else {
		app.Logger().Debug("Using HTTP client from httpclient service")
	}

	// Load tenant configs early to ensure we create all necessary backends
	m.loadTenantConfigs()

	// Create global backend proxies
	for backendID, serviceURL := range m.config.BackendServices {
		// Skip empty URLs (might be overridden by tenants later)
		if serviceURL == "" {
			app.Logger().Debug("Global backend URL is empty", "backend", backendID)
			continue
		}

		// Create reverse proxy for this backend
		if err := m.createBackendProxy(backendID, serviceURL); err != nil {
			app.Logger().Error("Failed to create backend proxy",
				"backend", backendID, "url", serviceURL, "error", err)
			continue
		}

		// Initialize route map for this backend
		// Ensure backendRoutes map is initialized
		if m.backendRoutes == nil {
			m.backendRoutes = make(map[string]map[string]http.HandlerFunc)
		}
		// Initialize route map for this backend
		if _, ok := m.backendRoutes[backendID]; !ok {
			m.backendRoutes[backendID] = make(map[string]http.HandlerFunc)
		}
	}

	// Create tenant-specific backend proxies for any tenants already registered
	// (This handles the rare case of tenants registered before Init, though typically
	// tenants are registered after Init via FileBasedTenantConfigLoader)
	m.createTenantProxies(context.Background())

	// Set default backend for the module
	m.defaultBackend = m.config.DefaultBackend

	// Convert logger to slog.Logger for use in handlers
	var logger *slog.Logger
	if slogLogger, ok := app.Logger().(*slog.Logger); ok {
		logger = slogLogger
	} else {
		// Create a new slog logger if conversion fails
		logger = slog.Default()
	}

	// Initialize health checker if enabled
	if m.config.HealthCheck.Enabled {
		m.healthChecker = NewHealthChecker(
			&m.config.HealthCheck,
			m.config.BackendServices,
			m.httpClient,
			logger,
		)

		// Set up event emitter for health checker
		m.healthChecker.SetEventEmitter(func(eventType string, data map[string]interface{}) {
			m.emitEvent(context.Background(), eventType, data) //nolint:contextcheck // module-level health events are not tied to a request context
		})

		// Set up circuit breaker provider for health checker
		m.healthChecker.SetCircuitBreakerProvider(func(backendID string) *HealthCircuitBreakerInfo {
			if cb, exists := m.circuitBreakers[backendID]; exists {
				return &HealthCircuitBreakerInfo{
					IsOpen:       cb.IsOpen(),
					State:        cb.GetState().String(),
					FailureCount: cb.GetFailureCount(),
				}
			}
			return nil
		})

		app.Logger().Info("Health checker initialized", "backends", len(m.config.BackendServices))
	}

	// Initialize dry run handler if enabled
	if m.config.DryRun.Enabled {
		m.dryRunHandler = NewDryRunHandler(
			m.config.DryRun,
			m.config.TenantIDHeader,
			logger,
		)
		app.Logger().Debug("Dry run handler initialized")
	}

	// Initialize circuit breakers for all backends if enabled
	if m.config.CircuitBreakerConfig.Enabled {
		for backendID := range m.config.BackendServices {
			// Check for backend-specific circuit breaker config
			var cbConfig CircuitBreakerConfig
			if backendCB, exists := m.config.BackendCircuitBreakers[backendID]; exists {
				cbConfig = backendCB
			} else {
				cbConfig = m.config.CircuitBreakerConfig
			}

			// Use module's request timeout if circuit breaker config doesn't specify one
			if cbConfig.RequestTimeout == 0 && m.config.RequestTimeout > 0 {
				cbConfig.RequestTimeout = m.config.RequestTimeout
			}

			// Create circuit breaker for this backend
			cb := NewCircuitBreakerWithConfig(backendID, cbConfig, m.metrics)
			cb.eventEmitter = func(eventType string, data map[string]interface{}) {
				m.emitEvent(context.Background(), eventType, data) //nolint:contextcheck // circuit breaker transitions occur outside request scope
			}
			m.circuitBreakers[backendID] = cb

			app.Logger().Debug("Initialized circuit breaker", "backend", backendID,
				"failure_threshold", cbConfig.FailureThreshold, "open_timeout", cbConfig.OpenTimeout)
		}
		app.Logger().Info("Circuit breakers initialized", "backends", len(m.circuitBreakers))
	}

	// Event emitter already set during health checker initialization above

	// Emit config loaded event
	m.emitEvent(context.Background(), EventTypeConfigLoaded, map[string]interface{}{ //nolint:contextcheck // configuration lifecycle events have no request context
		"backend_count":            len(m.config.BackendServices),
		"composite_routes_count":   len(m.config.CompositeRoutes),
		"circuit_breakers_enabled": len(m.circuitBreakers) > 0,
		"metrics_enabled":          m.enableMetrics,
		"cache_enabled":            m.config.CacheEnabled,
		"request_timeout":          m.config.RequestTimeout.String(),
	})

	// Initialize response cache if caching is enabled
	if err := m.setupResponseCache(); err != nil {
		return fmt.Errorf("failed to setup response cache: %w", err)
	}

	// Mark initialization complete so subsequent dynamic backend additions emit events
	m.initialized = true

	return nil
}

// validateConfig validates the module configuration.
// It checks for valid URLs, timeout values, and other configuration parameters.
func (m *ReverseProxyModule) validateConfig() error {
	// If no config, return error
	if m.config == nil {
		return ErrConfigurationNil
	}

	// Set default request timeout if not specified
	if m.config.RequestTimeout <= 0 {
		m.config.RequestTimeout = 10 * time.Second
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Debug("Using default request timeout", "timeout", m.config.RequestTimeout.String())
		}
	}

	// Configure metrics based on configuration
	m.enableMetrics = m.config.MetricsEnabled
	if m.enableMetrics && m.config.MetricsEndpoint == "" {
		// Set default metrics endpoint if metrics are enabled but no endpoint specified
		m.config.MetricsEndpoint = "/metrics"
	}

	// Validate backend service URLs (parse but don't connect)
	for backendID, serviceURL := range m.config.BackendServices {
		if serviceURL == "" {
			// Empty URLs are allowed but logged as warnings
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Warn("Empty URL for backend service", "backend", backendID)
			}
			continue
		}

		// Try to parse the URL
		_, err := url.Parse(serviceURL)
		if err != nil {
			return fmt.Errorf("invalid URL for backend '%s': %s - %w", backendID, serviceURL, err)
		}
	}

	// Validate default backend is defined if specified
	if m.config.DefaultBackend != "" {
		_, exists := m.config.BackendServices[m.config.DefaultBackend]
		if !exists {
			// The default backend must be defined in the backend services map
			return fmt.Errorf("%w: %s", ErrDefaultBackendNotDefined, m.config.DefaultBackend)
		}

		// Even if the URL is empty in global config, we'll allow it as it might be provided by a tenant
		// We'll only log a warning for empty URLs in the default backend
		if m.config.BackendServices[m.config.DefaultBackend] == "" && m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Warn("Default backend has empty URL in global config, will check for tenant-specific URL during routing",
				"backend", m.config.DefaultBackend)
		}
	}

	// Validate cache settings
	if m.config.CacheEnabled && m.config.CacheTTL <= 0 {
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Warn("Cache is enabled but CacheTTL is not set, using default of 60s")
		}
		m.config.CacheTTL = 60 * time.Second
	}

	// Validate tenant header is set if tenant ID is required
	if m.config.RequireTenantID && m.config.TenantIDHeader == "" {
		return ErrTenantIDRequired
	}

	return nil
}

// Constructor returns a ModuleConstructor function that initializes the module with
// the required services. It expects a service that implements the routerService
// interface to register routes with.
func (m *ReverseProxyModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		// Get the required router service
		handleFuncSvc, ok := services["router"].(routerService)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrServiceNotHandleFunc, "router")
		}
		if handleFuncSvc == nil {
			return nil, fmt.Errorf("%w: router service is nil", ErrServiceNotHandleFunc)
		}
		// Additional safety check for specific router implementations
		if reflect.ValueOf(handleFuncSvc).IsNil() {
			return nil, fmt.Errorf("%w: router service pointer is nil", ErrServiceNotHandleFunc)
		}
		m.router = handleFuncSvc

		// Get the optional httpclient service
		if httpClientInstance, exists := services["httpclient"]; exists {
			if client, ok := httpClientInstance.(*http.Client); ok {
				m.httpClient = client
				app.Logger().Debug("Using HTTP client from httpclient service")
			} else {
				app.Logger().Warn("httpclient service found but is not *http.Client",
					"type", fmt.Sprintf("%T", httpClientInstance))
			}
		}

		// Get the optional feature flag evaluator service
		if featureFlagSvc, exists := services["featureFlagEvaluator"]; exists {
			if evaluator, ok := featureFlagSvc.(FeatureFlagEvaluator); ok {
				m.featureFlagEvaluator = evaluator
				m.featureFlagEvaluatorProvided = true
				app.Logger().Debug("Using feature flag evaluator from service")
			} else {
				app.Logger().Warn("featureFlagEvaluator service found but does not implement FeatureFlagEvaluator",
					"type", fmt.Sprintf("%T", featureFlagSvc))
			}
		}

		// If no HTTP client service was found, we'll create a default one in Init()
		if m.httpClient == nil {
			app.Logger().Debug("No httpclient service available, will create default client")
		}

		return m, nil
	}
}

// Start sets up all routes for the module and registers them with the router.
// This includes backend routes, composite routes, and any custom endpoints.
func (m *ReverseProxyModule) Start(ctx context.Context) error {
	// Ensure configuration is loaded
	if m.config == nil {
		return fmt.Errorf("%w: module may not be properly initialized", ErrConfigurationNotLoaded)
	}

	// Load tenant-specific configurations
	m.loadTenantConfigs()

	// Create tenant-specific backend proxies after loading configs
	// This handles tenants that were registered after Init()
	m.createTenantProxies(ctx)

	// Setup routes for all backends
	if err := m.setupBackendRoutes(); err != nil {
		return err
	}

	// Setup composite routes
	if err := m.setupCompositeRoutes(ctx); err != nil {
		return err
	}

	// Register metrics endpoint if enabled
	if m.enableMetrics && m.metrics != nil && m.config.MetricsEndpoint != "" {
		m.registerMetricsEndpoint(m.config.MetricsEndpoint)
	}

	// Register routes with router
	if err := m.registerRoutes(); err != nil {
		return fmt.Errorf("failed to register routes: %w", err)
	}

	// Register debug endpoints if enabled
	if m.config.DebugEndpoints.Enabled {
		if err := m.registerDebugEndpoints(); err != nil {
			return fmt.Errorf("failed to register debug endpoints: %w", err)
		}
	}

	// Set up feature flag evaluation using aggregator pattern
	if err := m.setupFeatureFlagEvaluation(ctx); err != nil {
		return fmt.Errorf("failed to set up feature flag evaluation: %w", err)
	}

	// Initialize response cache if enabled
	if err := m.setupResponseCache(); err != nil {
		return fmt.Errorf("failed to set up response cache: %w", err)
	}

	// Start health checker if enabled
	if m.healthChecker != nil {
		if err := m.healthChecker.Start(ctx); err != nil {
			return fmt.Errorf("failed to start health checker: %w", err)
		}
	}

	// Emit module started event
	m.emitEvent(ctx, EventTypeModuleStarted, map[string]interface{}{
		"backend_count":          len(m.config.BackendServices),
		"composite_routes_count": len(m.config.CompositeRoutes),
		"health_checker_enabled": m.healthChecker != nil,
		"metrics_enabled":        m.enableMetrics,
	})

	// Emit proxy started event
	m.emitEvent(ctx, EventTypeProxyStarted, map[string]interface{}{
		"backend_count":  len(m.config.BackendServices),
		"server_running": true,
	})

	return nil
}

// Stop performs any cleanup needed when stopping the module.
// This method gracefully shuts down active connections and resources.
func (m *ReverseProxyModule) Stop(ctx context.Context) error {
	// Log that we're shutting down
	if m.app != nil && m.app.Logger() != nil {
		m.app.Logger().Info("Shutting down reverseproxy module")
	}

	// Stop health checker if running
	if m.healthChecker != nil {
		m.healthChecker.Stop(ctx)
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Debug("Health checker stopped")
		}
	}

	// If we have an HTTP client with a Transport, close idle connections
	if m.httpClient != nil && m.httpClient.Transport != nil {
		// Type assertion to access CloseIdleConnections method
		if transport, ok := m.httpClient.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Debug("Closed idle connections in reverseproxy module")
			}
		}
	}

	// Clean up the response cache if it exists
	if m.responseCache != nil {
		m.responseCache.cleanup()
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Debug("Cleaned up response cache in reverseproxy module")
		}
	}

	// Reset all internal state maps to release memory
	m.compositeRoutes = make(map[string]http.HandlerFunc)
	m.backendRoutes = make(map[string]map[string]http.HandlerFunc)

	// Reset circuit breakers
	for id := range m.circuitBreakers {
		if cb := m.circuitBreakers[id]; cb != nil {
			cb.reset()
		}
	}
	m.circuitBreakers = make(map[string]*CircuitBreaker)

	// Clear proxy references
	m.backendProxiesMutex.Lock()
	m.backendProxies = make(map[string]*httputil.ReverseProxy)
	m.backendProxiesMutex.Unlock()
	m.tenantProxiesMutex.Lock()
	for tenantId := range m.tenantBackendProxies {
		m.tenantBackendProxies[tenantId] = make(map[string]*httputil.ReverseProxy)
	}
	m.tenantProxiesMutex.Unlock()

	// Keep tenant configs but clear proxies
	for tenantID := range m.tenants {
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Debug("Cleaned up resources for tenant", "tenant", tenantID)
		}
	}

	// Emit proxy stopped event
	backendCount := 0
	if m.config != nil && m.config.BackendServices != nil {
		backendCount = len(m.config.BackendServices)
	}
	m.emitEvent(ctx, EventTypeProxyStopped, map[string]interface{}{
		"backend_count":  backendCount,
		"server_running": false,
	})

	// Emit module stopped event
	m.emitEvent(ctx, EventTypeModuleStopped, map[string]interface{}{
		"cleanup_complete": true,
	})

	if m.app != nil && m.app.Logger() != nil {
		m.app.Logger().Info("Reverseproxy module shutdown complete")
	}
	return nil
}

// OnTenantRegistered is called when a new tenant is registered with the application.
// Instead of immediately querying for tenant configuration, we store the tenant ID
// and defer configuration loading until the next appropriate phase to avoid deadlocks.
func (m *ReverseProxyModule) OnTenantRegistered(tenantID modular.TenantID) {
	// Store the tenant ID first, defer config loading to avoid deadlock
	// The actual configuration will be loaded in Start() or when needed
	m.tenants[tenantID] = nil

	// Check if app is available (module might not be fully initialized yet)
	if m.app != nil && m.app.Logger() != nil {
		m.app.Logger().Debug("Tenant registered with reverseproxy module", "tenantID", tenantID)
	}
}

// loadTenantConfigs loads all tenant-specific configurations.
// This should be called during Start() or another safe phase after tenant registration.
func (m *ReverseProxyModule) loadTenantConfigs() {
	if m.app != nil && m.app.Logger() != nil {
		m.app.Logger().Debug("Loading tenant configs", "count", len(m.tenants))
	}

	// Ensure we have a tenant application reference (tests may call this before Init)
	ta := m.tenantApp
	if ta == nil {
		if cast, ok := any(m.app).(modular.TenantApplication); ok {
			ta = cast
			m.tenantApp = cast
		} else {
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Warn("Tenant application not available; skipping tenant config load")
			}
			return
		}
	}
	for tenantID := range m.tenants {
		cp, err := ta.GetTenantConfig(tenantID, m.Name())
		if err != nil {
			m.app.Logger().Error("Failed to get config for tenant", "tenant", tenantID, "module", m.Name(), "error", err)
			continue
		}

		tenantCfg, ok := cp.GetConfig().(*ReverseProxyConfig)
		if !ok {
			m.app.Logger().Error("Failed to cast config for tenant", "tenant", tenantID, "module", m.Name())
			continue
		}

		// Merge the tenant config with the global config
		mergedCfg := mergeConfigs(m.config, tenantCfg)

		// Store the merged configuration
		m.tenants[tenantID] = mergedCfg
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Debug("Loaded and merged tenant config", "tenantID", tenantID, "defaultBackend", mergedCfg.DefaultBackend)
		}
	}
}

// createTenantProxies creates reverse proxies for all tenant-specific backend services.
// This method should be called after loadTenantConfigs() to ensure tenant proxies are
// created for any tenants that were registered after Init() was called.
func (m *ReverseProxyModule) createTenantProxies(ctx context.Context) {
	for tenantID, tenantCfg := range m.tenants {
		if tenantCfg == nil || tenantCfg.BackendServices == nil {
			continue
		}

		// Process each backend in tenant config
		for backendID, serviceURL := range tenantCfg.BackendServices {
			// Skip if URL is not provided
			if serviceURL == "" {
				continue
			}

			// Check if proxy already exists - use write lock to avoid race with test setup
			m.tenantProxiesMutex.Lock()
			tenantProxies, tenantMapExists := m.tenantBackendProxies[tenantID]
			proxyExists := tenantMapExists && tenantProxies != nil && tenantProxies[backendID] != nil

			if proxyExists {
				// Proxy already exists, skip creation
				m.tenantProxiesMutex.Unlock()
				continue
			}

			// No proxy exists, unlock to create it (avoid holding lock during creation)
			m.tenantProxiesMutex.Unlock()

			// Create a new proxy for this tenant's backend
			backendURL, err := url.Parse(serviceURL)
			if err != nil {
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Error("Failed to parse tenant backend URL",
						"tenant", tenantID, "backend", backendID, "url", serviceURL, "error", err)
				}
				continue
			}

			proxy := m.createReverseProxyForBackend(ctx, backendURL, backendID, "")

			// Re-acquire lock and double-check before storing (double-checked locking pattern)
			m.tenantProxiesMutex.Lock()
			if _, exists := m.tenantBackendProxies[tenantID]; !exists {
				m.tenantBackendProxies[tenantID] = make(map[string]*httputil.ReverseProxy)
			}

			// Check again in case another goroutine/test created it while we were unlocked
			if m.tenantBackendProxies[tenantID][backendID] == nil {
				// Store the tenant-specific proxy only if still nil
				m.tenantBackendProxies[tenantID][backendID] = proxy
			}
			m.tenantProxiesMutex.Unlock()

			// If there's no global URL for this backend, create one in the global map
			// BUT only if we actually stored a tenant proxy (not if test overrode it)
			m.tenantProxiesMutex.RLock()
			actuallyStoredTenantProxy := m.tenantBackendProxies[tenantID] != nil && m.tenantBackendProxies[tenantID][backendID] == proxy
			m.tenantProxiesMutex.RUnlock()

			backendWasAdded := false
			if actuallyStoredTenantProxy {
				m.backendProxiesMutex.Lock()
				if _, exists := m.backendProxies[backendID]; !exists {
					if m.app != nil && m.app.Logger() != nil {
						m.app.Logger().Debug("Using tenant-specific backend URL as global",
							"tenant_hash", obfuscateTenantID(tenantID), "backend", backendID, "url", serviceURL)
					}
					m.backendProxies[backendID] = proxy
					backendWasAdded = true
				}
				m.backendProxiesMutex.Unlock()

				// Emit backend added event only for dynamic additions after initialization
				if backendWasAdded && m.initialized {
					m.emitEvent(ctx, EventTypeBackendAdded, map[string]interface{}{
						"backend": backendID,
						"url":     serviceURL,
						"time":    time.Now().UTC().Format(time.RFC3339Nano),
					})
				}
			}

			// Initialize route map for this backend if needed
			if _, ok := m.backendRoutes[backendID]; !ok {
				m.backendRoutes[backendID] = make(map[string]http.HandlerFunc)
			}

			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Debug("Created tenant-specific proxy",
					"tenant", tenantID, "backend", backendID, "url", serviceURL)
			}
		}
	}
}

// OnTenantRemoved is called when a tenant is removed from the application.
// It removes the tenant's configuration and any associated resources.
func (m *ReverseProxyModule) OnTenantRemoved(tenantID modular.TenantID) {
	// Clean up tenant-specific resources
	delete(m.tenants, tenantID)

	// Check if app is available (module might not be fully initialized yet)
	if m.app != nil && m.app.Logger() != nil {
		m.app.Logger().Info("Tenant removed from reverseproxy module", "tenantID", tenantID)
	}
}

// ProvidesServices returns the services provided by this module.
// This module can provide a featureFlagEvaluator service if configured to do so,
// whether the evaluator was created internally or provided externally.
// This allows other modules to discover and use the evaluator.
func (m *ReverseProxyModule) ProvidesServices() []modular.ServiceProvider {
	var services []modular.ServiceProvider

	// Don't provide any services if config is nil
	if m.config == nil {
		return services
	}

	// Provide the reverse proxy module itself as a service
	services = append(services, modular.ServiceProvider{
		Name:        "reverseproxy.provider",
		Description: "Reverse proxy module providing request routing and load balancing",
		Instance:    m,
	})

	// Provide the feature flag evaluator service if we have one and feature flags are enabled.
	// This includes both internally created and externally provided evaluators so other modules can use them.
	if m.featureFlagEvaluator != nil && m.config.FeatureFlags.Enabled {
		services = append(services, modular.ServiceProvider{
			Name:     "featureFlagEvaluator",
			Instance: m.featureFlagEvaluator,
		})
	}

	return services
}

// routerService defines the interface for a service that can register
// HTTP handlers with URL patterns. This is typically implemented by an HTTP router.
type routerService interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler http.HandlerFunc)
	Mount(pattern string, h http.Handler)
	Use(middlewares ...func(http.Handler) http.Handler)
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// RequiresServices returns the services required by this module.
// The reverseproxy module requires a service that implements the routerService
// interface to register routes with, and optionally a http.Client and FeatureFlagEvaluator.
func (m *ReverseProxyModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "router",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*routerService)(nil)).Elem(),
		},
		{
			Name:               "httpclient",
			Required:           false, // Optional dependency
			MatchByInterface:   false, // Use name-based matching
			SatisfiesInterface: nil,
		},
		{
			Name:               "featureFlagEvaluator",
			Required:           false, // Optional dependency
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*FeatureFlagEvaluator)(nil)).Elem(),
		},
	}
}

// safeHandleFunc safely calls router.HandleFunc with panic recovery
func (m *ReverseProxyModule) safeHandleFunc(pattern string, handler http.HandlerFunc) {
	if m.router == nil {
		fmt.Printf("WARNING: attempted to register route '%s' but router is nil\n", pattern)
		return
	}
	if handler == nil {
		fmt.Printf("WARNING: attempted to register nil handler for pattern '%s'\n", pattern)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("WARNING: router.HandleFunc panicked for pattern '%s': %v\n", pattern, r)
		}
	}()

	// Triple-check router is still not nil and not a nil interface before calling
	if m.router != nil && !reflect.ValueOf(m.router).IsNil() {
		// Additional safety check: ensure router has HandleFunc method available
		if routerVal := reflect.ValueOf(m.router); routerVal.IsValid() && !routerVal.IsNil() {
			m.router.HandleFunc(pattern, handler)
		} else {
			fmt.Printf("WARNING: router value is invalid for pattern '%s'\n", pattern)
		}
	} else {
		fmt.Printf("WARNING: router became nil while trying to register pattern '%s'\n", pattern)
	}
}

// setupBackendRoutes sets up routes for all configured backends.
// For each backend with a valid URL, it registers a default catch-all route.
func (m *ReverseProxyModule) setupBackendRoutes() error {
	for backendID, proxy := range m.backendProxies {
		// Skip if URL is not provided
		if proxy == nil {
			continue
		}

		// Register default route for this backend (catch-all)
		defaultRoute := "/*"
		m.registerBackendRoute(backendID, defaultRoute)
	}

	return nil
}

// registerBackendRoute registers a route handler for a specific backend.
// It creates a handler function that routes requests to the appropriate backend,
// taking into account tenant-specific configurations.
func (m *ReverseProxyModule) registerBackendRoute(backendID, route string) {
	// Create the handler function
	handler := m.createBackendProxyHandler(backendID)

	// Store the handler in the backend routes map
	if _, ok := m.backendRoutes[backendID]; !ok {
		m.backendRoutes[backendID] = make(map[string]http.HandlerFunc)
	}
	m.backendRoutes[backendID][route] = handler

	// Register the handler with the router immediately if router is available
	if m.router != nil {
		m.safeHandleFunc(route, handler)
	}
}

// setupCompositeRoutes sets up routes that combine responses from multiple backends.
// For each composite route in the configuration, it creates a handler that fetches
// and combines responses from multiple backends.
func (m *ReverseProxyModule) setupCompositeRoutes(ctx context.Context) error {
	// Create a map of handlers for each composite route, keyed by tenant ID
	// Check if config is nil
	if m.config == nil {
		return nil
	}
	// An empty tenant ID represents the global/default handler
	type HandlerMap map[modular.TenantID]http.HandlerFunc
	compositeHandlers := make(map[string]HandlerMap)

	// Check if composite routes are configured
	if m.config.CompositeRoutes == nil {
		m.config.CompositeRoutes = make(map[string]CompositeRoute)
	}
	// First, set up global composite handlers from the global config
	for routePath, routeConfig := range m.config.CompositeRoutes {
		// Create the handler - use feature flag aware version if needed
		var handlerFunc http.HandlerFunc
		if routeConfig.FeatureFlagID != "" {
			// Use feature flag aware handler
			ffHandlerFunc, err := m.createFeatureFlagAwareCompositeHandlerFunc(ctx, routeConfig, nil)
			if err != nil {
				m.app.Logger().Error("Failed to create feature flag aware composite handler",
					"route", routePath, "error", err)
				continue
			}
			handlerFunc = ffHandlerFunc
		} else {
			// Use standard composite handler
			handler, err := m.createCompositeHandler(ctx, routeConfig, nil)
			if err != nil {
				m.app.Logger().Error("Failed to create global composite handler",
					"route", routePath, "error", err)
				continue
			}
			handlerFunc = handler.ServeHTTP
		}

		// Initialize the handler map for this route if not exists
		if _, exists := compositeHandlers[routePath]; !exists {
			compositeHandlers[routePath] = make(HandlerMap)
		}

		// Store the global handler with an empty tenant ID key
		compositeHandlers[routePath][""] = handlerFunc
	}

	// Now set up tenant-specific composite handlers
	for tenantID, tenantConfig := range m.tenants {
		// Skip if tenant config is nil
		if tenantConfig == nil || tenantConfig.CompositeRoutes == nil {
			continue
		}

		for routePath, routeConfig := range tenantConfig.CompositeRoutes {
			// Create the handler - use feature flag aware version if needed
			var handlerFunc http.HandlerFunc
			if routeConfig.FeatureFlagID != "" {
				// Use feature flag aware handler
				ffHandlerFunc, err := m.createFeatureFlagAwareCompositeHandlerFunc(ctx, routeConfig, tenantConfig)
				if err != nil {
					m.app.Logger().Error("Failed to create feature flag aware tenant composite handler",
						"tenant", tenantID, "route", routePath, "error", err)
					continue
				}
				handlerFunc = ffHandlerFunc
			} else {
				// Use standard composite handler
				handler, err := m.createCompositeHandler(ctx, routeConfig, tenantConfig)
				if err != nil {
					m.app.Logger().Error("Failed to create tenant composite handler",
						"tenant", tenantID, "route", routePath, "error", err)
					continue
				}
				handlerFunc = handler.ServeHTTP
			}

			// Initialize the handler map for this route if not exists
			if _, exists := compositeHandlers[routePath]; !exists {
				compositeHandlers[routePath] = make(HandlerMap)
			}

			// Store the tenant-specific handler
			compositeHandlers[routePath][tenantID] = handlerFunc
		}
	}

	// Create the final composite route handlers that route to tenant-specific handlers
	// based on the tenant ID from the request
	for routePath, handlerMap := range compositeHandlers {
		// Create a handler function that routes to the appropriate tenant handler
		routeHandler := func(handlerMap HandlerMap) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				// Try to get tenant ID from request
				tenantIDStr, hasTenant := TenantIDFromRequest(m.config.TenantIDHeader, r)

				// Check if tenant ID is required but not provided
				if m.config.RequireTenantID && !hasTenant {
					http.Error(w, fmt.Sprintf("Header %s is required", m.config.TenantIDHeader), http.StatusBadRequest)
					return
				}

				var handler http.HandlerFunc

				// If request has a tenant ID, try to find a tenant-specific handler
				if hasTenant {
					tenantID := modular.TenantID(tenantIDStr)
					if tenantHandler, ok := handlerMap[tenantID]; ok {
						handler = tenantHandler
					}
				}

				// Fall back to global handler if no tenant handler was found
				if handler == nil {
					if globalHandler, ok := handlerMap[""]; ok {
						handler = globalHandler
					} else {
						// No handler found - return error
						http.Error(w, "No handler for route", http.StatusInternalServerError)
						return
					}
				}

				// Execute the appropriate handler
				handler(w, r)
			}
		}(handlerMap)

		// Store the route handler
		m.compositeRoutes[routePath] = routeHandler
	}

	return nil
}

// registerRoutes configures all routes with the router
func (m *ReverseProxyModule) registerRoutes() error {
	// Ensure we have a router with comprehensive nil checks
	if m.router == nil {
		return ErrCannotRegisterRoutes
	}
	// Additional check for nil interface
	if reflect.ValueOf(m.router).IsNil() {
		return fmt.Errorf("%w: router interface is nil", ErrCannotRegisterRoutes)
	}

	// Case 1: No tenants - register basic and composite routes as usual
	if len(m.tenants) == 0 {
		return m.registerBasicRoutes()
	}

	// Case 2 & 3: With tenants - use chi's router capabilities for tenant routing
	return m.registerTenantAwareRoutes()
}

// registerBasicRoutes registers routes when no tenants are configured
func (m *ReverseProxyModule) registerBasicRoutes() error {
	registeredPaths := make(map[string]bool)

	// Register explicit routes from configuration with feature flag support
	for routePath, backendID := range m.config.Routes {
		// Check if this backend exists
		// Support backend group spec: if backendID contains comma, we'll select dynamically per request.
		isGroup := strings.Contains(backendID, ",")
		if !isGroup { // original single-backend validation
			m.backendProxiesMutex.RLock()
			defaultProxy, exists := m.backendProxies[backendID]
			m.backendProxiesMutex.RUnlock()
			if !exists || defaultProxy == nil {
				m.app.Logger().Warn("Backend not found for route", "route", routePath, "backend", backendID)
				continue
			}
		}

		// Create a handler that considers route configs for feature flag evaluation
		handler := func(routePath, backendID string) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				// Check tenant header enforcement first
				_, hasTenant := TenantIDFromRequest(m.config.TenantIDHeader, r)
				if m.config.RequireTenantID && !hasTenant {
					http.Error(w, fmt.Sprintf("Header %s is required", m.config.TenantIDHeader), http.StatusBadRequest)
					return
				}

				// If this is a backend group, pick one now (round-robin) and substitute
				resolvedBackendID := backendID
				if strings.Contains(backendID, ",") {
					selected, _, _ := m.selectBackendFromGroup(r.Context(), backendID)
					if selected != "" {
						resolvedBackendID = selected
					}
				}
				// Check if this route has feature flag configuration
				if m.config.RouteConfigs != nil {
					if routeConfig, ok := m.config.RouteConfigs[routePath]; ok && routeConfig.FeatureFlagID != "" {
						if !m.evaluateFeatureFlag(routeConfig.FeatureFlagID, r) {
							// Feature flag is disabled, use alternative backend
							alternativeBackend := m.getAlternativeBackend(routeConfig.AlternativeBackend)
							if alternativeBackend != "" {
								m.app.Logger().Debug("Feature flag disabled for route, using alternative backend",
									"route", routePath, "flagID", routeConfig.FeatureFlagID,
									"primary", backendID, "alternative", alternativeBackend)

								// Check if dry run is enabled for this route
								if routeConfig.DryRun && m.dryRunHandler != nil {
									// Determine which backend to compare against
									dryRunBackend := routeConfig.DryRunBackend
									if dryRunBackend == "" {
										dryRunBackend = backendID // Default to primary for comparison
									}

									m.app.Logger().Debug("Processing dry run request (feature flag disabled)",
										"route", routePath, "returnBackend", alternativeBackend, "compareBackend", dryRunBackend)

									// Use dry run handler - return alternative backend response, compare with dry run backend
									m.handleDryRunRequest(r.Context(), w, r, routeConfig, alternativeBackend, dryRunBackend)
									return
								}

								// Create handler for alternative backend
								altHandler := m.createBackendProxyHandler(alternativeBackend)
								altHandler(w, r)
								return
							} else {
								// No alternative backend available
								http.Error(w, "Backend temporarily unavailable", http.StatusServiceUnavailable)
								return
							}
						} else {
							// Feature flag is enabled, check for dry run
							if routeConfig.DryRun && m.dryRunHandler != nil {
								// Determine which backend to compare against
								dryRunBackend := routeConfig.DryRunBackend
								if dryRunBackend == "" {
									dryRunBackend = m.getAlternativeBackend(routeConfig.AlternativeBackend) // Default to alternative for comparison
								}

								if dryRunBackend != "" && dryRunBackend != backendID {
									m.app.Logger().Debug("Processing dry run request (feature flag enabled)",
										"route", routePath, "returnBackend", backendID, "compareBackend", dryRunBackend)

									// Use dry run handler - return primary backend response, compare with dry run backend
									m.handleDryRunRequest(r.Context(), w, r, routeConfig, backendID, dryRunBackend)
									return
								}
							}
						}
					}
				}

				// Debug backend resolution
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Debug("Using primary backend for route",
						"path", sanitizeForLogging(r.URL.Path),
						"route", routePath,
						"original_backend", backendID,
						"resolved_backend", resolvedBackendID)
				}

				// Use primary backend (feature flag enabled or no feature flag)
				primaryHandler := m.createBackendProxyHandler(resolvedBackendID)
				primaryHandler(w, r)
			}
		}(routePath, backendID)

		// Remember the handler for dynamic route resolution (especially for wildcard patterns)
		if _, ok := m.backendRoutes[backendID]; !ok {
			m.backendRoutes[backendID] = make(map[string]http.HandlerFunc)
		}
		m.backendRoutes[backendID][routePath] = handler

		m.safeHandleFunc(routePath, handler)
		registeredPaths[routePath] = true

		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Info("Registered route", "route", routePath, "backend", backendID)
		}
	}

	// Register all composite routes
	for pattern, handler := range m.compositeRoutes {
		m.safeHandleFunc(pattern, handler)
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Info("Registered composite route", "route", pattern)
		}
	}

	// Register catch-all route if not already registered and a default backend is configured
	if m.defaultBackend != "" && !registeredPaths["/*"] {
		m.backendProxiesMutex.RLock()
		defaultProxy, exists := m.backendProxies[m.defaultBackend]
		m.backendProxiesMutex.RUnlock()
		if !exists || defaultProxy == nil {
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Warn("Default backend configured but proxy not available", "backend", m.defaultBackend)
			}
			return nil
		}
		handler := func(w http.ResponseWriter, r *http.Request) {
			// Exclude internal endpoints from proxying
			if m.shouldExcludeFromProxy(r.URL.Path) {
				http.NotFound(w, r)
				return
			}

			// Enforce tenant header requirement before attempting resolution
			_, hasTenant := TenantIDFromRequest(m.config.TenantIDHeader, r)
			if m.config.RequireTenantID && !hasTenant {
				http.Error(w, fmt.Sprintf("Header %s is required", m.config.TenantIDHeader), http.StatusBadRequest)
				return
			}

			// Try to match composite routes first
			if compositeHandler, ok := m.findBestCompositeHandler(r.URL.Path); ok {
				compositeHandler(w, r)
				return
			}

			// Then try explicit route patterns (including wildcard patterns)
			if pattern, ok := m.findBestRoutePattern(r.URL.Path, m.config.Routes); ok {
				routeHandler := m.createTenantAwareHandler(pattern)
				routeHandler(w, r)
				return
			}

			// Fallback to default backend
			if m.defaultBackend != "" {
				h := m.createBackendProxyHandler(m.defaultBackend)
				h(w, r)
			} else {
				// No default backend configured, return 404
				http.NotFound(w, r)
			}
		}

		m.safeHandleFunc("/*", handler)
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Info("Registered catch-all route with default backend fallback", "backend", m.defaultBackend)
		}
	}

	return nil
}

// findBestCompositeHandler returns the most specific composite route handler that matches the request path.
// Specificity is determined by the longest matching pattern. If patterns have the same length, the first
// encountered handler is used, which is acceptable because composite route keys are typically unique.
func (m *ReverseProxyModule) findBestCompositeHandler(requestPath string) (http.HandlerFunc, bool) {
	var (
		selectedHandler http.HandlerFunc
		selectedPattern string
		matched         bool
	)

	for pattern, handler := range m.compositeRoutes {
		if handler == nil {
			continue
		}
		if !m.matchesRoute(requestPath, pattern) {
			continue
		}

		if !matched || len(pattern) > len(selectedPattern) {
			selectedHandler = handler
			selectedPattern = pattern
			matched = true
		}
	}

	return selectedHandler, matched
}

// findBestRoutePattern identifies the most specific route pattern matching the request path across the provided
// route maps. Earlier route maps take precedence when patterns are equally specific. This allows tenant-specific
// routes to override global ones while still supporting wildcard matching.
func (m *ReverseProxyModule) findBestRoutePattern(requestPath string, routeSets ...map[string]string) (string, bool) {
	bestPattern := ""
	bestLength := -1
	bestPriority := len(routeSets)
	matched := false

	for priority, routes := range routeSets {
		if routes == nil {
			continue
		}
		for pattern := range routes {
			if !m.matchesRoute(requestPath, pattern) {
				continue
			}
			patternLength := len(pattern)
			if !matched || patternLength > bestLength || (patternLength == bestLength && priority < bestPriority) {
				bestPattern = pattern
				bestLength = patternLength
				bestPriority = priority
				matched = true
			}
		}
	}

	return bestPattern, matched
}

// shouldExcludeFromProxy checks if a request path should be excluded from proxying
// to allow health/metrics/debug endpoints to be handled by internal handlers.
func (m *ReverseProxyModule) shouldExcludeFromProxy(path string) bool {
	// Health endpoint
	if path == "/health" || path == "/health/" {
		return true
	}

	// Metrics endpoints
	if m.config != nil && m.config.MetricsEndpoint != "" {
		metricsEndpoint := m.config.MetricsEndpoint
		if path == metricsEndpoint || path == metricsEndpoint+"/" {
			return true
		}
		// Health endpoint under metrics
		if path == metricsEndpoint+"/health" || path == metricsEndpoint+"/health/" {
			return true
		}
	}

	// Debug endpoints (if they are configured)
	if strings.HasPrefix(path, "/debug/") {
		return true
	}

	return false
}

// registerTenantAwareRoutes registers routes when tenants are configured
// Uses tenant-aware routing with proper default backend override support
func (m *ReverseProxyModule) registerTenantAwareRoutes() error {
	// Get all unique endpoints across all configurations (global and tenant-specific)
	allPaths := make(map[string]bool)

	// Add global routes
	for routePath := range m.config.Routes {
		allPaths[routePath] = true
	}

	// Add composite routes
	for routePath := range m.compositeRoutes {
		allPaths[routePath] = true
	}

	// Add tenant-specific routes
	for _, tenantCfg := range m.tenants {
		if tenantCfg != nil && tenantCfg.Routes != nil {
			for routePath := range tenantCfg.Routes {
				allPaths[routePath] = true
			}
		}
	}

	// Register specific routes first
	for path := range allPaths {
		// Create a handler that checks for tenant-specific routing
		handler := m.createTenantAwareHandler(path)

		m.safeHandleFunc(path, handler)

		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Debug("Registered tenant-aware route", "path", path)
		}
	}

	// Register the catch-all route if not already registered
	if !allPaths["/*"] {
		// Create a selective tenant-aware catch-all handler that excludes health/metrics endpoints
		catchAllHandler := func(w http.ResponseWriter, r *http.Request) {
			// Check if this is a path that should not be proxied
			if m.shouldExcludeFromProxy(r.URL.Path) {
				// Let other handlers handle this (health/metrics endpoints)
				http.NotFound(w, r)
				return
			}

			// Use the tenant-aware handler
			tenantHandler := m.createTenantAwareCatchAllHandler()
			tenantHandler(w, r)
		}
		m.safeHandleFunc("/*", catchAllHandler)

		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Debug("Registered tenant-aware catch-all route")
		}
	}

	return nil
}

// TenantIDFromRequest extracts tenant ID from the request header
func TenantIDFromRequest(tenantHeader string, r *http.Request) (string, bool) {
	tenantIDStr := r.Header.Get(tenantHeader)
	return tenantIDStr, tenantIDStr != ""
}

// GetConfig returns the module's configuration.
func (m *ReverseProxyModule) GetConfig() *ReverseProxyConfig {
	return m.config
}

// SetHttpClient overrides the default HTTP client used by the module.
// This method can be used to customize the HTTP client with advanced settings
// such as custom timeouts, transport configurations, or for testing purposes.
// It should be called before the Start method.
//
// Note: This also updates the transport for all existing reverse proxies.
// This method is retained for backward compatibility, but using the httpclient
// service is recommended for new code.
func (m *ReverseProxyModule) SetHttpClient(client *http.Client) {
	if client == nil {
		return
	}

	// Update the module's HTTP client
	m.httpClient = client

	// Update transport for all existing reverse proxies
	for _, proxy := range m.backendProxies {
		if proxy != nil {
			proxy.Transport = client.Transport
		}
	}

	// Update transport for tenant-specific reverse proxies
	for _, tenantProxies := range m.tenantBackendProxies {
		for _, proxy := range tenantProxies {
			if proxy != nil {
				proxy.Transport = client.Transport
			}
		}
	}
}

// SetResponseHeaderModifier sets a custom function to modify response headers dynamically.
// The function receives the response, backend ID, and tenant ID, allowing for dynamic
// header manipulation based on backend, tenant, or response content.
//
// Example use case: Dynamically consolidate CORS headers from multiple backends.
func (m *ReverseProxyModule) SetResponseHeaderModifier(modifier func(*http.Response, string, modular.TenantID) error) {
	m.responseHeaderModifier = modifier
}

// SetResponseTransformer sets a custom response transformer for a specific composite route pattern.
// The transformer receives responses from all backends and can create a custom merged response.
func (m *ReverseProxyModule) SetResponseTransformer(pattern string, transformer ResponseTransformer) {
	m.responseTransformers[pattern] = transformer
}

// SetPipelineConfig sets the pipeline configuration for a specific composite route pattern.
// This is required for routes using the "pipeline" strategy.
// The PipelineConfig includes a RequestBuilder (to construct each subsequent request
// from previous responses) and an optional ResponseMerger (to assemble the final response).
func (m *ReverseProxyModule) SetPipelineConfig(pattern string, config PipelineConfig) {
	m.pipelineConfigs[pattern] = &config
}

// SetFanOutMerger sets the fan-out merger function for a specific composite route pattern.
// This is required for routes using the "fan-out-merge" strategy.
// The merger receives all parallel backend response bodies and produces a unified response.
func (m *ReverseProxyModule) SetFanOutMerger(pattern string, merger FanOutMerger) {
	m.fanOutMergers[pattern] = merger
}

// SetEmptyResponsePolicy sets the empty response policy for a specific composite route pattern.
// This controls how empty backend responses are handled in pipeline and fan-out-merge strategies.
func (m *ReverseProxyModule) SetEmptyResponsePolicy(pattern string, policy EmptyResponsePolicy) {
	m.emptyResponsePolicies[pattern] = policy
}

// createReverseProxyForBackend creates a reverse proxy for a specific backend with per-backend configuration.
func (m *ReverseProxyModule) createReverseProxyForBackend(ctx context.Context, target *url.URL, backendID string, endpoint string) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Emit proxy created event
	m.emitEvent(ctx, EventTypeProxyCreated, map[string]interface{}{
		"backend_id": backendID,
		"target_url": target.String(),
		"endpoint":   endpoint,
	})

	// Use the module's custom transport if available, otherwise set a default timeout-aware transport
	if m.httpClient != nil && m.httpClient.Transport != nil {
		proxy.Transport = m.httpClient.Transport
	} else {
		// Create a timeout-aware transport that respects request context
		proxy.Transport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
		}
	}

	// Store the original target for use in the director function
	originalTarget := *target

	// Create a custom director that handles hostname forwarding and path rewriting
	proxy.Director = func(req *http.Request) {
		// Extract tenant ID from the request header if available
		var tenantIDStr string
		var hasTenant bool
		if m.config != nil {
			tenantIDStr, hasTenant = TenantIDFromRequest(m.config.TenantIDHeader, req)
		}

		// Get the appropriate configuration (tenant-specific or global)
		var config *ReverseProxyConfig
		if m.config != nil && hasTenant && m.tenants != nil {
			tenantID := modular.TenantID(tenantIDStr)
			if tenantCfg, ok := m.tenants[tenantID]; ok && tenantCfg != nil {
				config = tenantCfg
			} else {
				config = m.config
			}
		} else {
			config = m.config
		}

		// Apply path rewriting if configured
		rewrittenPath := m.applyPathRewritingForBackend(req.URL.Path, config, backendID, endpoint)

		// Set up the request URL
		req.URL.Scheme = originalTarget.Scheme
		req.URL.Host = originalTarget.Host
		req.URL.Path = singleJoiningSlash(originalTarget.Path, rewrittenPath)

		// Handle query parameters
		if originalTarget.RawQuery != "" && req.URL.RawQuery != "" {
			req.URL.RawQuery = originalTarget.RawQuery + "&" + req.URL.RawQuery
		} else if originalTarget.RawQuery != "" {
			req.URL.RawQuery = originalTarget.RawQuery
		}

		// Apply header rewriting
		m.applyHeaderRewritingForBackend(req, config, backendID, endpoint, &originalTarget)
	}

	// If a custom director factory is available, use it (this is for advanced use cases)
	if m.directorFactory != nil {
		// Get the backend ID from the target URL host
		backend := originalTarget.Host
		originalDirector := proxy.Director

		// Create a custom director that handles the backend routing
		proxy.Director = func(req *http.Request) {
			// Apply our standard director first
			originalDirector(req)

			// Then apply custom director if available
			var tenantIDStr string
			var hasTenant bool
			if m.config != nil {
				tenantIDStr, hasTenant = TenantIDFromRequest(m.config.TenantIDHeader, req)
			}

			if hasTenant {
				tenantID := modular.TenantID(tenantIDStr)
				customDirector := m.directorFactory(backend, tenantID)
				if customDirector != nil {
					customDirector(req)
					return
				}
			}

			// If no tenant-specific director was applied, try with empty tenant ID
			emptyTenantDirector := m.directorFactory(backend, "")
			if emptyTenantDirector != nil {
				emptyTenantDirector(req)
				return
			}
		}
	}

	// Set up error handler to return proper HTTP status codes for connection failures
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		// Log the error for debugging
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Error("Proxy error", "backend", backendID, "error", err.Error())
		}

		// Emit request failed event
		m.emitEvent(r.Context(), EventTypeRequestFailed, map[string]interface{}{
			"backend": backendID,
			"method":  r.Method,
			"path":    r.URL.Path,
			"error":   err.Error(),
		})

		// Determine error status and message based on error type
		statusCode, message := m.classifyProxyError(err)

		// For statusCapturingResponseWriter, use thread-safe methods
		if sw, ok := w.(*statusCapturingResponseWriter); ok {
			sw.mu.Lock()
			defer sw.mu.Unlock()
			if sw.wroteHeader {
				// Response already written (probably by timeout handler), don't write again
				return
			}

			// Directly access underlying ResponseWriter since we already hold the lock
			// Do not call sw.WriteHeader() or sw.Write() as they would try to acquire the lock again
			sw.ResponseWriter.Header().Set("Content-Type", "text/plain; charset=utf-8")
			sw.ResponseWriter.Header().Set("X-Content-Type-Options", "nosniff")

			sw.status = statusCode
			sw.wroteHeader = true
			sw.ResponseWriter.WriteHeader(statusCode)
			if _, writeErr := sw.ResponseWriter.Write([]byte(message + "\n")); writeErr != nil {
				// Log write error but don't block response completion
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Warn("Failed to write error response body", "backend", backendID, "error", writeErr.Error())
				}
			}
		} else {
			// For non-statusCapturingResponseWriter, use standard http.Error
			http.Error(w, message, statusCode)
		}

	}

	// Set up ModifyResponse to handle response header rewriting
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp == nil {
			return nil
		}

		// Extract tenant ID from the original request if available
		var tenantIDStr string
		var hasTenant bool
		if resp.Request != nil && m.config != nil {
			tenantIDStr, hasTenant = TenantIDFromRequest(m.config.TenantIDHeader, resp.Request)
		}

		// Get the appropriate configuration (tenant-specific or global)
		var config *ReverseProxyConfig
		if m.config != nil && hasTenant && m.tenants != nil {
			tenantID := modular.TenantID(tenantIDStr)
			if tenantCfg, ok := m.tenants[tenantID]; ok && tenantCfg != nil {
				config = tenantCfg
			} else {
				config = m.config
			}
		} else {
			config = m.config
		}

		// Apply configured response header rewriting
		m.applyResponseHeaderRewritingForBackend(resp, config, backendID, endpoint)

		// Apply custom response header modifier if set
		if m.responseHeaderModifier != nil {
			tenantID := modular.TenantID("")
			if hasTenant {
				tenantID = modular.TenantID(tenantIDStr)
			}
			if err := m.responseHeaderModifier(resp, backendID, tenantID); err != nil {
				if m.app != nil && m.app.Logger() != nil {
					// Sanitize tenantID before logging to prevent log forging via newlines
					safeTenantID := strings.ReplaceAll(strings.ReplaceAll(string(tenantID), "\n", ""), "\r", "")
					m.app.Logger().Error("Response header modifier error", "backend", backendID, "tenant", safeTenantID, "error", err.Error())
				}
				return err
			}
		}

		return nil
	}

	return proxy
}

// classifyProxyError determines the appropriate HTTP status code and user-friendly message
// based on the type of proxy error encountered. This helper function centralizes error
// classification logic to maintain consistency across error handling paths.
func (m *ReverseProxyModule) classifyProxyError(err error) (statusCode int, message string) {
	if err == nil {
		return http.StatusInternalServerError, "Internal server error"
	}

	errorMsg := strings.ToLower(err.Error())

	// Check for timeout errors first (most specific)
	if strings.Contains(errorMsg, "context deadline exceeded") ||
		strings.Contains(errorMsg, "timeout") ||
		strings.Contains(errorMsg, "deadline") {
		return http.StatusGatewayTimeout, "Gateway timeout"
	}

	// Check for connection errors
	if strings.Contains(errorMsg, "connection refused") ||
		strings.Contains(errorMsg, "no such host") {
		return http.StatusBadGateway, "Backend service unavailable"
	}

	// Default to internal server error
	return http.StatusInternalServerError, "Internal server error"
}

// createBackendProxy creates a reverse proxy for the specified backend ID and service URL.
// It parses the URL, creates the proxy, and stores it in the backendProxies map.
func (m *ReverseProxyModule) createBackendProxy(backendID, serviceURL string) error {
	// Check if we have backend-specific configuration
	var backendURL *url.URL
	var err error

	if m.config != nil && m.config.BackendConfigs != nil {
		if backendConfig, exists := m.config.BackendConfigs[backendID]; exists && backendConfig.URL != "" {
			// Use URL from backend configuration
			backendURL, err = url.Parse(backendConfig.URL)
		} else {
			// Fall back to service URL from BackendServices
			backendURL, err = url.Parse(serviceURL)
		}
	} else {
		// Use service URL from BackendServices
		backendURL, err = url.Parse(serviceURL)
	}

	if err != nil {
		return fmt.Errorf("failed to parse %s URL %s: %w", backendID, serviceURL, err)
	}
	// Ensure backendProxies map is initialized
	m.backendProxiesMutex.Lock()
	if m.backendProxies == nil {
		m.backendProxies = make(map[string]*httputil.ReverseProxy)
	}
	m.backendProxiesMutex.Unlock()

	// Set up proxy for this backend
	proxy := m.createReverseProxyForBackend(context.Background(), backendURL, backendID, "") //nolint:contextcheck // backend creation occurs during module initialization

	// Store the proxy for this backend
	m.backendProxiesMutex.Lock()
	m.backendProxies[backendID] = proxy
	m.backendProxiesMutex.Unlock()

	// Emit backend added event only for dynamic additions after initialization
	if m.initialized {
		m.emitEvent(context.Background(), EventTypeBackendAdded, map[string]interface{}{ //nolint:contextcheck // backend mutations originate from administrative actions without request context
			"backend": backendID,
			"url":     serviceURL,
			"time":    time.Now().UTC().Format(time.RFC3339Nano),
		})
	}

	return nil
}

// AddBackend dynamically adds a new backend to the module at runtime and emits an event.
// It updates the configuration, creates the proxy, and (optionally) registers a default route
// if one matching the backend name does not already exist.
func (m *ReverseProxyModule) AddBackend(backendID, serviceURL string) error { //nolint:ireturn
	if backendID == "" {
		return ErrBackendIDRequired
	}
	if serviceURL == "" {
		return ErrServiceURLRequired
	}
	if m.config.BackendServices == nil {
		m.config.BackendServices = make(map[string]string)
	}

	// Track whether this is an update of an existing backend so we can log accordingly
	if existingURL, exists := m.config.BackendServices[backendID]; exists {
		if m.app != nil && m.app.Logger() != nil {
			if existingURL != serviceURL {
				m.app.Logger().Info("Updating backend service URL", "backend", backendID, "old", existingURL, "new", serviceURL)
			} else {
				m.app.Logger().Debug("Backend already configured with requested URL", "backend", backendID)
			}
		}
	}

	// Persist in config and create proxy (this will emit backend.added event because initialized=true)
	m.config.BackendServices[backendID] = serviceURL
	if err := m.createBackendProxy(backendID, serviceURL); err != nil {
		return err
	}

	// If router already running and no route references this backend, add a basic pattern route for tests
	if m.router != nil {
		pattern := fmt.Sprintf("/%s/*", backendID)
		// Only add if not conflicting with existing routes
		if err := m.AddBackendRoute(backendID, pattern); err != nil {
			// Non-fatal: log only
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Warn("Failed to auto-register route for new backend", "backend", backendID, "error", err)
			}
		}
	}

	return nil
}

// RemoveBackend removes an existing backend at runtime and emits a backend.removed event.
func (m *ReverseProxyModule) RemoveBackend(backendID string) error { //nolint:ireturn
	if backendID == "" {
		return ErrBackendIDRequired
	}
	if m.config.BackendServices == nil {
		return ErrNoBackendsConfigured
	}
	serviceURL, exists := m.config.BackendServices[backendID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrBackendNotConfigured, backendID)
	}

	// Remove from maps
	delete(m.config.BackendServices, backendID)
	delete(m.backendProxies, backendID)
	delete(m.backendRoutes, backendID)
	delete(m.circuitBreakers, backendID)

	// Emit removal event
	if m.initialized {
		m.emitEvent(context.Background(), EventTypeBackendRemoved, map[string]interface{}{ //nolint:contextcheck // backend removal triggered outside request path
			"backend": backendID,
			"url":     serviceURL,
			"time":    time.Now().UTC().Format(time.RFC3339Nano),
		})
	}

	return nil
}

// selectBackendFromGroup performs a simple round-robin selection from a comma-separated backend group spec.
// Returns selected backend id, selected index, and total backends.
func (m *ReverseProxyModule) selectBackendFromGroup(ctx context.Context, group string) (string, int, int) {
	parts := strings.Split(group, ",")
	var backends []string
	for _, p := range parts {
		b := strings.TrimSpace(p)
		if b != "" {
			backends = append(backends, b)
		}
	}
	if len(backends) == 0 {
		return "", 0, 0
	}
	m.loadBalanceMutex.Lock()
	idx := m.loadBalanceCounters[group] % len(backends)
	m.loadBalanceCounters[group] = m.loadBalanceCounters[group] + 1
	m.loadBalanceMutex.Unlock()

	selected := backends[idx]

	// Emit load balancing decision events if module initialized so tests can observe
	if m.initialized {
		// Generic decision event (once per selection)
		m.emitEvent(ctx, EventTypeLoadBalanceDecision, map[string]interface{}{
			"group":            group,
			"selected_backend": selected,
			"index":            idx,
			"total":            len(backends),
			"time":             time.Now().UTC().Format(time.RFC3339Nano),
		})
		// Round-robin specific event includes rotation information
		m.emitEvent(ctx, EventTypeLoadBalanceRoundRobin, map[string]interface{}{
			"group":         group,
			"backend":       selected,
			"current_index": idx,
			"total":         len(backends),
			"time":          time.Now().UTC().Format(time.RFC3339Nano),
		})
	}

	return selected, idx, len(backends)
}

// sanitizeForLogging removes newline and carriage return characters from a string
// to prevent log injection attacks
func sanitizeForLogging(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// obfuscateTenantID creates a reproducible hash of a tenant ID for logging purposes,
// allowing operators to correlate requests by tenant without exposing the raw tenant ID
func obfuscateTenantID(tenantID modular.TenantID) string {
	hash := sha256.Sum256([]byte(tenantID))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for brevity
}

// Helper function to correctly join URL paths
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	if aslash && bslash {
		return a + b[1:]
	}
	if !aslash && !bslash {
		return a + "/" + b
	}
	return a + b
}

// copyResponseHeaders intelligently copies HTTP headers from source to target,
// handling single-value and multi-value headers appropriately.
//
// Single-value headers (defined in singleValueHTTPHeaders) use Header.Set() to
// prevent duplicates, taking the last value if the backend sent multiple values.
// This is critical for CORS headers where duplicates cause browser errors.
//
// Multi-value headers (like Set-Cookie, Link, Warning) use Header.Add() to preserve
// all values, as these headers are designed to have multiple instances per RFC 7230.
//
// This approach aligns with standard proxy behavior (Nginx, HAProxy, Envoy) and
// HTTP specifications while protecting against misconfigured backends.
//
// Example use cases:
//   - Deduplicating CORS headers from backends that incorrectly send duplicates
//   - Preserving multiple Set-Cookie headers when proxying auth services
//   - Maintaining proper cache headers when serving from cache
func copyResponseHeaders(source, target http.Header) {
	for key, values := range source {
		if singleValueHTTPHeaders[key] {
			// Use Set for single-value headers (prevents duplicates)
			if len(values) > 0 {
				// Use the last value if backend sent duplicates
				// This matches the behavior of most HTTP clients
				target.Set(key, values[len(values)-1])
			}
		} else {
			// Use Add for multi-value headers (preserves all values)
			// This is critical for headers like Set-Cookie where multiple
			// values are expected and semantically meaningful
			for _, value := range values {
				target.Add(key, value)
			}
		}
	}
}

// applyPathRewritingForBackend applies path rewriting rules for a specific backend and endpoint
func (m *ReverseProxyModule) applyPathRewritingForBackend(originalPath string, config *ReverseProxyConfig, backendID string, endpoint string) string {
	if config == nil {
		return originalPath
	}

	rewrittenPath := originalPath

	// Check if we have backend-specific configuration
	if config.BackendConfigs != nil && backendID != "" {
		if backendConfig, exists := config.BackendConfigs[backendID]; exists {
			// Apply backend-specific path rewriting first
			rewrittenPath = m.applySpecificPathRewriting(rewrittenPath, &backendConfig.PathRewriting)

			// Then check for endpoint-specific configuration
			if endpoint != "" && backendConfig.Endpoints != nil {
				if endpointConfig, exists := backendConfig.Endpoints[endpoint]; exists {
					// Apply endpoint-specific path rewriting
					rewrittenPath = m.applySpecificPathRewriting(rewrittenPath, &endpointConfig.PathRewriting)
				}
			}

			return rewrittenPath
		}
	}

	// No specific configuration found, return original path
	return originalPath
}

// applySpecificPathRewriting applies path rewriting rules from a specific PathRewritingConfig
func (m *ReverseProxyModule) applySpecificPathRewriting(originalPath string, config *PathRewritingConfig) string {
	if config == nil {
		return originalPath
	}

	rewrittenPath := originalPath

	// Apply base path stripping first
	if config.StripBasePath != "" {
		if strings.HasPrefix(rewrittenPath, config.StripBasePath) {
			rewrittenPath = rewrittenPath[len(config.StripBasePath):]
			// Ensure the path starts with /
			if !strings.HasPrefix(rewrittenPath, "/") {
				rewrittenPath = "/" + rewrittenPath
			}
		}
	}

	// Apply base path rewriting
	if config.BasePathRewrite != "" {
		// If there's a base path rewrite, prepend it to the path
		rewrittenPath = singleJoiningSlash(config.BasePathRewrite, rewrittenPath)
	}

	// Apply endpoint-specific rewriting rules
	if config.EndpointRewrites != nil {
		for _, rule := range config.EndpointRewrites {
			if rule.Pattern != "" && rule.Replacement != "" {
				// Check if the path matches the pattern
				if m.matchesPattern(rewrittenPath, rule.Pattern) {
					// Apply the replacement
					rewrittenPath = m.applyPatternReplacement(rewrittenPath, rule.Pattern, rule.Replacement)
					break // Apply only the first matching rule
				}
			}
		}
	}

	return rewrittenPath
}

// applyHeaderRewritingForBackend applies header rewriting rules for a specific backend and endpoint
func (m *ReverseProxyModule) applyHeaderRewritingForBackend(req *http.Request, config *ReverseProxyConfig, backendID string, endpoint string, target *url.URL) {
	if config == nil {
		return
	}

	// Check if we have backend-specific configuration
	if config.BackendConfigs != nil && backendID != "" {
		if backendConfig, exists := config.BackendConfigs[backendID]; exists {
			// Apply backend-specific header rewriting first
			m.applySpecificHeaderRewriting(req, &backendConfig.HeaderRewriting, target)

			// Then check for endpoint-specific configuration
			if endpoint != "" && backendConfig.Endpoints != nil {
				if endpointConfig, exists := backendConfig.Endpoints[endpoint]; exists {
					// Apply endpoint-specific header rewriting (this overrides backend-specific)
					m.applySpecificHeaderRewriting(req, &endpointConfig.HeaderRewriting, target)
				}
			}

			return
		}
	}

	// Fall back to default hostname handling (preserve original)
	// This preserves the original request's Host header, which is what we want by default
	// If the original request doesn't have a Host header, it will be set by the HTTP client
	// based on the request URL during request execution.
}

// applySpecificHeaderRewriting applies header rewriting rules from a specific HeaderRewritingConfig
func (m *ReverseProxyModule) applySpecificHeaderRewriting(req *http.Request, config *HeaderRewritingConfig, target *url.URL) {
	if config == nil {
		return
	}

	// Handle hostname configuration
	switch config.HostnameHandling {
	case HostnameUseBackend:
		// Set the Host header to the backend's hostname
		req.Host = target.Host
	case HostnameUseCustom:
		// Set the Host header to the custom hostname
		if config.CustomHostname != "" {
			req.Host = config.CustomHostname
		}
	case HostnamePreserveOriginal:
		fallthrough
	default:
		// Do nothing - preserve the original Host header
		// This is the default behavior
	}

	// Apply custom header setting
	if config.SetHeaders != nil {
		for headerName, headerValue := range config.SetHeaders {
			req.Header.Set(headerName, headerValue)
		}
	}

	// Apply header removal
	if config.RemoveHeaders != nil {
		for _, headerName := range config.RemoveHeaders {
			req.Header.Del(headerName)
		}
	}
}

// applyResponseHeaderRewritingForBackend applies response header rewriting rules for a specific backend and endpoint
func (m *ReverseProxyModule) applyResponseHeaderRewritingForBackend(resp *http.Response, config *ReverseProxyConfig, backendID string, endpoint string) {
	if config == nil || resp == nil {
		return
	}

	// Apply global response header configuration first
	m.applySpecificResponseHeaderRewriting(resp, &config.ResponseHeaderConfig)

	// Check if we have backend-specific configuration
	if config.BackendConfigs != nil && backendID != "" {
		if backendConfig, exists := config.BackendConfigs[backendID]; exists {
			// Apply backend-specific response header rewriting
			m.applySpecificResponseHeaderRewriting(resp, &backendConfig.ResponseHeaderRewriting)

			// Then check for endpoint-specific configuration
			if endpoint != "" && backendConfig.Endpoints != nil {
				if endpointConfig, exists := backendConfig.Endpoints[endpoint]; exists {
					// Apply endpoint-specific response header rewriting (this overrides backend-specific)
					m.applySpecificResponseHeaderRewriting(resp, &endpointConfig.ResponseHeaderRewriting)
				}
			}
		}
	}
}

// applySpecificResponseHeaderRewriting applies response header rewriting rules from a specific ResponseHeaderRewritingConfig
func (m *ReverseProxyModule) applySpecificResponseHeaderRewriting(resp *http.Response, config *ResponseHeaderRewritingConfig) {
	if config == nil || resp == nil {
		return
	}

	// Apply custom header setting
	if config.SetHeaders != nil {
		for headerName, headerValue := range config.SetHeaders {
			resp.Header.Set(headerName, headerValue)
		}
	}

	// Apply header removal
	if config.RemoveHeaders != nil {
		for _, headerName := range config.RemoveHeaders {
			resp.Header.Del(headerName)
		}
	}
}

// matchesPattern checks if a path matches a pattern using glob pattern matching
func (m *ReverseProxyModule) matchesPattern(path, pattern string) bool {
	// Use glob library for more efficient and feature-complete pattern matching
	g, err := glob.Compile(pattern)
	if err != nil {
		// Fallback to simple string matching if glob compilation fails
		return path == pattern
	}
	return g.Match(path)
}

// applyPatternReplacement applies a pattern replacement to a path
func (m *ReverseProxyModule) applyPatternReplacement(path, pattern, replacement string) string {
	// If pattern is an exact match, replace entirely
	if path == pattern {
		return replacement
	}

	// Use glob to match and extract parts for replacement
	g, err := glob.Compile(pattern)
	if err != nil {
		// Fallback to simple replacement if glob compilation fails
		return replacement
	}

	if !g.Match(path) {
		return path
	}

	// Handle common patterns efficiently
	if strings.HasSuffix(pattern, "/*") {
		prefix := pattern[:len(pattern)-2]
		if strings.HasPrefix(path, prefix) {
			suffix := path[len(prefix):]
			return singleJoiningSlash(replacement, suffix)
		}
	} else if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		if strings.HasPrefix(path, prefix) {
			suffix := path[len(prefix):]
			return replacement + suffix
		}
	}

	// For exact matches or simple patterns, use replacement
	return replacement
}

// statusCapturingResponseWriter wraps http.ResponseWriter to capture the status code
type statusCapturingResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	mu          sync.Mutex
}

func (w *statusCapturingResponseWriter) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *statusCapturingResponseWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Ensure headers are written before body
	if !w.wroteHeader {
		w.status = http.StatusOK
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(w.status)
	}
	n, err := w.ResponseWriter.Write(data)
	if err != nil {
		return n, fmt.Errorf("failed to write response data: %w", err)
	}
	return n, nil
}

// bufferingResponseWriter buffers the response until explicitly flushed
// This prevents race conditions in timeout scenarios where we need to override the response
type bufferingResponseWriter struct {
	header http.Header
	body   []byte
	status int
	mu     sync.Mutex // Protect concurrent access to header, body, and status
}

func (w *bufferingResponseWriter) Header() http.Header {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.header
}

func (w *bufferingResponseWriter) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = code
}

func (w *bufferingResponseWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.body = append(w.body, data...)
	return len(data), nil
}

// flushTo writes the buffered response to the actual ResponseWriter
func (w *bufferingResponseWriter) flushTo(target http.ResponseWriter) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Copy headers using smart deduplication
	copyResponseHeaders(w.header, target.Header())

	// Write status
	if w.status == 0 {
		w.status = http.StatusOK
	}
	target.WriteHeader(w.status)

	// Write body
	if len(w.body) > 0 {
		_, err := target.Write(w.body) //nolint:gosec // G705: reverse proxy transparently forwards upstream response body
		if err != nil {
			return fmt.Errorf("failed to write response body: %w", err)
		}
	}
	return nil
}

// createBackendProxyHandler creates an http.HandlerFunc that handles proxying requests
// to a specific backend, with support for tenant-specific backends and feature flag evaluation
func (m *ReverseProxyModule) createBackendProxyHandler(backend string) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Emit request received event
		m.emitEvent(r.Context(), EventTypeRequestReceived, map[string]interface{}{
			"backend":     backend,
			"method":      r.Method,
			"path":        r.URL.Path,
			"remote_addr": r.RemoteAddr,
		})

		// Apply timeout configuration - check for route-specific timeout first
		var requestTimeout time.Duration
		var timeoutSource string
		if m.config.RouteConfigs != nil {
			// Find matching route config by checking all patterns
			for routePattern, routeConfig := range m.config.RouteConfigs {
				if m.matchesRoute(r.URL.Path, routePattern) && routeConfig.Timeout > 0 {
					requestTimeout = routeConfig.Timeout
					timeoutSource = fmt.Sprintf("route %s", routePattern)
					break
				}
			}
		}

		// Fall back to global timeout if no route-specific timeout
		if requestTimeout == 0 {
			if m.config.GlobalTimeout > 0 {
				requestTimeout = m.config.GlobalTimeout
				timeoutSource = "global"
			} else if m.config.RequestTimeout > 0 {
				requestTimeout = m.config.RequestTimeout
				timeoutSource = "request"
			} else {
				requestTimeout = 30 * time.Second // Default fallback
				timeoutSource = "default"
			}
		}

		// Debug timeout configuration
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Debug("Request timeout configuration",
				"path", sanitizeForLogging(r.URL.Path),
				"backend", backend,
				"timeout", requestTimeout.String(),
				"timeout_source", timeoutSource)
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()
		r = r.WithContext(ctx)

		// Extract tenant ID from request header, if present
		tenantHeader := m.config.TenantIDHeader
		tenantID := modular.TenantID(r.Header.Get(tenantHeader))

		// Check if tenant ID is required but missing
		if m.config.RequireTenantID && tenantID == "" {
			http.Error(w, fmt.Sprintf("Header %s is required", tenantHeader), http.StatusBadRequest)
			return
		}

		// Check if the backend is controlled by a feature flag
		finalBackend := backend
		if m.config.BackendConfigs != nil {
			if backendConfig, exists := m.config.BackendConfigs[backend]; exists && backendConfig.FeatureFlagID != "" {
				// Evaluate the feature flag for this backend
				if !m.evaluateFeatureFlag(backendConfig.FeatureFlagID, r) {
					// Feature flag is disabled, use alternative backend
					alternativeBackend := m.getAlternativeBackend(backendConfig.AlternativeBackend)
					if alternativeBackend != "" && alternativeBackend != backend {
						finalBackend = alternativeBackend
						m.app.Logger().Debug("Feature flag disabled, using alternative backend",
							"original", backend, "alternative", finalBackend, "flagID", backendConfig.FeatureFlagID)
					} else {
						// No alternative backend available
						http.Error(w, "Backend temporarily unavailable", http.StatusServiceUnavailable)
						return
					}
				}
			}
		}

		// Record request to backend for health checking
		if m.healthChecker != nil {
			m.healthChecker.RecordBackendRequest(finalBackend)
		}

		// Get the appropriate proxy for this backend and tenant
		proxy, exists := m.getProxyForBackendAndTenant(finalBackend, tenantID)
		if !exists {
			http.Error(w, fmt.Sprintf("Backend %s not found", finalBackend), http.StatusInternalServerError)
			return
		}

		// Check if circuit breaker is enabled for this backend
		var cb *CircuitBreaker
		var cbEnabled bool
		var cbConfig CircuitBreakerConfig

		// First check for backend-specific circuit breaker configuration
		if m.config.BackendConfigs != nil {
			if backendConfig, exists := m.config.BackendConfigs[finalBackend]; exists && backendConfig.CircuitBreaker.Enabled {
				cbEnabled = true
				// Convert BackendCircuitBreakerConfig to CircuitBreakerConfig
				cbConfig = CircuitBreakerConfig{
					Enabled:          backendConfig.CircuitBreaker.Enabled,
					FailureThreshold: backendConfig.CircuitBreaker.FailureThreshold,
					OpenTimeout:      backendConfig.CircuitBreaker.RecoveryTimeout,
					// Use defaults for other fields if not specified in BackendCircuitBreakerConfig
					SuccessThreshold:        1,
					HalfOpenAllowedRequests: 1,
					WindowSize:              10,
					SuccessRateThreshold:    0.5,
				}
			}
		}

		// Fall back to global circuit breaker config if no backend-specific config
		if !cbEnabled && m.config.CircuitBreakerConfig.Enabled {
			cbEnabled = true
			// Check for legacy backend-specific circuit breaker in BackendCircuitBreakers
			if backendCB, exists := m.config.BackendCircuitBreakers[finalBackend]; exists {
				cbConfig = backendCB
			} else {
				cbConfig = m.config.CircuitBreakerConfig
			}
		}

		if cbEnabled {
			// Get or create circuit breaker for this backend
			if existingCB, exists := m.circuitBreakers[finalBackend]; exists {
				cb = existingCB
			} else {
				// Use module's request timeout if circuit breaker config doesn't specify one
				if cbConfig.RequestTimeout == 0 && m.config.RequestTimeout > 0 {
					cbConfig.RequestTimeout = m.config.RequestTimeout
				}

				// Create new circuit breaker with config and store for reuse
				cb = NewCircuitBreakerWithConfig(finalBackend, cbConfig, m.metrics)
				cb.eventEmitter = func(eventType string, data map[string]interface{}) { //nolint:contextcheck // circuit breaker events occur outside request handling
					m.emitEvent(context.Background(), eventType, data)
				}
				m.circuitBreakers[finalBackend] = cb
			}
		}

		// If circuit breaker is available, wrap the proxy request with it
		if cb != nil {
			// Ensure eventEmitter is set (defensive in case of early creation without emitter)
			if cb.eventEmitter == nil {
				cb.eventEmitter = func(eventType string, data map[string]interface{}) { //nolint:contextcheck // circuit breaker events occur outside request handling
					m.emitEvent(context.Background(), eventType, data)
				}
			}
			// Create a timeout-aware transport
			timeoutTransport := &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   requestTimeout,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: requestTimeout,
				ExpectContinueTimeout: 1 * time.Second,
			}

			// Create a copy of the proxy with the timeout transport
			proxyCopy := &httputil.ReverseProxy{
				Director:       proxy.Director,
				Transport:      timeoutTransport,
				FlushInterval:  proxy.FlushInterval,
				ErrorLog:       proxy.ErrorLog,
				BufferPool:     proxy.BufferPool,
				ModifyResponse: proxy.ModifyResponse,
				ErrorHandler:   proxy.ErrorHandler,
			}

			// Execute the request via circuit breaker with proper timeout handling
			done := make(chan struct{})
			var sw *statusCapturingResponseWriter
			var cbErr error
			var cbResp *http.Response

			// Create a context that will be cancelled if the parent request context is cancelled
			proxyCtx, proxyCancel := context.WithCancel(r.Context())
			defer proxyCancel() // Ensure cleanup

			go func() {
				defer close(done)
				defer proxyCancel() // Ensure context is cancelled when goroutine exits

				// Use a buffering response writer to prevent writing to actual response until timeout check
				bufWriter := &bufferingResponseWriter{
					header: make(http.Header),
					body:   make([]byte, 0),
				}
				sw = &statusCapturingResponseWriter{ResponseWriter: bufWriter, status: http.StatusOK}

				// Create a request with the proxy context to ensure proper cancellation
				proxyReq := r.WithContext(proxyCtx)

				// Use timeout-aware proxy directly to ensure real timeout behavior
				cbResp, cbErr = cb.Execute(proxyReq, func(req *http.Request) (*http.Response, error) { //nolint:bodyclose // synthetic response carries no body and is explicitly closed after execution
					proxyCopy.ServeHTTP(sw, req) //nolint:gosec // G704: reverse proxy intentionally forwards requests to configured backends

					// Create response with captured status
					resp := &http.Response{StatusCode: sw.status, Body: http.NoBody}

					// Return error for failure status codes to trigger circuit breaker failure recording
					if sw.status >= 500 {
						return resp, fmt.Errorf("%w: %d", ErrBackendErrorStatus, sw.status)
					}

					return resp, nil
				})
			}()

			// Wait for either completion or timeout
			select {
			case <-done:
				if cbResp != nil && cbResp.Body != nil {
					if err := cbResp.Body.Close(); err != nil && m.app != nil && m.app.Logger() != nil {
						m.app.Logger().Warn("Failed to close circuit breaker response body", "error", err)
					}
				}

				// Check if the request context was cancelled due to timeout OR if circuit breaker error indicates timeout
				contextCancelled := r.Context().Err() != nil
				timeoutError := cbErr != nil && (strings.Contains(cbErr.Error(), "context deadline exceeded") ||
					strings.Contains(cbErr.Error(), "timeout"))

				if contextCancelled || timeoutError {
					// Context was cancelled (timeout occurred) - treat as timeout regardless of backend response
					m.emitEvent(r.Context(), EventTypeRequestFailed, map[string]interface{}{
						"backend": backend,
						"method":  r.Method,
						"path":    r.URL.Path,
						"error":   "request timeout",
					})

					// Use thread-safe timeout response handling
					// Check if we have a statusCapturingResponseWriter to avoid race conditions
					if sw != nil {
						sw.mu.Lock()
						if !sw.wroteHeader {
							// Directly access underlying ResponseWriter since we already hold the lock
							sw.ResponseWriter.Header().Set("Content-Type", "text/plain; charset=utf-8")
							sw.ResponseWriter.Header().Set("X-Content-Type-Options", "nosniff")
							sw.status = http.StatusGatewayTimeout
							sw.wroteHeader = true
							sw.ResponseWriter.WriteHeader(http.StatusGatewayTimeout)
							fmt.Fprintln(sw.ResponseWriter, "Request timeout")
						}
						sw.mu.Unlock()
					} else {
						// Fallback for direct writing (buffering writer case)
						w.Header().Set("Content-Type", "text/plain; charset=utf-8")
						w.Header().Set("X-Content-Type-Options", "nosniff")
						w.WriteHeader(http.StatusGatewayTimeout)
						fmt.Fprintln(w, "Request timeout")
					}
					return
				}

				// Check for circuit breaker errors BEFORE flushing buffered response
				if errors.Is(cbErr, ErrCircuitOpen) {
					// Circuit is open
					if m.app != nil && m.app.Logger() != nil {
						m.app.Logger().Warn("Circuit breaker open, denying request",
							"backend", finalBackend, "tenant_hash", obfuscateTenantID(tenantID), "path", sanitizeForLogging(r.URL.Path))
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusServiceUnavailable)
					if _, err := w.Write([]byte(`{"error":"Service temporarily unavailable","code":"CIRCUIT_OPEN"}`)); err != nil {
						if m.app != nil && m.app.Logger() != nil {
							m.app.Logger().Error("Failed to write circuit breaker response", "error", err)
						}
					}
					return
				} else if cbErr != nil {
					// Check if this is a backend error status that should be passed through
					if errors.Is(cbErr, ErrBackendErrorStatus) && sw != nil {
						// Backend returned an error status - pass through the original response
						m.emitEvent(r.Context(), EventTypeRequestFailed, map[string]interface{}{
							"backend": backend,
							"method":  r.Method,
							"path":    r.URL.Path,
							"status":  sw.status,
							"error":   cbErr.Error(),
						})
						// Flush the buffered backend response (with original error status)
						if bufWriter, ok := sw.ResponseWriter.(*bufferingResponseWriter); ok {
							if err := bufWriter.flushTo(w); err != nil && m.app != nil && m.app.Logger() != nil {
								m.app.Logger().Error("Failed to flush buffered error response", "error", err)
							}
						}
						return
					}
					// Some other error occurred (connection failure, etc.) - emit failed event before returning
					if sw != nil {
						m.emitEvent(r.Context(), EventTypeRequestFailed, map[string]interface{}{
							"backend": backend,
							"method":  r.Method,
							"path":    r.URL.Path,
							"status":  sw.status,
							"error":   cbErr.Error(),
						})
					}
					// Only write error response if headers haven't been written yet
					if sw == nil || !sw.wroteHeader {
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					}
					return
				}

				// No timeout and no circuit breaker error - flush the buffered backend response
				if sw != nil {
					if bufWriter, ok := sw.ResponseWriter.(*bufferingResponseWriter); ok {
						if err := bufWriter.flushTo(w); err != nil && m.app != nil && m.app.Logger() != nil {
							m.app.Logger().Error("Failed to flush buffered response", "error", err)
						}
					}
				}
			case <-r.Context().Done():
				// Request timed out
				// Emit request failed event for timeout
				m.emitEvent(r.Context(), EventTypeRequestFailed, map[string]interface{}{
					"backend": backend,
					"method":  r.Method,
					"path":    r.URL.Path,
					"error":   "request timeout",
				})
				// Since we used a buffering response writer, write timeout response through buffer
				// This is safe because bufferingResponseWriter doesn't write to actual response yet
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.WriteHeader(http.StatusGatewayTimeout)
				fmt.Fprintln(w, "Request timeout")
				return
			}

			// Emit success or failure event based on status code
			if sw != nil {
				if sw.status >= 400 {
					m.emitEvent(r.Context(), EventTypeRequestFailed, map[string]interface{}{
						"backend": backend,
						"method":  r.Method,
						"path":    r.URL.Path,
						"status":  sw.status,
						"error":   fmt.Sprintf("upstream returned status %d", sw.status),
					})
				} else {
					m.emitEvent(r.Context(), EventTypeRequestProxied, map[string]interface{}{
						"backend": backend,
						"method":  r.Method,
						"path":    r.URL.Path,
						"status":  sw.status,
					})
				}
			}
		} else {
			// No circuit breaker, use the proxy directly but capture status and apply timeout
			// Create a request-specific proxy to avoid race conditions on shared Transport field
			proxyForRequest := &httputil.ReverseProxy{
				Director:       proxy.Director,
				Transport:      proxy.Transport, // Start with the original transport
				FlushInterval:  proxy.FlushInterval,
				ErrorLog:       proxy.ErrorLog,
				BufferPool:     proxy.BufferPool,
				ModifyResponse: proxy.ModifyResponse,
				ErrorHandler:   proxy.ErrorHandler, // Critical: copy the custom error handler
			}

			// Configure request-specific timeout transport without modifying shared proxy
			if proxyForRequest.Transport != nil {
				if transport, ok := proxyForRequest.Transport.(*http.Transport); ok {
					// Clone the transport and update timeout settings for this request
					transportCopy := transport.Clone()
					transportCopy.ResponseHeaderTimeout = requestTimeout
					transportCopy.DialContext = (&net.Dialer{
						Timeout:   requestTimeout,
						KeepAlive: 30 * time.Second,
					}).DialContext
					proxyForRequest.Transport = transportCopy
				}
			} else {
				// Set a timeout-aware transport if none exists
				proxyForRequest.Transport = &http.Transport{
					DialContext: (&net.Dialer{
						Timeout:   requestTimeout,
						KeepAlive: 30 * time.Second,
					}).DialContext,
					TLSHandshakeTimeout:   10 * time.Second,
					ResponseHeaderTimeout: requestTimeout,
					ExpectContinueTimeout: 1 * time.Second,
					MaxIdleConns:          100,
					MaxIdleConnsPerHost:   10,
					IdleConnTimeout:       90 * time.Second,
				}
			}

			// Create a timeout context for the request
			done := make(chan struct{})
			var swMutex sync.Mutex
			var sw *statusCapturingResponseWriter

			// Create a context that will be cancelled if the parent request context is cancelled
			proxyCtx, proxyCancel := context.WithCancel(r.Context())
			defer proxyCancel() // Ensure cleanup

			go func() {
				defer close(done)
				defer proxyCancel() // Ensure context is cancelled when goroutine exits

				swMutex.Lock()
				sw = &statusCapturingResponseWriter{ResponseWriter: w, status: http.StatusOK}
				swMutex.Unlock()

				// Create a request with the proxy context to ensure proper cancellation
				proxyReq := r.WithContext(proxyCtx)
				proxyForRequest.ServeHTTP(sw, proxyReq) //nolint:gosec // G704: reverse proxy intentionally forwards requests to configured backends
			}()

			// Wait for either completion or timeout
			select {
			case <-done:
				// Request completed successfully
			case <-r.Context().Done():
				// Request timed out
				// Emit request failed event for timeout
				m.emitEvent(r.Context(), EventTypeRequestFailed, map[string]interface{}{
					"backend": backend,
					"method":  r.Method,
					"path":    r.URL.Path,
					"error":   "request timeout",
				})

				// Use thread-safe access to status writer
				swMutex.Lock()
				localSW := sw
				swMutex.Unlock()

				if localSW != nil {
					localSW.mu.Lock()
					if !localSW.wroteHeader {
						// Directly access underlying ResponseWriter since we already hold the lock
						localSW.ResponseWriter.Header().Set("Content-Type", "text/plain; charset=utf-8")
						localSW.ResponseWriter.Header().Set("X-Content-Type-Options", "nosniff")
						localSW.status = http.StatusGatewayTimeout
						localSW.wroteHeader = true
						localSW.ResponseWriter.WriteHeader(http.StatusGatewayTimeout)
						fmt.Fprintln(localSW.ResponseWriter, "Request timeout")
					}
					localSW.mu.Unlock()
				} else {
					// Fallback to direct response writer (shouldn't happen in normal flow)
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					w.Header().Set("X-Content-Type-Options", "nosniff")
					w.WriteHeader(http.StatusGatewayTimeout)
					fmt.Fprintln(w, "Request timeout")
				}
				return
			}

			// Emit success or failure event based on status code
			swMutex.Lock()
			localSW := sw
			swMutex.Unlock()

			if localSW != nil {
				localSW.mu.Lock()
				status := localSW.status
				localSW.mu.Unlock()

				if status >= 400 {
					m.emitEvent(r.Context(), EventTypeRequestFailed, map[string]interface{}{
						"backend": backend,
						"method":  r.Method,
						"path":    r.URL.Path,
						"status":  status,
						"error":   fmt.Sprintf("upstream returned status %d", status),
					})
				} else {
					m.emitEvent(r.Context(), EventTypeRequestProxied, map[string]interface{}{
						"backend": backend,
						"method":  r.Method,
						"path":    r.URL.Path,
						"status":  status,
					})
				}
			}
		}
	}

	// Wrap with cache if enabled
	if m.responseCache != nil {
		return m.withCache(handler, backend)
	}

	return handler
}

// createBackendProxyHandler creates an http.HandlerFunc that handles proxying requests
// to a specific backend, with support for tenant-specific backends
func (m *ReverseProxyModule) createBackendProxyHandlerForTenant(tenantID modular.TenantID, backend string) http.HandlerFunc {
	// Get the appropriate proxy for this backend and tenant
	proxy, proxyExists := m.getProxyForBackendAndTenant(backend, tenantID)

	// Check if circuit breaker is enabled for this backend
	var cb *CircuitBreaker
	if m.config.CircuitBreakerConfig.Enabled {
		// Check for backend-specific circuit breaker
		var cbConfig CircuitBreakerConfig
		if backendCB, exists := m.config.BackendCircuitBreakers[backend]; exists {
			cbConfig = backendCB
		} else {
			cbConfig = m.config.CircuitBreakerConfig
		}

		// Get or create circuit breaker for this backend
		if existingCB, exists := m.circuitBreakers[backend]; exists {
			cb = existingCB
		} else {
			// Use module's request timeout if circuit breaker config doesn't specify one
			if cbConfig.RequestTimeout == 0 && m.config.RequestTimeout > 0 {
				cbConfig.RequestTimeout = m.config.RequestTimeout
			}

			// Create new circuit breaker with config and store for reuse
			cb = NewCircuitBreakerWithConfig(backend, cbConfig, m.metrics)
			cb.eventEmitter = func(eventType string, data map[string]interface{}) {
				m.emitEvent(context.Background(), eventType, data) //nolint:contextcheck // circuit breaker events occur outside request handling
			}
			m.circuitBreakers[backend] = cb
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Emit request received event (tenant-aware)
		m.emitEvent(r.Context(), EventTypeRequestReceived, map[string]interface{}{
			"backend": backend,
			"method":  r.Method,
			"path":    r.URL.Path,
			"tenant":  string(tenantID),
		})

		// Get tenant-specific merged config (fallback to global if not found)
		tenantCfg := m.config
		if mergedCfg, exists := m.tenants[tenantID]; exists && mergedCfg != nil {
			tenantCfg = mergedCfg
		}

		// Apply timeout configuration - check for route-specific timeout first
		var requestTimeout time.Duration
		var timeoutSource string
		if tenantCfg.RouteConfigs != nil {
			// Find matching route config by checking all patterns
			for routePattern, routeConfig := range tenantCfg.RouteConfigs {
				if m.matchesRoute(r.URL.Path, routePattern) && routeConfig.Timeout > 0 {
					requestTimeout = routeConfig.Timeout
					timeoutSource = fmt.Sprintf("route %s", routePattern)
					break
				}
			}
		}

		// Fall back to global timeout if no route-specific timeout
		if requestTimeout == 0 {
			if tenantCfg.GlobalTimeout > 0 {
				requestTimeout = tenantCfg.GlobalTimeout
				timeoutSource = "global"
			} else if tenantCfg.RequestTimeout > 0 {
				requestTimeout = tenantCfg.RequestTimeout
				timeoutSource = "request"
			} else {
				requestTimeout = 30 * time.Second // Default fallback
				timeoutSource = "default"
			}
		}

		// Debug timeout configuration
		if m.app != nil && m.app.Logger() != nil {
			safePath := sanitizeForLogging(r.URL.Path)
			obfuscatedTenantID := obfuscateTenantID(tenantID)
			m.app.Logger().Debug("Request timeout configuration",
				"path", safePath,
				"backend", backend,
				"tenant_hash", obfuscatedTenantID,
				"timeout", requestTimeout.String(),
				"timeout_source", timeoutSource)
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()
		r = r.WithContext(ctx)

		// Record request to backend for health checking
		if m.healthChecker != nil {
			m.healthChecker.RecordBackendRequest(backend)
		}

		if !proxyExists {
			http.Error(w, fmt.Sprintf("Backend %s not found", backend), http.StatusInternalServerError)
			return
		}

		// If circuit breaker is available, wrap the proxy request with it
		if cb != nil {
			// Create a custom RoundTripper that applies circuit breaking
			originalTransport := proxy.Transport
			if originalTransport == nil {
				originalTransport = http.DefaultTransport
			}

			// Execute the request via circuit breaker
			resp, err := cb.Execute(r, func(req *http.Request) (*http.Response, error) {
				// Create a ResponseWriter wrapper to capture response
				recorder := httptest.NewRecorder()

				// Create a copy of the proxy with the original transport
				proxyCopy := &httputil.ReverseProxy{
					Director:       proxy.Director,
					Transport:      originalTransport,
					FlushInterval:  proxy.FlushInterval,
					ErrorLog:       proxy.ErrorLog,
					BufferPool:     proxy.BufferPool,
					ModifyResponse: proxy.ModifyResponse,
					ErrorHandler:   proxy.ErrorHandler,
				}

				// Serve the request
				proxyCopy.ServeHTTP(recorder, req) //nolint:gosec // G704: reverse proxy intentionally forwards requests to configured backends

				// Convert recorder to response
				return recorder.Result(), nil
			})

			if errors.Is(err, ErrCircuitOpen) {
				// Circuit is open, return service unavailable
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Warn("Circuit breaker open, denying request",
						"backend", backend, "tenant_hash", obfuscateTenantID(tenantID), "path", sanitizeForLogging(r.URL.Path))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				if _, err := w.Write([]byte(`{"error":"Service temporarily unavailable","code":"CIRCUIT_OPEN"}`)); err != nil {
					if m.app != nil && m.app.Logger() != nil {
						m.app.Logger().Error("Failed to write circuit breaker response", "error", err)
					}
				}
				// Emit failed event for tenant path when circuit is open
				m.emitEvent(ctx, EventTypeRequestFailed, map[string]interface{}{
					"backend": backend,
					"method":  r.Method,
					"path":    r.URL.Path,
					"tenant":  string(tenantID),
					"status":  http.StatusServiceUnavailable,
					"error":   "circuit open",
				})
				return
			} else if err != nil {
				// Some other error occurred
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				m.emitEvent(ctx, EventTypeRequestFailed, map[string]interface{}{
					"backend": backend,
					"method":  r.Method,
					"path":    r.URL.Path,
					"tenant":  string(tenantID),
					"status":  http.StatusInternalServerError,
					"error":   err.Error(),
				})
				return
			}

			// Copy response to the original ResponseWriter
			copyResponseHeaders(resp.Header, w.Header())
			w.WriteHeader(resp.StatusCode)
			if resp.Body != nil {
				defer resp.Body.Close()
				_, err := io.Copy(w, resp.Body)
				if err != nil {
					// Log error but continue processing
					m.app.Logger().Error("Failed to copy response body", "error", err)
				}
			}

			// Emit event based on response status
			if resp.StatusCode >= 400 {
				m.emitEvent(ctx, EventTypeRequestFailed, map[string]interface{}{
					"backend": backend,
					"method":  r.Method,
					"path":    r.URL.Path,
					"tenant":  string(tenantID),
					"status":  resp.StatusCode,
					"error":   fmt.Sprintf("upstream returned status %d", resp.StatusCode),
				})
			} else {
				m.emitEvent(ctx, EventTypeRequestProxied, map[string]interface{}{
					"backend": backend,
					"method":  r.Method,
					"path":    r.URL.Path,
					"tenant":  string(tenantID),
					"status":  resp.StatusCode,
				})
			}
		} else {
			// No circuit breaker, use the proxy directly but capture status
			sw := &statusCapturingResponseWriter{ResponseWriter: w, status: http.StatusOK}
			proxy.ServeHTTP(sw, r) //nolint:gosec // G704: reverse proxy intentionally forwards requests to configured backends

			// Emit success or failure event based on status code
			if sw.status >= 400 {
				m.emitEvent(ctx, EventTypeRequestFailed, map[string]interface{}{
					"backend": backend,
					"method":  r.Method,
					"path":    r.URL.Path,
					"tenant":  string(tenantID),
					"status":  sw.status,
					"error":   fmt.Sprintf("upstream returned status %d", sw.status),
				})
			} else {
				m.emitEvent(ctx, EventTypeRequestProxied, map[string]interface{}{
					"backend": backend,
					"method":  r.Method,
					"path":    r.URL.Path,
					"tenant":  string(tenantID),
					"status":  sw.status,
				})
			}
		}
	}
}

// getProxyForBackendAndTenant returns the appropriate proxy for a backend and tenant.
// If a tenant-specific proxy exists, it will be returned; otherwise, the default proxy.
func (m *ReverseProxyModule) getProxyForBackendAndTenant(backendID string, tenantID modular.TenantID) (*httputil.ReverseProxy, bool) {
	// First check for a tenant-specific proxy if tenantID is provided
	if tenantID != "" {
		m.tenantProxiesMutex.RLock()
		tenantProxies, tenantExists := m.tenantBackendProxies[tenantID]
		m.tenantProxiesMutex.RUnlock()
		if tenantExists {
			if proxy, exists := tenantProxies[backendID]; exists && proxy != nil {
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Debug("Using tenant-specific proxy", "tenant_hash", obfuscateTenantID(tenantID), "backend", backendID)
				}
				return proxy, true
			} else {
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Debug("No tenant-specific proxy found", "tenant_hash", obfuscateTenantID(tenantID), "backend", backendID, "tenantProxiesExist", true, "proxyExistsForBackend", exists)
				}
			}
		} else {
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Debug("No tenant proxies found", "tenant_hash", obfuscateTenantID(tenantID), "backend", backendID)
			}
		}
	}

	// Fall back to the default proxy
	m.backendProxiesMutex.RLock()
	proxy, exists := m.backendProxies[backendID]
	m.backendProxiesMutex.RUnlock()
	if m.app != nil && m.app.Logger() != nil {
		m.app.Logger().Debug("Using global proxy", "backend", backendID, "exists", exists, "tenant_hash", obfuscateTenantID(tenantID))
	}
	return proxy, exists
}

// AddBackendRoute registers a new route for a specific backend.
// It allows dynamically adding routes to the reverse proxy after initialization.
func (m *ReverseProxyModule) AddBackendRoute(backendID, routePattern string) error {
	// Check if backend exists
	m.backendProxiesMutex.RLock()
	proxy, ok := m.backendProxies[backendID]
	m.backendProxiesMutex.RUnlock()
	if !ok {
		m.app.Logger().Error("Backend not found", "backend", backendID)
		return fmt.Errorf("%w: %s", ErrBackendNotFound, backendID)
	}

	// If proxy is nil, log the error and return
	if proxy == nil {
		m.app.Logger().Error("Backend proxy is nil", "backend", backendID)
		return fmt.Errorf("%w: %s", ErrBackendProxyNil, backendID)
	}

	// Create the handler function
	handler := m.createBackendProxyHandler(backendID)

	// Store the handler in the backend routes map
	if _, ok := m.backendRoutes[backendID]; !ok {
		m.backendRoutes[backendID] = make(map[string]http.HandlerFunc)
	}
	m.backendRoutes[backendID][routePattern] = handler

	// Register the handler with the router immediately if router is available
	if m.router != nil {
		m.safeHandleFunc(routePattern, handler)
		if m.app != nil {
			m.app.Logger().Info("Dynamically added route", "backend", backendID, "pattern", routePattern)
		}
	} else {
		if m.app != nil {
			m.app.Logger().Warn("Router not available, route will be registered on Start", "backend", backendID, "pattern", routePattern)
		}
	}
	return nil
}

// AddCompositeRoute adds a composite route that combines responses from multiple backends.
// The strategy parameter determines how the responses are combined.
func (m *ReverseProxyModule) AddCompositeRoute(pattern string, backends []string, strategy string) {
	// Initialize CompositeRoutes if nil
	if m.config.CompositeRoutes == nil {
		m.config.CompositeRoutes = make(map[string]CompositeRoute)
	}

	// Create the composite route configuration
	m.config.CompositeRoutes[pattern] = CompositeRoute{
		Pattern:  pattern,
		Backends: backends,
		Strategy: strategy,
	}
}

// RegisterCustomEndpoint adds a custom endpoint with a response transformer.
// This provides the most flexibility for combining and transforming responses
// from multiple backends using custom logic.
func (m *ReverseProxyModule) RegisterCustomEndpoint(pattern string, mapping EndpointMapping) {
	// Create a handler that will execute the requests to all configured endpoints
	// and then apply the response transformer
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Track responses from each backend
		responses := make(map[string]*http.Response)

		// We'll use this to track which responses need to be closed
		// so we can ensure proper cleanup even in error cases
		var responsesToClose []*http.Response
		defer func() {
			// Close all response bodies to avoid resource leaks
			for _, resp := range responsesToClose {
				if resp != nil && resp.Body != nil {
					resp.Body.Close()
				}
			}
		}()

		// Create a context with timeout for our requests
		timeout := 10 * time.Second // Default timeout
		if m.config.RequestTimeout > 0 {
			timeout = m.config.RequestTimeout
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		// Use the context when executing backend requests
		r = r.WithContext(ctx)

		// Get tenant ID if present
		tenantIDStr, hasTenant := TenantIDFromRequest(m.config.TenantIDHeader, r)
		tenantID := modular.TenantID(tenantIDStr)

		// Check if tenant ID is required but not provided
		if m.config.RequireTenantID && !hasTenant {
			http.Error(w, fmt.Sprintf("Header %s is required", m.config.TenantIDHeader), http.StatusBadRequest)
			return
		}

		// Execute all endpoint requests
		for _, endpoint := range mapping.Endpoints {
			// Get the backend service URL
			backendURL := ""

			// First check if we have a tenant-specific service URL
			if hasTenant {
				if tenantCfg, ok := m.tenants[tenantID]; ok && tenantCfg != nil {
					if tenantURL, ok := tenantCfg.BackendServices[endpoint.Backend]; ok && tenantURL != "" {
						backendURL = tenantURL
					}
				}
			}

			// Fall back to default service URL if no tenant-specific one found
			if backendURL == "" {
				var ok bool
				backendURL, ok = m.config.BackendServices[endpoint.Backend]
				if !ok {
					m.app.Logger().Warn("Backend not found in service configuration", "backend", endpoint.Backend)
					continue
				}
			}

			// Create the target URL
			targetURL, err := url.Parse(backendURL)
			if err != nil {
				m.app.Logger().Error("Failed to parse URL", "backend", endpoint.Backend, "url", backendURL, "error", err)
				continue
			}

			// Append the endpoint path
			targetURL.Path = path.Join(targetURL.Path, endpoint.Path)

			// Add query parameters if specified
			if len(endpoint.QueryParams) > 0 {
				q := targetURL.Query()
				for key, value := range endpoint.QueryParams {
					q.Set(key, value)
				}
				targetURL.RawQuery = q.Encode()
			} else {
				// Copy query params from original request
				targetURL.RawQuery = r.URL.RawQuery
			}

			// Create the request
			req, err := http.NewRequestWithContext(ctx, endpoint.Method, targetURL.String(), nil)
			if err != nil {
				m.app.Logger().Error("Failed to create request", "backend", endpoint.Backend, "error", err)
				continue
			}

			// Copy headers from original request
			for key, values := range r.Header {
				for _, value := range values {
					req.Header.Add(key, value)
				}
			}

			// Add custom headers if specified
			if endpoint.Headers != nil {
				for key, value := range endpoint.Headers {
					req.Header.Set(key, value)
				}
			}

			// Execute the request
			resp, err := m.httpClient.Do(req) //nolint:bodyclose,gosec // bodyclose: body is closed in defer cleanup; G704: reverse proxy intentionally forwards requests to configured backends
			if err != nil {
				m.app.Logger().Error("Failed to execute request", "backend", endpoint.Backend, "error", err)
				continue
			}

			// Add to the list of responses that need to be closed immediately
			responsesToClose = append(responsesToClose, resp) //nolint:bodyclose // Response body is closed in defer cleanup

			// Store the response
			responses[endpoint.Backend] = resp
		}

		// Apply the response transformer
		result, err := mapping.ResponseTransformer(ctx, r, responses)
		if err != nil {
			m.app.Logger().Error("Failed to transform response", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Write headers
		copyResponseHeaders(result.Headers, w.Header())

		// Ensure Content-Type is set if not specified by transformer
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}

		// Write status code and body
		w.WriteHeader(result.StatusCode)
		if _, err := w.Write(result.Body); err != nil { //nolint:gosec // G705: reverse proxy transparently forwards composite backend response body
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Error("Failed to write response body", "error", err)
			}
		}
	}

	// Register the handler with the router
	m.compositeRoutes[pattern] = handler

	// Log the registration
	m.app.Logger().Info("Registered custom endpoint", "pattern", pattern, "backends", len(mapping.Endpoints))
}

// mergeConfigs merges a tenant-specific configuration with the global configuration.
// It ensures that tenant-specific settings override global ones while preserving
// global settings that aren't specified in the tenant config.
func mergeConfigs(global, tenant *ReverseProxyConfig) *ReverseProxyConfig {
	// Start with a copy of the global config
	merged := &ReverseProxyConfig{
		BackendServices:        make(map[string]string),
		Routes:                 make(map[string]string),
		RouteConfigs:           make(map[string]RouteConfig),
		CompositeRoutes:        make(map[string]CompositeRoute),
		BackendCircuitBreakers: make(map[string]CircuitBreakerConfig),
		BackendConfigs:         make(map[string]BackendServiceConfig),
	}

	// Copy global backend services
	for id, backendUrl := range global.BackendServices {
		merged.BackendServices[id] = backendUrl
	}

	// Override with tenant-specific backend services
	if tenant.BackendServices != nil {
		for id, backendUrl := range tenant.BackendServices {
			// Only override if the tenant has specified a non-empty URL
			if backendUrl != "" {
				merged.BackendServices[id] = backendUrl
			}
		}
	}

	// Set default backend - prefer tenant's, but fallback to global if tenant doesn't specify
	if tenant.DefaultBackend != "" {
		merged.DefaultBackend = tenant.DefaultBackend
	} else {
		merged.DefaultBackend = global.DefaultBackend
	}

	// Copy global routes first
	for pattern, backend := range global.Routes {
		merged.Routes[pattern] = backend
	}
	// Then override with tenant routes
	if tenant.Routes != nil {
		for pattern, backend := range tenant.Routes {
			merged.Routes[pattern] = backend
		}
	}

	// Copy global route configs first
	for pattern, routeConfig := range global.RouteConfigs {
		merged.RouteConfigs[pattern] = routeConfig
	}
	// Then override with tenant route configs
	if tenant.RouteConfigs != nil {
		for pattern, routeConfig := range tenant.RouteConfigs {
			merged.RouteConfigs[pattern] = routeConfig
		}
	}

	// Copy global composite routes first
	for pattern, route := range global.CompositeRoutes {
		merged.CompositeRoutes[pattern] = route
	}
	// Then override with tenant composite routes
	if tenant.CompositeRoutes != nil {
		for pattern, route := range tenant.CompositeRoutes {
			merged.CompositeRoutes[pattern] = route
		}
	}

	// Tenant ID header - prefer tenant's if specified
	if tenant.TenantIDHeader != "" {
		merged.TenantIDHeader = tenant.TenantIDHeader
	} else {
		merged.TenantIDHeader = global.TenantIDHeader
	}

	// Require tenant ID - prefer tenant's if specified
	merged.RequireTenantID = tenant.RequireTenantID || global.RequireTenantID

	// Cache settings - prefer tenant's if specified
	if tenant.CacheEnabled {
		merged.CacheEnabled = true
		if tenant.CacheTTL > 0 {
			merged.CacheTTL = tenant.CacheTTL
		} else {
			merged.CacheTTL = global.CacheTTL
		}
	} else {
		merged.CacheEnabled = global.CacheEnabled
		merged.CacheTTL = global.CacheTTL
	}

	// Request timeout - prefer tenant's if specified
	if tenant.RequestTimeout > 0 {
		merged.RequestTimeout = tenant.RequestTimeout
	} else {
		merged.RequestTimeout = global.RequestTimeout
	}

	// Global timeout - prefer tenant's if specified
	if tenant.GlobalTimeout > 0 {
		merged.GlobalTimeout = tenant.GlobalTimeout
	} else {
		merged.GlobalTimeout = global.GlobalTimeout
	}

	// Metrics settings
	if tenant.MetricsEnabled {
		merged.MetricsEnabled = true
		if tenant.MetricsPath != "" {
			merged.MetricsPath = tenant.MetricsPath
		} else {
			merged.MetricsPath = global.MetricsPath
		}
		if tenant.MetricsEndpoint != "" {
			merged.MetricsEndpoint = tenant.MetricsEndpoint
		} else {
			merged.MetricsEndpoint = global.MetricsEndpoint
		}
	} else {
		merged.MetricsEnabled = global.MetricsEnabled
		merged.MetricsPath = global.MetricsPath
		merged.MetricsEndpoint = global.MetricsEndpoint
	}

	// Circuit breaker config - prefer tenant's if specified
	if tenant.CircuitBreakerConfig.Enabled {
		merged.CircuitBreakerConfig = tenant.CircuitBreakerConfig
	} else {
		merged.CircuitBreakerConfig = global.CircuitBreakerConfig
	}

	// Merge backend circuit breakers - tenant settings override global ones
	for backend, config := range global.BackendCircuitBreakers {
		merged.BackendCircuitBreakers[backend] = config
	}
	if tenant.BackendCircuitBreakers != nil {
		for backend, config := range tenant.BackendCircuitBreakers {
			merged.BackendCircuitBreakers[backend] = config
		}
	}

	// Health check config - prefer tenant's if specified
	if tenant.HealthCheck.Enabled {
		merged.HealthCheck = tenant.HealthCheck
	} else {
		merged.HealthCheck = global.HealthCheck
	}

	// Merge backend configurations - tenant settings override global ones
	for backendID, globalConfig := range global.BackendConfigs {
		merged.BackendConfigs[backendID] = globalConfig
	}
	for backendID, tenantConfig := range tenant.BackendConfigs {
		merged.BackendConfigs[backendID] = tenantConfig
	}

	return merged
}

// registerMetricsEndpoint registers an HTTP endpoint to expose collected metrics
func (m *ReverseProxyModule) registerMetricsEndpoint(endpoint string) {
	if endpoint == "" {
		endpoint = "/metrics/reverseproxy"
	}

	metricsHandler := func(w http.ResponseWriter, r *http.Request) {
		// Get current metrics data
		metrics := m.metrics.GetMetrics()

		// Convert to JSON
		jsonData, err := json.Marshal(metrics)
		if err != nil {
			m.app.Logger().Error("Failed to marshal metrics data", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Set content type and write response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(jsonData); err != nil {
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Error("Failed to write metrics response", "error", err)
			}
		}
	}

	// Register the metrics endpoint with the router
	if m.router != nil {
		m.safeHandleFunc(endpoint, metricsHandler)
		m.app.Logger().Info("Registered metrics endpoint", "endpoint", endpoint)
	}

	// Register health check endpoint if health checking is enabled
	if m.healthChecker != nil {
		healthEndpoint := endpoint + "/health"
		healthHandler := func(w http.ResponseWriter, r *http.Request) {
			// Get overall health status including circuit breaker information
			overallHealth := m.healthChecker.GetOverallHealthStatus(true)

			// Convert to JSON
			jsonData, err := json.Marshal(overallHealth)
			if err != nil {
				m.app.Logger().Error("Failed to marshal health status data", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// Set content type
			w.Header().Set("Content-Type", "application/json")

			// Set status code based on overall health
			if overallHealth.Healthy {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}

			if _, err := w.Write(jsonData); err != nil {
				m.app.Logger().Error("Failed to write health status response", "error", err)
			}
		}

		m.safeHandleFunc(healthEndpoint, healthHandler)
		m.app.Logger().Info("Registered health check endpoint", "endpoint", healthEndpoint)
	}
}

// registerDebugEndpoints registers debug endpoints if they are enabled
func (m *ReverseProxyModule) registerDebugEndpoints() error {
	if m.router == nil {
		return ErrCannotRegisterRoutes
	}
	// Additional check for nil interface
	if reflect.ValueOf(m.router).IsNil() {
		return fmt.Errorf("%w: router interface is nil", ErrCannotRegisterRoutes)
	}

	// Get tenant service if available
	var tenantService modular.TenantService
	if m.app != nil {
		err := m.app.GetService("tenantService", &tenantService)
		if err != nil {
			m.app.Logger().Warn("TenantService not available for debug endpoints", "error", err)
		}
	}

	// Create debug handler
	debugHandler := NewDebugHandler(
		m.config.DebugEndpoints,
		m.featureFlagEvaluator,
		m.config,
		tenantService,
		m.app.Logger(),
	)

	// Set circuit breakers and health checkers for debugging
	if len(m.circuitBreakers) > 0 {
		debugHandler.SetCircuitBreakers(m.circuitBreakers)
	}
	if m.healthChecker != nil {
		// Create a map with the health checker
		healthCheckers := map[string]*HealthChecker{
			"reverseproxy": m.healthChecker,
		}
		debugHandler.SetHealthCheckers(healthCheckers)
	}

	// Register debug endpoints individually since our routerService doesn't support http.ServeMux
	basePath := m.config.DebugEndpoints.BasePath

	// Feature flags debug endpoint
	flagsEndpoint := basePath + "/flags"
	m.safeHandleFunc(flagsEndpoint, debugHandler.HandleFlags)
	m.app.Logger().Info("Registered debug endpoint", "endpoint", flagsEndpoint)

	// General debug info endpoint
	infoEndpoint := basePath + "/info"
	m.safeHandleFunc(infoEndpoint, debugHandler.HandleInfo)
	m.app.Logger().Info("Registered debug endpoint", "endpoint", infoEndpoint)

	// Backend status endpoint
	backendsEndpoint := basePath + "/backends"
	m.safeHandleFunc(backendsEndpoint, debugHandler.HandleBackends)
	m.app.Logger().Info("Registered debug endpoint", "endpoint", backendsEndpoint)

	// Circuit breaker status endpoint
	circuitBreakersEndpoint := basePath + "/circuit-breakers"
	m.safeHandleFunc(circuitBreakersEndpoint, debugHandler.HandleCircuitBreakers)
	m.app.Logger().Info("Registered debug endpoint", "endpoint", circuitBreakersEndpoint)

	// Health check status endpoint
	healthChecksEndpoint := basePath + "/health-checks"
	m.safeHandleFunc(healthChecksEndpoint, debugHandler.HandleHealthChecks)
	m.app.Logger().Info("Registered debug endpoint", "endpoint", healthChecksEndpoint)

	m.app.Logger().Info("Debug endpoints registered", "basePath", basePath)
	return nil
}

// createTenantAwareHandler creates a handler that routes based on tenant-specific configuration for a specific path
func (m *ReverseProxyModule) createTenantAwareHandler(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Debug("Tenant-aware handler called", "path", path, "requestPath", sanitizeForLogging(r.URL.Path))
		}
		// Extract tenant ID from request
		tenantIDStr, hasTenant := TenantIDFromRequest(m.config.TenantIDHeader, r)

		// Check tenant header enforcement first
		if m.config.RequireTenantID && !hasTenant {
			http.Error(w, fmt.Sprintf("Header %s is required", m.config.TenantIDHeader), http.StatusBadRequest)
			return
		}

		// Get the appropriate configuration (tenant-specific or global)
		var effectiveConfig *ReverseProxyConfig
		if hasTenant {
			tenantID := modular.TenantID(tenantIDStr)
			if tenantCfg, exists := m.tenants[tenantID]; exists && tenantCfg != nil {
				effectiveConfig = tenantCfg
			} else {
				effectiveConfig = m.config
			}
		} else {
			effectiveConfig = m.config
		}

		// First priority: Check route configs with feature flag evaluation
		if effectiveConfig.RouteConfigs != nil {
			if routeConfig, ok := effectiveConfig.RouteConfigs[path]; ok {
				// Get the primary backend from the static routes
				if primaryBackend, routeExists := effectiveConfig.Routes[path]; routeExists {
					// Evaluate feature flag to determine which backend to use
					if routeConfig.FeatureFlagID != "" {
						if !m.evaluateFeatureFlag(routeConfig.FeatureFlagID, r) {
							// Feature flag is disabled, use alternative backend
							alternativeBackend := m.getAlternativeBackend(routeConfig.AlternativeBackend)
							if alternativeBackend != "" {
								m.app.Logger().Debug("Feature flag disabled for route, using alternative backend",
									"path", path, "flagID", routeConfig.FeatureFlagID,
									"primary", primaryBackend, "alternative", alternativeBackend)

								// Check if dry run is enabled for this route
								if routeConfig.DryRun && m.dryRunHandler != nil {
									// Determine which backend to compare against
									dryRunBackend := routeConfig.DryRunBackend
									if dryRunBackend == "" {
										dryRunBackend = primaryBackend // Default to primary for comparison
									}

									m.app.Logger().Debug("Processing dry run request (feature flag disabled)",
										"path", path, "returnBackend", alternativeBackend, "compareBackend", dryRunBackend)

									// Use dry run handler - return alternative backend response, compare with dry run backend
									m.handleDryRunRequest(r.Context(), w, r, routeConfig, alternativeBackend, dryRunBackend)
									return
								}

								if hasTenant {
									handler := m.createBackendProxyHandlerForTenant(modular.TenantID(tenantIDStr), alternativeBackend) //nolint:contextcheck // handler captures request context via *http.Request
									handler(w, r)
									return
								} else {
									handler := m.createBackendProxyHandler(alternativeBackend)
									handler(w, r)
									return
								}
							} else {
								// No alternative backend available
								http.Error(w, "Backend temporarily unavailable", http.StatusServiceUnavailable)
								return
							}
						} else {
							// Feature flag is enabled, use primary backend
							m.app.Logger().Debug("Feature flag enabled for route, using primary backend",
								"path", path, "flagID", routeConfig.FeatureFlagID, "backend", primaryBackend)
						}
					}
					// Use primary backend (either feature flag was enabled or no feature flag specified)
					// Check if dry run is enabled for this route
					if routeConfig.DryRun && m.dryRunHandler != nil {
						// Determine which backend to compare against
						dryRunBackend := routeConfig.DryRunBackend
						if dryRunBackend == "" {
							dryRunBackend = m.getAlternativeBackend(routeConfig.AlternativeBackend) // Default to alternative for comparison
						}

						if dryRunBackend != "" && dryRunBackend != primaryBackend {
							m.app.Logger().Debug("Processing dry run request (feature flag enabled or no flag)",
								"path", path, "returnBackend", primaryBackend, "compareBackend", dryRunBackend)

							// Use dry run handler - return primary backend response, compare with dry run backend
							m.handleDryRunRequest(r.Context(), w, r, routeConfig, primaryBackend, dryRunBackend)
							return
						}
					}

					if hasTenant {
						handler := m.createBackendProxyHandlerForTenant(modular.TenantID(tenantIDStr), primaryBackend) //nolint:contextcheck // handler captures request context via *http.Request
						handler(w, r)
						return
					} else {
						handler := m.createBackendProxyHandler(primaryBackend)
						handler(w, r)
						return
					}
				}
			}
		}

		if hasTenant {
			tenantID := modular.TenantID(tenantIDStr)

			// Check if we have a tenant-specific configuration
			if tenantCfg, exists := m.tenants[tenantID]; exists && tenantCfg != nil {
				// Check for tenant-specific route
				if tenantCfg.Routes != nil {
					if backendID, ok := tenantCfg.Routes[path]; ok {
						// Use tenant-specific backend for this path
						handler := m.createBackendProxyHandlerForTenant(tenantID, backendID) //nolint:contextcheck // handler captures request context via *http.Request
						handler(w, r)
						return
					}
				}

				// No tenant-specific route found, fall back to global routes first
				// before using tenant default backend
			}
		}

		// Fall back to global configuration
		// Check if there's a global route for this path
		if backendID, ok := m.config.Routes[path]; ok {
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Debug("Using global route", "path", path, "backend", backendID, "tenant_hash", obfuscateTenantID(modular.TenantID(tenantIDStr)))
			}
			m.backendProxiesMutex.RLock()
			_, exists := m.backendProxies[backendID]
			m.backendProxiesMutex.RUnlock()
			if exists {
				handler := m.createBackendProxyHandler(backendID)
				handler(w, r)
				return
			} else {
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Error("Global backend proxy not found", "backend", backendID)
				}
			}
		} else {
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Debug("No global route found", "path", path, "tenant_hash", obfuscateTenantID(modular.TenantID(tenantIDStr)))
			}
		}

		// Check if there's a composite route
		if compositeHandler, ok := m.compositeRoutes[path]; ok {
			compositeHandler.ServeHTTP(w, r)
			return
		}

		// After global routes are checked, check for tenant default backend
		if hasTenant {
			tenantID := modular.TenantID(tenantIDStr)
			if tenantCfg, exists := m.tenants[tenantID]; exists && tenantCfg != nil {
				// Check if tenant has default backend
				if tenantCfg.DefaultBackend != "" {
					handler := m.createBackendProxyHandlerForTenant(tenantID, tenantCfg.DefaultBackend) //nolint:contextcheck // tenant handler leverages request context
					handler(w, r)
					return
				}
			}
		}

		// Fall back to global default backend
		if m.defaultBackend != "" {
			m.backendProxiesMutex.RLock()
			_, exists := m.backendProxies[m.defaultBackend]
			m.backendProxiesMutex.RUnlock()
			if exists {
				if hasTenant {
					// Even for global default backend, use tenant-aware handler to get proper tenant proxy
					if m.app != nil && m.app.Logger() != nil {
						m.app.Logger().Debug("Using tenant-aware global default backend", "backend", m.defaultBackend, "tenant_hash", obfuscateTenantID(modular.TenantID(tenantIDStr)))
					}
					handler := m.createBackendProxyHandlerForTenant(modular.TenantID(tenantIDStr), m.defaultBackend) //nolint:contextcheck // handler obtains context from incoming request
					handler(w, r)
					return
				} else {
					if m.app != nil && m.app.Logger() != nil {
						m.app.Logger().Debug("Using global default backend", "backend", m.defaultBackend)
					}
					handler := m.createBackendProxyHandler(m.defaultBackend)
					handler(w, r)
					return
				}
			}
		}

		// No handler found
		http.NotFound(w, r)
	}
}

// createTenantAwareCatchAllHandler creates a catch-all handler that supports tenant-specific default backends
func (m *ReverseProxyModule) createTenantAwareCatchAllHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract tenant ID from request
		tenantIDStr, hasTenant := TenantIDFromRequest(m.config.TenantIDHeader, r)

		// Check tenant header enforcement first
		if m.config.RequireTenantID && !hasTenant {
			http.Error(w, fmt.Sprintf("Header %s is required", m.config.TenantIDHeader), http.StatusBadRequest)
			return
		}

		if hasTenant {
			tenantID := modular.TenantID(tenantIDStr)
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Debug("Processing tenant request", "tenant_hash", obfuscateTenantID(tenantID), "path", sanitizeForLogging(r.URL.Path))
			}

			// Check if we have a tenant-specific configuration
			if tenantCfg, exists := m.tenants[tenantID]; exists && tenantCfg != nil {
				// Check if tenant has a default backend (use it regardless of global default)
				if tenantCfg.DefaultBackend != "" {
					if m.app != nil && m.app.Logger() != nil {
						m.app.Logger().Debug("Using tenant default backend", "tenant_hash", obfuscateTenantID(tenantID), "backend", tenantCfg.DefaultBackend)
					}
					handler := m.createBackendProxyHandlerForTenant(tenantID, tenantCfg.DefaultBackend) //nolint:contextcheck // tenant default handler reuses request context
					handler(w, r)
					return
				}
			}
		} else {
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Debug("Processing non-tenant request", "path", r.URL.Path)
			}
		}

		// Fall back to global default backend
		if m.defaultBackend != "" {
			m.backendProxiesMutex.RLock()
			_, exists := m.backendProxies[m.defaultBackend]
			m.backendProxiesMutex.RUnlock()
			if exists {
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Debug("Using global default backend", "backend", m.defaultBackend)
				}
				handler := m.createBackendProxyHandler(m.defaultBackend)
				handler(w, r)
				return
			}
		}

		// No default backend available
		http.NotFound(w, r)
	}
}

// GetHealthStatus returns the health status of all backends.
func (m *ReverseProxyModule) GetHealthStatus() map[string]*HealthStatus {
	if m.healthChecker == nil {
		return nil
	}
	return m.healthChecker.GetHealthStatus()
}

// setupFeatureFlagEvaluation sets up the feature flag evaluation system using the aggregator pattern.
// It creates the internal file-based evaluator and registers it as "featureFlagEvaluator.file".
// If an external evaluator was provided via constructor, it registers it as "featureFlagEvaluator.external".
// Then it always creates an aggregator that discovers all evaluators and provides proper fallback behavior.
func (m *ReverseProxyModule) setupFeatureFlagEvaluation(ctx context.Context) error {
	if !m.config.FeatureFlags.Enabled {
		m.app.Logger().Debug("Feature flags disabled, skipping evaluation setup")
		return nil
	}

	// Convert the logger to *slog.Logger
	var logger *slog.Logger
	if slogLogger, ok := m.app.Logger().(*slog.Logger); ok {
		logger = slogLogger
	} else {
		// Fallback to a default logger if conversion fails
		logger = slog.Default()
	}

	// Always create the internal file-based evaluator
	fileEvaluator, err := NewFileBasedFeatureFlagEvaluator(ctx, m.app, logger)
	if err != nil {
		return fmt.Errorf("failed to create file-based feature flag evaluator: %w", err)
	}

	// Register the file evaluator as "featureFlagEvaluator.file"
	if err := m.app.RegisterService("featureFlagEvaluator.file", fileEvaluator); err != nil {
		return fmt.Errorf("failed to register file-based evaluator service: %w", err)
	}

	// If we received an external evaluator via constructor, register it so the aggregator can discover it
	if m.featureFlagEvaluatorProvided && m.featureFlagEvaluator != nil {
		// Register the external evaluator with a unique name so the aggregator can find it
		if err := m.app.RegisterService("featureFlagEvaluator.external", m.featureFlagEvaluator); err != nil {
			return fmt.Errorf("failed to register external evaluator service: %w", err)
		}
		m.app.Logger().Info("Registered external feature flag evaluator for aggregation")
	}

	// Always create and use the aggregator - this ensures fallback behavior works correctly
	// The aggregator will discover all registered evaluators including external ones
	aggregator := NewFeatureFlagAggregator(m.app, logger)
	m.featureFlagEvaluator = aggregator

	if m.featureFlagEvaluatorProvided {
		m.app.Logger().Info("Created feature flag aggregator with external evaluator and file-based fallback")
	} else {
		m.app.Logger().Info("Created feature flag aggregator with file-based fallback")
	}

	return nil
}

// setupResponseCache initializes the response cache if caching is enabled
func (m *ReverseProxyModule) setupResponseCache() error {
	// Check if caching is enabled globally
	cachingEnabled := m.config.CacheEnabled
	cacheTTL := m.config.CacheTTL

	// Check if caching is enabled for any tenant
	if !cachingEnabled {
		for _, tenantConfig := range m.tenants {
			if tenantConfig != nil && tenantConfig.CacheEnabled {
				cachingEnabled = true
				// Use the first tenant's TTL if global TTL is not set
				if cacheTTL == 0 && tenantConfig.CacheTTL > 0 {
					cacheTTL = tenantConfig.CacheTTL
				}
				break
			}
		}
	}

	// Initialize cache if needed by any configuration
	if cachingEnabled {
		// Default cache size and cleanup interval
		maxCacheSize := 1000
		cleanupInterval := 5 * time.Minute

		// Use a reasonable default TTL if none specified
		if cacheTTL == 0 {
			cacheTTL = 60 * time.Second
		}

		m.responseCache = newResponseCache(cacheTTL, maxCacheSize, cleanupInterval)

		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Info("Response cache initialized (tenant-aware)",
				"globalCacheEnabled", m.config.CacheEnabled,
				"ttl", cacheTTL,
				"maxSize", maxCacheSize)
		}
	}

	return nil
}

// withCache wraps an HTTP handler with caching functionality
func (m *ReverseProxyModule) withCache(handler http.HandlerFunc, backend string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get effective config (considering tenant overrides)
		effectiveConfig := m.getEffectiveConfigForRequest(r)
		if effectiveConfig == nil || !effectiveConfig.CacheEnabled || m.responseCache == nil {
			// Caching disabled for this request or cache not initialized
			handler(w, r)
			return
		}

		// Only cache GET requests
		if r.Method != http.MethodGet {
			handler(w, r)
			return
		}

		// Generate cache key
		cacheKey := m.generateCacheKey(r, backend)

		// Check for cached response
		if cachedResp, found := m.responseCache.Get(cacheKey); found && cachedResp != nil {
			// Serve from cache
			copyResponseHeaders(cachedResp.Headers, w.Header())
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(cachedResp.StatusCode)
			if _, err := w.Write(cachedResp.Body); err != nil { //nolint:gosec // G705: reverse proxy transparently forwards upstream cached response body
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Error("Failed to write cached response body", "error", err)
				}
			}
			return
		}

		// Cache miss - capture response
		recorder := &cacheResponseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			headers:        make(http.Header),
			body:           make([]byte, 0),
		}

		// Call original handler
		handler(recorder, r)

		// Cache successful GET responses
		if recorder.statusCode == http.StatusOK && len(recorder.body) > 0 {
			m.responseCache.Set(cacheKey, recorder.statusCode, recorder.headers, recorder.body, effectiveConfig.CacheTTL)
		}

		// Send response to client
		copyResponseHeaders(recorder.headers, w.Header())
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(recorder.statusCode)
		if _, err := w.Write(recorder.body); err != nil {
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Error("Failed to write proxied response body", "error", err)
			}
		}
	}
}

// getEffectiveConfigForRequest returns the effective configuration for a request (considering tenant overrides)
func (m *ReverseProxyModule) getEffectiveConfigForRequest(r *http.Request) *ReverseProxyConfig {
	tenantIDStr, hasTenant := TenantIDFromRequest(m.config.TenantIDHeader, r)
	if hasTenant {
		tenantID := modular.TenantID(tenantIDStr)
		if tenantCfg, exists := m.tenants[tenantID]; exists && tenantCfg != nil {
			return tenantCfg
		}
	}
	return m.config
}

// generateCacheKey creates a unique cache key for the request
func (m *ReverseProxyModule) generateCacheKey(r *http.Request, backend string) string {
	// Include tenant ID in cache key for tenant isolation
	tenantIDStr, _ := TenantIDFromRequest(m.config.TenantIDHeader, r)
	key := fmt.Sprintf("%s:%s:%s:%s", backend, tenantIDStr, r.Method, r.URL.String())

	// Hash the key to keep it manageable
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// matchesRoute checks if a request path matches a route pattern
func (m *ReverseProxyModule) matchesRoute(requestPath, routePattern string) bool {
	// Handle exact matches
	if requestPath == routePattern {
		return true
	}

	// Handle wildcard patterns
	if strings.HasSuffix(routePattern, "/*") {
		prefix := strings.TrimSuffix(routePattern, "/*")
		return strings.HasPrefix(requestPath, prefix)
	}

	// Handle glob patterns if needed
	if strings.Contains(routePattern, "*") {
		if g, err := glob.Compile(routePattern); err == nil {
			return g.Match(requestPath)
		}
	}

	return false
}

// cacheResponseRecorder captures response data for caching
type cacheResponseRecorder struct {
	http.ResponseWriter
	statusCode int
	headers    http.Header
	body       []byte
}

func (r *cacheResponseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	// Copy headers from the underlying response writer
	for key, values := range r.Header() {
		r.headers[key] = values
	}
}

func (r *cacheResponseRecorder) Write(data []byte) (int, error) {
	// Capture the data for caching
	r.body = append(r.body, data...)
	return len(data), nil
}

func (r *cacheResponseRecorder) Header() http.Header {
	return r.ResponseWriter.Header()
}

// GetBackendHealthStatus returns the health status of a specific backend.
func (m *ReverseProxyModule) GetBackendHealthStatus(backendID string) (*HealthStatus, bool) {
	if m.healthChecker == nil {
		return nil, false
	}
	return m.healthChecker.GetBackendHealthStatus(backendID)
}

// IsHealthCheckEnabled returns whether health checking is enabled.
func (m *ReverseProxyModule) IsHealthCheckEnabled() bool {
	return m.config.HealthCheck.Enabled
}

// GetOverallHealthStatus returns the overall health status of all backends.
func (m *ReverseProxyModule) GetOverallHealthStatus(includeDetails bool) *OverallHealthStatus {
	if m.healthChecker == nil {
		return &OverallHealthStatus{
			Healthy:       false,
			TotalBackends: 0,
			LastCheck:     time.Now(),
		}
	}
	return m.healthChecker.GetOverallHealthStatus(includeDetails)
}

// evaluateFeatureFlag evaluates a feature flag for the given request context.
// Returns true if the feature flag is enabled or if no evaluator is available.
func (m *ReverseProxyModule) evaluateFeatureFlag(flagID string, req *http.Request) bool {
	if m.featureFlagEvaluator == nil || flagID == "" {
		return true // No evaluator or flag ID means always enabled
	}

	// Extract tenant ID from request
	var tenantID modular.TenantID
	if m.config != nil && m.config.TenantIDHeader != "" {
		tenantIDStr, _ := TenantIDFromRequest(m.config.TenantIDHeader, req)
		tenantID = modular.TenantID(tenantIDStr)
	}

	// Evaluate the feature flag with default true (enabled by default)
	return m.featureFlagEvaluator.EvaluateFlagWithDefault(req.Context(), flagID, tenantID, req, true)
}

// getAlternativeBackend returns the appropriate backend when a feature flag is disabled.
// It returns the alternative backend if specified, otherwise returns the default backend.
func (m *ReverseProxyModule) getAlternativeBackend(alternativeBackend string) string {
	if alternativeBackend != "" {
		return alternativeBackend
	}
	// Fall back to the module's default backend if no alternative is specified
	return m.defaultBackend
}

// handleDryRunRequest processes a request with dry run enabled, sending it to both backends
// and returning the response from the appropriate backend based on configuration.
func (m *ReverseProxyModule) handleDryRunRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, routeConfig RouteConfig, primaryBackend, secondaryBackend string) {
	// Emit request received event for dry run
	m.emitEvent(ctx, EventTypeRequestReceived, map[string]interface{}{
		"method":        r.Method,
		"path":          r.URL.Path,
		"backend":       primaryBackend,
		"dryRunBackend": secondaryBackend,
		"dryRun":        true,
		"remote_addr":   r.RemoteAddr,
	})

	if m.dryRunHandler == nil {
		// Dry run not initialized, fall back to regular handling
		m.app.Logger().Warn("Dry run requested but handler not initialized, falling back to regular handling")

		// Emit request failed event for dry run handler not available
		m.emitEvent(ctx, EventTypeRequestFailed, map[string]interface{}{
			"method":  r.Method,
			"path":    r.URL.Path,
			"backend": primaryBackend,
			"error":   "dry run handler not initialized",
			"reason":  "handler_not_available",
		})

		if primaryBackend == "composite" {
			// Handle composite route specially
			m.app.Logger().Debug("Dry run fallback for composite route not available, returning 503")
			http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		} else {
			handler := m.createBackendProxyHandler(primaryBackend)
			handler(w, r)
		}
		return
	}

	// Read and preserve the request body before it gets consumed
	var bodyBytes []byte
	var err error
	if r.Body != nil {
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			m.app.Logger().Error("Failed to read request body for dry run", "error", err)
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		r.Body.Close()
	}

	// Create a new request with the preserved body for the return backend
	returnRequest := r.Clone(ctx)
	if len(bodyBytes) > 0 {
		returnRequest.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		returnRequest.ContentLength = int64(len(bodyBytes))
	}

	// Determine which response to return to the client
	var returnBackend string
	if m.config.DryRun.DefaultResponseBackend == "secondary" {
		returnBackend = secondaryBackend
	} else {
		returnBackend = primaryBackend
	}

	// Create a response recorder to capture the return backend's response
	recorder := httptest.NewRecorder()

	// Get the handler for the backend we want to return to the client
	var returnHandler http.HandlerFunc
	m.backendProxiesMutex.RLock()
	_, exists := m.backendProxies[returnBackend]
	m.backendProxiesMutex.RUnlock()
	if exists {
		returnHandler = m.createBackendProxyHandler(returnBackend)
	} else {
		m.app.Logger().Error("Return backend not found", "backend", returnBackend)
		http.Error(w, "Backend not found", http.StatusBadGateway)
		return
	}

	// Send request to the return backend and capture response
	returnHandler(recorder, returnRequest)

	// Emit request processed event for successful dry run processing
	m.emitEvent(ctx, EventTypeRequestProcessed, map[string]interface{}{
		"method":          r.Method,
		"path":            r.URL.Path,
		"backend":         returnBackend,
		"dryRunBackend":   secondaryBackend,
		"statusCode":      recorder.Code,
		"dryRun":          true,
		"returnedBackend": returnBackend,
	})

	// Copy the recorded response to the original response writer
	// Copy headers
	copyResponseHeaders(recorder.Header(), w.Header())
	w.WriteHeader(recorder.Code)
	if _, err := w.Write(recorder.Body.Bytes()); err != nil {
		m.app.Logger().Error("Failed to write response body", "error", err)
	}

	// Now perform dry run comparison in the background (async)
	go func(requestCtx context.Context) {
		// Add panic recovery for background goroutine
		defer func() {
			if r := recover(); r != nil {
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Error("Background dry run goroutine panicked", "panic", r)
				}
			}
		}()

		// Use the passed context for background processing
		// Create a copy of the request for background comparison with preserved body
		reqCopy := r.Clone(requestCtx)
		if len(bodyBytes) > 0 {
			reqCopy.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			reqCopy.ContentLength = int64(len(bodyBytes))
		}

		// Get the actual backend URLs
		primaryURL, exists := m.config.BackendServices[primaryBackend]
		if !exists {
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Error("Primary backend URL not found for dry run", "backend", primaryBackend)
			}
			return
		}

		secondaryURL, exists := m.config.BackendServices[secondaryBackend]
		if !exists {
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Error("Secondary backend URL not found for dry run", "backend", secondaryBackend)
			}
			return
		}

		// Capture endpoint path before processing to avoid accessing potentially invalid request
		endpointPath := reqCopy.URL.Path

		// Process dry run comparison with actual URLs using the background context
		result, err := m.dryRunHandler.ProcessDryRun(requestCtx, reqCopy, primaryURL, secondaryURL)
		if err != nil {
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Error("Background dry run processing failed", "error", err)
			}
			return
		}

		// Add nil checks before accessing result fields
		if result != nil && !isEmptyComparisonResult(result.Comparison) {
			// Emit dry run comparison event
			m.emitEvent(requestCtx, EventTypeDryRunComparison, map[string]interface{}{
				"endpoint":         endpointPath,
				"primaryBackend":   primaryBackend,
				"secondaryBackend": secondaryBackend,
				"returnedBackend":  returnBackend,
				"statusCodeMatch":  result.Comparison.StatusCodeMatch,
				"bodyMatch":        result.Comparison.BodyMatch,
				"headersMatch":     result.Comparison.HeadersMatch,
				"differences":      len(result.Comparison.Differences),
				"primaryStatus":    result.PrimaryResponse.StatusCode,
				"secondaryStatus":  result.SecondaryResponse.StatusCode,
				"timestamp":        result.Timestamp,
			})

			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Debug("Dry run comparison completed",
					"endpoint", endpointPath,
					"primaryBackend", primaryBackend,
					"secondaryBackend", secondaryBackend,
					"returnedBackend", returnBackend,
					"statusCodeMatch", result.Comparison.StatusCodeMatch,
					"bodyMatch", result.Comparison.BodyMatch,
					"differences", len(result.Comparison.Differences),
				)
			}
		} else {
			if m.app != nil && m.app.Logger() != nil {
				if result == nil {
					m.app.Logger().Error("Dry run result is nil")
				} else {
					m.app.Logger().Error("Dry run result comparison is empty")
				}
			}
		}
	}(ctx)
}

// isEmptyComparisonResult checks if a ComparisonResult is empty or represents no differences.
// isEmptyComparisonResult determines whether a ComparisonResult is considered "empty".
//
// An "empty" ComparisonResult means that either:
//   - No matches were found (all match fields are false) and there are no recorded differences,
//   - Or, the result does not indicate any differences (Differences and HeaderDiffs are empty).
//
// Specifically, this function returns true if:
//   - All of StatusCodeMatch, HeadersMatch, and BodyMatch are false, and both Differences and HeaderDiffs are empty.
//   - There are no differences recorded at all.
//
// It returns false if:
//   - Any differences are present (Differences or HeaderDiffs are non-empty), or
//   - All match fields are true (indicating a successful comparison).
//
// This is used to determine if a dry run comparison yielded any differences or if the result is a default/empty value.
func isEmptyComparisonResult(result ComparisonResult) bool {
	// Check if all boolean fields are false (indicating no matches found)
	if !result.StatusCodeMatch && !result.HeadersMatch && !result.BodyMatch {
		// If no matches and no differences recorded, it's likely an empty/default result
		if len(result.Differences) == 0 && len(result.HeaderDiffs) == 0 {
			return true
		}
	}

	// If there are differences recorded, it's not empty
	if len(result.Differences) > 0 || len(result.HeaderDiffs) > 0 {
		return false
	}

	// If all match fields are true and no differences, it's a successful comparison (not empty)
	if result.StatusCodeMatch && result.HeadersMatch && result.BodyMatch {
		return false
	}

	// Default case: If none of the above conditions matched, we conservatively assume the result is empty.
	// This ensures that only explicit differences or matches are treated as non-empty; ambiguous or default-initialized results are considered empty.
	return true
}

// RegisterObservers implements the ObservableModule interface.
// This allows the reverseproxy module to register as an observer for events it's interested in.
func (m *ReverseProxyModule) RegisterObservers(subject modular.Subject) error {
	m.subject = subject
	return nil
}

// EmitEvent implements the ObservableModule interface.
// This allows the reverseproxy module to emit events that other modules or observers can receive.
func (m *ReverseProxyModule) EmitEvent(ctx context.Context, event cloudevents.Event) error {
	// Lazily bind to application's subject if not already set, so events emitted
	// during Init/early lifecycle still reach observers when using ObservableApplication.
	if m.subject == nil && m.app != nil {
		if subj, ok := any(m.app).(modular.Subject); ok {
			m.subject = subj
		}
	}
	if m.subject == nil {
		return ErrNoSubjectForEventEmission
	}
	if err := m.subject.NotifyObservers(ctx, event); err != nil {
		return fmt.Errorf("failed to notify observers: %w", err)
	}
	return nil
}

// emitEvent is a helper method to create and emit CloudEvents for the reverseproxy module.
// This centralizes the event creation logic and ensures consistent event formatting.
// emitEvent is a helper method to create and emit CloudEvents for the reverseproxy module.
// This centralizes the event creation logic and ensures consistent event formatting.
// If no subject is available for event emission, it silently skips the event emission
func (m *ReverseProxyModule) emitEvent(ctx context.Context, eventType string, data map[string]interface{}) {
	// Lazily bind to application's subject if not already set, so events emitted
	// during Init/early lifecycle still reach observers when using ObservableApplication.
	if m.subject == nil && m.app != nil {
		if subj, ok := any(m.app).(modular.Subject); ok {
			m.subject = subj
		}
	}
	// Skip event emission if no subject is available (non-observable application)
	if m.subject == nil {
		return
	}

	event := modular.NewCloudEvent(eventType, "reverseproxy-service", data, nil)

	// Try to emit through the module's registered subject first
	if emitErr := m.EmitEvent(ctx, event); emitErr != nil {
		// If no subject is registered, quietly skip to allow non-observable apps to run cleanly
		if errors.Is(emitErr, ErrNoSubjectForEventEmission) {
			return
		}
		// If module subject isn't available, try to emit directly through app if it's a Subject
		if m.app != nil {
			if subj, ok := any(m.app).(modular.Subject); ok {
				// Error occurred during app notification, but we don't log it to avoid
				// noisy test output. Error handling is centralized in EmitEvent.
				// The error is intentionally ignored here as emission is best-effort.
				_ = subj.NotifyObservers(ctx, event)
				return // Successfully emitted via app, no need to log error
			}
		}
		// Note: No logger field available in module, skipping additional error logging
		// to eliminate noisy test output. Error handling is centralized in EmitEvent.
	}
}

// GetRegisteredEventTypes implements the ObservableModule interface.
// Returns all event types that this reverseproxy module can emit.
func (m *ReverseProxyModule) GetRegisteredEventTypes() []string {
	return []string{
		EventTypeConfigLoaded,
		EventTypeConfigValidated,
		EventTypeProxyCreated,
		EventTypeProxyStarted,
		EventTypeProxyStopped,
		EventTypeRequestReceived,
		EventTypeRequestProxied,
		EventTypeRequestFailed,
		EventTypeRequestProcessed,
		EventTypeDryRunComparison,
		EventTypeBackendHealthy,
		EventTypeBackendUnhealthy,
		EventTypeBackendAdded,
		EventTypeBackendRemoved,
		EventTypeLoadBalanceDecision,
		EventTypeLoadBalanceRoundRobin,
		EventTypeCircuitBreakerOpen,
		EventTypeCircuitBreakerClosed,
		EventTypeCircuitBreakerHalfOpen,
		EventTypeModuleStarted,
		EventTypeModuleStopped,
		EventTypeError,
	}
}
