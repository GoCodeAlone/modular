package jsonschema

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Schema represents a compiled JSON schema.
// A compiled schema can be used multiple times for validation operations
// and is thread-safe for concurrent use. Schemas should be compiled once
// and reused for performance.
type Schema interface {
	// Validate validates the given value against the JSON schema.
	// The value can be any Go data structure that represents JSON data.
	// Returns an error if validation fails, with details about the failure.
	//
	// Example:
	//	data := map[string]interface{}{"name": "John", "age": 30}
	//	err := schema.Validate(data)
	//	if err != nil {
	//	    // Handle validation error
	//	}
	Validate(value interface{}) error
}

// JSONSchemaService defines the operations that can be performed with JSON schemas.
// This service provides methods for compiling schemas from various sources and
// validating JSON data in different formats.
//
// The service is thread-safe and can be used concurrently from multiple goroutines.
// Schemas should be compiled once and cached for reuse to avoid performance overhead.
//
// Example usage:
//
//	// Compile schema once
//	schema, err := service.CompileSchema("./user-schema.json")
//	if err != nil {
//	    return err
//	}
//	
//	// Use for multiple validations
//	err = service.ValidateBytes(schema, jsonData1)
//	err = service.ValidateBytes(schema, jsonData2)
type JSONSchemaService interface {
	// CompileSchema compiles a JSON schema from a file path or URL.
	// The source can be a local file path, HTTP/HTTPS URL, or other URI
	// supported by the underlying JSON schema library.
	//
	// Supported source formats:
	//   - Local files: "/path/to/schema.json", "./schemas/user.json"
	//   - HTTP URLs: "https://example.com/schemas/user.json"
	//   - HTTPS URLs: "https://schemas.org/draft/2020-12/schema"
	//
	// The compiled schema is cached and can be reused for multiple validations.
	// Compilation is relatively expensive, so schemas should be compiled once
	// at application startup when possible.
	//
	// Example:
	//	schema, err := service.CompileSchema("./schemas/user.json")
	//	if err != nil {
	//	    return fmt.Errorf("failed to compile schema: %w", err)
	//	}
	CompileSchema(source string) (Schema, error)

	// ValidateBytes validates raw JSON data against a compiled schema.
	// The data parameter should contain valid JSON bytes. The method
	// unmarshals the JSON and validates it against the schema.
	//
	// This is useful when you have JSON data as a byte slice, such as
	// from HTTP request bodies, file contents, or network messages.
	//
	// Example:
	//	jsonData := []byte(`{"name": "John", "age": 30}`)
	//	err := service.ValidateBytes(schema, jsonData)
	//	if err != nil {
	//	    return fmt.Errorf("validation failed: %w", err)
	//	}
	ValidateBytes(schema Schema, data []byte) error

	// ValidateReader validates JSON from an io.Reader against a compiled schema.
	// This method reads JSON data from the reader, unmarshals it, and validates
	// against the schema. The reader is consumed entirely during validation.
	//
	// This is useful for validating streaming JSON data, HTTP request bodies,
	// or large JSON files without loading everything into memory first.
	//
	// Example:
	//	file, err := os.Open("data.json")
	//	if err != nil {
	//	    return err
	//	}
	//	defer file.Close()
	//	
	//	err = service.ValidateReader(schema, file)
	//	if err != nil {
	//	    return fmt.Errorf("file validation failed: %w", err)
	//	}
	ValidateReader(schema Schema, reader io.Reader) error

	// ValidateInterface validates a Go interface{} against a compiled schema.
	// The data parameter should be a Go data structure that represents JSON data,
	// such as maps, slices, and primitive types returned by json.Unmarshal.
	//
	// This is useful when you already have unmarshaled JSON data or when
	// working with Go structs that need validation against a schema.
	//
	// Example:
	//	userData := map[string]interface{}{
	//	    "name": "Alice",
	//	    "age":  25,
	//	    "email": "alice@example.com",
	//	}
	//	err := service.ValidateInterface(schema, userData)
	//	if err != nil {
	//	    return fmt.Errorf("user data invalid: %w", err)
	//	}
	ValidateInterface(schema Schema, data interface{}) error
}

// schemaServiceImpl is the concrete implementation of JSONSchemaService.
// It uses the santhosh-tekuri/jsonschema library for JSON schema compilation
// and validation. The implementation is thread-safe and can handle concurrent
// schema compilation and validation operations.
type schemaServiceImpl struct {
	compiler *jsonschema.Compiler
}

// schemaWrapper wraps the jsonschema.Schema to implement our Schema interface.
// This wrapper provides a consistent interface while hiding the underlying
// implementation details from consumers of the service.
type schemaWrapper struct {
	schema *jsonschema.Schema
}

// Validate validates the given value against the JSON schema.
// Returns a wrapped error with additional context if validation fails.
func (s *schemaWrapper) Validate(value interface{}) error {
	if err := s.schema.Validate(value); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}
	return nil
}

// NewJSONSchemaService creates a new JSON schema service.
// The service is initialized with a fresh compiler instance and is ready
// to compile schemas and perform validations immediately.
//
// The service uses sensible defaults and supports JSON Schema draft versions
// as configured by the underlying jsonschema library.
func NewJSONSchemaService() JSONSchemaService {
	return &schemaServiceImpl{
		compiler: jsonschema.NewCompiler(),
	}
}

// CompileSchema compiles a JSON schema from the specified source.
// The source can be a file path, URL, or other URI supported by the compiler.
// Returns a Schema interface that can be used for validation operations.
func (s *schemaServiceImpl) CompileSchema(source string) (Schema, error) {
	schema, err := s.compiler.Compile(source)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema from %s: %w", source, err)
	}
	return &schemaWrapper{schema: schema}, nil
}

// ValidateBytes validates raw JSON data against a compiled schema.
// The method unmarshals the JSON data and then validates it against the schema.
// Returns an error if either unmarshaling or validation fails.
func (s *schemaServiceImpl) ValidateBytes(schema Schema, data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON data: %w", err)
	}
	if err := schema.Validate(v); err != nil {
		return fmt.Errorf("JSON validation failed: %w", err)
	}
	return nil
}

// ValidateReader validates JSON from an io.Reader against a compiled schema.
// The method reads and unmarshals JSON from the reader, then validates it.
// The reader is consumed entirely during the operation.
func (s *schemaServiceImpl) ValidateReader(schema Schema, reader io.Reader) error {
	v, err := jsonschema.UnmarshalJSON(reader)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON from reader: %w", err)
	}
	if err := schema.Validate(v); err != nil {
		return fmt.Errorf("JSON validation failed: %w", err)
	}
	return nil
}

// ValidateInterface validates a Go interface{} against a compiled schema.
// The data should be a structure that represents JSON data (maps, slices, primitives).
// This is the most direct validation method when you already have unmarshaled data.
func (s *schemaServiceImpl) ValidateInterface(schema Schema, data interface{}) error {
	if err := schema.Validate(data); err != nil {
		return fmt.Errorf("interface validation failed: %w", err)
	}
	return nil
}
