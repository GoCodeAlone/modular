package reverseproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Event Observation Scenarios

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithEventObservationEnabled() error {
	ctx.resetContext()

	// Create application with reverse proxy config - use ObservableApplication for event support
	logger := &testLogger{}
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)

	// Apply per-app empty feeders to avoid mutating global modular.ConfigFeeders and ensure isolation
	if cfSetter, ok := ctx.app.(interface{ SetConfigFeeders([]modular.Feeder) }); ok {
		cfSetter.SetConfigFeeders([]modular.Feeder{})
	}

	// Register a test router service required by the module
	mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}
	ctx.app.RegisterService("router", mockRouter)

	// Create a test backend server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend response"))
	}))
	ctx.testServers = append(ctx.testServers, testServer)

	// Create reverse proxy configuration
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"test-backend": testServer.URL,
		},
		Routes: map[string]string{
			"/api/test": "test-backend",
		},
		DefaultBackend:       "test-backend",
		CircuitBreakerConfig: CircuitBreakerConfig{Enabled: true, FailureThreshold: 3, OpenTimeout: 500 * time.Millisecond},
	}

	// Create reverse proxy module
	ctx.module = NewModule()
	ctx.service = ctx.module

	// Create test event observer
	ctx.eventObserver = newTestEventObserver()

	// Register our test observer BEFORE registering module to capture all events
	if err := ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver); err != nil {
		return fmt.Errorf("failed to register test observer: %w", err)
	}

	// Register module
	ctx.app.RegisterModule(ctx.module)

	// Register reverse proxy config section
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Initialize the application (this should trigger config loaded events)
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize app: %v", err)
	}

	// Start the application to complete initialization and enable event emission
	if err := ctx.app.Start(); err != nil {
		return fmt.Errorf("failed to start app: %v", err)
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) theReverseProxyModuleStarts() error {
	// Start the application
	if err := ctx.app.Start(); err != nil {
		return fmt.Errorf("failed to start app: %v", err)
	}

	// Give time for all events to be emitted
	time.Sleep(200 * time.Millisecond)
	return nil
}

func (ctx *ReverseProxyBDDTestContext) aProxyCreatedEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Allow time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeProxyCreated {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeProxyCreated, eventTypes)
}

func (ctx *ReverseProxyBDDTestContext) aProxyStartedEventShouldBeEmitted() error {
	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeProxyStarted {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeProxyStarted, eventTypes)
}

func (ctx *ReverseProxyBDDTestContext) aModuleStartedEventShouldBeEmitted() error {
	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeModuleStarted {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeModuleStarted, eventTypes)
}

func (ctx *ReverseProxyBDDTestContext) theEventsShouldContainProxyConfigurationDetails() error {
	events := ctx.eventObserver.GetEvents()

	// Check module started event has configuration details
	for _, event := range events {
		if event.Type() == EventTypeModuleStarted {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to extract module started event data: %v", err)
			}

			// Check for key configuration fields
			if _, exists := data["backend_count"]; !exists {
				return fmt.Errorf("module started event should contain backend_count field")
			}

			return nil
		}
	}

	return fmt.Errorf("module started event not found")
}

func (ctx *ReverseProxyBDDTestContext) theReverseProxyModuleStops() error {
	return ctx.app.Stop()
}

func (ctx *ReverseProxyBDDTestContext) aProxyStoppedEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Allow time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeProxyStopped {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeProxyStopped, eventTypes)
}

func (ctx *ReverseProxyBDDTestContext) aModuleStoppedEventShouldBeEmitted() error {
	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeModuleStopped {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeModuleStopped, eventTypes)
}

// Request routing events

func (ctx *ReverseProxyBDDTestContext) iHaveABackendServiceConfigured() error {
	// This is already done in the setup, just ensure it's ready
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iSendARequestToTheReverseProxy() error {
	// Clear previous events to focus on this request
	ctx.eventObserver.ClearEvents()

	// Send a request through the module to trigger request events
	resp, err := ctx.makeRequestThroughModule("GET", "/api/test", nil)
	if err != nil {
		return err
	}
	if resp != nil {
		resp.Body.Close()
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) aRequestReceivedEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Allow time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeRequestReceived {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeRequestReceived, eventTypes)
}

func (ctx *ReverseProxyBDDTestContext) theEventShouldContainRequestDetails() error {
	events := ctx.eventObserver.GetEvents()

	// Check request received event has request details
	for _, event := range events {
		if event.Type() == EventTypeRequestReceived {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to extract request received event data: %v", err)
			}

			// Check for key request fields
			if _, exists := data["backend"]; !exists {
				return fmt.Errorf("request received event should contain backend field")
			}
			if _, exists := data["method"]; !exists {
				return fmt.Errorf("request received event should contain method field")
			}

			return nil
		}
	}

	return fmt.Errorf("request received event not found")
}

func (ctx *ReverseProxyBDDTestContext) theRequestIsSuccessfullyProxiedToTheBackend() error {
	// Wait for the request to be processed
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (ctx *ReverseProxyBDDTestContext) aRequestProxiedEventShouldBeEmitted() error {
	time.Sleep(200 * time.Millisecond) // Allow time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeRequestProxied {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeRequestProxied, eventTypes)
}

func (ctx *ReverseProxyBDDTestContext) theEventShouldContainBackendAndResponseDetails() error {
	events := ctx.eventObserver.GetEvents()

	// Check request proxied event has backend and response details
	for _, event := range events {
		if event.Type() == EventTypeRequestProxied {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to extract request proxied event data: %v", err)
			}

			// Check for key response fields
			if _, exists := data["backend"]; !exists {
				return fmt.Errorf("request proxied event should contain backend field")
			}

			return nil
		}
	}

	return fmt.Errorf("request proxied event not found")
}

// Request failure events

func (ctx *ReverseProxyBDDTestContext) iHaveAnUnavailableBackendServiceConfigured() error {
	// Create a backend that returns HTTP 500 errors to trigger request.failed events
	// This is more reliable than connection failures in test environments
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Debug: log that this backend was hit
		if ctx.app != nil && ctx.app.Logger() != nil {
			ctx.app.Logger().Info("Failing backend hit", "path", r.URL.Path, "method", r.Method)
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("backend error"))
	}))
	ctx.testServers = append(ctx.testServers, failingServer)

	// Configure with the failing backend URL and ensure routing targets it
	ctx.config.BackendServices = map[string]string{
		"unavailable-backend": failingServer.URL,
	}
	// Route the test path to the unavailable backend and set it as default
	ctx.config.Routes = map[string]string{
		"/api/test": "unavailable-backend",
	}
	ctx.config.DefaultBackend = "unavailable-backend"

	// Ensure the module has a proxy entry for the unavailable backend before Start registers routes
	// This is necessary because proxies are created during Init based on the initial config,
	// and we updated the config after Init in this scenario.
	if ctx.module != nil {
		if err := ctx.module.createBackendProxy("unavailable-backend", failingServer.URL); err != nil {
			return fmt.Errorf("failed to create proxy for unavailable backend: %w", err)
		}

		// Also register the route with the test router
		var router *testRouter
		if err := ctx.app.GetService("router", &router); err == nil && router != nil {
			handler := ctx.module.createBackendProxyHandler("unavailable-backend")
			router.HandleFunc("/api/test", handler)
		}
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theRequestFailsToReachTheBackend() error {
	// Wait for the request to fail
	time.Sleep(300 * time.Millisecond)
	return nil
}

func (ctx *ReverseProxyBDDTestContext) aRequestFailedEventShouldBeEmitted() error {
	time.Sleep(200 * time.Millisecond) // Allow time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeRequestFailed {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeRequestFailed, eventTypes)
}

func (ctx *ReverseProxyBDDTestContext) theEventShouldContainErrorDetails() error {
	events := ctx.eventObserver.GetEvents()

	// Check request failed event has error details
	for _, event := range events {
		if event.Type() == EventTypeRequestFailed {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to extract request failed event data: %v", err)
			}

			// Check for error field
			if _, exists := data["error"]; !exists {
				return fmt.Errorf("request failed event should contain error field")
			}

			return nil
		}
	}

	return fmt.Errorf("request failed event not found")
}

// Circuit Breaker events

func (ctx *ReverseProxyBDDTestContext) iHaveCircuitBreakerEnabledForBackends() error {
	// Update configuration to ensure circuit breakers are enabled
	if ctx.config == nil {
		return fmt.Errorf("configuration not available")
	}

	ctx.config.CircuitBreakerConfig = CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 2,
		OpenTimeout:      100 * time.Millisecond,
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) aCircuitBreakerOpensDueToFailures() error {
	// Create a failing backend to trigger circuit breaker
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("backend failure"))
	}))
	ctx.testServers = append(ctx.testServers, failingServer)

	// Update configuration to use failing backend
	if ctx.config != nil {
		ctx.config.BackendServices["failing-backend"] = failingServer.URL
		ctx.config.Routes["/failing/*"] = "failing-backend"
	}

	// Create the backend proxy in the module so it can be used
	if ctx.module != nil {
		if err := ctx.module.createBackendProxy("failing-backend", failingServer.URL); err != nil {
			return fmt.Errorf("failed to create proxy for failing backend: %w", err)
		}

		// Manually register the new route with the test router since it was added after Start()
		var router *testRouter
		if err := ctx.app.GetService("router", &router); err == nil && router != nil {
			handler := ctx.module.createBackendProxyHandler("failing-backend")
			// Register both the specific path and a wildcard route that the test router can handle
			router.HandleFunc("/failing/test", handler)
			router.HandleFunc("/failing", handler)
		}
	}

	// Clear previous events to focus on circuit breaker events
	if ctx.eventObserver != nil {
		ctx.eventObserver.ClearEvents()
	}

	// Make requests to trigger circuit breaker - use failure threshold + 1
	failureThreshold := 2 // From config in iHaveCircuitBreakerEnabledForBackends
	for i := 0; i < failureThreshold+1; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", "/failing/test", nil)
		if ctx.app != nil && ctx.app.Logger() != nil {
			ctx.app.Logger().Info("Made circuit breaker test request",
				"request_num", i+1,
				"error", err,
				"status_code", func() int {
					if resp != nil {
						return resp.StatusCode
					}
					return 0
				}())
		}
		if err == nil && resp != nil {
			resp.Body.Close()
		}
		// Small delay between requests to allow circuit breaker to process
		time.Sleep(10 * time.Millisecond)
	}

	// Give time for circuit breaker to open and emit events
	time.Sleep(50 * time.Millisecond)

	return nil
}

func (ctx *ReverseProxyBDDTestContext) aCircuitBreakerOpenEventShouldBeEmitted() error {
	events := ctx.eventObserver.GetEvents()

	// Debug: log all events that were captured
	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	if ctx.app != nil && ctx.app.Logger() != nil {
		ctx.app.Logger().Info("Captured events for circuit breaker test",
			"event_count", len(events),
			"event_types", eventTypes)
	}

	for _, event := range events {
		if event.Type() == EventTypeCircuitBreakerOpen {
			return nil
		}
	}

	return fmt.Errorf("no circuit breaker open events found, captured events: %v", eventTypes)
}

func (ctx *ReverseProxyBDDTestContext) theEventShouldContainFailureThresholdDetails() error {
	events := ctx.eventObserver.GetEvents()

	for _, event := range events {
		if event.Type() == EventTypeCircuitBreakerOpen {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to parse circuit breaker open event data: %w", err)
			}

			if _, hasThreshold := data["threshold"]; !hasThreshold {
				return fmt.Errorf("circuit breaker open event missing threshold field")
			}
			return nil
		}
	}

	return fmt.Errorf("no circuit breaker open events found")
}

func (ctx *ReverseProxyBDDTestContext) aCircuitBreakerTransitionsToHalfopen() error {
	// Wait for circuit breaker to transition to half-open
	time.Sleep(150 * time.Millisecond)

	// Send a test request to trigger half-open transition
	resp, err := ctx.makeRequestThroughModule("GET", "/failing/test", nil)
	if err == nil && resp != nil {
		resp.Body.Close()
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) aCircuitBreakerHalfopenEventShouldBeEmitted() error {
	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCircuitBreakerHalfOpen {
			return nil
		}
	}

	return fmt.Errorf("no circuit breaker half-open events found")
}

func (ctx *ReverseProxyBDDTestContext) aCircuitBreakerClosesAfterRecovery() error {
	// Wait for circuit breaker to transition to half-open first
	time.Sleep(150 * time.Millisecond)

	// Instead of trying to change the backend URL, we'll work with the circuit breaker directly
	// by making successful requests that simulate the backend recovering

	// Find the failing backend route that was set up in the circuit breaker open scenario
	var testRoute string = "/failing/test" // This was set up in aCircuitBreakerOpensDueToFailures

	// The trick is to replace the failing server with a healthy one in the same test server slot
	// Find the failing backend in our test servers and replace its handler
	for _, server := range ctx.testServers {
		// Check if this server corresponds to our failing backend
		for backendName, backendURL := range ctx.config.BackendServices {
			if backendURL == server.URL && backendName == "failing-backend" {
				// Replace the server's handler with a healthy one
				server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("backend recovered"))
				})
				break
			}
		}
	}

	// Alternative approach: Create a new healthy backend server and update the proxy configuration
	recoveryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend recovered"))
	}))
	ctx.testServers = append(ctx.testServers, recoveryServer)

	// Update the failing backend to point to the healthy server in the module's internal structures
	if ctx.module != nil {
		// Find the failing backend and update its proxy configuration
		for backendName, _ := range ctx.config.BackendServices {
			if backendName == "failing-backend" {
				// Update the backend URL
				ctx.config.BackendServices[backendName] = recoveryServer.URL
				// Create a new proxy for the recovered backend
				if err := ctx.module.createBackendProxy(backendName, recoveryServer.URL); err != nil {
					return fmt.Errorf("failed to update proxy for recovered backend: %w", err)
				}
				break
			}
		}
	}

	// Now make successful requests to trigger circuit breaker closure
	// The circuit breaker should detect the successful responses and close
	for i := 0; i < 5; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", testRoute, nil)
		if ctx.app != nil && ctx.app.Logger() != nil {
			ctx.app.Logger().Info("Circuit breaker recovery request",
				"request_num", i+1,
				"error", err,
				"status_code", func() int {
					if resp != nil {
						return resp.StatusCode
					}
					return 0
				}())
		}
		if err == nil && resp != nil {
			resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Give additional time for circuit breaker to process the successful responses and close
	time.Sleep(200 * time.Millisecond)
	return nil
}

func (ctx *ReverseProxyBDDTestContext) aCircuitBreakerClosedEventShouldBeEmitted() error {
	events := ctx.eventObserver.GetEvents()

	// Debug: list all captured events
	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	for _, event := range events {
		if event.Type() == EventTypeCircuitBreakerClosed {
			return nil
		}
	}

	return fmt.Errorf("no circuit breaker closed events found. Captured events: %v", eventTypes)
}

// Backend management events

func (ctx *ReverseProxyBDDTestContext) aNewBackendIsAddedToTheConfiguration() error {
	// Create a unique backend name using timestamp to avoid conflicts across scenarios
	backendName := fmt.Sprintf("dynamic-backend-%d", time.Now().UnixNano())
	routePath := fmt.Sprintf("/%s/*", backendName)

	// Add a new backend to the configuration
	newServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("new backend response"))
	}))
	ctx.testServers = append(ctx.testServers, newServer)

	// Dynamically add the backend to the running module to trigger event emission
	// This will update the configuration and emit the event
	if ctx.service != nil {
		err := ctx.service.AddBackend(backendName, newServer.URL)
		if err != nil {
			return fmt.Errorf("failed to add backend dynamically: %w", err)
		}

		// Also add a route for the new backend (optional - for completeness)
		err = ctx.service.AddBackendRoute(backendName, routePath)
		if err != nil {
			// This is non-fatal - route addition might fail if pattern conflicts
			ctx.app.Logger().Warn("Failed to add route for new backend", "backend", backendName, "route", routePath, "error", err.Error())
		}
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) aBackendAddedEventShouldBeEmitted() error {
	// Retry mechanism to handle timing issues - events may be emitted asynchronously
	maxAttempts := 10
	for attempt := 0; attempt < maxAttempts; attempt++ {
		events := ctx.eventObserver.GetEvents()

		for _, event := range events {
			if event.Type() == EventTypeBackendAdded {
				return nil
			}
		}

		// If not found and not last attempt, wait a bit and retry
		if attempt < maxAttempts-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Final check with debug info
	events := ctx.eventObserver.GetEvents()
	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("no backend added events found. Captured events: %v", eventTypes)
}

func (ctx *ReverseProxyBDDTestContext) theEventShouldContainBackendConfiguration() error {
	events := ctx.eventObserver.GetEvents()

	for _, event := range events {
		if event.Type() == EventTypeBackendAdded {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to parse backend added event data: %w", err)
			}

			// Check for the actual field name used in the event (see module.go: "backend")
			if _, hasBackend := data["backend"]; !hasBackend {
				return fmt.Errorf("backend added event missing backend field")
			}
			return nil
		}
	}

	return fmt.Errorf("no backend added events found")
}

func (ctx *ReverseProxyBDDTestContext) aBackendIsRemovedFromTheConfiguration() error {
	// Remove the test-backend from the module to trigger event emission
	if ctx.service != nil {
		err := ctx.service.RemoveBackend("test-backend")
		if err != nil {
			return fmt.Errorf("failed to remove backend: %w", err)
		}
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) aBackendRemovedEventShouldBeEmitted() error {
	// Wait a bit for async event emission to complete
	time.Sleep(100 * time.Millisecond)

	events := ctx.eventObserver.GetEvents()

	// Debug: log all captured events
	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	for _, event := range events {
		if event.Type() == EventTypeBackendRemoved {
			return nil
		}
	}

	return fmt.Errorf("no backend removed events found. Captured events: %v", eventTypes)
}

func (ctx *ReverseProxyBDDTestContext) theEventShouldContainRemovalDetails() error {
	events := ctx.eventObserver.GetEvents()

	for _, event := range events {
		if event.Type() == EventTypeBackendRemoved {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to parse backend removed event data: %w", err)
			}

			if _, hasBackend := data["backend"]; !hasBackend {
				return fmt.Errorf("backend removed event missing backend field")
			}
			return nil
		}
	}

	return fmt.Errorf("no backend removed events found")
}

// Coverage helper steps

func (ctx *ReverseProxyBDDTestContext) iSendAFailingRequestThroughTheProxy() error {
	// Send a request that is likely to fail
	resp, err := ctx.makeRequestThroughModule("GET", "/nonexistent", nil)
	if err == nil && resp != nil {
		resp.Body.Close()
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) allRegisteredEventsShouldBeEmittedDuringTesting() error {
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

	// Verify all registered events were emitted
	missingEvents := []string{}
	for _, eventType := range registeredEvents {
		if !emittedEvents[eventType] {
			missingEvents = append(missingEvents, eventType)
		}
	}

	if len(missingEvents) > 0 {
		return fmt.Errorf("missing events during testing: %v", missingEvents)
	}

	return nil
}

// Missing step implementations for event-related scenarios

func (ctx *ReverseProxyBDDTestContext) aBackendBecomesHealthy() error {
	// This step simulates a backend becoming healthy after being unhealthy
	// The key insight: we need to create a clear unhealthy->healthy transition
	// IMPORTANT: Do NOT reset context here - we need to preserve the observer from previous steps

	// Check if we have the necessary setup from previous steps
	if ctx.app == nil {
		return fmt.Errorf("application not initialized - ensure previous Given steps set up the application")
	}
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not initialized - ensure previous Given steps set up event observation")
	}
	if ctx.module == nil {
		return fmt.Errorf("module not initialized - ensure previous Given steps set up the module")
	}

	// Clear existing events to focus on the health transition we're about to trigger
	if ctx.eventObserver != nil {
		ctx.eventObserver.ClearEvents()
		if ctx.app != nil && ctx.app.Logger() != nil {
			ctx.app.Logger().Info("Cleared events to focus on health transition")
		}
	}

	// Create a new backend that starts unhealthy and becomes healthy
	// Use a shared counter to ensure thread-safe access
	healthToggleCount := 0
	healthToggleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			healthToggleCount++
			if ctx.app != nil && ctx.app.Logger() != nil {
				ctx.app.Logger().Info("Health check request", "count", healthToggleCount, "backend", "health-transition-backend")
			}
			// First health check fails to establish unhealthy state
			if healthToggleCount == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("initial unhealthy"))
				return
			}
			// All subsequent checks succeed to trigger healthy transition
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("now healthy"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("backend response"))
		}
	}))
	ctx.testServers = append(ctx.testServers, healthToggleServer)

	// Add this as a completely new backend to ensure clean state
	newBackendName := fmt.Sprintf("health-transition-backend-%d", time.Now().UnixNano())

	// Add to configuration
	if ctx.config.BackendServices == nil {
		ctx.config.BackendServices = make(map[string]string)
	}
	ctx.config.BackendServices[newBackendName] = healthToggleServer.URL

	if ctx.config.HealthCheck.HealthEndpoints == nil {
		ctx.config.HealthCheck.HealthEndpoints = make(map[string]string)
	}
	ctx.config.HealthCheck.HealthEndpoints[newBackendName] = "/health"

	// Add the backend to the health checker with fresh state
	if ctx.module != nil && ctx.module.healthChecker != nil {
		// CRITICAL: Ensure health checker event emitter is properly connected
		ctx.module.healthChecker.SetEventEmitter(func(eventType string, data map[string]interface{}) {
			// Debug logging
			if ctx.app != nil && ctx.app.Logger() != nil {
				ctx.app.Logger().Info("Health checker emitting event", "type", eventType, "data", data)
			}
			// Direct emission through module's event system
			if ctx.app != nil && ctx.app.Logger() != nil {
				ctx.app.Logger().Info("About to call module.emitEvent", "type", eventType)
			}
			ctx.module.emitEvent(context.Background(), eventType, data)
			if ctx.app != nil && ctx.app.Logger() != nil {
				ctx.app.Logger().Info("Finished calling module.emitEvent", "type", eventType)
			}
		})

		// Update health checker to monitor the new backend
		ctx.module.healthChecker.UpdateBackends(context.Background(), ctx.config.BackendServices)
		ctx.module.healthChecker.UpdateHealthConfig(context.Background(), &ctx.config.HealthCheck)
	}

	// Also register with the service for completeness
	if ctx.service != nil {
		if err := ctx.service.AddBackend(newBackendName, healthToggleServer.URL); err != nil {
			return fmt.Errorf("failed to add new backend for health test: %w", err)
		}
	}

	// Wait for health checker to:
	// 1. First detect the backend as unhealthy (first 2 checks fail)
	// 2. Then detect it as healthy (subsequent checks succeed)
	// This should trigger the healthy event
	// Health check interval is 500ms, so wait for at least 4 cycles to ensure transition
	time.Sleep(2500 * time.Millisecond) // Allow time for multiple health check cycles (increased)

	// Debug: Check if health checker is actually running and has the backend
	if ctx.module != nil && ctx.module.healthChecker != nil {
		healthStatus := ctx.module.healthChecker.GetHealthStatus()
		if ctx.app != nil && ctx.app.Logger() != nil {
			ctx.app.Logger().Info("Health checker status after waiting", "backend_count", len(healthStatus))
			for backendID, status := range healthStatus {
				ctx.app.Logger().Info("Backend health status", "backend", backendID, "healthy", status.Healthy, "lastCheck", status.LastCheck, "totalChecks", status.TotalChecks)
			}
		}
	}

	// Debug: Check if module has proper subject for event emission
	if ctx.module != nil {
		if ctx.app != nil && ctx.app.Logger() != nil {
			ctx.app.Logger().Info("Module event emission debug", "module_subject_nil", ctx.module.subject == nil, "app_subject_available", ctx.app != nil)
			if obsApp, ok := ctx.app.(modular.Subject); ok {
				ctx.app.Logger().Info("Application is observable", "subject_type", fmt.Sprintf("%T", obsApp))
			} else {
				ctx.app.Logger().Info("Application is NOT observable")
			}
		}
	}

	// Debug: Check what events we captured
	if ctx.eventObserver != nil {
		events := ctx.eventObserver.GetEvents()
		if ctx.app != nil && ctx.app.Logger() != nil {
			ctx.app.Logger().Info("Events captured after health transition", "count", len(events))
			for i, event := range events {
				ctx.app.Logger().Info("Health transition event", "index", i, "type", event.Type())
			}
		}
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) loadBalancingDecisionsAreMade() error {
	// This step triggers load balancing decisions by making multiple requests
	// to routes configured with comma-separated backend groups (round-robin).
	// IMPORTANT: Must configure routes before module initialization for load balancing to work!

	// Clear existing context and start fresh like the working test
	var existingObserver *testEventObserver
	if ctx.eventObserver != nil {
		existingObserver = ctx.eventObserver
	}
	ctx.resetContext()

	// Create multiple test backend servers
	for i := 0; i < 2; i++ {
		testServer := httptest.NewServer(http.HandlerFunc(func(idx int) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Backend", fmt.Sprintf("lb-backend-%d", idx))
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(fmt.Sprintf("load balance backend %d", idx)))
			}
		}(i)))
		ctx.testServers = append(ctx.testServers, testServer)
	}

	// Build configuration with backend group route BEFORE module initialization
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"lb-backend-1": ctx.testServers[0].URL,
			"lb-backend-2": ctx.testServers[1].URL,
		},
		Routes: map[string]string{
			// Use concrete path with comma-separated backends for load balancing
			"/api/loadbalance": "lb-backend-1,lb-backend-2",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"lb-backend-1": {URL: ctx.testServers[0].URL},
			"lb-backend-2": {URL: ctx.testServers[1].URL},
		},
	}

	// Use observable app so events are captured for load balancing scenarios
	logger := &testLogger{}
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)

	// Register router
	mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}
	ctx.app.RegisterService("router", mockRouter)

	// Register / create observer
	if existingObserver != nil {
		ctx.eventObserver = existingObserver
	} else {
		ctx.eventObserver = newTestEventObserver()
	}
	_ = ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver)

	// Create module & register
	ctx.module = NewModule()
	ctx.service = ctx.module
	ctx.app.RegisterModule(ctx.module)

	// Register config section & init app
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize app: %w", err)
	}

	// Make several requests to the load-balanced route to trigger load balancing decisions
	for i := 0; i < 5; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", "/api/loadbalance", nil)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}
