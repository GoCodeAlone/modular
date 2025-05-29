// Package reverseproxy provides a flexible reverse proxy module with support for multiple backends,
// composite responses, and tenant awareness.
package reverseproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/GoCodeAlone/modular"
)

// ReverseProxyModule provides a modular reverse proxy implementation with support for
// multiple backends, composite routes that combine responses from different backends,
// and tenant-specific routing configurations.
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
}

// NewModule creates a new ReverseProxyModule with default settings.
// It initializes the HTTP client with optimized connection pooling and timeouts,
// and prepares the internal data structures needed for routing.
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
	// Register the config section
	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(&ReverseProxyConfig{}))

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
	m.config = cfg.GetConfig().(*ReverseProxyConfig)

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

			proxy := m.createReverseProxy(backendURL)

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

	return nil
}

// validateConfig validates the module configuration.
// It checks for valid URLs, timeout values, and other configuration parameters.
func (m *ReverseProxyModule) validateConfig() error {
	// If no config, return error
	if m.config == nil {
		return fmt.Errorf("configuration is nil")
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
			return fmt.Errorf("default backend '%s' is not defined in backend_services", m.config.DefaultBackend)
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
		return fmt.Errorf("tenant ID is required but TenantIDHeader is not set")
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
			return nil, fmt.Errorf("service %s does not implement HandleFunc interface", "router")
		}
		m.router = handleFuncSvc

		// Get the optional httpclient service
		if clientService, ok := services["httpclient"].(*http.Client); ok {
			// Use the provided HTTP client
			m.httpClient = clientService
			app.Logger().Info("Using HTTP client from httpclient service")
		}

		return m, nil
	}
}

// Start sets up all routes for the module and registers them with the router.
// This includes backend routes, composite routes, and any custom endpoints.
func (m *ReverseProxyModule) Start(context.Context) error {
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
	m.registerRoutes()

	return nil
}

// Stop performs any cleanup needed when stopping the module.
// This method gracefully shuts down active connections and resources.
func (m *ReverseProxyModule) Stop(ctx context.Context) error {
	// Log that we're shutting down
	if m.app != nil && m.app.Logger() != nil {
		m.app.Logger().Info("Shutting down reverseproxy module")
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
	if _, ok := m.tenants[tenantID]; ok {
		delete(m.tenants, tenantID)
	}
	m.app.Logger().Info("Tenant removed from reverseproxy module", "tenantID", tenantID)
}

// ProvidesServices returns the services provided by this module.
// Currently, this module does not provide any services.
func (m *ReverseProxyModule) ProvidesServices() []modular.ServiceProvider {
	return nil
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
// interface to register routes with, and optionally a http.Client.
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
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*http.Client)(nil)).Elem(),
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
		// Create the global handler
		handler, err := m.createCompositeHandler(routeConfig, nil)
		if err != nil {
			m.app.Logger().Error("Failed to create global composite handler",
				"route", routePath, "error", err)
			continue
		}

		// Initialize the handler map for this route if not exists
		if _, exists := compositeHandlers[routePath]; !exists {
			compositeHandlers[routePath] = make(HandlerMap)
		}

		// Store the global handler with an empty tenant ID key
		compositeHandlers[routePath][""] = handler.ServeHTTP
	}

	// Now set up tenant-specific composite handlers
	for tenantID, tenantConfig := range m.tenants {
		// Skip if tenant config is nil
		if tenantConfig == nil || tenantConfig.CompositeRoutes == nil {
			continue
		}

		for routePath, routeConfig := range tenantConfig.CompositeRoutes {
			// Create the tenant-specific handler
			handler, err := m.createCompositeHandler(routeConfig, tenantConfig)
			if err != nil {
				m.app.Logger().Error("Failed to create tenant composite handler",
					"tenant", tenantID, "route", routePath, "error", err)
				continue
			}

			// Initialize the handler map for this route if not exists
			if _, exists := compositeHandlers[routePath]; !exists {
				compositeHandlers[routePath] = make(HandlerMap)
			}

			// Store the tenant-specific handler
			compositeHandlers[routePath][tenantID] = handler.ServeHTTP
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
		return fmt.Errorf("cannot register routes: router is nil")
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

	// Register explicit routes from configuration
	for routePath, backendID := range m.config.Routes {
		// Check if this backend exists
		defaultProxy, exists := m.backendProxies[backendID]
		if !exists || defaultProxy == nil {
			m.app.Logger().Warn("Backend not found for route", "route", routePath, "backend", backendID)
			continue
		}

		// Create and register the handler
		handler := m.createBackendProxyHandler(backendID)
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

		// Create a catch-all route handler for the default backend
		handler := m.createBackendProxyHandler(m.defaultBackend)

		// Register the catch-all default route
		m.router.HandleFunc("/*", handler)
		if m.app != nil && m.app.Logger() != nil {
			m.app.Logger().Info("Registered default backend", "backend", m.defaultBackend)
		}
	}

	return nil
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
		// Create a tenant-aware catch-all handler
		catchAllHandler := m.createTenantAwareCatchAllHandler()
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

// createReverseProxy is a helper method that creates a new reverse proxy with the module's configured transport.
// This ensures that all proxies use the same transport settings, even if created after SetHttpClient is called.
func (m *ReverseProxyModule) createReverseProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Use the module's custom transport if available
	if m.httpClient != nil && m.httpClient.Transport != nil {
		proxy.Transport = m.httpClient.Transport
	}

	// Store the original target for use in the director function
	originalTarget := *target

	// If a custom director factory is available, use it
	if m.directorFactory != nil {
		// Get the backend ID from the target URL host
		backend := originalTarget.Host

		// Create a custom director that handles the backend routing
		proxy.Director = func(req *http.Request) {
			// Extract tenant ID from the request header if available
			tenantIDStr, hasTenant := TenantIDFromRequest(m.config.TenantIDHeader, req)

			// Create a default director that sets up the request URL
			defaultDirector := func(req *http.Request) {
				req.URL.Scheme = originalTarget.Scheme
				req.URL.Host = originalTarget.Host
				req.URL.Path = singleJoiningSlash(originalTarget.Path, req.URL.Path)
				if originalTarget.RawQuery != "" && req.URL.RawQuery != "" {
					req.URL.RawQuery = originalTarget.RawQuery + "&" + req.URL.RawQuery
				} else if originalTarget.RawQuery != "" {
					req.URL.RawQuery = originalTarget.RawQuery
				}
				// Set host header if not already set
				if _, ok := req.Header["Host"]; !ok {
					req.Host = originalTarget.Host
				}
			}

			// Apply custom director based on tenant ID if available
			if hasTenant {
				tenantID := modular.TenantID(tenantIDStr)
				customDirector := m.directorFactory(backend, tenantID)
				if customDirector != nil {
					customDirector(req)
					return
				}
			}

			// If no tenant-specific director was applied (or if it was nil),
			// try with the default (empty) tenant ID
			emptyTenantDirector := m.directorFactory(backend, "")
			if emptyTenantDirector != nil {
				emptyTenantDirector(req)
				return
			}

			// Fall back to default director if no custom directors worked
			defaultDirector(req)
		}
	}

	return proxy
}

// createBackendProxy creates a reverse proxy for the specified backend ID and service URL.
// It parses the URL, creates the proxy, and stores it in the backendProxies map.
func (m *ReverseProxyModule) createBackendProxy(backendID, serviceURL string) error {
	// Create reverse proxy for this backend
	backendURL, err := url.Parse(serviceURL)
	if err != nil {
		return fmt.Errorf("failed to parse %s URL %s: %w", backendID, serviceURL, err)
	}

	// Set up proxy for this backend
	proxy := m.createReverseProxy(backendURL)

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

// createBackendProxyHandler creates an http.HandlerFunc that handles proxying requests
// to a specific backend, with support for tenant-specific backends
func (m *ReverseProxyModule) createBackendProxyHandler(backend string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract tenant ID from request header, if present
		tenantHeader := m.config.TenantIDHeader
		tenantID := modular.TenantID(r.Header.Get(tenantHeader))

		// Get the appropriate proxy for this backend and tenant
		proxy, exists := m.getProxyForBackendAndTenant(backend, tenantID)
		if !exists {
			http.Error(w, fmt.Sprintf("Backend %s not found", backend), http.StatusInternalServerError)
			return
		}

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
				w.Write([]byte(`{"error":"Service temporarily unavailable","code":"CIRCUIT_OPEN"}`))
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
				io.Copy(w, resp.Body)
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
				w.Write([]byte(`{"error":"Service temporarily unavailable","code":"CIRCUIT_OPEN"}`))
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
				io.Copy(w, resp.Body)
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
		return fmt.Errorf("backend %s not found", backendID)
	}

	// If proxy is nil, log the error and return
	if proxy == nil {
		m.app.Logger().Error("Backend proxy is nil", "backend", backendID)
		return fmt.Errorf("backend proxy for %s is nil", backendID)
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
			if endpoint.QueryParams != nil && len(endpoint.QueryParams) > 0 {
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
			resp, err := m.httpClient.Do(req)
			if err != nil {
				m.app.Logger().Error("Failed to execute request", "backend", endpoint.Backend, "error", err)
				continue
			}

			// Add to the list of responses that need to be closed
			responsesToClose = append(responsesToClose, resp)

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
		w.Write(result.Body)
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
		CompositeRoutes:        make(map[string]CompositeRoute),
		BackendCircuitBreakers: make(map[string]CircuitBreakerConfig),
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

	return merged
}

// getBackendMap returns a map of backend IDs to their URLs from the global configuration.
func (m *ReverseProxyModule) getBackendMap() map[string]string {
	if m.config == nil || m.config.BackendServices == nil {
		return map[string]string{}
	}
	return m.config.BackendServices
}

// getTenantBackendMap returns a map of backend IDs to their URLs for a specific tenant.
func (m *ReverseProxyModule) getTenantBackendMap(tenantID modular.TenantID) map[string]string {
	if m.tenants == nil {
		return map[string]string{}
	}

	tenant, exists := m.tenants[tenantID]
	if !exists || tenant == nil || tenant.BackendServices == nil {
		return map[string]string{}
	}

	return tenant.BackendServices
}

// getBackendURLsByTenant returns all backend URLs for a specific tenant.
func (m *ReverseProxyModule) getBackendURLsByTenant(tenantID modular.TenantID) map[string]string {
	return m.getTenantBackendMap(tenantID)
}

// getBackendByPathAndTenant returns the backend URL for a specific path and tenant.
func (m *ReverseProxyModule) getBackendByPathAndTenant(path string, tenantID modular.TenantID) (string, bool) {
	// Get the tenant-specific backend map
	backendMap := m.getTenantBackendMap(tenantID)

	// Check if there's a direct match for the path
	if url, ok := backendMap[path]; ok {
		return url, true
	}

	// If no direct match, try to find the most specific match
	var bestMatch string
	var bestMatchLength int

	for pattern, url := range backendMap {
		// Check if path starts with the pattern and the pattern is longer than our current best match
		if strings.HasPrefix(path, pattern) && len(pattern) > bestMatchLength {
			bestMatch = url
			bestMatchLength = len(pattern)
		}
	}

	if bestMatchLength > 0 {
		return bestMatch, true
	}

	return "", false
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
		w.Write(jsonData)
	}

	// Register the metrics endpoint with the router
	if m.router != nil {
		m.router.HandleFunc(endpoint, metricsHandler)
		m.app.Logger().Info("Registered metrics endpoint", "endpoint", endpoint)
	}
}

// createRouteHeadersMiddleware creates a middleware for tenant-specific routing
// This creates a middleware that routes based on header values
func (m *ReverseProxyModule) createRouteHeadersMiddleware(tenantID modular.TenantID, routeMap map[string]http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this request has the tenant header
			headerValue := r.Header.Get(m.config.TenantIDHeader)

			// If header matches this tenant, use tenant-specific routing
			if headerValue == string(tenantID) {
				// Find the appropriate handler for this route from the tenant's route map
				for route, handler := range routeMap {
					if route == "/*" || r.URL.Path == route {
						handler.ServeHTTP(w, r)
						return
					}
				}
				// If no specific route found, fall through to next handler
			}

			// Continue with the default handler chain
			next.ServeHTTP(w, r)
		})
	}
}

// createTenantAwareHandler creates a handler that routes based on tenant-specific configuration for a specific path
func (m *ReverseProxyModule) createTenantAwareHandler(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract tenant ID from request
		tenantIDStr, hasTenant := TenantIDFromRequest(m.config.TenantIDHeader, r)

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
