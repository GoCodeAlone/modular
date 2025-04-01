package modular

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"slices"
	"syscall"
	"time"
)

//go:generate mockgen -destination=./mock_application_test.go -source=$GOFILE -package=$GOPACKAGE

type AppRegistry interface {
	SvcRegistry() ServiceRegistry
}

// StdApplication represents the core StdApplication container
type StdApplication struct {
	cfgProvider    ConfigProvider
	cfgSections    map[string]ConfigProvider
	svcRegistry    ServiceRegistry
	moduleRegistry ModuleRegistry
	logger         Logger
	ctx            context.Context
	cancel         context.CancelFunc
	tenantService  TenantService // Added tenant service reference
}

// NewStdApplication creates a new application instance
func NewStdApplication(cp ConfigProvider, logger Logger) Application {
	return &StdApplication{
		cfgProvider:    cp,
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         logger,
	}
}

// ConfigProvider retrieves the application config provider
func (app *StdApplication) ConfigProvider() ConfigProvider {
	return app.cfgProvider
}

// SvcRegistry retrieves the service svcRegistry
func (app *StdApplication) SvcRegistry() ServiceRegistry {
	return app.svcRegistry
}

// RegisterModule adds a module to the application
func (app *StdApplication) RegisterModule(module Module) {
	app.moduleRegistry[module.Name()] = module
}

// RegisterConfigSection registers a configuration section with the application
func (app *StdApplication) RegisterConfigSection(section string, cp ConfigProvider) {
	app.cfgSections[section] = cp
}

// ConfigSections retrieves all registered configuration sections
func (app *StdApplication) ConfigSections() map[string]ConfigProvider {
	return app.cfgSections
}

// GetConfigSection retrieves a configuration section
func (app *StdApplication) GetConfigSection(section string) (ConfigProvider, error) {
	cp, exists := app.cfgSections[section]
	if !exists {
		return nil, fmt.Errorf("config section '%s' not found", section)
	}
	return cp, nil
}

// RegisterService adds a service with type checking
func (app *StdApplication) RegisterService(name string, service any) error {
	if _, exists := app.svcRegistry[name]; exists {
		return fmt.Errorf("service '%s' already registered", name)
	}

	app.svcRegistry[name] = service
	app.logger.Info("Registered service", "name", name, "type", reflect.TypeOf(service))
	return nil
}

// GetService retrieves a service with type assertion
func (app *StdApplication) GetService(name string, target any) error {
	service, exists := app.svcRegistry[name]
	if !exists {
		return fmt.Errorf("service '%s' not found", name)
	}

	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	if targetValue.IsNil() {
		return fmt.Errorf("target cannot be nil")
	}

	serviceType := reflect.TypeOf(service)
	if !targetValue.Elem().IsValid() {
		return fmt.Errorf("target value is invalid")
	}

	targetType := targetValue.Elem().Type()

	// Special case for interfaces
	if targetType.Kind() == reflect.Interface && serviceType.Implements(targetType) {
		targetValue.Elem().Set(reflect.ValueOf(service))
		return nil
	}

	// Special case for structs with embedded interfaces
	if targetType.Kind() == reflect.Struct {
		for i := 0; i < targetType.NumField(); i++ {
			field := targetType.Field(i)
			// Check if the field is an interface and the service implements it
			if field.Type.Kind() == reflect.Interface && serviceType.Implements(field.Type) {
				// Set the interface field to the service value
				fieldValue := targetValue.Elem().Field(i)
				if fieldValue.CanSet() {
					fieldValue.Set(reflect.ValueOf(service))
					return nil
				}
			}
		}
	}

	// Handle pointers correctly - check if the service type is directly assignable to target type
	// or if service is a pointer and the underlying type is assignable
	if !serviceType.AssignableTo(targetType) {
		// For pointers, we might need to dereference one level
		if serviceType.Kind() == reflect.Ptr && serviceType.Elem().AssignableTo(targetType) {
			// Dereference the service pointer and set target to the dereferenced value
			targetValue.Elem().Set(reflect.ValueOf(service).Elem())
			return nil
		}

		return fmt.Errorf("service '%s' of type %s cannot be assigned to %s",
			name, serviceType, targetType)
	}

	targetValue.Elem().Set(reflect.ValueOf(service))
	return nil
}

// Init initializes the application with the provided modules
func (app *StdApplication) Init() error {
	for name, module := range app.moduleRegistry {
		module.RegisterConfig(app)
		app.logger.Info("Registering module", "name", name)
	}

	if err := AppConfigLoader(app); err != nil {
		return fmt.Errorf("failed to load app config: %w", err)
	}

	// Register services provided by modules
	for name, module := range app.moduleRegistry {
		for _, svc := range module.ProvidesServices() {
			if err := app.RegisterService(svc.Name, svc.Instance); err != nil {
				return fmt.Errorf("module '%s' failed to register service: %w", name, err)
			}
		}
	}

	// Build dependency graph
	moduleOrder, err := app.resolveDependencies()
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Initialize modules in order
	for _, moduleName := range moduleOrder {
		// Inject required services
		app.moduleRegistry[moduleName], err = app.injectServices(app.moduleRegistry[moduleName])
		if err != nil {
			return fmt.Errorf("failed to inject services for module '%s': %w", moduleName, err)
		}

		if err = app.moduleRegistry[moduleName].Init(app); err != nil {
			return fmt.Errorf("failed to initialize module '%s': %w", moduleName, err)
		}
		app.logger.Info(fmt.Sprintf("Initialized module %s", moduleName))
	}

	// Initialize tenant configuration after modules have registered their configurations
	if err = app.initTenantConfigurations(); err != nil {
		return fmt.Errorf("failed to initialize tenant configurations: %w", err)
	}

	return nil
}

// initTenantConfigurations initializes tenant configurations after modules have registered their configs
func (app *StdApplication) initTenantConfigurations() error {
	var tenantSvc TenantService
	if err := app.GetService("tenantService", &tenantSvc); err == nil {
		app.tenantService = tenantSvc

		// If there's a TenantConfigLoader service, use it to load tenant configs
		var loader TenantConfigLoader
		if err = app.GetService("tenantConfigLoader", &loader); err == nil {
			app.logger.Info("Loading tenant configurations using TenantConfigLoader")
			if err = loader.LoadTenantConfigurations(app, tenantSvc); err != nil {
				return fmt.Errorf("failed to load tenant configurations: %w", err)
			}
		}

		// Register tenant-aware modules with the tenant service
		// instead of directly notifying them, to avoid duplicate notifications
		for _, module := range app.moduleRegistry {
			if tenantAwareModule, ok := module.(TenantAwareModule); ok {
				// The tenant service will handle notification about existing tenants
				if standardTenantSvc, ok := tenantSvc.(*StandardTenantService); ok {
					standardTenantSvc.RegisterTenantAwareModule(tenantAwareModule)
				}
			}
		}
	} else {
		app.logger.Info("Tenant service not found, skipping tenant configuration initialization")
	}

	return nil
}

// Start starts the application
func (app *StdApplication) Start() error {
	// Create cancellable context for the application
	ctx, cancel := context.WithCancel(context.Background())
	app.ctx = ctx
	app.cancel = cancel

	// Start modules in dependency order
	modules, err := app.resolveDependencies()
	if err != nil {
		return err
	}

	for _, name := range modules {
		module := app.moduleRegistry[name]
		app.logger.Info("Starting module", "module", name)
		if err := module.Start(ctx); err != nil {
			return fmt.Errorf("failed to start module %s: %w", name, err)
		}
	}

	return nil
}

// Stop stops the application
func (app *StdApplication) Stop() error {
	// Get modules in reverse dependency order
	modules, err := app.resolveDependencies()
	if err != nil {
		return err
	}

	// Reverse the slice
	slices.Reverse(modules)

	// Create timeout context for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop modules in reverse order
	var lastErr error
	for _, name := range modules {
		module := app.moduleRegistry[name]
		app.logger.Info("Stopping module", "module", name)
		if err := module.Stop(ctx); err != nil {
			app.logger.Error("Error stopping module", "module", name, "error", err)
			lastErr = err
		}
	}

	// Cancel the main application context
	if app.cancel != nil {
		app.cancel()
	}

	return lastErr
}

// Run starts the application and blocks until termination
func (app *StdApplication) Run() error {
	// Initialize
	if err := app.Init(); err != nil {
		return err
	}

	// Start all modules
	if err := app.Start(); err != nil {
		return err
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for termination signal
	sig := <-sigChan
	app.logger.Info("Received signal, shutting down", "signal", sig)

	// Stop all modules
	return app.Stop()
}

// injectServices injects required services into a module
func (app *StdApplication) injectServices(module Module) (Module, error) {
	requiredServices := make(map[string]any)
	for _, dep := range module.RequiresServices() {
		if service, exists := app.svcRegistry[dep.Name]; exists {
			if valid, err := checkServiceCompatibility(service, dep); !valid {
				return nil, fmt.Errorf("failed to inject service '%s': %w", dep.Name, err)
			}

			requiredServices[dep.Name] = service
		} else if dep.Required {
			return nil, fmt.Errorf("required service '%s' not found for module '%s'",
				dep.Name, module.Name())
		}
	}

	// If module supports constructor injection, use it
	if withConstructor, ok := module.(ModuleWithConstructor); ok {
		constructor := withConstructor.Constructor()
		newModule, err := constructor(app, requiredServices)
		if err != nil {
			return nil, fmt.Errorf("failed to construct module '%s': %w", module.Name(), err)
		}

		// Replace in registry with constructed instance
		app.moduleRegistry[module.Name()] = newModule
		module = newModule
	}

	// TODO: potentially add support for field injection or other DI methods

	return module, nil
}

// checkServiceCompatibility checks if a service satisfies the dependency requirements
func checkServiceCompatibility(service any, dep ServiceDependency) (bool, error) {
	if service == nil {
		return false, fmt.Errorf("service '%s' is nil", dep.Name)
	}

	serviceType := reflect.TypeOf(service)

	// Check concrete type if specified
	if dep.Type != nil && !serviceType.AssignableTo(dep.Type) {
		return false, fmt.Errorf("service '%s' of type %s doesn't satisfy required type %s",
			dep.Name, serviceType, dep.Type)
	}

	// Check interface satisfaction - handle pointer types better
	if dep.SatisfiesInterface != nil && dep.SatisfiesInterface.Kind() == reflect.Interface {
		// Direct implementation check
		if serviceType.Implements(dep.SatisfiesInterface) {
			return true, nil
		}

		// For pointer types, check if the pointed-to type implements it
		if serviceType.Kind() == reflect.Ptr && serviceType.Elem().Implements(dep.SatisfiesInterface) {
			return true, nil
		}

		return false, fmt.Errorf("service '%s' of type %s doesn't satisfy required interface %s",
			dep.Name, serviceType, dep.SatisfiesInterface)
	}

	return true, nil
}

// Logger represents a logger
func (app *StdApplication) Logger() Logger {
	return app.logger
}

// resolveDependencies returns modules in initialization order
func (app *StdApplication) resolveDependencies() ([]string, error) {
	// Create dependency graph
	graph := make(map[string][]string)
	for name, module := range app.moduleRegistry {
		graph[name] = module.Dependencies()
	}

	// Topological sort
	var result []string
	visited := make(map[string]bool)
	temp := make(map[string]bool)

	var visit func(string) error
	visit = func(node string) error {
		if temp[node] {
			return fmt.Errorf("circular dependency detected: %s", node)
		}
		if visited[node] {
			return nil
		}
		temp[node] = true

		for _, dep := range graph[node] {
			if _, exists := app.moduleRegistry[dep]; !exists {
				return fmt.Errorf("module '%s' depends on non-existent module '%s'", node, dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}

		visited[node] = true
		temp[node] = false
		result = append(result, node)
		return nil
	}

	// Visit all nodes
	for node := range graph {
		if !visited[node] {
			if err := visit(node); err != nil {
				return nil, err
			}
		}
	}

	// log result
	app.logger.Info("Module initialization order", "order", result)

	return result, nil
}

// GetTenantService returns the application's tenant service if available
func (app *StdApplication) GetTenantService() (TenantService, error) {
	var ts TenantService
	if err := app.GetService("tenantService", &ts); err != nil {
		return nil, fmt.Errorf("tenant service not available: %w", err)
	}
	return ts, nil
}

// WithTenant creates a tenant context from the application context
func (app *StdApplication) WithTenant(tenantID TenantID) (*TenantContext, error) {
	if app.ctx == nil {
		return nil, fmt.Errorf("application context not initialized")
	}
	return NewTenantContext(app.ctx, tenantID), nil
}

// GetTenantConfig retrieves configuration for a specific tenant and section
func (app *StdApplication) GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error) {
	ts, err := app.GetTenantService()
	if err != nil {
		return nil, err
	}
	return ts.GetTenantConfig(tenantID, section)
}

type Application interface {
	ConfigProvider() ConfigProvider
	SvcRegistry() ServiceRegistry
	RegisterModule(module Module)
	RegisterConfigSection(section string, cp ConfigProvider)
	ConfigSections() map[string]ConfigProvider
	GetConfigSection(section string) (ConfigProvider, error)
	RegisterService(name string, service any) error
	GetService(name string, target any) error
	Init() error
	Start() error
	Stop() error
	Run() error
	Logger() Logger
}

type TenantApplication interface {
	GetTenantService() (TenantService, error)
	WithTenant(tenantID TenantID) (*TenantContext, error)
	GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error)
}
