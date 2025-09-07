// Package health defines interfaces for health monitoring and aggregation services
package health

import (
	"context"
	"time"
)

// HealthChecker defines the interface for individual health check implementations
type HealthChecker interface {
	// Check performs a health check and returns the current status
	Check(ctx context.Context) (*CheckResult, error)

	// Name returns the unique name of this health check
	Name() string

	// Description returns a human-readable description of what this check validates
	Description() string
}

// HealthAggregator defines the interface for aggregating multiple health checks
type HealthAggregator interface {
	// RegisterCheck registers a health check with the aggregator
	RegisterCheck(ctx context.Context, checker HealthChecker) error

	// UnregisterCheck removes a health check from the aggregator
	UnregisterCheck(ctx context.Context, name string) error

	// CheckAll runs all registered health checks and returns aggregated status
	CheckAll(ctx context.Context) (*AggregatedStatus, error)

	// CheckOne runs a specific health check by name
	CheckOne(ctx context.Context, name string) (*CheckResult, error)

	// GetStatus returns the current aggregated health status without running checks
	GetStatus(ctx context.Context) (*AggregatedStatus, error)

	// IsReady returns true if the system is ready to accept traffic
	IsReady(ctx context.Context) (bool, error)

	// IsLive returns true if the system is alive (for liveness probes)
	IsLive(ctx context.Context) (bool, error)
}

// HealthMonitor defines the interface for continuous health monitoring
type HealthMonitor interface {
	// StartMonitoring begins continuous health monitoring with the specified interval
	StartMonitoring(ctx context.Context, interval time.Duration) error

	// StopMonitoring stops continuous health monitoring
	StopMonitoring(ctx context.Context) error

	// IsMonitoring returns true if monitoring is currently active
	IsMonitoring() bool

	// GetHistory returns health check history for analysis
	GetHistory(ctx context.Context, checkName string, since time.Time) ([]*CheckResult, error)

	// SetCallback sets a callback function to be called on status changes
	SetCallback(callback StatusChangeCallback) error
}

// CheckResult represents the result of a single health check
type CheckResult struct {
	Name      string                 `json:"name"`
	Status    HealthStatus           `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`

	// Check-specific details
	Details map[string]interface{} `json:"details,omitempty"`

	// Trend information
	ConsecutiveFailures  int `json:"consecutive_failures"`
	ConsecutiveSuccesses int `json:"consecutive_successes"`
}

// AggregatedStatus represents the aggregated status of all health checks
type AggregatedStatus struct {
	OverallStatus   HealthStatus            `json:"overall_status"`
	ReadinessStatus HealthStatus            `json:"readiness_status"`
	LivenessStatus  HealthStatus            `json:"liveness_status"`
	Timestamp       time.Time               `json:"timestamp"`
	CheckResults    map[string]*CheckResult `json:"check_results"`
	Summary         *StatusSummary          `json:"summary"`
	Metadata        map[string]interface{}  `json:"metadata,omitempty"`
}

// StatusSummary provides a summary of health check results
type StatusSummary struct {
	TotalChecks    int `json:"total_checks"`
	PassingChecks  int `json:"passing_checks"`
	WarningChecks  int `json:"warning_checks"`
	CriticalChecks int `json:"critical_checks"`
	FailingChecks  int `json:"failing_checks"`
	UnknownChecks  int `json:"unknown_checks"`
}

// HealthStatus represents the status of a health check
type HealthStatus string

const (
	StatusHealthy  HealthStatus = "healthy"
	StatusWarning  HealthStatus = "warning"
	StatusCritical HealthStatus = "critical"
	StatusUnknown  HealthStatus = "unknown"
)

// CheckType defines the type of health check for categorization
type CheckType string

const (
	CheckTypeLiveness   CheckType = "liveness"  // For liveness probes
	CheckTypeReadiness  CheckType = "readiness" // For readiness probes
	CheckTypeGeneral    CheckType = "general"   // General health monitoring
	CheckTypeDeepHealth CheckType = "deep"      // Deep health checks (slower)
)

// CheckConfig represents configuration for a health check
type CheckConfig struct {
	Name                string                 `json:"name"`
	Type                CheckType              `json:"type"`
	Interval            time.Duration          `json:"interval"`
	Timeout             time.Duration          `json:"timeout"`
	FailureThreshold    int                    `json:"failure_threshold"`
	SuccessThreshold    int                    `json:"success_threshold"`
	InitialDelaySeconds int                    `json:"initial_delay_seconds"`
	Enabled             bool                   `json:"enabled"`
	Tags                []string               `json:"tags,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// StatusChangeCallback is called when health status changes
type StatusChangeCallback func(ctx context.Context, previous, current *AggregatedStatus) error
