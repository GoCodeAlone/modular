# modular
Modular Go

[![GitHub License](https://img.shields.io/github/license/CrisisTextLine/modular)](https://github.com/GoCodeAlone/modular/blob/main/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular)
[![CodeQL](https://github.com/GoCodeAlone/modular/actions/workflows/github-code-scanning/codeql/badge.svg)](https://github.com/GoCodeAlone/modular/actions/workflows/github-code-scanning/codeql)
[![Dependabot Updates](https://github.com/GoCodeAlone/modular/actions/workflows/dependabot/dependabot-updates/badge.svg)](https://github.com/GoCodeAlone/modular/actions/workflows/dependabot/dependabot-updates)
[![CI](https://github.com/GoCodeAlone/modular/actions/workflows/ci.yml/badge.svg)](https://github.com/GoCodeAlone/modular/actions/workflows/ci.yml)
[![Modules CI](https://github.com/GoCodeAlone/modular/actions/workflows/modules-ci.yml/badge.svg)](https://github.com/GoCodeAlone/modular/actions/workflows/modules-ci.yml)
[![Examples CI](https://github.com/GoCodeAlone/modular/actions/workflows/examples-ci.yml/badge.svg)](https://github.com/GoCodeAlone/modular/actions/workflows/examples-ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/GoCodeAlone/modular)](https://goreportcard.com/report/github.com/GoCodeAlone/modular)
[![codecov](https://codecov.io/gh/CrisisTextLine/modular/graph/badge.svg?token=2HCVC9RTN8)](https://codecov.io/gh/CrisisTextLine/modular)

## Overview
Modular is a package that provides a structured way to create modular applications in Go. It allows you to build applications as collections of modules that can be easily added, removed, or replaced. Key features include:

- **Module lifecycle management**: Initialize, start, and gracefully stop modules
- **Dependency management**: Automatically resolve and order module dependencies
- **Service registry**: Register and retrieve application services
- **Configuration management**: Handle configuration for modules and services
- **Configuration validation**: Validate configurations with defaults, required fields, and custom logic
- **Sample config generation**: Generate sample configuration files in various formats
- **Dependency injection**: Inject required services into modules
- **Multi-tenancy support**: Build applications that serve multiple tenants with isolated configurations
- **Observer pattern**: Event-driven communication with CloudEvents support for standardized event handling

## 🧩 Available Modules

Modular comes with a rich ecosystem of pre-built modules that you can easily integrate into your applications:

| Module                             | Description                              | Configuration | Documentation                                   |
|------------------------------------|------------------------------------------|---------------|-----------------------------------------------|
| [auth](./modules/auth)             | Authentication and authorization with JWT, sessions, password hashing, and OAuth2/OIDC support | Yes | [Documentation](./modules/auth/README.md) |
| [cache](./modules/cache)           | Multi-backend caching with Redis and in-memory support | Yes | [Documentation](./modules/cache/README.md) |
| [chimux](./modules/chimux)         | Chi router integration with middleware support | Yes | [Documentation](./modules/chimux/README.md) |
| [database](./modules/database)     | Database connectivity and SQL operations with multiple driver support | Yes | [Documentation](./modules/database/README.md) |
| [eventbus](./modules/eventbus)     | Asynchronous event handling and pub/sub messaging | Yes | [Documentation](./modules/eventbus/README.md) |
| [eventlogger](./modules/eventlogger) | Structured logging for Observer pattern events with CloudEvents support | Yes | [Documentation](./modules/eventlogger/README.md) |
| [httpclient](./modules/httpclient) | Configurable HTTP client with connection pooling, timeouts, and verbose logging | Yes | [Documentation](./modules/httpclient/README.md) |
| [httpserver](./modules/httpserver) | HTTP/HTTPS server with TLS support, graceful shutdown, and configurable timeouts | Yes | [Documentation](./modules/httpserver/README.md) |
| [jsonschema](./modules/jsonschema) | JSON Schema validation services          | No | [Documentation](./modules/jsonschema/README.md) |
| [letsencrypt](./modules/letsencrypt) | SSL/TLS certificate automation with Let's Encrypt | Yes | [Documentation](./modules/letsencrypt/README.md) |
| [reverseproxy](./modules/reverseproxy) | Reverse proxy with load balancing, circuit breaker, and health monitoring | Yes | [Documentation](./modules/reverseproxy/README.md) |
| [scheduler](./modules/scheduler)   | Job scheduling with cron expressions and worker pools | Yes | [Documentation](./modules/scheduler/README.md) |

Each module is designed to be:
- **Plug-and-play**: Easy integration with minimal configuration
- **Configurable**: Extensive configuration options via YAML, environment variables, or code
- **Production-ready**: Built with best practices, proper error handling, and comprehensive testing
- **Well-documented**: Complete documentation with examples and API references

> 📖 For detailed information about each module, see the [modules directory](modules/README.md) or click on the individual module links above.

## 🌩️ Observer Pattern with CloudEvents Support

Modular includes a powerful Observer pattern implementation with CloudEvents specification support, enabling event-driven communication between components while maintaining full backward compatibility.

### Key Features

- **Traditional Observer Pattern**: Subject/Observer interfaces for event emission and handling
- **CloudEvents Integration**: Industry-standard event format with built-in validation and serialization
- **Dual Event Support**: Emit and handle both traditional ObserverEvents and CloudEvents
- **ObservableApplication**: Enhanced application with automatic lifecycle event emission
- **EventLogger Module**: Structured logging for all events with multiple output targets
- **Transport Independence**: Events ready for HTTP, gRPC, AMQP, and other transports

### Quick Example

```go
// Create observable application with CloudEvents support
app := modular.NewObservableApplication(configProvider, logger)

// Register event logger for structured logging
app.RegisterModule(eventlogger.NewModule())

// Emit CloudEvents using standardized format
event := modular.NewCloudEvent(
    "com.myapp.user.created",   // Type
    "user-service",             // Source  
    userData,                   // Data
    metadata,                   // Extensions
)
err := app.NotifyCloudEventObservers(context.Background(), event)
```

### Documentation

- **[CloudEvents Integration Guide](./CLOUDEVENTS.md)**: Comprehensive documentation for CloudEvents support
- **[Observer Pattern Guide](./OBSERVER_PATTERN.md)**: Traditional Observer pattern documentation  
- **[EventLogger Module](./modules/eventlogger/README.md)**: Structured event logging
- **[Observer Pattern Example](./examples/observer-pattern/)**: Complete working example with CloudEvents

## Examples

The `examples/` directory contains complete, working examples that demonstrate how to use Modular with different patterns and module combinations:

| Example | Description | Features |
|---------|-------------|----------|
| [**basic-app**](./examples/basic-app/) | Simple modular application | HTTP server, routing, configuration |
| [**reverse-proxy**](./examples/reverse-proxy/) | HTTP reverse proxy server | Load balancing, backend routing, CORS |
| [**http-client**](./examples/http-client/) | HTTP client with proxy backend | HTTP client integration, request routing |
| [**advanced-logging**](./examples/advanced-logging/) | Advanced HTTP client logging | Verbose logging, file output, request/response inspection |
| [**observer-pattern**](./examples/observer-pattern/) | Event-driven architecture demo | Observer pattern, CloudEvents, event logging, real-time events |

### Quick Start with Examples

Each example is a complete, standalone application that you can run immediately:

```bash
cd examples/basic-app
GOWORK=off go build
./basic-app
```

Visit the [examples directory](./examples/) for detailed documentation, configuration guides, and step-by-step instructions for each example.

### Learning Path

- **Start with [basic-app](./examples/basic-app/)** to understand core modular patterns
- **Try [reverse-proxy](./examples/reverse-proxy/)** to see advanced routing and CORS configuration
- **Explore [http-client](./examples/http-client/)** for HTTP client integration patterns
- **Study [advanced-logging](./examples/advanced-logging/)** for debugging and monitoring techniques
- **Learn [observer-pattern](./examples/observer-pattern/)** for event-driven architecture with CloudEvents

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
    app := modular.NewStdApplication(configProvider, logger)
    
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
func (m *DatabaseModule) RegisterConfig(app modular.Application) error {
    m.config = &DatabaseConfig{
        DSN: "postgres://user:password@localhost:5432/dbname",
    }
    app.RegisterConfigSection("database", modular.NewStdConfigProvider(m.config))
    return nil
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
func (m *DatabaseModule) Init(app modular.Application) error {
    // Initialize database connection
    db, err := sql.Open("postgres", m.config.DSN)
    if (err != nil) {
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
    return func(app modular.Application, services map[string]any) (modular.Module, error) {
        // Services that were requested in RequiresServices() are available here
        db := services["database"].(*sql.DB)
        
        // Create a new module instance with injected services
        return &APIModule{
            db: db,
        }, nil
    }
}
```

### Interface-Based Service Matching

Modular supports finding and injecting services based on interface compatibility, regardless of the service name:

```go
// Define an interface that services should implement
type LoggerService interface {
    Log(level string, message string)
}

// Require a service that implements a specific interface
func (m *MyModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            Name:               "logger", // The name you'll use to access it in the Constructor
            Required:           true,
            MatchByInterface:   true, // Enable interface-based matching
            SatisfiesInterface: reflect.TypeOf((*LoggerService)(nil)).Elem(),
        },
    }
}

// Constructor will receive any service implementing LoggerService
func (m *MyModule) Constructor() modular.ModuleConstructor {
    return func(app modular.Application, services map[string]any) (modular.Module, error) {
        // This will work even if the actual registered service name is different
        logger := services["logger"].(LoggerService)
        return &MyModule{logger: logger}, nil
    }
}
```

See [DOCUMENTATION.md](DOCUMENTATION.md) for more advanced details about service dependencies and interface matching.

### Logger Management

The framework provides methods to get and set the application logger, allowing for dynamic logger configuration at runtime:

```go
// Get the current logger
currentLogger := app.Logger()

// Switch to a different logger (e.g., for different log levels or output destinations)
newLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
app.SetLogger(newLogger)

// The new logger is now used by the application and all modules
app.Logger().Info("Logger has been switched to JSON format with debug level")
```

This is useful for scenarios such as:
- **Dynamic log level changes**: Switch between debug and production logging based on runtime conditions
- **Configuration-driven logging**: Update logger configuration based on config file changes
- **Environment-specific loggers**: Use different loggers for development vs production
- **Log rotation**: Switch to new log files without restarting the application

**Example: Dynamic log level switching**

```go
// Switch to debug logging when needed
debugLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
app.SetLogger(debugLogger)

// Later, switch back to info level
infoLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
app.SetLogger(infoLogger)
```

### Configuration Management

```go
// Define your configuration struct
type AppConfig struct {
    Name    string `json:"name" yaml:"name" default:"DefaultApp" desc:"Application name"`
    Version string `json:"version" yaml:"version" required:"true" desc:"Application version"`
    Debug   bool   `json:"debug" yaml:"debug" default:"false" desc:"Enable debug mode"`
}

// Implement ConfigValidator interface for custom validation
func (c *AppConfig) Validate() error {
    // Custom validation logic
    if c.Version == "0.0.0" {
        return fmt.Errorf("invalid version: %s", c.Version)
    }
    return nil
}
```

### Configuration Validation and Default Values

Modular now includes powerful configuration validation features:

#### Default Values with Struct Tags

```go
// Define struct with default values
type ServerConfig struct {
    Host        string `yaml:"host" default:"localhost" desc:"Server host"`
    Port        int    `yaml:"port" default:"8080" desc:"Server port"`
    ReadTimeout int    `yaml:"readTimeout" default:"30" desc:"Read timeout in seconds"`
    Debug       bool   `yaml:"debug" default:"false" desc:"Enable debug mode"`
}
```

Default values are automatically applied to fields that have zero/empty values when configurations are loaded.

#### Required Field Validation

```go
type DatabaseConfig struct {
    Host     string `yaml:"host" default:"localhost" desc:"Database host"`
    Port     int    `yaml:"port" default:"5432" desc:"Database port"`
    Name     string `yaml:"name" required:"true" desc:"Database name"` // Must be provided
    User     string `yaml:"user" default:"postgres" desc:"Database user"`
    Password string `yaml:"password" required:"true" desc:"Database password"` // Must be provided
}
```

Required fields are validated during configuration loading, and appropriate errors are returned if they're missing.

#### Custom Validation Logic

```go
// Implement the ConfigValidator interface
func (c *AppConfig) Validate() error {
    // Validate environment is one of the expected values
    validEnvs := map[string]bool{"dev": true, "test": true, "prod": true}
    if !validEnvs[c.Environment] {
        return fmt.Errorf("%w: environment must be one of [dev, test, prod]", modular.ErrConfigValidationFailed)
    }
    
    // Additional custom validation
    if c.Server.Port < 1024 || c.Server.Port > 65535 {
        return fmt.Errorf("%w: server port must be between 1024 and 65535", modular.ErrConfigValidationFailed)
    }
    
    return nil
}
```

#### Generating Sample Configuration Files

```go
// Generate a sample configuration file
cfg := &AppConfig{}
err := modular.SaveSampleConfig(cfg, "yaml", "config-sample.yaml")
if err != nil {
    log.Fatalf("Error generating sample config: %v", err)
}
```

Sample configurations can be generated in YAML, JSON, or TOML formats, with all default values pre-populated.

#### Command-Line Integration

```go
func main() {
    // Generate sample config file if requested
    if len(os.Args) > 1 && os.Args[1] == "--generate-config" {
        format := "yaml"
        if len(os.Args) > 2 {
            format = os.Args[2]
        }
        outputFile := "config-sample." + format
        if len(os.Args) > 3 {
            outputFile = os.Args[3]
        }
        
        cfg := &AppConfig{}
        if err := modular.SaveSampleConfig(cfg, format, outputFile); err != nil {
            fmt.Printf("Error generating sample config: %v\n", err)
            os.Exit(1)  // Error condition should exit with non-zero code
        }
        fmt.Printf("Sample config generated at %s\n", outputFile)
        os.Exit(0)
    }
    
    // Continue with normal application startup...
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

### ConfigValidator

Interface for implementing custom configuration validation logic:

```go
type ConfigValidator interface {
    Validate() error
}
```

## CLI Tool

Modular comes with a command-line tool (`modcli`) to help you create new modules and configurations.

### Installation

You can install the CLI tool using one of the following methods:

#### Using go install (recommended)

```bash
go install github.com/GoCodeAlone/modular/cmd/modcli@latest
```

This will download, build, and install the latest version of the CLI tool directly to your GOPATH's bin directory, which should be in your PATH.

#### Download pre-built binaries

Download the latest release from the [GitHub Releases page](https://github.com/GoCodeAlone/modular/releases) and add it to your PATH.

#### Build from source

```bash
# Clone the repository
git clone https://github.com/GoCodeAlone/modular.git
cd modular/cmd/modcli

# Build the CLI tool
go build -o modcli

# Move to a directory in your PATH
mv modcli /usr/local/bin/  # Linux/macOS
# or add the current directory to your PATH
```

### Usage

Generate a new module:

```bash
modcli generate module --name MyFeature
```

Generate a configuration:

```bash
modcli generate config --name Server
```

For more details on available commands:

```bash
modcli --help
```

Each command includes interactive prompts to guide you through the process of creating modules or configurations with the features you need.

## 📚 Additional Resources

- **[Detailed Documentation](DOCUMENTATION.md)** - Comprehensive guide covering advanced topics, best practices, and in-depth examples
- **[Debugging and Troubleshooting](DOCUMENTATION.md#debugging-and-troubleshooting)** - Diagnostic tools and solutions for common issues
- **[Available Modules](modules/README.md)** - Complete list of pre-built modules with documentation
- **[Examples](examples/)** - Working example applications demonstrating various features

### Having Issues?

If you're experiencing problems with module interfaces (e.g., "Module does not implement Startable"), check out the [debugging section](DOCUMENTATION.md#debugging-and-troubleshooting) which includes diagnostic tools like:

```go
// Debug module interface implementations
modular.DebugModuleInterfaces(app, "your-module-name")

// Check all modules at once
modular.DebugAllModuleInterfaces(app)
```

## License

[MIT License](LICENSE)