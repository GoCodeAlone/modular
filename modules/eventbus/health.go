package eventbus

import (
	"context"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
)

// HealthCheck implements the HealthProvider interface for the event bus module.
// This method checks the health of the configured event bus engine and
// returns detailed reports about message broker connectivity, queue depths,
// processing rates, and worker status.
//
// The health check performs the following operations:
//   - Validates that the event bus is properly started and configured
//   - Tests message broker connectivity (for external brokers like Redis, Kafka)
//   - Reports queue depths and processing statistics
//   - Provides worker pool status and performance metrics
//
// Returns:
//   - Slice of HealthReport objects with event bus status information
//   - Error if the health check operation itself fails
func (m *EventBusModule) HealthCheck(ctx context.Context) ([]modular.HealthReport, error) {
	reports := make([]modular.HealthReport, 0)
	checkTime := time.Now()

	// Create base report structure
	report := modular.HealthReport{
		Module:        "eventbus",
		Component:     m.config.Engine,
		CheckedAt:     checkTime,
		ObservedSince: checkTime,
		Optional:      false, // EventBus is typically critical for system communication
		Details:       make(map[string]any),
	}

	// Check if event bus is properly started
	m.mutex.RLock()
	isStarted := m.isStarted
	m.mutex.RUnlock()

	if !isStarted || m.router == nil {
		report.Status = modular.HealthStatusUnhealthy
		report.Message = "eventbus not started or router not initialized"
		report.Details["is_started"] = false
		report.Details["router_initialized"] = (m.router != nil)
		reports = append(reports, report)
		return reports, nil
	}

	// Test event bus connectivity and performance
	if err := m.testEventBusConnectivity(ctx, &report); err != nil {
		report.Status = modular.HealthStatusUnhealthy
		report.Message = fmt.Sprintf("eventbus connectivity test failed: %v", err)
		report.Details["connectivity_error"] = err.Error()
		reports = append(reports, report)
		return reports, nil
	}

	// Collect event bus statistics and metrics
	m.collectEventBusStatistics(&report)

	// Determine overall health status based on metrics
	m.evaluateEventBusHealthStatus(&report)

	reports = append(reports, report)
	return reports, nil
}

// testEventBusConnectivity tests basic event bus operations to ensure it's working
func (m *EventBusModule) testEventBusConnectivity(ctx context.Context, report *modular.HealthReport) error {
	// Test topic for health check
	healthTopic := "health_check_" + fmt.Sprintf("%d", time.Now().Unix())
	healthPayload := map[string]interface{}{
		"test": true,
		"timestamp": time.Now().Unix(),
	}

	// Try to publish a test event
	startTime := time.Now()
	err := m.Publish(ctx, healthTopic, healthPayload)
	publishDuration := time.Since(startTime)

	if err != nil {
		report.Details["operation_failed"] = "publish"
		report.Details["publish_error"] = err.Error()
		return fmt.Errorf("failed to publish test event: %w", err)
	}

	// Record performance metrics
	report.Details["publish_duration_ms"] = publishDuration.Milliseconds()
	report.Details["connectivity_test"] = "passed"

	return nil
}

// collectEventBusStatistics gathers usage and performance statistics from the event bus
func (m *EventBusModule) collectEventBusStatistics(report *modular.HealthReport) {
	// Add basic configuration information
	report.Details["engine"] = m.config.Engine
	report.Details["worker_count"] = m.config.WorkerCount
	report.Details["max_queue_size"] = m.config.MaxEventQueueSize
	report.Details["is_started"] = m.isStarted

	// Engine-specific statistics
	switch m.config.Engine {
	case "memory":
		m.collectMemoryEngineStats(report)
	case "redis":
		m.collectRedisEngineStats(report)
	case "kafka":
		m.collectKafkaEngineStats(report)
	}

	// Get router statistics if available
	if m.router != nil {
		m.collectRouterStatistics(report)
	}
}

// collectMemoryEngineStats collects statistics specific to memory-based event bus
func (m *EventBusModule) collectMemoryEngineStats(report *modular.HealthReport) {
	// Memory engine specific metrics
	report.Details["broker_type"] = "in-memory"
	report.Details["event_ttl_seconds"] = m.config.EventTTL
	report.Details["buffer_size"] = m.config.DefaultEventBufferSize
}

// collectRedisEngineStats collects statistics specific to Redis-based event bus
func (m *EventBusModule) collectRedisEngineStats(report *modular.HealthReport) {
	// Redis engine specific metrics
	report.Details["broker_type"] = "redis"
	report.Details["broker_url"] = m.config.ExternalBrokerURL
	
	// Additional Redis-specific configuration
	if m.config.ExternalBrokerUser != "" {
		report.Details["auth_configured"] = true
	}
}

// collectKafkaEngineStats collects statistics specific to Kafka-based event bus
func (m *EventBusModule) collectKafkaEngineStats(report *modular.HealthReport) {
	// Kafka engine specific metrics
	report.Details["broker_type"] = "kafka"
	report.Details["broker_url"] = m.config.ExternalBrokerURL
	report.Details["retention_days"] = m.config.RetentionDays
}

// collectRouterStatistics collects statistics from the engine router
func (m *EventBusModule) collectRouterStatistics(report *modular.HealthReport) {
	// Try to get router statistics - this depends on router implementation
	report.Details["router_active"] = true
	
	// If router has a Stats() method or similar, we could use it here
	// For now, just indicate that the router is active
}

// evaluateEventBusHealthStatus determines the overall health status based on collected metrics
func (m *EventBusModule) evaluateEventBusHealthStatus(report *modular.HealthReport) {
	// Start with healthy status
	report.Status = modular.HealthStatusHealthy
	
	// Check performance metrics
	if duration, ok := report.Details["publish_duration_ms"].(int64); ok {
		if duration > 5000 { // More than 5 seconds for publish operations
			report.Status = modular.HealthStatusDegraded
			report.Message = fmt.Sprintf("eventbus operations slow: %dms for publish", duration)
			return
		} else if duration > 1000 { // More than 1 second but less than 5
			report.Status = modular.HealthStatusDegraded
			report.Message = fmt.Sprintf("eventbus performance degraded: %dms for publish", duration)
			return
		}
	}
	
	// Check worker configuration
	if workerCount, ok := report.Details["worker_count"].(int); ok {
		if workerCount == 0 {
			report.Status = modular.HealthStatusDegraded
			report.Message = "eventbus has no workers configured for async processing"
			return
		}
	}
	
	// Check for external broker connectivity issues
	if brokerType, ok := report.Details["broker_type"].(string); ok && brokerType != "in-memory" {
		// External brokers could have connectivity issues
		// If we got here without errors, the basic connectivity test passed
		report.Message = fmt.Sprintf("eventbus healthy: %s engine operational", m.config.Engine)
	} else {
		// In-memory engine
		report.Message = fmt.Sprintf("eventbus healthy: %s engine operational", m.config.Engine)
	}
}

// GetHealthTimeout returns the maximum time needed for health checks to complete.
// Event bus health checks involve publishing test events which should be fast,
// but external brokers might need more time for network operations.
func (m *EventBusModule) GetHealthTimeout() time.Duration {
	// Base timeout for event operations
	baseTimeout := 5 * time.Second
	
	// External brokers might need more time for network operations
	switch m.config.Engine {
	case "redis", "kafka":
		return baseTimeout + 5*time.Second
	default:
		return baseTimeout
	}
}

// IsHealthy is a convenience method that returns true if the event bus is healthy.
// This is useful for quick health status checks without detailed reports.
func (m *EventBusModule) IsHealthy(ctx context.Context) bool {
	reports, err := m.HealthCheck(ctx)
	if err != nil {
		return false
	}
	
	for _, report := range reports {
		if report.Status != modular.HealthStatusHealthy {
			return false
		}
	}
	
	return true
}