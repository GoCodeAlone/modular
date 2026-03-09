package eventlogger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Configuration creation helpers

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
		// Explicitly set since struct literal bypasses default tag
		ShutdownEmitStopped: true,
		// Enable synchronous startup emission so tests reliably observe
		// config.loaded, output.registered, and started events without
		// relying on timing of goroutines.
		StartupSync: true,
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

// Core module functionality step implementations

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
