package modular

import (
	"context"
	"errors"
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

	// If the context requests synchronous delivery, invoke observers directly.
	// Otherwise, notify observers in goroutines to avoid blocking.
	synchronous := IsSynchronousNotification(ctx)
	for _, registration := range app.observers {
		registration := registration // capture for goroutine

		// Check if observer is interested in this event type
		if len(registration.eventTypes) > 0 && !registration.eventTypes[event.Type()] {
			continue // observer not interested in this event type
		}

		notify := func() {
			defer func() {
				if r := recover(); r != nil {
					app.logger.Error("Observer panicked", "observerID", registration.observer.ObserverID(), "event", event.Type(), "panic", r)
				}
			}()

			if err := registration.observer.OnEvent(ctx, event); err != nil {
				app.logger.Error("Observer error", "observerID", registration.observer.ObserverID(), "event", event.Type(), "error", err)
			}
		}

		if synchronous {
			notify()
		} else {
			go notify()
		}
	}

	return nil
}

// emitEvent is a helper method to emit CloudEvents with proper source information
func (app *ObservableApplication) emitEvent(ctx context.Context, event cloudevents.Event) {
	// Use a separate goroutine to avoid blocking application operations
	go func() {
		if err := app.NotifyObservers(ctx, event); err != nil {
			app.logger.Error("Failed to notify observers", "event", event.Type(), "error", err)
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

	// Emit synchronously so tests observing immediate module registration are reliable.
	ctx := WithSynchronousNotification(context.Background())
	evt := NewModuleLifecycleEvent("application", "module", module.Name(), "", "registered", map[string]interface{}{
		"moduleType": getTypeName(module),
	})
	app.emitEvent(ctx, evt)
}

// RegisterService registers a service and emits CloudEvent
func (app *ObservableApplication) RegisterService(name string, service any) error {
	err := app.StdApplication.RegisterService(name, service)
	if err != nil {
		return err
	}

	evt := NewCloudEvent(EventTypeServiceRegistered, "application", map[string]interface{}{
		"serviceName": name,
		"serviceType": getTypeName(service),
	}, nil)
	app.emitEvent(context.Background(), evt)

	return nil
}

// Init initializes the application and emits lifecycle events
func (app *ObservableApplication) Init() error {
	ctx := context.Background()

	app.logger.Debug("ObservableApplication initializing", "modules", len(app.moduleRegistry))

	// Emit application starting initialization
	evtInitStart := NewModuleLifecycleEvent("application", "application", "", "", "init_start", nil)
	app.emitEvent(ctx, evtInitStart)

	// Backward compatibility: emit legacy config.loaded event.
	// Historically the framework emitted config loaded/validated events during initialization.
	// Even though structured lifecycle events now exist, tests (and possibly external observers)
	// still expect these generic configuration events to appear.
	cfgLoaded := NewCloudEvent(EventTypeConfigLoaded, "application", map[string]interface{}{"phase": "init"}, nil)
	app.emitEvent(ctx, cfgLoaded)

	// Register observers for any ObservableModule instances BEFORE calling module Init()
	for _, module := range app.moduleRegistry {
		app.logger.Debug("Checking module for ObservableModule interface", "module", module.Name())
		if observableModule, ok := module.(ObservableModule); ok {
			app.logger.Debug("ObservableApplication registering observers for module", "module", module.Name())
			if err := observableModule.RegisterObservers(app); err != nil {
				app.logger.Error("Failed to register observers for module", "module", module.Name(), "error", err)
			}
		} else {
			app.logger.Debug("Module does not implement ObservableModule", "module", module.Name())
		}
	}
	app.logger.Debug("ObservableApplication finished registering observers")

	app.logger.Debug("ObservableApplication initializing modules with observable application instance")
	err := app.InitWithApp(app)
	if err != nil {
		failureEvt := NewModuleLifecycleEvent("application", "application", "", "", "failed", map[string]interface{}{"phase": "init", "error": err.Error()})
		app.emitEvent(ctx, failureEvt)
		return err
	}

	// Backward compatibility: emit legacy config.validated event after successful initialization.
	cfgValidated := NewCloudEvent(EventTypeConfigValidated, "application", map[string]interface{}{"phase": "init_complete"}, nil)
	app.emitEvent(ctx, cfgValidated)

	// Emit initialization complete
	evtInitComplete := NewModuleLifecycleEvent("application", "application", "", "", "initialized", map[string]interface{}{"phase": "init_complete"})
	app.emitEvent(ctx, evtInitComplete)

	return nil
}

// Start starts the application and emits lifecycle events
func (app *ObservableApplication) Start() error {
	ctx := context.Background()

	err := app.StdApplication.Start()
	if err != nil {
		failureEvt := NewModuleLifecycleEvent("application", "application", "", "", "failed", map[string]interface{}{"phase": "start", "error": err.Error()})
		app.emitEvent(ctx, failureEvt)
		return err
	}

	// Emit application started event
	startedEvt := NewModuleLifecycleEvent("application", "application", "", "", "started", nil)
	app.emitEvent(ctx, startedEvt)

	return nil
}

// Stop stops the application and emits lifecycle events
func (app *ObservableApplication) Stop() error {
	ctx := context.Background()

	err := app.StdApplication.Stop()
	if err != nil {
		failureEvt := NewModuleLifecycleEvent("application", "application", "", "", "failed", map[string]interface{}{"phase": "stop", "error": err.Error()})
		app.emitEvent(ctx, failureEvt)
		return err
	}

	// Emit application stopped event
	stoppedEvt := NewModuleLifecycleEvent("application", "application", "", "", "stopped", nil)
	app.emitEvent(ctx, stoppedEvt)

	return nil
}

// RequestReload triggers a dynamic configuration reload for the specified sections
func (app *ObservableApplication) RequestReload(sections ...string) error {
	return errors.New("dynamic reload not available")
}

// RegisterHealthProvider registers a health check provider for a module
func (app *ObservableApplication) RegisterHealthProvider(moduleName string, provider HealthProvider, optional bool) error {
	return errors.New("health provider registration not available in ObservableApplication")
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
