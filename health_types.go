package modular

import (
	"time"
)

// HealthStatus represents the health status of a component
type HealthStatus struct {
	// Status is the overall health state
	Status HealthState

	// Message provides human-readable status description
	Message string

	// Timestamp indicates when this status was last updated
	Timestamp time.Time

	// ModuleName is the name of the module this status relates to
	ModuleName string

	// Details contains component-specific health details
	Details map[string]interface{}

	// Checks contains results of individual health checks
	Checks []HealthCheckResult

	// Duration indicates how long the health check took
	Duration time.Duration

	// Version is the module version reporting this status
	Version string

	// Critical indicates if this component is critical for overall health
	Critical bool

	// Trend indicates if health is improving, degrading, or stable
	Trend HealthTrend
}

// HealthState represents the possible health states
type HealthState string

const (
	// HealthStateHealthy indicates the component is functioning normally
	HealthStateHealthy HealthState = "healthy"

	// HealthStateDegraded indicates the component has issues but is functional
	HealthStateDegraded HealthState = "degraded"

	// HealthStateUnhealthy indicates the component is not functioning properly
	HealthStateUnhealthy HealthState = "unhealthy"

	// HealthStateUnknown indicates the health state cannot be determined
	HealthStateUnknown HealthState = "unknown"
)

// HealthTrend indicates the direction of health change
type HealthTrend string

const (
	// HealthTrendStable indicates health is stable
	HealthTrendStable HealthTrend = "stable"

	// HealthTrendImproving indicates health is improving
	HealthTrendImproving HealthTrend = "improving"

	// HealthTrendDegrading indicates health is degrading
	HealthTrendDegrading HealthTrend = "degrading"
)

// HealthCheckResult represents the result of an individual health check
type HealthCheckResult struct {
	// Name is the name of this health check
	Name string

	// Status is the result of this check
	Status HealthState

	// Message provides details about this check result
	Message string

	// Timestamp indicates when this check was performed
	Timestamp time.Time

	// Duration indicates how long this check took
	Duration time.Duration

	// Error contains error information if the check failed
	Error string

	// Metadata contains check-specific metadata
	Metadata map[string]interface{}
}

// ReadinessStatus represents the readiness status of a component or system
type ReadinessStatus struct {
	// Ready indicates if the component is ready to serve requests
	Ready bool

	// Message provides human-readable readiness description
	Message string

	// Timestamp indicates when this status was last updated
	Timestamp time.Time

	// RequiredModules lists modules that must be healthy for readiness
	RequiredModules []string

	// OptionalModules lists modules that don't affect readiness
	OptionalModules []string

	// FailedModules lists modules that are currently failing
	FailedModules []string

	// Details contains readiness-specific details
	Details map[string]interface{}
}

// AggregatedHealthStatus represents the overall health across all modules
type AggregatedHealthStatus struct {
	// OverallStatus is the worst status among all modules
	OverallStatus HealthState

	// ReadinessStatus indicates if the system is ready
	ReadinessStatus ReadinessStatus

	// ModuleStatuses contains health status for each module
	ModuleStatuses map[string]HealthStatus

	// TotalModules is the total number of modules
	TotalModules int

	// HealthyModules is the number of healthy modules
	HealthyModules int

	// DegradedModules is the number of degraded modules
	DegradedModules int

	// UnhealthyModules is the number of unhealthy modules
	UnhealthyModules int

	// Timestamp indicates when this aggregation was performed
	Timestamp time.Time

	// Summary provides a high-level summary of system health
	Summary string
}
