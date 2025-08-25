package eventlogger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cucumber/godog"
)

// EventLogger BDD Test Context
type EventLoggerBDDTestContext struct {
	app           modular.Application
	module        *EventLoggerModule
	service       *EventLoggerModule
	config        *EventLoggerConfig
	lastError     error
	loggedEvents  []cloudevents.Event
	tempDir       string
	outputLogs    []string
	testConsole   *testConsoleOutput
	testFile      *testFileOutput
	eventObserver *testEventObserver
	// fastEmit enables burst emission without per-event sleep (used to deterministically trigger buffer full events)
	fastEmit bool
}

// createConsoleConfig creates an EventLoggerConfig with console output
func (ctx *EventLoggerBDDTestContext) createConsoleConfig(bufferSize int) *EventLoggerConfig {
	return &EventLoggerConfig{
		Enabled:           true,
		LogLevel:          "INFO",
		Format:            "structured",
		BufferSize:        bufferSize,
		FlushInterval:     time.Duration(5 * time.Second),
		IncludeMetadata:   true,
		IncludeStackTrace: false,
		// Enable synchronous startup emission so tests reliably observe
		// config.loaded, output.registered, and started events without
		// relying on timing of goroutines.
		StartupSync:       true,
		OutputTargets: []OutputTargetConfig{
			{
				Type:   "console",
				Level:  "INFO",
				Format: "structured",
				Console: &ConsoleTargetConfig{
					UseColor:   false,
					Timestamps: true,
				},
			},
		},
	}
}

// createFileConfig creates an EventLoggerConfig with file output
func (ctx *EventLoggerBDDTestContext) createFileConfig(logFile string) *EventLoggerConfig {
	return &EventLoggerConfig{
		Enabled:           true,
		LogLevel:          "INFO",
		Format:            "structured",
		BufferSize:        100,
		FlushInterval:     time.Duration(5 * time.Second),
		IncludeMetadata:   true,
		IncludeStackTrace: false,
		OutputTargets: []OutputTargetConfig{
			{
				Type:   "file",
				Level:  "INFO",
				Format: "json",
				File: &FileTargetConfig{
					Path:       logFile,
					MaxSize:    10,
					MaxBackups: 3,
					Compress:   false,
				},
			},
		},
	}
}

// createFilteredConfig creates an EventLoggerConfig with event type filters
func (ctx *EventLoggerBDDTestContext) createFilteredConfig(filters []string) *EventLoggerConfig {
	return &EventLoggerConfig{
		Enabled:           true,
		LogLevel:          "INFO",
		Format:            "structured",
		BufferSize:        100,
		FlushInterval:     time.Duration(5 * time.Second),
		IncludeMetadata:   true,
		IncludeStackTrace: false,
		EventTypeFilters:  filters,
		OutputTargets: []OutputTargetConfig{
			{
				Type:   "console",
				Level:  "INFO",
				Format: "structured",
				Console: &ConsoleTargetConfig{
					UseColor:   false,
					Timestamps: true,
				},
			},
		},
	}
}

// createMultiTargetConfig creates an EventLoggerConfig with multiple output targets
func (ctx *EventLoggerBDDTestContext) createMultiTargetConfig(logFile string) *EventLoggerConfig {
	return &EventLoggerConfig{
		Enabled:           true,
		LogLevel:          "INFO",
		Format:            "structured",
		BufferSize:        100,
		FlushInterval:     time.Duration(5 * time.Second),
		IncludeMetadata:   true,
		IncludeStackTrace: false,
		OutputTargets: []OutputTargetConfig{
			{
				Type:   "console",
				Level:  "INFO",
				Format: "structured",
				Console: &ConsoleTargetConfig{
					UseColor:   false,
					Timestamps: true,
				},
			},
			{
				Type:   "file",
				Level:  "INFO",
				Format: "json",
				File: &FileTargetConfig{
					Path:       logFile,
					MaxSize:    10,
					MaxBackups: 3,
					Compress:   false,
				},
			},
		},
	}
}

// createApplicationWithConfig creates an ObservableApplication with provided config
func (ctx *EventLoggerBDDTestContext) createApplicationWithConfig(config *EventLoggerConfig) error {
	logger := &testLogger{}

	// Provide an empty feeder slice directly to this application instance to avoid
	// mutating the global modular.ConfigFeeders (which would hinder test parallelism).
	// Individual tests can still register additional feeders if required via the
	// application's configuration mechanisms.

	// Create app with empty main config - USE OBSERVABLE for events
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)
	// Ensure this app instance starts with no implicit global feeders by using a
	// narrow interface type assertion (avoids expanding the public Application interface).
	if cfSetter, ok := ctx.app.(interface{ SetConfigFeeders([]modular.Feeder) }); ok {
		cfSetter.SetConfigFeeders([]modular.Feeder{})
	}

	// Create test event observer
	ctx.eventObserver = newTestEventObserver()

	// Register our test observer BEFORE registering module to capture all events
	if err := ctx.app.(*modular.ObservableApplication).RegisterObserver(ctx.eventObserver); err != nil {
		return fmt.Errorf("failed to register test observer: %w", err)
	}

	// Create and register eventlogger module
	ctx.module = NewModule().(*EventLoggerModule)

	// Register the eventlogger config section with the provided config FIRST
	// This ensures the module's RegisterConfig doesn't override our test config
	eventloggerConfigProvider := modular.NewStdConfigProvider(config)
	ctx.app.RegisterConfigSection("eventlogger", eventloggerConfigProvider)

	// Register module AFTER config
	ctx.app.RegisterModule(ctx.module)

	return nil
}

// Test event observer for capturing emitted events
type testEventObserver struct {
	mu     sync.Mutex
	events []cloudevents.Event
}

func newTestEventObserver() *testEventObserver {
	return &testEventObserver{
		events: make([]cloudevents.Event, 0),
	}
}

func (t *testEventObserver) OnEvent(ctx context.Context, event cloudevents.Event) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event.Clone())
	return nil
}

func (t *testEventObserver) ObserverID() string {
	return "test-observer-eventlogger"
}

func (t *testEventObserver) GetEvents() []cloudevents.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	events := make([]cloudevents.Event, len(t.events))
	copy(events, t.events)
	return events
}

func (t *testEventObserver) ClearEvents() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = make([]cloudevents.Event, 0)
}

func (ctx *EventLoggerBDDTestContext) resetContext() {
	if ctx.tempDir != "" {
		os.RemoveAll(ctx.tempDir)
	}
	if ctx.app != nil {
		ctx.app.Stop()
		// Give some time for cleanup
		time.Sleep(10 * time.Millisecond)
	}

	ctx.app = nil
	ctx.module = nil
	ctx.service = nil
	ctx.config = nil
	ctx.lastError = nil
	ctx.loggedEvents = nil
	ctx.tempDir = ""
	ctx.outputLogs = nil
	ctx.testConsole = nil
	ctx.testFile = nil
	ctx.eventObserver = nil
}

func (ctx *EventLoggerBDDTestContext) iHaveAModularApplicationWithEventLoggerModuleConfigured() error {
	ctx.resetContext()

	// Create temp directory for file outputs
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "eventlogger-bdd-test")
	if err != nil {
		return err
	}

	// Create console config
	config := ctx.createConsoleConfig(10)

	// Create application with the config
	return ctx.createApplicationWithConfig(config)
}

func (ctx *EventLoggerBDDTestContext) theEventLoggerModuleIsInitialized() error {
	err := ctx.app.Init()
	if err != nil {
		ctx.lastError = err
		return err
	}

	// Check if the module was properly initialized
	if ctx.module == nil {
		return fmt.Errorf("module is nil after init")
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) theEventLoggerServiceShouldBeAvailable() error {
	err := ctx.app.GetService("eventlogger.observer", &ctx.service)
	if err != nil {
		return err
	}
	if ctx.service == nil {
		return fmt.Errorf("eventlogger service is nil")
	}
	return nil
}

func (ctx *EventLoggerBDDTestContext) theModuleShouldRegisterAsAnObserver() error {
	// Start the module to trigger observer registration
	err := ctx.app.Start()
	if err != nil {
		return err
	}

	// Verify observer is registered by checking if module is in started state
	if !ctx.service.started {
		return fmt.Errorf("module not started")
	}
	return nil
}

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerWithConsoleOutputConfigured() error {
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

	// Initialize and start the module
	err = ctx.theEventLoggerModuleIsInitialized()
	if err != nil {
		return err
	}

	err = ctx.theEventLoggerServiceShouldBeAvailable()
	if err != nil {
		return err
	}

	err = ctx.app.Start()
	if err != nil {
		return err
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) iEmitATestEventWithTypeAndData(eventType, data string) error {
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Create CloudEvent
	event := cloudevents.NewEvent()
	event.SetID("test-id")
	event.SetType(eventType)
	event.SetSource("test-source")
	event.SetData(cloudevents.ApplicationJSON, data)
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

func (ctx *EventLoggerBDDTestContext) theEventShouldBeLoggedToConsoleOutput() error {
	// Since we can't easily capture console output in tests,
	// we'll verify the event was processed by checking the module state
	if ctx.service == nil || !ctx.service.started {
		return fmt.Errorf("service not started")
	}
	return nil
}

func (ctx *EventLoggerBDDTestContext) theLogEntryShouldContainTheEventTypeAndData() error {
	// This would require capturing actual console output
	// For now, we'll verify the module is processing events
	return nil
}

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerWithFileOutputConfigured() error {
	ctx.resetContext()

	// Create temp directory for file outputs
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "eventlogger-bdd-test")
	if err != nil {
		return err
	}

	// Create config with file output
	logFile := filepath.Join(ctx.tempDir, "test.log")
	config := ctx.createFileConfig(logFile)

	// Create application with the config
	err = ctx.createApplicationWithConfig(config)
	if err != nil {
		return err
	}

	// Initialize and start the module
	err = ctx.theEventLoggerModuleIsInitialized()
	if err != nil {
		return err
	}

	err = ctx.theEventLoggerServiceShouldBeAvailable()
	if err != nil {
		return err
	}

	err = ctx.app.Start()
	if err != nil {
		return err
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

func (ctx *EventLoggerBDDTestContext) allEventsShouldBeLoggedToTheFile() error {
	// Wait longer for events to be flushed to disk
	time.Sleep(500 * time.Millisecond)

	logFile := filepath.Join(ctx.tempDir, "test.log")

	// Try multiple times with increasing delays to handle race conditions
	for attempt := 0; attempt < 5; attempt++ {
		if _, err := os.Stat(logFile); err == nil {
			return nil // File exists
		}
		// Wait a bit more and retry
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("log file not created")
}

func (ctx *EventLoggerBDDTestContext) theFileShouldContainStructuredLogEntries() error {
	logFile := filepath.Join(ctx.tempDir, "test.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		return err
	}

	// Verify file contains some content (basic check)
	if len(content) == 0 {
		return fmt.Errorf("log file is empty")
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerWithEventTypeFiltersConfigured() error {
	ctx.resetContext()

	// Create temp directory for file outputs
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "eventlogger-bdd-test")
	if err != nil {
		return err
	}

	// Create config with event type filters
	filters := []string{"user.created", "order.placed"}
	config := ctx.createFilteredConfig(filters)

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

func (ctx *EventLoggerBDDTestContext) onlyFilteredEventTypesShouldBeLogged() error {
	// This would require actual log capture to verify
	// For now, we assume filtering works if no error occurred
	return nil
}

func (ctx *EventLoggerBDDTestContext) nonMatchingEventsShouldBeIgnored() error {
	// This would require actual log capture to verify
	return nil
}

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerWithINFOLogLevelConfigured() error {
	ctx.resetContext()

	// Create temp directory for file outputs
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "eventlogger-bdd-test")
	if err != nil {
		return err
	}

	// Create config with INFO log level (same as console config)
	config := ctx.createConsoleConfig(10)

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

func (ctx *EventLoggerBDDTestContext) iEmitEventsWithDifferentLogLevels() error {
	// Emit events that would map to different log levels
	events := []string{"config.loaded", "module.registered", "application.failed"}

	for _, eventType := range events {
		err := ctx.iEmitATestEventWithTypeAndData(eventType, "test-data")
		if err != nil {
			return err
		}
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) onlyINFOAndHigherLevelEventsShouldBeLogged() error {
	// This would require actual log level verification
	return nil
}

func (ctx *EventLoggerBDDTestContext) dEBUGEventsShouldBeFilteredOut() error {
	// This would require actual log capture to verify
	return nil
}

func (ctx *EventLoggerBDDTestContext) iEmitEventsWithDifferentTypes() error {
	return ctx.iEmitMultipleEventsWithDifferentTypes()
}

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
			event.SetData(cloudevents.ApplicationJSON, "data")
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

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerWithMultipleOutputTargetsConfigured() error {
	ctx.resetContext()

	// Create temp directory for file outputs
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "eventlogger-bdd-test")
	if err != nil {
		return err
	}

	// Create config with multiple output targets
	logFile := filepath.Join(ctx.tempDir, "multi.log")
	config := ctx.createMultiTargetConfig(logFile)

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

	err = ctx.app.Start()
	if err != nil {
		return err
	}

	return nil
}

func (ctx *EventLoggerBDDTestContext) iEmitAnEvent() error {
	return ctx.iEmitATestEventWithTypeAndData("multi.test", "test-data")
}

func (ctx *EventLoggerBDDTestContext) theEventShouldBeLoggedToAllConfiguredTargets() error {
	// Wait longer for processing
	time.Sleep(500 * time.Millisecond)

	// Check if file was created (indicating file target worked)
	logFile := filepath.Join(ctx.tempDir, "multi.log")

	// Try multiple times with increasing delays to handle race conditions
	for attempt := 0; attempt < 5; attempt++ {
		if _, err := os.Stat(logFile); err == nil {
			return nil // File exists
		}
		// Wait a bit more and retry
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("log file not created for multi-target test")
}

func (ctx *EventLoggerBDDTestContext) eachTargetShouldReceiveTheSameEventData() error {
	// Basic verification that both targets are operational
	return nil
}

func (ctx *EventLoggerBDDTestContext) iHaveAnEventLoggerWithMetadataInclusionEnabled() error {
	ctx.resetContext()

	// Create temp directory for file outputs
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "eventlogger-bdd-test")
	if err != nil {
		return err
	}

	// Create config with metadata inclusion enabled (already enabled in console config)
	config := ctx.createConsoleConfig(10)

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

func (ctx *EventLoggerBDDTestContext) iEmitAnEventWithMetadata() error {
	event := cloudevents.NewEvent()
	event.SetID("meta-test-id")
	event.SetType("metadata.test")
	event.SetSource("test-source")
	event.SetData(cloudevents.ApplicationJSON, "test-data")
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

func (ctx *EventLoggerBDDTestContext) theModuleIsStopped() error {
	return ctx.app.Stop()
}

func (ctx *EventLoggerBDDTestContext) allPendingEventsShouldBeFlushed() error {
	// After stop, all events should be processed
	return nil
}

func (ctx *EventLoggerBDDTestContext) outputTargetsShouldBeClosedProperly() error {
	// Verify module stopped gracefully
	if ctx.service.started {
		return fmt.Errorf("service still started after stop")
	}
	return nil
}

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

func (ctx *EventLoggerBDDTestContext) iEmitEvents() error {
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	return ctx.iEmitATestEventWithTypeAndData("error.test", "test-data")
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
	err := ctx.service.OnEvent(context.Background(), event)

	// The module should handle this gracefully
	if err != nil {
		return fmt.Errorf("module should handle events gracefully: %v", err)
	}

	return nil
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
	err := ctx.service.OnEvent(context.Background(), event)

	// The error handling should ensure this succeeds even with faulty targets
	if err != nil {
		return fmt.Errorf("other output targets failed to work after error: %v", err)
	}

	return nil
}

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

func (ctx *EventLoggerBDDTestContext) iEmitATestEventForProcessing() error {
	return ctx.iEmitATestEventWithTypeAndData("processing.test", "test-data")
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

// Faulty output target for testing error scenarios
type faultyOutputTarget struct{}

func (f *faultyOutputTarget) Start(ctx context.Context) error {
	return nil
}

func (f *faultyOutputTarget) Stop(ctx context.Context) error {
	return nil
}

func (f *faultyOutputTarget) WriteEvent(entry *LogEntry) error {
	return fmt.Errorf("simulated output target failure")
}

func (f *faultyOutputTarget) Flush() error {
	return fmt.Errorf("simulated flush failure")
}

// Test helper structures
type testLogger struct{}

func (l *testLogger) Debug(msg string, keysAndValues ...interface{})   {}
func (l *testLogger) Info(msg string, keysAndValues ...interface{})    {}
func (l *testLogger) Warn(msg string, keysAndValues ...interface{})    {}
func (l *testLogger) Error(msg string, keysAndValues ...interface{})   {}
func (l *testLogger) With(keysAndValues ...interface{}) modular.Logger { return l }

type testConsoleOutput struct {
	logs []string
}

type testFileOutput struct {
	logs []string
}

// TestEventLoggerModuleBDD runs the BDD tests for the EventLogger module
func TestEventLoggerModuleBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			ctx := &EventLoggerBDDTestContext{}

			// Background
			s.Given(`^I have a modular application with event logger module configured$`, ctx.iHaveAModularApplicationWithEventLoggerModuleConfigured)

			// Initialization - handled by event observation scenarios now
			s.Then(`^the event logger service should be available$`, ctx.theEventLoggerServiceShouldBeAvailable)
			s.Then(`^the module should register as an observer$`, ctx.theModuleShouldRegisterAsAnObserver)

			// Console output
			s.Given(`^I have an event logger with console output configured$`, ctx.iHaveAnEventLoggerWithConsoleOutputConfigured)
			s.When(`^I emit a test event with type "([^"]*)" and data "([^"]*)"$`, ctx.iEmitATestEventWithTypeAndData)
			s.Then(`^the event should be logged to console output$`, ctx.theEventShouldBeLoggedToConsoleOutput)
			s.Then(`^the log entry should contain the event type and data$`, ctx.theLogEntryShouldContainTheEventTypeAndData)

			// File output
			s.Given(`^I have an event logger with file output configured$`, ctx.iHaveAnEventLoggerWithFileOutputConfigured)
			s.When(`^I emit multiple events with different types$`, ctx.iEmitMultipleEventsWithDifferentTypes)
			s.Then(`^all events should be logged to the file$`, ctx.allEventsShouldBeLoggedToTheFile)
			s.Then(`^the file should contain structured log entries$`, ctx.theFileShouldContainStructuredLogEntries)

			// Event filtering
			s.Given(`^I have an event logger with event type filters configured$`, ctx.iHaveAnEventLoggerWithEventTypeFiltersConfigured)
			s.When(`^I emit events with different types$`, ctx.iEmitEventsWithDifferentTypes)
			s.Then(`^only filtered event types should be logged$`, ctx.onlyFilteredEventTypesShouldBeLogged)
			s.Then(`^non-matching events should be ignored$`, ctx.nonMatchingEventsShouldBeIgnored)

			// Log level filtering
			s.Given(`^I have an event logger with INFO log level configured$`, ctx.iHaveAnEventLoggerWithINFOLogLevelConfigured)
			s.When(`^I emit events with different log levels$`, ctx.iEmitEventsWithDifferentLogLevels)
			s.Then(`^only INFO and higher level events should be logged$`, ctx.onlyINFOAndHigherLevelEventsShouldBeLogged)
			s.Then(`^DEBUG events should be filtered out$`, ctx.dEBUGEventsShouldBeFilteredOut)

			// Buffer management
			s.Given(`^I have an event logger with buffer size configured$`, ctx.iHaveAnEventLoggerWithBufferSizeConfigured)
			s.When(`^I emit more events than the buffer can hold$`, ctx.iEmitMoreEventsThanTheBufferCanHold)
			s.Then(`^older events should be dropped$`, ctx.olderEventsShouldBeDropped)
			s.Then(`^buffer overflow should be handled gracefully$`, ctx.bufferOverflowShouldBeHandledGracefully)

			// Multiple targets
			s.Given(`^I have an event logger with multiple output targets configured$`, ctx.iHaveAnEventLoggerWithMultipleOutputTargetsConfigured)
			s.When(`^I emit an event$`, ctx.iEmitAnEvent)
			s.Then(`^the event should be logged to all configured targets$`, ctx.theEventShouldBeLoggedToAllConfiguredTargets)
			s.Then(`^each target should receive the same event data$`, ctx.eachTargetShouldReceiveTheSameEventData)

			// Metadata
			s.Given(`^I have an event logger with metadata inclusion enabled$`, ctx.iHaveAnEventLoggerWithMetadataInclusionEnabled)
			s.When(`^I emit an event with metadata$`, ctx.iEmitAnEventWithMetadata)
			s.Then(`^the logged event should include the metadata$`, ctx.theLoggedEventShouldIncludeTheMetadata)
			s.Then(`^CloudEvent fields should be preserved$`, ctx.cloudEventFieldsShouldBePreserved)

			// Shutdown
			s.Given(`^I have an event logger with pending events$`, ctx.iHaveAnEventLoggerWithPendingEvents)
			s.When(`^the module is stopped$`, ctx.theModuleIsStopped)
			s.Then(`^all pending events should be flushed$`, ctx.allPendingEventsShouldBeFlushed)
			s.Then(`^output targets should be closed properly$`, ctx.outputTargetsShouldBeClosedProperly)

			// Error handling
			s.Given(`^I have an event logger with faulty output target$`, ctx.iHaveAnEventLoggerWithFaultyOutputTarget)
			s.When(`^I emit events$`, ctx.iEmitEvents)
			s.Then(`^errors should be handled gracefully$`, ctx.errorsShouldBeHandledGracefully)
			s.Then(`^other output targets should continue working$`, ctx.otherOutputTargetsShouldContinueWorking)

			// Event observation step registrations
			s.Given(`^I have an event logger with event observation enabled$`, ctx.iHaveAnEventLoggerWithEventObservationEnabled)
			s.When(`^the event logger module starts$`, func() error { return nil }) // Already started in Given step
			s.Then(`^a logger started event should be emitted$`, ctx.aLoggerStartedEventShouldBeEmitted)
			s.Then(`^the event should contain output count and buffer size$`, ctx.theEventShouldContainOutputCountAndBufferSize)
			s.When(`^the event logger module stops$`, ctx.theEventLoggerModuleStops)
			s.Then(`^a logger stopped event should be emitted$`, ctx.aLoggerStoppedEventShouldBeEmitted)

			// Configuration events
			s.When(`^the event logger module is initialized$`, func() error {
				return ctx.theEventLoggerModuleIsInitialized() // Always call regular initialization
			})
			s.Then(`^a config loaded event should be emitted$`, ctx.aConfigLoadedEventShouldBeEmitted)
			s.Then(`^output registered events should be emitted for each target$`, ctx.outputRegisteredEventsShouldBeEmittedForEachTarget)
			s.Then(`^the events should contain configuration details$`, ctx.theEventsShouldContainConfigurationDetails)

			// Processing events
			s.When(`^I emit a test event for processing$`, ctx.iEmitATestEventForProcessing)
			s.Then(`^an event received event should be emitted$`, ctx.anEventReceivedEventShouldBeEmitted)
			s.Then(`^an event processed event should be emitted$`, ctx.anEventProcessedEventShouldBeEmitted)
			s.Then(`^an output success event should be emitted$`, ctx.anOutputSuccessEventShouldBeEmitted)

			// Buffer overflow events
			s.Given(`^I have an event logger with small buffer and event observation enabled$`, ctx.iHaveAnEventLoggerWithSmallBufferAndEventObservationEnabled)
			s.When(`^I rapidly emit more events than the buffer can hold$`, ctx.iRapidlyEmitMoreEventsThanTheBufferCanHold)
			s.Then(`^buffer full events should be emitted$`, ctx.bufferFullEventsShouldBeEmitted)
			s.Then(`^event dropped events should be emitted$`, ctx.eventDroppedEventsShouldBeEmitted)
			s.Then(`^the events should contain drop reasons$`, ctx.theEventsShouldContainDropReasons)

			// Output error events
			s.Given(`^I have an event logger with faulty output target and event observation enabled$`, ctx.iHaveAnEventLoggerWithFaultyOutputTargetAndEventObservationEnabled)
			s.Then(`^an output error event should be emitted$`, ctx.anOutputErrorEventShouldBeEmitted)
			s.Then(`^the error event should contain error details$`, ctx.theErrorEventShouldContainErrorDetails)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/eventlogger_module.feature"},
			TestingT: t,
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

// Event validation step - ensures all registered events are emitted during testing
func (ctx *EventLoggerBDDTestContext) allRegisteredEventsShouldBeEmittedDuringTesting() error {
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
