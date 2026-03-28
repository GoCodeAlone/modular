package httpclient

import (
	"fmt"
	"net/http"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Event Observation BDD Test Steps

func (ctx *HTTPClientBDDTestContext) iHaveAnHTTPClientWithEventObservationEnabled() error {
	ctx.resetContext()

	logger := &bddTestLogger{}

	// Create httpclient configuration for testing
	ctx.clientConfig = &Config{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		RequestTimeout:      30 * time.Second,
		TLSTimeout:          10 * time.Second,
		DisableCompression:  false,
		DisableKeepAlives:   false,
	}

	// Create provider with the httpclient config
	clientConfigProvider := modular.NewStdConfigProvider(ctx.clientConfig)

	// Create app with empty main config - USE OBSERVABLE for events
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)

	// Create and register httpclient module
	ctx.module = NewHTTPClientModule().(*HTTPClientModule)

	// Create test event observer
	ctx.eventObserver = newTestEventObserver()

	// Register our test observer BEFORE registering module to capture all events
	if err := ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver); err != nil {
		return fmt.Errorf("failed to register test observer: %w", err)
	}

	// Register module
	ctx.app.RegisterModule(ctx.module)

	// Now override the config section with our direct configuration
	ctx.app.RegisterConfigSection("httpclient", clientConfigProvider)

	// Initialize the application (this triggers automatic RegisterObservers)
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize app: %v", err)
	}

	if err := ctx.app.Start(); err != nil {
		return fmt.Errorf("failed to start app: %v", err)
	}

	// Get the httpclient service
	var service interface{}
	if err := ctx.app.GetService("httpclient-service", &service); err != nil {
		return fmt.Errorf("failed to get httpclient service: %w", err)
	}

	// Cast to HTTPClientModule
	if httpClientService, ok := service.(*HTTPClientModule); ok {
		ctx.service = httpClientService
	} else {
		return fmt.Errorf("service is not an HTTPClientModule")
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) aClientStartedEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Allow time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeClientStarted {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeClientStarted, eventTypes)
}

func (ctx *HTTPClientBDDTestContext) aConfigLoadedEventShouldBeEmitted() error {
	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeConfigLoaded {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeConfigLoaded, eventTypes)
}

func (ctx *HTTPClientBDDTestContext) theEventsShouldContainClientConfigurationDetails() error {
	events := ctx.eventObserver.GetEvents()

	// Check config loaded event has configuration details
	for _, event := range events {
		if event.Type() == EventTypeConfigLoaded {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to extract config loaded event data: %v", err)
			}

			// Check for key configuration fields
			if _, exists := data["request_timeout"]; !exists {
				return fmt.Errorf("config loaded event should contain request_timeout field")
			}
			if _, exists := data["max_idle_conns"]; !exists {
				return fmt.Errorf("config loaded event should contain max_idle_conns field")
			}

			return nil
		}
	}

	return fmt.Errorf("config loaded event not found")
}

func (ctx *HTTPClientBDDTestContext) iAddARequestModifier() error {
	if ctx.service == nil {
		return fmt.Errorf("httpclient service not available")
	}

	// Add a simple request modifier
	ctx.service.AddRequestModifier("test-modifier", func(req *http.Request) error {
		req.Header.Set("X-Test-Modifier", "added")
		return nil
	})

	return nil
}

func (ctx *HTTPClientBDDTestContext) aModifierAddedEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Allow time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeModifierAdded {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeModifierAdded, eventTypes)
}

func (ctx *HTTPClientBDDTestContext) iRemoveARequestModifier() error {
	if ctx.service == nil {
		return fmt.Errorf("httpclient service not available")
	}

	// Remove the modifier we added
	ctx.service.RemoveRequestModifier("test-modifier")

	return nil
}

func (ctx *HTTPClientBDDTestContext) aModifierRemovedEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Allow time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeModifierRemoved {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeModifierRemoved, eventTypes)
}

func (ctx *HTTPClientBDDTestContext) iChangeTheClientTimeout() error {
	if ctx.service == nil {
		return fmt.Errorf("httpclient service not available")
	}

	// Change the timeout to trigger an event
	ctx.service.WithTimeout(15) // 15 seconds
	ctx.customTimeout = 15 * time.Second

	return nil
}

func (ctx *HTTPClientBDDTestContext) aTimeoutChangedEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Allow time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeTimeoutChanged {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeTimeoutChanged, eventTypes)
}

func (ctx *HTTPClientBDDTestContext) theEventShouldContainTheNewTimeoutValue() error {
	events := ctx.eventObserver.GetEvents()

	// Check timeout changed event has the new timeout value
	for _, event := range events {
		if event.Type() == EventTypeTimeoutChanged {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to extract timeout changed event data: %v", err)
			}

			// Check for timeout value
			if timeoutValue, exists := data["new_timeout"]; exists {
				expectedTimeout := int(ctx.customTimeout.Seconds())

				// Handle type conversion - CloudEvents may convert integers to float64
				var actualTimeout int
				switch v := timeoutValue.(type) {
				case int:
					actualTimeout = v
				case float64:
					actualTimeout = int(v)
				default:
					return fmt.Errorf("timeout changed event new_timeout has unexpected type: %T", timeoutValue)
				}

				if actualTimeout == expectedTimeout {
					return nil
				}
				return fmt.Errorf("timeout changed event new_timeout mismatch: expected %d, got %d", expectedTimeout, actualTimeout)
			}

			return fmt.Errorf("timeout changed event should contain correct new_timeout value")
		}
	}

	return fmt.Errorf("timeout changed event not found")
}

// Event validation step - ensures all registered events are emitted during testing
func (ctx *HTTPClientBDDTestContext) allRegisteredEventsShouldBeEmittedDuringTesting() error {
	// Get all registered event types from the module
	registeredEvents := ctx.module.GetRegisteredEventTypes()

	// Create event validation observer
	validator := modular.NewEventValidationObserver("event-validator", registeredEvents)
	_ = validator // Use validator to avoid unused variable error

	// Check which events were emitted during testing
	emittedEvents := make(map[string]bool)
	for _, event := range ctx.eventObserver.GetEvents() {
		emittedEvents[event.Type()] = true
	}

	// Check for missing events
	var missingEvents []string
	for _, eventType := range registeredEvents {
		if !emittedEvents[eventType] {
			missingEvents = append(missingEvents, eventType)
		}
	}

	if len(missingEvents) > 0 {
		return fmt.Errorf("the following registered events were not emitted during testing: %v", missingEvents)
	}

	return nil
}
