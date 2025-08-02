package modular

import (
	"context"
	"sync"
	"time"
)

// ObservableDecorator wraps an application to add observer pattern capabilities.
// It emits CloudEvents for application lifecycle events and manages observers.
type ObservableDecorator struct {
	*BaseApplicationDecorator
	observers     []ObserverFunc
	observerMutex sync.RWMutex
}

// NewObservableDecorator creates a new observable decorator with the provided observers
func NewObservableDecorator(inner Application, observers ...ObserverFunc) *ObservableDecorator {
	return &ObservableDecorator{
		BaseApplicationDecorator: NewBaseApplicationDecorator(inner),
		observers:                observers,
	}
}

// AddObserver adds a new observer function
func (d *ObservableDecorator) AddObserver(observer ObserverFunc) {
	d.observerMutex.Lock()
	defer d.observerMutex.Unlock()
	d.observers = append(d.observers, observer)
}

// RemoveObserver removes an observer function (not commonly used with functional observers)
func (d *ObservableDecorator) RemoveObserver(observer ObserverFunc) {
	d.observerMutex.Lock()
	defer d.observerMutex.Unlock()
	// Note: Function comparison is limited in Go, this is best effort
	for i, obs := range d.observers {
		// This comparison may not work as expected due to Go function comparison limitations
		// In practice, you'd typically not remove functional observers
		if &obs == &observer {
			d.observers = append(d.observers[:i], d.observers[i+1:]...)
			break
		}
	}
}

// emitEvent emits a CloudEvent to all registered observers
func (d *ObservableDecorator) emitEvent(ctx context.Context, eventType string, data interface{}, metadata map[string]interface{}) {
	event := NewCloudEvent(eventType, "application", data, metadata)

	d.observerMutex.RLock()
	observers := make([]ObserverFunc, len(d.observers))
	copy(observers, d.observers)
	d.observerMutex.RUnlock()

	// Notify observers in goroutines to avoid blocking
	for _, observer := range observers {
		observer := observer // capture for goroutine
		go func() {
			defer func() {
				if r := recover(); r != nil {
					d.Logger().Error("Observer panicked", "event", eventType, "panic", r)
				}
			}()

			if err := observer(ctx, event); err != nil {
				d.Logger().Error("Observer error", "event", eventType, "error", err)
			}
		}()
	}
}

// Override key lifecycle methods to emit events

// Init overrides the base Init method to emit lifecycle events
func (d *ObservableDecorator) Init() error {
	ctx := context.Background()

	// Emit before init event
	d.emitEvent(ctx, "com.modular.application.before.init", nil, map[string]interface{}{
		"phase":     "before_init",
		"timestamp": time.Now().Format(time.RFC3339),
	})

	err := d.BaseApplicationDecorator.Init()

	if err != nil {
		// Emit init failed event
		d.emitEvent(ctx, "com.modular.application.init.failed", map[string]interface{}{
			"error": err.Error(),
		}, map[string]interface{}{
			"phase":     "init_failed",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return err
	}

	// Emit after init event
	d.emitEvent(ctx, "com.modular.application.after.init", nil, map[string]interface{}{
		"phase":     "after_init",
		"timestamp": time.Now().Format(time.RFC3339),
	})

	return nil
}

// Start overrides the base Start method to emit lifecycle events
func (d *ObservableDecorator) Start() error {
	ctx := context.Background()

	// Emit before start event
	d.emitEvent(ctx, "com.modular.application.before.start", nil, map[string]interface{}{
		"phase":     "before_start",
		"timestamp": time.Now().Format(time.RFC3339),
	})

	err := d.BaseApplicationDecorator.Start()

	if err != nil {
		// Emit start failed event
		d.emitEvent(ctx, "com.modular.application.start.failed", map[string]interface{}{
			"error": err.Error(),
		}, map[string]interface{}{
			"phase":     "start_failed",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return err
	}

	// Emit after start event
	d.emitEvent(ctx, "com.modular.application.after.start", nil, map[string]interface{}{
		"phase":     "after_start",
		"timestamp": time.Now().Format(time.RFC3339),
	})

	return nil
}

// Stop overrides the base Stop method to emit lifecycle events
func (d *ObservableDecorator) Stop() error {
	ctx := context.Background()

	// Emit before stop event
	d.emitEvent(ctx, "com.modular.application.before.stop", nil, map[string]interface{}{
		"phase":     "before_stop",
		"timestamp": time.Now().Format(time.RFC3339),
	})

	err := d.BaseApplicationDecorator.Stop()

	if err != nil {
		// Emit stop failed event
		d.emitEvent(ctx, "com.modular.application.stop.failed", map[string]interface{}{
			"error": err.Error(),
		}, map[string]interface{}{
			"phase":     "stop_failed",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return err
	}

	// Emit after stop event
	d.emitEvent(ctx, "com.modular.application.after.stop", nil, map[string]interface{}{
		"phase":     "after_stop",
		"timestamp": time.Now().Format(time.RFC3339),
	})

	return nil
}
