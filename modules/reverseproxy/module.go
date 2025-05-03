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
	"time"

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

	tenants              map[modular.TenantID]*ReverseProxyConfig
	tenantBackendProxies map[string]map[modular.TenantID]*httputil.ReverseProxy
}

// NewModule creates a new ReverseProxyModule with default settings.
// It initializes the HTTP client with optimized connection pooling and timeouts,
// and prepares the internal data structures needed for routing.
func NewModule() (*ReverseProxyModule, error) {
	// Create a customized transport with connection pooling settings
	transport := &http.Transport{
		MaxIdleConns:        100,              // Maximum number of idle connections across all hosts
		MaxIdleConnsPerHost: 10,               // Maximum number of idle connections per host
		IdleConnTimeout:     90 * time.Second, // How long to keep idle connections alive
		TLSHandshakeTimeout: 10 * time.Second, // Maximum time for TLS handshake
		DisableCompression:  false,            // Enable compression by default
	}

	// Configure the HTTP client with the transport and reasonable timeouts
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // Overall request timeout (30s instead of 10s)
	}

	module := &ReverseProxyModule{
		httpClient:           client,
		backendProxies:       make(map[string]*httputil.ReverseProxy),
		backendRoutes:        make(map[string]map[string]http.HandlerFunc),
		compositeRoutes:      make(map[string]http.HandlerFunc),
		tenants:              make(map[modular.TenantID]*ReverseProxyConfig),
		tenantBackendProxies: make(map[string]map[modular.TenantID]*httputil.ReverseProxy),
	}

	return module, nil
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

	// Initialize route maps for each backend
	for backendID, serviceURL := range m.config.BackendServices {
		m.backendRoutes[backendID] = make(map[string]http.HandlerFunc)

		// Skip if URL is not provided
		if serviceURL == "" {
			continue
		}
		// Create reverse proxy for backend
		backendURL, err := url.Parse(serviceURL)
		if err != nil {
			return fmt.Errorf("failed to parse %s URL %s: %w", backendID, serviceURL, err)
		}
		// Set up default route handler for this backend
		proxy := m.createReverseProxy(backendURL)

		// Store the default proxy for this backend
		m.backendProxies[backendID] = proxy

		// build tenant map of backend routes
		tenantProxies := make(map[modular.TenantID]*httputil.ReverseProxy)
		for tenantID, tenantCfg := range m.tenants {
			if tenantCfg == nil {
				continue
			}
			if tenantCfg.BackendServices[backendID] == "" {
				continue
			}

			if tenantCfg.BackendServices[backendID] == serviceURL {
				// Use the same proxy for the tenant as the default backend
				tenantProxies[tenantID] = proxy
				continue
			}

			// Create reverse proxy for tenant backend
			tenantBackendURL, err := url.Parse(tenantCfg.BackendServices[backendID])
			if err != nil {
				return fmt.Errorf("failed to parse %s URL %s: %w", backendID, serviceURL, err)
			}

			tenantProxies[tenantID] = m.createReverseProxy(tenantBackendURL)
		}

		// Store the tenant-specific proxies for this backend
		m.tenantBackendProxies[backendID] = tenantProxies
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
		handleFuncSvc, ok := services["router"].(handleFuncService)
		if !ok {
			return nil, fmt.Errorf("service %s does not implement HandleFunc interface", "router")
		}

		m.router = handleFuncSvc

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

		cfg, ok := cp.GetConfig().(*ReverseProxyConfig)
		if !ok {
			m.app.Logger().Error("Failed to cast config for tenant", "tenant", tenantID, "module", m.Name())
			continue
		}

		m.tenants[tenantID] = cfg
		m.app.Logger().Debug("Loaded tenant config", "tenantID", tenantID)
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
// interface to register routes with.
func (m *ReverseProxyModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{Name: "router", Required: true, MatchByInterface: true, SatisfiesInterface: reflect.TypeOf((*handleFuncService)(nil)).Elem()},
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
func (m *ReverseProxyModule) registerBackendRoute(backendID, route string, defaultProxy *httputil.ReverseProxy, tenantProxies map[modular.TenantID]*httputil.ReverseProxy) {
	// Create the handler function
	handler := func(w http.ResponseWriter, r *http.Request) {
		proxy := defaultProxy
		r2 := r.Clone(r.Context())
		r2.URL.Path = path.Join("/", r.URL.Path)
		r2.URL.RawPath = path.Join("/", r.URL.RawPath)

		// Add any backend-specific headers or modifications here
		tenantID, ok := TenantIDFromRequest(m.config.TenantIDHeader, r)
		if ok {
			// Check if we have a tenant-specific proxy
			if tenantProxy, ok := tenantProxies[modular.TenantID(tenantID)]; ok {
				// Use the tenant-specific proxy
				proxy = tenantProxy
			}
		} else {
			// Check if tenant ID is required
			if m.config.RequireTenantID {
				http.Error(w, fmt.Sprintf("Header %s is required", m.config.TenantIDHeader), http.StatusBadRequest)
				return
			}
		}

		proxy.ServeHTTP(w, r2)
	}

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
	// Set up composite handlers for routes that need to combine responses from multiple backends
	for routePath, routeConfig := range m.config.CompositeRoutes {
		// Get the backends for this composite route
		backendIDs := routeConfig.Backends

		// Skip if no backends are configured
		if len(backendIDs) == 0 {
			continue
		}

		// Create Backend structs from backend IDs
		var backends []*Backend
		for _, backendID := range backendIDs {
			backendURL := m.config.BackendServices[backendID]
			if backendURL == "" {
				m.app.Logger().Warn("Backend URL not configured", "backendID", backendID)
				continue
			}

			backends = append(backends, &Backend{
				ID:     backendID,
				URL:    backendURL,
				Client: m.httpClient,
			})
		}

		// Create composite handler with the new signature
		// We set parallel to true and use a reasonable timeout
		handler := NewCompositeHandler(
			backends,
			true,           // parallel execution
			10*time.Second, // response timeout
		)

		// Configure circuit breakers for each backend
		handler.ConfigureCircuitBreakers(
			m.config.CircuitBreakerConfig,
			m.config.BackendCircuitBreakers,
		)

		// Setup response cache if needed
		if m.config.CacheEnabled {
			// Create response cache if it doesn't exist yet
			if m.responseCache == nil {
				ttl := time.Duration(m.config.CacheTTL) * time.Second
				if ttl == 0 {
					ttl = 60 * time.Second // Default TTL of 60 seconds
				}
				// Create cache with reasonable defaults
				m.responseCache = newResponseCache(ttl, 1000, 5*time.Minute)
			}
			handler.SetResponseCache(m.responseCache)
		}

		// Register composite handler for this route
		m.compositeRoutes[routePath] = handler.ServeHTTP
	}

	return nil
}

// registerRoutes registers all routes with the router.
// This includes backend routes and composite routes.
func (m *ReverseProxyModule) registerRoutes() {
	// Register all backend-specific routes first
	for backendID, routes := range m.backendRoutes {
		for route, handler := range routes {
			m.router.HandleFunc(route, handler)
			m.app.Logger().Info("Registered route", "route", route, "backend", backendID)
		}
	}

	// Register all composite routes
	for route, handler := range m.compositeRoutes {
		m.router.HandleFunc(route, handler)
		m.app.Logger().Info("Registered composite route", "route", route)
	}
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

	return proxy
}

// AddBackendRoute adds a route pattern for a specific backend.
// The route pattern is registered with the router during the Start phase.
// Example: AddBackendRoute("twitter", "/api/twitter/*") will route all requests
// to /api/twitter/* to the "twitter" backend.
func (m *ReverseProxyModule) AddBackendRoute(backendID, pattern string) {
	// Initialize backend routes map if needed
	if _, ok := m.backendRoutes[backendID]; !ok {
		m.backendRoutes[backendID] = make(map[string]http.HandlerFunc)
	}

	// Ensure the backend is configured
	serviceURL, ok := m.config.BackendServices[backendID]
	if !ok || serviceURL == "" {
		m.app.Logger().Error("Backend not configured or has empty URL", "backend", backendID)
		return
	}

	// Create reverse proxy for backend
	backendURL, err := url.Parse(serviceURL)
	if err != nil {
		m.app.Logger().Error("Failed to parse URL", "backend", backendID, "error", err)
		return
	}

	// Create or update the reverse proxy for the backend
	proxy := m.createReverseProxy(backendURL)
	m.backendProxies[backendID] = proxy

	// Get tenant-specific proxies for this backend or create a new map
	tenantProxies := m.tenantBackendProxies[backendID]
	if tenantProxies == nil {
		tenantProxies = make(map[modular.TenantID]*httputil.ReverseProxy)
		m.tenantBackendProxies[backendID] = tenantProxies
	}

	// Create tenant-specific proxies if needed
	for tenantID, tenantCfg := range m.tenants {
		if tenantCfg == nil || tenantCfg.BackendServices == nil {
			continue
		}

		if tenantServiceURL, ok := tenantCfg.BackendServices[backendID]; ok && tenantServiceURL != "" {
			if tenantServiceURL == serviceURL {
				// Use the same proxy for the tenant as the default backend
				tenantProxies[tenantID] = proxy
				continue
			}

			// Create tenant-specific reverse proxy
			tenantBackendURL, err := url.Parse(tenantServiceURL)
			if err != nil {
				m.app.Logger().Error("Failed to parse tenant URL", "tenant", tenantID, "backend", backendID, "error", err)
				continue
			}
			tenantProxies[tenantID] = m.createReverseProxy(tenantBackendURL)
		}
	}

	// Register the route handler for this pattern
	m.registerBackendRoute(backendID, pattern, proxy, tenantProxies)

	m.app.Logger().Info("Registered backend route", "pattern", pattern, "backend", backendID)
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
		}

		// Ensure content type is set
		if w.Header().Get("Content-Type") == "" {
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
