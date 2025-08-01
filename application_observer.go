package modular

import (
	"context"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// observerRegistration holds information about a registered observer
type observerRegistration struct {
	observer     Observer
	eventTypes   map[string]bool // set of event types this observer is interested in
	registeredAt time.Time
}

// ObservableApplication extends StdApplication with observer pattern capabilities.
// This struct embeds StdApplication and adds observer management functionality.
// It uses CloudEvents specification for standardized event handling and interoperability.
type ObservableApplication struct {
	*StdApplication
	observers     map[string]*observerRegistration // key is observer ID
	observerMutex sync.RWMutex
}

// NewObservableApplication creates a new application instance with observer pattern support.
// This wraps the standard application with observer capabilities while maintaining
// all existing functionality.
func NewObservableApplication(cp ConfigProvider, logger Logger) *ObservableApplication {
	stdApp := NewStdApplication(cp, logger).(*StdApplication)
	return &ObservableApplication{
		StdApplication: stdApp,
		observers:      make(map[string]*observerRegistration),
	}
}

// RegisterObserver adds an observer to receive notifications from the application.
// Observers can optionally filter events by type using the eventTypes parameter.
// If eventTypes is empty, the observer receives all events.
func (app *ObservableApplication) RegisterObserver(observer Observer, eventTypes ...string) error {
	app.observerMutex.Lock()
	defer app.observerMutex.Unlock()

	// Convert event types slice to map for O(1) lookups
	eventTypeMap := make(map[string]bool)
	for _, eventType := range eventTypes {
		eventTypeMap[eventType] = true
	}

	app.observers[observer.ObserverID()] = &observerRegistration{
		observer:     observer,
		eventTypes:   eventTypeMap,
		registeredAt: time.Now(),
	}

	app.logger.Info("Observer registered", "observerID", observer.ObserverID(), "eventTypes", eventTypes)
	return nil
}

// UnregisterObserver removes an observer from receiving notifications.
// This method is idempotent and won't error if the observer wasn't registered.
func (app *ObservableApplication) UnregisterObserver(observer Observer) error {
	app.observerMutex.Lock()
	defer app.observerMutex.Unlock()

	if _, exists := app.observers[observer.ObserverID()]; exists {
		delete(app.observers, observer.ObserverID())
		app.logger.Info("Observer unregistered", "observerID", observer.ObserverID())
	}

	return nil
}

// NotifyObservers sends a CloudEvent to all registered observers.
// The notification process is non-blocking for the caller and handles observer errors gracefully.
func (app *ObservableApplication) NotifyObservers(ctx context.Context, event cloudevents.Event) error {
	app.observerMutex.RLock()
	defer app.observerMutex.RUnlock()

	// Ensure timestamp is set
	if event.Time().IsZero() {
		event.SetTime(time.Now())
	}

	// Validate the CloudEvent
	if err := ValidateCloudEvent(event); err != nil {
		app.logger.Error("Invalid CloudEvent", "eventType", event.Type(), "error", err)
		return err
	}

	// Notify observers in goroutines to avoid blocking
	for _, registration := range app.observers {
		registration := registration // capture for goroutine

		// Check if observer is interested in this event type
		if len(registration.eventTypes) > 0 && !registration.eventTypes[event.Type()] {
			continue // observer not interested in this event type
		}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					app.logger.Error("Observer panicked", "observerID", registration.observer.ObserverID(), "event", event.Type(), "panic", r)
				}
			}()

			if err := registration.observer.OnEvent(ctx, event); err != nil {
				app.logger.Error("Observer error", "observerID", registration.observer.ObserverID(), "event", event.Type(), "error", err)
			}
		}()
	}

	return nil
}

// emitEvent is a helper method to emit CloudEvents with proper source information
func (app *ObservableApplication) emitEvent(ctx context.Context, eventType string, data interface{}, metadata map[string]interface{}) {
	event := NewCloudEvent(eventType, "application", data, metadata)

	// Use a separate goroutine to avoid blocking application operations
	go func() {
		if err := app.NotifyObservers(ctx, event); err != nil {
			app.logger.Error("Failed to notify observers", "event", eventType, "error", err)
		}
	}()
}

// GetObservers returns information about currently registered observers.
// This is useful for debugging and monitoring.
func (app *ObservableApplication) GetObservers() []ObserverInfo {
	app.observerMutex.RLock()
	defer app.observerMutex.RUnlock()

	info := make([]ObserverInfo, 0, len(app.observers))
	for _, registration := range app.observers {
		eventTypes := make([]string, 0, len(registration.eventTypes))
		for eventType := range registration.eventTypes {
			eventTypes = append(eventTypes, eventType)
		}

		info = append(info, ObserverInfo{
			ID:           registration.observer.ObserverID(),
			EventTypes:   eventTypes,
			RegisteredAt: registration.registeredAt,
		})
	}

	return info
}

// Override key methods to emit events

// RegisterModule registers a module and emits CloudEvent
func (app *ObservableApplication) RegisterModule(module Module) {
	app.StdApplication.RegisterModule(module)

	data := map[string]interface{}{
		"moduleName": module.Name(),
		"moduleType": getTypeName(module),
	}

	// Emit CloudEvent for standardized event handling
	app.emitEvent(context.Background(), EventTypeModuleRegistered, data, nil)
}

// RegisterService registers a service and emits CloudEvent
func (app *ObservableApplication) RegisterService(name string, service any) error {
	err := app.StdApplication.RegisterService(name, service)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"serviceName": name,
		"serviceType": getTypeName(service),
	}

	// Emit CloudEvent for standardized event handling
	app.emitEvent(context.Background(), EventTypeServiceRegistered, data, nil)

	return nil
}

// Init initializes the application and emits lifecycle events
func (app *ObservableApplication) Init() error {
	ctx := context.Background()

	// Emit application starting initialization
	app.emitEvent(ctx, EventTypeConfigLoaded, nil, map[string]interface{}{
		"phase": "init_start",
	})

	err := app.StdApplication.Init()
	if err != nil {
		failureData := map[string]interface{}{
			"phase": "init",
			"error": err.Error(),
		}
		app.emitEvent(ctx, EventTypeApplicationFailed, failureData, nil)
		return err
	}

	// Register observers for any ObservableModule instances
	for _, module := range app.moduleRegistry {
		if observableModule, ok := module.(ObservableModule); ok {
			if err := observableModule.RegisterObservers(app); err != nil {
				app.logger.Error("Failed to register observers for module", "module", module.Name(), "error", err)
			}
		}
	}

	// Emit initialization complete
	app.emitEvent(ctx, EventTypeConfigValidated, nil, map[string]interface{}{
		"phase": "init_complete",
	})

	return nil
}

// Start starts the application and emits lifecycle events
func (app *ObservableApplication) Start() error {
	ctx := context.Background()

	err := app.StdApplication.Start()
	if err != nil {
		failureData := map[string]interface{}{
			"phase": "start",
			"error": err.Error(),
		}
		app.emitEvent(ctx, EventTypeApplicationFailed, failureData, nil)
		return err
	}

	// Emit application started event
	app.emitEvent(ctx, EventTypeApplicationStarted, nil, nil)

	return nil
}

// Stop stops the application and emits lifecycle events
func (app *ObservableApplication) Stop() error {
	ctx := context.Background()

	err := app.StdApplication.Stop()
	if err != nil {
		failureData := map[string]interface{}{
			"phase": "stop",
			"error": err.Error(),
		}
		app.emitEvent(ctx, EventTypeApplicationFailed, failureData, nil)
		return err
	}

	// Emit application stopped event
	app.emitEvent(ctx, EventTypeApplicationStopped, nil, nil)

	return nil
}

// getTypeName returns the type name of an interface{} value
func getTypeName(v interface{}) string {
	if v == nil {
		return "nil"
	}

	// Use reflection to get the type name
	// This is a simplified version that gets the basic type name
	switch v := v.(type) {
	case Module:
		return "Module:" + v.Name()
	default:
		return "unknown"
	}
}
