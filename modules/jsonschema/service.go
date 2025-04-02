package jsonschema

import (
	"encoding/json"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"io"
)

// Schema represents a compiled JSON schema
type Schema interface {
	// Validate validates the given value against the JSON schema
	Validate(value interface{}) error
}

// JSONSchemaService defines the operations that can be performed with JSON schemas
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

// schemaServiceImpl is the concrete implementation of JSONSchemaService
type schemaServiceImpl struct {
	compiler *jsonschema.Compiler
}

// schemaWrapper wraps the jsonschema.Schema to implement our Schema interface
type schemaWrapper struct {
	schema *jsonschema.Schema
}

func (s *schemaWrapper) Validate(value interface{}) error {
	return s.schema.Validate(value)
}

// NewJSONSchemaService creates a new JSON schema service
func NewJSONSchemaService() JSONSchemaService {
	return &schemaServiceImpl{
		compiler: jsonschema.NewCompiler(),
	}
}

func (s *schemaServiceImpl) CompileSchema(source string) (Schema, error) {
	schema, err := s.compiler.Compile(source)
	if err != nil {
		return nil, err
	}
	return &schemaWrapper{schema: schema}, nil
}

func (s *schemaServiceImpl) ValidateBytes(schema Schema, data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	return schema.Validate(v)
}

func (s *schemaServiceImpl) ValidateReader(schema Schema, reader io.Reader) error {
	v, err := jsonschema.UnmarshalJSON(reader)
	if err != nil {
		return err
	}
	return schema.Validate(v)
}

func (s *schemaServiceImpl) ValidateInterface(schema Schema, data interface{}) error {
	return schema.Validate(data)
}
