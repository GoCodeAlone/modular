// Package lifecycle defines interfaces for lifecycle event management and dispatching
package lifecycle

import (
	"context"
	"time"
)

// EventDispatcher defines the interface for dispatching lifecycle events
type EventDispatcher interface {
	// Dispatch sends a lifecycle event to all registered observers
	Dispatch(ctx context.Context, event *Event) error

	// RegisterObserver registers an observer to receive lifecycle events
	RegisterObserver(ctx context.Context, observer EventObserver) error

	// UnregisterObserver removes an observer from receiving events
	UnregisterObserver(ctx context.Context, observerID string) error

	// GetObservers returns all currently registered observers
	GetObservers(ctx context.Context) ([]EventObserver, error)

	// Start begins the event dispatcher service
	Start(ctx context.Context) error

	// Stop gracefully shuts down the event dispatcher
	Stop(ctx context.Context) error

	// IsRunning returns true if the dispatcher is currently running
	IsRunning() bool
}

// EventObserver defines the interface for observing lifecycle events
type EventObserver interface {
	// OnEvent is called when a lifecycle event is dispatched
	OnEvent(ctx context.Context, event *Event) error

	// ID returns the unique identifier for this observer
	ID() string

	// EventTypes returns the types of events this observer wants to receive
	EventTypes() []EventType

	// Priority returns the priority of this observer (higher = called first)
	Priority() int
}

// EventStore defines the interface for persisting and querying lifecycle events
type EventStore interface {
	// Store persists a lifecycle event
	Store(ctx context.Context, event *Event) error

	// Get retrieves a specific event by ID
	Get(ctx context.Context, eventID string) (*Event, error)

	// Query retrieves events matching the given criteria
	Query(ctx context.Context, criteria *QueryCriteria) ([]*Event, error)

	// Delete removes events matching the given criteria
	Delete(ctx context.Context, criteria *QueryCriteria) error

	// GetEventHistory returns event history for a specific source
	GetEventHistory(ctx context.Context, source string, since time.Time) ([]*Event, error)
}

// Event represents a lifecycle event
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Source    string                 `json:"source"` // module name, application, etc.
	Timestamp time.Time              `json:"timestamp"`
	Phase     LifecyclePhase         `json:"phase"`
	Status    EventStatus            `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Duration  *time.Duration         `json:"duration,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`

	// Correlation and tracing
	CorrelationID string `json:"correlation_id,omitempty"`
	ParentEventID string `json:"parent_event_id,omitempty"`
	TraceID       string `json:"trace_id,omitempty"`

	// Event versioning and schema
	Version   string `json:"version"`
	SchemaURL string `json:"schema_url,omitempty"`
}

// EventType defines the type of lifecycle event
type EventType string

const (
	EventTypeApplicationStarting  EventType = "application.starting"
	EventTypeApplicationStarted   EventType = "application.started"
	EventTypeApplicationStopping  EventType = "application.stopping"
	EventTypeApplicationStopped   EventType = "application.stopped"
	EventTypeModuleRegistering    EventType = "module.registering"
	EventTypeModuleRegistered     EventType = "module.registered"
	EventTypeModuleInitializing   EventType = "module.initializing"
	EventTypeModuleInitialized    EventType = "module.initialized"
	EventTypeModuleStarting       EventType = "module.starting"
	EventTypeModuleStarted        EventType = "module.started"
	EventTypeModuleStopping       EventType = "module.stopping"
	EventTypeModuleStopped        EventType = "module.stopped"
	EventTypeConfigurationLoading EventType = "configuration.loading"
	EventTypeConfigurationLoaded  EventType = "configuration.loaded"
	EventTypeConfigurationChanged EventType = "configuration.changed"
	EventTypeServiceRegistering   EventType = "service.registering"
	EventTypeServiceRegistered    EventType = "service.registered"
	EventTypeHealthCheckStarted   EventType = "health.check.started"
	EventTypeHealthCheckCompleted EventType = "health.check.completed"
	EventTypeHealthStatusChanged  EventType = "health.status.changed"
)

// LifecyclePhase represents the phase of the application/module lifecycle
type LifecyclePhase string

const (
	PhaseUnknown        LifecyclePhase = "unknown"
	PhaseRegistration   LifecyclePhase = "registration"
	PhaseInitialization LifecyclePhase = "initialization"
	PhaseConfiguration  LifecyclePhase = "configuration"
	PhaseStartup        LifecyclePhase = "startup"
	PhaseRunning        LifecyclePhase = "running"
	PhaseShutdown       LifecyclePhase = "shutdown"
	PhaseStopped        LifecyclePhase = "stopped"
)

// EventStatus represents the status of an event
type EventStatus string

const (
	EventStatusStarted   EventStatus = "started"
	EventStatusCompleted EventStatus = "completed"
	EventStatusFailed    EventStatus = "failed"
	EventStatusSkipped   EventStatus = "skipped"
)

// QueryCriteria defines criteria for querying events
type QueryCriteria struct {
	EventTypes    []EventType      `json:"event_types,omitempty"`
	Sources       []string         `json:"sources,omitempty"`
	Phases        []LifecyclePhase `json:"phases,omitempty"`
	Statuses      []EventStatus    `json:"statuses,omitempty"`
	Since         *time.Time       `json:"since,omitempty"`
	Until         *time.Time       `json:"until,omitempty"`
	CorrelationID string           `json:"correlation_id,omitempty"`
	TraceID       string           `json:"trace_id,omitempty"`
	Limit         int              `json:"limit,omitempty"`
	Offset        int              `json:"offset,omitempty"`
	OrderBy       string           `json:"order_by,omitempty"` // "timestamp", "type", "source"
	OrderDesc     bool             `json:"order_desc,omitempty"`
}

// DispatchConfig represents configuration for the event dispatcher
type DispatchConfig struct {
	BufferSize        int           `json:"buffer_size"`        // Event buffer size
	MaxRetries        int           `json:"max_retries"`        // Max retries for failed dispatch
	RetryDelay        time.Duration `json:"retry_delay"`        // Delay between retries
	ObserverTimeout   time.Duration `json:"observer_timeout"`   // Timeout for observer callbacks
	EnablePersistence bool          `json:"enable_persistence"` // Whether to persist events
	EnableMetrics     bool          `json:"enable_metrics"`     // Whether to collect metrics
}

// EventMetrics represents metrics about event processing
type EventMetrics struct {
	TotalEvents          int64                 `json:"total_events"`
	EventsByType         map[EventType]int64   `json:"events_by_type"`
	EventsByStatus       map[EventStatus]int64 `json:"events_by_status"`
	FailedDispatches     int64                 `json:"failed_dispatches"`
	AverageLatency       time.Duration         `json:"average_latency"`
	LastEventTime        time.Time             `json:"last_event_time"`
	ActiveObservers      int64                 `json:"active_observers"`
	BackpressureWarnings int64                 `json:"backpressure_warnings"`
	DispatchErrors       int64                 `json:"dispatch_errors"`
	ObserverErrors       int64                 `json:"observer_errors"`
	ObserverPanics       int64                 `json:"observer_panics"`
}
