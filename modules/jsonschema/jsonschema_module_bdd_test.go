package jsonschema

import (
	"context"
	"sync"
	"testing"

	"github.com/GoCodeAlone/modular"
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

// Shared utilities and test context structures

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
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
