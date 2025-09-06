package modular

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"slices"
	"strings"
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
//
//	app := modular.NewStdApplication(configProvider, logger)

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

	// SetVerboseConfig enables or disables verbose configuration debugging.
	// When enabled, DEBUG level logging will be performed during configuration
	// processing to show which config is being processed, which key is being
	// evaluated, and which attribute or key is being searched for.
	SetVerboseConfig(enabled bool)

	// IsVerboseConfig returns whether verbose configuration debugging is enabled.
	IsVerboseConfig() bool

	// ServiceIntrospector groups advanced service registry introspection helpers.
	// Use this instead of adding new methods directly to Application.
	ServiceIntrospector() ServiceIntrospector
}

// ServiceIntrospector provides advanced service registry introspection helpers.
// This extension interface allows future additions without expanding Application.
type ServiceIntrospector interface {
	GetServicesByModule(moduleName string) []string
	GetServiceEntry(serviceName string) (*ServiceRegistryEntry, bool)
	GetServicesByInterface(interfaceType reflect.Type) []*ServiceRegistryEntry
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
//
//	app := modular.NewStdApplication(configProvider, logger)
//	// Register tenant service and tenant-aware modules
//	tenantCtx, err := app.WithTenant("tenant-123")
//	if err != nil {
//	    return err
//	}
//	// Use tenant context for tenant-specific operations
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
	cfgProvider         ConfigProvider
	cfgSections         map[string]ConfigProvider
	svcRegistry         ServiceRegistry          // Backwards compatible view
	enhancedSvcRegistry *EnhancedServiceRegistry // Enhanced registry with module tracking
	moduleRegistry      ModuleRegistry
	logger              Logger
	ctx                 context.Context
	cancel              context.CancelFunc
	tenantService       TenantService // Added tenant service reference
	verboseConfig       bool          // Flag for verbose configuration debugging
	initialized         bool          // Tracks whether Init has already been successfully executed
	configFeeders       []Feeder      // Optional per-application feeders (override global ConfigFeeders if non-nil)
}

// ServiceIntrospectorImpl implements ServiceIntrospector backed by StdApplication's enhanced registry.
type ServiceIntrospectorImpl struct {
	app *StdApplication
}

func (s *ServiceIntrospectorImpl) GetServicesByModule(moduleName string) []string {
	return s.app.enhancedSvcRegistry.GetServicesByModule(moduleName)
}

func (s *ServiceIntrospectorImpl) GetServiceEntry(serviceName string) (*ServiceRegistryEntry, bool) {
	return s.app.enhancedSvcRegistry.GetServiceEntry(serviceName)
}

func (s *ServiceIntrospectorImpl) GetServicesByInterface(interfaceType reflect.Type) []*ServiceRegistryEntry {
	return s.app.enhancedSvcRegistry.GetServicesByInterface(interfaceType)
}

// ServiceIntrospector returns an implementation of ServiceIntrospector.
func (app *StdApplication) ServiceIntrospector() ServiceIntrospector {
	return &ServiceIntrospectorImpl{app: app}
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
//
//	// Create configuration
//	appConfig := &MyAppConfig{}
//	configProvider := modular.NewStdConfigProvider(appConfig)
//
//	// Create logger (implement modular.Logger interface)
//	logger := &MyLogger{}
//
//	// Create application
//	app := modular.NewStdApplication(configProvider, logger)
//
//	// Register modules
//	app.RegisterModule(&DatabaseModule{})
//	app.RegisterModule(&WebServerModule{})
//
//	// Run application
//	if err := app.Run(); err != nil {
//	    log.Fatal(err)
//	}
func NewStdApplication(cp ConfigProvider, logger Logger) Application {
	enhancedRegistry := NewEnhancedServiceRegistry()

	app := &StdApplication{
		cfgProvider:         cp,
		cfgSections:         make(map[string]ConfigProvider),
		enhancedSvcRegistry: enhancedRegistry,
		svcRegistry:         enhancedRegistry.AsServiceRegistry(), // Backwards compatible view
		moduleRegistry:      make(ModuleRegistry),
		logger:              logger,
		configFeeders:       nil, // default to nil to signal use of package-level ConfigFeeders
	}

	// Register the logger as a service so modules can depend on it
	if app.enhancedSvcRegistry != nil {
		_, _ = app.enhancedSvcRegistry.RegisterService("logger", logger) // Ignore error for logger service
		app.svcRegistry = app.enhancedSvcRegistry.AsServiceRegistry()    // Update backwards compatible view
	}

	return app
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

// SetConfigFeeders sets per-application configuration feeders overriding the package-level ConfigFeeders for this app's Init lifecycle.
// Passing nil resets to use the global ConfigFeeders again.
func (app *StdApplication) SetConfigFeeders(feeders []Feeder) {
	app.configFeeders = feeders
}

// RegisterService adds a service with type checking
func (app *StdApplication) RegisterService(name string, service any) error {
	var actualName string

	// Register with enhanced registry if available (handles automatic conflict resolution)
	if app.enhancedSvcRegistry != nil {
		var err error
		actualName, err = app.enhancedSvcRegistry.RegisterService(name, service)
		if err != nil {
			return err
		}

		// Update backwards compatible view
		app.svcRegistry = app.enhancedSvcRegistry.AsServiceRegistry()
	} else {
		// Check for duplicates using the backwards compatible registry
		if _, exists := app.svcRegistry[name]; exists {
			// Preserve contract: duplicate registrations are an error
			if app.logger != nil {
				app.logger.Debug("Service already registered", "name", name)
			}
			return ErrServiceAlreadyRegistered
		}

		// Fallback to direct registration for compatibility
		app.svcRegistry[name] = service
		actualName = name
	}

	serviceType := reflect.TypeOf(service)
	var typeName string
	if serviceType != nil {
		typeName = serviceType.String()
	} else {
		typeName = "<nil>"
	}
	if app.logger != nil {
		app.logger.Debug("Registered service", "name", name, "actualName", actualName, "type", typeName)
	}
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
	return app.InitWithApp(app)
}

// InitWithApp initializes the application with the provided modules, using appToPass as the application instance passed to modules
func (app *StdApplication) InitWithApp(appToPass Application) error {
	// Make Init idempotent: if already initialized, skip re-initialization to avoid
	// duplicate service registrations and other side effects. This supports tests
	// and scenarios that may call Init more than once.
	if app.initialized {
		if app.logger != nil {
			app.logger.Debug("Application already initialized, skipping Init")
		}
		return nil
	}

	errs := make([]error, 0)
	for name, module := range app.moduleRegistry {
		configurableModule, ok := module.(Configurable)
		if !ok {
			if app.logger != nil {
				app.logger.Debug("Module does not implement Configurable, skipping", "module", name)
			}
			continue
		}
		err := configurableModule.RegisterConfig(appToPass)
		if err != nil {
			errs = append(errs, fmt.Errorf("module %s failed to register config: %w", name, err))
			continue
		}
		if app.logger != nil {
			app.logger.Debug("Registering module", "name", name)
		}
	}

	// Configuration loading (AppConfigLoader will consult app.configFeeders directly now)
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
		module := app.moduleRegistry[moduleName]

		if _, ok := module.(ServiceAware); ok {
			// Inject required services
			app.moduleRegistry[moduleName], err = app.injectServices(module)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to inject services for module '%s': %w", moduleName, err))
				continue
			}
			module = app.moduleRegistry[moduleName] // Update reference after injection
		}

		// Set current module context for service registration tracking
		if app.enhancedSvcRegistry != nil {
			app.enhancedSvcRegistry.SetCurrentModule(module)
		}

		if err = module.Init(appToPass); err != nil {
			errs = append(errs, fmt.Errorf("module '%s' failed to initialize: %w", moduleName, err))
			continue
		}

		if _, ok := module.(ServiceAware); ok {
			// Register services provided by modules
			for _, svc := range module.(ServiceAware).ProvidesServices() {
				if err = app.RegisterService(svc.Name, svc.Instance); err != nil {
					// Collect registration errors (e.g., duplicates) for reporting
					errs = append(errs, fmt.Errorf("module '%s' failed to register service '%s': %w", moduleName, svc.Name, err))
					continue
				}
			}
		}

		// Clear current module context
		if app.enhancedSvcRegistry != nil {
			app.enhancedSvcRegistry.ClearCurrentModule()
		}

		app.logger.Info(fmt.Sprintf("Initialized module %s of type %T", moduleName, app.moduleRegistry[moduleName]))
	}

	// Initialize tenant configuration after modules have registered their configurations
	if err = app.initTenantConfigurations(); err != nil {
		errs = append(errs, fmt.Errorf("failed to initialize tenant configurations: %w", err))
	}

	// Mark as initialized only after completing Init flow
	if len(errs) == 0 {
		app.initialized = true
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
		if app.typeImplementsInterface(serviceType, dep.SatisfiesInterface) {
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
	// Also update the service registry so modules get the new logger via DI
	app.svcRegistry["logger"] = logger
}

// SetVerboseConfig enables or disables verbose configuration debugging
func (app *StdApplication) SetVerboseConfig(enabled bool) {
	app.verboseConfig = enabled
	if enabled {
		app.logger.Debug("Verbose configuration debugging enabled")
	} else {
		app.logger.Debug("Verbose configuration debugging disabled")
	}
}

// IsVerboseConfig returns whether verbose configuration debugging is enabled
func (app *StdApplication) IsVerboseConfig() bool {
	return app.verboseConfig
}

// DependencyEdge represents a dependency edge with its source type
type DependencyEdge struct {
	From string
	To   string
	Type EdgeType
	// For interface-based dependencies, show which interface is involved
	InterfaceType reflect.Type
	ServiceName   string
}

// EdgeType represents the type of dependency edge
type EdgeType int

const (
	EdgeTypeModule EdgeType = iota
	EdgeTypeNamedService
	EdgeTypeInterfaceService
)

func (e EdgeType) String() string {
	switch e {
	case EdgeTypeModule:
		return "module"
	case EdgeTypeNamedService:
		return "named-service"
	case EdgeTypeInterfaceService:
		return "interface-service"
	default:
		return "unknown"
	}
}

// resolveDependencies returns modules in initialization order
func (app *StdApplication) resolveDependencies() ([]string, error) {
	// Create dependency graph and track dependency edges
	graph := make(map[string][]string)
	dependencyEdges := make([]DependencyEdge, 0)

	for name, module := range app.moduleRegistry {
		if _, ok := module.(DependencyAware); !ok {
			app.logger.Debug("Module does not implement DependencyAware, skipping", "module", name)
			graph[name] = nil
			continue
		}
		deps := module.(DependencyAware).Dependencies()
		graph[name] = deps

		// Track module-level dependency edges
		for _, dep := range deps {
			dependencyEdges = append(dependencyEdges, DependencyEdge{
				From: name,
				To:   dep,
				Type: EdgeTypeModule,
			})
		}
	}

	// Analyze service dependencies to augment the graph with implicit dependencies
	serviceEdges := app.addImplicitDependencies(graph)
	dependencyEdges = append(dependencyEdges, serviceEdges...)

	// Filter out artificial self interface-service edges which do not represent real
	// initialization ordering constraints but can appear when a module both provides
	// and (optionally) consumes an interface-based service it implements.
	pruned := dependencyEdges[:0]
	for _, e := range dependencyEdges {
		if e.Type == EdgeTypeInterfaceService && e.From == e.To {
			app.logger.Debug("Pruning self interface dependency edge", "module", e.From, "interface", e.InterfaceType)
			// Also remove from graph adjacency list if present
			adj := graph[e.From]
			if len(adj) > 0 {
				filtered := adj[:0]
				for _, dep := range adj {
					if dep != e.To {
						filtered = append(filtered, dep)
					}
				}
				graph[e.From] = filtered
			}
			continue
		}
		pruned = append(pruned, e)
	}
	dependencyEdges = pruned

	// Enhanced topological sort with path tracking
	var result []string
	visited := make(map[string]bool)
	temp := make(map[string]bool)
	path := make([]string, 0)

	var visit func(string) error
	visit = func(node string) error {
		if temp[node] {
			// Found cycle - construct detailed cycle information
			cycle := app.constructCyclePath(path, node, dependencyEdges)
			return fmt.Errorf("%w: %s", ErrCircularDependency, cycle)
		}
		if visited[node] {
			return nil
		}
		temp[node] = true
		path = append(path, node)

		// Sort dependencies to ensure deterministic order
		deps := make([]string, len(graph[node]))
		copy(deps, graph[node])
		slices.Sort(deps)

		for _, dep := range deps {
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
		path = path[:len(path)-1] // Remove from path
		result = append(result, node)
		return nil
	}

	// Visit all nodes in sorted order to ensure deterministic behavior
	var nodes []string
	for node := range graph {
		nodes = append(nodes, node)
	}
	slices.Sort(nodes)

	for _, node := range nodes {
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

// constructCyclePath constructs a detailed cycle path showing the dependency chain
func (app *StdApplication) constructCyclePath(path []string, cycleNode string, edges []DependencyEdge) string {
	// Find the start of the cycle
	cycleStart := -1
	for i, node := range path {
		if node == cycleNode {
			cycleStart = i
			break
		}
	}

	if cycleStart == -1 {
		// Fallback to simple cycle indication
		return fmt.Sprintf("cycle detected involving %s", cycleNode)
	}

	// Build the cycle path with edge type information
	cyclePath := path[cycleStart:]
	cyclePath = append(cyclePath, cycleNode) // Complete the cycle

	var pathDetails []string
	for i := 0; i < len(cyclePath)-1; i++ {
		from := cyclePath[i]
		to := cyclePath[i+1]

		// Find the edge that connects these nodes
		edgeInfo := app.findDependencyEdge(from, to, edges)
		pathDetails = append(pathDetails, fmt.Sprintf("%s →%s %s", from, edgeInfo, to))
	}

	return fmt.Sprintf("cycle: %s", strings.Join(pathDetails, " → "))
}

// findDependencyEdge finds the dependency edge between two modules and returns a description
func (app *StdApplication) findDependencyEdge(from, to string, edges []DependencyEdge) string {
	for _, edge := range edges {
		if edge.From == from && edge.To == to {
			switch edge.Type {
			case EdgeTypeModule:
				return "(module)"
			case EdgeTypeNamedService:
				return fmt.Sprintf("(service:%s)", edge.ServiceName)
			case EdgeTypeInterfaceService:
				interfaceName := "unknown"
				if edge.InterfaceType != nil {
					interfaceName = edge.InterfaceType.String() // Use String() for fully qualified name
				}
				return fmt.Sprintf("(interface:%s)", interfaceName)
			}
		}
	}
	return "(unknown)" // Fallback
}

// addImplicitDependencies analyzes service provider/consumer relationships to find implicit dependencies
// where modules provide services that other modules require via interface matching.
// Returns the edges that were added for cycle detection.
func (app *StdApplication) addImplicitDependencies(graph map[string][]string) []DependencyEdge {
	// Collect all required interfaces and service providers
	requiredInterfaces, serviceProviders := app.collectServiceRequirements()

	// Find interface implementations with interface type information
	interfaceMatches := app.findInterfaceMatches(requiredInterfaces)

	// Add dependencies to the graph and collect edges
	var edges []DependencyEdge
	namedEdges := app.addNameBasedDependencies(graph, serviceProviders)
	interfaceEdges := app.addInterfaceBasedDependenciesWithTypeInfo(graph, interfaceMatches)

	edges = append(edges, namedEdges...)
	edges = append(edges, interfaceEdges...)

	return edges
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
	required      bool
}

// InterfaceMatch represents a consumer-provider match for an interface-based dependency
type InterfaceMatch struct {
	Consumer      string
	Provider      string
	InterfaceType reflect.Type
	ServiceName   string
	Required      bool
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
			required:      svcDep.Required,
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

// findInterfaceMatches finds which modules provide services that implement required interfaces
// and returns structured matches with type information for better cycle detection
func (app *StdApplication) findInterfaceMatches(
	requiredInterfaces map[string][]interfaceRequirement,
) []InterfaceMatch {
	var matches []InterfaceMatch

	for moduleName, module := range app.moduleRegistry {
		svcAwareModule, ok := module.(ServiceAware)
		if !ok {
			continue
		}

		moduleMatches := app.findModuleInterfaceMatches(moduleName, svcAwareModule, requiredInterfaces)
		matches = append(matches, moduleMatches...)
	}

	return matches
}

// findModuleInterfaceMatches finds interface matches for a specific module
func (app *StdApplication) findModuleInterfaceMatches(
	moduleName string,
	svcAwareModule ServiceAware,
	requiredInterfaces map[string][]interfaceRequirement,
) []InterfaceMatch {
	var matches []InterfaceMatch

	for _, svcProvider := range svcAwareModule.ProvidesServices() {
		// Check if this service satisfies any required interfaces
		for _, requirements := range requiredInterfaces {
			for _, requirement := range requirements {
				svcType := reflect.TypeOf(svcProvider.Instance)
				if app.typeImplementsInterface(svcType, requirement.interfaceType) {
					// Skip accidental self-dependencies where service names differ but interfaces match
					if app.shouldSkipAccidentalSelfDependency(moduleName, requirement.moduleName, svcProvider.Name, requirement.serviceName) {
						continue
					}

					matches = append(matches, InterfaceMatch{
						Consumer:      requirement.moduleName,
						Provider:      moduleName,
						InterfaceType: requirement.interfaceType,
						ServiceName:   requirement.serviceName,
						Required:      requirement.required,
					})

					app.logger.Debug("Interface match found",
						"consumer", requirement.moduleName,
						"provider", moduleName,
						"service", requirement.serviceName,
						"interface", requirement.interfaceType.Name())
				}
			}
		}
	}

	return matches
}

// shouldSkipAccidentalSelfDependency determines if a self-dependency should be skipped
// to prevent accidental self-dependencies where different service names match the same interface.
// Returns true if this is an accidental self-dependency that should be skipped.
// Only allows intentional self-dependencies where both module and service names match.
func (app *StdApplication) shouldSkipAccidentalSelfDependency(providerModule, consumerModule, providerServiceName, consumerServiceName string) bool {
	// Allow self-dependencies only when the service names match (intentional self-dependency)
	// Skip self-dependencies when only interfaces match but service names differ (accidental)
	return providerModule == consumerModule && providerServiceName != consumerServiceName
}

// typeImplementsInterface checks if a type implements an interface
func (app *StdApplication) typeImplementsInterface(svcType, interfaceType reflect.Type) bool {
	if svcType == nil || interfaceType == nil {
		return false
	}
	if svcType.Implements(interfaceType) {
		return true
	}
	if svcType.Kind() == reflect.Ptr {
		et := svcType.Elem()
		if et != nil && et.Implements(interfaceType) {
			return true
		}
	}
	return false
}

// addNameBasedDependencies adds dependencies based on direct service name matching
func (app *StdApplication) addNameBasedDependencies(graph map[string][]string, serviceProviders map[string]string) []DependencyEdge {
	var edges []DependencyEdge

	for consumerName, module := range app.moduleRegistry {
		svcAwareModule, ok := module.(ServiceAware)
		if !ok {
			continue
		}

		moduleEdges := app.addModuleNameBasedDependencies(consumerName, svcAwareModule, graph, serviceProviders)
		edges = append(edges, moduleEdges...)
	}

	return edges
}

// addModuleNameBasedDependencies adds name-based dependencies for a specific module
func (app *StdApplication) addModuleNameBasedDependencies(
	consumerName string,
	svcAwareModule ServiceAware,
	graph map[string][]string,
	serviceProviders map[string]string,
) []DependencyEdge {
	var edges []DependencyEdge

	for _, svcDep := range svcAwareModule.RequiresServices() {
		if !svcDep.Required || svcDep.MatchByInterface {
			continue // Skip optional or interface-based dependencies
		}

		edge := app.addNameBasedDependency(consumerName, svcDep, graph, serviceProviders)
		if edge != nil {
			edges = append(edges, *edge)
		}
	}

	return edges
}

// addNameBasedDependency adds a single name-based dependency
func (app *StdApplication) addNameBasedDependency(
	consumerName string,
	svcDep ServiceDependency,
	graph map[string][]string,
	serviceProviders map[string]string,
) *DependencyEdge {
	providerModule, exists := serviceProviders[svcDep.Name]
	if !exists || providerModule == consumerName {
		return nil
	}

	// Check if dependency already exists
	for _, existingDep := range graph[consumerName] {
		if existingDep == providerModule {
			return nil // Already exists
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

	return &DependencyEdge{
		From:        consumerName,
		To:          providerModule,
		Type:        EdgeTypeNamedService,
		ServiceName: svcDep.Name,
	}
}

// addInterfaceBasedDependenciesWithTypeInfo adds dependencies based on interface matches
func (app *StdApplication) addInterfaceBasedDependenciesWithTypeInfo(graph map[string][]string, matches []InterfaceMatch) []DependencyEdge {
	var edges []DependencyEdge

	for _, match := range matches {
		edge := app.addInterfaceBasedDependencyWithTypeInfo(match, graph)
		if edge != nil {
			edges = append(edges, *edge)
		}
	}

	return edges
}

// addInterfaceBasedDependencyWithTypeInfo adds a single interface-based dependency with type information
func (app *StdApplication) addInterfaceBasedDependencyWithTypeInfo(match InterfaceMatch, graph map[string][]string) *DependencyEdge {
	// Handle self-providing interface dependencies:
	//  - If the dependency is optional (not required), skip adding a self edge to avoid false cycles
	//  - If the dependency is required, adding the self edge will surface a cycle which communicates
	//    that the requirement cannot be satisfied (the module would need the service before it is provided)
	if match.Consumer == match.Provider {
		if !match.Required {
			app.logger.Debug("Skipping optional self interface dependency", "module", match.Consumer, "interface", match.InterfaceType.Name(), "service", match.ServiceName)
			return nil
		}
		app.logger.Debug("Adding required self interface dependency to expose unsatisfiable self-requirement", "module", match.Consumer, "interface", match.InterfaceType.Name(), "service", match.ServiceName)
	}
	// Check if this dependency already exists
	for _, existingDep := range graph[match.Consumer] {
		if existingDep == match.Provider {
			return nil
		}
	}

	// Add the dependency (including self-dependencies for cycle detection)
	if graph[match.Consumer] == nil {
		graph[match.Consumer] = make([]string, 0)
	}
	graph[match.Consumer] = append(graph[match.Consumer], match.Provider)

	app.logger.Debug("Added interface-based dependency",
		"consumer", match.Consumer,
		"provider", match.Provider,
		"interface", match.InterfaceType.Name(),
		"service", match.ServiceName)

	return &DependencyEdge{
		From:          match.Consumer,
		To:            match.Provider,
		Type:          EdgeTypeInterfaceService,
		InterfaceType: match.InterfaceType,
		ServiceName:   match.ServiceName,
	}
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

// (Intentionally removed old direct service introspection methods; use ServiceIntrospector())
