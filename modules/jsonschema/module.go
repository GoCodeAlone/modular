// Package jsonschema provides JSON Schema validation capabilities for the modular framework.
//
// This module integrates JSON Schema validation into the modular framework, allowing
// applications to validate JSON data against predefined schemas. It supports schema
// compilation from files or URLs and provides multiple validation methods for different
// data sources.
//
// # Features
//
// The jsonschema module provides the following capabilities:
//   - JSON Schema compilation from files, URLs, or embedded schemas
//   - Validation of JSON data from multiple sources (bytes, readers, interfaces)
//   - Error reporting with detailed validation failure information
//   - Service interface for dependency injection
//   - Support for JSON Schema draft versions through underlying library
//   - Thread-safe schema compilation and validation
//
// # Schema Sources
//
// Schemas can be loaded from various sources:
//   - Local files: "/path/to/schema.json"
//   - URLs: "https://example.com/api/schema.json"
//   - Embedded schemas: "embedded://user-schema"
//   - Schema registry: "registry://user/v1"
//
// # Service Registration
//
// The module registers a JSON schema service for dependency injection:
//
//	// Get the JSON schema service
//	schemaService := app.GetService("jsonschema.service").(jsonschema.JSONSchemaService)
//
//	// Compile a schema
//	schema, err := schemaService.CompileSchema("/path/to/user-schema.json")
//
//	// Validate JSON data
//	err = schemaService.ValidateBytes(schema, jsonData)
//
// # Usage Examples
//
// Schema compilation and basic validation:
//
//	// Compile a schema from file
//	schema, err := schemaService.CompileSchema("./schemas/user.json")
//	if err != nil {
//	    return fmt.Errorf("failed to compile schema: %w", err)
//	}
//
//	// Validate JSON bytes
//	jsonData := []byte(`{"name": "John", "age": 30}`)
//	err = schemaService.ValidateBytes(schema, jsonData)
//	if err != nil {
//	    return fmt.Errorf("validation failed: %w", err)
//	}
//
// Validating different data sources:
//
//	// Validate from HTTP request body
//	err = schemaService.ValidateReader(schema, request.Body)
//
//	// Validate Go structs/interfaces
//	userData := map[string]interface{}{
//	    "name": "Alice",
//	    "age":  25,
//	    "email": "alice@example.com",
//	}
//	err = schemaService.ValidateInterface(schema, userData)
//
// HTTP API validation example:
//
//	func validateUserHandler(schemaService jsonschema.JSONSchemaService) http.HandlerFunc {
//	    // Compile schema once at startup
//	    userSchema, err := schemaService.CompileSchema("./schemas/user.json")
//	    if err != nil {
//	        log.Fatal("Failed to compile user schema:", err)
//	    }
//
//	    return func(w http.ResponseWriter, r *http.Request) {
//	        // Validate request body against schema
//	        if err := schemaService.ValidateReader(userSchema, r.Body); err != nil {
//	            http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
//	            return
//	        }
//
//	        // Process valid request...
//	    }
//	}
//
// Configuration validation:
//
//	// Validate application configuration
//	configSchema, err := schemaService.CompileSchema("./schemas/config.json")
//	if err != nil {
//	    return err
//	}
//
//	configData := map[string]interface{}{
//	    "database": map[string]interface{}{
//	        "host": "localhost",
//	        "port": 5432,
//	    },
//	    "logging": map[string]interface{}{
//	        "level": "info",
//	    },
//	}
//
//	if err := schemaService.ValidateInterface(configSchema, configData); err != nil {
//	    return fmt.Errorf("invalid configuration: %w", err)
//	}
//
// # Schema Definition Examples
//
// User schema example (user.json):
//
//	{
//	  "$schema": "https://json-schema.org/draft/2020-12/schema",
//	  "type": "object",
//	  "properties": {
//	    "name": {
//	      "type": "string",
//	      "minLength": 1,
//	      "maxLength": 100
//	    },
//	    "age": {
//	      "type": "integer",
//	      "minimum": 0,
//	      "maximum": 150
//	    },
//	    "email": {
//	      "type": "string",
//	      "format": "email"
//	    }
//	  },
//	  "required": ["name", "age"],
//	  "additionalProperties": false
//	}
//
// # Error Handling
//
// The module provides detailed error information for validation failures,
// including the specific path and reason for each validation error. This
// helps in providing meaningful feedback to users and debugging schema issues.
package jsonschema

import (
	"context"
	"fmt"

	"github.com/CrisisTextLine/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Name is the unique identifier for the jsonschema module.
const Name = "modular.jsonschema"

// Module provides JSON Schema validation capabilities for the modular framework.
// It integrates JSON Schema validation into the service system and provides
// a simple interface for schema compilation and data validation.
//
// The module implements the following interfaces:
//   - modular.Module: Basic module lifecycle
//   - modular.ServiceAware: Service dependency management
//   - modular.ObservableModule: Event observation and emission
//
// The module is stateless and thread-safe, making it suitable for
// concurrent validation operations in web applications and services.
type Module struct {
	schemaService JSONSchemaService
	subject       modular.Subject
}

// NewModule creates a new instance of the JSON schema module.
// This is the primary constructor for the jsonschema module and should be used
// when registering the module with the application.
//
// The module is stateless and creates a new schema service instance
// with a configured JSON schema compiler.
//
// Example:
//
//	app.RegisterModule(jsonschema.NewModule())
func NewModule() *Module {
	module := &Module{}
	module.schemaService = NewJSONSchemaServiceWithEventEmitter(module)
	return module
}

// Name returns the unique identifier for this module.
// This name is used for service registration and dependency resolution.
func (m *Module) Name() string {
	return Name
}

// Init initializes the JSON schema module.
// The module requires no initialization and is ready to use immediately.
// This method is called during application startup but performs no operations.
func (m *Module) Init(app modular.Application) error {
	return nil
}

// ProvidesServices declares services provided by this module.
// The jsonschema module provides a schema validation service that can be
// injected into other modules for JSON data validation.
//
// Provided services:
//   - "jsonschema.service": The JSONSchemaService interface for validation operations
func (m *Module) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        "jsonschema.service",
			Description: "JSON Schema validation service for data validation",
			Instance:    m.schemaService,
		},
	}
}

// RequiresServices declares services required by this module.
// The jsonschema module operates independently and requires no external services.
func (m *Module) RequiresServices() []modular.ServiceDependency {
	return nil
}

// RegisterObservers implements the ObservableModule interface.
// This allows the jsonschema module to register as an observer for events it's interested in.
func (m *Module) RegisterObservers(subject modular.Subject) error {
	m.subject = subject
	// The jsonschema module currently does not need to observe other events,
	// but this method stores the subject for event emission.
	return nil
}

// EmitEvent implements the ObservableModule interface.
// This allows the jsonschema module to emit events to registered observers.
func (m *Module) EmitEvent(ctx context.Context, event cloudevents.Event) error {
	if m.subject == nil {
		return ErrNoSubjectForEventEmission
	}
	if err := m.subject.NotifyObservers(ctx, event); err != nil {
		return fmt.Errorf("failed to notify observers: %w", err)
	}
	return nil
}

// GetRegisteredEventTypes implements the ObservableModule interface.
// Returns all event types that this jsonschema module can emit.
func (m *Module) GetRegisteredEventTypes() []string {
	return []string{
		EventTypeSchemaCompiled,
		EventTypeSchemaError,
		EventTypeValidationSuccess,
		EventTypeValidationFailed,
		EventTypeValidateBytes,
		EventTypeValidateReader,
		EventTypeValidateInterface,
	}
}
