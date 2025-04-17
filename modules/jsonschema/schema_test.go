package jsonschema_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/modules/jsonschema"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Define static error
var errInvalidJSONSchemaService = errors.New("service is not of type jsonschema.JSONSchemaService or is nil")

func TestJSONSchemaService(t *testing.T) {
	// Create a simple schema
	schemaJSON := `{
		"type": "object",
		"properties": {
			"name": { "type": "string" },
			"age": { "type": "integer", "minimum": 0 }
		},
		"required": ["name", "age"]
	}`

	// Create a temporary schema file
	tmpFile, err := os.CreateTemp("", "schema-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(schemaJSON); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Create the schema service
	service := jsonschema.NewJSONSchemaService()

	// Compile the schema
	schema, err := service.CompileSchema(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to compile schema: %v", err)
	}

	// Valid JSON
	validJSON := `{"name": "John", "age": 30}`
	if err := service.ValidateBytes(schema, []byte(validJSON)); err != nil {
		t.Errorf("Expected valid JSON to pass validation: %v", err)
	}

	// Invalid JSON (missing required field)
	invalidJSON := `{"name": "John"}`
	if err := service.ValidateBytes(schema, []byte(invalidJSON)); err == nil {
		t.Error("Expected invalid JSON to fail validation")
	}

	// Reader interface validation
	reader := strings.NewReader(validJSON)
	if err := service.ValidateReader(schema, reader); err != nil {
		t.Errorf("Expected valid JSON reader to pass validation: %v", err)
	}

	// Interface validation
	validData := map[string]interface{}{
		"name": "John",
		"age":  30,
	}
	if err := service.ValidateInterface(schema, validData); err != nil {
		t.Errorf("Expected valid interface to pass validation: %v", err)
	}
}

// In your module implementation, you would request the JSONSchemaService:
type YourModule struct {
	t             *testing.T
	app           modular.Application
	schemaService jsonschema.JSONSchemaService
	shutdown      []func()
	schema        jsonschema.Schema
}

func (m *YourModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "jsonschema.service",
			Required:           true,
			SatisfiesInterface: reflect.TypeOf((*jsonschema.JSONSchemaService)(nil)).Elem(),
		},
	}
}

func (m *YourModule) RegisterConfig(app modular.Application) {
}

func (m *YourModule) Init(app modular.Application) error {
	schemaJSON := `{
		"type": "object",
		"properties": {
			"name": { "type": "string" },
			"age": { "type": "integer", "minimum": 0 }
		},
		"required": ["name", "age"]
	}`

	// Create a temporary schema file
	tmpFile, err := os.CreateTemp("", "schema-*.json")
	if err != nil {
		m.t.Fatal(err)
	}
	m.shutdown = append(m.shutdown, func() { os.Remove(tmpFile.Name()) })

	if _, err = tmpFile.WriteString(schemaJSON); err != nil {
		m.t.Fatal(err)
	}
	tmpFile.Close()

	schema, err := m.schemaService.CompileSchema(tmpFile.Name())
	if err != nil {
		m.t.Fatalf("Failed to compile schema: %v", err)
	}

	m.schema = schema

	return nil
}

func (m *YourModule) Start(ctx context.Context) error {
	err := m.schemaService.ValidateBytes(m.schema, []byte(`{"name": "John", "age": 30}`))
	if err != nil {
		m.t.Errorf("Expected valid JSON to pass validation: %v", err)
	}
	m.app.Logger().Info("Successfully validated JSON against schema")
	return nil
}

func (m *YourModule) Stop(ctx context.Context) error {
	defer func() {
		for _, shutdown := range m.shutdown {
			shutdown()
		}
	}()
	return nil
}

func (m *YourModule) Name() string {
	return "yourmodule"
}

func (m *YourModule) Dependencies() []string {
	return []string{jsonschema.Name}
}

func (m *YourModule) ProvidesServices() []modular.ServiceProvider {
	return nil
}

func (m *YourModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		// Get the JSONSchemaService from the services map
		schemaService, ok := services["jsonschema.service"].(jsonschema.JSONSchemaService)
		if !ok {
			return nil, fmt.Errorf("service 'jsonschema.service': %w", errInvalidJSONSchemaService)
		}

		return &YourModule{
			t:             m.t,
			app:           app,
			schemaService: schemaService,
			shutdown:      []func(){},
		}, nil
	}
}

// Example showing how to use JSONSchemaService in another module
func TestExample_dependentModule(t *testing.T) {
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(nil),
		slog.New(slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{},
		)),
	)

	app.RegisterModule(jsonschema.NewModule())
	app.RegisterModule(&YourModule{t: t})

	if err := app.Init(); err != nil {
		t.Errorf("Failed to initialize application: %v", err)
	}
	if err := app.Start(); err != nil {
		t.Errorf("Failed to start application: %v", err)
	}
	if err := app.Stop(); err != nil {
		t.Errorf("Failed to stop application: %v", err)
	}
}
