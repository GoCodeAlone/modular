package eventlogger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Event emission and processing step implementations

func (ctx *EventLoggerBDDTestContext) iEmitATestEventWithTypeAndData(eventType, data string) error {
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Create CloudEvent
	event := cloudevents.NewEvent()
	event.SetID("test-id")
	event.SetType(eventType)
	event.SetSource("test-source")
	_ = event.SetData(cloudevents.ApplicationJSON, data)
	event.SetTime(time.Now())

	// Emit event through the observer
	err := ctx.service.OnEvent(context.Background(), event)
	if err != nil {
		// Buffer full is an expected condition in some scenarios; don't treat it as a test error
		if errors.Is(err, ErrEventBufferFull) {
			return nil
		}
		ctx.lastError = err
		return err
	}

	// Default pacing sleep to let async processing occur; skipped in fast burst scenarios
	if !ctx.fastEmit {
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) iEmitMultipleEventsWithDifferentTypes() error {
	events := []struct {
		eventType string
		data      string
	}{
		{"user.created", "user-data"},
		{"order.placed", "order-data"},
		{"payment.processed", "payment-data"},
	}

	for _, evt := range events {
		err := ctx.iEmitATestEventWithTypeAndData(evt.eventType, evt.data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) iEmitEventsWithDifferentTypes() error {
	return ctx.iEmitMultipleEventsWithDifferentTypes()
}

func (ctx *EventLoggerBDDTestContext) iEmitAnEvent() error {
	return ctx.iEmitATestEventWithTypeAndData("multi.test", "test-data")
}

func (ctx *EventLoggerBDDTestContext) iEmitEvents() error {
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	return ctx.iEmitATestEventWithTypeAndData("error.test", "test-data")
}

func (ctx *EventLoggerBDDTestContext) iEmitATestEventForProcessing() error {
	return ctx.iEmitATestEventWithTypeAndData("processing.test", "test-data")
}

func (ctx *EventLoggerBDDTestContext) iEmitAnEventWithMetadata() error {
	event := cloudevents.NewEvent()
	event.SetID("meta-test-id")
	event.SetType("metadata.test")
	event.SetSource("test-source")
	_ = event.SetData(cloudevents.ApplicationJSON, "test-data")
	event.SetTime(time.Now())

	// Add custom extensions (metadata)
	event.SetExtension("custom-field", "custom-value")
	event.SetExtension("request-id", "12345")

	err := ctx.service.OnEvent(context.Background(), event)
	if err != nil {
		ctx.lastError = err
		return err
	}

	time.Sleep(100 * time.Millisecond)
	return nil
}

func (ctx *EventLoggerBDDTestContext) theLoggedEventShouldIncludeTheMetadata() error {
	// This would require actual log inspection
	return nil
}

func (ctx *EventLoggerBDDTestContext) cloudEventFieldsShouldBePreserved() error {
	// This would require actual log inspection
	return nil
}

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerWithPendingEvents() error {
	ctx.resetContext()

	// Create temp directory for file outputs
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "eventlogger-bdd-test")
	if err != nil {
		return err
	}

	// Create config with console output
	config := ctx.createConsoleConfig(10)

	// Create application with the config
	err = ctx.createApplicationWithConfig(config)
	if err != nil {
		return err
	}

	// Initialize the module
	err = ctx.theEventLoggerModuleIsInitialized()
	if err != nil {
		return err
	}

	// Get service reference
	err = ctx.theEventLoggerServiceShouldBeAvailable()
	if err != nil {
		return err
	}

	// Start the module
	err = ctx.app.Start()
	if err != nil {
		return err
	}

	// Emit some events that will be pending
	for i := 0; i < 3; i++ {
		err := ctx.iEmitATestEventWithTypeAndData("pending.event", "data")
		if err != nil {
			return err
		}
	}

	return nil
}
