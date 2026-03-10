package modular

import (
	"testing"
)

// Test_ApplicationSetLogger tests the SetLogger functionality
func Test_ApplicationSetLogger(t *testing.T) {
	// Create initial logger
	initialLogger := &logger{t}

	// Create application with initial logger
	app := NewStdApplication(
		NewStdConfigProvider(testCfg{Str: "test"}),
		initialLogger,
	)

	// Verify initial logger is set
	if app.Logger() != initialLogger {
		t.Error("Initial logger not set correctly")
	}

	// Create a new logger
	newLogger := &logger{t}

	// Set the new logger
	app.SetLogger(newLogger)

	// Verify the logger was changed
	if app.Logger() != newLogger {
		t.Error("SetLogger did not update the logger correctly")
	}

	// Verify the old logger is no longer referenced
	if app.Logger() == initialLogger {
		t.Error("SetLogger did not replace the old logger")
	}
}

// Test_ApplicationSetLoggerWithNilLogger tests SetLogger with nil logger
func Test_ApplicationSetLoggerWithNilLogger(t *testing.T) {
	// Create initial logger
	initialLogger := &logger{t}

	// Create application with initial logger
	app := NewStdApplication(
		NewStdConfigProvider(testCfg{Str: "test"}),
		initialLogger,
	)

	// Set logger to nil
	app.SetLogger(nil)

	// Verify logger is now nil
	if app.Logger() != nil {
		t.Error("SetLogger did not set logger to nil correctly")
	}
}

// Test_ApplicationSetLoggerRuntimeUsage tests runtime logger switching with actual usage
func Test_ApplicationSetLoggerRuntimeUsage(t *testing.T) {
	// Create initial logger
	initialLogger := &logger{t}

	// Create application with initial logger
	app := NewStdApplication(
		NewStdConfigProvider(testCfg{Str: "test"}),
		initialLogger,
	)

	// Verify initial logger is set
	if app.Logger() != initialLogger {
		t.Error("Initial logger not set correctly")
	}

	// Create a new mock logger to switch to
	newMockLogger := &MockLogger{}
	// Set up a simple expectation that might be called later
	newMockLogger.On("Debug", "Test message", []any{"key", "value"}).Return().Maybe()

	// Switch to the new logger
	app.SetLogger(newMockLogger)

	// Verify the logger was switched
	if app.Logger() != newMockLogger {
		t.Error("Logger was not switched correctly")
	}

	// Verify the old logger is no longer referenced
	if app.Logger() == initialLogger {
		t.Error("SetLogger did not replace the old logger")
	}

	// Test that the new logger is actually used when the application logs something
	app.Logger().Debug("Test message", "key", "value")

	// Verify mock expectations were met (if any were called)
	newMockLogger.AssertExpectations(t)
}

func TestSetVerboseConfig(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{
			name:    "Enable verbose config",
			enabled: true,
		},
		{
			name:    "Disable verbose config",
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock logger to capture debug messages
			mockLogger := &MockLogger{}

			// Set up expectations for debug messages
			if tt.enabled {
				mockLogger.On("Debug", "Verbose configuration debugging enabled", []any(nil)).Return()
			} else {
				mockLogger.On("Debug", "Verbose configuration debugging disabled", []any(nil)).Return()
			}

			// Create application with mock logger
			app := NewStdApplication(
				NewStdConfigProvider(testCfg{Str: "test"}),
				mockLogger,
			)

			// Test that verbose config is initially false
			if app.IsVerboseConfig() != false {
				t.Error("Expected verbose config to be initially false")
			}

			// Set verbose config
			app.SetVerboseConfig(tt.enabled)

			// Verify the setting was applied
			if app.IsVerboseConfig() != tt.enabled {
				t.Errorf("Expected verbose config to be %v, got %v", tt.enabled, app.IsVerboseConfig())
			}

			// Verify mock expectations were met
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestIsVerboseConfig(t *testing.T) {
	mockLogger := &MockLogger{}

	// Create application
	app := NewStdApplication(
		NewStdConfigProvider(testCfg{Str: "test"}),
		mockLogger,
	)

	// Test initial state
	if app.IsVerboseConfig() != false {
		t.Error("Expected IsVerboseConfig to return false initially")
	}

	// Test after enabling
	mockLogger.On("Debug", "Verbose configuration debugging enabled", []any(nil)).Return()
	app.SetVerboseConfig(true)
	if app.IsVerboseConfig() != true {
		t.Error("Expected IsVerboseConfig to return true after enabling")
	}

	// Test after disabling
	mockLogger.On("Debug", "Verbose configuration debugging disabled", []any(nil)).Return()
	app.SetVerboseConfig(false)
	if app.IsVerboseConfig() != false {
		t.Error("Expected IsVerboseConfig to return false after disabling")
	}

	mockLogger.AssertExpectations(t)
}
