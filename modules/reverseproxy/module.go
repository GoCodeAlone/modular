// Package reverseproxy provides a flexible reverse proxy module with support for multiple backends,
// composite responses, and tenant awareness.
package reverseproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/CrisisTextLine/modular"
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
	app             modular.TenantApplication
	responseCache   *responseCache
	circuitBreakers map[string]*CircuitBreaker
	directorFactory func(backend string, tenant modular.TenantID) func(*http.Request)

	tenants              map[modular.TenantID]*ReverseProxyConfig
	tenantBackendProxies map[modular.TenantID]map[string]*httputil.ReverseProxy
	preProxyTransforms   map[string]func(*http.Request)

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
}

// Compile-time assertions to ensure interface compliance
var _ modular.Module = (*ReverseProxyModule)(nil)
var _ modular.Constructable = (*ReverseProxyModule)(nil)
var _ modular.ServiceAware = (*ReverseProxyModule)(nil)
var _ modular.TenantAwareModule = (*ReverseProxyModule)(nil)
var _ modular.Startable = (*ReverseProxyModule)(nil)
var _ modular.Stoppable = (*ReverseProxyModule)(nil)

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
		httpClient:           nil,
		backendProxies:       make(map[string]*httputil.ReverseProxy),
		backendRoutes:        make(map[string]map[string]http.HandlerFunc),
		compositeRoutes:      make(map[string]http.HandlerFunc),
		tenants:              make(map[modular.TenantID]*ReverseProxyConfig),
		tenantBackendProxies: make(map[modular.TenantID]map[string]*httputil.ReverseProxy),
		preProxyTransforms:   make(map[string]func(*http.Request)),
		circuitBreakers:      make(map[string]*CircuitBreaker),
		enableMetrics:        true,
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
	m.app = app.(modular.TenantApplication)
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

		// Configure the HTTP client with the transport and reasonable timeouts
		m.httpClient = &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second, // Overall request timeout
		}

		app.Logger().Info("Using default HTTP client (no httpclient service available)")
	} else {
		app.Logger().Info("Using HTTP client from httpclient service")
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
		if _, ok := m.backendRoutes[backendID]; !ok {
			m.backendRoutes[backendID] = make(map[string]http.HandlerFunc)
		}
	}

	// Create tenant-specific backend proxies
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

			// Create a new proxy for this tenant's backend
			backendURL, err := url.Parse(serviceURL)
			if err != nil {
				app.Logger().Error("Failed to parse tenant backend URL",
					"tenant", tenantID, "backend", backendID, "url", serviceURL, "error", err)
				continue
			}

			proxy := m.createReverseProxyForBackend(backendURL, backendID, "")

			// Ensure tenant map exists for this backend
			if _, exists := m.tenantBackendProxies[tenantID]; !exists {
				m.tenantBackendProxies[tenantID] = make(map[string]*httputil.ReverseProxy)
			}

			// Store the tenant-specific proxy
			m.tenantBackendProxies[tenantID][backendID] = proxy

			// If there's no global URL for this backend, create one in the global map
			if _, exists := m.backendProxies[backendID]; !exists {
				app.Logger().Info("Using tenant-specific backend URL as global",
					"tenant", tenantID, "backend", backendID, "url", serviceURL)
				m.backendProxies[backendID] = proxy

				// Initialize route map for this backend
				if _, ok := m.backendRoutes[backendID]; !ok {
					m.backendRoutes[backendID] = make(map[string]http.HandlerFunc)
				}
			}

			app.Logger().Debug("Created tenant-specific proxy",
				"tenant", tenantID, "backend", backendID, "url", serviceURL)
		}
	}

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
		app.Logger().Info("Dry run handler initialized")
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

			// Create circuit breaker for this backend
			cb := NewCircuitBreakerWithConfig(backendID, cbConfig, m.metrics)
			m.circuitBreakers[backendID] = cb

			app.Logger().Debug("Initialized circuit breaker", "backend", backendID,
				"failure_threshold", cbConfig.FailureThreshold, "open_timeout", cbConfig.OpenTimeout)
		}
		app.Logger().Info("Circuit breakers initialized", "backends", len(m.circuitBreakers))
	}

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
			m.app.Logger().Info("Using default request timeout", "timeout", m.config.RequestTimeout)
		}
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
		m.router = handleFuncSvc

		// Get the optional httpclient service
		if httpClientInstance, exists := services["httpclient"]; exists {
			if client, ok := httpClientInstance.(*http.Client); ok {
				m.httpClient = client
				app.Logger().Info("Using HTTP client from httpclient service")
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
				app.Logger().Info("Using feature flag evaluator from service")
			} else {
				app.Logger().Warn("featureFlagEvaluator service found but does not implement FeatureFlagEvaluator",
					"type", fmt.Sprintf("%T", featureFlagSvc))
			}
		}

		// If no HTTP client service was found, we'll create a default one in Init()
		if m.httpClient == nil {
			app.Logger().Info("No httpclient service available, will create default client")
		}

		return m, nil
	}
}

// Start sets up all routes for the module and registers them with the router.
// This includes backend routes, composite routes, and any custom endpoints.
func (m *ReverseProxyModule) Start(ctx context.Context) error {
	// Load tenant-specific configurations
	m.loadTenantConfigs()

	// Setup routes for all backends
	if err := m.setupBackendRoutes(); err != nil {
		return err
	}

	// Setup composite routes
	if err := m.setupCompositeRoutes(); err != nil {
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

	// Create and configure feature flag evaluator if none was provided via service
	if m.featureFlagEvaluator == nil && m.config.FeatureFlags.Enabled {
		// Convert the logger to *slog.Logger
		var logger *slog.Logger
		if slogLogger, ok := m.app.Logger().(*slog.Logger); ok {
			logger = slogLogger
		} else {
			// Fallback to a default logger if conversion fails
			logger = slog.Default()
		}

		//nolint:contextcheck // Constructor doesn't need context, it creates the evaluator for later use
		evaluator, err := NewFileBasedFeatureFlagEvaluator(m.app, logger)
		if err != nil {
			return fmt.Errorf("failed to create feature flag evaluator: %w", err)
		}
		m.featureFlagEvaluator = evaluator

		m.app.Logger().Info("Created built-in feature flag evaluator using tenant-aware configuration")
	}

	// Start health checker if enabled
	if m.healthChecker != nil {
		if err := m.healthChecker.Start(ctx); err != nil {
			return fmt.Errorf("failed to start health checker: %w", err)
		}
	}

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
	m.backendProxies = make(map[string]*httputil.ReverseProxy)
	for tenantId := range m.tenantBackendProxies {
		m.tenantBackendProxies[tenantId] = make(map[string]*httputil.ReverseProxy)
	}

	// Keep tenant configs but clear proxies
	for tenantID := range m.tenants {
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Debug("Cleaned up resources for tenant", "tenant", tenantID)
		}
	}

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

	m.app.Logger().Debug("Tenant registered with reverseproxy module", "tenantID", tenantID)
}

// loadTenantConfigs loads all tenant-specific configurations.
// This should be called during Start() or another safe phase after tenant registration.
func (m *ReverseProxyModule) loadTenantConfigs() {
	if m.app != nil && m.app.Logger() != nil {
		m.app.Logger().Debug("Loading tenant configs", "count", len(m.tenants))
	}
	for tenantID := range m.tenants {
		cp, err := m.app.GetTenantConfig(tenantID, m.Name())
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

// OnTenantRemoved is called when a tenant is removed from the application.
// It removes the tenant's configuration and any associated resources.
func (m *ReverseProxyModule) OnTenantRemoved(tenantID modular.TenantID) {
	// Clean up tenant-specific resources
	delete(m.tenants, tenantID)
	m.app.Logger().Info("Tenant removed from reverseproxy module", "tenantID", tenantID)
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
		m.router.HandleFunc(route, handler)
	}
}

// setupCompositeRoutes sets up routes that combine responses from multiple backends.
// For each composite route in the configuration, it creates a handler that fetches
// and combines responses from multiple backends.
func (m *ReverseProxyModule) setupCompositeRoutes() error {
	// Create a map of handlers for each composite route, keyed by tenant ID
	// An empty tenant ID represents the global/default handler
	type HandlerMap map[modular.TenantID]http.HandlerFunc
	compositeHandlers := make(map[string]HandlerMap)

	// First, set up global composite handlers from the global config
	for routePath, routeConfig := range m.config.CompositeRoutes {
		// Create the handler - use feature flag aware version if needed
		var handlerFunc http.HandlerFunc
		if routeConfig.FeatureFlagID != "" {
			// Use feature flag aware handler
			ffHandlerFunc, err := m.createFeatureFlagAwareCompositeHandlerFunc(routeConfig, nil)
			if err != nil {
				m.app.Logger().Error("Failed to create feature flag aware composite handler",
					"route", routePath, "error", err)
				continue
			}
			handlerFunc = ffHandlerFunc
		} else {
			// Use standard composite handler
			handler, err := m.createCompositeHandler(routeConfig, nil)
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
				ffHandlerFunc, err := m.createFeatureFlagAwareCompositeHandlerFunc(routeConfig, tenantConfig)
				if err != nil {
					m.app.Logger().Error("Failed to create feature flag aware tenant composite handler",
						"tenant", tenantID, "route", routePath, "error", err)
					continue
				}
				handlerFunc = ffHandlerFunc
			} else {
				// Use standard composite handler
				handler, err := m.createCompositeHandler(routeConfig, tenantConfig)
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
	// Ensure we have a router
	if m.router == nil {
		return ErrCannotRegisterRoutes
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
		defaultProxy, exists := m.backendProxies[backendID]
		if !exists || defaultProxy == nil {
			m.app.Logger().Warn("Backend not found for route", "route", routePath, "backend", backendID)
			continue
		}

		// Create a handler that considers route configs for feature flag evaluation
		handler := func(routePath, backendID string) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
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

				// Use primary backend (feature flag enabled or no feature flag)
				primaryHandler := m.createBackendProxyHandler(backendID)
				primaryHandler(w, r)
			}
		}(routePath, backendID)

		m.router.HandleFunc(routePath, handler)
		registeredPaths[routePath] = true

		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Info("Registered route", "route", routePath, "backend", backendID)
		}
	}

	// Register all composite routes
	for pattern, handler := range m.compositeRoutes {
		m.router.HandleFunc(pattern, handler)
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Info("Registered composite route", "route", pattern)
		}
	}

	// Register default backend as catch-all if specified and not already registered
	if m.defaultBackend != "" && !registeredPaths["/*"] {
		// Check if the default backend exists in the global proxy map
		defaultProxy, exists := m.backendProxies[m.defaultBackend]
		if !exists || defaultProxy == nil {
			return nil
		}

		// Create a selective catch-all route handler that excludes health/metrics endpoints
		handler := func(w http.ResponseWriter, r *http.Request) {
			// Check if this is a health or metrics path that should not be proxied
			if m.shouldExcludeFromProxy(r.URL.Path) {
				// Let other handlers handle this (health/metrics endpoints)
				http.NotFound(w, r)
				return
			}

			// Use the default backend proxy handler
			backendHandler := m.createBackendProxyHandler(m.defaultBackend)
			backendHandler(w, r)
		}

		// Register the selective catch-all default route
		m.router.HandleFunc("/*", handler)
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Info("Registered selective catch-all route for default backend", "backend", m.defaultBackend)
		}
	}

	return nil
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
		m.router.HandleFunc(path, handler)

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
		m.router.HandleFunc("/*", catchAllHandler)

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

// createReverseProxyForBackend creates a reverse proxy for a specific backend with per-backend configuration.
func (m *ReverseProxyModule) createReverseProxyForBackend(target *url.URL, backendID string, endpoint string) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Use the module's custom transport if available
	if m.httpClient != nil && m.httpClient.Transport != nil {
		proxy.Transport = m.httpClient.Transport
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

	return proxy
}

// createBackendProxy creates a reverse proxy for the specified backend ID and service URL.
// It parses the URL, creates the proxy, and stores it in the backendProxies map.
func (m *ReverseProxyModule) createBackendProxy(backendID, serviceURL string) error {
	// Check if we have backend-specific configuration
	var backendURL *url.URL
	var err error

	if m.config.BackendConfigs != nil {
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

	// Set up proxy for this backend
	proxy := m.createReverseProxyForBackend(backendURL, backendID, "")

	// Store the proxy for this backend
	m.backendProxies[backendID] = proxy

	return nil
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

// createBackendProxyHandler creates an http.HandlerFunc that handles proxying requests
// to a specific backend, with support for tenant-specific backends and feature flag evaluation
func (m *ReverseProxyModule) createBackendProxyHandler(backend string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract tenant ID from request header, if present
		tenantHeader := m.config.TenantIDHeader
		tenantID := modular.TenantID(r.Header.Get(tenantHeader))

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
		if m.config.CircuitBreakerConfig.Enabled {
			// Check for backend-specific circuit breaker
			var cbConfig CircuitBreakerConfig
			if backendCB, exists := m.config.BackendCircuitBreakers[finalBackend]; exists {
				cbConfig = backendCB
			} else {
				cbConfig = m.config.CircuitBreakerConfig
			}

			// Get or create circuit breaker for this backend
			if existingCB, exists := m.circuitBreakers[finalBackend]; exists {
				cb = existingCB
			} else {
				// Create new circuit breaker with config and store for reuse
				cb = NewCircuitBreakerWithConfig(finalBackend, cbConfig, m.metrics)
				m.circuitBreakers[finalBackend] = cb
			}
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
				proxyCopy.ServeHTTP(recorder, req)

				// Convert recorder to response
				return recorder.Result(), nil
			})

			if errors.Is(err, ErrCircuitOpen) {
				// Circuit is open, return service unavailable
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Warn("Circuit breaker open, denying request",
						"backend", finalBackend, "tenant", tenantID, "path", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				if _, err := w.Write([]byte(`{"error":"Service temporarily unavailable","code":"CIRCUIT_OPEN"}`)); err != nil {
					if m.app != nil && m.app.Logger() != nil {
						m.app.Logger().Error("Failed to write circuit breaker response", "error", err)
					}
				}
				return
			} else if err != nil {
				// Some other error occurred
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// Copy response to the original ResponseWriter
			for k, vals := range resp.Header {
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(resp.StatusCode)
			if resp.Body != nil {
				defer resp.Body.Close()
				_, err := io.Copy(w, resp.Body)
				if err != nil {
					// Log error but continue processing
					m.app.Logger().Error("Failed to copy response body", "error", err)
				}
			}
		} else {
			// No circuit breaker, use the proxy directly
			proxy.ServeHTTP(w, r)
		}
	}
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
			// Create new circuit breaker with config and store for reuse
			cb = NewCircuitBreakerWithConfig(backend, cbConfig, m.metrics)
			m.circuitBreakers[backend] = cb
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
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
				proxyCopy.ServeHTTP(recorder, req)

				// Convert recorder to response
				return recorder.Result(), nil
			})

			if errors.Is(err, ErrCircuitOpen) {
				// Circuit is open, return service unavailable
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Warn("Circuit breaker open, denying request",
						"backend", backend, "tenant", tenantID, "path", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				if _, err := w.Write([]byte(`{"error":"Service temporarily unavailable","code":"CIRCUIT_OPEN"}`)); err != nil {
					if m.app != nil && m.app.Logger() != nil {
						m.app.Logger().Error("Failed to write circuit breaker response", "error", err)
					}
				}
				return
			} else if err != nil {
				// Some other error occurred
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// Copy response to the original ResponseWriter
			for k, vals := range resp.Header {
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(resp.StatusCode)
			if resp.Body != nil {
				defer resp.Body.Close()
				_, err := io.Copy(w, resp.Body)
				if err != nil {
					// Log error but continue processing
					m.app.Logger().Error("Failed to copy response body", "error", err)
				}
			}
		} else {
			// No circuit breaker, use the proxy directly
			proxy.ServeHTTP(w, r)
		}
	}
}

// getProxyForBackendAndTenant returns the appropriate proxy for a backend and tenant.
// If a tenant-specific proxy exists, it will be returned; otherwise, the default proxy.
func (m *ReverseProxyModule) getProxyForBackendAndTenant(backendID string, tenantID modular.TenantID) (*httputil.ReverseProxy, bool) {
	// First check for a tenant-specific proxy if tenantID is provided
	if tenantID != "" {
		if tenantProxies, exists := m.tenantBackendProxies[tenantID]; exists {
			if proxy, exists := tenantProxies[backendID]; exists && proxy != nil {
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Debug("Using tenant-specific proxy", "tenant", tenantID, "backend", backendID)
				}
				return proxy, true
			}
		}
	}

	// Fall back to the default proxy
	proxy, exists := m.backendProxies[backendID]
	if m.app != nil && m.app.Logger() != nil {
		m.app.Logger().Debug("Using global proxy", "backend", backendID, "exists", exists)
	}
	return proxy, exists
}

// AddBackendRoute registers a new route for a specific backend.
// It allows dynamically adding routes to the reverse proxy after initialization.
func (m *ReverseProxyModule) AddBackendRoute(backendID, routePattern string) error {
	// Check if backend exists
	proxy, ok := m.backendProxies[backendID]
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
		m.router.HandleFunc(routePattern, handler)
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
			resp, err := m.httpClient.Do(req) //nolint:bodyclose // Response body is closed in defer cleanup
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
		for key, values := range result.Headers {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Ensure Content-Type is set if not specified by transformer
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}

		// Write status code and body
		w.WriteHeader(result.StatusCode)
		if _, err := w.Write(result.Body); err != nil {
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

	// Set default backend - prefer tenant's if specified
	if tenant.DefaultBackend != "" {
		merged.DefaultBackend = tenant.DefaultBackend
	} else {
		merged.DefaultBackend = global.DefaultBackend
	}

	// Merge routes - tenant routes override global routes
	for pattern, backend := range global.Routes {
		merged.Routes[pattern] = backend
	}
	if tenant.Routes != nil {
		for pattern, backend := range tenant.Routes {
			merged.Routes[pattern] = backend
		}
	}

	// Merge route configs - tenant route configs override global route configs
	for pattern, routeConfig := range global.RouteConfigs {
		merged.RouteConfigs[pattern] = routeConfig
	}
	if tenant.RouteConfigs != nil {
		for pattern, routeConfig := range tenant.RouteConfigs {
			merged.RouteConfigs[pattern] = routeConfig
		}
	}

	// Merge composite routes - tenant routes override global routes
	for pattern, route := range global.CompositeRoutes {
		merged.CompositeRoutes[pattern] = route
	}
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
		m.router.HandleFunc(endpoint, metricsHandler)
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

		m.router.HandleFunc(healthEndpoint, healthHandler)
		m.app.Logger().Info("Registered health check endpoint", "endpoint", healthEndpoint)
	}
}

// registerDebugEndpoints registers debug endpoints if they are enabled
func (m *ReverseProxyModule) registerDebugEndpoints() error {
	if m.router == nil {
		return ErrCannotRegisterRoutes
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
	m.router.HandleFunc(flagsEndpoint, debugHandler.HandleFlags)
	m.app.Logger().Info("Registered debug endpoint", "endpoint", flagsEndpoint)

	// General debug info endpoint
	infoEndpoint := basePath + "/info"
	m.router.HandleFunc(infoEndpoint, debugHandler.HandleInfo)
	m.app.Logger().Info("Registered debug endpoint", "endpoint", infoEndpoint)

	// Backend status endpoint
	backendsEndpoint := basePath + "/backends"
	m.router.HandleFunc(backendsEndpoint, debugHandler.HandleBackends)
	m.app.Logger().Info("Registered debug endpoint", "endpoint", backendsEndpoint)

	// Circuit breaker status endpoint
	circuitBreakersEndpoint := basePath + "/circuit-breakers"
	m.router.HandleFunc(circuitBreakersEndpoint, debugHandler.HandleCircuitBreakers)
	m.app.Logger().Info("Registered debug endpoint", "endpoint", circuitBreakersEndpoint)

	// Health check status endpoint
	healthChecksEndpoint := basePath + "/health-checks"
	m.router.HandleFunc(healthChecksEndpoint, debugHandler.HandleHealthChecks)
	m.app.Logger().Info("Registered debug endpoint", "endpoint", healthChecksEndpoint)

	m.app.Logger().Info("Debug endpoints registered", "basePath", basePath)
	return nil
}

// createTenantAwareHandler creates a handler that routes based on tenant-specific configuration for a specific path
func (m *ReverseProxyModule) createTenantAwareHandler(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract tenant ID from request
		tenantIDStr, hasTenant := TenantIDFromRequest(m.config.TenantIDHeader, r)

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
									handler := m.createBackendProxyHandlerForTenant(modular.TenantID(tenantIDStr), alternativeBackend)
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
						handler := m.createBackendProxyHandlerForTenant(modular.TenantID(tenantIDStr), primaryBackend)
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
						handler := m.createBackendProxyHandlerForTenant(tenantID, backendID)
						handler(w, r)
						return
					}
				}

				// No tenant-specific route, check if tenant has default backend
				if tenantCfg.DefaultBackend != "" {
					handler := m.createBackendProxyHandlerForTenant(tenantID, tenantCfg.DefaultBackend)
					handler(w, r)
					return
				}
			}
		}

		// Fall back to global configuration
		// Check if there's a global route for this path
		if backendID, ok := m.config.Routes[path]; ok {
			if _, exists := m.backendProxies[backendID]; exists {
				handler := m.createBackendProxyHandler(backendID)
				handler(w, r)
				return
			}
		}

		// Check if there's a composite route
		if compositeHandler, ok := m.compositeRoutes[path]; ok {
			compositeHandler.ServeHTTP(w, r)
			return
		}

		// Fall back to global default backend
		if m.defaultBackend != "" {
			if _, exists := m.backendProxies[m.defaultBackend]; exists {
				handler := m.createBackendProxyHandler(m.defaultBackend)
				handler(w, r)
				return
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

		if hasTenant {
			tenantID := modular.TenantID(tenantIDStr)
			if m.app != nil && m.app.Logger() != nil {
				m.app.Logger().Debug("Processing tenant request", "tenant", tenantID, "path", r.URL.Path)
			}

			// Check if we have a tenant-specific configuration
			if tenantCfg, exists := m.tenants[tenantID]; exists && tenantCfg != nil {
				// Check if tenant has a default backend (use it regardless of global default)
				if tenantCfg.DefaultBackend != "" {
					if m.app != nil && m.app.Logger() != nil {
						m.app.Logger().Debug("Using tenant default backend", "tenant", tenantID, "backend", tenantCfg.DefaultBackend)
					}
					handler := m.createBackendProxyHandlerForTenant(tenantID, tenantCfg.DefaultBackend)
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
			if _, exists := m.backendProxies[m.defaultBackend]; exists {
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
	if m.dryRunHandler == nil {
		// Dry run not initialized, fall back to regular handling
		m.app.Logger().Warn("Dry run requested but handler not initialized, falling back to regular handling")
		handler := m.createBackendProxyHandler(primaryBackend)
		handler(w, r)
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
	if _, exists := m.backendProxies[returnBackend]; exists {
		returnHandler = m.createBackendProxyHandler(returnBackend)
	} else {
		m.app.Logger().Error("Return backend not found", "backend", returnBackend)
		http.Error(w, "Backend not found", http.StatusBadGateway)
		return
	}

	// Send request to the return backend and capture response
	returnHandler(recorder, returnRequest)

	// Copy the recorded response to the original response writer
	// Copy headers
	for key, vals := range recorder.Header() {
		for _, v := range vals {
			w.Header().Add(key, v)
		}
	}
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
