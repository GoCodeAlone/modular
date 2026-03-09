package httpserver

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/cucumber/godog"
)

// Event observation step implementations
func (ctx *HTTPServerBDDTestContext) iHaveAnHTTPServerWithEventObservationEnabled() error {
	ctx.resetContext()

	logger := &testLogger{}

	// Apply per-app empty feeders instead of mutating global modular.ConfigFeeders

	// Create httpserver configuration for testing - pick a unique free port to avoid conflicts across scenarios
	freePort, err := findFreePort()
	if err != nil {
		return fmt.Errorf("failed to acquire free port: %v", err)
	}
	ctx.serverConfig = &HTTPServerConfig{
		Host:            "127.0.0.1",
		Port:            freePort,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}

	// Create provider with the httpserver config
	serverConfigProvider := modular.NewStdConfigProvider(ctx.serverConfig)

	// Create app with empty main config - USE OBSERVABLE for events
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)
	if cfSetter, ok := ctx.app.(interface{ SetConfigFeeders([]modular.Feeder) }); ok {
		cfSetter.SetConfigFeeders([]modular.Feeder{})
	}

	// Create test event observer
	ctx.eventObserver = newTestEventObserver()

	// Register our test observer BEFORE registering module to capture all events
	if err := ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver); err != nil {
		return fmt.Errorf("failed to register test observer: %w", err)
	}

	// Create a proper router service like the working tests
	router := http.NewServeMux()
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			fmt.Printf("Warning: Failed to write health response: %v\n", err)
		}
	})
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("test response")); err != nil {
			fmt.Printf("Warning: Failed to write test response: %v\n", err)
		}
	})

	// Register the router service
	if err := ctx.app.RegisterService("router", router); err != nil {
		return fmt.Errorf("failed to register router service: %w", err)
	}

	// Create and register httpserver module
	module, ok := NewHTTPServerModule().(*HTTPServerModule)
	if !ok {
		return fmt.Errorf("failed to cast module to HTTPServerModule")
	}
	ctx.module = module

	// Register the HTTP server config section first
	ctx.app.RegisterConfigSection("httpserver", serverConfigProvider)

	// Register module
	ctx.app.RegisterModule(ctx.module)

	// Initialize the application (this triggers automatic RegisterObservers)
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize app: %v", err)
	}

	if err := ctx.app.Start(); err != nil {
		return fmt.Errorf("failed to start app: %v", err)
	}

	// Get the httpserver service
	var service interface{}
	if err := ctx.app.GetService("httpserver", &service); err != nil {
		return fmt.Errorf("failed to get httpserver service: %w", err)
	}

	// Cast to HTTPServerModule
	if httpServerService, ok := service.(*HTTPServerModule); ok {
		ctx.service = httpServerService
		// Explicitly (re)bind observers to this app to avoid any stale subject from previous scenarios
		if subj, ok := ctx.app.(modular.Subject); ok {
			_ = ctx.service.RegisterObservers(subj)
		}
	} else {
		return fmt.Errorf("service is not an HTTPServerModule")
	}

	return nil
}

func (ctx *HTTPServerBDDTestContext) iHaveAnHTTPServerWithTLSAndEventObservationEnabled() error {
	ctx.resetContext()

	logger := &testLogger{}

	// Apply per-app empty feeders instead of mutating global modular.ConfigFeeders

	// Create httpserver configuration with TLS for testing - use a unique free port
	freePort, err := findFreePort()
	if err != nil {
		return fmt.Errorf("failed to acquire free port: %v", err)
	}
	ctx.serverConfig = &HTTPServerConfig{
		Host:            "127.0.0.1",
		Port:            freePort,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		TLS: &TLSConfig{
			Enabled:      true,
			CertFile:     "",
			KeyFile:      "",
			AutoGenerate: true,
			Domains:      []string{"localhost"},
		},
	}

	// Create provider with the httpserver config
	serverConfigProvider := modular.NewStdConfigProvider(ctx.serverConfig)

	// Create app with empty main config - USE OBSERVABLE for events
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)
	if cfSetter, ok := ctx.app.(interface{ SetConfigFeeders([]modular.Feeder) }); ok {
		cfSetter.SetConfigFeeders([]modular.Feeder{})
	}

	// Create test event observer
	ctx.eventObserver = newTestEventObserver()

	// Register our test observer BEFORE registering module to capture all events
	if err := ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver); err != nil {
		return fmt.Errorf("failed to register test observer: %w", err)
	}

	// Create a proper router service like the working tests
	router := http.NewServeMux()
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			fmt.Printf("Warning: Failed to write health response: %v\n", err)
		}
	})
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("test response")); err != nil {
			fmt.Printf("Warning: Failed to write test response: %v\n", err)
		}
	})

	// Register the router service
	if err := ctx.app.RegisterService("router", router); err != nil {
		return fmt.Errorf("failed to register router service: %w", err)
	}

	// Create and register httpserver module
	module, ok := NewHTTPServerModule().(*HTTPServerModule)
	if !ok {
		return fmt.Errorf("failed to cast module to HTTPServerModule")
	}
	ctx.module = module

	// Register the HTTP server config section first
	ctx.app.RegisterConfigSection("httpserver", serverConfigProvider)

	// Register module
	ctx.app.RegisterModule(ctx.module)

	// Initialize the application (this triggers automatic RegisterObservers)
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize app: %v", err)
	}

	if err := ctx.app.Start(); err != nil {
		return fmt.Errorf("failed to start app: %v", err)
	}

	// Get the httpserver service
	var service interface{}
	if err := ctx.app.GetService("httpserver", &service); err != nil {
		return fmt.Errorf("failed to get httpserver service: %w", err)
	}

	// Cast to HTTPServerModule
	if httpServerService, ok := service.(*HTTPServerModule); ok {
		ctx.service = httpServerService
		// Explicitly (re)bind observers to this app to avoid any stale subject from previous scenarios
		if subj, ok := ctx.app.(modular.Subject); ok {
			_ = ctx.service.RegisterObservers(subj)
		}
	} else {
		return fmt.Errorf("service is not an HTTPServerModule")
	}

	return nil
}

func (ctx *HTTPServerBDDTestContext) aServerStartedEventShouldBeEmitted() error {
	time.Sleep(500 * time.Millisecond) // Allow time for server startup and event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeServerStarted {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeServerStarted, eventTypes)
}

func (ctx *HTTPServerBDDTestContext) aConfigLoadedEventShouldBeEmitted() error {
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

func (ctx *HTTPServerBDDTestContext) theEventsShouldContainServerConfigurationDetails() error {
	events := ctx.eventObserver.GetEvents()

	// Check config loaded event has configuration details
	for _, event := range events {
		if event.Type() == EventTypeConfigLoaded {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to extract config loaded event data: %v", err)
			}

			// Check for key configuration fields
			if _, exists := data["http_address"]; !exists {
				return fmt.Errorf("config loaded event should contain http_address field")
			}
			if _, exists := data["read_timeout"]; !exists {
				return fmt.Errorf("config loaded event should contain read_timeout field")
			}

			return nil
		}
	}

	return fmt.Errorf("config loaded event not found")
}

func (ctx *HTTPServerBDDTestContext) aTLSEnabledEventShouldBeEmitted() error {
	time.Sleep(500 * time.Millisecond) // Allow time for server startup and event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeTLSEnabled {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeTLSEnabled, eventTypes)
}

func (ctx *HTTPServerBDDTestContext) aTLSConfiguredEventShouldBeEmitted() error {
	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeTLSConfigured {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeTLSConfigured, eventTypes)
}

func (ctx *HTTPServerBDDTestContext) theEventsShouldContainTLSConfigurationDetails() error {
	events := ctx.eventObserver.GetEvents()

	// Check TLS configured event has configuration details
	for _, event := range events {
		if event.Type() == EventTypeTLSConfigured {
			var data map[string]interface{}
			if err := event.DataAs(&data); err != nil {
				return fmt.Errorf("failed to extract TLS configured event data: %v", err)
			}

			// Check for key TLS configuration fields
			if _, exists := data["https_port"]; !exists {
				return fmt.Errorf("TLS configured event should contain https_port field")
			}
			if _, exists := data["cert_method"]; !exists {
				return fmt.Errorf("TLS configured event should contain cert_method field")
			}

			return nil
		}
	}

	return fmt.Errorf("TLS configured event not found")
}

// Request event step implementations
func (ctx *HTTPServerBDDTestContext) theHTTPServerProcessesARequest() error {
	// Make a test HTTP request to the server to trigger request events
	if ctx.service == nil {
		return fmt.Errorf("server not available")
	}

	// Give the server a moment to fully start
	time.Sleep(200 * time.Millisecond)

	// Re-register the test observer to guarantee we're observing with the exact instance
	// used in assertions. If any other observer with the same ID was registered earlier,
	// this will replace it with our instance.
	if subj, ok := ctx.app.(modular.Subject); ok && ctx.eventObserver != nil {
		_ = subj.RegisterObserver(ctx.eventObserver)
	}

	// Note: Do not clear previously captured events here. Earlier setup or environment
	// interactions may legitimately emit request events (e.g., readiness checks). Clearing
	// could hide these or introduce timing flakiness. The subsequent assertions will
	// scan the buffer for the expected request events regardless of prior emissions.

	// Determine scheme based on TLS configuration
	scheme := "http"
	client := &http.Client{Timeout: 5 * time.Second}
	if ctx.serverConfig != nil && ctx.serverConfig.TLS != nil && ctx.serverConfig.TLS.Enabled {
		// Use HTTPS with insecure skip verify since we're using auto-generated/self-signed certs in tests
		scheme = "https"
		client = &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	}

	// Build URL using actual bound address if available
	url := ""
	if ctx.service != nil && ctx.service.server != nil && ctx.service.server.Addr != "" {
		url = fmt.Sprintf("%s://%s/", scheme, ctx.service.server.Addr)
	} else {
		url = fmt.Sprintf("%s://%s:%d/", scheme, ctx.serverConfig.Host, ctx.serverConfig.Port)
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to make request to %s: %v", url, err)
	}
	defer resp.Body.Close()

	// Read the response to ensure the request completes
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("failed to read response body: %v", readErr)
	}
	_ = body // Read the body but don't log it

	// Since events are now synchronous, they should be emitted immediately
	// But give a small buffer for any remaining async processing
	time.Sleep(100 * time.Millisecond)

	return nil
}

func (ctx *HTTPServerBDDTestContext) aRequestReceivedEventShouldBeEmitted() error {
	// Wait briefly and poll the direct flag set by OnEvent
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		ctx.eventObserver.mu.Lock()
		ok := ctx.eventObserver.sawRequestReceived
		ctx.eventObserver.mu.Unlock()
		if ok {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}

	events := ctx.eventObserver.GetEvents()
	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}
	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeRequestReceived, eventTypes)
}

func (ctx *HTTPServerBDDTestContext) aRequestHandledEventShouldBeEmitted() error {
	// Wait briefly and poll the direct flag set by OnEvent
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		ctx.eventObserver.mu.Lock()
		ok := ctx.eventObserver.sawRequestHandled
		ctx.eventObserver.mu.Unlock()
		if ok {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}

	events := ctx.eventObserver.GetEvents()
	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}
	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", EventTypeRequestHandled, eventTypes)
}

func (ctx *HTTPServerBDDTestContext) theEventsShouldContainRequestDetails() error {
	// Wait briefly to account for async observer delivery and then validate payload
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		events := ctx.eventObserver.GetEvents()
		for _, event := range events {
			if event.Type() == EventTypeRequestReceived {
				var data map[string]interface{}
				if err := event.DataAs(&data); err != nil {
					return fmt.Errorf("failed to extract request received event data: %v", err)
				}

				// Check for key request fields
				if _, exists := data["method"]; !exists {
					return fmt.Errorf("request received event should contain method field")
				}
				if _, exists := data["url"]; !exists {
					return fmt.Errorf("request received event should contain url field")
				}

				return nil
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	return fmt.Errorf("request received event not found")
}

// Event validation step - ensures all registered events are emitted during testing
func (ctx *HTTPServerBDDTestContext) allRegisteredEventsShouldBeEmittedDuringTesting() error {
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

// TestHTTPServerModuleEvents runs the event observation BDD tests for the HTTP server module
func TestHTTPServerModuleEvents(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			testCtx := &HTTPServerBDDTestContext{}

			// Background
			ctx.Given(`^I have a modular application with httpserver module configured$`, testCtx.iHaveAModularApplicationWithHTTPServerModuleConfigured)

			// Basic HTTP server configuration
			ctx.Given(`^I have an HTTP server configuration$`, testCtx.iHaveAnHTTPServerConfiguration)
			ctx.When(`^the httpserver module is initialized$`, testCtx.theHTTPServerModuleIsInitialized)
			ctx.Then(`^the HTTP server service should be available$`, testCtx.theHTTPServerServiceShouldBeAvailable)
			ctx.Then(`^the server should be configured with default settings$`, testCtx.theServerShouldBeConfiguredWithDefaultSettings)

			// Event observation BDD scenarios
			ctx.Given(`^I have an httpserver with event observation enabled$`, testCtx.iHaveAnHTTPServerWithEventObservationEnabled)
			ctx.When(`^the httpserver module starts$`, func() error { return nil }) // Already started in Given step
			ctx.When(`^the HTTP server is started$`, testCtx.theHTTPServerIsStarted)
			ctx.Then(`^a server started event should be emitted$`, testCtx.aServerStartedEventShouldBeEmitted)
			ctx.Then(`^a config loaded event should be emitted$`, testCtx.aConfigLoadedEventShouldBeEmitted)
			ctx.Then(`^the events should contain server configuration details$`, testCtx.theEventsShouldContainServerConfigurationDetails)
			ctx.Then(`^the server should listen on the configured address$`, testCtx.theServerShouldListenOnTheConfiguredAddress)
			ctx.Then(`^the server should accept HTTP requests$`, testCtx.theServerShouldAcceptHTTPRequests)

			// TLS configuration events
			ctx.Given(`^I have an httpserver with TLS and event observation enabled$`, testCtx.iHaveAnHTTPServerWithTLSAndEventObservationEnabled)
			ctx.When(`^the TLS server module starts$`, func() error { return nil }) // Already started in Given step
			ctx.When(`^the HTTPS server is started$`, testCtx.theHTTPSServerIsStarted)
			ctx.Then(`^a TLS enabled event should be emitted$`, testCtx.aTLSEnabledEventShouldBeEmitted)
			ctx.Then(`^a TLS configured event should be emitted$`, testCtx.aTLSConfiguredEventShouldBeEmitted)
			ctx.Then(`^the events should contain TLS configuration details$`, testCtx.theEventsShouldContainTLSConfigurationDetails)
			ctx.Then(`^the server should listen on the configured TLS port$`, testCtx.theServerShouldListenOnTheConfiguredTLSPort)
			ctx.Then(`^the server should accept HTTPS requests$`, testCtx.theServerShouldAcceptHTTPSRequests)

			// Request handling events
			ctx.When(`^the httpserver processes a request$`, testCtx.theHTTPServerProcessesARequest)
			ctx.Then(`^a request received event should be emitted$`, testCtx.aRequestReceivedEventShouldBeEmitted)
			ctx.Then(`^a request handled event should be emitted$`, testCtx.aRequestHandledEventShouldBeEmitted)
			ctx.Then(`^the events should contain request details$`, testCtx.theEventsShouldContainRequestDetails)

			// Event validation (mega-scenario)
			ctx.Then(`^all registered events should be emitted during testing$`, testCtx.allRegisteredEventsShouldBeEmittedDuringTesting)

			// Additional steps needed for full coverage
			ctx.Given(`^I have an HTTPS server configuration with TLS enabled$`, testCtx.iHaveAnHTTPSServerConfigurationWithTLSEnabled)
			ctx.Given(`^I have an HTTP server with custom timeout settings$`, testCtx.iHaveAnHTTPServerWithCustomTimeoutSettings)
			ctx.Given(`^I have a running HTTP server$`, testCtx.iHaveARunningHTTPServer)
			ctx.Given(`^I have an HTTP server running$`, testCtx.iHaveAnHTTPServerRunning)
			ctx.Given(`^I have an HTTP server with health checks enabled$`, testCtx.iHaveAnHTTPServerWithHealthChecksEnabled)
			ctx.Given(`^I have an HTTP server service available$`, testCtx.iHaveAnHTTPServerServiceAvailable)
			ctx.Given(`^I have an HTTP server with middleware configured$`, testCtx.iHaveAnHTTPServerWithMiddlewareConfigured)
			ctx.Given(`^I have a TLS configuration without certificate files$`, testCtx.iHaveATLSConfigurationWithoutCertificateFiles)
			ctx.Given(`^I have an HTTP server with monitoring enabled$`, testCtx.iHaveAnHTTPServerWithMonitoringEnabled)

			// Additional When steps
			ctx.When(`^the server processes requests$`, testCtx.theServerProcessesRequests)
			ctx.When(`^the server shutdown is initiated$`, testCtx.theServerShutdownIsInitiated)
			ctx.When(`^I request the health check endpoint$`, testCtx.iRequestTheHealthCheckEndpoint)
			ctx.When(`^I register custom handlers with the server$`, testCtx.iRegisterCustomHandlersWithTheServer)
			ctx.When(`^requests are processed through the server$`, testCtx.requestsAreProcessedThroughTheServer)
			ctx.When(`^the HTTPS server is started with auto-generation$`, testCtx.theHTTPSServerIsStartedWithAutoGeneration)
			ctx.When(`^an error occurs during request processing$`, testCtx.anErrorOccursDuringRequestProcessing)

			// Additional Then steps
			ctx.Then(`^the read timeout should be respected$`, testCtx.theReadTimeoutShouldBeRespected)
			ctx.Then(`^the write timeout should be respected$`, testCtx.theWriteTimeoutShouldBeRespected)
			ctx.Then(`^the idle timeout should be respected$`, testCtx.theIdleTimeoutShouldBeRespected)
			ctx.Then(`^the server should stop accepting new connections$`, testCtx.theServerShouldStopAcceptingNewConnections)
			ctx.Then(`^existing connections should be allowed to complete$`, testCtx.existingConnectionsShouldBeAllowedToComplete)
			ctx.Then(`^the shutdown should complete within the timeout$`, testCtx.theShutdownShouldCompleteWithinTheTimeout)
			ctx.Then(`^the health check should return server status$`, testCtx.theHealthCheckShouldReturnServerStatus)
			ctx.Then(`^the response should indicate server health$`, testCtx.theResponseShouldIndicateServerHealth)
			ctx.Then(`^the handlers should be available for requests$`, testCtx.theHandlersShouldBeAvailableForRequests)
			ctx.Then(`^the server should route requests to the correct handlers$`, testCtx.theServerShouldRouteRequestsToTheCorrectHandlers)
			ctx.Then(`^the middleware should be applied to requests$`, testCtx.theMiddlewareShouldBeAppliedToRequests)
			ctx.Then(`^the middleware chain should execute in order$`, testCtx.theMiddlewareChainShouldExecuteInOrder)
			ctx.Then(`^the server should generate self-signed certificates$`, testCtx.theServerShouldGenerateSelfSignedCertificates)
			ctx.Then(`^the server should use the generated certificates$`, testCtx.theServerShouldUseTheGeneratedCertificates)
			ctx.Then(`^the server should handle errors gracefully$`, testCtx.theServerShouldHandleErrorsGracefully)
			ctx.Then(`^appropriate error responses should be returned$`, testCtx.appropriateErrorResponsesShouldBeReturned)
			ctx.Then(`^server metrics should be collected$`, testCtx.serverMetricsShouldBeCollected)
			ctx.Then(`^the metrics should include request counts and response times$`, testCtx.theMetricsShouldIncludeRequestCountsAndResponseTimes)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run event observation feature tests")
	}
}
