package eventlogger

import (
	"fmt"
	"os"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Event observation setup and step implementations

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerWithEventObservationEnabled() error {
	ctx.resetContext()

	// Create temp directory for file outputs
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "eventlogger-bdd-test")
	if err != nil {
		return err
	}

	// Create config with console output for event observation
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
	} else {
		return fmt.Errorf("service is not an EventLoggerModule")
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) aLoggerStartedEventShouldBeEmitted() error {
	// Poll for the started event to tolerate scheduling jitter of the async startup emitter.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		events := ctx.eventObserver.GetEvents()
		for _, event := range events {
			if event.Type() == EventTypeLoggerStarted {
				return nil
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	// One final capture for diagnostics
	events := ctx.eventObserver.GetEvents()
	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}
	return fmt.Errorf("event of type %s was not emitted within timeout. Captured events: %v", EventTypeLoggerStarted, eventTypes)
}

func (ctx *EventLoggerBDDTestContext) theEventLoggerModuleStops() error {
	return ctx.app.Stop()
}

func (ctx *EventLoggerBDDTestContext) aLoggerStoppedEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Allow time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeLoggerStopped {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeLoggerStopped, eventTypes)
}

func (ctx *EventLoggerBDDTestContext) theEventShouldContainOutputCountAndBufferSize() error {
	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeLoggerStarted {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to extract event data: %v", err)
			}

			// Check for output_count
			if _, exists := data["output_count"]; !exists {
				return fmt.Errorf("logger started event should contain output_count")
			}

			// Check for buffer_size
			if _, exists := data["buffer_size"]; !exists {
				return fmt.Errorf("logger started event should contain buffer_size")
			}

			return nil
		}
	}
	return fmt.Errorf("logger started event not found")
}

func (ctx *EventLoggerBDDTestContext) aConfigLoadedEventShouldBeEmitted() error {
	time.Sleep(200 * time.Millisecond) // Allow more time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeConfigLoaded {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeConfigLoaded, eventTypes)
}

func (ctx *EventLoggerBDDTestContext) outputRegisteredEventsShouldBeEmittedForEachTarget() error {
	time.Sleep(200 * time.Millisecond) // Allow more time for async event emission

	events := ctx.eventObserver.GetEvents()
	outputRegisteredCount := 0

	for _, event := range events {
		if event.Type() == EventTypeOutputRegistered {
			outputRegisteredCount++
		}
	}

	// Should have one output registered event for each target
	expectedCount := len(ctx.service.outputs)
	if outputRegisteredCount != expectedCount {
		// Debug: show all event types to help diagnose
		eventTypes := make([]string, len(events))
		for i, event := range events {
			eventTypes[i] = event.Type()
		}
		return fmt.Errorf("expected %d output registered events, got %d. Captured events: %v", expectedCount, outputRegisteredCount, eventTypes)
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) theEventsShouldContainConfigurationDetails() error {
	events := ctx.eventObserver.GetEvents()

	// Check config loaded event has configuration details
	for _, event := range events {
		if event.Type() == EventTypeConfigLoaded {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to extract config loaded event data: %v", err)
			}

			// Check for key configuration fields
			if _, exists := data["enabled"]; !exists {
				return fmt.Errorf("config loaded event should contain enabled field")
			}
			if _, exists := data["buffer_size"]; !exists {
				return fmt.Errorf("config loaded event should contain buffer_size field")
			}

			return nil
		}
	}

	return fmt.Errorf("config loaded event not found")
}

func (ctx *EventLoggerBDDTestContext) anEventReceivedEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Allow time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeEventReceived {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeEventReceived, eventTypes)
}

func (ctx *EventLoggerBDDTestContext) anEventProcessedEventShouldBeEmitted() error {
	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeEventProcessed {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeEventProcessed, eventTypes)
}

func (ctx *EventLoggerBDDTestContext) anOutputSuccessEventShouldBeEmitted() error {
	time.Sleep(300 * time.Millisecond) // Allow more time for async processing and event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeOutputSuccess {
			return nil
		}
	}

	// Debug: show all event types to help diagnose
	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeOutputSuccess, eventTypes)
}
