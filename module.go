// Package modular provides a flexible, modular application framework for Go.
// It supports configuration management, dependency injection, service registration,
// and multi-tenant functionality.
//
// The modular framework allows you to build applications composed of independent
// modules that can declare dependencies, provide services, and be configured
// individually. Each module implements the Module interface and can optionally
// implement additional interfaces like Configurable, ServiceAware, Startable, etc.
//
// Basic usage:
//
//	app := modular.NewStdApplication(configProvider, logger)
//	app.RegisterModule(&MyModule{})
//	if err := app.Run(); err != nil {
//		log.Fatal(err)
//	}
package modular

import "context"

// Module represents a registrable component in the application.
// All modules must implement this interface to be managed by the application.
//
// A module is the basic building block of a modular application. It encapsulates
// a specific piece of functionality and can interact with other modules through
// the application's service registry and configuration system.
type Module interface {
	// Name returns the unique identifier for this module.
	// The name is used for dependency resolution and service registration.
	// It must be unique within the application and should be descriptive
	// of the module's purpose.
	//
	// Example: "database", "auth", "httpserver", "cache"
	Name() string

	// Init initializes the module with the application context.
	// This method is called during application initialization after
	// all modules have been registered and their configurations loaded.
	//
	// The Init method should:
	//   - Validate any required configuration
	//   - Initialize internal state
	//   - Register any services this module provides
	//   - Prepare for Start() to be called
	//
	// Init is called in dependency order - modules that depend on others
	// are initialized after their dependencies.
	Init(app Application) error
}

// Configurable is an interface for modules that can have configuration.
// Modules implementing this interface can register configuration sections
// with the application, allowing them to receive typed configuration data.
//
// The configuration system supports multiple formats (JSON, YAML, TOML)
// and multiple sources (files, environment variables, etc.).
type Configurable interface {
	// RegisterConfig registers configuration requirements with the application.
	// This method is called during application initialization before Init().
	//
	// Implementation should:
	//   - Define the configuration structure
	//   - Register the configuration section with app.RegisterConfigSection()
	//   - Set up any configuration validation rules
	//
	// Example:
	//   func (m *MyModule) RegisterConfig(app Application) error {
	//       cfg := &MyModuleConfig{}
	//       provider := modular.NewStdConfigProvider(cfg)
	//       app.RegisterConfigSection(m.Name(), provider)
	//       return nil
	//   }
	RegisterConfig(app Application) error
}

// DependencyAware is an interface for modules that depend on other modules.
// The framework uses this information to determine initialization order,
// ensuring dependencies are initialized before dependent modules.
//
// Dependencies are resolved by module name and must be exact matches.
// Circular dependencies will cause initialization to fail.
type DependencyAware interface {
	// Dependencies returns names of other modules this module depends on.
	// The returned slice should contain the exact names returned by
	// the Name() method of the dependency modules.
	//
	// Dependencies are initialized before this module during application startup.
	// If any dependency is missing, application initialization will fail.
	//
	// Example:
	//   func (m *WebModule) Dependencies() []string {
	//       return []string{"database", "auth", "cache"}
	//   }
	Dependencies() []string
}

// ServiceAware is an interface for modules that can provide or consume services.
// Services enable loose coupling between modules by providing a registry
// for sharing functionality without direct dependencies.
//
// Modules can both provide services for other modules to use and require
// services that other modules provide. The framework handles service
// injection automatically based on these declarations.
type ServiceAware interface {
	// ProvidesServices returns a list of services provided by this module.
	// These services will be registered in the application's service registry
	// after the module is initialized, making them available to other modules.
	//
	// Each ServiceProvider should specify:
	//   - Name: unique identifier for the service
	//   - Instance: the actual service implementation
	//
	// Example:
	//   func (m *DatabaseModule) ProvidesServices() []ServiceProvider {
	//       return []ServiceProvider{
	//           {Name: "database", Instance: m.db},
	//           {Name: "migrator", Instance: m.migrator},
	//       }
	//   }
	ProvidesServices() []ServiceProvider

	// RequiresServices returns a list of services required by this module.
	// These services must be provided by other modules or the application
	// for this module to function correctly.
	//
	// Services can be matched by name or by interface. When using interface
	// matching, the framework will find any service that implements the
	// specified interface.
	//
	// Example:
	//   func (m *WebModule) RequiresServices() []ServiceDependency {
	//       return []ServiceDependency{
	//           {Name: "database", Required: true},
	//           {Name: "logger", SatisfiesInterface: reflect.TypeOf((*Logger)(nil)).Elem(), MatchByInterface: true},
	//       }
	//   }
	RequiresServices() []ServiceDependency
}

// Startable is an interface for modules that need to perform startup operations.
// Modules implementing this interface will have their Start method called
// after all modules have been initialized successfully.
//
// Start operations typically involve:
//   - Starting background goroutines
//   - Opening network listeners
//   - Connecting to external services
//   - Beginning periodic tasks
type Startable interface {
	// Start begins the module's runtime operations.
	// This method is called after Init() and after all modules have been initialized.
	// Start is called in dependency order - dependencies start before dependents.
	//
	// The provided context is the application's lifecycle context. When this
	// context is cancelled, the module should stop its operations gracefully.
	//
	// Start should be non-blocking for short-running initialization, but may
	// spawn goroutines for long-running operations. Use the provided context
	// to handle graceful shutdown.
	//
	// Example:
	//   func (m *HTTPServerModule) Start(ctx context.Context) error {
	//       go func() {
	//           <-ctx.Done()
	//           m.server.Shutdown(context.Background())
	//       }()
	//       return m.server.ListenAndServe()
	//   }
	Start(ctx context.Context) error
}

// Stoppable is an interface for modules that need to perform cleanup operations.
// Modules implementing this interface will have their Stop method called
// during application shutdown, in reverse dependency order.
//
// Stop operations typically involve:
//   - Gracefully shutting down background goroutines
//   - Closing network connections
//   - Flushing buffers and saving state
//   - Releasing external resources
type Stoppable interface {
	// Stop performs graceful shutdown of the module.
	// This method is called during application shutdown, in reverse dependency
	// order (dependents stop before their dependencies).
	//
	// The provided context includes a timeout for the shutdown process.
	// Modules should respect this timeout and return promptly when it expires.
	//
	// Stop should:
	//   - Stop accepting new work
	//   - Complete or cancel existing work
	//   - Close resources and connections
	//   - Return any critical errors that occurred during shutdown
	//
	// Example:
	//   func (m *DatabaseModule) Stop(ctx context.Context) error {
	//       return m.db.Close()
	//   }
	Stop(ctx context.Context) error
}

// Constructable is an interface for modules that support constructor-based dependency injection.
// This is an advanced feature that allows modules to be reconstructed with their
// dependencies automatically injected as constructor parameters.
//
// This is useful when a module needs its dependencies available during construction
// rather than after initialization, or when using dependency injection frameworks.
type Constructable interface {
	// Constructor returns a function to construct this module with dependency injection.
	// The returned function should have the signature:
	//   func(app Application, services map[string]any) (Module, error)
	//
	// The services map contains all services that this module declared as requirements.
	// The constructor can also accept individual service types as parameters, and
	// the framework will automatically provide them based on type matching.
	//
	// Example:
	//   func (m *WebModule) Constructor() ModuleConstructor {
	//       return func(app Application, services map[string]any) (Module, error) {
	//           db := services["database"].(Database)
	//           return NewWebModule(db), nil
	//       }
	//   }
	Constructor() ModuleConstructor
}

// ModuleConstructor is a function type that creates module instances with dependency injection.
// Constructor functions receive the application instance and a map of resolved services
// that the module declared as requirements.
//
// The constructor should:
//   - Extract required services from the services map
//   - Perform any type assertions needed
//   - Create and return a new module instance
//   - Return an error if construction fails
//
// Constructor functions enable advanced dependency injection patterns and can also
// accept typed parameters that the framework will resolve automatically.
type ModuleConstructor func(app Application, services map[string]any) (Module, error)

// ModuleWithConstructor defines modules that support constructor-based dependency injection.
// This is a convenience interface that combines Module and Constructable.
//
// Modules implementing this interface will be reconstructed using their constructor
// after dependencies are resolved, allowing for cleaner dependency injection patterns.
type ModuleWithConstructor interface {
	Module
	Constructable
}

// ModuleRegistry represents a registry of modules keyed by their names.
// This is used internally by the application to manage registered modules
// and resolve dependencies between them.
//
// The registry ensures each module name is unique and provides efficient
// lookup during dependency resolution and lifecycle management.
type ModuleRegistry map[string]Module
