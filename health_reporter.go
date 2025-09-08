package modular

import (
	"context"
	"time"
)

// HealthProvider defines the interface for components that can report their health status.
// This interface follows the design brief specification for FR-048 Health Aggregation,
// providing structured health reports with module and component information.
//
// Components implementing this interface can participate in system-wide health monitoring
// and provide detailed information about their operational state.
//
// Health checks should be:
//   - Fast: typically complete within a few seconds
//   - Reliable: not prone to false positives/negatives
//   - Meaningful: accurately reflect the component's ability to serve requests
//   - Non-disruptive: not impact normal operations when executed
type HealthProvider interface {
	// HealthCheck performs a health check and returns health reports.
	// The context can be used to timeout long-running health checks.
	//
	// Implementations should:
	//   - Respect context cancellation and timeouts
	//   - Return meaningful status and messages
	//   - Include relevant metadata for debugging
	//   - Be idempotent and safe to call repeatedly
	//
	// Returns a slice of HealthReport objects, allowing a single provider
	// to report on multiple components or aspects of the service.
	HealthCheck(ctx context.Context) ([]HealthReport, error)
}

// HealthReporter defines the legacy interface for backward compatibility.
// New implementations should use HealthProvider instead.
//
// Deprecated: Use HealthProvider interface instead. This interface is maintained
// for backward compatibility but will be removed in a future version.
type HealthReporter interface {
	// CheckHealth performs a health check and returns the current status.
	// The context can be used to timeout long-running health checks.
	//
	// Implementations should:
	//   - Respect context cancellation and timeouts
	//   - Return meaningful status and messages
	//   - Include relevant metadata for debugging
	//   - Be idempotent and safe to call repeatedly
	//
	// The returned HealthResult should always be valid, even if the check fails.
	CheckHealth(ctx context.Context) HealthResult

	// HealthCheckName returns a human-readable name for this health check.
	// This name is used in logs, metrics, and health dashboards.
	// It should be unique within the application and descriptive of what is being checked.
	HealthCheckName() string

	// HealthCheckTimeout returns the maximum time this health check needs to complete.
	// This is used by health aggregators to set appropriate context timeouts.
	//
	// Typical values:
	//   - Simple checks (memory, CPU): 1-5 seconds
	//   - Database connectivity: 5-15 seconds
	//   - External service calls: 10-30 seconds
	//
	// A zero duration indicates the health check should use a reasonable default timeout.
	HealthCheckTimeout() time.Duration
}

// HealthAggregator interface defines how health reports are collected and aggregated
// as specified in the design brief for FR-048.
type HealthAggregator interface {
	// Collect gathers health reports from all registered providers and
	// returns an aggregated view of the system's health status.
	// The context can be used to timeout the collection process.
	Collect(ctx context.Context) (AggregatedHealth, error)
}

// ObserverEvent represents an event that can be observed in the system.
// This is a generic interface that allows different event types to be handled uniformly.
type ObserverEvent interface {
	// GetEventType returns the type identifier for this event
	GetEventType() string
	
	// GetEventSource returns the source that generated this event
	GetEventSource() string
	
	// GetTimestamp returns when this event occurred
	GetTimestamp() time.Time
}