package modular

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthReporter_CheckHealth tests the actual behavior of health checking
func TestHealthReporter_CheckHealth(t *testing.T) {
	tests := []struct {
		name       string
		reporter   HealthReporter
		ctx        context.Context
		wantStatus HealthStatus
		wantErr    bool
	}{
		{
			name:       "healthy service returns healthy status",
			reporter:   newTestHealthReporter("test-service", true, nil),
			ctx:        context.Background(),
			wantStatus: HealthStatusHealthy,
			wantErr:    false,
		},
		{
			name:       "unhealthy service returns unhealthy status",
			reporter:   newTestHealthReporter("failing-service", false, errors.New("connection failed")),
			ctx:        context.Background(),
			wantStatus: HealthStatusUnhealthy,
			wantErr:    false,
		},
		{
			name:       "context cancellation returns unknown status",
			reporter:   newSlowHealthReporter("slow-service", 100*time.Millisecond),
			ctx:        createCancelledContext(),
			wantStatus: HealthStatusUnknown,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.reporter.CheckHealth(tt.ctx)

			// Verify status matches expectation
			assert.Equal(t, tt.wantStatus, result.Status, "Health status should match expected")

			// Verify timestamp is set
			assert.WithinDuration(t, time.Now(), result.Timestamp, time.Second, "Timestamp should be recent")

			// Verify message is not empty
			assert.NotEmpty(t, result.Message, "Health result should include a message")
		})
	}
}

// TestHealthReporter_HealthCheckName tests service name reporting
func TestHealthReporter_HealthCheckName(t *testing.T) {
	tests := []struct {
		name         string
		reporter     HealthReporter
		expectedName string
	}{
		{
			name:         "returns configured service name",
			reporter:     newTestHealthReporter("database-service", true, nil),
			expectedName: "database-service",
		},
		{
			name:         "returns different service name",
			reporter:     newTestHealthReporter("cache-service", true, nil),
			expectedName: "cache-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualName := tt.reporter.HealthCheckName()
			assert.Equal(t, tt.expectedName, actualName, "Service name should match configuration")
		})
	}
}

// TestHealthReporter_HealthCheckTimeout tests timeout configuration
func TestHealthReporter_HealthCheckTimeout(t *testing.T) {
	tests := []struct {
		name            string
		reporter        HealthReporter
		expectedTimeout time.Duration
	}{
		{
			name:            "returns configured timeout",
			reporter:        newTestHealthReporterWithTimeout("service", 10*time.Second),
			expectedTimeout: 10 * time.Second,
		},
		{
			name:            "returns different timeout",
			reporter:        newTestHealthReporterWithTimeout("service", 5*time.Minute),
			expectedTimeout: 5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualTimeout := tt.reporter.HealthCheckTimeout()
			assert.Equal(t, tt.expectedTimeout, actualTimeout, "Timeout should match configuration")
		})
	}
}

// TestHealthResult tests the HealthResult data structure behavior
func TestHealthResult(t *testing.T) {
	t.Run("should construct with all required fields", func(t *testing.T) {
		timestamp := time.Now()
		details := map[string]interface{}{
			"connection_count": 42,
			"uptime":           "5m30s",
		}

		result := HealthResult{
			Status:    HealthStatusHealthy,
			Message:   "Service is healthy",
			Timestamp: timestamp,
			Details:   details,
		}

		// Verify all fields are properly set
		assert.Equal(t, HealthStatusHealthy, result.Status)
		assert.Equal(t, "Service is healthy", result.Message)
		assert.Equal(t, timestamp, result.Timestamp)
		assert.Equal(t, details, result.Details)
	})

	t.Run("should handle empty details gracefully", func(t *testing.T) {
		result := HealthResult{
			Status:    HealthStatusUnhealthy,
			Message:   "Service failed",
			Timestamp: time.Now(),
			Details:   nil,
		}

		assert.Equal(t, HealthStatusUnhealthy, result.Status)
		assert.Nil(t, result.Details, "Details can be nil")
	})

	t.Run("should preserve structured details", func(t *testing.T) {
		details := map[string]interface{}{
			"error_count":    3,
			"last_error":     "timeout occurred",
			"retry_attempts": []int{1, 2, 3},
		}

		result := HealthResult{
			Status:    HealthStatusDegraded,
			Message:   "Partial service degradation",
			Timestamp: time.Now(),
			Details:   details,
		}

		// Verify complex details are preserved
		assert.Equal(t, 3, result.Details["error_count"])
		assert.Equal(t, "timeout occurred", result.Details["last_error"])
		assert.IsType(t, []int{}, result.Details["retry_attempts"])
	})
}

// TestHealthStatus tests health status behavior and methods
func TestHealthStatus(t *testing.T) {
	tests := []struct {
		name           string
		status         HealthStatus
		expectedString string
		isHealthy      bool
	}{
		{
			name:           "healthy status",
			status:         HealthStatusHealthy,
			expectedString: "healthy",
			isHealthy:      true,
		},
		{
			name:           "degraded status",
			status:         HealthStatusDegraded,
			expectedString: "degraded",
			isHealthy:      false,
		},
		{
			name:           "unhealthy status",
			status:         HealthStatusUnhealthy,
			expectedString: "unhealthy",
			isHealthy:      false,
		},
		{
			name:           "unknown status",
			status:         HealthStatusUnknown,
			expectedString: "unknown",
			isHealthy:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test string conversion
			assert.Equal(t, tt.expectedString, tt.status.String(), "Status string should match expected")
			assert.Equal(t, tt.expectedString, tt.status.String(), "Status cast to string should match expected")

			// Test health check
			assert.Equal(t, tt.isHealthy, tt.status.IsHealthy(), "IsHealthy should match expected")
		})
	}

	// Test status comparison and equality
	t.Run("should support equality checks", func(t *testing.T) {
		assert.Equal(t, HealthStatusHealthy, HealthStatusHealthy)
		assert.NotEqual(t, HealthStatusHealthy, HealthStatusDegraded)
		assert.NotEqual(t, HealthStatusDegraded, HealthStatusUnhealthy)
	})
}

// TestHealthReporter_ModuleIntegration tests how modules integrate with HealthReporter interface
func TestHealthReporter_ModuleIntegration(t *testing.T) {
	t.Run("should integrate with module lifecycle", func(t *testing.T) {
		// Create a test module that implements HealthReporter
		module := &testHealthModule{
			name:      "test-module",
			isHealthy: true,
			timeout:   10 * time.Second,
		}

		// Verify it implements both Module and HealthReporter interfaces
		var healthReporter HealthReporter = module
		var moduleInterface Module = module

		require.NotNil(t, healthReporter, "Module should implement HealthReporter")
		require.NotNil(t, moduleInterface, "Module should implement Module interface")

		// Test health reporter functionality
		result := healthReporter.CheckHealth(context.Background())
		assert.Equal(t, HealthStatusHealthy, result.Status)
		assert.Equal(t, "test-module", healthReporter.HealthCheckName())
		assert.Equal(t, 10*time.Second, healthReporter.HealthCheckTimeout())

		// Test module functionality
		assert.Equal(t, "test-module", moduleInterface.Name())
	})

	t.Run("should support service registration with health checking", func(t *testing.T) {
		// Create application and register health-aware module
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		healthModule := &testHealthModule{
			name:      "health-service",
			isHealthy: true,
			timeout:   5 * time.Second,
		}

		// Register module
		app.RegisterModule(healthModule)

		// Verify the module can be retrieved and used for health checks
		modules := app.GetModules()
		assert.Contains(t, modules, "health-service")

		// Simulate health aggregation by checking health reporter
		if hr, ok := modules["health-service"].(HealthReporter); ok {
			result := hr.CheckHealth(context.Background())
			assert.Equal(t, HealthStatusHealthy, result.Status)
		} else {
			t.Error("Module should implement HealthReporter interface")
		}
	})
}

// TestHealthReporter_ErrorHandling tests error scenarios and edge cases
func TestHealthReporter_ErrorHandling(t *testing.T) {
	t.Run("should handle context timeout gracefully", func(t *testing.T) {
		reporter := newSlowHealthReporter("slow-service", 100*time.Millisecond)

		// Create context that times out before health check completes
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		result := reporter.CheckHealth(ctx)
		assert.Equal(t, HealthStatusUnknown, result.Status, "Timed out health check should return unknown status")
		assert.Contains(t, result.Message, "timeout", "Message should indicate timeout")
	})

	t.Run("should provide detailed error information for failures", func(t *testing.T) {
		errorDetails := map[string]interface{}{
			"error":           "connection refused",
			"last_successful": "2023-01-01T10:00:00Z",
			"retry_count":     3,
		}
		reporter := newTestHealthReporterWithDetails("db-service", false, errorDetails)

		result := reporter.CheckHealth(context.Background())
		assert.Equal(t, HealthStatusUnhealthy, result.Status)
		assert.Contains(t, result.Message, "failed")
		assert.Equal(t, errorDetails, result.Details, "Should preserve error details")
	})

	t.Run("should distinguish between unhealthy and degraded states", func(t *testing.T) {
		degradedDetails := map[string]interface{}{
			"available_workers": 2,
			"total_workers":     5,
			"performance":       "reduced",
		}
		reporter := newTestHealthReporterWithStatus("worker-service", HealthStatusDegraded, degradedDetails)

		result := reporter.CheckHealth(context.Background())
		assert.Equal(t, HealthStatusDegraded, result.Status)
		assert.False(t, result.Status.IsHealthy(), "Degraded should not be considered healthy")
		assert.Contains(t, result.Message, "degraded")
		assert.Equal(t, degradedDetails, result.Details)
	})

	t.Run("should handle context cancellation", func(t *testing.T) {
		reporter := newSlowHealthReporter("cancelable-service", 50*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		result := reporter.CheckHealth(ctx)
		assert.Equal(t, HealthStatusUnknown, result.Status)
		assert.Contains(t, result.Message, "cancel", "Message should indicate cancellation")
	})
}

// Test helper implementations that provide real behavior for testing

// testHealthModule implements both Module and HealthReporter for integration testing
type testHealthModule struct {
	name      string
	isHealthy bool
	timeout   time.Duration
	details   map[string]interface{}
}

// Module interface implementation
func (m *testHealthModule) Name() string                          { return m.name }
func (m *testHealthModule) Dependencies() []string                { return nil }
func (m *testHealthModule) Init(Application) error                { return nil }
func (m *testHealthModule) Start(context.Context) error           { return nil }
func (m *testHealthModule) Stop(context.Context) error            { return nil }
func (m *testHealthModule) RegisterConfig(Application) error      { return nil }
func (m *testHealthModule) ProvidesServices() []ServiceProvider   { return nil }
func (m *testHealthModule) RequiresServices() []ServiceDependency { return nil }

// HealthReporter interface implementation
func (m *testHealthModule) CheckHealth(ctx context.Context) HealthResult {
	status := HealthStatusHealthy
	message := "Service is healthy"
	if !m.isHealthy {
		status = HealthStatusUnhealthy
		message = "Service health check failed"
	}

	return HealthResult{
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
		Details:   m.details,
	}
}

func (m *testHealthModule) HealthCheckName() string {
	return m.name
}

func (m *testHealthModule) HealthCheckTimeout() time.Duration {
	if m.timeout > 0 {
		return m.timeout
	}
	return 30 * time.Second
}

// Test helper functions for creating health reporters with specific behaviors

func newTestHealthReporter(name string, isHealthy bool, err error) HealthReporter {
	return &testHealthModule{
		name:      name,
		isHealthy: isHealthy,
		timeout:   30 * time.Second,
	}
}

func newTestHealthReporterWithTimeout(name string, timeout time.Duration) HealthReporter {
	return &testHealthModule{
		name:      name,
		isHealthy: true,
		timeout:   timeout,
	}
}

func newTestHealthReporterWithDetails(name string, isHealthy bool, details map[string]interface{}) HealthReporter {
	return &testHealthModule{
		name:      name,
		isHealthy: isHealthy,
		timeout:   30 * time.Second,
		details:   details,
	}
}

func newTestHealthReporterWithStatus(name string, status HealthStatus, details map[string]interface{}) HealthReporter {
	return &customStatusHealthReporter{
		name:    name,
		status:  status,
		timeout: 30 * time.Second,
		details: details,
	}
}

func newSlowHealthReporter(name string, delay time.Duration) HealthReporter {
	return &slowHealthReporter{
		name:    name,
		delay:   delay,
		timeout: 30 * time.Second,
	}
}

func createCancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// Additional helper implementations

type customStatusHealthReporter struct {
	name    string
	status  HealthStatus
	timeout time.Duration
	details map[string]interface{}
}

func (r *customStatusHealthReporter) CheckHealth(ctx context.Context) HealthResult {
	message := "Service is " + r.status.String()
	return HealthResult{
		Status:    r.status,
		Message:   message,
		Timestamp: time.Now(),
		Details:   r.details,
	}
}

func (r *customStatusHealthReporter) HealthCheckName() string {
	return r.name
}

func (r *customStatusHealthReporter) HealthCheckTimeout() time.Duration {
	return r.timeout
}

type slowHealthReporter struct {
	name    string
	delay   time.Duration
	timeout time.Duration
}

func (r *slowHealthReporter) CheckHealth(ctx context.Context) HealthResult {
	select {
	case <-ctx.Done():
		var message string
		if ctx.Err() == context.DeadlineExceeded {
			message = "Health check timeout"
		} else {
			message = "Health check cancelled"
		}
		return HealthResult{
			Status:    HealthStatusUnknown,
			Message:   message,
			Timestamp: time.Now(),
		}
	case <-time.After(r.delay):
		return HealthResult{
			Status:    HealthStatusHealthy,
			Message:   "Service is healthy (after delay)",
			Timestamp: time.Now(),
		}
	}
}

func (r *slowHealthReporter) HealthCheckName() string {
	return r.name
}

func (r *slowHealthReporter) HealthCheckTimeout() time.Duration {
	return r.timeout
}
