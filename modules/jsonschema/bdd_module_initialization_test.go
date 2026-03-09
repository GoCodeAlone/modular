package jsonschema

import (
	"fmt"

	"github.com/GoCodeAlone/modular"
)

// Core module initialization and setup step methods

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
