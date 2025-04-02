# modular
Modular Go

[![GitHub License](https://img.shields.io/github/license/GoCodeAlone/modular)](https://github.com/GoCodeAlone/modular/blob/main/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular)
[![CodeQL](https://github.com/GoCodeAlone/modular/actions/workflows/github-code-scanning/codeql/badge.svg)](https://github.com/GoCodeAlone/modular/actions/workflows/github-code-scanning/codeql)
[![Dependabot Updates](https://github.com/GoCodeAlone/modular/actions/workflows/dependabot/dependabot-updates/badge.svg)](https://github.com/GoCodeAlone/modular/actions/workflows/dependabot/dependabot-updates)
[![CI](https://github.com/GoCodeAlone/modular/actions/workflows/ci.yml/badge.svg)](https://github.com/GoCodeAlone/modular/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/GoCodeAlone/modular)](https://goreportcard.com/report/github.com/GoCodeAlone/modular)
[![codecov](https://codecov.io/gh/GoCodeAlone/modular/graph/badge.svg?token=2HCVC9RTN8)](https://codecov.io/gh/GoCodeAlone/modular)

## Overview
Modular is a package that provides a structured way to create modular applications in Go. It allows you to build applications as collections of modules that can be easily added, removed, or replaced. Key features include:

- **Module lifecycle management**: Initialize, start, and gracefully stop modules
- **Dependency management**: Automatically resolve and order module dependencies
- **Service registry**: Register and retrieve application services
- **Configuration management**: Handle configuration for modules and services
- **Dependency injection**: Inject required services into modules
- **Multi-tenancy support**: Build applications that serve multiple tenants with isolated configurations

## Available Modules

Modular comes with a collection of reusable modules that you can incorporate into your applications:

| Module                             | Description                     | Documentation                                   |
|------------------------------------|---------------------------------|-------------------------------------------------|
| [jsonschema](./modules/jsonschema) | JSON Schema validation services | [Documentation](./modules/jsonschema/README.md) |

For more information about the available modules, see the [modules directory](./modules).

## Installation

```go
go get github.com/GoCodeAlone/modular
```

## Usage

### Basic Application

```go
package main

import (
    "github.com/GoCodeAlone/modular"
    "log/slog"
    "os"
)

func main() {
    // Create a logger
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    
    // Create config provider with application configuration
    config := &AppConfig{
        Name: "MyApp",
        Version: "1.0.0",
    }
    configProvider := modular.NewStdConfigProvider(config)
    
    // Create the application
    app := modular.NewApplication(configProvider, logger)
    
    // Register modules
    app.RegisterModule(NewDatabaseModule())
    app.RegisterModule(NewAPIModule())
    
    // Run the application (this will block until the application is terminated)
    if err := app.Run(); err != nil {
        logger.Error("Application error", "error", err)
        os.Exit(1)
    }
}
```

### Creating a Module

```go
type DatabaseModule struct {
    db     *sql.DB
    config *DatabaseConfig
}

func NewDatabaseModule() modular.Module {
    return &DatabaseModule{}
}

// RegisterConfig registers the module's configuration
func (m *DatabaseModule) RegisterConfig(app *modular.Application) {
    m.config = &DatabaseConfig{
        DSN: "postgres://user:password@localhost:5432/dbname",
    }
    app.RegisterConfigSection("database", modular.NewStdConfigProvider(m.config))
}

// Name returns the module's unique name
func (m *DatabaseModule) Name() string {
    return "database"
}

// Dependencies returns other modules this module depends on
func (m *DatabaseModule) Dependencies() []string {
    return []string{} // No dependencies
}

// Init initializes the module
func (m *DatabaseModule) Init(app *modular.Application) error {
    // Initialize database connection
    db, err := sql.Open("postgres", m.config.DSN)
    if err != nil {
        return err
    }
    m.db = db
    return nil
}

// ProvidesServices returns services provided by this module
func (m *DatabaseModule) ProvidesServices() []modular.ServiceProvider {
    return []modular.ServiceProvider{
        {
            Name:        "database",
            Description: "Database connection",
            Instance:    m.db,
        },
    }
}

// RequiresServices returns services required by this module
func (m *DatabaseModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{} // No required services
}

// Start starts the module
func (m *DatabaseModule) Start(ctx context.Context) error {
    return nil // Database is already connected
}

// Stop stops the module
func (m *DatabaseModule) Stop(ctx context.Context) error {
    return m.db.Close()
}
```

### Service Dependencies

```go
// A module that depends on another service
func (m *APIModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            Name:     "database",
            Required: true,  // Application won't start if this service is missing
        },
        {
            Name:     "cache",
            Required: false, // Optional dependency
        },
    }
}

// Using constructor injection
func (m *APIModule) Constructor() modular.ModuleConstructor {
    return func(app *modular.Application, services map[string]any) (modular.Module, error) {
        // Services that were requested in RequiresServices() are available here
        db := services["database"].(*sql.DB)
        
        // Create a new module instance with injected services
        return &APIModule{
            db: db,
        }, nil
    }
}
```

### Configuration Management

```go
// Define your configuration struct
type AppConfig struct {
    Name    string `json:"name" yaml:"name"`
    Version string `json:"version" yaml:"version"`
}

// Implement ConfigSetup interface if needed
func (c *AppConfig) Setup() error {
    // Validate configuration or set defaults
    if c.Name == "" {
        c.Name = "DefaultApp"
    }
    return nil
}
```

## Multi-Tenant Support

Modular provides built-in support for multi-tenant applications through:

### Tenant Contexts

```go
// Creating a tenant context
tenantID := modular.TenantID("tenant1")
ctx := modular.NewTenantContext(context.Background(), tenantID)

// Using tenant context with the application
tenantCtx, err := app.WithTenant(tenantID)
if err != nil {
    log.Fatal("Failed to create tenant context:", err)
}

// Extract tenant ID from a context
if id, ok := modular.GetTenantIDFromContext(ctx); ok {
    fmt.Println("Current tenant:", id)
}
```

### Tenant-Aware Configuration

```go
// Register a tenant service in your module
func (m *MultiTenantModule) ProvidesServices() []modular.ServiceProvider {
    return []modular.ServiceProvider{
        {
            Name:        "tenantService",
            Description: "Tenant management service",
            Instance:    modular.NewStandardTenantService(m.logger),
        },
        {
            Name:        "tenantConfigLoader",
            Description: "Tenant configuration loader",
            Instance:    modular.DefaultTenantConfigLoader("./configs/tenants"),
        },
    }
}

// Create tenant-aware configuration
func (m *MultiTenantModule) RegisterConfig(app *modular.Application) {
    // Default config
    defaultConfig := &MyConfig{
        Setting: "default",
    }
    
    // Get tenant service (must be provided by another module)
    var tenantService modular.TenantService
    app.GetService("tenantService", &tenantService)
    
    // Create tenant-aware config provider
    tenantAwareConfig := modular.NewTenantAwareConfig(
        modular.NewStdConfigProvider(defaultConfig),
        tenantService,
        "mymodule",
    )
    
    app.RegisterConfigSection("mymodule", tenantAwareConfig)
}

// Using tenant-aware configs in your code
func (m *MultiTenantModule) ProcessRequestWithTenant(ctx context.Context) {
    // Get config specific to the tenant in the context
    config, ok := m.config.(*modular.TenantAwareConfig)
    if !ok {
        // Handle non-tenant-aware config
        return
    }
    
    // Get tenant-specific configuration
    myConfig := config.GetConfigWithContext(ctx).(*MyConfig)
    
    // Use tenant-specific settings
    fmt.Println("Tenant setting:", myConfig.Setting)
}
```

### Tenant-Aware Modules

```go
// Implement the TenantAwareModule interface
type MultiTenantModule struct {
    modular.Module
    tenantData map[modular.TenantID]*TenantData
}

func (m *MultiTenantModule) OnTenantRegistered(tenantID modular.TenantID) {
    // Initialize resources for this tenant
    m.tenantData[tenantID] = &TenantData{
        initialized: true,
    }
}

func (m *MultiTenantModule) OnTenantRemoved(tenantID modular.TenantID) {
    // Clean up tenant resources
    delete(m.tenantData, tenantID)
}
```

### Loading Tenant Configurations

```go
// Set up a file-based tenant config loader
configLoader := modular.NewFileBasedTenantConfigLoader(modular.TenantConfigParams{
    ConfigNameRegex: regexp.MustCompile("^tenant-[\\w-]+\\.(json|yaml)$"),
    ConfigDir:       "./configs/tenants",
    ConfigFeeders:   []modular.Feeder{},
})

// Register the loader as a service
app.RegisterService("tenantConfigLoader", configLoader)
```

## Key Interfaces

### Module

The core interface that all modules must implement:

```go
type Module interface {
    RegisterConfig(app *Application)
    Init(app *Application) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Name() string
    Dependencies() []string
    ProvidesServices() []ServiceProvider
    RequiresServices() []ServiceDependency
}
```

### TenantAwareModule

Interface for modules that need to respond to tenant lifecycle events:

```go
type TenantAwareModule interface {
    Module
    OnTenantRegistered(tenantID TenantID)
    OnTenantRemoved(tenantID TenantID)
}
```

### TenantService

Interface for managing tenants:

```go
type TenantService interface {
    GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error)
    GetTenants() []TenantID
    RegisterTenant(tenantID TenantID, configs map[string]ConfigProvider) error
}
```

### ConfigProvider

Interface for configuration providers:

```go
type ConfigProvider interface {
    GetConfig() any
}
```

## License

[MIT License](LICENSE)
