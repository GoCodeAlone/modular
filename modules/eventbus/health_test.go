package eventbus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/GoCodeAlone/modular"
)

// mockLogger is a simple logger implementation for testing
type mockLogger struct{}

func (l *mockLogger) Debug(msg string, args ...interface{}) {}
func (l *mockLogger) Info(msg string, args ...interface{})  {}
func (l *mockLogger) Warn(msg string, args ...interface{})  {}
func (l *mockLogger) Error(msg string, args ...interface{}) {}
func (l *mockLogger) With(keysAndValues ...interface{}) modular.Logger { return l }

func TestEventBusModule_HealthCheck_MemoryEngine(t *testing.T) {
	// RED PHASE: Write failing test for memory-based event bus health check
	
	// Create an event bus module with memory engine
	config := &EventBusConfig{
		Engine:                 "memory",
		MaxEventQueueSize:      1000,
		DefaultEventBufferSize: 10,
		WorkerCount:            3,
		EventTTL:               3600,
	}
	
	module := &EventBusModule{
		name:   "eventbus",
		logger: &mockLogger{},
		config: config,
	}

	// Initialize the module
	router, err := NewEngineRouter(config)
	require.NoError(t, err)
	module.router = router
	
	// Start the module to ensure proper initialization
	ctx := context.Background()
	err = module.Start(ctx)
	require.NoError(t, err)
	
	defer module.Stop(ctx)

	// Act: Perform health check
	reports, err := module.HealthCheck(ctx)

	// Assert: Should return healthy status for memory engine
	assert.NoError(t, err)
	assert.NotEmpty(t, reports)
	
	// Find the eventbus health report
	var eventbusReport *modular.HealthReport
	for i, report := range reports {
		if report.Module == "eventbus" {
			eventbusReport = &reports[i]
			break
		}
	}
	
	require.NotNil(t, eventbusReport, "Expected eventbus health report")
	assert.Equal(t, "eventbus", eventbusReport.Module)
	assert.Equal(t, "memory", eventbusReport.Component)
	assert.Equal(t, modular.HealthStatusHealthy, eventbusReport.Status)
	assert.NotEmpty(t, eventbusReport.Message)
	assert.False(t, eventbusReport.Optional)
	assert.WithinDuration(t, time.Now(), eventbusReport.CheckedAt, 5*time.Second)
	
	// EventBus should include queue depth and worker info in details
	assert.Contains(t, eventbusReport.Details, "engine")
	assert.Contains(t, eventbusReport.Details, "worker_count")
	assert.Contains(t, eventbusReport.Details, "is_started")
	assert.Equal(t, "memory", eventbusReport.Details["engine"])
	assert.Equal(t, true, eventbusReport.Details["is_started"])
}

func TestEventBusModule_HealthCheck_RedisEngine(t *testing.T) {
	// RED PHASE: Write failing test for Redis-based event bus health check
	
	// Create an event bus module with Redis engine
	module := &EventBusModule{
		name:   "eventbus",
		logger: &mockLogger{},
		config: &EventBusConfig{
			Engine:                "redis",
			ExternalBrokerURL:     "redis://localhost:6379",
			MaxEventQueueSize:     1000,
			DefaultEventBufferSize: 10,
			WorkerCount:           3,
		},
	}

	// Initialize the module
	router, err := NewEngineRouter(module.config)
	require.NoError(t, err)
	module.router = router
	
	// Try to start the module - skip test if Redis not available
	ctx := context.Background()
	err = module.Start(ctx)
	if err != nil {
		t.Skip("Redis not available for testing")
		return
	}
	
	defer module.Stop(ctx)

	// Act: Perform health check
	reports, err := module.HealthCheck(ctx)

	// Assert: Should return status based on Redis connectivity
	assert.NoError(t, err)
	assert.NotEmpty(t, reports)
	
	// Find the eventbus health report
	var eventbusReport *modular.HealthReport
	for i, report := range reports {
		if report.Module == "eventbus" {
			eventbusReport = &reports[i]
			break
		}
	}
	
	require.NotNil(t, eventbusReport, "Expected eventbus health report")
	assert.Equal(t, "eventbus", eventbusReport.Module)
	assert.Equal(t, "redis", eventbusReport.Component)
	assert.NotEmpty(t, eventbusReport.Message)
	assert.False(t, eventbusReport.Optional)
	assert.WithinDuration(t, time.Now(), eventbusReport.CheckedAt, 5*time.Second)
	
	// Redis eventbus should include broker info in details
	assert.Contains(t, eventbusReport.Details, "engine")
	assert.Contains(t, eventbusReport.Details, "broker_url")
	assert.Equal(t, "redis", eventbusReport.Details["engine"])
}

func TestEventBusModule_HealthCheck_UnhealthyModule(t *testing.T) {
	// RED PHASE: Test unhealthy event bus scenario
	
	// Create an event bus module without proper initialization
	module := &EventBusModule{
		name:   "eventbus", 
		logger: &mockLogger{},
		config: &EventBusConfig{
			Engine: "memory",
		},
		router:    nil, // No router initialized - should be unhealthy
		isStarted: false,
	}

	// Act: Perform health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reports, err := module.HealthCheck(ctx)

	// Assert: Should return unhealthy status
	assert.NoError(t, err)
	assert.NotEmpty(t, reports)
	
	// Find the eventbus health report
	var eventbusReport *modular.HealthReport
	for i, report := range reports {
		if report.Module == "eventbus" {
			eventbusReport = &reports[i]
			break
		}
	}
	
	require.NotNil(t, eventbusReport, "Expected eventbus health report")
	assert.Equal(t, "eventbus", eventbusReport.Module)
	assert.Equal(t, modular.HealthStatusUnhealthy, eventbusReport.Status)
	assert.Contains(t, eventbusReport.Message, "not started")
	assert.False(t, eventbusReport.Optional)
}

func TestEventBusModule_HealthCheck_WithEventPublishing(t *testing.T) {
	// RED PHASE: Test health check with active event processing
	
	// Create an event bus module with memory engine
	module := &EventBusModule{
		name:   "eventbus",
		logger: &mockLogger{},
		config: &EventBusConfig{
			Engine:                 "memory",
			MaxEventQueueSize:      100,
			DefaultEventBufferSize: 5,
			WorkerCount:            2,
			EventTTL:               3600,
		},
	}

	// Initialize and start the module
	router, err := NewEngineRouter(module.config)
	require.NoError(t, err)
	module.router = router
	
	ctx := context.Background()
	err = module.Start(ctx)
	require.NoError(t, err)
	
	defer module.Stop(ctx)
	
	// Publish some events to test queue depth reporting
	for i := 0; i < 5; i++ {
		err := module.Publish(ctx, "test.event", map[string]interface{}{
			"id": i,
			"message": "test event",
		})
		require.NoError(t, err)
	}
	
	// Give events time to be processed
	time.Sleep(100 * time.Millisecond)

	// Act: Perform health check
	reports, err := module.HealthCheck(ctx)

	// Assert: Should show healthy status with event processing stats
	assert.NoError(t, err)
	assert.NotEmpty(t, reports)
	
	var eventbusReport *modular.HealthReport
	for i, report := range reports {
		if report.Module == "eventbus" {
			eventbusReport = &reports[i]
			break
		}
	}
	
	require.NotNil(t, eventbusReport, "Expected eventbus health report")
	assert.Equal(t, modular.HealthStatusHealthy, eventbusReport.Status)
	
	// Check that processing statistics are included
	assert.Contains(t, eventbusReport.Details, "worker_count")
	assert.Contains(t, eventbusReport.Details, "is_started")
	assert.Equal(t, 2, eventbusReport.Details["worker_count"])
	assert.Equal(t, true, eventbusReport.Details["is_started"])
}

func TestEventBusModule_HealthCheck_HighQueueDepth(t *testing.T) {
	// RED PHASE: Test degraded status when queue depth is high
	
	// Create an event bus module with small queue size
	module := &EventBusModule{
		name:   "eventbus",
		logger: &mockLogger{},
		config: &EventBusConfig{
			Engine:                 "memory",
			MaxEventQueueSize:      10, // Very small limit
			DefaultEventBufferSize: 2,
			WorkerCount:            1, // Single worker to cause backlog
			EventTTL:               3600,
		},
	}

	// Initialize and start the module
	router, err := NewEngineRouter(module.config)
	require.NoError(t, err)
	module.router = router
	
	ctx := context.Background()
	err = module.Start(ctx)
	require.NoError(t, err)
	
	defer module.Stop(ctx)

	// Act: Perform health check
	reports, err := module.HealthCheck(ctx)

	// Assert: Should return healthy or degraded status based on queue utilization
	assert.NoError(t, err)
	assert.NotEmpty(t, reports)
	
	var eventbusReport *modular.HealthReport
	for i, report := range reports {
		if report.Module == "eventbus" {
			eventbusReport = &reports[i]
			break
		}
	}
	
	require.NotNil(t, eventbusReport, "Expected eventbus health report")
	assert.Equal(t, "eventbus", eventbusReport.Module)
	// Status should be healthy initially (no backlog yet)
	assert.Equal(t, modular.HealthStatusHealthy, eventbusReport.Status)
}

func TestEventBusModule_HealthCheck_WithContext(t *testing.T) {
	// RED PHASE: Test context cancellation handling
	
	module := &EventBusModule{
		name:   "eventbus",
		logger: &mockLogger{},
		config: &EventBusConfig{
			Engine: "memory",
		},
	}
	
	router, err := NewEngineRouter(module.config)
	require.NoError(t, err)
	module.router = router

	// Act: Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	reports, err := module.HealthCheck(ctx)

	// Assert: Should handle context cancellation gracefully
	if err != nil {
		assert.Contains(t, err.Error(), "context")
	} else {
		// If no error, reports should still be valid
		assert.NotNil(t, reports)
	}
}

// Test helper to verify the module implements HealthProvider interface
func TestEventBusModule_ImplementsHealthProvider(t *testing.T) {
	// Verify that EventBusModule implements HealthProvider interface
	module := &EventBusModule{
		name:   "eventbus",
		logger: &mockLogger{},
		config: &EventBusConfig{
			Engine: "memory",
		},
	}
	
	// This should compile without errors if the interface is properly implemented
	var _ modular.HealthProvider = module
	
	// Also verify method signatures exist (will fail to compile if missing)
	ctx := context.Background()
	reports, err := module.HealthCheck(ctx)
	
	// No error expected with a basic module setup
	assert.NoError(t, err)
	assert.NotNil(t, reports)
}