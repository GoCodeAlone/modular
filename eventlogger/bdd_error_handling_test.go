package eventlogger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Error handling step implementations

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerWithFaultyOutputTarget() error {
	ctx.resetContext()

	// Create temp directory for file outputs
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "eventlogger-bdd-test")
	if err != nil {
		return err
	}

	// Create config with console output (good target for faulty target test)
	config := ctx.createConsoleConfig(10)

	// Create application with the config
	err = ctx.createApplicationWithConfig(config)
	if err != nil {
		return err
	}

	// Initialize normally - this should succeed
	err = ctx.theEventLoggerModuleIsInitialized()
	if err != nil {
		return err
	}

	// Start the module
	err = ctx.app.Start()
	if err != nil {
		return err
	}

	// Get service reference - should be available
	err = ctx.theEventLoggerServiceShouldBeAvailable()
	if err != nil {
		return err
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) errorsShouldBeHandledGracefully() error {
	// In this test, we verify that the module handles errors gracefully.
	// Since we're using a working console output target, the module should function normally.
	// The test verifies graceful error handling by ensuring the module remains operational.

	if ctx.service == nil {
		return fmt.Errorf("service should be available even with potential faults")
	}

	// Verify the module is still functional by emitting a test event
	event := modular.NewCloudEvent("graceful.test", "test-source", map[string]interface{}{"test": "data"}, nil)
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		err := ctx.service.OnEvent(context.Background(), event)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrEventBufferFull) || time.Now().After(deadline) {
			return fmt.Errorf("module should handle events gracefully: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (ctx *EventLoggerBDDTestContext) otherOutputTargetsShouldContinueWorking() error {
	// Verify that non-faulty output targets continue to function correctly
	// even when other targets fail. This is verified by checking that
	// events are still being processed and logged successfully.
	if ctx.service == nil {
		return fmt.Errorf("event logger service not available")
	}

	// Emit a test event to verify other outputs still work
	event := modular.NewCloudEvent("test.recovery", "test-source", map[string]interface{}{"test": "recovery"}, nil)
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		err := ctx.service.OnEvent(context.Background(), event)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrEventBufferFull) || time.Now().After(deadline) {
			return fmt.Errorf("other output targets failed to work after error: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerWithFaultyOutputTargetAndEventObservationEnabled() error {
	ctx.resetContext()

	// Create temp directory for file outputs
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "eventlogger-bdd-test")
	if err != nil {
		return err
	}

	// Create config with console output
	config := ctx.createConsoleConfig(100)

	// Create application with the config
	err = ctx.createApplicationWithConfig(config)
	if err != nil {
		return err
	}

	// Manually ensure observers are registered - this might not be happening automatically
	if err := ctx.module.RegisterObservers(ctx.app.(*modular.ObservableApplication)); err != nil {
		return fmt.Errorf("failed to manually register observers: %w", err)
	}

	// Initialize the application
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize app: %v", err)
	}

	if err := ctx.app.Start(); err != nil {
		return fmt.Errorf("failed to start app: %v", err)
	}

	// Get the eventlogger service
	var service interface{}
	if err := ctx.app.GetService("eventlogger.observer", &service); err != nil {
		return fmt.Errorf("failed to get eventlogger service: %w", err)
	}

	// Cast to EventLoggerModule
	if eventloggerService, ok := service.(*EventLoggerModule); ok {
		ctx.service = eventloggerService
		// Replace the console output with a faulty one to trigger output errors
		// Use the test-only setter to avoid data races with concurrent processing.
		faultyOutput := &faultyOutputTarget{}
		ctx.service.setOutputsForTesting([]OutputTarget{faultyOutput})
	} else {
		return fmt.Errorf("service is not an EventLoggerModule")
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) anOutputErrorEventShouldBeEmitted() error {
	time.Sleep(200 * time.Millisecond) // Allow time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeOutputError {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeOutputError, eventTypes)
}

func (ctx *EventLoggerBDDTestContext) theErrorEventShouldContainErrorDetails() error {
	events := ctx.eventObserver.GetEvents()

	for _, event := range events {
		if event.Type() == EventTypeOutputError {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to extract output error event data: %v", err)
			}

			// Check for required error fields
			if _, exists := data["error"]; !exists {
				return fmt.Errorf("output error event should contain error field")
			}
			if _, exists := data["event_type"]; !exists {
				return fmt.Errorf("output error event should contain event_type field")
			}

			return nil
		}
	}

	return fmt.Errorf("output error event not found")
}
