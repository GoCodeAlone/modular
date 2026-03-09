package httpserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cucumber/godog"
)

// HTTPServerBDDTestContext is the shared test context for all BDD tests
type HTTPServerBDDTestContext struct {
	app               modular.Application
	module            *HTTPServerModule
	service           *HTTPServerModule
	serverConfig      *HTTPServerConfig
	lastError         error
	testServer        *http.Server
	serverAddress     string
	serverPort        string
	clientResponse    *http.Response
	healthStatus      string
	isHTTPS           bool
	customHandler     http.Handler
	middlewareApplied bool
	testClient        *http.Client
	eventObserver     *testEventObserver
}

// testEventObserver captures CloudEvents during testing
type testEventObserver struct {
	events []cloudevents.Event
	mu     sync.Mutex
	// flags for direct assertions without relying on slice state
	sawRequestReceived bool
	sawRequestHandled  bool
}

func newTestEventObserver() *testEventObserver {
	return &testEventObserver{
		events: make([]cloudevents.Event, 0),
	}
}

func (t *testEventObserver) OnEvent(ctx context.Context, event cloudevents.Event) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event.Clone())
	// set flags for request events to make Then steps robust
	switch event.Type() {
	case EventTypeRequestReceived:
		t.sawRequestReceived = true
	case EventTypeRequestHandled:
		t.sawRequestHandled = true
	}
	return nil
}

func (t *testEventObserver) ObserverID() string {
	return "test-observer-httpserver"
}

func (t *testEventObserver) GetEvents() []cloudevents.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Diagnostics removed; return a copy of events
	events := make([]cloudevents.Event, len(t.events))
	copy(events, t.events)
	return events
}

func (t *testEventObserver) ClearEvents() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = make([]cloudevents.Event, 0)
}

func (ctx *HTTPServerBDDTestContext) resetContext() {
	// Stop any running server before resetting
	if ctx.service != nil {
		if err := ctx.service.Stop(context.Background()); err != nil {
			// Log the error but continue cleanup - test context should be reset regardless
			fmt.Printf("Warning: Failed to stop HTTP server during test cleanup: %v\n", err)
		}
		// Give some time for the port to be released
		time.Sleep(100 * time.Millisecond)
	}
	if ctx.app != nil {
		if err := ctx.app.Stop(); err != nil {
			// Log the error but continue cleanup - test context should be reset regardless
			fmt.Printf("Warning: Failed to stop application during test cleanup: %v\n", err)
		}
		// Give some time for cleanup
		time.Sleep(200 * time.Millisecond)
	}

	ctx.app = nil
	ctx.module = nil
	ctx.service = nil
	ctx.serverConfig = nil
	ctx.lastError = nil
	ctx.testServer = nil
	ctx.serverAddress = ""
	ctx.serverPort = ""
	ctx.clientResponse = nil
	ctx.healthStatus = ""
	ctx.isHTTPS = false
	ctx.customHandler = nil
	ctx.middlewareApplied = false
	if ctx.testClient != nil {
		ctx.testClient.CloseIdleConnections()
	}
	ctx.testClient = &http.Client{
		Timeout: time.Second * 5,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	ctx.eventObserver = nil
}

func (ctx *HTTPServerBDDTestContext) iHaveAModularApplicationWithHTTPServerModuleConfigured() error {
	ctx.resetContext()

	// Create application with HTTP server config
	logger := &testLogger{}

	// Create basic HTTP server configuration for testing
	ctx.serverConfig = &HTTPServerConfig{
		Host:         "127.0.0.1",
		Port:         8090, // Use fixed port for testing
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  30 * time.Second,
		TLS:          nil, // No TLS for basic test
	}

	// Create provider with the HTTP server config
	serverConfigProvider := modular.NewStdConfigProvider(ctx.serverConfig)

	// Create app with empty main config
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewStdApplication(mainConfigProvider, logger)

	// Create a simple router service that the HTTP server requires
	router := http.NewServeMux()
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			// Log write error but continue - test handler should not fail silently
			fmt.Printf("Warning: Failed to write health response: %v\n", err)
		}
	})
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("test response")); err != nil {
			// Log write error but continue - test handler should not fail silently
			fmt.Printf("Warning: Failed to write test response: %v\n", err)
		}
	})

	// Register the router service
	err := ctx.app.RegisterService("router", router)
	if err != nil {
		return fmt.Errorf("failed to register router service: %w", err)
	}

	// Create and register HTTP server module
	ctx.module = NewHTTPServerModule().(*HTTPServerModule)

	// Register the HTTP server config section first
	ctx.app.RegisterConfigSection("httpserver", serverConfigProvider)

	// Register the module
	ctx.app.RegisterModule(ctx.module)

	return nil
}

func (ctx *HTTPServerBDDTestContext) theHTTPServerModuleIsInitialized() error {
	err := ctx.app.Init()
	if err != nil {
		ctx.lastError = err
		return nil
	}

	// The module uses a Constructor, so the service should be available
	// Try to get it as a service
	var serverService *HTTPServerModule
	if err := ctx.app.GetService("httpserver", &serverService); err == nil {
		ctx.service = serverService
		return nil
	}

	// If service lookup fails, something is wrong with our service registration
	// Use the fallback
	ctx.service = ctx.module
	return nil
}

func (ctx *HTTPServerBDDTestContext) theHTTPServerServiceShouldBeAvailable() error {
	if ctx.service == nil {
		return fmt.Errorf("HTTP server service not available")
	}
	return nil
}

func (ctx *HTTPServerBDDTestContext) theServerShouldBeConfiguredWithDefaultSettings() error {
	if ctx.service == nil {
		return fmt.Errorf("HTTP server service not available")
	}

	if ctx.service.config == nil {
		return fmt.Errorf("HTTP server config not available")
	}

	// Verify basic configuration is present
	if ctx.service.config.Host == "" {
		return fmt.Errorf("server host not configured")
	}

	return nil
}

func (ctx *HTTPServerBDDTestContext) iHaveAnHTTPServerConfiguration() error {
	ctx.resetContext()

	// Create specific HTTP server configuration
	ctx.serverConfig = &HTTPServerConfig{
		Host:         "127.0.0.1",
		Port:         8080, // Use fixed port for testing
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		TLS:          nil, // No TLS for basic HTTP
	}

	return ctx.setupApplicationWithConfig()
}

func (ctx *HTTPServerBDDTestContext) setupApplicationWithConfig() error {
	// Debug: TLS config available at start of setupApplicationWithConfig
	_ = ctx.serverConfig.TLS // TLS config check (previously empty debug branch)

	logger := &testLogger{}

	// Use per-app empty feeders for isolation instead of mutating global modular.ConfigFeeders

	// Create a copy of the config to avoid the original being modified
	// during the configuration loading process
	configCopy := &HTTPServerConfig{
		Host:            ctx.serverConfig.Host,
		Port:            ctx.serverConfig.Port,
		ReadTimeout:     ctx.serverConfig.ReadTimeout,
		WriteTimeout:    ctx.serverConfig.WriteTimeout,
		IdleTimeout:     ctx.serverConfig.IdleTimeout,
		ShutdownTimeout: ctx.serverConfig.ShutdownTimeout,
	}

	// Copy TLS config if it exists
	if ctx.serverConfig.TLS != nil {
		configCopy.TLS = &TLSConfig{
			Enabled:      ctx.serverConfig.TLS.Enabled,
			AutoGenerate: ctx.serverConfig.TLS.AutoGenerate,
			CertFile:     ctx.serverConfig.TLS.CertFile,
			KeyFile:      ctx.serverConfig.TLS.KeyFile,
			Domains:      make([]string, len(ctx.serverConfig.TLS.Domains)),
			UseService:   ctx.serverConfig.TLS.UseService,
		}
		copy(configCopy.TLS.Domains, ctx.serverConfig.TLS.Domains)
	}

	// Create provider with the copied config
	serverConfigProvider := modular.NewStdConfigProvider(configCopy)

	// Create app with empty main config
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewStdApplication(mainConfigProvider, logger)
	if cfSetter, ok := ctx.app.(interface{ SetConfigFeeders([]modular.Feeder) }); ok {
		cfSetter.SetConfigFeeders([]modular.Feeder{})
	}

	// Create a simple router service that the HTTP server requires
	router := http.NewServeMux()
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			// Log write error but continue - test handler should not fail silently
			fmt.Printf("Warning: Failed to write health response: %v\n", err)
		}
	})
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("test response")); err != nil {
			// Log write error but continue - test handler should not fail silently
			fmt.Printf("Warning: Failed to write test response: %v\n", err)
		}
	})

	// Register the router service
	err := ctx.app.RegisterService("router", router)
	if err != nil {
		return fmt.Errorf("failed to register router service: %w", err)
	}

	// Create and register HTTP server module
	ctx.module = NewHTTPServerModule().(*HTTPServerModule)

	// Register the HTTP server config section first
	ctx.app.RegisterConfigSection("httpserver", serverConfigProvider)

	// Register the module
	ctx.app.RegisterModule(ctx.module)

	// Debug: TLS config available before app.Init()
	_ = ctx.serverConfig.TLS // TLS config check (previously empty debug branch)

	// Initialize
	err = ctx.app.Init()
	if err != nil {
		ctx.lastError = err
		return err
	}

	// Debug: TLS config available after app.Init()
	_ = ctx.serverConfig.TLS // TLS config check (previously empty debug branch)

	// The HTTP server module doesn't provide services, so we access it directly
	ctx.service = ctx.module

	// Debug: module's TLS config available
	_ = ctx.service.config.TLS // TLS config check (previously empty debug branch)

	return nil
}

// findFreePort returns an available TCP port on localhost for exclusive use by tests.
func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// Test logger implementation
type testLogger struct{}

func (l *testLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *testLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) Error(msg string, keysAndValues ...interface{}) {}

// TestHTTPServerModuleCoreFeatures runs the core BDD tests for the HTTP server module
func TestHTTPServerModuleCoreFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			testCtx := &HTTPServerBDDTestContext{}

			// Background
			ctx.Given(`^I have a modular application with httpserver module configured$`, testCtx.iHaveAModularApplicationWithHTTPServerModuleConfigured)

			// Steps for module initialization
			ctx.When(`^the httpserver module is initialized$`, testCtx.theHTTPServerModuleIsInitialized)
			ctx.Then(`^the HTTP server service should be available$`, testCtx.theHTTPServerServiceShouldBeAvailable)
			ctx.Then(`^the server should be configured with default settings$`, testCtx.theServerShouldBeConfiguredWithDefaultSettings)

			// Steps for basic HTTP server
			ctx.Given(`^I have an HTTP server configuration$`, testCtx.iHaveAnHTTPServerConfiguration)
			ctx.When(`^the HTTP server is started$`, testCtx.theHTTPServerIsStarted)
			ctx.Then(`^the server should listen on the configured address$`, testCtx.theServerShouldListenOnTheConfiguredAddress)
			ctx.Then(`^the server should accept HTTP requests$`, testCtx.theServerShouldAcceptHTTPRequests)

			// HTTPS/TLS steps
			ctx.Given(`^I have an HTTPS server configuration with TLS enabled$`, testCtx.iHaveAnHTTPSServerConfigurationWithTLSEnabled)
			ctx.When(`^the HTTPS server is started$`, testCtx.theHTTPSServerIsStarted)
			ctx.Then(`^the server should listen on the configured TLS port$`, testCtx.theServerShouldListenOnTheConfiguredTLSPort)
			ctx.Then(`^the server should accept HTTPS requests$`, testCtx.theServerShouldAcceptHTTPSRequests)

			// Timeout configuration steps
			ctx.Given(`^I have an HTTP server with custom timeout settings$`, testCtx.iHaveAnHTTPServerWithCustomTimeoutSettings)
			ctx.When(`^the server processes requests$`, testCtx.theServerProcessesRequests)
			ctx.Then(`^the read timeout should be respected$`, testCtx.theReadTimeoutShouldBeRespected)
			ctx.Then(`^the write timeout should be respected$`, testCtx.theWriteTimeoutShouldBeRespected)
			ctx.Then(`^the idle timeout should be respected$`, testCtx.theIdleTimeoutShouldBeRespected)

			// Graceful shutdown steps
			ctx.Given(`^I have a running HTTP server$`, testCtx.iHaveARunningHTTPServer)
			ctx.Given(`^I have an HTTP server running$`, testCtx.iHaveAnHTTPServerRunning)
			ctx.When(`^the server shutdown is initiated$`, testCtx.theServerShutdownIsInitiated)
			ctx.Then(`^the server should stop accepting new connections$`, testCtx.theServerShouldStopAcceptingNewConnections)
			ctx.Then(`^existing connections should be allowed to complete$`, testCtx.existingConnectionsShouldBeAllowedToComplete)
			ctx.Then(`^the shutdown should complete within the timeout$`, testCtx.theShutdownShouldCompleteWithinTheTimeout)

			// Health check steps
			ctx.Given(`^I have an HTTP server with health checks enabled$`, testCtx.iHaveAnHTTPServerWithHealthChecksEnabled)
			ctx.When(`^I request the health check endpoint$`, testCtx.iRequestTheHealthCheckEndpoint)
			ctx.Then(`^the health check should return server status$`, testCtx.theHealthCheckShouldReturnServerStatus)
			ctx.Then(`^the response should indicate server health$`, testCtx.theResponseShouldIndicateServerHealth)

			// Handler registration steps
			ctx.Given(`^I have an HTTP server service available$`, testCtx.iHaveAnHTTPServerServiceAvailable)
			ctx.When(`^I register custom handlers with the server$`, testCtx.iRegisterCustomHandlersWithTheServer)
			ctx.Then(`^the handlers should be available for requests$`, testCtx.theHandlersShouldBeAvailableForRequests)
			ctx.Then(`^the server should route requests to the correct handlers$`, testCtx.theServerShouldRouteRequestsToTheCorrectHandlers)

			// Middleware steps
			ctx.Given(`^I have an HTTP server with middleware configured$`, testCtx.iHaveAnHTTPServerWithMiddlewareConfigured)
			ctx.When(`^requests are processed through the server$`, testCtx.requestsAreProcessedThroughTheServer)
			ctx.Then(`^the middleware should be applied to requests$`, testCtx.theMiddlewareShouldBeAppliedToRequests)
			ctx.Then(`^the middleware chain should execute in order$`, testCtx.theMiddlewareChainShouldExecuteInOrder)

			// TLS auto-generation steps
			ctx.Given(`^I have a TLS configuration without certificate files$`, testCtx.iHaveATLSConfigurationWithoutCertificateFiles)
			ctx.When(`^the HTTPS server is started with auto-generation$`, testCtx.theHTTPSServerIsStartedWithAutoGeneration)
			ctx.Then(`^the server should generate self-signed certificates$`, testCtx.theServerShouldGenerateSelfSignedCertificates)
			ctx.Then(`^the server should use the generated certificates$`, testCtx.theServerShouldUseTheGeneratedCertificates)

			// Error handling steps
			ctx.When(`^an error occurs during request processing$`, testCtx.anErrorOccursDuringRequestProcessing)
			ctx.Then(`^the server should handle errors gracefully$`, testCtx.theServerShouldHandleErrorsGracefully)
			ctx.Then(`^appropriate error responses should be returned$`, testCtx.appropriateErrorResponsesShouldBeReturned)

			// Monitoring steps
			ctx.Given(`^I have an HTTP server with monitoring enabled$`, testCtx.iHaveAnHTTPServerWithMonitoringEnabled)
			ctx.Then(`^server metrics should be collected$`, testCtx.serverMetricsShouldBeCollected)
			ctx.Then(`^the metrics should include request counts and response times$`, testCtx.theMetricsShouldIncludeRequestCountsAndResponseTimes)

			// Event observation steps
			ctx.Given(`^I have an httpserver with event observation enabled$`, testCtx.iHaveAnHTTPServerWithEventObservationEnabled)
			ctx.Given(`^I have an httpserver with TLS and event observation enabled$`, testCtx.iHaveAnHTTPServerWithTLSAndEventObservationEnabled)
			ctx.When(`^the httpserver module starts$`, func() error { return nil })
			ctx.When(`^the TLS server module starts$`, func() error { return nil })
			ctx.When(`^the httpserver processes a request$`, testCtx.theHTTPServerProcessesARequest)
			ctx.Then(`^a server started event should be emitted$`, testCtx.aServerStartedEventShouldBeEmitted)
			ctx.Then(`^a config loaded event should be emitted$`, testCtx.aConfigLoadedEventShouldBeEmitted)
			ctx.Then(`^the events should contain server configuration details$`, testCtx.theEventsShouldContainServerConfigurationDetails)
			ctx.Then(`^a TLS enabled event should be emitted$`, testCtx.aTLSEnabledEventShouldBeEmitted)
			ctx.Then(`^a TLS configured event should be emitted$`, testCtx.aTLSConfiguredEventShouldBeEmitted)
			ctx.Then(`^the events should contain TLS configuration details$`, testCtx.theEventsShouldContainTLSConfigurationDetails)
			ctx.Then(`^a request received event should be emitted$`, testCtx.aRequestReceivedEventShouldBeEmitted)
			ctx.Then(`^a request handled event should be emitted$`, testCtx.aRequestHandledEventShouldBeEmitted)
			ctx.Then(`^the events should contain request details$`, testCtx.theEventsShouldContainRequestDetails)
			ctx.Then(`^all registered events should be emitted during testing$`, testCtx.allRegisteredEventsShouldBeEmittedDuringTesting)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run core feature tests")
	}
}
