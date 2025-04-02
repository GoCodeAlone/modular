# JSON Schema Module for Modular

[![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/jsonschema.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/jsonschema)

A [Modular](https://github.com/GoCodeAlone/modular) module that provides JSON Schema validation capabilities.

## Overview

The JSON Schema module provides a service for validating JSON data against JSON Schema specifications. It wraps [github.com/santhosh-tekuri/jsonschema/v6](https://github.com/santhosh-tekuri/jsonschema) to provide a clean, service-oriented interface that integrates with the Modular framework.

## Features

- Compile JSON schemas from file paths or URLs
- Validate JSON data in multiple formats:
  - Raw JSON bytes
  - io.Reader interface
  - Go interface{} values
- Simple integration with other Modular modules

## Installation

```bash
GOPROXY=direct go get github.com/GoCodeAlone/modular/modules/jsonschema@jsonchema/v1.0.0
```

## Usage

### Registering the Module

```go
import (
    "github.com/GoCodeAlone/modular"
    "github.com/GoCodeAlone/modular/modules/jsonschema"
)

func main() {
    app := modular.NewStdApplication(
        modular.NewStdConfigProvider(nil),
        logger,
    )
    
    // Register the JSON Schema module
    app.RegisterModule(jsonschema.NewModule())
    
    // Register your modules that depend on the schema service
    app.RegisterModule(NewYourModule())
    
    // Run the application
    if err := app.Run(); err != nil {
        logger.Error("Application error", "error", err)
    }
}
```

### Using the JSON Schema Service

```go
type YourModule struct {
    schemaService jsonschema.JSONSchemaService
}

// Request the JSON Schema service
func (m *YourModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            Name:               "jsonschema.service",
            Required:           true,
            SatisfiesInterface: reflect.TypeOf((*jsonschema.JSONSchemaService)(nil)).Elem(),
        },
    }
}

// Inject the service using constructor injection
func (m *YourModule) Constructor() modular.ModuleConstructor {
    return func(app *modular.StdApplication, services map[string]any) (modular.Module, error) {
        schemaService, ok := services["jsonschema.service"].(jsonschema.JSONSchemaService)
        if !ok {
            return nil, fmt.Errorf("service 'jsonschema.service' not found or wrong type")
        }
        
        return &YourModule{
            schemaService: schemaService,
        }, nil
    }
}

// Example of using the schema service
func (m *YourModule) ValidateData(schemaPath string, data []byte) error {
    // Compile the schema
    schema, err := m.schemaService.CompileSchema(schemaPath)
    if err != nil {
        return fmt.Errorf("failed to compile schema: %w", err)
    }
    
    // Validate data against the schema
    if err := m.schemaService.ValidateBytes(schema, data); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    return nil
}
```

## API Reference

### Types

#### Schema

```go
type Schema interface {
    // Validate validates the given value against the JSON schema
    Validate(value interface{}) error
}
```

#### JSONSchemaService

```go
type JSONSchemaService interface {
    // CompileSchema compiles a JSON schema from a file path or URL
    CompileSchema(source string) (Schema, error)
    
    // ValidateBytes validates raw JSON data against a compiled schema
    ValidateBytes(schema Schema, data []byte) error
    
    // ValidateReader validates JSON from an io.Reader against a compiled schema
    ValidateReader(schema Schema, reader io.Reader) error
    
    // ValidateInterface validates a Go interface{} against a compiled schema
    ValidateInterface(schema Schema, data interface{}) error
}
```

## License

[MIT License](../../LICENSE)
