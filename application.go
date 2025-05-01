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

type AppRegistry interface {
	SvcRegistry() ServiceRegistry
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
	Application
	GetTenantService() (TenantService, error)
	WithTenant(tenantID TenantID) (*TenantContext, error)
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
	for name, module := range app.moduleRegistry {
		configurableModule, ok := module.(Configurable)
		if !ok {
			app.logger.Debug("Module does not implement Configurable, skipping", "module", name)
			continue
		}
		err := configurableModule.RegisterConfig(app)
		if err != nil {
			return fmt.Errorf("failed to register config for module %s: %w", name, err)
		}
		app.logger.Debug("Registering module", "name", name)
	}

	if err := AppConfigLoader(app); err != nil {
		return fmt.Errorf("failed to load app config: %w", err)
	}

	// Build dependency graph
	moduleOrder, err := app.resolveDependencies()
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Initialize modules in order
	for _, moduleName := range moduleOrder {
		if _, ok := app.moduleRegistry[moduleName].(ServiceAware); ok {
			// Inject required services
			app.moduleRegistry[moduleName], err = app.injectServices(app.moduleRegistry[moduleName])
			if err != nil {
				return fmt.Errorf("failed to inject services for module '%s': %w", moduleName, err)
			}
		}

		if err = app.moduleRegistry[moduleName].Init(app); err != nil {
			return fmt.Errorf("failed to initialize module '%s': %w", moduleName, err)
		}

		if _, ok := app.moduleRegistry[moduleName].(ServiceAware); ok {
			// Register services provided by modules
			for _, svc := range app.moduleRegistry[moduleName].(ServiceAware).ProvidesServices() {
				if err = app.RegisterService(svc.Name, svc.Instance); err != nil {
					return fmt.Errorf("module '%s' failed to register service: %w", moduleName, err)
				}
			}
		}

		app.logger.Info(fmt.Sprintf("Initialized module %s of type %T", moduleName, app.moduleRegistry[moduleName]))
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
	requiredServices := make(map[string]any)
	for _, dep := range module.(ServiceAware).RequiresServices() {
		var service any
		var serviceFound bool
		var serviceName string

		// Interface-based matching case
		if dep.MatchByInterface && dep.SatisfiesInterface != nil && dep.SatisfiesInterface.Kind() == reflect.Interface {
			// Find first service that implements the required interface
			for name, svc := range app.svcRegistry {
				if svc == nil {
					continue
				}

				svcType := reflect.TypeOf(svc)
				if svcType.Implements(dep.SatisfiesInterface) ||
					(svcType.Kind() == reflect.Ptr && svcType.Elem().Implements(dep.SatisfiesInterface)) {
					service = svc
					serviceFound = true
					serviceName = name
					app.logger.Debug("Found service by interface match",
						"interface", dep.SatisfiesInterface,
						"service", name,
						"module", module.Name())
					break
				}
			}
		} else if dep.Name != "" { // Regular name-based lookup
			service, serviceFound = app.svcRegistry[dep.Name]
			serviceName = dep.Name
		}

		if serviceFound {
			if valid, err := checkServiceCompatibility(service, dep); !valid {
				return nil, fmt.Errorf("failed to inject service '%s': %w", serviceName, err)
			}

			requiredServices[serviceName] = service
		} else if dep.Required {
			if dep.MatchByInterface {
				return nil, fmt.Errorf("%w: no service found implementing interface %v for %s",
					ErrRequiredServiceNotFound, dep.SatisfiesInterface, module.Name())
			} else {
				return nil, fmt.Errorf("%w: %s for %s",
					ErrRequiredServiceNotFound, dep.Name, module.Name())
			}
		}
	}

	// If module supports constructor injection, use it
	if withConstructor, ok := module.(Constructable); ok {
		constructor := withConstructor.Constructor()
		newModule, err := constructor(app, requiredServices)
		if err != nil {
			return nil, fmt.Errorf("failed to construct module '%s': %w", module.Name(), err)
		}

		// Replace in registry with constructed instance
		app.moduleRegistry[module.Name()] = newModule
		module = newModule
	}

	return module, nil
}

// checkServiceCompatibility checks if a service satisfies the dependency requirements
func checkServiceCompatibility(service any, dep ServiceDependency) (bool, error) {
	if service == nil {
		return false, fmt.Errorf("%w: %s", ErrServiceNil, dep.Name)
	}

	serviceType := reflect.TypeOf(service)

	// Check concrete type if specified
	if dep.Type != nil && !serviceType.AssignableTo(dep.Type) {
		return false, fmt.Errorf("%w: service '%s' of type %s doesn't satisfy required type %s",
			ErrServiceWrongType, dep.Name, serviceType, dep.Type)
	}

	// Check interface satisfaction
	if dep.SatisfiesInterface != nil && dep.SatisfiesInterface.Kind() == reflect.Interface {
		if serviceType.Implements(dep.SatisfiesInterface) ||
			(serviceType.Kind() == reflect.Ptr && serviceType.Elem().Implements(dep.SatisfiesInterface)) {
			return true, nil
		}

		return false, fmt.Errorf("%w: service '%s' of type %s doesn't satisfy required interface %s",
			ErrServiceWrongInterface, dep.Name, serviceType, dep.SatisfiesInterface)
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
		if _, ok := module.(DependencyAware); !ok {
			app.logger.Debug("Module does not implement DependencyAware, skipping", "module", name)
			graph[name] = nil
			continue
		}
		graph[name] = module.(DependencyAware).Dependencies()
	}

	// Analyze service dependencies to augment the graph with implicit dependencies
	if err := app.addImplicitDependencies(graph); err != nil {
		return nil, err
	}

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
func (app *StdApplication) addImplicitDependencies(graph map[string][]string) error {
	// First, build a map of which modules provide which services
	serviceProviders := make(map[string]string) // service name -> provider module name

	// Map to store which interfaces are implemented by which services
	interfaceProviders := make(map[reflect.Type][]string) // interface type -> slice of service names

	for moduleName, module := range app.moduleRegistry {
		svcAwareModule, ok := module.(ServiceAware)
		if !ok {
			continue // Skip modules that don't provide/require services
		}

		// Register services provided by this module
		for _, svcProvider := range svcAwareModule.ProvidesServices() {
			if svcProvider.Instance == nil {
				continue
			}

			serviceProviders[svcProvider.Name] = moduleName

			// Determine what interfaces this service implements
			svcType := reflect.TypeOf(svcProvider.Instance)

			// For each service dependency across all modules, check if this service implements its interface
			for _, checkModule := range app.moduleRegistry {
				checkSvcAware, ok := checkModule.(ServiceAware)
				if !ok {
					continue
				}

				for _, svcDep := range checkSvcAware.RequiresServices() {
					// We're only interested in interface-based dependencies
					if !svcDep.MatchByInterface || svcDep.SatisfiesInterface == nil ||
						svcDep.SatisfiesInterface.Kind() != reflect.Interface {
						continue
					}

					// Check if this service implements the required interface
					if svcType.Implements(svcDep.SatisfiesInterface) ||
						(svcType.Kind() == reflect.Ptr && svcType.Elem().Implements(svcDep.SatisfiesInterface)) {
						// Add to the map of interface providers
						interfaceList := interfaceProviders[svcDep.SatisfiesInterface]
						if interfaceList == nil {
							interfaceList = make([]string, 0)
						}

						// Only add if not already in the list
						if !slices.Contains(interfaceList, svcProvider.Name) {
							interfaceList = append(interfaceList, svcProvider.Name)
							interfaceProviders[svcDep.SatisfiesInterface] = interfaceList
							app.logger.Debug("Found service implementing interface",
								"service", svcProvider.Name,
								"interface", svcDep.SatisfiesInterface.String(),
								"provider", moduleName)
						}
					}
				}
			}
		}
	}

	// Now analyze each module that requires services
	for consumerName, module := range app.moduleRegistry {
		svcAwareModule, ok := module.(ServiceAware)
		if !ok {
			continue
		}

		// For each required service, check if we can identify the provider module
		for _, svcDep := range svcAwareModule.RequiresServices() {
			// Skip optional dependencies
			if !svcDep.Required {
				continue
			}

			var providerModuleName string

			// Case 1: Direct service name match
			if svcDep.Name != "" && !svcDep.MatchByInterface {
				if provider, ok := serviceProviders[svcDep.Name]; ok {
					providerModuleName = provider
				}
			}

			// Case 2: Interface match
			if svcDep.MatchByInterface && svcDep.SatisfiesInterface != nil &&
				svcDep.SatisfiesInterface.Kind() == reflect.Interface {

				// Find services that implement this interface
				if serviceNames, ok := interfaceProviders[svcDep.SatisfiesInterface]; ok && len(serviceNames) > 0 {
					// Use the first service that implements the interface
					serviceName := serviceNames[0]
					if provider, ok := serviceProviders[serviceName]; ok {
						providerModuleName = provider
					}
				}
			}

			// If we found a provider and it's not the same module, add the dependency
			if providerModuleName != "" && providerModuleName != consumerName {
				// Check if this dependency already exists
				exists := false
				for _, existingDep := range graph[consumerName] {
					if existingDep == providerModuleName {
						exists = true
						break
					}
				}

				// Add the dependency if it doesn't already exist
				if !exists {
					if graph[consumerName] == nil {
						graph[consumerName] = make([]string, 0)
					}
					graph[consumerName] = append(graph[consumerName], providerModuleName)
					app.logger.Debug("Added implicit dependency due to service requirement",
						"consumer", consumerName,
						"provider", providerModuleName,
						"service", svcDep.Name)
				}
			}
		}
	}

	return nil
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
