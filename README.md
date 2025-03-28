# modular
Modular Go

[![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular)

## Overview
Modular is a package that provides a structured way to create modular applications in Go. It allows you to build applications as collections of modules that can be easily added, removed, or replaced. Key features include:

- **Module lifecycle management**: Initialize, start, and gracefully stop modules
- **Dependency management**: Automatically resolve and order module dependencies
- **Service registry**: Register and retrieve application services
- **Configuration management**: Handle configuration for modules and services
- **Dependency injection**: Inject required services into modules

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

### ConfigProvider

Interface for configuration providers:

```go
type ConfigProvider interface {
    GetConfig() any
}
```

## License

[MIT License](LICENSE)
