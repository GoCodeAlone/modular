package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
	"github.com/GoCodeAlone/modular/modules/chimux"
	"github.com/GoCodeAlone/modular/modules/httpserver"
	"github.com/GoCodeAlone/modular/modules/jsonschema"
	"github.com/go-chi/chi/v5"
)

type AppConfig struct {
	Name string `yaml:"name" default:"JSON Schema Demo"`
}

type ValidationRequest struct {
	Schema string      `json:"schema"`
	Data   interface{} `json:"data"`
}

type ValidationResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

type SchemaLibrary struct {
	schemas map[string]string
}

type JSONSchemaModule struct {
	schemaService jsonschema.JSONSchemaService
	router        chi.Router
	library       *SchemaLibrary
}

func (m *JSONSchemaModule) Name() string {
	return "jsonschema-demo"
}

func (m *JSONSchemaModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "jsonschema.service",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*jsonschema.JSONSchemaService)(nil)).Elem(),
		},
		{
			Name:               "router",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*chi.Router)(nil)).Elem(),
		},
	}
}

func (m *JSONSchemaModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		schemaService, ok := services["jsonschema.service"].(jsonschema.JSONSchemaService)
		if !ok {
			return nil, fmt.Errorf("JSON schema service not found or wrong type")
		}

		router, ok := services["router"].(chi.Router)
		if !ok {
			return nil, fmt.Errorf("router service not found or wrong type")
		}

		return &JSONSchemaModule{
			schemaService: schemaService,
			router:        router,
			library:       NewSchemaLibrary(),
		}, nil
	}
}

func (m *JSONSchemaModule) Init(app modular.Application) error {
	// Set up HTTP routes
	m.router.Route("/api/schema", func(r chi.Router) {
		r.Post("/validate", m.validateData)
		r.Get("/library", m.getSchemaLibrary)
		r.Get("/library/{name}", m.getSchema)
		r.Post("/validate/{name}", m.validateWithSchema)
	})

	m.router.Get("/health", m.healthCheck)

	slog.Info("JSON Schema demo module initialized")
	return nil
}

func (m *JSONSchemaModule) validateData(w http.ResponseWriter, r *http.Request) {
	var req ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Schema == "" {
		http.Error(w, "Schema is required", http.StatusBadRequest)
		return
	}

	// Create a temporary schema file
	schemaFile := "/tmp/temp_schema.json"
	if err := os.WriteFile(schemaFile, []byte(req.Schema), 0644); err != nil {
		http.Error(w, "Failed to write schema", http.StatusInternalServerError)
		return
	}
	defer os.Remove(schemaFile)

	// Compile the schema
	schema, err := m.schemaService.CompileSchema(schemaFile)
	if err != nil {
		response := ValidationResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Schema compilation error: %v", err)},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate the data
	response := ValidationResponse{Valid: true}
	if err := m.schemaService.ValidateInterface(schema, req.Data); err != nil {
		response.Valid = false
		response.Errors = []string{err.Error()}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *JSONSchemaModule) getSchemaLibrary(w http.ResponseWriter, r *http.Request) {
	schemas := make(map[string]interface{})
	for name, schemaStr := range m.library.schemas {
		var schema interface{}
		if err := json.Unmarshal([]byte(schemaStr), &schema); err == nil {
			schemas[name] = schema
		}
	}

	response := map[string]interface{}{
		"schemas": schemas,
		"count":   len(schemas),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *JSONSchemaModule) getSchema(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	schemaStr, exists := m.library.schemas[name]
	if !exists {
		http.Error(w, "Schema not found", http.StatusNotFound)
		return
	}

	var schema interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		http.Error(w, "Invalid schema JSON", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schema)
}

func (m *JSONSchemaModule) validateWithSchema(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	schemaStr, exists := m.library.schemas[name]
	if !exists {
		http.Error(w, "Schema not found", http.StatusNotFound)
		return
	}

	var data interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	// Create a temporary schema file
	schemaFile := "/tmp/schema_" + name + ".json"
	if err := os.WriteFile(schemaFile, []byte(schemaStr), 0644); err != nil {
		http.Error(w, "Failed to write schema", http.StatusInternalServerError)
		return
	}
	defer os.Remove(schemaFile)

	// Compile the schema
	schema, err := m.schemaService.CompileSchema(schemaFile)
	if err != nil {
		response := ValidationResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Schema compilation error: %v", err)},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate the data
	response := ValidationResponse{Valid: true}
	if err := m.schemaService.ValidateInterface(schema, data); err != nil {
		response.Valid = false
		// Split error message into individual errors
		errorStr := err.Error()
		response.Errors = strings.Split(errorStr, "\n")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *JSONSchemaModule) healthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":          "healthy",
		"service":         "jsonschema-demo",
		"schemas_loaded":  len(m.library.schemas),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func NewSchemaLibrary() *SchemaLibrary {
	library := &SchemaLibrary{
		schemas: make(map[string]string),
	}

	// User schema
	library.schemas["user"] = `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"id": {
				"type": "integer",
				"minimum": 1
			},
			"name": {
				"type": "string",
				"minLength": 1,
				"maxLength": 100
			},
			"email": {
				"type": "string",
				"format": "email"
			},
			"age": {
				"type": "integer",
				"minimum": 0,
				"maximum": 150
			},
			"role": {
				"type": "string",
				"enum": ["admin", "user", "guest"]
			}
		},
		"required": ["id", "name", "email"],
		"additionalProperties": false
	}`

	// Product schema
	library.schemas["product"] = `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"id": {
				"type": "string",
				"pattern": "^PROD-[0-9]+$"
			},
			"name": {
				"type": "string",
				"minLength": 1,
				"maxLength": 200
			},
			"price": {
				"type": "number",
				"minimum": 0
			},
			"currency": {
				"type": "string",
				"enum": ["USD", "EUR", "GBP"]
			},
			"category": {
				"type": "string",
				"minLength": 1
			},
			"tags": {
				"type": "array",
				"items": {
					"type": "string"
				},
				"uniqueItems": true
			},
			"metadata": {
				"type": "object",
				"additionalProperties": true
			}
		},
		"required": ["id", "name", "price", "currency"],
		"additionalProperties": false
	}`

	// Order schema
	library.schemas["order"] = `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"order_id": {
				"type": "string",
				"pattern": "^ORD-[0-9]{8}$"
			},
			"customer_id": {
				"type": "integer",
				"minimum": 1
			},
			"items": {
				"type": "array",
				"minItems": 1,
				"items": {
					"type": "object",
					"properties": {
						"product_id": {
							"type": "string"
						},
						"quantity": {
							"type": "integer",
							"minimum": 1
						},
						"unit_price": {
							"type": "number",
							"minimum": 0
						}
					},
					"required": ["product_id", "quantity", "unit_price"]
				}
			},
			"total": {
				"type": "number",
				"minimum": 0
			},
			"status": {
				"type": "string",
				"enum": ["pending", "confirmed", "shipped", "delivered", "cancelled"]
			},
			"created_at": {
				"type": "string",
				"format": "date-time"
			}
		},
		"required": ["order_id", "customer_id", "items", "total", "status"],
		"additionalProperties": false
	}`

	// Configuration schema
	library.schemas["config"] = `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"app_name": {
				"type": "string",
				"minLength": 1
			},
			"version": {
				"type": "string",
				"pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+$"
			},
			"debug": {
				"type": "boolean"
			},
			"database": {
				"type": "object",
				"properties": {
					"host": {
						"type": "string",
						"minLength": 1
					},
					"port": {
						"type": "integer",
						"minimum": 1,
						"maximum": 65535
					},
					"username": {
						"type": "string",
						"minLength": 1
					},
					"password": {
						"type": "string",
						"minLength": 1
					}
				},
				"required": ["host", "port", "username"]
			},
			"features": {
				"type": "object",
				"additionalProperties": {
					"type": "boolean"
				}
			}
		},
		"required": ["app_name", "version"],
		"additionalProperties": false
	}`

	return library
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create config provider
	appConfig := &AppConfig{}
	configProvider := modular.NewStdConfigProvider(appConfig)

	// Create application
	app := modular.NewStdApplication(configProvider, logger)

	// Set up configuration feeders
	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewYamlFeeder("config.yaml"),
		feeders.NewEnvFeeder(),
	}

	// Register modules
	app.RegisterModule(jsonschema.NewModule())
	app.RegisterModule(chimux.NewChiMuxModule())
	app.RegisterModule(httpserver.NewHTTPServerModule())
	app.RegisterModule(&JSONSchemaModule{})

	logger.Info("Starting JSON Schema Demo Application")

	// Run application
	if err := app.Run(); err != nil {
		logger.Error("Application error", "error", err)
		os.Exit(1)
	}
}