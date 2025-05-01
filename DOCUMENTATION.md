# Modular Framework Detailed Documentation

## Table of Contents

- [Introduction](#introduction)
- [Core Concepts](#core-concepts)
  - [Application](#application)
  - [Modules](#modules)
    - [Core Module Interface](#core-module-interface)
    - [Optional Module Interfaces](#optional-module-interfaces)
  - [Service Registry](#service-registry)
  - [Configuration Management](#configuration-management)
- [Module Lifecycle](#module-lifecycle)
  - [Registration](#registration)
  - [Configuration](#configuration)
  - [Initialization](#initialization)
  - [Startup](#startup)
  - [Shutdown](#shutdown)
- [Service Dependencies](#service-dependencies)
  - [Basic Service Dependencies](#basic-service-dependencies)
  - [Interface-Based Service Matching](#interface-based-service-matching)
  - [Multiple Interface Implementations](#multiple-interface-implementations)
  - [Dependency Resolution with Interface Matching](#dependency-resolution-with-interface-matching)
  - [Best Practices](#best-practices-for-service-dependencies)
- [Service Injection Techniques](#service-injection-techniques)
  - [Constructor Injection](#constructor-injection)
  - [Init-Time Injection](#init-time-injection)
- [Configuration System](#configuration-system)
  - [Config Providers](#config-providers)
  - [Configuration Validation](#configuration-validation)
  - [Default Values](#default-values)
  - [Required Fields](#required-fields)
  - [Custom Validation Logic](#custom-validation-logic)
  - [Configuration Feeders](#configuration-feeders)
- [Multi-tenancy Support](#multi-tenancy-support)
  - [Tenant Context](#tenant-context)
  - [Tenant Service](#tenant-service)
  - [Tenant-Aware Modules](#tenant-aware-modules)
  - [Tenant-Aware Configuration](#tenant-aware-configuration)
  - [Tenant Configuration Loading](#tenant-configuration-loading)
- [Error Handling](#error-handling)
  - [Common Error Types](#common-error-types)
  - [Error Wrapping](#error-wrapping)
- [Testing Modules](#testing-modules)
  - [Mock Application](#mock-application)
  - [Testing Services](#testing-services)

## Introduction

The Modular framework provides a structured approach to building modular Go applications. This document offers in-depth explanations of the framework's features and capabilities, providing developers with the knowledge they need to build robust, maintainable applications.

## Core Concepts

### Application

The Application is the central container that holds all modules, services, and configurations. It manages the lifecycle of modules and provides the infrastructure for dependency injection and service discovery.

```go
// Create a new application
logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
config := &AppConfig{} // Your application config structure
configProvider := modular.NewStdConfigProvider(config)
app := modular.NewStdApplication(configProvider, logger) // Note: NewStdApplication, not NewApplication
```

The framework provides two main application interfaces:

- **Application**: The core interface with basic functionality for modules, services, and configuration
- **TenantApplication**: Extends Application with tenant-specific operations

### Modules

Modules are the building blocks of a Modular application. Each module encapsulates a specific piece of functionality and can provide services to other modules. The Modular framework uses Go interface composition to allow modules to opt-in to different features.

#### Core Module Interface

At its core, the minimal Module interface is extremely simple:

```go
// Module represents a registrable component in the application
type Module interface {
    // Name returns the unique identifier for this module
    Name() string
    
    // Init initializes the module with the application context
    Init(app Application) error
}
```

This minimal interface makes it easy to create simple modules with minimal boilerplate. Everything else is optional.

#### Optional Module Interfaces

Modules can implement additional interfaces to gain more functionality:

```go
// Configurable is an interface for modules that can have configuration
type Configurable interface {
    // RegisterConfig registers configuration requirements
    RegisterConfig(app Application) error
}

// DependencyAware is an interface for modules that can have dependencies
type DependencyAware interface {
    // Dependencies returns names of other modules this module depends on
    Dependencies() []string
}

// ServiceAware is an interface for modules that can provide or require services
type ServiceAware interface {
    // ProvidesServices returns a list of services provided by this module
    ProvidesServices() []ServiceProvider
    
    // RequiresServices returns a list of services required by this module
    RequiresServices() []ServiceDependency
}

// Startable is an interface for modules that can be started
type Startable interface {
    Start(ctx context.Context) error
}

// Stoppable is an interface for modules that can be stopped
type Stoppable interface {
    Stop(ctx context.Context) error
}

// Constructable is an interface for modules that can be constructed with a constructor
type Constructable interface {
    // Constructor returns a function to construct this module
    Constructor() ModuleConstructor
}
```

The application checks if a module implements these interfaces using Go's type assertions and calls the appropriate methods if they exist. This allows modules to only implement the interfaces they need, keeping the code clean and focused.

For example, if your module doesn't provide any services, you don't need to implement `ProvidesServices()`. If your module doesn't need to be explicitly started, you don't need to implement `Start()`.

### Service Registry

The Service Registry is a central repository of services that modules can provide and consume. It enables loose coupling between modules through dependency injection.

```go
// Register a service
app.RegisterService("database", dbConnection)

// Get a service
var db *sql.DB
app.GetService("database", &db)
```

### Configuration Management

Modular provides a flexible configuration system that supports configuration sections for different modules, validation rules, and various sources through config feeders.

```go
// Define configuration
type DatabaseConfig struct {
    Host     string `yaml:"host" default:"localhost"`
    Port     int    `yaml:"port" default:"5432"`
    User     string `yaml:"user" required:"true"`
    Password string `yaml:"password" required:"true"`
    Database string `yaml:"database" required:"true"`
}

// Register configuration
app.RegisterConfigSection("database", modular.NewStdConfigProvider(&DatabaseConfig{}))
```

## Module Lifecycle

### Registration

Modules are registered with the Application, which adds them to an internal registry:

```go
app.RegisterModule(NewDatabaseModule())
app.RegisterModule(NewAPIModule())
```

### Configuration

During the application's `Init` phase, each module that implements the `Configurable` interface will have its `RegisterConfig` method called:

```go
// Implement the Configurable interface
func (m *MyModule) RegisterConfig(app modular.Application) error {
    m.config = &MyConfig{
        // Default values
        Port: 8080,
    }
    app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(m.config))
    return nil // Note: This method returns error
}
```

### Initialization

After configuration, modules are initialized in dependency order:

```go
func (m *MyModule) Init(app modular.Application) error {
    // Initialize the module with the configuration
    if m.config.Debug {
        app.Logger().Debug("Initializing module in debug mode", "module", m.Name())
    }
    
    // Set up resources
    return nil
}
```

### Startup

When the application starts, each module that implements the `Startable` interface will have its `Start` method called:

```go
// Implement the Startable interface
func (m *MyModule) Start(ctx context.Context) error {
    // Start services
    m.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", m.config.Port),
        Handler: m.router,
    }
    
    go func() {
        if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            m.logger.Error("Server error", "error", err)
        }
    }()
    
    return nil
}
```

### Shutdown

When the application stops, each module that implements the `Stoppable` interface will have its `Stop` method called in reverse initialization order:

```go
// Implement the Stoppable interface
func (m *MyModule) Stop(ctx context.Context) error {
    // Graceful shutdown
    return m.server.Shutdown(ctx)
}
```

## Service Dependencies

### Basic Service Dependencies

At the core of Modular's dependency injection system is the `ServiceDependency` struct, which allows modules to declare what services they require:

```go
type ServiceDependency struct {
    Name               string       // Service name to lookup
    Required           bool         // If true, application fails to start if service is missing
    Type               reflect.Type // Concrete type (if known)
    SatisfiesInterface reflect.Type // Interface type (if known)
    MatchByInterface   bool         // If true, find first service that satisfies interface type
}
```

The simplest form of dependency is a name-based lookup:

```go
func (m *MyModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            Name:     "database",
            Required: true,
        },
    }
}
```

With this approach, the framework will look for a service registered with the exact name "database" and inject it into your module.

### Interface-Based Service Matching

A more flexible approach is to specify that your module requires a service that implements a particular interface, regardless of what name it was registered under. This is achieved using the `MatchByInterface` and `SatisfiesInterface` fields:

```go
func (m *MyModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            Name:               "router", // The name used to access this service in your code
            Required:           true,
            MatchByInterface:   true,
            SatisfiesInterface: reflect.TypeOf((*Router)(nil)).Elem(), // The interface the service should implement
        },
    }
}
```

With this configuration, the framework will:

1. Search through all registered services (regardless of their names)
2. Find any service that implements the `Router` interface
3. Inject that service into your module under the name "router"

This allows for greater flexibility in how services are provided and consumed:

- Service providers can name their services however they want (e.g., "chi.router", "gin.router")
- Service consumers can rely on interface compatibility rather than specific implementations
- Implementations can be swapped without changing consumer code

#### Example: Router Service

Consider a scenario where you have a module that needs a router service:

```go
// Define the router interface
type Router interface {
    HandleFunc(pattern string, handler func(http.ResponseWriter, http.Request))
}

// Module that requires any router service
func (m *APIModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            Name:               "router",
            Required:           true,
            MatchByInterface:   true,
            SatisfiesInterface: reflect.TypeOf((*Router)(nil)).Elem(),
        },
    }
}

// Constructor that uses the router
func (m *APIModule) Constructor() modular.ModuleConstructor {
    return func(app modular.Application, services map[string]any) (modular.Module, error) {
        router := services["router"].(Router) // Cast to the interface type
        
        // Register API routes
        router.HandleFunc("/api/users", m.handleUsers)
        
        return m, nil
    }
}
```

Now you can use different router implementations without changing your API module:

```go
// Chi router module
app.RegisterModule(chimux.NewModule())

// OR a custom router
app.RegisterService("custom.router", &MyCustomRouter{})
```

Either way, the `APIModule` will receive a service that implements the `Router` interface, regardless of the actual implementation type or registered name.

### Multiple Interface Implementations

If multiple services in the application implement the same interface, the framework will use the first matching service it finds. This behavior is deterministic but may not always select the service you expect.

For more control in this scenario, you should:

1. Use more specific interfaces for different use cases
2. Use name-based lookup when you need a specific implementation
3. Consider using a selector pattern where a coordinator service decides which implementation to use

#### Example: Multiple Logger Implementations

```go
// If multiple services implement the Logger interface,
// you might want to be more specific:
func (m *MyModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            // When you need any logger:
            Name:               "logger",
            Required:           true,
            MatchByInterface:   true,
            SatisfiesInterface: reflect.TypeOf((*Logger)(nil)).Elem(),
        },
        {
            // When you need a specific logger:
            Name:     "json.logger", // Specific service name
            Required: true,
        },
    }
}
```

### Dependency Resolution with Interface Matching

The Modular framework automatically creates implicit dependencies between modules based on interface matching. This ensures that modules providing services are initialized before modules that require those services.

For example, if:
- Module A requires a service implementing interface X
- Module B provides a service implementing interface X

Then Module B will be initialized before Module A, even if there is no explicit dependency declared between them.

This automatic resolution ensures that services are available when needed, regardless of the order in which modules are registered with the application.

### Best Practices for Service Dependencies

When using interface-based service matching:

1. **Design Focused Interfaces**: Use the interface segregation principle - define small, focused interfaces rather than large, general ones.

2. **Document Required Interfaces**: Clearly document what interfaces your module expects services to implement.

3. **Export Interfaces**: Make interfaces public in their own package so they can be imported by both service providers and consumers.

4. **Use Interface-Based Matching Judiciously**: For optional dependencies or when you want to be flexible about implementations.

5. **Consider Name Conventions**: Even with interface matching, consider using consistent naming conventions for common service types.

## Service Injection Techniques

### Constructor Injection

Constructor injection is the recommended approach for most scenarios:

```go
// Implement the ServiceAware interface
func (m *MyModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            Name:               "db",
            Required:           true,
            MatchByInterface:   true,
            SatisfiesInterface: reflect.TypeOf((*Database)(nil)).Elem(),
        },
    }
}

func (m *MyModule) ProvidesServices() []modular.ServiceProvider {
    return nil // This module doesn't provide any services
}

// Implement the Constructable interface
func (m *MyModule) Constructor() modular.ModuleConstructor {
    return func(app modular.Application, services map[string]any) (modular.Module, error) {
        db, ok := services["db"].(Database)
        if !ok {
            return nil, errors.New("invalid database service")
        }
        
        // Create a new instance with the service
        return &MyModule{
            db: db,
            // Initialize other fields
        }, nil
    }
}
```

Benefits of constructor injection:
- Clear separation of concerns
- Immutable module state after construction
- Easy to test with mock services

### Init-Time Injection

For simpler modules, you can use init-time injection:

```go
// Implement the ServiceAware interface
type SimpleModule struct {
    db Database
}

func (m *SimpleModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            Name:     "database",
            Required: true,
        },
    }
}

func (m *SimpleModule) ProvidesServices() []modular.ServiceProvider {
    return nil // This module doesn't provide any services
}

func (m *SimpleModule) Init(app modular.Application) error {
    // Get the service during initialization
    if err := app.GetService("database", &m.db); err != nil {
        return fmt.Errorf("failed to get database service: %w", err)
    }
    
    return nil
}
```

## Configuration System

### Config Providers

Config Providers are responsible for supplying configuration values to modules. The basic interface is simple:

```go
type ConfigProvider interface {
    GetConfig() any
}
```

The standard implementation, `StdConfigProvider`, wraps a Go struct:

```go
config := &MyConfig{}
provider := modular.NewStdConfigProvider(config)
```

### Configuration Validation

Modular supports configuration validation through struct tags and the `ConfigValidator` interface:

```go
type ConfigValidator interface {
    Validate() error
}
```

### Default Values

Default values are specified using struct tags:

```go
type ServerConfig struct {
    Host string `yaml:"host" default:"localhost"`
    Port int    `yaml:"port" default:"8080"`
}
```

These values are applied during configuration loading if the field is empty or zero.

### Required Fields

Fields can be marked as required:

```go
type DatabaseConfig struct {
    User     string `yaml:"user" required:"true"`
    Password string `yaml:"password" required:"true"`
}
```

If these fields are not provided, the configuration loading will fail with an appropriate error.

### Custom Validation Logic

For more complex validation, implement the `ConfigValidator` interface:

```go
func (c *ServerConfig) Validate() error {
    if c.Port < 1024 || c.Port > 65535 {
        return fmt.Errorf("%w: port must be between 1024 and 65535", modular.ErrConfigValidationFailed)
    }
    return nil
}
```

### Configuration Feeders

Feeders provide a way to load configuration from different sources:

```go
// Load from YAML file
yamlFeeder := feeders.NewYAMLFeeder("config.yaml")

// Load from environment variables
envFeeder := feeders.NewEnvFeeder("MYAPP_")

// Load from .env file
dotEnvFeeder := feeders.NewDotEnvFeeder(".env")

// Apply feeders to config
err := yamlFeeder.Feed(config)
if err != nil {
    // Handle error
}
```

Multiple feeders can be chained, with later feeders overriding values from earlier ones.

## Multi-tenancy Support

### Tenant Context

Tenant Contexts allow operations to be performed in the context of a specific tenant:

```go
// Create a tenant context
tenantID := modular.TenantID("tenant1")
ctx := modular.NewTenantContext(context.Background(), tenantID)

// Get tenant ID from context
if tid, ok := modular.GetTenantIDFromContext(ctx); ok {
    fmt.Println("Current tenant:", tid)
}
```

### Tenant Service

The TenantService interface defines operations for managing tenants:

```go
type TenantService interface {
    // Get tenant-specific configuration
    GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error)
    
    // Get all registered tenant IDs
    GetTenants() []TenantID
    
    // Register a new tenant with configurations
    RegisterTenant(tenantID TenantID, configs map[string]ConfigProvider) error
    
    // Register a module as tenant-aware
    RegisterTenantAwareModule(module TenantAwareModule) error
}
```

### Tenant-Aware Modules

Modules can implement the `TenantAwareModule` interface to respond to tenant lifecycle events:

```go
type TenantAwareModule interface {
    Module
    OnTenantRegistered(tenantID TenantID)
    OnTenantRemoved(tenantID TenantID)
}
```

Implementation example:

```go
func (m *MyModule) OnTenantRegistered(tenantID modular.TenantID) {
    // Initialize resources for this tenant
    m.tenantResources[tenantID] = &TenantResource{
        Cache: cache.New(),
    }
}

func (m *MyModule) OnTenantRemoved(tenantID modular.TenantID) {
    // Cleanup tenant resources
    if resource, exists := m.tenantResources[tenantID]; exists {
        resource.Cache.Close()
        delete(m.tenantResources, tenantID)
    }
}
```

### Tenant-Aware Configuration

Tenant-specific configurations allow different settings per tenant:

```go
// Create tenant-aware config
tenantAwareConfig := modular.NewTenantAwareConfig(
    modular.NewStdConfigProvider(&DefaultConfig{}),
    tenantService,
    "mymodule",
)

// Get tenant-specific config
ctx := GetTenantContext() // From request or other source
config := tenantAwareConfig.GetConfigWithContext(ctx).(*MyConfig)
```

### Tenant Configuration Loading

Modular provides utilities for loading tenant configurations from files:

```go
// Set up file-based tenant config loader
loader := modular.NewFileBasedTenantConfigLoader(modular.TenantConfigParams{
    ConfigNameRegex: regexp.MustCompile(`^tenant-[\w-]+\.(json|yaml)$`),
    ConfigDir:       "./configs/tenants",
})

// Register the loader
app.RegisterService("tenantConfigLoader", loader)
```

## Error Handling

### Common Error Types

Modular defines common error types to help with error handling:

```go
// Service errors
modular.ErrServiceAlreadyRegistered
modular.ErrServiceNotFound
modular.ErrServiceIncompatible

// Config errors
modular.ErrConfigSectionNotFound
modular.ErrConfigValidationFailed

// Dependency errors
modular.ErrCircularDependency
modular.ErrModuleDependencyMissing

// Tenant errors
modular.ErrTenantNotFound
modular.ErrTenantConfigNotFound
```

### Error Wrapping

Modular follows Go's error wrapping conventions to provide context:

```go
if err := doSomething(); err != nil {
    return fmt.Errorf("module '%s' failed: %w", m.Name(), err)
}
```

This allows for error inspection using `errors.Is` and `errors.As`.

## Testing Modules

### Mock Application

Modular facilitates testing with mock implementations:

```go
// Create a mock application
mockApp := &MockApplication{
    configSections: make(map[string]modular.ConfigProvider),
    services: make(map[string]any),
}

// Register test services
mockApp.RegisterService("database", &MockDatabase{})

// Test your module
module := NewMyModule()
err := module.Init(mockApp)
assert.NoError(t, err)
```

### Testing Services

Test service implementations to ensure they meet interface requirements:

```go
func TestDatabaseService(t *testing.T) {
    // Create mock DB
    db := &MockDB{}
    
    // Verify it implements the interface
    var _ DatabaseService = db
    
    // Test specific methods
    err := db.Connect()
    assert.NoError(t, err)
    
    result, err := db.Query("SELECT 1")
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

By using dependency injection and interfaces, Modular makes it easy to test modules in isolation.