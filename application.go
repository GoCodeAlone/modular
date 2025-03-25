package modular

import (
	"fmt"
	"reflect"
)

type AppRegistry interface {
	SvcRegistry() ServiceRegistry
}

// Application represents the core Application container
type Application struct {
	cfgProvider    ConfigProvider
	cfgSections    map[string]ConfigProvider
	svcRegistry    ServiceRegistry
	moduleRegistry ModuleRegistry
	logger         Logger
}

// NewApplication creates a new application instance
func NewApplication(cp ConfigProvider, logger Logger) *Application {
	return &Application{
		cfgProvider:    cp,
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         logger,
	}
}

// ConfigProvider retrieves the application config provider
func (app *Application) ConfigProvider() ConfigProvider {
	return app.cfgProvider
}

// SvcRegistry retrieves the service svcRegistry
func (app *Application) SvcRegistry() ServiceRegistry {
	return app.svcRegistry
}

// RegisterModule adds a module to the application
func (app *Application) RegisterModule(module Module) {
	app.moduleRegistry[module.Name()] = module
}

// RegisterConfigSection registers a configuration section with the application
func (app *Application) RegisterConfigSection(section string, cp ConfigProvider) {
	app.cfgSections[section] = cp
}

// GetConfigSection retrieves a configuration section
func (app *Application) GetConfigSection(section string) (ConfigProvider, error) {
	cp, exists := app.cfgSections[section]
	if !exists {
		return nil, fmt.Errorf("config section '%s' not found", section)
	}
	return cp, nil
}

// RegisterService adds a service with type checking
func (app *Application) RegisterService(name string, service any) error {
	if _, exists := app.svcRegistry[name]; exists {
		return fmt.Errorf("service '%s' already registered", name)
	}

	app.svcRegistry[name] = service
	app.logger.Info("Registered service", "name", name, "type", reflect.TypeOf(service))
	return nil
}

// GetService retrieves a service with type assertion
func (app *Application) GetService(name string, target any) error {
	service, exists := app.svcRegistry[name]
	if !exists {
		return fmt.Errorf("service '%s' not found", name)
	}

	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	serviceType := reflect.TypeOf(service)
	targetType := targetValue.Elem().Type()

	if !serviceType.AssignableTo(targetType) {
		return fmt.Errorf("service '%s' of type %s cannot be assigned to %s",
			name, serviceType, targetType)
	}

	targetValue.Elem().Set(reflect.ValueOf(service))
	return nil
}

// Init initializes the application with the provided modules
func (app *Application) Init() error {
	for name, module := range app.moduleRegistry {
		module.RegisterConfig(app)
		app.logger.Info("Registering module", "name", name)
	}

	if err := loadAppConfig(app); err != nil {
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

	return nil
}

// injectServices injects required services into a module
func (app *Application) injectServices(module Module) (Module, error) {
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
	}
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
func (app *Application) Logger() Logger {
	return app.logger
}

// resolveDependencies returns modules in initialization order
func (app *Application) resolveDependencies() ([]string, error) {
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
