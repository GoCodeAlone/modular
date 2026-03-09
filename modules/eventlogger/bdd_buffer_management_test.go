package eventlogger

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Buffer management step implementations

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerWithBufferSizeConfigured() error {
	ctx.resetContext()

	// Create temp directory for file outputs
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "eventlogger-bdd-test")
	if err != nil {
		return err
	}

	// Create config with small buffer size for testing
	config := ctx.createConsoleConfig(3) // Small buffer for testing

	// Create application with the config
	err = ctx.createApplicationWithConfig(config)
	if err != nil {
		return err
	}

	err = ctx.theEventLoggerModuleIsInitialized()
	if err != nil {
		return err
	}

	err = ctx.theEventLoggerServiceShouldBeAvailable()
	if err != nil {
		return err
	}

	return ctx.app.Start()
}

func (ctx *EventLoggerBDDTestContext) iEmitMoreEventsThanTheBufferCanHold() error {
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}
	// Enable fast emission mode to skip per-event sleeps elsewhere
	ctx.fastEmit = true
	for i := 0; i < 50; i++ { // burst size large enough to overflow small buffers
		e := cloudevents.NewEvent()
		e.SetID("overflow-" + fmt.Sprint(i))
		e.SetType(fmt.Sprintf("buffer.test.%d", i))
		e.SetSource("test-source")
		e.SetTime(time.Now())
		_ = ctx.service.OnEvent(context.Background(), e)
	}
	// Allow time for processing and operational events (buffer full / dropped) to be emitted synchronously
	time.Sleep(150 * time.Millisecond)
	return nil
}

// iRapidlyEmitMoreEventsThanTheBufferCanHold emits a large burst of events without per-event
// sleeping to intentionally overflow the buffer and trigger buffer full / dropped events.
func (ctx *EventLoggerBDDTestContext) iRapidlyEmitMoreEventsThanTheBufferCanHold() error {
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}
	// Emit a burst concurrently to maximize instantaneous pressure on the small buffer.
	total := 500
	var wg sync.WaitGroup
	wg.Add(total)
	for i := 0; i < total; i++ {
		i := i
		go func() {
			defer wg.Done()
			event := cloudevents.NewEvent()
			event.SetID("test-id")
			event.SetType(fmt.Sprintf("buffer.test.%d", i))
			event.SetSource("test-source")
			_ = event.SetData(cloudevents.ApplicationJSON, "data")
			event.SetTime(time.Now())
			_ = ctx.service.OnEvent(context.Background(), event)
		}()
	}
	wg.Wait()
	// Allow brief time for operational events emission
	time.Sleep(200 * time.Millisecond)
	return nil
}

func (ctx *EventLoggerBDDTestContext) olderEventsShouldBeDropped() error {
	// Buffer overflow should be handled - check no errors occurred
	return ctx.lastError
}

func (ctx *EventLoggerBDDTestContext) bufferOverflowShouldBeHandledGracefully() error {
	// Verify module is still operational
	if ctx.service == nil || !ctx.service.started {
		return fmt.Errorf("service not operational")
	}
	return nil
}

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerWithSmallBufferAndEventObservationEnabled() error {
	ctx.resetContext()

	// Create temp directory for file outputs
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "eventlogger-bdd-test")
	if err != nil {
		return err
	}

	// Create config with very small buffer for buffer overflow testing
	config := ctx.createConsoleConfig(1) // Buffer size 1 to force rapid saturation
	ctx.fastEmit = true                  // Enable burst emission to increase likelihood of overflow

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

func (ctx *EventLoggerBDDTestContext) bufferFullEventsShouldBeEmitted() error {
	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeBufferFull {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeBufferFull, eventTypes)
}

func (ctx *EventLoggerBDDTestContext) eventDroppedEventsShouldBeEmitted() error {
	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeEventDropped {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeEventDropped, eventTypes)
}

func (ctx *EventLoggerBDDTestContext) theEventsShouldContainDropReasons() error {
	events := ctx.eventObserver.GetEvents()

	// Check event dropped events contain drop reasons
	for _, event := range events {
		if event.Type() == EventTypeEventDropped {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to extract event dropped event data: %v", err)
			}

			// Check for drop reason
			if _, exists := data["reason"]; !exists {
				return fmt.Errorf("event dropped event should contain reason field")
			}

			return nil
		}
	}

	return fmt.Errorf("event dropped event not found")
}
