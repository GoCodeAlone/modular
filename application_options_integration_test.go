package modular

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplicationOptionsIntegration tests the real application options integration
func TestApplicationOptionsIntegration(t *testing.T) {
	t.Run("should_create_application_with_dynamic_reload_option", func(t *testing.T) {
		// Create base application
		config := &appTestConfig{Str: "test"}
		configProvider := NewStdConfigProvider(config)
		logger := &appTestLogger{t: t}

		// Apply dynamic reload option
		reloadConfig := DynamicReloadConfig{
			Enabled:       true,
			ReloadTimeout: 30 * time.Second,
		}

		app := NewStdApplicationWithOptions(
			configProvider,
			logger,
			WithDynamicReload(reloadConfig),
		)

		require.NoError(t, app.Init())

		// Verify that ReloadOrchestrator service is registered
		var orchestrator *ReloadOrchestrator
		err := app.GetService("reloadOrchestrator", &orchestrator)
		assert.NoError(t, err, "ReloadOrchestrator service should be registered")
		assert.NotNil(t, orchestrator, "ReloadOrchestrator should not be nil")

		// Test dynamic reload functionality
		err = app.RequestReload()
		assert.NoError(t, err, "RequestReload should work")
	})

	t.Run("should_create_application_with_health_aggregator_option", func(t *testing.T) {
		// Create base application
		config := &appTestConfig{Str: "test"}
		configProvider := NewStdConfigProvider(config)
		logger := &appTestLogger{t: t}

		// Apply health aggregator option
		healthConfig := HealthAggregatorConfig{
			Enabled:       true,
			CheckInterval: 10 * time.Second,
			CheckTimeout:  5 * time.Second,
		}

		app := NewStdApplicationWithOptions(
			configProvider,
			logger,
			WithHealthAggregator(healthConfig),
		)

		require.NoError(t, app.Init())

		// Verify that HealthAggregator service is registered
		var aggregator HealthAggregator
		err := app.GetService("healthAggregator", &aggregator)
		assert.NoError(t, err, "HealthAggregator service should be registered")
		assert.NotNil(t, aggregator, "HealthAggregator should not be nil")
	})

	t.Run("should_support_multiple_options", func(t *testing.T) {
		// Create base application
		config := &appTestConfig{Str: "test"}
		configProvider := NewStdConfigProvider(config)
		logger := &appTestLogger{t: t}

		// Apply both options
		reloadConfig := DynamicReloadConfig{
			Enabled:       true,
			ReloadTimeout: 45 * time.Second,
		}

		healthConfig := HealthAggregatorConfig{
			Enabled:       true,
			CheckInterval: 20 * time.Second,
			CheckTimeout:  10 * time.Second,
		}

		app := NewStdApplicationWithOptions(
			configProvider,
			logger,
			WithDynamicReload(reloadConfig),
			WithHealthAggregator(healthConfig),
		)

		require.NoError(t, app.Init())

		// Verify both services are registered
		var orchestrator *ReloadOrchestrator
		err := app.GetService("reloadOrchestrator", &orchestrator)
		assert.NoError(t, err)
		assert.NotNil(t, orchestrator)

		var aggregator HealthAggregator
		err = app.GetService("healthAggregator", &aggregator)
		assert.NoError(t, err)
		assert.NotNil(t, aggregator)
	})
}

// Test helper types
type appTestConfig struct {
	Str string `json:"str"`
}

type appTestLogger struct {
	t *testing.T
}

func (l *appTestLogger) Debug(msg string, args ...any) {
	l.t.Logf("DEBUG: %s %v", msg, args)
}

func (l *appTestLogger) Info(msg string, args ...any) {
	l.t.Logf("INFO: %s %v", msg, args)
}

func (l *appTestLogger) Warn(msg string, args ...any) {
	l.t.Logf("WARN: %s %v", msg, args)
}

func (l *appTestLogger) Error(msg string, args ...any) {
	l.t.Logf("ERROR: %s %v", msg, args)
}
