package modular

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthInterfaceStandardization tests the migration from HealthReporter to HealthProvider
func TestHealthInterfaceStandardization(t *testing.T) {
	t.Run("should_provide_adapter_for_legacy_HealthReporter", func(t *testing.T) {
		// Create a legacy HealthReporter implementation
		legacyReporter := &testLegacyHealthReporter{
			name:    "legacy-service",
			timeout: 5 * time.Second,
			result: HealthResult{
				Status:    HealthStatusHealthy,
				Message:   "Service is running",
				Timestamp: time.Now(),
			},
		}

		// Convert it to the new HealthProvider interface using an adapter
		provider := NewHealthReporterAdapter(legacyReporter, "legacy-module")

		// Test that it implements HealthProvider correctly
		ctx := context.Background()
		reports, err := provider.HealthCheck(ctx)

		require.NoError(t, err)
		require.Len(t, reports, 1)

		report := reports[0]
		assert.Equal(t, "legacy-module", report.Module)
		assert.Equal(t, "legacy-service", report.Component)
		assert.Equal(t, HealthStatusHealthy, report.Status)
		assert.Equal(t, "Service is running", report.Message)
		assert.False(t, report.CheckedAt.IsZero())
	})

	t.Run("should_handle_legacy_reporter_errors_gracefully", func(t *testing.T) {
		// Create a failing legacy reporter
		legacyReporter := &testLegacyHealthReporter{
			name:    "failing-service",
			timeout: 1 * time.Second,
			result: HealthResult{
				Status:    HealthStatusUnhealthy,
				Message:   "Database connection failed",
				Timestamp: time.Now(),
			},
		}

		provider := NewHealthReporterAdapter(legacyReporter, "database-module")

		ctx := context.Background()
		reports, err := provider.HealthCheck(ctx)

		require.NoError(t, err)
		require.Len(t, reports, 1)

		report := reports[0]
		assert.Equal(t, "database-module", report.Module)
		assert.Equal(t, "failing-service", report.Component)
		assert.Equal(t, HealthStatusUnhealthy, report.Status)
		assert.Equal(t, "Database connection failed", report.Message)
	})

	t.Run("should_provide_utility_for_single_report_providers", func(t *testing.T) {
		// Test utility for creating simple single-report providers
		provider := NewSimpleHealthProvider("test-module", "test-component", func(ctx context.Context) (HealthStatus, string, error) {
			return HealthStatusHealthy, "All systems operational", nil
		})

		ctx := context.Background()
		reports, err := provider.HealthCheck(ctx)

		require.NoError(t, err)
		require.Len(t, reports, 1)

		report := reports[0]
		assert.Equal(t, "test-module", report.Module)
		assert.Equal(t, "test-component", report.Component)
		assert.Equal(t, HealthStatusHealthy, report.Status)
		assert.Equal(t, "All systems operational", report.Message)
	})

	t.Run("should_handle_context_cancellation", func(t *testing.T) {
		// Test that adapters properly handle context cancellation
		slowReporter := &testLegacyHealthReporter{
			name:    "slow-service",
			timeout: 10 * time.Second, // Long timeout
			delay:   2 * time.Second,  // Simulate slow check
			result: HealthResult{
				Status:    HealthStatusHealthy,
				Message:   "Slow but healthy",
				Timestamp: time.Now(),
			},
		}

		provider := NewHealthReporterAdapter(slowReporter, "slow-module")

		// Create a context with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := provider.HealthCheck(ctx)

		// Should respect context cancellation
		assert.Error(t, err)
		assert.Equal(t, context.DeadlineExceeded, err)
	})
}

// TestHealthProviderUtilities tests utility functions for creating health providers
func TestHealthProviderUtilities(t *testing.T) {
	t.Run("should_create_static_healthy_provider", func(t *testing.T) {
		provider := NewStaticHealthProvider("static-module", "static-component", HealthStatusHealthy, "Always healthy")

		ctx := context.Background()
		reports, err := provider.HealthCheck(ctx)

		require.NoError(t, err)
		require.Len(t, reports, 1)

		report := reports[0]
		assert.Equal(t, "static-module", report.Module)
		assert.Equal(t, "static-component", report.Component)
		assert.Equal(t, HealthStatusHealthy, report.Status)
		assert.Equal(t, "Always healthy", report.Message)
	})

	t.Run("should_create_composite_provider", func(t *testing.T) {
		// Create multiple providers
		provider1 := NewStaticHealthProvider("module1", "component1", HealthStatusHealthy, "OK")
		provider2 := NewStaticHealthProvider("module2", "component2", HealthStatusDegraded, "Slow")

		// Combine them
		composite := NewCompositeHealthProvider(provider1, provider2)

		ctx := context.Background()
		reports, err := composite.HealthCheck(ctx)

		require.NoError(t, err)
		require.Len(t, reports, 2)

		// Should get reports from both providers
		assert.Equal(t, "module1", reports[0].Module)
		assert.Equal(t, HealthStatusHealthy, reports[0].Status)

		assert.Equal(t, "module2", reports[1].Module)
		assert.Equal(t, HealthStatusDegraded, reports[1].Status)
	})
}

// Test helper implementations

type testLegacyHealthReporter struct {
	name    string
	timeout time.Duration
	delay   time.Duration
	result  HealthResult
}

func (r *testLegacyHealthReporter) CheckHealth(ctx context.Context) HealthResult {
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
			// Delay completed
		case <-ctx.Done():
			// Context cancelled during delay
			return HealthResult{
				Status:    HealthStatusUnknown,
				Message:   "Health check cancelled",
				Timestamp: time.Now(),
			}
		}
	}

	return r.result
}

func (r *testLegacyHealthReporter) HealthCheckName() string {
	return r.name
}

func (r *testLegacyHealthReporter) HealthCheckTimeout() time.Duration {
	return r.timeout
}
