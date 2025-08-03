// Package modular provides Observer pattern interfaces for event-driven communication.
// These interfaces use CloudEvents specification for standardized event format
// and better interoperability with external systems.
package modular

import (
	"context"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Observer defines the interface for objects that want to be notified of events.
// Observers register with Subjects to receive notifications when events occur.
// This follows the traditional Observer pattern where observers are notified
// of state changes or events in subjects they're watching.
// Events use the CloudEvents specification for standardization.
type Observer interface {
	// OnEvent is called when an event occurs that the observer is interested in.
	// The context can be used for cancellation and timeouts.
	// Observers should handle events quickly to avoid blocking other observers.
	OnEvent(ctx context.Context, event cloudevents.Event) error

	// ObserverID returns a unique identifier for this observer.
	// This ID is used for registration tracking and debugging.
	ObserverID() string
}

// Subject defines the interface for objects that can be observed.
// Subjects maintain a list of observers and notify them when events occur.
// This is the core interface that event emitters implement.
// Events use the CloudEvents specification for standardization.
type Subject interface {
	// RegisterObserver adds an observer to receive notifications.
	// Observers can optionally filter events by type using the eventTypes parameter.
	// If eventTypes is empty, the observer receives all events.
	RegisterObserver(observer Observer, eventTypes ...string) error

	// UnregisterObserver removes an observer from receiving notifications.
	// This method should be idempotent and not error if the observer
	// wasn't registered.
	UnregisterObserver(observer Observer) error

	// NotifyObservers sends an event to all registered observers.
	// The notification process should be non-blocking for the caller
	// and handle observer errors gracefully.
	NotifyObservers(ctx context.Context, event cloudevents.Event) error

	// GetObservers returns information about currently registered observers.
	// This is useful for debugging and monitoring.
	GetObservers() []ObserverInfo
}

// ObserverInfo provides information about a registered observer.
// This is used for debugging, monitoring, and administrative interfaces.
type ObserverInfo struct {
	// ID is the unique identifier of the observer
	ID string `json:"id"`

	// EventTypes are the event types this observer is subscribed to.
	// Empty slice means all events.
	EventTypes []string `json:"eventTypes"`

	// RegisteredAt indicates when the observer was registered
	RegisteredAt time.Time `json:"registeredAt"`
}

// EventType constants for common application events.
// These provide a standardized vocabulary for CloudEvent types emitted by the core framework.
// Following CloudEvents specification, these use reverse domain notation.
const (
	// Module lifecycle events
	EventTypeModuleRegistered  = "com.modular.module.registered"
	EventTypeModuleInitialized = "com.modular.module.initialized"
	EventTypeModuleStarted     = "com.modular.module.started"
	EventTypeModuleStopped     = "com.modular.module.stopped"
	EventTypeModuleFailed      = "com.modular.module.failed"

	// Service lifecycle events
	EventTypeServiceRegistered   = "com.modular.service.registered"
	EventTypeServiceUnregistered = "com.modular.service.unregistered"
	EventTypeServiceRequested    = "com.modular.service.requested"

	// Configuration events
	EventTypeConfigLoaded    = "com.modular.config.loaded"
	EventTypeConfigValidated = "com.modular.config.validated"
	EventTypeConfigChanged   = "com.modular.config.changed"

	// Application lifecycle events
	EventTypeApplicationStarted = "com.modular.application.started"
	EventTypeApplicationStopped = "com.modular.application.stopped"
	EventTypeApplicationFailed  = "com.modular.application.failed"
)

// ObservableModule is an optional interface that modules can implement
// to participate in the observer pattern. Modules implementing this interface
// can emit their own events and register observers for events they're interested in.
// All events use the CloudEvents specification for standardization.
type ObservableModule interface {
	Module

	// RegisterObservers is called during module initialization to allow
	// the module to register as an observer for events it's interested in.
	// The subject parameter is typically the application itself.
	RegisterObservers(subject Subject) error

	// EmitEvent allows modules to emit their own CloudEvents.
	// This should typically delegate to the application's NotifyObservers method.
	EmitEvent(ctx context.Context, event cloudevents.Event) error
}

// FunctionalObserver provides a simple way to create observers using functions.
// This is useful for quick observer creation without defining full structs.
type FunctionalObserver struct {
	id      string
	handler func(ctx context.Context, event cloudevents.Event) error
}

// NewFunctionalObserver creates a new observer that uses the provided function
// to handle events. This is a convenience constructor for simple use cases.
func NewFunctionalObserver(id string, handler func(ctx context.Context, event cloudevents.Event) error) Observer {
	return &FunctionalObserver{
		id:      id,
		handler: handler,
	}
}

// OnEvent implements the Observer interface by calling the handler function.
func (f *FunctionalObserver) OnEvent(ctx context.Context, event cloudevents.Event) error {
	return f.handler(ctx, event)
}

// ObserverID implements the Observer interface by returning the observer ID.
func (f *FunctionalObserver) ObserverID() string {
	return f.id
}
