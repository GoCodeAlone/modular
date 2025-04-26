package chimux

import (
	"context"
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

// ChiMuxModule represents the chimux module
type ChiMuxModule struct {
	name          string
	config        *ChiMuxConfig
	tenantConfigs map[modular.TenantID]*ChiMuxConfig
	router        *chi.Mux
	app           modular.TenantApplication
	logger        modular.Logger
}

// Make sure the ChiMuxModule implements all required interfaces
var _ modular.Module = (*ChiMuxModule)(nil)
var _ modular.TenantAwareModule = (*ChiMuxModule)(nil)
var _ RouterService = (*ChiMuxModule)(nil)
var _ ChiRouterService = (*ChiMuxModule)(nil)

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

	// Retrieve the registered config section for access
	cfg, err := app.GetConfigSection(m.name)
	if err != nil {
		return fmt.Errorf("failed to get config section '%s': %w", m.name, err)
	}

	m.config = cfg.GetConfig().(*ChiMuxConfig)
	app.Logger().Debug("Registered config section", "module", m.Name())
	return nil
}

// Init initializes the module
func (m *ChiMuxModule) Init(app modular.Application) error {
	var ok bool
	m.app, ok = app.(modular.TenantApplication)
	if !ok {
		return fmt.Errorf("chimux module requires a TenantApplication")
	}

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

	tcfg, err := m.app.GetTenantConfig(tenantID, m.Name())
	if err != nil {
		m.logger.Error("Failed to get tenant config", "tenantID", tenantID, "error", err)
		return
	}

	if tcfg != nil && tcfg.GetConfig() != nil {
		m.tenantConfigs[tenantID] = tcfg.GetConfig().(*ChiMuxConfig)
	}
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

// ChiRouter returns the underlying chi.Router instance
func (m *ChiMuxModule) ChiRouter() chi.Router {
	return m.router
}

// RouterService interface implementation
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

// Route creates a new sub-router for the given pattern
func (m *ChiMuxModule) Route(pattern string, fn func(r Router)) {
	m.router.Route(pattern, func(r chi.Router) {
		fn(chiRouterWrapper{r})
	})
}

// Mount attaches another http.Handler at the given pattern
func (m *ChiMuxModule) Mount(pattern string, handler http.Handler) {
	m.router.Mount(pattern, handler)
}

// Use appends middleware to the chain
func (m *ChiMuxModule) Use(middleware ...func(http.Handler) http.Handler) {
	m.router.Use(middleware...)
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

// chiRouterWrapper wraps a chi.Router to implement the Router interface
type chiRouterWrapper struct {
	router chi.Router
}

// Get implements RouterService.Get for the wrapper
func (r chiRouterWrapper) Get(pattern string, handler http.HandlerFunc) {
	r.router.Get(pattern, handler)
}

// Post implements RouterService.Post for the wrapper
func (r chiRouterWrapper) Post(pattern string, handler http.HandlerFunc) {
	r.router.Post(pattern, handler)
}

// Put implements RouterService.Put for the wrapper
func (r chiRouterWrapper) Put(pattern string, handler http.HandlerFunc) {
	r.router.Put(pattern, handler)
}

// Delete implements RouterService.Delete for the wrapper
func (r chiRouterWrapper) Delete(pattern string, handler http.HandlerFunc) {
	r.router.Delete(pattern, handler)
}

// Patch implements RouterService.Patch for the wrapper
func (r chiRouterWrapper) Patch(pattern string, handler http.HandlerFunc) {
	r.router.Patch(pattern, handler)
}

// Head implements RouterService.Head for the wrapper
func (r chiRouterWrapper) Head(pattern string, handler http.HandlerFunc) {
	r.router.Head(pattern, handler)
}

// Options implements RouterService.Options for the wrapper
func (r chiRouterWrapper) Options(pattern string, handler http.HandlerFunc) {
	r.router.Options(pattern, handler)
}

// Route implements RouterService.Route for the wrapper
func (r chiRouterWrapper) Route(pattern string, fn func(r Router)) {
	r.router.Route(pattern, func(subRouter chi.Router) {
		fn(chiRouterWrapper{subRouter})
	})
}

// Mount implements RouterService.Mount for the wrapper
func (r chiRouterWrapper) Mount(pattern string, handler http.Handler) {
	r.router.Mount(pattern, handler)
}

// Use implements RouterService.Use for the wrapper
func (r chiRouterWrapper) Use(middleware ...func(http.Handler) http.Handler) {
	r.router.Use(middleware...)
}

// Handle implements RouterService.Handle for the wrapper
func (r chiRouterWrapper) Handle(pattern string, handler http.Handler) {
	r.router.Handle(pattern, handler)
}

// HandleFunc implements RouterService.HandleFunc for the wrapper
func (r chiRouterWrapper) HandleFunc(pattern string, handler http.HandlerFunc) {
	r.router.HandleFunc(pattern, handler)
}
