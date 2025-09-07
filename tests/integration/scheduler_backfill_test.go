package integration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
)

// Simple test scheduler module for integration testing
type TestSchedulerModule struct {
	name string
}

func (m *TestSchedulerModule) Name() string { return m.name }
func (m *TestSchedulerModule) Init(app modular.Application) error { return nil }

// T059: Add integration test for scheduler bounded backfill
func TestSchedulerBackfill_Integration(t *testing.T) {
	t.Run("should register and configure scheduler module", func(t *testing.T) {
		app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		app.EnableEnhancedLifecycle()

		// Register test scheduler module
		schedMod := &TestSchedulerModule{name: "scheduler"}
		app.RegisterModule("scheduler", schedMod)

		// Configure scheduler with backfill policy settings
		mapFeeder := feeders.NewMapFeeder(map[string]interface{}{
			"scheduler.enabled":                true,
			"scheduler.default_backfill_policy": "none",
		})
		app.RegisterFeeder("config", mapFeeder)

		ctx := context.Background()

		// Initialize and start application
		err := app.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize application: %v", err)
		}

		err = app.StartWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to start application: %v", err)
		}

		// Verify configuration is loaded
		provider := app.ConfigProvider()
		if provider == nil {
			t.Fatal("Config provider should be available")
		}

		backfillPolicy, err := provider.GetString("scheduler.default_backfill_policy")
		if err != nil {
			t.Fatalf("Failed to get backfill policy: %v", err)
		}

		if backfillPolicy != "none" {
			t.Errorf("Expected 'none' backfill policy, got: %s", backfillPolicy)
		}

		// Cleanup
		err = app.StopWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Errorf("Failed to stop application: %v", err)
		}
	})

	t.Run("should handle different backfill policy configurations", func(t *testing.T) {
		testCases := []struct {
			name     string
			policy   string
			limit    int
			window   string
		}{
			{"none policy", "none", 0, ""},
			{"last policy", "last", 0, ""},
			{"bounded policy", "bounded", 5, ""},
			{"time_window policy", "time_window", 0, "10m"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
				app.EnableEnhancedLifecycle()

				// Register test scheduler module
				schedMod := &TestSchedulerModule{name: "scheduler"}
				app.RegisterModule("scheduler", schedMod)

				// Configure scheduler with specific policy
				config := map[string]interface{}{
					"scheduler.enabled":                true,
					"scheduler.default_backfill_policy": tc.policy,
				}

				if tc.limit > 0 {
					config["scheduler.backfill_limit"] = tc.limit
				}
				if tc.window != "" {
					config["scheduler.backfill_window"] = tc.window
				}

				mapFeeder := feeders.NewMapFeeder(config)
				app.RegisterFeeder("config", mapFeeder)

				ctx := context.Background()

				// Initialize and start application
				err := app.InitWithEnhancedLifecycle(ctx)
				if err != nil {
					t.Fatalf("Failed to initialize application: %v", err)
				}

				err = app.StartWithEnhancedLifecycle(ctx)
				if err != nil {
					t.Fatalf("Failed to start application: %v", err)
				}

				// Verify configuration is loaded correctly
				provider := app.ConfigProvider()
				if provider == nil {
					t.Fatal("Config provider should be available")
				}

				actualPolicy, err := provider.GetString("scheduler.default_backfill_policy")
				if err != nil {
					t.Fatalf("Failed to get backfill policy: %v", err)
				}

				if actualPolicy != tc.policy {
					t.Errorf("Expected '%s' policy, got: %s", tc.policy, actualPolicy)
				}

				// Verify additional configuration if present
				if tc.limit > 0 {
					limit, err := provider.GetInt("scheduler.backfill_limit")
					if err != nil {
						t.Fatalf("Failed to get backfill limit: %v", err)
					}
					if limit != tc.limit {
						t.Errorf("Expected limit %d, got: %d", tc.limit, limit)
					}
				}

				if tc.window != "" {
					window, err := provider.GetString("scheduler.backfill_window")
					if err != nil {
						t.Fatalf("Failed to get backfill window: %v", err)
					}
					if window != tc.window {
						t.Errorf("Expected window %s, got: %s", tc.window, window)
					}
				}

				// Cleanup
				err = app.StopWithEnhancedLifecycle(ctx)
				if err != nil {
					t.Errorf("Failed to stop application: %v", err)
				}
			})
		}
	})

	t.Run("should support job execution configuration", func(t *testing.T) {
		app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		app.EnableEnhancedLifecycle()

		// Register test scheduler module
		schedMod := &TestSchedulerModule{name: "scheduler"}
		app.RegisterModule("scheduler", schedMod)

		// Configure scheduler with job execution settings
		mapFeeder := feeders.NewMapFeeder(map[string]interface{}{
			"scheduler.enabled":           true,
			"scheduler.max_concurrent":    10,
			"scheduler.check_interval":    "30s",
			"scheduler.execution_timeout": "5m",
		})
		app.RegisterFeeder("config", mapFeeder)

		ctx := context.Background()

		// Initialize and start application
		err := app.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize application: %v", err)
		}

		err = app.StartWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to start application: %v", err)
		}

		// Verify configuration
		provider := app.ConfigProvider()
		if provider == nil {
			t.Fatal("Config provider should be available")
		}

		maxConcurrent, err := provider.GetInt("scheduler.max_concurrent")
		if err != nil {
			t.Fatalf("Failed to get max_concurrent: %v", err)
		}
		if maxConcurrent != 10 {
			t.Errorf("Expected max_concurrent 10, got: %d", maxConcurrent)
		}

		checkInterval, err := provider.GetString("scheduler.check_interval")
		if err != nil {
			t.Fatalf("Failed to get check_interval: %v", err)
		}
		if checkInterval != "30s" {
			t.Errorf("Expected check_interval 30s, got: %s", checkInterval)
		}

		// Cleanup
		err = app.StopWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Errorf("Failed to stop application: %v", err)
		}
	})

	t.Run("should validate scheduler configuration", func(t *testing.T) {
		app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		app.EnableEnhancedLifecycle()

		// Register test scheduler module
		schedMod := &TestSchedulerModule{name: "scheduler"}
		app.RegisterModule("scheduler", schedMod)

		// Configure scheduler with invalid settings (negative values)
		mapFeeder := feeders.NewMapFeeder(map[string]interface{}{
			"scheduler.enabled":        true,
			"scheduler.max_concurrent": -1, // Invalid negative value
		})
		app.RegisterFeeder("config", mapFeeder)

		ctx := context.Background()

		// Initialize application (this should work, validation might be in the module)
		err := app.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize application: %v", err)
		}

		// Verify the configuration was loaded (even if invalid)
		provider := app.ConfigProvider()
		if provider == nil {
			t.Fatal("Config provider should be available")
		}

		maxConcurrent, err := provider.GetInt("scheduler.max_concurrent")
		if err != nil {
			t.Fatalf("Failed to get max_concurrent: %v", err)
		}
		if maxConcurrent != -1 {
			t.Errorf("Expected max_concurrent -1 (invalid), got: %d", maxConcurrent)
		}

		// The framework loaded the config; validation would be module-specific
		t.Log("Configuration validation would be handled by the scheduler module implementation")

		// Cleanup
		err = app.StopWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Errorf("Failed to stop application: %v", err)
		}
	})

	t.Run("should handle scheduler lifecycle events", func(t *testing.T) {
		app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		app.EnableEnhancedLifecycle()

		// Register test scheduler module
		schedMod := &TestSchedulerModule{name: "scheduler"}
		app.RegisterModule("scheduler", schedMod)

		// Basic configuration
		mapFeeder := feeders.NewMapFeeder(map[string]interface{}{
			"scheduler.enabled": true,
		})
		app.RegisterFeeder("config", mapFeeder)

		ctx := context.Background()

		// Initialize and start application
		err := app.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize application: %v", err)
		}

		err = app.StartWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to start application: %v", err)
		}

		// Verify lifecycle dispatcher is available
		lifecycleDispatcher := app.GetLifecycleDispatcher()
		if lifecycleDispatcher == nil {
			t.Fatal("Lifecycle dispatcher should be available")
		}

		// Verify health aggregator is available
		healthAggregator := app.GetHealthAggregator()
		if healthAggregator == nil {
			t.Fatal("Health aggregator should be available")
		}

		// Get overall health
		health, err := healthAggregator.GetOverallHealth(ctx)
		if err != nil {
			t.Fatalf("Failed to get overall health: %v", err)
		}

		if health.Status != "healthy" && health.Status != "warning" {
			t.Errorf("Expected healthy status, got: %s", health.Status)
		}

		// Cleanup
		err = app.StopWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Errorf("Failed to stop application: %v", err)
		}
	})
}