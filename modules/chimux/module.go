// Package chimux provides a Chi-based HTTP router module for the modular framework.
//
// This module wraps the popular Go Chi router and integrates it with the modular
// framework's service system, providing HTTP routing, middleware management, CORS
// support, and tenant-aware configuration.
//
// # Features
//
// The chimux module offers the following capabilities:
//   - HTTP routing with pattern matching and parameter extraction
//   - Middleware chain management with automatic service discovery
//   - CORS configuration with per-tenant customization
//   - Base path support for sub-application mounting
//   - Tenant-aware configuration for multi-tenant applications
//   - Service registration for dependency injection
//
// # Requirements
//
// The chimux module requires a TenantApplication to operate. It will return an
// error if initialized with a regular Application instance.
//
// # Configuration
//
// The module can be configured through the ChiMuxConfig structure:
//
//	config := &ChiMuxConfig{
//	    AllowedOrigins:   []string{"https://example.com", "https://app.example.com"},
//	    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
//	    AllowedHeaders:   []string{"Origin", "Accept", "Content-Type", "Authorization"},
//	    AllowCredentials: true,
//	    MaxAge:           3600,
//	    Timeout:          30000,
//	    BasePath:         "/api/v1",
//	}
//
// # Service Registration
//
// The module registers multiple services for different use cases:
//   - "chimux.router": The full ChiMuxModule instance
//   - "router": BasicRouter interface for simple routing needs
//   - "chi.router": Direct access to the underlying Chi router
//
// # Usage Examples
//
// Basic routing:
//
//	router := app.GetService("router").(chimux.BasicRouter)
//	router.Get("/users", getUsersHandler)
//	router.Post("/users", createUserHandler)
//	router.Get("/users/{id}", getUserHandler)
//
// Advanced routing with Chi features:
//
//	chiRouter := app.GetService("chi.router").(chi.Router)
//	chiRouter.Route("/api", func(r chi.Router) {
//	    r.Use(authMiddleware)
//	    r.Get("/protected", protectedHandler)
//	})
//
// Middleware integration:
//
//	// Modules implementing MiddlewareProvider are automatically discovered
//	type MyModule struct{}
//
//	func (m *MyModule) ProvideMiddleware() []chimux.Middleware {
//	    return []chimux.Middleware{
//	        myCustomMiddleware,
//	        loggingMiddleware,
//	    }
//	}
//
// # Tenant Support
//
// The module supports tenant-specific configurations:
//
//	// Different tenants can have different CORS settings
//	tenant1Config := &ChiMuxConfig{
//	    AllowedOrigins: []string{"https://tenant1.example.com"},
//	}
//	tenant2Config := &ChiMuxConfig{
//	    AllowedOrigins: []string{"https://tenant2.example.com"},
//	}
package chimux

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// ModuleName is the unique identifier for the chimux module.
const ModuleName = "chimux"

// ServiceName is the name of the primary service provided by this module.
// Use this to request the chimux router service through dependency injection.
const ServiceName = "chimux.router"

// Error definitions for the chimux module.
var (
	// ErrRequiresTenantApplication is returned when the module is initialized
	// with a non-tenant application. The chimux module requires tenant support
	// for proper multi-tenant routing and configuration.
	ErrRequiresTenantApplication = errors.New("chimux module requires a TenantApplication")
	// Sentinel errors for runtime operations (avoid dynamic error construction per err113)
	ErrMiddlewareNotFound       = errors.New("middleware not found")
	ErrMiddlewareAlreadyRemoved = errors.New("middleware already removed")
	ErrRouteNotFound            = errors.New("route not found")
	ErrRouteAlreadyDisabled     = errors.New("route already disabled")
)

// ChiMuxModule provides HTTP routing functionality using the Chi router library.
// It integrates Chi with the modular framework's service system and provides
// tenant-aware configuration, middleware management, and CORS support.
//
// The module implements the following interfaces:
//   - modular.Module: Basic module lifecycle
//   - modular.Configurable: Configuration management
//   - modular.ServiceAware: Service dependency management
//   - modular.Startable: Startup logic
//   - modular.Stoppable: Shutdown logic
//   - modular.TenantAwareModule: Tenant lifecycle management
//   - BasicRouter: Basic HTTP routing
//   - Router: Extended Chi router functionality
//   - ChiRouterService: Direct Chi router access
//
// The router is thread-safe and supports concurrent request handling.
type ChiMuxModule struct {
	name          string
	config        *ChiMuxConfig
	tenantConfigs map[modular.TenantID]*ChiMuxConfig
	router        *chi.Mux
	app           modular.TenantApplication
	logger        modular.Logger
	subject       modular.Subject // Added for event observation
	// disabledRoutes keeps track of routes that have been disabled at runtime.
	// Keyed by HTTP method (uppercase) then the original registered pattern.
	// A disabled route short‑circuits matching before reaching the underlying chi mux
	// allowing dynamic feature flag style shutdown without removing the route from
	// the registry (so it can be re‑enabled later). Patterns are stored exactly as
	// originally registered to avoid ambiguity with chi's internal normalized form.
	disabledRoutes map[string]map[string]bool
	// disabledMu guards access to disabledRoutes for concurrent reads/writes.
	disabledMu sync.RWMutex
	// routeRegistry tracks registered routes with their methods for runtime management.
	routeRegistry []struct{ method, pattern string }
	// middleware tracking for runtime enable/disable
	middlewareMu    sync.RWMutex
	middlewares     map[string]*controllableMiddleware // keyed by middleware name provided at registration
	middlewareOrder []string                           // preserves deterministic application order for rebuilds
}

// NewChiMuxModule creates a new instance of the chimux module.
// This is the primary constructor for the chimux module and should be used
// when registering the module with the application.
//
// Example:
//
//	app.RegisterModule(chimux.NewChiMuxModule())
func NewChiMuxModule() modular.Module {
	return &ChiMuxModule{
		name:           ModuleName,
		tenantConfigs:  make(map[modular.TenantID]*ChiMuxConfig),
		disabledRoutes: make(map[string]map[string]bool),
		middlewares:    make(map[string]*controllableMiddleware),
	}
}

// controllableMiddleware wraps a middleware with an enabled flag so it can be disabled at runtime.
type controllableMiddleware struct {
	name    string
	fn      Middleware
	enabled atomic.Bool
}

func (cm *controllableMiddleware) Wrap(next http.Handler) http.Handler {
	underlying := cm.fn(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cm.enabled.Load() {
			underlying.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Name returns the unique identifier for this module.
// This name is used for service registration, dependency resolution,
// and configuration section identification.
func (m *ChiMuxModule) Name() string {
	return m.name
}

// RegisterConfig registers the module's configuration structure.
// This method is called during application initialization to register
// the default configuration values for the chimux module.
//
// Default configuration:
//   - AllowedOrigins: ["*"] (all origins allowed)
//   - AllowedMethods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
//   - AllowedHeaders: ["Origin", "Accept", "Content-Type", "X-Requested-With", "Authorization"]
//   - AllowCredentials: false
//   - MaxAge: 300 seconds (5 minutes)
//   - Timeout: 60s (60 seconds)
func (m *ChiMuxModule) RegisterConfig(app modular.Application) error {
	// Register the configuration with default values
	defaultConfig := &ChiMuxConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Accept", "Content-Type", "X-Requested-With", "Authorization"},
		AllowCredentials: false,
		MaxAge:           300,
		Timeout:          60 * time.Second,
	}

	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(defaultConfig))
	app.Logger().Debug("Registered config section", "module", m.Name())
	return nil
}

// Init initializes the chimux module with the application context.
// This method is called after all modules have been registered and their
// configurations loaded. It sets up the Chi router, applies middleware,
// and configures CORS settings.
//
// The initialization process:
//  1. Validates that the application supports tenants
//  2. Loads the module configuration
//  3. Creates and configures the Chi router
//  4. Sets up default middleware (RequestID, RealIP, Logger, Recoverer)
//  5. Applies CORS middleware based on configuration
//  6. Discovers and applies middleware from other modules
//
// Requirements:
//   - Must be used with a TenantApplication
//   - Configuration must be properly loaded
func (m *ChiMuxModule) Init(app modular.Application) error {
	if err := m.initApplication(app); err != nil {
		return err
	}

	if err := m.initConfig(app); err != nil {
		return err
	}

	if err := m.initRouter(); err != nil {
		return err
	}

	if err := m.setupMiddleware(app); err != nil {
		return err
	}

	// Emit configuration loaded event
	ctx := context.Background()
	m.emitEvent(ctx, EventTypeConfigLoaded, map[string]interface{}{
		"allowed_origins":   m.config.AllowedOrigins,
		"allowed_methods":   m.config.AllowedMethods,
		"allowed_headers":   m.config.AllowedHeaders,
		"allow_credentials": m.config.AllowCredentials,
		"max_age":           m.config.MaxAge,
		"timeout_ms":        m.config.Timeout,
		"base_path":         m.config.BasePath,
	})

	// Emit configuration validated event
	m.emitEvent(ctx, EventTypeConfigValidated, map[string]interface{}{
		"validation_status": "success",
		"config_sections":   []string{"cors", "router", "middleware"},
	})

	m.logger.Info("Chimux module initialized")
	return nil
}

// initApplication initializes the application context
func (m *ChiMuxModule) initApplication(app modular.Application) error {
	var ok bool
	m.app, ok = app.(modular.TenantApplication)
	if !ok {
		return fmt.Errorf("%w", ErrRequiresTenantApplication)
	}

	m.logger = m.app.Logger()
	m.logger.Info("Initializing chimux module")
	return nil
}

// initConfig initializes the module configuration
func (m *ChiMuxModule) initConfig(app modular.Application) error {
	cfg, err := app.GetConfigSection(m.name)
	if err != nil {
		return fmt.Errorf("failed to get config section '%s': %w", m.name, err)
	}

	m.config = cfg.GetConfig().(*ChiMuxConfig)

	return nil
}

// initRouter initializes the chi router with default middleware
func (m *ChiMuxModule) initRouter() error {
	m.router = chi.NewRouter()
	m.logger.Debug("Created chi router instance", "module", m.Name())

	// Set up default middleware
	m.router.Use(middleware.RequestID)
	m.router.Use(middleware.RealIP)
	m.router.Use(middleware.Logger)
	m.router.Use(middleware.Recoverer)

	middleware.DefaultLogger = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.logger.Debug("Request", "method", r.Method, "path", r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}

	// Apply CORS middleware using the configuration
	m.router.Use(m.corsMiddleware())

	// Apply disabled routes middleware early so disabled routes short-circuit
	m.router.Use(m.disabledRouteMiddleware())

	// Apply request monitoring middleware for event emission (after disabled check so we don't emit normal request events for disabled routes)
	m.router.Use(m.requestMonitoringMiddleware())

	// Emit CORS configured event
	m.emitEvent(context.Background(), EventTypeCorsConfigured, map[string]interface{}{
		"allowed_origins":     m.config.AllowedOrigins,
		"allowed_methods":     m.config.AllowedMethods,
		"allowed_headers":     m.config.AllowedHeaders,
		"credentials_enabled": m.config.AllowCredentials,
	})

	// Emit router created event
	m.emitEvent(context.Background(), EventTypeRouterCreated, map[string]interface{}{
		"base_path":    m.config.BasePath,
		"cors_enabled": len(m.config.AllowedOrigins) > 0,
	})

	m.logger.Debug("Applied CORS middleware with config",
		"allowedOrigins", m.config.AllowedOrigins,
		"allowedMethods", m.config.AllowedMethods,
		"allowedHeaders", m.config.AllowedHeaders,
		"allowCredentials", m.config.AllowCredentials,
		"maxAge", m.config.MaxAge)

	return nil
}

// setupMiddleware finds and applies middleware from service providers
func (m *ChiMuxModule) setupMiddleware(app modular.Application) error {
	// Find middleware providers using interface-based matching
	var middlewareProviders []MiddlewareProvider

	// Look for all services in the registry that implement MiddlewareProvider
	for name, service := range app.SvcRegistry() {
		if service == nil {
			continue
		}

		serviceType := reflect.TypeOf(service)
		middlewareProviderType := reflect.TypeOf((*MiddlewareProvider)(nil)).Elem()

		if serviceType.Implements(middlewareProviderType) ||
			(serviceType.Kind() == reflect.Ptr && serviceType.Elem().Implements(middlewareProviderType)) {
			if provider, ok := service.(MiddlewareProvider); ok {
				middlewareProviders = append(middlewareProviders, provider)
				m.logger.Debug("Found middleware provider", "name", name)
			}
		}
	}

	// Apply middleware from providers
	for _, provider := range middlewareProviders {
		for _, mw := range provider.ProvideMiddleware() {
			m.router.Use(mw)
		}
	}

	return nil
}

// Start performs startup logic for the module.
// This method loads tenant-specific configurations that may have been
// registered after module initialization. It's called after all modules
// have been initialized and are ready to start.
//
// The startup process:
//  1. Loads configurations for all registered tenants
//  2. Applies tenant-specific CORS and routing settings
//  3. Prepares the router for incoming requests
func (m *ChiMuxModule) Start(ctx context.Context) error {
	m.logger.Info("Starting chimux module")

	// Load tenant configurations now that it's safe to do so
	m.loadTenantConfigs()

	// Re-emit config loaded event (redundant-safe) to ensure observers in complex full-suite
	// executions capture this critical lifecycle marker. This guards against any ordering
	// or observer registration timing nuances seen in integrated test runs.
	m.emitEvent(ctx, EventTypeConfigLoaded, map[string]interface{}{
		"allowed_origins":   m.config.AllowedOrigins,
		"allowed_methods":   m.config.AllowedMethods,
		"allowed_headers":   m.config.AllowedHeaders,
		"allow_credentials": m.config.AllowCredentials,
		"max_age":           m.config.MaxAge,
		"timeout_ms":        m.config.Timeout,
		"base_path":         m.config.BasePath,
		"phase":             "start",
	})

	// Emit router started event (router is ready to handle requests)
	m.emitEvent(ctx, EventTypeRouterStarted, map[string]interface{}{
		"router_status": "started",
		"start_time":    time.Now(),
		"tenant_count":  len(m.tenantConfigs),
		"base_path":     m.config.BasePath,
	})

	// Emit module started event
	m.emitEvent(ctx, EventTypeModuleStarted, map[string]interface{}{
		"tenant_count":     len(m.tenantConfigs),
		"base_path":        m.config.BasePath,
		"cors_enabled":     len(m.config.AllowedOrigins) > 0,
		"middleware_count": len(m.router.Middlewares()),
	})

	return nil
}

// Stop performs shutdown logic for the module.
// This method gracefully shuts down the router and cleans up resources.
// Note that the HTTP server itself is typically managed by a separate
// HTTP server module.
func (m *ChiMuxModule) Stop(ctx context.Context) error {
	m.logger.Info("Stopping chimux module")

	// Emit router stopped event (router is shutting down)
	m.emitEvent(ctx, EventTypeRouterStopped, map[string]interface{}{
		"router_status": "stopped",
		"stop_time":     time.Now(),
		"tenant_count":  len(m.tenantConfigs),
	})

	// Emit module stopped event
	m.emitEvent(ctx, EventTypeModuleStopped, map[string]interface{}{
		"tenant_count": len(m.tenantConfigs),
		"routes_count": len(m.router.Routes()),
	})

	return nil
}

// Dependencies returns the names of modules this module depends on.
// The chimux module has no hard dependencies and can be started independently.
// However, it will automatically discover and integrate with modules that
// implement MiddlewareProvider.
func (m *ChiMuxModule) Dependencies() []string {
	return nil
}

// ProvidesServices declares services provided by this module.
// The chimux module provides multiple service interfaces to accommodate
// different usage patterns and integration needs.
//
// Provided services:
//   - "chimux.router": The full ChiMuxModule instance
//   - "router": BasicRouter interface for simple routing needs
//   - "chi.router": Direct access to the underlying Chi router
func (m *ChiMuxModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        ServiceName,
			Description: "Chi router service for HTTP routing",
			Instance:    m,
		},
		{
			Name:        "router",
			Description: "Basic router service interface",
			Instance:    m,
		},
		{
			Name:        "chi.router",
			Description: "Full Chi router with Route/Group support",
			Instance:    m.ChiRouter(),
		},
	}
}

// RequiresServices declares services required by this module.
// The chimux module optionally depends on middleware providers.
// It will automatically discover and integrate with any modules
// that implement the MiddlewareProvider interface.
func (m *ChiMuxModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "middleware.provider",
			Required:           false,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*MiddlewareProvider)(nil)).Elem(),
		},
	}
}

// Constructor provides a dependency injection constructor for the module.
// This method is used by the dependency injection system to create
// the module instance with any required services.
func (m *ChiMuxModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		return m, nil
	}
}

// OnTenantRegistered is called when a new tenant is registered.
// This method is part of the TenantAwareModule interface and allows
// the chimux module to prepare tenant-specific configurations.
//
// The actual configuration loading is deferred to avoid deadlocks
// during the tenant registration process.
func (m *ChiMuxModule) OnTenantRegistered(tenantID modular.TenantID) {
	// Check if logger is available (module might not be fully initialized yet)
	if m.logger != nil {
		m.logger.Info("Tenant registered in chimux module", "tenantID", tenantID)
	}

	// Just register the tenant ID and defer config loading to avoid deadlock
	// The actual configuration will be loaded during Start() or when needed
	m.tenantConfigs[tenantID] = nil
}

// OnTenantRemoved is called when a tenant is removed.
// This method cleans up any tenant-specific configurations and resources.
func (m *ChiMuxModule) OnTenantRemoved(tenantID modular.TenantID) {
	// Check if logger is available (module might not be fully initialized yet)
	if m.logger != nil {
		m.logger.Info("Tenant removed from chimux module", "tenantID", tenantID)
	}
	delete(m.tenantConfigs, tenantID)
}

// GetTenantConfig retrieves the loaded configuration for a specific tenant.
// Returns the tenant-specific configuration if available, or the base
// configuration as a fallback.
//
// This method is useful for modules that need to access tenant-specific
// router configurations at runtime.
func (m *ChiMuxModule) GetTenantConfig(tenantID modular.TenantID) *ChiMuxConfig {
	if cfg, ok := m.tenantConfigs[tenantID]; ok {
		return cfg
	}
	return m.config
}

// loadTenantConfigs loads all tenant-specific configurations.
// This should be called during Start() or another safe phase after tenant registration.
func (m *ChiMuxModule) loadTenantConfigs() {
	for tenantID := range m.tenantConfigs {
		// Skip tenants that already have loaded configs
		if m.tenantConfigs[tenantID] != nil {
			continue
		}

		cp, err := m.app.GetTenantConfig(tenantID, m.Name())
		if err != nil {
			m.logger.Error("Failed to get config for tenant", "tenant", tenantID, "module", m.Name(), "error", err)
			continue
		}

		if cp != nil && cp.GetConfig() != nil {
			m.tenantConfigs[tenantID] = cp.GetConfig().(*ChiMuxConfig)
			m.logger.Debug("Loaded tenant config", "tenantID", tenantID)
		}
	}
}

// ChiRouter returns the underlying chi.Router instance
func (m *ChiMuxModule) ChiRouter() chi.Router {
	return m.router
}

// Get registers a GET handler for the pattern
func (m *ChiMuxModule) Get(pattern string, handler http.HandlerFunc) {
	m.router.Get(pattern, handler)
	m.routeRegistry = append(m.routeRegistry, struct{ method, pattern string }{"GET", pattern})

	// Emit route registered event
	m.emitEvent(context.Background(), EventTypeRouteRegistered, map[string]interface{}{
		"method":  "GET",
		"pattern": pattern,
	})
}

// Post registers a POST handler for the pattern
func (m *ChiMuxModule) Post(pattern string, handler http.HandlerFunc) {
	m.router.Post(pattern, handler)
	m.routeRegistry = append(m.routeRegistry, struct{ method, pattern string }{"POST", pattern})

	// Emit route registered event
	m.emitEvent(context.Background(), EventTypeRouteRegistered, map[string]interface{}{
		"method":  "POST",
		"pattern": pattern,
	})
}

// Put registers a PUT handler for the pattern
func (m *ChiMuxModule) Put(pattern string, handler http.HandlerFunc) {
	m.router.Put(pattern, handler)
	m.routeRegistry = append(m.routeRegistry, struct{ method, pattern string }{"PUT", pattern})

	// Emit route registered event
	m.emitEvent(context.Background(), EventTypeRouteRegistered, map[string]interface{}{
		"method":  "PUT",
		"pattern": pattern,
	})
}

// Delete registers a DELETE handler for the pattern
func (m *ChiMuxModule) Delete(pattern string, handler http.HandlerFunc) {
	m.router.Delete(pattern, handler)
	m.routeRegistry = append(m.routeRegistry, struct{ method, pattern string }{"DELETE", pattern})

	// Emit route registered event
	m.emitEvent(context.Background(), EventTypeRouteRegistered, map[string]interface{}{
		"method":  "DELETE",
		"pattern": pattern,
	})
}

// Patch registers a PATCH handler for the pattern
func (m *ChiMuxModule) Patch(pattern string, handler http.HandlerFunc) {
	m.router.Patch(pattern, handler)
	m.routeRegistry = append(m.routeRegistry, struct{ method, pattern string }{"PATCH", pattern})
}

// Head registers a HEAD handler for the pattern
func (m *ChiMuxModule) Head(pattern string, handler http.HandlerFunc) {
	m.router.Head(pattern, handler)
	m.routeRegistry = append(m.routeRegistry, struct{ method, pattern string }{"HEAD", pattern})
}

// Options registers an OPTIONS handler for the pattern
func (m *ChiMuxModule) Options(pattern string, handler http.HandlerFunc) {
	m.router.Options(pattern, handler)
	m.routeRegistry = append(m.routeRegistry, struct{ method, pattern string }{"OPTIONS", pattern})
}

// Mount attaches another http.Handler at the given pattern
func (m *ChiMuxModule) Mount(pattern string, handler http.Handler) {
	m.router.Mount(pattern, handler)
}

// Use appends middleware to the chain
func (m *ChiMuxModule) Use(middlewares ...func(http.Handler) http.Handler) {
	// Backwards compatible: wrap anonymous middlewares assigning generated names
	for idx, mw := range middlewares {
		name := fmt.Sprintf("mw_%d_%d", time.Now().UnixNano(), idx)
		m.UseNamed(name, mw)
	}
}

// UseNamed registers a named middleware that can later be disabled via RemoveMiddleware.
func (m *ChiMuxModule) UseNamed(name string, mw Middleware) {
	cm := &controllableMiddleware{name: name, fn: mw}
	cm.enabled.Store(true)
	m.middlewareMu.Lock()
	m.middlewares[name] = cm
	m.middlewareOrder = append(m.middlewareOrder, name)
	m.middlewareMu.Unlock()
	m.router.Use(cm.Wrap)
	m.emitEvent(context.Background(), EventTypeMiddlewareAdded, map[string]interface{}{
		"middleware_count": 1,
		"total_middleware": len(m.router.Middlewares()),
		"name":             name,
	})
}

// RemoveMiddleware disables a previously registered named middleware. It does not restructure
// the chi chain; instead the wrapper becomes a no-op. Emits EventTypeMiddlewareRemoved.
func (m *ChiMuxModule) RemoveMiddleware(name string) error {
	m.middlewareMu.Lock()
	defer m.middlewareMu.Unlock()
	cm, ok := m.middlewares[name]
	if !ok {
		return fmt.Errorf("%w: %s", ErrMiddlewareNotFound, name)
	}
	if !cm.enabled.Load() {
		return fmt.Errorf("%w: %s", ErrMiddlewareAlreadyRemoved, name)
	}
	cm.enabled.Store(false)
	// Count remaining enabled
	enabledCount := 0
	for _, n := range m.middlewareOrder {
		if mw := m.middlewares[n]; mw != nil && mw.enabled.Load() {
			enabledCount++
		}
	}
	m.emitEvent(context.Background(), EventTypeMiddlewareRemoved, map[string]interface{}{
		"name":              name,
		"remaining_enabled": enabledCount,
	})
	return nil
}

// Handle registers a handler for a specific pattern
func (m *ChiMuxModule) Handle(pattern string, handler http.Handler) {
	m.router.Handle(pattern, handler)
	m.routeRegistry = append(m.routeRegistry, struct{ method, pattern string }{"ANY", pattern})
}

// HandleFunc registers a handler function for a specific pattern
func (m *ChiMuxModule) HandleFunc(pattern string, handler http.HandlerFunc) {
	m.router.HandleFunc(pattern, handler)
	m.routeRegistry = append(m.routeRegistry, struct{ method, pattern string }{"ANY", pattern})
}

// ServeHTTP implements the http.Handler interface to properly handle base path prefixing
func (m *ChiMuxModule) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.config.BasePath != "" {
		// Check if the request path starts with the base path
		if !strings.HasPrefix(r.URL.Path, m.config.BasePath) {
			http.NotFound(w, r)
			return
		}

		// Adjust the path to remove the base path prefix
		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = strings.TrimPrefix(r.URL.Path, m.config.BasePath)
		if r2.URL.Path == "" {
			r2.URL.Path = "/"
		}

		// Serve the modified request
		m.router.ServeHTTP(w, r2)
		return
	}

	// If no base path, serve the request directly
	m.router.ServeHTTP(w, r)
}

// Chi Router methods - delegate to the underlying router
func (m *ChiMuxModule) Route(pattern string, fn func(chi.Router)) chi.Router {
	return m.router.Route(pattern, fn)
}

func (m *ChiMuxModule) Group(fn func(chi.Router)) chi.Router {
	return m.router.Group(fn)
}

func (m *ChiMuxModule) With(middlewares ...func(http.Handler) http.Handler) chi.Router {
	return m.router.With(middlewares...)
}

func (m *ChiMuxModule) Method(method, pattern string, h http.Handler) {
	m.router.Method(method, pattern, h)
}

func (m *ChiMuxModule) MethodFunc(method, pattern string, h http.HandlerFunc) {
	m.router.MethodFunc(method, pattern, h)
}

func (m *ChiMuxModule) Connect(pattern string, h http.HandlerFunc) {
	m.router.Connect(pattern, h)
}

func (m *ChiMuxModule) Trace(pattern string, h http.HandlerFunc) {
	m.router.Trace(pattern, h)
}

func (m *ChiMuxModule) NotFound(h http.HandlerFunc) {
	m.router.NotFound(h)
}

func (m *ChiMuxModule) MethodNotAllowed(h http.HandlerFunc) {
	m.router.MethodNotAllowed(h)
}

// Routes returns the router's route information (part of chi.Routes interface)
func (m *ChiMuxModule) Routes() []chi.Route {
	return m.router.Routes()
}

func (m *ChiMuxModule) Middlewares() chi.Middlewares {
	return m.router.Middlewares()
}

func (m *ChiMuxModule) Match(rctx *chi.Context, method, path string) bool {
	return m.router.Match(rctx, method, path)
}

// DisableRoute disables an existing route (method + pattern) at runtime without removing
// it from the underlying chi router. Subsequent requests that match the route will
// receive a 404 Not Found. Emits EventTypeRouteRemoved once when the route is disabled.
// Returns error if the route was not found or already disabled.
func (m *ChiMuxModule) DisableRoute(method, pattern string) error {
	method = strings.ToUpper(method)
	// Verify route exists in registry
	found := false
	for _, rt := range m.routeRegistry {
		if rt.pattern == pattern && rt.method == method {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%w: %s %s", ErrRouteNotFound, method, pattern)
	}

	m.disabledMu.Lock()
	defer m.disabledMu.Unlock()
	if _, ok := m.disabledRoutes[method]; !ok {
		m.disabledRoutes[method] = make(map[string]bool)
	}
	if m.disabledRoutes[method][pattern] {
		return fmt.Errorf("%w: %s %s", ErrRouteAlreadyDisabled, method, pattern)
	}
	m.disabledRoutes[method][pattern] = true

	// Emit route removed event to signal disabling
	m.emitEvent(context.Background(), EventTypeRouteRemoved, map[string]interface{}{
		"method":  method,
		"pattern": pattern,
		"reason":  "disabled",
	})
	return nil
}

// IsRouteDisabled returns whether a route (method + pattern) is disabled.
func (m *ChiMuxModule) IsRouteDisabled(method, pattern string) bool {
	method = strings.ToUpper(method)
	m.disabledMu.RLock()
	defer m.disabledMu.RUnlock()
	if routes, ok := m.disabledRoutes[method]; ok {
		return routes[pattern]
	}
	return false
}

// disabledRouteMiddleware short-circuits requests to disabled routes returning 404.
// We attempt to determine the matched route pattern using chi's RouteContext. For dynamic
// patterns, chi stores the patterns traversed; we take the last element as the concrete pattern.
func (m *ChiMuxModule) disabledRouteMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Obtain route patterns if available
			rctx := chi.RouteContext(r.Context())
			var pattern string
			if rctx != nil && len(rctx.RoutePatterns) > 0 {
				pattern = rctx.RoutePatterns[len(rctx.RoutePatterns)-1]
			} else {
				// Fallback to the raw request path. WARNING: For parameterized routes (e.g. /users/{id})
				// chi records the pattern as /users/{id} but r.URL.Path will be the concrete value
				// such as /users/123. This means a disabled route registered as /users/{id} will NOT
				// match here and the route may remain active. Admin tooling disabling dynamic routes
				// should therefore prefer invoking DisableRoute() with the original pattern captured
				// at registration time rather than a concrete request path.
				pattern = r.URL.Path
			}
			method := r.Method
			if m.IsRouteDisabled(method, pattern) {
				// Respond 404 without invoking next middleware/handler
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// corsMiddleware creates a CORS middleware handler using the module's configuration
func (m *ChiMuxModule) corsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers based on configuration
			origin := r.Header.Get("Origin")

			// Check if the origin is allowed
			allowed := false
			for _, allowedOrigin := range m.config.AllowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)

				// Set allowed methods
				if len(m.config.AllowedMethods) > 0 {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.config.AllowedMethods, ", "))
				}

				// Set allowed headers
				if len(m.config.AllowedHeaders) > 0 {
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.config.AllowedHeaders, ", "))
				}

				// Set allow credentials
				if m.config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				// Set max age
				if m.config.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", m.config.MaxAge))
				}
			}

			// Handle preflight OPTIONS requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// requestMonitoringMiddleware creates a middleware that emits request events
func (m *ChiMuxModule) requestMonitoringMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Emit request received event
			m.emitEvent(r.Context(), EventTypeRequestReceived, map[string]interface{}{
				"method":      r.Method,
				"path":        r.URL.Path,
				"remote_addr": r.RemoteAddr,
				"user_agent":  r.UserAgent(),
			})

			// Wrap response writer to capture status code
			wrapper := &responseWriterWrapper{ResponseWriter: w, statusCode: 200}

			// Capture context for defer function
			ctx := r.Context()

			// Process request
			defer func() {
				if err := recover(); err != nil {
					// Emit request failed event for panics
					m.emitEvent(ctx, EventTypeRequestFailed, map[string]interface{}{
						"method":      r.Method,
						"path":        r.URL.Path,
						"error":       fmt.Sprintf("%v", err),
						"status_code": 500,
					})
					panic(err) // Re-panic to maintain behavior
				} else {
					// Emit request processed event for successful requests
					m.emitEvent(ctx, EventTypeRequestProcessed, map[string]interface{}{
						"method":      r.Method,
						"path":        r.URL.Path,
						"status_code": wrapper.statusCode,
					})
				}
			}()

			next.ServeHTTP(wrapper, r)

			// Check for error status codes
			if wrapper.statusCode >= 400 {
				m.emitEvent(r.Context(), EventTypeRequestFailed, map[string]interface{}{
					"method":      r.Method,
					"path":        r.URL.Path,
					"status_code": wrapper.statusCode,
					"error":       "HTTP error status",
				})
			}
		})
	}
}

// responseWriterWrapper wraps http.ResponseWriter to capture status code
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// RegisterObservers implements the ObservableModule interface.
// This allows the chimux module to register as an observer for events it's interested in.
func (m *ChiMuxModule) RegisterObservers(subject modular.Subject) error {
	m.subject = subject
	return nil
}

// EmitEvent implements the ObservableModule interface.
// This allows the chimux module to emit events that other modules or observers can receive.
func (m *ChiMuxModule) EmitEvent(ctx context.Context, event cloudevents.Event) error {
	if m.subject == nil {
		return ErrNoSubjectForEventEmission
	}
	if err := m.subject.NotifyObservers(ctx, event); err != nil {
		return fmt.Errorf("failed to notify observers: %w", err)
	}
	return nil
}

// emitEvent is a helper method to create and emit CloudEvents for the chimux module.
// This centralizes the event creation logic and ensures consistent event formatting.
// If no subject is available for event emission, it silently skips the event emission
// to avoid noisy error messages in tests and non-observable applications.
func (m *ChiMuxModule) emitEvent(ctx context.Context, eventType string, data map[string]interface{}) {
	// Skip event emission if no subject is available (non-observable application)
	if m.subject == nil {
		return
	}

	event := modular.NewCloudEvent(eventType, "chimux-service", data, nil)

	if emitErr := m.EmitEvent(ctx, event); emitErr != nil {
		// If no subject is registered, quietly skip to allow non-observable apps to run cleanly
		if errors.Is(emitErr, ErrNoSubjectForEventEmission) {
			return
		}
		// Use structured logger to avoid noisy stdout during tests
		if m.logger != nil {
			m.logger.Debug("Failed to emit chimux event", "eventType", eventType, "error", emitErr)
		}
		// Note: Removed fmt.Printf to eliminate noisy test output
	}
}

// GetRegisteredEventTypes implements the ObservableModule interface.
// Returns all event types that this chimux module can emit.
func (m *ChiMuxModule) GetRegisteredEventTypes() []string {
	return []string{
		EventTypeConfigLoaded,
		EventTypeConfigValidated,
		EventTypeRouterCreated,
		EventTypeRouterStarted,
		EventTypeRouterStopped,
		EventTypeRouteRegistered,
		EventTypeRouteRemoved,
		EventTypeMiddlewareAdded,
		EventTypeMiddlewareRemoved,
		EventTypeCorsConfigured,
		EventTypeCorsEnabled,
		EventTypeModuleStarted,
		EventTypeModuleStopped,
		EventTypeRequestReceived,
		EventTypeRequestProcessed,
		EventTypeRequestFailed,
	}
}
