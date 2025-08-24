# Modular Framework Detailed Documentation

## Table of Contents

- [Introduction](#introduction)
- [Application Builder API](#application-builder-api)
  - [Builder Pattern](#builder-pattern)
  - [Functional Options](#functional-options)
  - [Decorator Pattern](#decorator-pattern)
- [Core Concepts](#core-concepts)
  - [Application](#application)
  - [Modules](#modules)
    - [Core Module Interface](#core-module-interface)
    - [Optional Module Interfaces](#optional-module-interfaces)
  - [Service Registry](#service-registry)
  - [Configuration Management](#configuration-management)
- [Observer Pattern Integration](#observer-pattern-integration)
  - [CloudEvents Support](#cloudevents-support)
  - [Functional Observers](#functional-observers)
  - [Observable Decorators](#observable-decorators)
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
  - [Instance-Aware Configuration](#instance-aware-configuration)
- [Multi-tenancy Support](#multi-tenancy-support)
  - [Tenant Context](#tenant-context)
  - [Tenant Service](#tenant-service)
  - [Tenant-Aware Modules](#tenant-aware-modules)
  - [Tenant-Aware Configuration](#tenant-aware-configuration)
  - [Tenant Configuration Loading](#tenant-configuration-loading)
- [Error Handling](#error-handling)
- [Debugging and Troubleshooting](#debugging-and-troubleshooting)
  - [Module Interface Debugging](#module-interface-debugging)
  - [Common Issues](#common-issues)
  - [Diagnostic Tools](#diagnostic-tools)
  - [Common Error Types](#common-error-types)
  - [Error Wrapping](#error-wrapping)
- [Testing Modules](#testing-modules)
  - [Mock Application](#mock-application)
  - [Testing Services](#testing-services)

## Introduction

The Modular framework provides a structured approach to building modular Go applications. This document offers in-depth explanations of the framework's features and capabilities, providing developers with the knowledge they need to build robust, maintainable applications.

## Application Builder API

### Builder Pattern

The Modular framework v2.0 introduces a powerful builder pattern for constructing applications. This provides a clean, composable way to configure applications with various decorators and options.

#### Basic Usage

```go
app, err := modular.NewApplication(
    modular.WithLogger(logger),
    modular.WithConfigProvider(configProvider),
    modular.WithModules(
        &DatabaseModule{},
        &APIModule{},
    ),
)
if err != nil {
    return err
}
```

### Functional Options

The builder uses functional options to provide flexibility and extensibility:

#### Core Options

- **`WithLogger(logger)`**: Sets the application logger (required)
- **`WithConfigProvider(provider)`**: Sets the main configuration provider
- **`WithBaseApplication(app)`**: Wraps an existing application with decorators
- **`WithModules(modules...)`**: Registers multiple modules at construction time

#### Configuration Options

- **`WithConfigDecorators(decorators...)`**: Applies configuration decorators for enhanced config processing
- **`InstanceAwareConfig()`**: Enables instance-aware configuration decoration
- **`TenantAwareConfigDecorator(loader)`**: Enables tenant-specific configuration overrides

#### Enhanced Functionality Options

- **`WithTenantAware(loader)`**: Adds multi-tenant capabilities with automatic tenant resolution
- **`WithObserver(observers...)`**: Adds event observers for application lifecycle and custom events

### Decorator Pattern

The framework uses the decorator pattern to add cross-cutting concerns without modifying core application logic:

#### TenantAwareDecorator

Wraps applications to add multi-tenant functionality:

```go
app, err := modular.NewApplication(
    modular.WithLogger(logger),
    modular.WithConfigProvider(configProvider),
    modular.WithTenantAware(&MyTenantLoader{}),
    modular.WithModules(modules...),
)
```

Features:
- Automatic tenant resolution during startup
- Tenant-scoped configuration and services
- Integration with tenant-aware modules

#### ObservableDecorator

Adds observer pattern capabilities with CloudEvents integration:

```go
eventObserver := func(ctx context.Context, event cloudevents.Event) error {
    log.Printf("Event: %s from %s", event.Type(), event.Source())
    return nil
}

app, err := modular.NewApplication(
    modular.WithLogger(logger),
    modular.WithConfigProvider(configProvider),
    modular.WithObserver(eventObserver),
    modular.WithModules(modules...),
)
```

Features:
- Automatic emission of application lifecycle events
- CloudEvents specification compliance
- Multiple observer support with error isolation

#### Benefits of Decorator Pattern

1. **Separation of Concerns**: Cross-cutting functionality is isolated in decorators
2. **Composability**: Multiple decorators can be combined as needed
3. **Flexibility**: Applications can be enhanced without changing core logic
4. **Testability**: Decorators can be tested independently

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

### Module-Aware Environment Variable Resolution

The modular framework includes intelligent environment variable resolution that automatically searches for module-specific environment variables to prevent naming conflicts between modules. When a module registers configuration with `env` tags, the framework searches for environment variables in the following priority order:

1. `MODULENAME_ENV_VAR` (module name prefix - highest priority)
2. `ENV_VAR_MODULENAME` (module name suffix - medium priority)  
3. `ENV_VAR` (original variable name - lowest priority)

This allows different modules to use the same configuration field names without conflicts.

#### Example

Consider a reverse proxy module with this configuration:

```go
type ReverseProxyConfig struct {
    DefaultBackend string `env:"DEFAULT_BACKEND"`
    RequestTimeout int    `env:"REQUEST_TIMEOUT"`
}
```

The framework will search for environment variables in this order:

```bash
# For the reverseproxy module's DEFAULT_BACKEND field:
REVERSEPROXY_DEFAULT_BACKEND=http://api.example.com    # Highest priority
DEFAULT_BACKEND_REVERSEPROXY=http://alt.example.com    # Medium priority
DEFAULT_BACKEND=http://fallback.example.com            # Lowest priority
```

If `REVERSEPROXY_DEFAULT_BACKEND` is set, it will be used. If not, the framework falls back to `DEFAULT_BACKEND_REVERSEPROXY`, and finally to `DEFAULT_BACKEND`.

#### Benefits

- **🚫 No Naming Conflicts**: Different modules can use the same field names safely
- **🔧 Module-Specific Overrides**: Easily configure specific modules without affecting others
- **⬅️ Backward Compatibility**: Existing environment variable configurations continue to work
- **📦 Automatic Resolution**: No code changes required in modules - works automatically
- **🎯 Predictable Patterns**: Consistent naming conventions across all modules

#### Multiple Modules Example

```bash
# Database module configuration
DATABASE_HOST=db.internal.example.com     # Specific to database module
DATABASE_PORT=5432
DATABASE_TIMEOUT=120

# HTTP server module configuration  
HTTPSERVER_HOST=api.external.example.com  # Specific to HTTP server
HTTPSERVER_PORT=8080
HTTPSERVER_TIMEOUT=30

# Fallback values (used by any module if specific values not found)
HOST=localhost
PORT=8000
TIMEOUT=60
```

In this example, the database module gets its specific configuration, the HTTP server gets its specific configuration, and any other modules would use the fallback values.

#### Module Name Resolution

The module name used for environment variable prefixes comes from the module's `Name()` method and is automatically converted to uppercase. For example:

- Module name `"reverseproxy"` → Environment prefix `REVERSEPROXY_`
- Module name `"httpserver"` → Environment prefix `HTTPSERVER_`
- Module name `"database"` → Environment prefix `DATABASE_`

### Instance-Aware Configuration

Instance-aware configuration is a powerful feature that allows you to manage multiple instances of the same configuration type using environment variables with instance-specific prefixes. This is particularly useful for scenarios like multiple database connections, cache instances, or service endpoints where each instance needs separate configuration.

#### Overview

Traditional configuration approaches often struggle with multiple instances because they rely on fixed environment variable names. For example, if you need multiple database connections, both would compete for the same `DSN` environment variable:

```yaml
database:
  connections:
    primary: {}    # Would use DSN env var
    secondary: {}  # Would also use DSN env var - conflict!
```

Instance-aware configuration solves this by using instance-specific prefixes:

```bash
# Single instance (backward compatible)
DRIVER=postgres
DSN=postgres://localhost/db

# Multiple instances with prefixes  
DB_PRIMARY_DRIVER=postgres
DB_PRIMARY_DSN=postgres://localhost/primary
DB_SECONDARY_DRIVER=mysql
DB_SECONDARY_DSN=mysql://localhost/secondary
```

#### InstanceAwareEnvFeeder

The `InstanceAwareEnvFeeder` is the core component that handles environment variable feeding for multiple instances:

```go
// Create an instance-aware feeder with a prefix function
feeder := modular.NewInstanceAwareEnvFeeder(func(instanceKey string) string {
    return "DB_" + strings.ToUpper(instanceKey) + "_"
})

// Feed a single instance with instance-specific environment variables
config := &database.ConnectionConfig{}
err := feeder.FeedKey("primary", config)
// This will look for DB_PRIMARY_DRIVER, DB_PRIMARY_DSN, etc.
```

The `InstanceAwareEnvFeeder` implements three interfaces:

1. **Basic Feeder**: `Feed(interface{}) error` - For backward compatibility
2. **ComplexFeeder**: `FeedKey(string, interface{}) error` - For instance-specific feeding
3. **InstanceAwareFeeder**: `FeedInstances(interface{}) error` - For feeding multiple instances at once

#### InstanceAwareConfigProvider

The `InstanceAwareConfigProvider` wraps configuration objects and associates them with instance prefix functions:

```go
// Create instance-aware config provider
prefixFunc := func(instanceKey string) string {
    return "DB_" + strings.ToUpper(instanceKey) + "_"
}

config := &database.Config{
    Connections: map[string]database.ConnectionConfig{
        "primary":   {},
        "secondary": {},
    },
}

provider := modular.NewInstanceAwareConfigProvider(config, prefixFunc)
app.RegisterConfigSection("database", provider)
```

#### Module Integration

Modules can implement the `InstanceAwareConfigSupport` interface to enable automatic instance-aware configuration:

```go
// InstanceAwareConfigSupport indicates support for instance-aware feeding
type InstanceAwareConfigSupport interface {
    // GetInstanceConfigs returns a map of instance configurations
    GetInstanceConfigs() map[string]interface{}
}
```

Example implementation in the database module:

```go
// GetInstanceConfigs returns the connections map for instance-aware configuration
func (c *Config) GetInstanceConfigs() map[string]interface{} {
    instances := make(map[string]interface{})
    for name, connection := range c.Connections {
        // Create a copy to avoid modifying the original
        connCopy := connection
        instances[name] = &connCopy
    }
    return instances
}
```

#### Environment Variable Patterns

Instance-aware configuration supports consistent naming patterns:

```bash
# Pattern: <PREFIX><INSTANCE_KEY>_<FIELD_NAME>

# Database connections
DB_PRIMARY_DRIVER=postgres
DB_PRIMARY_DSN=postgres://user:pass@localhost/primary
DB_PRIMARY_MAX_OPEN_CONNECTIONS=25

DB_SECONDARY_DRIVER=mysql  
DB_SECONDARY_DSN=mysql://user:pass@localhost/secondary
DB_SECONDARY_MAX_OPEN_CONNECTIONS=10

# Cache instances
CACHE_SESSION_DRIVER=redis
CACHE_SESSION_ADDR=localhost:6379
CACHE_SESSION_DB=0

CACHE_OBJECTS_DRIVER=redis
CACHE_OBJECTS_ADDR=localhost:6379
CACHE_OBJECTS_DB=1

# HTTP servers
HTTP_API_PORT=8080
HTTP_API_HOST=0.0.0.0

HTTP_ADMIN_PORT=8081
HTTP_ADMIN_HOST=127.0.0.1
```

#### Configuration Struct Requirements

For instance-aware configuration to work, configuration structs must have `env` struct tags:

```go
type ConnectionConfig struct {
    Driver string `json:"driver" yaml:"driver" env:"DRIVER"`
    DSN    string `json:"dsn" yaml:"dsn" env:"DSN"`
    MaxOpenConnections int `json:"max_open_connections" yaml:"max_open_connections" env:"MAX_OPEN_CONNECTIONS"`
    MaxIdleConnections int `json:"max_idle_connections" yaml:"max_idle_connections" env:"MAX_IDLE_CONNECTIONS"`
}
```

The `env` tag specifies the environment variable name that will be combined with the instance prefix.

#### Complete Example

Here's a complete example showing how to use instance-aware configuration for multiple database connections:

```go
package main

import (
    "fmt"
    "os"
    
    "github.com/CrisisTextLine/modular"
    "github.com/CrisisTextLine/modular/modules/database"
)

func main() {
    // Set up environment variables for multiple database connections
    os.Setenv("DB_PRIMARY_DRIVER", "postgres")
    os.Setenv("DB_PRIMARY_DSN", "postgres://localhost/primary")
    os.Setenv("DB_SECONDARY_DRIVER", "mysql")
    os.Setenv("DB_SECONDARY_DSN", "mysql://localhost/secondary")
    os.Setenv("DB_CACHE_DRIVER", "sqlite")
    os.Setenv("DB_CACHE_DSN", ":memory:")

    // Create application
    app := modular.NewStdApplication(
        modular.NewStdConfigProvider(&AppConfig{}),
        logger,
    )

    // Register database module (it automatically sets up instance-aware config)
    app.RegisterModule(database.NewModule())

    // Initialize application
    err := app.Init()
    if err != nil {
        panic(err)
    }

    // Get database manager
    var dbManager *database.Module
    app.GetService("database.manager", &dbManager)

    // Access different database connections
    primaryDB, _ := dbManager.GetConnection("primary")   // Uses DB_PRIMARY_*
    secondaryDB, _ := dbManager.GetConnection("secondary") // Uses DB_SECONDARY_*
    cacheDB, _ := dbManager.GetConnection("cache")       // Uses DB_CACHE_*
}
```

#### Manual Instance Configuration

You can also manually configure instances without automatic module support:

```go
// Define configuration with instances
type MyConfig struct {
    Services map[string]ServiceConfig `json:"services" yaml:"services"`
}

type ServiceConfig struct {
    URL     string `json:"url" yaml:"url" env:"URL"`
    Timeout int    `json:"timeout" yaml:"timeout" env:"TIMEOUT"`
    APIKey  string `json:"api_key" yaml:"api_key" env:"API_KEY"`
}

// Set up environment variables
os.Setenv("SVC_AUTH_URL", "https://auth.example.com")
os.Setenv("SVC_AUTH_TIMEOUT", "30")
os.Setenv("SVC_AUTH_API_KEY", "auth-key-123")

os.Setenv("SVC_PAYMENT_URL", "https://payment.example.com")
os.Setenv("SVC_PAYMENT_TIMEOUT", "60")
os.Setenv("SVC_PAYMENT_API_KEY", "payment-key-456")

// Create instance-aware feeder
feeder := modular.NewInstanceAwareEnvFeeder(func(instanceKey string) string {
    return "SVC_" + strings.ToUpper(instanceKey) + "_"
})

// Configure each service instance
config := &MyConfig{
    Services: map[string]ServiceConfig{
        "auth":    {},
        "payment": {},
    },
}

// Feed each instance
for name, serviceConfig := range config.Services {
    configPtr := &serviceConfig
    if err := feeder.FeedKey(name, configPtr); err != nil {
        return fmt.Errorf("failed to configure service %s: %w", name, err)
    }
    config.Services[name] = *configPtr
}
```

#### Best Practices

1. **Consistent Naming**: Use consistent prefix patterns across your application
   ```bash
   DB_<INSTANCE>_<FIELD>     # Database connections
   CACHE_<INSTANCE>_<FIELD>  # Cache instances  
   HTTP_<INSTANCE>_<FIELD>   # HTTP servers
   ```

2. **Uppercase Instance Keys**: Convert instance keys to uppercase for environment variables
   ```go
   prefixFunc := func(instanceKey string) string {
       return "DB_" + strings.ToUpper(instanceKey) + "_"
   }
   ```

3. **Environment Variable Documentation**: Document expected environment variables
   ```bash
   # Required environment variables:
   DB_PRIMARY_DRIVER=postgres
   DB_PRIMARY_DSN=postgres://...
   DB_READONLY_DRIVER=postgres
   DB_READONLY_DSN=postgres://...
   ```

4. **Graceful Defaults**: Provide sensible defaults for non-critical configuration
   ```go
   type ConnectionConfig struct {
       Driver string `env:"DRIVER"`
       DSN    string `env:"DSN"`
       MaxOpenConnections int `env:"MAX_OPEN_CONNECTIONS" default:"25"`
   }
   ```

5. **Validation**: Implement validation for instance configurations
   ```go
   func (c *ConnectionConfig) Validate() error {
       if c.Driver == "" {
           return errors.New("driver is required")
       }
       if c.DSN == "" {
           return errors.New("DSN is required")
       }
       return nil
   }
   ```

#### Benefits

Instance-aware configuration provides several key benefits:

- **🔄 Backward Compatibility**: All existing functionality is preserved
- **🏗️ Extensible Design**: Easy to add to any module configuration
- **🔧 Multiple Patterns**: Supports both single and multi-instance configurations
- **📦 Module Support**: Enhanced support across database, cache, and HTTP server modules
- **✅ No Conflicts**: Different instances don't interfere with each other
- **🎯 Consistent Naming**: Predictable environment variable patterns
- **⚙️ Automatic Configuration**: Modules handle instance-aware configuration automatically

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

## Debugging and Troubleshooting

The Modular framework provides several debugging utilities to help diagnose common issues with module lifecycle, interface implementation, and service injection.

### Module Interface Debugging

#### DebugModuleInterfaces

Use `DebugModuleInterfaces` to check which interfaces a specific module implements:

```go
import "github.com/CrisisTextLine/modular"

// Debug a specific module
modular.DebugModuleInterfaces(app, "your-module-name")
```

**Output example:**
```
🔍 Debugging module 'web-server' (type: *webserver.Module)
   Memory address: 0x14000026840
   ✅ Module
   ✅ Configurable
   ❌ DependencyAware
   ✅ ServiceAware
   ✅ Startable
   ✅ Stoppable
   ❌ Constructable
   📦 Provides 1 services, Requires 0 services
```

#### DebugAllModuleInterfaces

Debug all registered modules at once:

```go
// Debug all modules in the application
modular.DebugAllModuleInterfaces(app)
```

#### CompareModuleInstances

Compare module instances before and after initialization to detect if modules are being replaced:

```go
// Store reference before initialization
originalModule := app.moduleRegistry["module-name"]

// After initialization
currentModule := app.moduleRegistry["module-name"]

modular.CompareModuleInstances(originalModule, currentModule, "module-name")
```

### Common Issues

#### 1. "Module does not implement Startable, skipping"

**Symptoms:** Module has a `Start` method but is reported as not implementing `Startable`.

**Common Causes:**

1. **Incorrect method signature** - Most common issue:
   ```go
   // ❌ WRONG - missing context parameter
   func (m *MyModule) Start() error { return nil }
   
   // ✅ CORRECT
   func (m *MyModule) Start(ctx context.Context) error { return nil }
   ```

2. **Wrong context import:**
   ```go
   // ❌ WRONG - old context package
   import "golang.org/x/net/context"
   
   // ✅ CORRECT - standard library
   import "context"
   ```

3. **Constructor returns module without Startable interface:**
   ```go
   // ❌ PROBLEMATIC - returns different type
   func (m *MyModule) Constructor() ModuleConstructor {
       return func(app Application, services map[string]any) (Module, error) {
           return &DifferentModuleType{}, nil // Lost Startable!
       }
   }
   
   // ✅ CORRECT - preserves all interfaces
   func (m *MyModule) Constructor() ModuleConstructor {
       return func(app Application, services map[string]any) (Module, error) {
           return m, nil // Or create new instance with all interfaces
       }
   }
   ```

#### 2. Service Injection Failures

**Symptoms:** `"failed to inject services for module"` errors.

**Debugging steps:**
1. Verify service names match exactly
2. Check that required services are provided by other modules
3. Ensure dependency order is correct
4. Use interface-based matching for flexibility

#### 3. Module Replacement Issues

**Symptoms:** Module works before `Init()` but fails after.

**Cause:** Constructor-based injection replaces the original module instance.

**Solution:** Ensure your Constructor returns a module that implements all the same interfaces.

### Diagnostic Tools

#### CheckModuleStartableImplementation

For detailed analysis of why a module doesn't implement Startable:

```go
import "github.com/CrisisTextLine/modular"

// Check specific module
modular.CheckModuleStartableImplementation(yourModule)
```

**Output includes:**
- Method signature analysis
- Expected vs actual parameter types
- Interface compatibility check

#### Example Debugging Workflow

When troubleshooting module issues:

```go
func debugModuleIssues(app *modular.StdApplication) {
    // 1. Check all modules before initialization
    fmt.Println("=== BEFORE INIT ===")
    modular.DebugAllModuleInterfaces(app)
    
    // 2. Store references to original modules
    originalModules := make(map[string]modular.Module)
    for name, module := range app.SvcRegistry() {
        originalModules[name] = module
    }
    
    // 3. Initialize
    err := app.Init()
    if err != nil {
        fmt.Printf("Init error: %v\n", err)
    }
    
    // 4. Check modules after initialization
    fmt.Println("=== AFTER INIT ===")
    modular.DebugAllModuleInterfaces(app)
    
    // 5. Compare instances
    for name, original := range originalModules {
        if current, exists := app.SvcRegistry()[name]; exists {
            modular.CompareModuleInstances(original, current, name)
        }
    }
    
    // 6. Check specific problematic modules
    if problematicModule, exists := app.SvcRegistry()["problematic-module"]; exists {
        modular.CheckModuleStartableImplementation(problematicModule)
    }
}
```

#### Best Practices for Debugging

1. **Add debugging early:** Use debugging utilities during development, not just when issues occur.

2. **Check before and after Init():** Many issues occur during the initialization phase when modules are replaced via constructors.

3. **Verify method signatures:** Double-check that your Start/Stop methods match the expected interface signatures exactly.

4. **Use specific error messages:** The debugging tools provide detailed information about why interfaces aren't implemented.

5. **Test interface implementations:** Add compile-time checks to ensure your modules implement expected interfaces:
   ```go
   // Compile-time interface check
   var _ modular.Startable = (*MyModule)(nil)
   var _ modular.Stoppable = (*MyModule)(nil)
   ```

6. **Check memory addresses:** If memory addresses differ before and after Init(), your module was replaced by a constructor.

By using these debugging tools and following these practices, you can quickly identify and resolve module interface and lifecycle issues in your Modular applications.