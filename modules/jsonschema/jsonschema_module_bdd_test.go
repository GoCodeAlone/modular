package jsonschema

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
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
	capturedEvents []cloudevents.Event
	eventObserver  *testEventObserver
}

// testEventObserver captures events for testing
type testEventObserver struct {
	mu     sync.RWMutex
	events []cloudevents.Event
	id     string
}

func newTestEventObserver() *testEventObserver {
	return &testEventObserver{
		id: "test-observer-jsonschema",
	}
}

func (o *testEventObserver) OnEvent(ctx context.Context, event cloudevents.Event) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, event)
	return nil
}

func (o *testEventObserver) ObserverID() string {
	return o.id
}

func (o *testEventObserver) GetEvents() []cloudevents.Event {
	o.mu.RLock()
	defer o.mu.RUnlock()
	// Return a copy of the slice to avoid race conditions
	result := make([]cloudevents.Event, len(o.events))
	copy(result, o.events)
	return result
}

func (o *testEventObserver) ClearEvents() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = nil
}

func (ctx *JSONSchemaBDDTestContext) resetContext() {
	ctx.app = nil
	ctx.module = nil
	ctx.service = nil
	ctx.lastError = nil
	ctx.compiledSchema = nil
	ctx.validationPass = false
	ctx.capturedEvents = nil
	ctx.eventObserver = newTestEventObserver()
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

// Event observation step methods
func (ctx *JSONSchemaBDDTestContext) iHaveAJSONSchemaServiceWithEventObservationEnabled() error {
	ctx.resetContext()

	// Create application with jsonschema config - use ObservableApplication for event support
	logger := &testLogger{}
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)

	// Create jsonschema module
	ctx.module = NewModule()
	ctx.service = ctx.module.schemaService

	// Register the module
	ctx.app.RegisterModule(ctx.module)

	// Initialize
	if err := ctx.app.Init(); err != nil {
		return err
	}

	// Start the application
	if err := ctx.app.Start(); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	// Register the event observer with the jsonschema module
	if err := ctx.module.RegisterObservers(ctx.app.(modular.Subject)); err != nil {
		return fmt.Errorf("failed to register observers: %w", err)
	}

	// Register our test observer to capture events
	if err := ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver); err != nil {
		return fmt.Errorf("failed to register test observer: %w", err)
	}

	return nil
}

func (ctx *JSONSchemaBDDTestContext) iCompileAValidSchema() error {
	schemaJSON := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "number"}
		},
		"required": ["name"]
	}`

	// Create temporary file for schema
	tempFile, err := os.CreateTemp("", "test-schema-*.json")
	if err != nil {
		return err
	}
	defer tempFile.Close()

	ctx.tempFile = tempFile.Name()
	if _, err := tempFile.WriteString(schemaJSON); err != nil {
		return err
	}

	// Compile the schema
	ctx.compiledSchema, ctx.lastError = ctx.service.CompileSchema(ctx.tempFile)
	return nil
}

func (ctx *JSONSchemaBDDTestContext) aSchemaCompiledEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeSchemaCompiled {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("schema compiled event not found. Captured events: %v", eventTypes)
}

func (ctx *JSONSchemaBDDTestContext) theEventShouldContainTheSourceInformation() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeSchemaCompiled {
			var eventData map[string]interface{}
			if err := event.DataAs(&eventData); err != nil {
				continue
			}
			if source, ok := eventData["source"]; ok && source != "" {
				return nil
			}
		}
	}

	return fmt.Errorf("schema compiled event with source information not found")
}

func (ctx *JSONSchemaBDDTestContext) aSchemaErrorEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeSchemaError {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("schema error event not found. Captured events: %v", eventTypes)
}

func (ctx *JSONSchemaBDDTestContext) iValidateValidUserJSONDataWithBytesMethod() error {
	if ctx.compiledSchema == nil {
		// Create a user data schema first
		if err := ctx.iHaveACompiledSchemaForUserData(); err != nil {
			return err
		}
	}

	validJSON := `{"name": "John Doe", "age": 30}`
	ctx.lastError = ctx.service.ValidateBytes(ctx.compiledSchema, []byte(validJSON))
	return nil
}

func (ctx *JSONSchemaBDDTestContext) iValidateInvalidUserJSONDataWithBytesMethod() error {
	if ctx.compiledSchema == nil {
		// Create a user data schema first
		if err := ctx.iHaveACompiledSchemaForUserData(); err != nil {
			return err
		}
	}

	invalidJSON := `{"age": "not a number"}` // missing required "name" field and age is not a number
	ctx.lastError = ctx.service.ValidateBytes(ctx.compiledSchema, []byte(invalidJSON))
	return nil
}

func (ctx *JSONSchemaBDDTestContext) aValidateBytesEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeValidateBytes {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("validate bytes event not found. Captured events: %v", eventTypes)
}

func (ctx *JSONSchemaBDDTestContext) aValidationSuccessEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeValidationSuccess {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("validation success event not found. Captured events: %v", eventTypes)
}

func (ctx *JSONSchemaBDDTestContext) aValidationFailedEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeValidationFailed {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("validation failed event not found. Captured events: %v", eventTypes)
}

func (ctx *JSONSchemaBDDTestContext) iValidateDataUsingTheReaderMethod() error {
	if ctx.compiledSchema == nil {
		// Create a user data schema first
		if err := ctx.iHaveACompiledSchemaForUserData(); err != nil {
			return err
		}
	}

	validJSON := `{"name": "John Doe", "age": 30}`
	reader := strings.NewReader(validJSON)
	ctx.lastError = ctx.service.ValidateReader(ctx.compiledSchema, reader)
	return nil
}

func (ctx *JSONSchemaBDDTestContext) iValidateDataUsingTheInterfaceMethod() error {
	if ctx.compiledSchema == nil {
		// Create a user data schema first
		if err := ctx.iHaveACompiledSchemaForUserData(); err != nil {
			return err
		}
	}

	userData := map[string]interface{}{
		"name": "John Doe",
		"age":  30,
	}
	ctx.lastError = ctx.service.ValidateInterface(ctx.compiledSchema, userData)
	return nil
}

func (ctx *JSONSchemaBDDTestContext) aValidateReaderEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeValidateReader {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("validate reader event not found. Captured events: %v", eventTypes)
}

func (ctx *JSONSchemaBDDTestContext) aValidateInterfaceEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeValidateInterface {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("validate interface event not found. Captured events: %v", eventTypes)
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

			// Event observation steps
			ctx.Given(`^I have a jsonschema service with event observation enabled$`, testCtx.iHaveAJSONSchemaServiceWithEventObservationEnabled)
			ctx.When(`^I compile a valid schema$`, testCtx.iCompileAValidSchema)
			ctx.Then(`^a schema compiled event should be emitted$`, testCtx.aSchemaCompiledEventShouldBeEmitted)
			ctx.Then(`^the event should contain the source information$`, testCtx.theEventShouldContainTheSourceInformation)
			ctx.Then(`^a schema error event should be emitted$`, testCtx.aSchemaErrorEventShouldBeEmitted)
			ctx.When(`^I validate valid user JSON data with bytes method$`, testCtx.iValidateValidUserJSONDataWithBytesMethod)
			ctx.When(`^I validate invalid user JSON data with bytes method$`, testCtx.iValidateInvalidUserJSONDataWithBytesMethod)
			ctx.Then(`^a validate bytes event should be emitted$`, testCtx.aValidateBytesEventShouldBeEmitted)
			ctx.Then(`^a validation success event should be emitted$`, testCtx.aValidationSuccessEventShouldBeEmitted)
			ctx.Then(`^a validation failed event should be emitted$`, testCtx.aValidationFailedEventShouldBeEmitted)
			ctx.When(`^I validate data using the reader method$`, testCtx.iValidateDataUsingTheReaderMethod)
			ctx.When(`^I validate data using the interface method$`, testCtx.iValidateDataUsingTheInterfaceMethod)
			ctx.Then(`^a validate reader event should be emitted$`, testCtx.aValidateReaderEventShouldBeEmitted)
			ctx.Then(`^a validate interface event should be emitted$`, testCtx.aValidateInterfaceEventShouldBeEmitted)
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

// Event validation step - ensures all registered events are emitted during testing
func (ctx *JSONSchemaBDDTestContext) allRegisteredEventsShouldBeEmittedDuringTesting() error {
	// Get all registered event types from the module
	registeredEvents := ctx.module.GetRegisteredEventTypes()
	
	// Create event validation observer
	validator := modular.NewEventValidationObserver("event-validator", registeredEvents)
	_ = validator // Use validator to avoid unused variable error
	
	// Check which events were emitted during testing
	emittedEvents := make(map[string]bool)
	for _, event := range ctx.eventObserver.GetEvents() {
		emittedEvents[event.Type()] = true
	}
	
	// Check for missing events
	var missingEvents []string
	for _, eventType := range registeredEvents {
		if !emittedEvents[eventType] {
			missingEvents = append(missingEvents, eventType)
		}
	}
	
	if len(missingEvents) > 0 {
		return fmt.Errorf("the following registered events were not emitted during testing: %v", missingEvents)
	}
	
	return nil
}
