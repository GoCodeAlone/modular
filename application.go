package modular

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"slices"
	"syscall"
	"time"
)

// AppRegistry provides registry functionality for applications.
// This interface provides access to the application's service registry,
// allowing modules and components to access registered services.
type AppRegistry interface {
	// SvcRegistry retrieves the service registry.
	// The service registry contains all services registered by modules
	// and the application, providing a central location for service lookup.
	SvcRegistry() ServiceRegistry
}

// Application represents the core application interface with configuration, module management, and service registration.
// This is the main interface that modules interact with during initialization and runtime.
//
// The Application provides a complete framework for:
//   - Managing module lifecycle (registration, initialization, startup, shutdown)
//   - Configuration management with multiple sections and providers
//   - Service registry for inter-module communication
//   - Dependency injection and resolution
//   - Graceful startup and shutdown coordination
//
// Basic usage pattern:
//   app := modular.NewStdApplication(configProvider, logger)
//   app.RegisterModule(&MyModule{})
//   app.RegisterModule(&AnotherModule{})
//   if err := app.Run(); err != nil {
//       log.Fatal(err)
//   }
type Application interface {
	// ConfigProvider retrieves the application's main configuration provider.
	// This provides access to application-level configuration that isn't
	// specific to any particular module.
	ConfigProvider() ConfigProvider

	// SvcRegistry retrieves the service registry.
	// Modules use this to register services they provide and lookup
	// services they need from other modules.
	SvcRegistry() ServiceRegistry

	// RegisterModule adds a module to the application.
	// Modules must be registered before calling Init(). The framework
	// will handle initialization order based on declared dependencies.
	//
	// Example:
	//   app.RegisterModule(&DatabaseModule{})
	//   app.RegisterModule(&WebServerModule{})
	RegisterModule(module Module)

	// RegisterConfigSection registers a configuration section with the application.
	// This allows modules to register their configuration requirements,
	// making them available for loading from configuration sources.
	//
	// Example:
	//   cfg := &MyModuleConfig{}
	//   provider := modular.NewStdConfigProvider(cfg)
	//   app.RegisterConfigSection("mymodule", provider)
	RegisterConfigSection(section string, cp ConfigProvider)

	// ConfigSections retrieves all registered configuration sections.
	// Returns a map of section names to their configuration providers.
	// Useful for debugging and introspection.
	ConfigSections() map[string]ConfigProvider

	// GetConfigSection retrieves a specific configuration section.
	// Returns an error if the section doesn't exist.
	//
	// Example:
	//   provider, err := app.GetConfigSection("database")
	//   if err != nil {
	//       return err
	//   }
	//   cfg := provider.GetConfig().(*DatabaseConfig)
	GetConfigSection(section string) (ConfigProvider, error)

	// RegisterService adds a service to the service registry with type checking.
	// Services registered here become available to all modules that declare
	// them as dependencies.
	//
	// Returns an error if a service with the same name is already registered.
	//
	// Example:
	//   db := &DatabaseConnection{}
	//   err := app.RegisterService("database", db)
	RegisterService(name string, service any) error

	// GetService retrieves a service from the registry with type assertion.
	// The target parameter must be a pointer to the expected type.
	// The framework will perform type checking and assignment.
	//
	// Example:
	//   var db *DatabaseConnection
	//   err := app.GetService("database", &db)
	GetService(name string, target any) error

	// Init initializes the application and all registered modules.
	// This method:
	//   - Calls RegisterConfig on all configurable modules
	//   - Loads configuration from all registered sources
	//   - Resolves module dependencies
	//   - Initializes modules in dependency order
	//   - Registers services provided by modules
	//
	// Must be called before Start() or Run().
	Init() error

	// Start starts the application and all startable modules.
	// Modules implementing the Startable interface will have their
	// Start method called in dependency order.
	//
	// This is typically used when you want to start the application
	// but handle the shutdown logic yourself (rather than using Run()).
	Start() error

	// Stop stops the application and all stoppable modules.
	// Modules implementing the Stoppable interface will have their
	// Stop method called in reverse dependency order.
	//
	// Provides a timeout context for graceful shutdown.
	Stop() error

	// Run starts the application and blocks until termination.
	// This is equivalent to calling Init(), Start(), and then waiting
	// for a termination signal (SIGINT, SIGTERM) before calling Stop().
	//
	// This is the most common way to run a modular application:
	//   if err := app.Run(); err != nil {
	//       log.Fatal(err)
	//   }
	Run() error

	// Logger retrieves the application's logger.
	// This logger is used by the framework and can be used by modules
	// for consistent logging throughout the application.
	Logger() Logger

	// SetLogger sets the application's logger.
	// Should be called before module registration to ensure
	// all framework operations use the new logger.
	SetLogger(logger Logger)
}

// TenantApplication extends Application with multi-tenant functionality.
// This interface adds tenant-aware capabilities to the standard Application,
// allowing the same application instance to serve multiple tenants with
// isolated configurations and contexts.
//
// Multi-tenant applications can:
//   - Maintain separate configurations per tenant
//   - Provide tenant-specific service instances
//   - Isolate tenant data and operations
//   - Support dynamic tenant registration and management
//
// Example usage:
//   app := modular.NewStdApplication(configProvider, logger)
//   // Register tenant service and tenant-aware modules
//   tenantCtx, err := app.WithTenant("tenant-123")
//   if err != nil {
//       return err
//   }
//   // Use tenant context for tenant-specific operations
type TenantApplication interface {
	Application

	// GetTenantService returns the application's tenant service if available.
	// The tenant service manages tenant registration, lookup, and lifecycle.
	// Returns an error if no tenant service has been registered.
	//
	// Example:
	//   tenantSvc, err := app.GetTenantService()
	//   if err != nil {
	//       return fmt.Errorf("multi-tenancy not configured: %w", err)
	//   }
	GetTenantService() (TenantService, error)

	// WithTenant creates a tenant context from the application context.
	// Tenant contexts provide scoped access to tenant-specific configurations
	// and services, enabling isolation between different tenants.
	//
	// The returned context can be used for tenant-specific operations
	// and will carry tenant identification through the call chain.
	//
	// Example:
	//   tenantCtx, err := app.WithTenant("customer-456")
	//   if err != nil {
	//       return err
	//   }
	//   // Use tenantCtx for tenant-specific operations
	WithTenant(tenantID TenantID) (*TenantContext, error)

	// GetTenantConfig retrieves configuration for a specific tenant and section.
	// This allows modules to access tenant-specific configuration that may
	// override or extend the default application configuration.
	//
	// The section parameter specifies which configuration section to retrieve
	// (e.g., "database", "cache", etc.), and the framework will return the
	// tenant-specific version if available, falling back to defaults otherwise.
	//
	// Example:
	//   cfg, err := app.GetTenantConfig("tenant-789", "database")
	//   if err != nil {
	//       return err
	//   }
	//   dbConfig := cfg.GetConfig().(*DatabaseConfig)
	GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error)
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

// NewStdApplication creates a new application instance with the provided configuration and logger.
// This is the standard way to create a modular application.
//
// Parameters:
//   - cp: ConfigProvider for application-level configuration
//   - logger: Logger implementation for framework and module logging
//
// The created application will have empty registries that can be populated by
// registering modules and services. The application must be initialized with
// Init() before it can be started.
//
// Example:
//   // Create configuration
//   appConfig := &MyAppConfig{}
//   configProvider := modular.NewStdConfigProvider(appConfig)
//   
//   // Create logger (implement modular.Logger interface)
//   logger := &MyLogger{}
//   
//   // Create application
//   app := modular.NewStdApplication(configProvider, logger)
//   
//   // Register modules
//   app.RegisterModule(&DatabaseModule{})
//   app.RegisterModule(&WebServerModule{})
//   
//   // Run application
//   if err := app.Run(); err != nil {
//       log.Fatal(err)
//   }
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
		return nil, fmt.Errorf("%w: %s", ErrConfigSectionNotFound, section)
	}
	return cp, nil
}

// RegisterService adds a service with type checking
func (app *StdApplication) RegisterService(name string, service any) error {
	if _, exists := app.svcRegistry[name]; exists {
		return fmt.Errorf("%w: %s", ErrServiceAlreadyRegistered, name)
	}

	app.svcRegistry[name] = service
	app.logger.Debug("Registered service", "name", name, "type", reflect.TypeOf(service))
	return nil
}

// GetService retrieves a service with type assertion
func (app *StdApplication) GetService(name string, target any) error {
	service, exists := app.svcRegistry[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr || targetValue.IsNil() {
		return ErrTargetNotPointer
	}

	if !targetValue.Elem().IsValid() {
		return ErrTargetValueInvalid
	}

	serviceType := reflect.TypeOf(service)
	targetType := targetValue.Elem().Type()

	// Case 1: Target is an interface that the service implements
	if targetType.Kind() == reflect.Interface && serviceType.Implements(targetType) {
		targetValue.Elem().Set(reflect.ValueOf(service))
		return nil
	}

	// Case 2: Target is a struct with embedded interfaces
	if targetType.Kind() == reflect.Struct {
		for i := 0; i < targetType.NumField(); i++ {
			field := targetType.Field(i)
			if field.Type.Kind() == reflect.Interface && serviceType.Implements(field.Type) {
				fieldValue := targetValue.Elem().Field(i)
				if fieldValue.CanSet() {
					fieldValue.Set(reflect.ValueOf(service))
					return nil
				}
			}
		}
	}

	// Case 3: Direct assignment or pointer dereference
	if serviceType.AssignableTo(targetType) {
		targetValue.Elem().Set(reflect.ValueOf(service))
		return nil
	} else if serviceType.Kind() == reflect.Ptr && serviceType.Elem().AssignableTo(targetType) {
		targetValue.Elem().Set(reflect.ValueOf(service).Elem())
		return nil
	}

	return fmt.Errorf("%w: service '%s' of type %s cannot be assigned to %s",
		ErrServiceIncompatible, name, serviceType, targetType)
}

// Init initializes the application with the provided modules
func (app *StdApplication) Init() error {
	errs := make([]error, 0)
	for name, module := range app.moduleRegistry {
		configurableModule, ok := module.(Configurable)
		if !ok {
			app.logger.Debug("Module does not implement Configurable, skipping", "module", name)
			continue
		}
		err := configurableModule.RegisterConfig(app)
		if err != nil {
			errs = append(errs, fmt.Errorf("module %s failed to register config: %w", name, err))
			continue
		}
		app.logger.Debug("Registering module", "name", name)
	}

	if err := AppConfigLoader(app); err != nil {
		errs = append(errs, fmt.Errorf("failed to load app config: %w", err))
	}

	// Build dependency graph
	moduleOrder, err := app.resolveDependencies()
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to resolve module dependencies: %w", err))
	}

	// Initialize modules in order
	for _, moduleName := range moduleOrder {
		if _, ok := app.moduleRegistry[moduleName].(ServiceAware); ok {
			// Inject required services
			app.moduleRegistry[moduleName], err = app.injectServices(app.moduleRegistry[moduleName])
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to inject services for module '%s': %w", moduleName, err))
				continue
			}
		}

		if err = app.moduleRegistry[moduleName].Init(app); err != nil {
			errs = append(errs, fmt.Errorf("module '%s' failed to initialize: %w", moduleName, err))
			continue
		}

		if _, ok := app.moduleRegistry[moduleName].(ServiceAware); ok {
			// Register services provided by modules
			for _, svc := range app.moduleRegistry[moduleName].(ServiceAware).ProvidesServices() {
				if err = app.RegisterService(svc.Name, svc.Instance); err != nil {
					errs = append(errs, fmt.Errorf("module '%s' failed to register service '%s': %w", svc.Name, moduleName, err))
					continue
				}
			}
		}

		app.logger.Info(fmt.Sprintf("Initialized module %s of type %T", moduleName, app.moduleRegistry[moduleName]))
	}

	// Initialize tenant configuration after modules have registered their configurations
	if err = app.initTenantConfigurations(); err != nil {
		errs = append(errs, fmt.Errorf("failed to initialize tenant configurations: %w", err))
	}

	return errors.Join(errs...)
}

// initTenantConfigurations initializes tenant configurations after modules have registered their configs
func (app *StdApplication) initTenantConfigurations() error {
	var tenantSvc TenantService
	if err := app.GetService("tenantService", &tenantSvc); err == nil {
		app.tenantService = tenantSvc

		// If there's a TenantConfigLoader service, use it to load tenant configs
		var loader TenantConfigLoader
		if err = app.GetService("tenantConfigLoader", &loader); err == nil {
			app.logger.Debug("Loading tenant configurations using TenantConfigLoader")
			if err = loader.LoadTenantConfigurations(app, tenantSvc); err != nil {
				return fmt.Errorf("failed to load tenant configurations: %w", err)
			}
		}

		// Register tenant-aware modules with the tenant service
		for _, module := range app.moduleRegistry {
			if tenantAwareModule, ok := module.(TenantAwareModule); ok {
				if err := tenantSvc.RegisterTenantAwareModule(tenantAwareModule); err != nil {
					app.logger.Warn("Failed to register tenant-aware module", "module", module.Name(), "error", err)
				}
			}
		}
	} else {
		app.logger.Debug("Tenant service not found, skipping tenant configuration initialization")
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
		startableModule, ok := module.(Startable)
		if !ok {
			app.logger.Debug("Module does not implement Startable, skipping", "module", name)
			continue
		}
		app.logger.Info("Starting module", "module", name)
		if err := startableModule.Start(ctx); err != nil {
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
		stoppableModule, ok := module.(Stoppable)
		if !ok {
			app.logger.Debug("Module does not implement Stoppable, skipping", "module", name)
			continue
		}
		app.logger.Info("Stopping module", "module", name)
		if err = stoppableModule.Stop(ctx); err != nil {
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
	serviceAwModule := module.(ServiceAware)
	dependencies := serviceAwModule.RequiresServices()

	requiredServices, err := app.resolveServiceDependencies(dependencies, module.Name())
	if err != nil {
		return nil, err
	}

	app.logger.Debug("Injecting dependencies", "dependencies", dependencies, "module", module.Name())

	// If module supports constructor injection, use it
	if withConstructor, ok := module.(Constructable); ok {
		return app.constructModuleWithServices(withConstructor, requiredServices)
	}

	return module, nil
}

// resolveServiceDependencies resolves all service dependencies for a module
func (app *StdApplication) resolveServiceDependencies(
	dependencies []ServiceDependency,
	moduleName string,
) (map[string]any, error) {
	requiredServices := make(map[string]any)

	// First, handle all name-based dependencies
	if err := app.resolveNameBasedDependencies(dependencies, requiredServices, moduleName); err != nil {
		return nil, err
	}

	// Then, handle all interface-based dependencies
	if err := app.resolveInterfaceBasedDependencies(dependencies, requiredServices, moduleName); err != nil {
		return nil, err
	}

	return requiredServices, nil
}

// resolveNameBasedDependencies resolves dependencies by name
func (app *StdApplication) resolveNameBasedDependencies(
	dependencies []ServiceDependency,
	requiredServices map[string]any,
	moduleName string,
) error {
	for _, dep := range dependencies {
		if dep.MatchByInterface {
			continue // Skip interface-based dependencies
		}

		service, serviceFound := app.svcRegistry[dep.Name]
		if serviceFound {
			if valid, err := checkServiceCompatibility(service, dep); !valid {
				return fmt.Errorf("failed to inject service '%s': %w", dep.Name, err)
			}
			requiredServices[dep.Name] = service
		} else if dep.Required {
			return fmt.Errorf("%w: %s for %s", ErrRequiredServiceNotFound, dep.Name, moduleName)
		}
	}
	return nil
}

// resolveInterfaceBasedDependencies resolves dependencies by interface
func (app *StdApplication) resolveInterfaceBasedDependencies(
	dependencies []ServiceDependency,
	requiredServices map[string]any,
	moduleName string,
) error {
	for _, dep := range dependencies {
		// Skip if not interface-based or already satisfied
		if !dep.MatchByInterface || requiredServices[dep.Name] != nil {
			continue
		}

		// Check for invalid interface configuration
		if dep.SatisfiesInterface == nil {
			if dep.Required {
				return fmt.Errorf("%w in module '%s': %w (hint: use reflect.TypeOf((*InterfaceName)(nil)).Elem())", ErrInvalidInterfaceConfiguration, moduleName, ErrInterfaceConfigurationNil)
			}
			continue // Skip optional services with invalid interface config
		}

		if dep.SatisfiesInterface.Kind() != reflect.Interface {
			if dep.Required {
				return fmt.Errorf("%w in module '%s': %w", ErrInvalidInterfaceConfiguration, moduleName, ErrInterfaceConfigurationNotInterface)
			}
			continue // Skip optional services with invalid interface config
		}

		matchedService, matchedServiceName := app.findServiceByInterface(dep)

		if matchedService != nil {
			if valid, err := checkServiceCompatibility(matchedService, dep); !valid {
				return fmt.Errorf("failed to inject service '%s': %w", matchedServiceName, err)
			}
			requiredServices[dep.Name] = matchedService
		} else if dep.Required {
			return fmt.Errorf("%w: no service found implementing interface %v for %s",
				ErrRequiredServiceNotFound, dep.SatisfiesInterface, moduleName)
		}
	}
	return nil
}

// findServiceByInterface finds a service that implements the specified interface
func (app *StdApplication) findServiceByInterface(dep ServiceDependency) (service any, serviceName string) {
	for serviceName, service := range app.svcRegistry {
		serviceType := reflect.TypeOf(service)
		if serviceType.Implements(dep.SatisfiesInterface) {
			return service, serviceName
		}
	}
	return nil, ""
}

// constructModuleWithServices constructs a module using constructor injection
func (app *StdApplication) constructModuleWithServices(
	withConstructor Constructable,
	requiredServices map[string]any,
) (Module, error) {
	constructor := withConstructor.Constructor()

	if err := app.validateConstructor(constructor); err != nil {
		return nil, err
	}

	args, err := app.prepareConstructorArguments(constructor, requiredServices)
	if err != nil {
		return nil, err
	}

	return app.callConstructor(constructor, args)
}

// validateConstructor validates that the constructor is a proper function
func (app *StdApplication) validateConstructor(constructor any) error {
	constructorType := reflect.TypeOf(constructor)
	if constructorType.Kind() != reflect.Func {
		return ErrConstructorNotFunction
	}
	return nil
}

// prepareConstructorArguments prepares arguments for constructor call
func (app *StdApplication) prepareConstructorArguments(
	constructor any,
	requiredServices map[string]any,
) ([]reflect.Value, error) {
	constructorType := reflect.TypeOf(constructor)
	args := make([]reflect.Value, constructorType.NumIn())

	for i := 0; i < constructorType.NumIn(); i++ {
		paramType := constructorType.In(i)
		arg, err := app.resolveConstructorParameter(paramType, requiredServices, i)
		if err != nil {
			return nil, err
		}
		args[i] = arg
	}

	return args, nil
}

// resolveConstructorParameter resolves a single constructor parameter
func (app *StdApplication) resolveConstructorParameter(
	paramType reflect.Type,
	requiredServices map[string]any,
	paramIndex int,
) (reflect.Value, error) {
	// Check if this parameter expects the Application
	if app.isApplicationParameter(paramType) {
		return reflect.ValueOf(Application(app)), nil
	}

	// Handle services map parameter
	if app.isServicesMapParameter(paramType) {
		return reflect.ValueOf(requiredServices), nil
	}

	// Find matching service by type from the services map
	matchedService := app.findServiceByType(paramType, requiredServices)
	if matchedService == nil {
		return reflect.Value{}, fmt.Errorf("%w %d of type %v", ErrConstructorParameterServiceNotFound, paramIndex, paramType)
	}

	return reflect.ValueOf(matchedService), nil
}

// isApplicationParameter checks if parameter expects the Application interface
func (app *StdApplication) isApplicationParameter(paramType reflect.Type) bool {
	return paramType.Kind() == reflect.Interface && paramType.String() == "modular.Application"
}

// isServicesMapParameter checks if parameter is a services map
func (app *StdApplication) isServicesMapParameter(paramType reflect.Type) bool {
	return paramType.Kind() == reflect.Map &&
		paramType.Key().Kind() == reflect.String &&
		(paramType.Elem().Kind() == reflect.Interface || paramType.Elem().String() == "interface {}")
}

// findServiceByType finds a service that matches the parameter type
func (app *StdApplication) findServiceByType(paramType reflect.Type, requiredServices map[string]any) any {
	for _, service := range requiredServices {
		serviceType := reflect.TypeOf(service)
		if serviceType.AssignableTo(paramType) ||
			(paramType.Kind() == reflect.Interface && serviceType.Implements(paramType)) {
			return service
		}
	}
	return nil
}

// callConstructor calls the constructor with prepared arguments
func (app *StdApplication) callConstructor(constructor any, args []reflect.Value) (Module, error) {
	results := reflect.ValueOf(constructor).Call(args)
	if len(results) != 2 {
		return nil, ErrConstructorInvalidReturnCount
	}

	// Check for error
	if !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}

	newModule, ok := results[0].Interface().(Module)
	if !ok {
		return nil, ErrConstructorInvalidReturnType
	}

	return newModule, nil
}

// checkServiceCompatibility verifies if a service is compatible with a dependency
func checkServiceCompatibility(service any, dep ServiceDependency) (bool, error) {
	serviceType := reflect.TypeOf(service)

	// Check interface compatibility if specified
	if dep.SatisfiesInterface != nil {
		if dep.SatisfiesInterface.Kind() == reflect.Interface {
			if !serviceType.Implements(dep.SatisfiesInterface) {
				return false, fmt.Errorf("%w: %v", ErrServiceInterfaceIncompatible, dep.SatisfiesInterface)
			}
		}
	}

	return true, nil
}

// Logger represents a logger
func (app *StdApplication) Logger() Logger {
	return app.logger
}

// SetLogger sets the application's logger
func (app *StdApplication) SetLogger(logger Logger) {
	app.logger = logger
}

// resolveDependencies returns modules in initialization order
func (app *StdApplication) resolveDependencies() ([]string, error) {
	// Create dependency graph
	graph := make(map[string][]string)
	for name, module := range app.moduleRegistry {
		if _, ok := module.(DependencyAware); !ok {
			app.logger.Debug("Module does not implement DependencyAware, skipping", "module", name)
			graph[name] = nil
			continue
		}
		graph[name] = module.(DependencyAware).Dependencies()
	}

	// Analyze service dependencies to augment the graph with implicit dependencies
	app.addImplicitDependencies(graph)

	// Topological sort
	var result []string
	visited := make(map[string]bool)
	temp := make(map[string]bool)

	var visit func(string) error
	visit = func(node string) error {
		if temp[node] {
			return fmt.Errorf("%w: %s", ErrCircularDependency, node)
		}
		if visited[node] {
			return nil
		}
		temp[node] = true

		for _, dep := range graph[node] {
			if _, exists := app.moduleRegistry[dep]; !exists {
				return fmt.Errorf("%w: %s depends on non-existent module %s",
					ErrModuleDependencyMissing, node, dep)
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
	app.logger.Debug("Module initialization order", "order", result)

	return result, nil
}

// addImplicitDependencies analyzes service provider/consumer relationships to find implicit dependencies
// where modules provide services that other modules require via interface matching.
func (app *StdApplication) addImplicitDependencies(graph map[string][]string) {
	// Collect all required interfaces and service providers
	requiredInterfaces, serviceProviders := app.collectServiceRequirements()

	// Find interface implementations
	interfaceImplementations := app.findInterfaceImplementations(requiredInterfaces)

	// Add dependencies to the graph
	app.addNameBasedDependencies(graph, serviceProviders)
	app.addInterfaceBasedDependencies(graph, interfaceImplementations)
}

// collectServiceRequirements builds maps of required interfaces and service providers
func (app *StdApplication) collectServiceRequirements() (
	requiredInterfaces map[string][]interfaceRequirement,
	serviceProviders map[string]string,
) {
	requiredInterfaces = make(map[string][]interfaceRequirement)
	serviceProviders = make(map[string]string)

	for moduleName, module := range app.moduleRegistry {
		svcAwareModule, ok := module.(ServiceAware)
		if !ok {
			continue
		}

		// Collect required interfaces
		app.collectRequiredInterfaces(moduleName, svcAwareModule, requiredInterfaces)

		// Collect service providers
		app.collectServiceProviders(moduleName, svcAwareModule, serviceProviders)
	}

	return requiredInterfaces, serviceProviders
}

// interfaceRequirement represents a module's requirement for a specific interface
type interfaceRequirement struct {
	interfaceType reflect.Type
	moduleName    string
	serviceName   string
}

// collectRequiredInterfaces collects all interface-based service requirements for a module
func (app *StdApplication) collectRequiredInterfaces(
	moduleName string,
	svcAwareModule ServiceAware,
	requiredInterfaces map[string][]interfaceRequirement,
) {
	for _, svcDep := range svcAwareModule.RequiresServices() {
		if !app.isInterfaceBasedDependency(svcDep) {
			continue
		}

		records := requiredInterfaces[svcDep.Name]
		records = append(records, interfaceRequirement{
			interfaceType: svcDep.SatisfiesInterface,
			moduleName:    moduleName,
			serviceName:   svcDep.Name,
		})
		requiredInterfaces[svcDep.Name] = records

		app.logger.Debug("Registered required interface",
			"module", moduleName,
			"service", svcDep.Name,
			"interface", svcDep.SatisfiesInterface.String())
	}
}

// isInterfaceBasedDependency checks if a service dependency is interface-based
func (app *StdApplication) isInterfaceBasedDependency(svcDep ServiceDependency) bool {
	return svcDep.MatchByInterface &&
		svcDep.SatisfiesInterface != nil &&
		svcDep.SatisfiesInterface.Kind() == reflect.Interface
}

// collectServiceProviders registers services provided by a module
func (app *StdApplication) collectServiceProviders(
	moduleName string,
	svcAwareModule ServiceAware,
	serviceProviders map[string]string,
) {
	for _, svcProvider := range svcAwareModule.ProvidesServices() {
		if svcProvider.Name != "" && svcProvider.Instance != nil {
			serviceProviders[svcProvider.Name] = moduleName
		}
	}
}

// findInterfaceImplementations finds which modules provide services that implement required interfaces
func (app *StdApplication) findInterfaceImplementations(
	requiredInterfaces map[string][]interfaceRequirement,
) map[string][]string {
	interfaceImplementations := make(map[string][]string)

	for moduleName, module := range app.moduleRegistry {
		svcAwareModule, ok := module.(ServiceAware)
		if !ok {
			continue
		}

		app.checkModuleServiceImplementations(moduleName, svcAwareModule, requiredInterfaces, interfaceImplementations)
	}

	return interfaceImplementations
}

// checkModuleServiceImplementations checks if a module's services implement any required interfaces
func (app *StdApplication) checkModuleServiceImplementations(
	moduleName string,
	svcAwareModule ServiceAware,
	requiredInterfaces map[string][]interfaceRequirement,
	interfaceImplementations map[string][]string,
) {
	for _, svcProvider := range svcAwareModule.ProvidesServices() {
		if svcProvider.Instance == nil {
			continue
		}

		svcType := reflect.TypeOf(svcProvider.Instance)
		app.matchServiceToInterfaces(moduleName, svcProvider, svcType, requiredInterfaces, interfaceImplementations)
	}
}

// matchServiceToInterfaces checks if a service implements any required interfaces
func (app *StdApplication) matchServiceToInterfaces(
	providerModule string,
	svcProvider ServiceProvider,
	svcType reflect.Type,
	requiredInterfaces map[string][]interfaceRequirement,
	interfaceImplementations map[string][]string,
) {
	for reqServiceName, interfaceRecords := range requiredInterfaces {
		for _, record := range interfaceRecords {
			if app.serviceImplementsInterface(
				providerModule, record, svcType, svcProvider, reqServiceName, interfaceImplementations,
			) {
				break // Found a match, no need to check other records for this service
			}
		}
	}
}

// serviceImplementsInterface checks if a service implements a specific interface requirement
func (app *StdApplication) serviceImplementsInterface(
	providerModule string,
	record interfaceRequirement,
	svcType reflect.Type,
	svcProvider ServiceProvider,
	reqServiceName string,
	interfaceImplementations map[string][]string,
) bool {
	// Skip if it's the same module
	if record.moduleName == providerModule {
		return false
	}

	// Check if the provided service implements the required interface
	if !app.typeImplementsInterface(svcType, record.interfaceType) {
		return false
	}

	// This module provides a service that another module requires
	consumerModule := record.moduleName

	// Add dependency from consumer to provider
	if _, exists := interfaceImplementations[consumerModule]; !exists {
		interfaceImplementations[consumerModule] = make([]string, 0)
	}

	// Only add if not already in the list
	if !slices.Contains(interfaceImplementations[consumerModule], providerModule) {
		interfaceImplementations[consumerModule] = append(
			interfaceImplementations[consumerModule], providerModule)

		app.logger.Debug("Found interface implementation match",
			"provider", providerModule,
			"provider_service", svcProvider.Name,
			"consumer", consumerModule,
			"required_service", reqServiceName,
			"interface", record.interfaceType.String())
	}

	return true
}

// typeImplementsInterface checks if a type implements an interface
func (app *StdApplication) typeImplementsInterface(svcType, interfaceType reflect.Type) bool {
	return svcType.Implements(interfaceType) ||
		(svcType.Kind() == reflect.Ptr && svcType.Elem().Implements(interfaceType))
}

// addNameBasedDependencies adds dependencies based on direct service name matching
func (app *StdApplication) addNameBasedDependencies(graph map[string][]string, serviceProviders map[string]string) {
	for consumerName, module := range app.moduleRegistry {
		svcAwareModule, ok := module.(ServiceAware)
		if !ok {
			continue
		}

		app.addModuleNameBasedDependencies(consumerName, svcAwareModule, graph, serviceProviders)
	}
}

// addModuleNameBasedDependencies adds name-based dependencies for a specific module
func (app *StdApplication) addModuleNameBasedDependencies(
	consumerName string,
	svcAwareModule ServiceAware,
	graph map[string][]string,
	serviceProviders map[string]string,
) {
	for _, svcDep := range svcAwareModule.RequiresServices() {
		if !svcDep.Required || svcDep.MatchByInterface {
			continue // Skip optional or interface-based dependencies
		}

		app.addNameBasedDependency(consumerName, svcDep, graph, serviceProviders)
	}
}

// addNameBasedDependency adds a single name-based dependency
func (app *StdApplication) addNameBasedDependency(
	consumerName string,
	svcDep ServiceDependency,
	graph map[string][]string,
	serviceProviders map[string]string,
) {
	providerModule, exists := serviceProviders[svcDep.Name]
	if !exists || providerModule == consumerName {
		return
	}

	// Check if dependency already exists
	for _, existingDep := range graph[consumerName] {
		if existingDep == providerModule {
			return // Already exists
		}
	}

	// Add the dependency
	if graph[consumerName] == nil {
		graph[consumerName] = make([]string, 0)
	}
	graph[consumerName] = append(graph[consumerName], providerModule)

	app.logger.Debug("Added name-based dependency",
		"consumer", consumerName,
		"provider", providerModule,
		"service", svcDep.Name)
}

// addInterfaceBasedDependencies adds dependencies based on interface implementations
func (app *StdApplication) addInterfaceBasedDependencies(graph, interfaceImplementations map[string][]string) {
	for consumer, providers := range interfaceImplementations {
		for _, provider := range providers {
			app.addInterfaceBasedDependency(consumer, provider, graph)
		}
	}
}

// addInterfaceBasedDependency adds a single interface-based dependency
func (app *StdApplication) addInterfaceBasedDependency(consumer, provider string, graph map[string][]string) {
	// Skip self-dependencies
	if consumer == provider {
		return
	}

	// Check if this dependency already exists
	for _, existingDep := range graph[consumer] {
		if existingDep == provider {
			return
		}
	}

	// Add the dependency
	if graph[consumer] == nil {
		graph[consumer] = make([]string, 0)
	}
	graph[consumer] = append(graph[consumer], provider)

	app.logger.Debug("Added interface-based dependency",
		"consumer", consumer,
		"provider", provider)
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
		return nil, ErrAppContextNotInitialized
	}
	return NewTenantContext(app.ctx, tenantID), nil
}

// GetTenantConfig retrieves configuration for a specific tenant and section
func (app *StdApplication) GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error) {
	ts, err := app.GetTenantService()
	if err != nil {
		return nil, err
	}
	provider, err := ts.GetTenantConfig(tenantID, section)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant config: %w", err)
	}
	return provider, nil
}
