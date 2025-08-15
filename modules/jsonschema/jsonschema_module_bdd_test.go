package jsonschema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/CrisisTextLine/modular"
	"github.com/cucumber/godog"
)

// JSONSchema BDD Test Context
type JSONSchemaBDDTestContext struct {
	app            modular.Application
	module         *Module
	service        JSONSchemaService
	lastError      error
	compiledSchema Schema
	validationPass bool
	tempFile       string
}

func (ctx *JSONSchemaBDDTestContext) resetContext() {
	ctx.app = nil
	ctx.module = nil
	ctx.service = nil
	ctx.lastError = nil
	ctx.compiledSchema = nil
	ctx.validationPass = false
}

func (ctx *JSONSchemaBDDTestContext) iHaveAModularApplicationWithJSONSchemaModuleConfigured() error {
	ctx.resetContext()

	// Create application with jsonschema module
	logger := &testLogger{}

	// Create app with empty main config
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewStdApplication(mainConfigProvider, logger)

	// Create and register jsonschema module
	ctx.module = NewModule()

	// Register the module
	ctx.app.RegisterModule(ctx.module)

	return nil
}

func (ctx *JSONSchemaBDDTestContext) theJSONSchemaModuleIsInitialized() error {
	err := ctx.app.Init()
	if err != nil {
		ctx.lastError = err
		return nil
	}

	// Get the jsonschema service
	var schemaService JSONSchemaService
	if err := ctx.app.GetService("jsonschema.service", &schemaService); err == nil {
		ctx.service = schemaService
	}

	return nil
}

func (ctx *JSONSchemaBDDTestContext) theJSONSchemaServiceShouldBeAvailable() error {
	if ctx.service == nil {
		return fmt.Errorf("jsonschema service not available")
	}
	return nil
}

func (ctx *JSONSchemaBDDTestContext) iHaveAJSONSchemaServiceAvailable() error {
	err := ctx.iHaveAModularApplicationWithJSONSchemaModuleConfigured()
	if err != nil {
		return err
	}

	return ctx.theJSONSchemaModuleIsInitialized()
}

func (ctx *JSONSchemaBDDTestContext) iCompileASchemaFromAJSONString() error {
	if ctx.service == nil {
		return fmt.Errorf("jsonschema service not available")
	}

	// Create a temporary schema file
	schemaString := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer", "minimum": 0}
		},
		"required": ["name"]
	}`

	// Write to temporary file
	tmpFile, err := os.CreateTemp("", "schema-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(schemaString)
	if err != nil {
		return fmt.Errorf("failed to write schema: %w", err)
	}

	ctx.tempFile = tmpFile.Name()

	schema, err := ctx.service.CompileSchema(ctx.tempFile)
	if err != nil {
		ctx.lastError = err
		return fmt.Errorf("failed to compile schema: %w", err)
	}

	ctx.compiledSchema = schema
	return nil
}

func (ctx *JSONSchemaBDDTestContext) theSchemaShouldBeCompiledSuccessfully() error {
	if ctx.compiledSchema == nil {
		return fmt.Errorf("schema was not compiled")
	}

	if ctx.lastError != nil {
		return fmt.Errorf("schema compilation failed: %v", ctx.lastError)
	}

	return nil
}

func (ctx *JSONSchemaBDDTestContext) iHaveACompiledSchemaForUserData() error {
	return ctx.iCompileASchemaFromAJSONString()
}

func (ctx *JSONSchemaBDDTestContext) iValidateValidUserJSONData() error {
	if ctx.service == nil || ctx.compiledSchema == nil {
		return fmt.Errorf("jsonschema service or schema not available")
	}

	validJSON := []byte(`{"name": "John Doe", "age": 30}`)

	err := ctx.service.ValidateBytes(ctx.compiledSchema, validJSON)
	if err != nil {
		ctx.lastError = err
		ctx.validationPass = false
	} else {
		ctx.validationPass = true
	}

	return nil
}

func (ctx *JSONSchemaBDDTestContext) theValidationShouldPass() error {
	if !ctx.validationPass {
		return fmt.Errorf("validation should have passed but failed: %v", ctx.lastError)
	}

	return nil
}

func (ctx *JSONSchemaBDDTestContext) iValidateInvalidUserJSONData() error {
	if ctx.service == nil || ctx.compiledSchema == nil {
		return fmt.Errorf("jsonschema service or schema not available")
	}

	invalidJSON := []byte(`{"age": "not a number"}`) // Missing required "name" field, invalid type for age

	err := ctx.service.ValidateBytes(ctx.compiledSchema, invalidJSON)
	if err != nil {
		ctx.lastError = err
		ctx.validationPass = false
	} else {
		ctx.validationPass = true
	}

	return nil
}

func (ctx *JSONSchemaBDDTestContext) theValidationShouldFailWithAppropriateErrors() error {
	if ctx.validationPass {
		return fmt.Errorf("validation should have failed but passed")
	}

	if ctx.lastError == nil {
		return fmt.Errorf("expected validation error but got none")
	}

	// Check that error message contains useful information
	errMsg := ctx.lastError.Error()
	if errMsg == "" {
		return fmt.Errorf("validation error message is empty")
	}

	return nil
}

func (ctx *JSONSchemaBDDTestContext) iHaveACompiledSchema() error {
	return ctx.iCompileASchemaFromAJSONString()
}

func (ctx *JSONSchemaBDDTestContext) iValidateDataFromBytes() error {
	if ctx.service == nil || ctx.compiledSchema == nil {
		return fmt.Errorf("jsonschema service or schema not available")
	}

	testData := []byte(`{"name": "Test User", "age": 25}`)
	err := ctx.service.ValidateBytes(ctx.compiledSchema, testData)
	if err != nil {
		ctx.lastError = err
	}

	return nil
}

func (ctx *JSONSchemaBDDTestContext) iValidateDataFromReader() error {
	if ctx.service == nil || ctx.compiledSchema == nil {
		return fmt.Errorf("jsonschema service or schema not available")
	}

	testData := `{"name": "Test User", "age": 25}`
	reader := strings.NewReader(testData)

	err := ctx.service.ValidateReader(ctx.compiledSchema, reader)
	if err != nil {
		ctx.lastError = err
	}

	return nil
}

func (ctx *JSONSchemaBDDTestContext) iValidateDataFromInterface() error {
	if ctx.service == nil || ctx.compiledSchema == nil {
		return fmt.Errorf("jsonschema service or schema not available")
	}

	testData := map[string]interface{}{
		"name": "Test User",
		"age":  25,
	}

	err := ctx.service.ValidateInterface(ctx.compiledSchema, testData)
	if err != nil {
		ctx.lastError = err
	}

	return nil
}

func (ctx *JSONSchemaBDDTestContext) allValidationMethodsShouldWorkCorrectly() error {
	if ctx.lastError != nil {
		return fmt.Errorf("one or more validation methods failed: %v", ctx.lastError)
	}

	return nil
}

func (ctx *JSONSchemaBDDTestContext) iTryToCompileAnInvalidSchema() error {
	if ctx.service == nil {
		return fmt.Errorf("jsonschema service not available")
	}

	invalidSchemaString := `{"type": "invalid_type"}` // Invalid schema type

	// Write to temporary file
	tmpFile, err := os.CreateTemp("", "invalid-schema-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(invalidSchemaString)
	if err != nil {
		return fmt.Errorf("failed to write schema: %w", err)
	}

	_, err = ctx.service.CompileSchema(tmpFile.Name())
	if err != nil {
		ctx.lastError = err
	}

	return nil
}

func (ctx *JSONSchemaBDDTestContext) aSchemaCompilationErrorShouldBeReturned() error {
	if ctx.lastError == nil {
		return fmt.Errorf("expected schema compilation error but got none")
	}

	// Check that error message contains useful information
	errMsg := ctx.lastError.Error()
	if errMsg == "" {
		return fmt.Errorf("schema compilation error message is empty")
	}

	return nil
}

// Test logger implementation
type testLogger struct{}

func (l *testLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *testLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) Error(msg string, keysAndValues ...interface{}) {}

// TestJSONSchemaModuleBDD runs the BDD tests for the JSONSchema module
func TestJSONSchemaModuleBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			testCtx := &JSONSchemaBDDTestContext{}

			// Background
			ctx.Given(`^I have a modular application with jsonschema module configured$`, testCtx.iHaveAModularApplicationWithJSONSchemaModuleConfigured)

			// Steps for module initialization
			ctx.When(`^the jsonschema module is initialized$`, testCtx.theJSONSchemaModuleIsInitialized)
			ctx.Then(`^the jsonschema service should be available$`, testCtx.theJSONSchemaServiceShouldBeAvailable)

			// Steps for basic functionality
			ctx.Given(`^I have a jsonschema service available$`, testCtx.iHaveAJSONSchemaServiceAvailable)
			ctx.When(`^I compile a schema from a JSON string$`, testCtx.iCompileASchemaFromAJSONString)
			ctx.Then(`^the schema should be compiled successfully$`, testCtx.theSchemaShouldBeCompiledSuccessfully)

			// Steps for validation
			ctx.Given(`^I have a compiled schema for user data$`, testCtx.iHaveACompiledSchemaForUserData)
			ctx.When(`^I validate valid user JSON data$`, testCtx.iValidateValidUserJSONData)
			ctx.Then(`^the validation should pass$`, testCtx.theValidationShouldPass)

			ctx.When(`^I validate invalid user JSON data$`, testCtx.iValidateInvalidUserJSONData)
			ctx.Then(`^the validation should fail with appropriate errors$`, testCtx.theValidationShouldFailWithAppropriateErrors)

			// Steps for different validation methods
			ctx.Given(`^I have a compiled schema$`, testCtx.iHaveACompiledSchema)
			ctx.When(`^I validate data from bytes$`, testCtx.iValidateDataFromBytes)
			ctx.When(`^I validate data from reader$`, testCtx.iValidateDataFromReader)
			ctx.When(`^I validate data from interface$`, testCtx.iValidateDataFromInterface)
			ctx.Then(`^all validation methods should work correctly$`, testCtx.allValidationMethodsShouldWorkCorrectly)

			// Steps for error handling
			ctx.When(`^I try to compile an invalid schema$`, testCtx.iTryToCompileAnInvalidSchema)
			ctx.Then(`^a schema compilation error should be returned$`, testCtx.aSchemaCompilationErrorShouldBeReturned)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
