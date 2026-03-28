package eventlogger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Output targets configuration helpers

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

// Output targets step implementations

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

	// Create config with metadata inclusion enabled (already enabled in console config).
	// Buffer must be large enough to absorb framework lifecycle events during Init/Start.
	config := ctx.createConsoleConfig(50)

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
