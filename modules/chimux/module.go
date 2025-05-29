package chimux

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/GoCodeAlone/modular"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// ModuleName is the name of this module
const ModuleName = "chimux"

// ServiceName is the name of the service provided by this module (the chi router)
const ServiceName = "chimux.router"

// Error definitions
var (
	// ErrRequiresTenantApplication is returned when the module is initialized with a non-tenant application
	ErrRequiresTenantApplication = errors.New("chimux module requires a TenantApplication")
)

// ChiMuxModule represents the chimux module
type ChiMuxModule struct {
	name          string
	config        *ChiMuxConfig
	tenantConfigs map[modular.TenantID]*ChiMuxConfig
	router        *chi.Mux
	app           modular.TenantApplication
	logger        modular.Logger
}

// NewChiMuxModule creates a new instance of the chimux module
func NewChiMuxModule() modular.Module {
	return &ChiMuxModule{
		name:          ModuleName,
		tenantConfigs: make(map[modular.TenantID]*ChiMuxConfig),
	}
}

// Name returns the name of the module
func (m *ChiMuxModule) Name() string {
	return m.name
}

// RegisterConfig registers the module's configuration structure
func (m *ChiMuxModule) RegisterConfig(app modular.Application) error {
	// Register the configuration with default values
	defaultConfig := &ChiMuxConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Accept", "Content-Type", "X-Requested-With", "Authorization"},
		AllowCredentials: false,
		MaxAge:           300,
		Timeout:          60000,
	}

	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(defaultConfig))
	app.Logger().Debug("Registered config section", "module", m.Name())
	return nil
}

// Init initializes the module
func (m *ChiMuxModule) Init(app modular.Application) error {
	var ok bool
	m.app, ok = app.(modular.TenantApplication)
	if !ok {
		return fmt.Errorf("%w", ErrRequiresTenantApplication)
	}

	// Retrieve the registered config section for access
	cfg, err := app.GetConfigSection(m.name)
	if err != nil {
		return fmt.Errorf("failed to get config section '%s': %w", m.name, err)
	}

	m.config = cfg.GetConfig().(*ChiMuxConfig)

	m.logger = m.app.Logger()
	m.logger.Info("Initializing chimux module")

	// Create the chi router instance
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
	m.logger.Debug("Applied CORS middleware with config",
		"allowedOrigins", m.config.AllowedOrigins,
		"allowedMethods", m.config.AllowedMethods,
		"allowedHeaders", m.config.AllowedHeaders,
		"allowCredentials", m.config.AllowCredentials,
		"maxAge", m.config.MaxAge)

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

	// NOTE: BasePath handling is now managed in the ServeHTTP method to properly
	// handle all routes regardless of when they're registered

	m.logger.Info("Chimux module initialized")
	return nil
}

// Start performs startup logic for the module
func (m *ChiMuxModule) Start(context.Context) error {
	m.logger.Info("Starting chimux module")

	// Load tenant configurations now that it's safe to do so
	m.loadTenantConfigs()

	return nil
}

// Stop performs shutdown logic for the module
func (m *ChiMuxModule) Stop(context.Context) error {
	m.logger.Info("Stopping chimux module")
	return nil
}

// Dependencies returns the names of modules this module depends on
func (m *ChiMuxModule) Dependencies() []string {
	return nil
}

// ProvidesServices declares services provided by this module
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
			Instance:    m,
		},
	}
}

// RequiresServices declares services required by this module
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

// Constructor provides a dependency injection constructor for the module
func (m *ChiMuxModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		return m, nil
	}
}

// OnTenantRegistered is called when a new tenant is registered
func (m *ChiMuxModule) OnTenantRegistered(tenantID modular.TenantID) {
	m.logger.Info("Tenant registered in chimux module", "tenantID", tenantID)

	// Just register the tenant ID and defer config loading to avoid deadlock
	// The actual configuration will be loaded during Start() or when needed
	m.tenantConfigs[tenantID] = nil
}

// OnTenantRemoved is called when a tenant is removed
func (m *ChiMuxModule) OnTenantRemoved(tenantID modular.TenantID) {
	m.logger.Info("Tenant removed from chimux module", "tenantID", tenantID)
	delete(m.tenantConfigs, tenantID)
}

// GetTenantConfig retrieves the loaded configuration for a specific tenant
// Returns the base config if no specific tenant config is found.
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
}

// Post registers a POST handler for the pattern
func (m *ChiMuxModule) Post(pattern string, handler http.HandlerFunc) {
	m.router.Post(pattern, handler)
}

// Put registers a PUT handler for the pattern
func (m *ChiMuxModule) Put(pattern string, handler http.HandlerFunc) {
	m.router.Put(pattern, handler)
}

// Delete registers a DELETE handler for the pattern
func (m *ChiMuxModule) Delete(pattern string, handler http.HandlerFunc) {
	m.router.Delete(pattern, handler)
}

// Patch registers a PATCH handler for the pattern
func (m *ChiMuxModule) Patch(pattern string, handler http.HandlerFunc) {
	m.router.Patch(pattern, handler)
}

// Head registers a HEAD handler for the pattern
func (m *ChiMuxModule) Head(pattern string, handler http.HandlerFunc) {
	m.router.Head(pattern, handler)
}

// Options registers an OPTIONS handler for the pattern
func (m *ChiMuxModule) Options(pattern string, handler http.HandlerFunc) {
	m.router.Options(pattern, handler)
}

// Mount attaches another http.Handler at the given pattern
func (m *ChiMuxModule) Mount(pattern string, handler http.Handler) {
	m.router.Mount(pattern, handler)
}

// Use appends middleware to the chain
func (m *ChiMuxModule) Use(middlewares ...func(http.Handler) http.Handler) {
	m.router.Use(middlewares...)
}

// Handle registers a handler for a specific pattern
func (m *ChiMuxModule) Handle(pattern string, handler http.Handler) {
	m.router.Handle(pattern, handler)
}

// HandleFunc registers a handler function for a specific pattern
func (m *ChiMuxModule) HandleFunc(pattern string, handler http.HandlerFunc) {
	m.router.HandleFunc(pattern, handler)
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
