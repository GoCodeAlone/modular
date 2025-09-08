package modular

import (
	"fmt"
	"time"
)

// HealthStatus represents the overall health state of a component or service.
// It follows a standard set of states that can be used for monitoring and alerting.
type HealthStatus int

const (
	// HealthStatusUnknown indicates that the health status cannot be determined.
	// This is typically used when health checks are not yet complete or have failed
	// to execute due to timeouts or other issues.
	HealthStatusUnknown HealthStatus = iota

	// HealthStatusHealthy indicates that the component is operating normally.
	// All health checks are passing and the component is ready to serve requests.
	HealthStatusHealthy

	// HealthStatusDegraded indicates that the component is operational but
	// not performing optimally. Some non-critical functionality may be impaired.
	HealthStatusDegraded

	// HealthStatusUnhealthy indicates that the component is not functioning
	// properly and may not be able to serve requests reliably.
	HealthStatusUnhealthy
)

// String returns the string representation of the health status.
func (s HealthStatus) String() string {
	switch s {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusDegraded:
		return "degraded"
	case HealthStatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// IsHealthy returns true if the status represents a healthy state
func (s HealthStatus) IsHealthy() bool {
	return s == HealthStatusHealthy
}

// HealthReport represents a health report as defined in the design brief for FR-048.
// This structure provides detailed information about the health of a specific
// module or component, including timing and observability information.
type HealthReport struct {
	// Module is the identifier for the module that provides this health check
	Module string `json:"module"`

	// Component is an optional identifier for the specific component within the module
	// (e.g., "database-connection", "cache-client", "worker-pool")
	Component string `json:"component,omitempty"`

	// Status is the health status determined by the check
	Status HealthStatus `json:"status"`

	// Message provides human-readable details about the health status.
	// This should be concise but informative for debugging and monitoring.
	Message string `json:"message,omitempty"`

	// CheckedAt indicates when the health check was performed
	CheckedAt time.Time `json:"checkedAt"`

	// ObservedSince indicates when this status was first observed
	// This helps track how long a component has been in its current state
	ObservedSince time.Time `json:"observedSince"`

	// Optional indicates whether this component is optional for overall readiness
	// Optional components don't affect the readiness status but are included in health
	Optional bool `json:"optional"`

	// Details contains additional structured information about the health check
	// This can include metrics, diagnostic information, or other contextual data
	Details map[string]any `json:"details,omitempty"`
}

// HealthResult contains the result of a health check operation.
// It includes the status, timing information, and optional metadata
// about the health check execution.
//
// Deprecated: Use HealthReport instead for new implementations.
// This type is maintained for backward compatibility.
type HealthResult struct {
	// Status is the overall health status determined by the check
	Status HealthStatus

	// Message provides human-readable details about the health status.
	// This should be concise but informative for debugging and monitoring.
	Message string

	// Timestamp indicates when the health check was performed
	Timestamp time.Time

	// CheckDuration is the time it took to complete the health check
	CheckDuration time.Duration

	// Details provides detailed information about the health check
	// This can include additional diagnostic information, nested results, etc.
	Details map[string]interface{}

	// Metadata contains additional key-value pairs with health check details.
	// This can include metrics, error details, or other contextual information.
	Metadata map[string]interface{}
}

// HealthComponent represents the health information for a single component
// within an aggregate health snapshot.
type HealthComponent struct {
	// Name is the identifier for this component
	Name string

	// Status is the health status of this component
	Status HealthStatus

	// Message provides details about the component's health
	Message string

	// CheckDuration is how long the health check took
	CheckDuration time.Duration

	// LastChecked indicates when this component was last evaluated
	LastChecked time.Time

	// Metadata contains additional component-specific health information
	Metadata map[string]interface{}
}

// HealthSummary provides a summary of health check results
type HealthSummary struct {
	// HealthyCount is the number of healthy components
	HealthyCount int
	
	// TotalCount is the total number of components checked
	TotalCount int
	
	// DegradedCount is the number of degraded components
	DegradedCount int
	
	// UnhealthyCount is the number of unhealthy components
	UnhealthyCount int
}

// AggregatedHealth represents the combined health status of multiple components
// as defined in the design brief for FR-048. This structure provides distinct
// readiness and health status along with individual component reports.
type AggregatedHealth struct {
	// Readiness indicates whether the system is ready to accept traffic
	// This only considers non-optional (required) components
	Readiness HealthStatus `json:"readiness"`

	// Health indicates the overall health status across all components
	// This includes both required and optional components
	Health HealthStatus `json:"health"`

	// Reports contains the individual health reports from all providers
	Reports []HealthReport `json:"reports"`

	// GeneratedAt indicates when this aggregated health was collected
	GeneratedAt time.Time `json:"generatedAt"`
}

// AggregateHealthSnapshot represents the combined health status of multiple components
// at a specific point in time. This is used for system-wide health monitoring.
//
// Deprecated: Use AggregatedHealth instead for new implementations.
// This type is maintained for backward compatibility.
type AggregateHealthSnapshot struct {
	// OverallStatus is the aggregated health status across all components
	OverallStatus HealthStatus

	// ReadinessStatus indicates whether the system is ready to serve requests
	// This may differ from OverallStatus in cases where a system is degraded
	// but still ready to serve traffic
	ReadinessStatus HealthStatus

	// Components contains the individual health status for each monitored component
	// Using HealthResult for compatibility with existing tests
	Components map[string]HealthResult

	// Summary provides a summary of the health check results
	Summary HealthSummary

	// GeneratedAt indicates when this snapshot was created
	GeneratedAt time.Time

	// Timestamp is an alias for GeneratedAt for compatibility
	Timestamp time.Time

	// SnapshotID is a unique identifier for this health snapshot,
	// useful for tracking and correlation in logs and monitoring systems
	SnapshotID string

	// Metadata contains additional system-wide health information
	Metadata map[string]interface{}
}

// IsHealthy returns true if the overall status is healthy
func (s *AggregateHealthSnapshot) IsHealthy() bool {
	return s.OverallStatus == HealthStatusHealthy
}

// IsReady returns true if the system is ready to serve requests
func (s *AggregateHealthSnapshot) IsReady() bool {
	return s.ReadinessStatus == HealthStatusHealthy || s.ReadinessStatus == HealthStatusDegraded
}

// GetUnhealthyComponents returns a slice of component names that are not healthy
func (s *AggregateHealthSnapshot) GetUnhealthyComponents() []string {
	var unhealthy []string
	for name, component := range s.Components {
		if component.Status != HealthStatusHealthy {
			unhealthy = append(unhealthy, name)
		}
	}
	return unhealthy
}

// HealthTrigger represents what triggered a health evaluation
type HealthTrigger int

const (
	// HealthTriggerThreshold indicates the health check was triggered by a threshold
	HealthTriggerThreshold HealthTrigger = iota
	
	// HealthTriggerScheduled indicates the health check was triggered by a schedule
	HealthTriggerScheduled
	
	// HealthTriggerOnDemand indicates the health check was triggered manually/on-demand
	HealthTriggerOnDemand
	
	// HealthTriggerStartup indicates the health check was triggered at startup
	HealthTriggerStartup
	
	// HealthTriggerPostReload indicates the health check was triggered after a config reload
	HealthTriggerPostReload
)

// String returns the string representation of the health trigger
func (h HealthTrigger) String() string {
	switch h {
	case HealthTriggerThreshold:
		return "threshold"
	case HealthTriggerScheduled:
		return "scheduled"
	case HealthTriggerOnDemand:
		return "on-demand"
	case HealthTriggerStartup:
		return "startup"
	case HealthTriggerPostReload:
		return "post-reload"
	default:
		return "unknown"
	}
}

// ParseHealthTrigger parses a string into a HealthTrigger
func ParseHealthTrigger(s string) (HealthTrigger, error) {
	switch s {
	case "threshold":
		return HealthTriggerThreshold, nil
	case "scheduled":
		return HealthTriggerScheduled, nil
	case "on-demand":
		return HealthTriggerOnDemand, nil
	case "startup":
		return HealthTriggerStartup, nil
	case "post-reload":
		return HealthTriggerPostReload, nil
	default:
		return 0, fmt.Errorf("invalid health trigger: %s", s)
	}
}

// HealthEvaluatedEvent represents an event emitted when health evaluation completes
type HealthEvaluatedEvent struct {
	// EvaluationID is a unique identifier for this health evaluation
	EvaluationID string
	
	// Timestamp indicates when the evaluation was performed
	Timestamp time.Time
	
	// Snapshot contains the health snapshot result
	Snapshot AggregateHealthSnapshot
	
	// Duration indicates how long the evaluation took
	Duration time.Duration
	
	// TriggerType indicates what triggered this health evaluation
	TriggerType HealthTrigger
	
	// StatusChanged indicates whether the health status changed from the previous evaluation
	StatusChanged bool
	
	// PreviousStatus contains the previous health status if it changed
	PreviousStatus HealthStatus
	
	// Metrics contains additional metrics about the health evaluation
	Metrics *HealthEvaluationMetrics
}

// EventType returns the standardized event type for health evaluations
func (e *HealthEvaluatedEvent) EventType() string {
	return "health.evaluated"
}

// EventSource returns the standardized event source for health evaluations
func (e *HealthEvaluatedEvent) EventSource() string {
	return "modular.core.health"
}

// GetEventType returns the type identifier for this event (implements ObserverEvent)
func (e *HealthEvaluatedEvent) GetEventType() string {
	return e.EventType()
}

// GetEventSource returns the source that generated this event (implements ObserverEvent)
func (e *HealthEvaluatedEvent) GetEventSource() string {
	return e.EventSource()
}

// GetTimestamp returns when this event occurred (implements ObserverEvent)
func (e *HealthEvaluatedEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// StructuredFields returns the structured field data for this event
func (e *HealthEvaluatedEvent) StructuredFields() map[string]interface{} {
	fields := map[string]interface{}{
		"evaluation_id": e.EvaluationID,
		"duration_ms":   e.Duration.Milliseconds(),
		"trigger_type":  e.TriggerType.String(),
		"overall_status": e.Snapshot.OverallStatus.String(),
	}
	
	if e.StatusChanged {
		fields["status_changed"] = true
		fields["previous_status"] = e.PreviousStatus.String()
	}
	
	// Add metrics if available
	if e.Metrics != nil {
		fields["components_evaluated"] = e.Metrics.ComponentsEvaluated
		fields["failed_evaluations"] = e.Metrics.FailedEvaluations
		fields["average_response_time_ms"] = e.Metrics.AverageResponseTimeMs
	}
	
	return fields
}

// Additional types and functions needed for tests to compile
type HealthEvaluationMetrics struct {
	ComponentsEvaluated   int
	FailedEvaluations     int
	AverageResponseTimeMs float64
	ComponentsSkipped     int
	ComponentsTimedOut    int
	TotalEvaluationTime   time.Duration
	SlowestComponentName  string
	SlowestComponentTime  time.Duration
}

// CalculateEfficiency returns the efficiency percentage of the health evaluation
func (h *HealthEvaluationMetrics) CalculateEfficiency() float64 {
	if h.ComponentsEvaluated == 0 {
		return 0.0
	}
	successful := h.ComponentsEvaluated - h.FailedEvaluations - h.ComponentsSkipped - h.ComponentsTimedOut
	return (float64(successful) / float64(h.ComponentsEvaluated)) * 100.0
}

// HasPerformanceBottleneck returns true if there are performance bottlenecks
func (h *HealthEvaluationMetrics) HasPerformanceBottleneck() bool {
	return h.SlowestComponentTime > 500*time.Millisecond || h.AverageResponseTimeMs > 200.0
}

// BottleneckPercentage returns the percentage of components that are bottlenecks
func (h *HealthEvaluationMetrics) BottleneckPercentage() float64 {
	if h.ComponentsEvaluated == 0 {
		return 0.0
	}
	// For simplicity, consider a bottleneck if slowest component is more than 2x average
	if h.AverageResponseTimeMs == 0 {
		return 0.0
	}
	slowestMs := float64(h.SlowestComponentTime.Milliseconds())
	if slowestMs > h.AverageResponseTimeMs*2 {
		return 10.0 // Simplified: assume 10% are bottlenecks if there's a slow component
	}
	return 0.0
}

// Filter functions for health events
func FilterHealthEventsByStatusChange(events []ObserverEvent, statusChanged bool) []ObserverEvent {
	var filtered []ObserverEvent
	for _, event := range events {
		if healthEvent, ok := event.(*HealthEvaluatedEvent); ok {
			if healthEvent.StatusChanged == statusChanged {
				filtered = append(filtered, event)
			}
		}
	}
	return filtered
}

func FilterHealthEventsByTrigger(events []ObserverEvent, trigger HealthTrigger) []ObserverEvent {
	var filtered []ObserverEvent
	for _, event := range events {
		if healthEvent, ok := event.(*HealthEvaluatedEvent); ok {
			if healthEvent.TriggerType == trigger {
				filtered = append(filtered, event)
			}
		}
	}
	return filtered
}

func FilterHealthEventsByStatus(events []ObserverEvent, status HealthStatus) []ObserverEvent {
	var filtered []ObserverEvent
	for _, event := range events {
		if healthEvent, ok := event.(*HealthEvaluatedEvent); ok {
			if healthEvent.Snapshot.OverallStatus == status {
				filtered = append(filtered, event)
			}
		}
	}
	return filtered
}