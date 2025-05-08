// Package reverseproxy provides a flexible reverse proxy module with support for multiple backends,
// composite responses, and tenant awareness.
package reverseproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/GoCodeAlone/modular"
)

// ReverseProxyModule provides a modular reverse proxy implementation with support for
// multiple backends, composite routes that combine responses from different backends,
// and tenant-specific routing configurations.
type ReverseProxyModule struct {
	config          *ReverseProxyConfig
	router          handleFuncService
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
	tenantBackendProxies map[string]map[modular.TenantID]*httputil.ReverseProxy
	preProxyTransforms   map[string]func(*http.Request)
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
		tenantBackendProxies: make(map[string]map[modular.TenantID]*httputil.ReverseProxy),
		preProxyTransforms:   make(map[string]func(*http.Request)),
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

	// Initialize maps
	m.backendProxies = make(map[string]*httputil.ReverseProxy)
	m.backendRoutes = make(map[string]map[string]http.HandlerFunc)
	m.tenantBackendProxies = make(map[string]map[modular.TenantID]*httputil.ReverseProxy)

	// First, collect all valid backend URLs from both global and tenant configs
	validBackends := make(map[string]string)

	// Start with global backend services, even if empty
	for backendID, serviceURL := range m.config.BackendServices {
		validBackends[backendID] = serviceURL
	}

	// Look for tenant-specific overrides of empty global URLs
	for tenantID, tenantCfg := range m.tenants {
		if tenantCfg == nil || tenantCfg.BackendServices == nil {
			continue
		}

		for backendID, serviceURL := range tenantCfg.BackendServices {
			if serviceURL != "" {
				// If the global URL is empty but tenant URL is valid, use the tenant URL
				if globalURL, exists := validBackends[backendID]; !exists || globalURL == "" {
					app.Logger().Info("Using tenant-specific backend URL",
						"tenant", tenantID, "backend", backendID, "url", serviceURL)
					validBackends[backendID] = serviceURL
				}
			}
		}
	}

	// Now create proxies for all valid backends
	for backendID, serviceURL := range validBackends {
		// Skip truly empty URLs (not overridden by any tenant)
		if serviceURL == "" {
			app.Logger().Warn("Backend has no URL in any config, skipping", "backend", backendID)
			continue
		}

		// Create reverse proxy for backend
		backendURL, err := url.Parse(serviceURL)
		if err != nil {
			return fmt.Errorf("failed to parse %s URL %s: %w", backendID, serviceURL, err)
		}

		// Set up proxy for this backend
		proxy := m.createReverseProxy(backendURL)

		// Store the proxy for this backend
		m.backendProxies[backendID] = proxy

		// Initialize tenant map for this backend
		m.tenantBackendProxies[backendID] = make(map[modular.TenantID]*httputil.ReverseProxy)

		// Initialize route map for this backend
		m.backendRoutes[backendID] = make(map[string]http.HandlerFunc)
	}

	// Now set up tenant-specific backends
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

			// Always create a tenant-specific proxy if service URL is provided,
			// regardless of whether it differs from the global URL
			backendURL, err := url.Parse(serviceURL)
			if err != nil {
				app.Logger().Error("Failed to parse tenant backend URL",
					"tenant", tenantID, "backend", backendID, "url", serviceURL, "error", err)
				continue
			}

			// Create a new proxy for this tenant's backend
			proxy := m.createReverseProxy(backendURL)

			// Ensure tenant map exists for this backend
			if _, exists := m.tenantBackendProxies[backendID]; !exists {
				m.tenantBackendProxies[backendID] = make(map[modular.TenantID]*httputil.ReverseProxy)
			}

			// Store the tenant-specific proxy
			m.tenantBackendProxies[backendID][tenantID] = proxy
			app.Logger().Debug("Created tenant-specific proxy",
				"tenant", tenantID, "backend", backendID, "url", serviceURL)
		}
	}

	// Set default backend for the module
	m.defaultBackend = m.config.DefaultBackend

	return nil
}

// Constructor returns a ModuleConstructor function that initializes the module with
// the required services. It expects a service that implements the handleFuncService
// interface to register routes with.
func (m *ReverseProxyModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		// Get the required router service
		handleFuncSvc, ok := services["router"].(handleFuncService)
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

	// Register routes with router
	m.registerRoutes()

	return nil
}

// Stop performs any cleanup needed when stopping the module.
// Currently, this is a no-op.
func (m *ReverseProxyModule) Stop(context.Context) error {
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
		m.app.Logger().Debug("Loaded and merged tenant config", "tenantID", tenantID)
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

// handleFuncService defines the interface for a service that can register
// HTTP handlers with URL patterns. This is typically implemented by an HTTP router.
type handleFuncService interface {
	HandleFunc(pattern string, handler http.HandlerFunc)
}

// RequiresServices returns the services required by this module.
// The reverseproxy module requires a service that implements the handleFuncService
// interface to register routes with, and optionally a http.Client.
func (m *ReverseProxyModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "router",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*handleFuncService)(nil)).Elem(),
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
		m.registerBackendRoute(backendID, defaultRoute, proxy, m.tenantBackendProxies[backendID])
	}

	return nil
}

// registerBackendRoute registers a route handler for a specific backend.
// It creates a handler function that routes requests to the appropriate backend,
// taking into account tenant-specific configurations.
func (m *ReverseProxyModule) registerBackendRoute(backendID, route string, _ *httputil.ReverseProxy, tenantProxies map[modular.TenantID]*httputil.ReverseProxy) {
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
func (m *ReverseProxyModule) registerTenantAwareRoutes() error {
	// Try to get chi.Router from the router service
	chiRouter, ok := m.router.(chi.Router)
	if !ok {
		// Fall back to basic routing if chi.Router is not available
		m.app.Logger().Warn("Router doesn't implement chi.Router, falling back to basic routing")
		return m.registerBasicRoutes()
	}

	// Get all unique endpoints across all configurations (global and tenant-specific)
	allPaths := make(map[string]bool)

	// Add global routes
	for routePath := range m.config.Routes {
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

	// Add composite routes
	for routePath := range m.compositeRoutes {
		allPaths[routePath] = true
	}

	// Register a handler for each unique path
	for path := range allPaths {
		// Create a handler map for this specific path across all tenants
		tenantHandlers := make(map[modular.TenantID]http.HandlerFunc)

		// Add global handlers if available
		if backendID, ok := m.config.Routes[path]; ok {
			if _, exists := m.backendProxies[backendID]; exists {
				tenantHandlers[""] = m.createBackendProxyHandler(backendID)
			}
		}

		// Add tenant-specific handlers
		for tenantID, tenantCfg := range m.tenants {
			if tenantCfg == nil || tenantCfg.Routes == nil {
				continue
			}

			if backendID, ok := tenantCfg.Routes[path]; ok {
				// Check if the backend exists
				if _, exists := m.backendProxies[backendID]; exists {
					// Check for tenant-specific proxies for this backend
					tenantProxies := make(map[modular.TenantID]*httputil.ReverseProxy)

					// Add tenant-specific proxy if it exists
					if tProxy, exists := m.tenantBackendProxies[backendID][tenantID]; exists {
						tenantProxies[tenantID] = tProxy
					}

					tenantHandlers[tenantID] = m.createBackendProxyHandler(backendID)
				}
			}
		}

		// Add composite route handler if exists
		if compositeHandler, ok := m.compositeRoutes[path]; ok {
			// Add the composite handler as a global handler
			// It already handles tenant-specific routing internally
			tenantHandlers[""] = compositeHandler
		}

		// Skip if no handlers found for this path
		if len(tenantHandlers) == 0 {
			continue
		}

		// Register a single handler that will route based on tenant ID
		chiRouter.HandleFunc(path, func(tenantHandlers map[modular.TenantID]http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				tenantIDStr, hasTenant := TenantIDFromRequest(m.config.TenantIDHeader, r)
				tenantID := modular.TenantID(tenantIDStr)

				// First try tenant-specific handler
				if hasTenant {
					if handler, ok := tenantHandlers[tenantID]; ok {
						handler(w, r)
						return
					}
				}

				// Fall back to global handler
				if handler, ok := tenantHandlers[""]; ok {
					handler(w, r)
					return
				}

				// If no handler found, return 404
				m.app.Logger().Debug("No handler found for path and tenant",
					"path", path,
					"tenant", tenantIDStr,
					"availableTenants", getAvailableTenantIDs(tenantHandlers))
				http.NotFound(w, r)
			}
		}(tenantHandlers))

		m.app.Logger().Info("Registered tenant-aware route", "path", path)
	}

	// Finally, register a default backend catch-all route if configured
	// and not already registered
	if m.defaultBackend != "" && !allPaths["/*"] {
		// Check if the default backend exists
		if m.backendProxies[m.defaultBackend] != nil {
			// Create tenant handlers for the catch-all route
			tenantHandlers := make(map[modular.TenantID]http.HandlerFunc)

			// Add global default handler
			tenantHandlers[""] = m.createBackendProxyHandler(m.defaultBackend)

			// Add tenant-specific default handlers
			for tenantID, tenantCfg := range m.tenants {
				if tenantCfg == nil {
					continue
				}

				// Use tenant's default backend if specified
				backendID := m.defaultBackend
				if tenantCfg.DefaultBackend != "" {
					backendID = tenantCfg.DefaultBackend
				}

				// Check for tenant-specific proxy
				if _, exists := m.tenantBackendProxies[backendID]; exists {
					// Create tenant-specific proxies map
					tenantProxies := make(map[modular.TenantID]*httputil.ReverseProxy)
					// Add tenant-specific proxy if it exists
					if tProxy, exists := m.tenantBackendProxies[backendID][tenantID]; exists {
						tenantProxies[tenantID] = tProxy
					}
					tenantHandlers[tenantID] = m.createBackendProxyHandler(backendID)
				}
			}

			// Register the catch-all handler
			chiRouter.HandleFunc("/*", func(tenantHandlers map[modular.TenantID]http.HandlerFunc) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					tenantIDStr, hasTenant := TenantIDFromRequest(m.config.TenantIDHeader, r)
					tenantID := modular.TenantID(tenantIDStr)

					// First try tenant-specific handler
					if hasTenant {
						if handler, ok := tenantHandlers[tenantID]; ok {
							handler(w, r)
							return
						}
					}

					// Fall back to global handler
					if handler, ok := tenantHandlers[""]; ok {
						handler(w, r)
						return
					}

					// If no handler found, return 404
					http.NotFound(w, r)
				}
			}(tenantHandlers))

			m.app.Logger().Info("Registered default backend catch-all route", "backend", m.defaultBackend)
		}
	}

	return nil
}

// Helper function to get available tenant IDs for debugging
func getAvailableTenantIDs(tenantHandlers map[modular.TenantID]http.HandlerFunc) string {
	tenants := make([]string, 0, len(tenantHandlers))
	for tenantID := range tenantHandlers {
		if tenantID == "" {
			tenants = append(tenants, "global")
		} else {
			tenants = append(tenants, string(tenantID))
		}
	}
	return strings.Join(tenants, ", ")
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
		// Get the default proxy for this backend
		defaultProxy, exists := m.backendProxies[backend]
		if !exists {
			http.Error(w, fmt.Sprintf("Backend %s not found", backend), http.StatusInternalServerError)
			return
		}

		// Check for tenant header
		tenantHeader := m.config.TenantIDHeader
		tenantID := modular.TenantID(r.Header.Get(tenantHeader))

		// If we have a tenant ID and tenant-specific proxies exist for this backend
		if tenantID != "" {
			if tenantProxies, exists := m.tenantBackendProxies[backend]; exists {
				// Check if we have a tenant-specific proxy for this tenant
				if tenantProxy, tenantExists := tenantProxies[tenantID]; tenantExists {
					// Use the tenant-specific proxy
					tenantProxy.ServeHTTP(w, r)
					return
				}
			}
		}

		// Fall back to the default proxy if no tenant-specific proxy was found
		defaultProxy.ServeHTTP(w, r)
	}
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

		// Create a context with timeout for our requests
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
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
			w.Header().Set("Content-Type", "application/json")
		}

		// Write status code and body
		w.WriteHeader(result.StatusCode)
		w.Write(result.Body)

		// Close all response bodies
		for _, resp := range responses {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
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
		CompositeRoutes:        make(map[string]CompositeRoute),
		BackendCircuitBreakers: make(map[string]CircuitBreakerConfig),
	}

	// Copy global backend services
	for id, url := range global.BackendServices {
		merged.BackendServices[id] = url
	}

	// Override with tenant-specific backend services
	if tenant.BackendServices != nil {
		for id, url := range tenant.BackendServices {
			// Only override if the tenant has specified a non-empty URL
			if url != "" {
				merged.BackendServices[id] = url
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
