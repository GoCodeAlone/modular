package eventlogger

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Queue-until-ready scenario implementations

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerModuleConfiguredButNotStarted() error {
	ctx.resetContext()

	// Create temp directory for file outputs
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "eventlogger-bdd-test")
	if err != nil {
		return err
	}

	// Create config with console output and a reasonable queue size for testing
	config := ctx.createConsoleConfig(10)

	// Create application with the config
	err = ctx.createApplicationWithConfig(config)
	if err != nil {
		return err
	}

	// Initialize the module but DON'T start it yet
	err = ctx.theEventLoggerModuleIsInitialized()
	if err != nil {
		return err
	}

	// Get service reference
	err = ctx.theEventLoggerServiceShouldBeAvailable()
	if err != nil {
		return err
	}

	// Inject test console output for capturing logs
	ctx.testConsole = &testConsoleOutput{baseTestOutput: baseTestOutput{logs: make([]string, 0)}}
	ctx.service.setOutputsForTesting([]OutputTarget{ctx.testConsole})

	// Verify module is not started yet
	if ctx.service.started {
		return fmt.Errorf("module should not be started yet")
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) iEmitEventsBeforeTheEventloggerStarts() error {
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Store the events we're going to emit for later verification
	ctx.loggedEvents = make([]cloudevents.Event, 0)

	// Emit multiple test events before start
	testEvents := []struct {
		eventType string
		data      string
	}{
		{"pre.start.event1", "data1"},
		{"pre.start.event2", "data2"},
		{"pre.start.event3", "data3"},
	}

	for _, evt := range testEvents {
		event := cloudevents.NewEvent()
		event.SetID("pre-start-" + evt.data)
		event.SetType(evt.eventType)
		event.SetSource("test-source")
		_ = event.SetData(cloudevents.ApplicationJSON, evt.data)
		event.SetTime(time.Now())

		// Store for later verification
		ctx.loggedEvents = append(ctx.loggedEvents, event)

		// Emit event through the observer
		err := ctx.service.OnEvent(context.Background(), event)
		if err != nil {
			return fmt.Errorf("unexpected error during pre-start emission: %w", err)
		}
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) theEventsShouldBeQueuedWithoutErrors() error {
	// Verify the module is still not started
	if ctx.service.started {
		return fmt.Errorf("module should not be started yet")
	}

	// Verify queue has events (we'll access this through module internals for testing)
	ctx.service.mutex.Lock()
	queueLen := len(ctx.service.eventQueue)
	ctx.service.mutex.Unlock()

	if queueLen == 0 {
		return fmt.Errorf("expected queued events, but queue is empty")
	}

	// We expect at least our test events, but there may be additional framework events
	expectedMinLen := len(ctx.loggedEvents)
	if queueLen < expectedMinLen {
		return fmt.Errorf("expected at least %d queued events, got %d", expectedMinLen, queueLen)
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) theEventloggerStarts() error {
	return ctx.app.Start()
}

func (ctx *EventLoggerBDDTestContext) allQueuedEventsShouldBeProcessedAndLogged() error {
	// Wait for events to be processed
	time.Sleep(200 * time.Millisecond)

	// Verify module is started
	if !ctx.service.started {
		return fmt.Errorf("module should be started")
	}

	// Verify queue is cleared
	ctx.service.mutex.Lock()
	queueLen := len(ctx.service.eventQueue)
	ctx.service.mutex.Unlock()

	if queueLen != 0 {
		return fmt.Errorf("expected queue to be cleared after start, but has %d events", queueLen)
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) theEventsShouldBeProcessedInOrder() error {
	// Give processing time to complete
	time.Sleep(300 * time.Millisecond)

	// Get the captured logs from our test console output
	if ctx.testConsole == nil {
		return fmt.Errorf("test console output not configured")
	}

	logs := ctx.testConsole.GetLogs()
	if len(logs) == 0 {
		return fmt.Errorf("no events were logged to test console")
	}

	// Verify that the test events we emitted are present in order
	expectedEvents := []string{
		"pre.start.event1",
		"pre.start.event2",
		"pre.start.event3",
	}

	// Track first occurrence of each event to verify order
	firstOccurrence := make(map[string]int)
	for i, log := range logs {
		for _, expected := range expectedEvents {
			if containsEventType(log, expected) {
				if _, found := firstOccurrence[expected]; !found {
					firstOccurrence[expected] = i
				}
				break
			}
		}
	}

	// Verify all expected events were found
	if len(firstOccurrence) != len(expectedEvents) {
		missingEvents := make([]string, 0)
		for _, expected := range expectedEvents {
			if _, found := firstOccurrence[expected]; !found {
				missingEvents = append(missingEvents, expected)
			}
		}
		return fmt.Errorf("expected %d events to be processed, but found %d. Missing events: %v",
			len(expectedEvents), len(firstOccurrence), missingEvents)
	}

	// Verify the order matches what we expect (events should be processed in emission order)
	for i := 1; i < len(expectedEvents); i++ {
		currentEvent := expectedEvents[i]
		previousEvent := expectedEvents[i-1]

		currentPos, currentFound := firstOccurrence[currentEvent]
		previousPos, previousFound := firstOccurrence[previousEvent]

		if !currentFound || !previousFound {
			return fmt.Errorf("missing events in order check")
		}

		if currentPos <= previousPos {
			return fmt.Errorf("events not processed in expected order. %s (pos %d) should come after %s (pos %d)",
				currentEvent, currentPos, previousEvent, previousPos)
		}
	}

	return nil
}

// Queue overflow scenario implementations

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerModuleConfiguredWithQueueOverflowTesting() error {
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

	// Initialize the module but DON'T start it yet
	err = ctx.theEventLoggerModuleIsInitialized()
	if err != nil {
		return err
	}

	// Get service reference
	err = ctx.theEventLoggerServiceShouldBeAvailable()
	if err != nil {
		return err
	}

	// Inject test console output for capturing logs
	ctx.testConsole = &testConsoleOutput{baseTestOutput: baseTestOutput{logs: make([]string, 0)}}
	ctx.service.setOutputsForTesting([]OutputTarget{ctx.testConsole})

	// Artificially reduce queue size for testing overflow
	ctx.service.mutex.Lock()
	ctx.service.queueMaxSize = 3 // Small queue for testing overflow
	ctx.service.mutex.Unlock()

	return nil
}

func (ctx *EventLoggerBDDTestContext) iEmitMoreEventsThanTheQueueCanHoldBeforeStart() error {
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Store the events we're going to emit for later verification
	ctx.loggedEvents = make([]cloudevents.Event, 0)

	// Emit more events than the queue can hold (queue size is 3)
	for i := 0; i < 6; i++ {
		event := cloudevents.NewEvent()
		event.SetID(fmt.Sprintf("overflow-test-%d", i))
		event.SetType(fmt.Sprintf("queue.overflow.event%d", i))
		event.SetSource("test-source")
		_ = event.SetData(cloudevents.ApplicationJSON, fmt.Sprintf("data%d", i))
		event.SetTime(time.Now())

		// Store for later verification
		ctx.loggedEvents = append(ctx.loggedEvents, event)

		// Emit event through the observer
		err := ctx.service.OnEvent(context.Background(), event)
		if err != nil {
			return fmt.Errorf("unexpected error during overflow emission: %w", err)
		}
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) olderEventsShouldBeDroppedFromTheQueue() error {
	// Verify queue is at max size
	ctx.service.mutex.Lock()
	queueLen := len(ctx.service.eventQueue)
	maxSize := ctx.service.queueMaxSize
	ctx.service.mutex.Unlock()

	if queueLen != maxSize {
		return fmt.Errorf("expected queue length to be %d (max size), got %d", maxSize, queueLen)
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) newerEventsShouldBePreservedInTheQueue() error {
	// Verify queue has events (already checked in previous step)
	ctx.service.mutex.Lock()
	queueLen := len(ctx.service.eventQueue)
	ctx.service.mutex.Unlock()

	if queueLen == 0 {
		return fmt.Errorf("expected preserved events in queue, but queue is empty")
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) onlyThePreservedEventsShouldBeProcessed() error {
	// Wait longer for events to be processed with polling for queue clearance
	maxWait := 2 * time.Second
	checkInterval := 50 * time.Millisecond
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		ctx.service.mutex.Lock()
		queueLen := len(ctx.service.eventQueue)
		ctx.service.mutex.Unlock()

		if queueLen == 0 {
			break
		}
		time.Sleep(checkInterval)
	}

	// Final verification that queue is cleared
	ctx.service.mutex.Lock()
	queueLen := len(ctx.service.eventQueue)
	ctx.service.mutex.Unlock()

	if queueLen != 0 {
		return fmt.Errorf("expected queue to be cleared after start, but has %d events after %v", queueLen, maxWait)
	}

	// Additional wait for logs to be written to test console
	time.Sleep(200 * time.Millisecond)

	// Get the captured logs from our test console output
	if ctx.testConsole == nil {
		return fmt.Errorf("test console output not configured")
	}

	logs := ctx.testConsole.GetLogs()
	if len(logs) == 0 {
		return fmt.Errorf("no events were logged to test console")
	}

	// In the overflow scenario, we emit events 0-5 to overflow the queue (max 3)
	// With queue size 3, we expect to see the last 3 events (3, 4, 5) preserved
	// and the first 3 events (0, 1, 2) should be dropped
	preservedEvents := []string{"queue.overflow.event3", "queue.overflow.event4", "queue.overflow.event5"}
	droppedEvents := []string{"queue.overflow.event0", "queue.overflow.event1", "queue.overflow.event2"}

	// Check that preserved events are present with better string matching
	foundPreserved := make([]bool, len(preservedEvents))
	for i, expected := range preservedEvents {
		for _, log := range logs {
			if containsEventType(log, expected) {
				foundPreserved[i] = true
				break
			}
		}
	}

	// Provide detailed debugging information if events are missing
	for i, expected := range preservedEvents {
		if !foundPreserved[i] {
			// Create debug information about what we found vs expected
			var foundEventTypes []string
			for _, log := range logs {
				// Extract event type from log line format: [timestamp] LEVEL TYPE
				if strings.Contains(log, "queue.overflow.event") {
					foundEventTypes = append(foundEventTypes, extractEventTypeFromLog(log))
				}
			}
			return fmt.Errorf("expected preserved event %s not found in logs. Found queue overflow events: %v. Total logs: %d",
				expected, foundEventTypes, len(logs))
		}
	}

	// Check that dropped events are NOT present
	for _, dropped := range droppedEvents {
		for _, log := range logs {
			if containsEventType(log, dropped) {
				return fmt.Errorf("found dropped event %s in logs, but it should have been dropped due to overflow", dropped)
			}
		}
	}

	return nil
}
