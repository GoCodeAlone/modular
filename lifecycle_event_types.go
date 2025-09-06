package modular

import (
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// LifecycleEvent represents a structured event during module/application lifecycle
type LifecycleEvent struct {
	// ID is a unique identifier for this event
	ID string

	// Type indicates the type of lifecycle event
	Type LifecycleEventType

	// Phase indicates which lifecycle phase this event is for
	Phase LifecyclePhase

	// ModuleName is the name of the module this event relates to (if applicable)
	ModuleName string

	// ModuleType is the type of the module (if applicable)
	ModuleType string

	// Timestamp is when this event occurred
	Timestamp time.Time

	// Duration indicates how long the lifecycle phase took (for completion events)
	Duration *time.Duration

	// Status indicates the result of the lifecycle phase
	Status LifecycleEventStatus

	// Error contains error information if the event represents a failure
	Error *LifecycleEventError

	// Metadata contains additional context-specific information
	Metadata map[string]interface{}

	// CorrelationID links related events together
	CorrelationID string

	// Dependencies lists module dependencies relevant to this event
	Dependencies []string

	// Services lists services provided/required relevant to this event
	Services []string

	// CloudEvent is the underlying CloudEvents representation
	CloudEvent *cloudevents.Event
}

// LifecycleEventType represents the type of lifecycle event
type LifecycleEventType string

const (
	// LifecycleEventTypeRegistering indicates module registration phase
	LifecycleEventTypeRegistering LifecycleEventType = "registering"

	// LifecycleEventTypeStarting indicates module start phase
	LifecycleEventTypeStarting LifecycleEventType = "starting"

	// LifecycleEventTypeStarted indicates module started successfully
	LifecycleEventTypeStarted LifecycleEventType = "started"

	// LifecycleEventTypeStopping indicates module stop phase
	LifecycleEventTypeStopping LifecycleEventType = "stopping"

	// LifecycleEventTypeStopped indicates module stopped successfully
	LifecycleEventTypeStopped LifecycleEventType = "stopped"

	// LifecycleEventTypeError indicates an error occurred
	LifecycleEventTypeError LifecycleEventType = "error"

	// LifecycleEventTypeConfigurationChange indicates configuration change
	LifecycleEventTypeConfigurationChange LifecycleEventType = "configuration_change"
)

// LifecyclePhase represents which phase of the lifecycle the event is for
type LifecyclePhase string

const (
	// LifecyclePhaseRegistration indicates the registration phase
	LifecyclePhaseRegistration LifecyclePhase = "registration"

	// LifecyclePhaseInitialization indicates the initialization phase
	LifecyclePhaseInitialization LifecyclePhase = "initialization"

	// LifecyclePhaseStartup indicates the startup phase
	LifecyclePhaseStartup LifecyclePhase = "startup"

	// LifecyclePhaseRuntime indicates the runtime phase
	LifecyclePhaseRuntime LifecyclePhase = "runtime"

	// LifecyclePhaseShutdown indicates the shutdown phase
	LifecyclePhaseShutdown LifecyclePhase = "shutdown"
)

// LifecycleEventStatus represents the status of a lifecycle event
type LifecycleEventStatus string

const (
	// LifecycleEventStatusSuccess indicates successful completion
	LifecycleEventStatusSuccess LifecycleEventStatus = "success"

	// LifecycleEventStatusFailure indicates failure
	LifecycleEventStatusFailure LifecycleEventStatus = "failure"

	// LifecycleEventStatusInProgress indicates operation in progress
	LifecycleEventStatusInProgress LifecycleEventStatus = "in_progress"

	// LifecycleEventStatusSkipped indicates operation was skipped
	LifecycleEventStatusSkipped LifecycleEventStatus = "skipped"
)

// LifecycleEventError represents error information in a lifecycle event
type LifecycleEventError struct {
	// Type is the error type/category
	Type string

	// Message is the human-readable error message
	Message string

	// Code is a machine-readable error code
	Code string

	// Stack contains stack trace information (if available)
	Stack string

	// Cause references the underlying cause error
	Cause string

	// Recoverable indicates if this error is recoverable
	Recoverable bool
}
