package reverseproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// ReverseProxy BDD Test Context - shared across all BDD test files
type ReverseProxyBDDTestContext struct {
	app          modular.Application
	module       *ReverseProxyModule
	service      *ReverseProxyModule
	config       *ReverseProxyConfig
	lastError    error
	testServers  []*httptest.Server
	lastResponse *http.Response
	// Cached parsed debug endpoint payloads to allow multiple assertions without re-reading body
	debugBackendsData     map[string]interface{}
	debugFlagsData        map[string]interface{}
	eventObserver         *testEventObserver
	healthCheckServers    []*httptest.Server
	metricsEnabled        bool
	debugEnabled          bool
	featureFlagService    *FileBasedFeatureFlagEvaluator
	dryRunEnabled         bool
	controlledFailureMode *bool // For controlling backend failure in tests
	// Health status control for testing
	healthyBackendHealthy   *bool
	unhealthyBackendHealthy *bool
	// HTTP testing support
	httpRecorder     *httptest.ResponseRecorder
	lastResponseBody []byte
	// Metrics endpoint path used in metrics-related tests
	metricsEndpointPath string
	// Request tracking for tenant isolation validation
	tenantARequests *[]*http.Request
	tenantBRequests *[]*http.Request
	// Mutex for protecting tenant request arrays during concurrent access
	tenantRequestsMu sync.RWMutex
	// Shutdown testing support channels
	ongoingRequestResults      chan requestResult
	ongoingRequestStartSignals chan bool
	shutdownStarted            chan bool
	// Authentication testing support for debug endpoints
	unauthenticatedResponses    []*http.Response
	authenticatedResponses      []*http.Response
	authenticatedResponseBodies [][]byte
}

// requestResult tracks the outcome of requests during testing scenarios
type requestResult struct {
	path     string
	success  bool
	error    error
	duration time.Duration
}

// testEventObserver captures CloudEvents during testing
type testEventObserver struct {
	mu     sync.RWMutex
	events []cloudevents.Event
}

func newTestEventObserver() *testEventObserver {
	return &testEventObserver{
		events: make([]cloudevents.Event, 0),
	}
}

func (t *testEventObserver) OnEvent(ctx context.Context, event cloudevents.Event) error {
	// Debug logging for event capture
	fmt.Printf("DEBUG: testEventObserver.OnEvent called with event type: %s\n", event.Type())
	t.mu.Lock()
	t.events = append(t.events, event.Clone())
	eventCount := len(t.events)
	t.mu.Unlock()
	fmt.Printf("DEBUG: Event stored, total events now: %d\n", eventCount)
	return nil
}

func (t *testEventObserver) ObserverID() string {
	return "test-observer-reverseproxy"
}

func (t *testEventObserver) GetEvents() []cloudevents.Event {
	t.mu.RLock()
	events := make([]cloudevents.Event, len(t.events))
	copy(events, t.events)
	t.mu.RUnlock()
	return events
}

func (t *testEventObserver) ClearEvents() {
	t.mu.Lock()
	t.events = make([]cloudevents.Event, 0)
	t.mu.Unlock()
}

// Safely append to tenant A requests
func (ctx *ReverseProxyBDDTestContext) appendTenantARequest(req *http.Request) {
	ctx.tenantRequestsMu.Lock()
	defer ctx.tenantRequestsMu.Unlock()
	if ctx.tenantARequests != nil {
		*ctx.tenantARequests = append(*ctx.tenantARequests, req)
	}
}

// Safely append to tenant B requests
func (ctx *ReverseProxyBDDTestContext) appendTenantBRequest(req *http.Request) {
	ctx.tenantRequestsMu.Lock()
	defer ctx.tenantRequestsMu.Unlock()
	if ctx.tenantBRequests != nil {
		*ctx.tenantBRequests = append(*ctx.tenantBRequests, req)
	}
}

// Safely read tenant A requests length
func (ctx *ReverseProxyBDDTestContext) getTenantARequestsLen() int {
	ctx.tenantRequestsMu.RLock()
	defer ctx.tenantRequestsMu.RUnlock()
	if ctx.tenantARequests != nil {
		return len(*ctx.tenantARequests)
	}
	return 0
}

// Safely read tenant B requests length
func (ctx *ReverseProxyBDDTestContext) getTenantBRequestsLen() int {
	ctx.tenantRequestsMu.RLock()
	defer ctx.tenantRequestsMu.RUnlock()
	if ctx.tenantBRequests != nil {
		return len(*ctx.tenantBRequests)
	}
	return 0
}

// Safely get a copy of tenant A requests
func (ctx *ReverseProxyBDDTestContext) getTenantARequestsCopy() []*http.Request {
	ctx.tenantRequestsMu.RLock()
	defer ctx.tenantRequestsMu.RUnlock()
	if ctx.tenantARequests != nil {
		requests := make([]*http.Request, len(*ctx.tenantARequests))
		copy(requests, *ctx.tenantARequests)
		return requests
	}
	return nil
}

// Safely get a copy of tenant B requests
func (ctx *ReverseProxyBDDTestContext) getTenantBRequestsCopy() []*http.Request {
	ctx.tenantRequestsMu.RLock()
	defer ctx.tenantRequestsMu.RUnlock()
	if ctx.tenantBRequests != nil {
		requests := make([]*http.Request, len(*ctx.tenantBRequests))
		copy(requests, *ctx.tenantBRequests)
		return requests
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) resetContext() {
	// Stop the module explicitly before closing servers
	if ctx.module != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ctx.module.Stop(stopCtx)

		// Wait for health checker to fully stop with proper verification
		if ctx.module.healthChecker != nil {
			// Wait up to 2 seconds for health checker to stop
			maxWait := 200 // 200 * 10ms = 2 seconds
			for i := 0; i < maxWait && ctx.module.healthChecker.IsRunning(); i++ {
				time.Sleep(10 * time.Millisecond)
			}

			// Log warning if still running
			if ctx.module.healthChecker.IsRunning() {
				fmt.Printf("WARNING: Health checker still running after %d ms\n", maxWait*10)
			}
		}
	}

	// Clear cache if service has one to ensure no state leakage between scenarios
	if ctx.service != nil && ctx.service.responseCache != nil {
		ctx.service.responseCache.Clear()
	}

	// Close test servers
	for _, server := range ctx.testServers {
		if server != nil {
			server.Close()
		}
	}

	// Close health check servers
	for _, server := range ctx.healthCheckServers {
		if server != nil {
			server.Close()
		}
	}

	// Properly shutdown the application if it exists
	if ctx.app != nil {
		// Call Shutdown if the app implements Stoppable interface
		if stoppable, ok := ctx.app.(interface{ Shutdown() error }); ok {
			stoppable.Shutdown()
		}
	}

	// Increased delay to allow all background goroutines to fully shut down
	// This is critical for preventing goroutine leaks between test scenarios
	time.Sleep(500 * time.Millisecond)

	ctx.app = nil
	ctx.module = nil
	ctx.service = nil

	// Create a new config instance with initialized maps to avoid sharing state between scenarios
	ctx.config = &ReverseProxyConfig{
		BackendServices: make(map[string]string),
		Routes:          make(map[string]string),
		BackendConfigs:  make(map[string]BackendServiceConfig),
		HealthCheck: HealthCheckConfig{
			HealthEndpoints: make(map[string]string),
		},
	}

	ctx.lastError = nil
	ctx.testServers = nil
	ctx.lastResponse = nil
	ctx.healthCheckServers = nil
	ctx.metricsEnabled = false
	ctx.debugEnabled = false
	ctx.featureFlagService = nil
	ctx.dryRunEnabled = false
	ctx.controlledFailureMode = nil
	ctx.metricsEndpointPath = ""
	ctx.tenantRequestsMu.Lock()
	ctx.httpRecorder = nil
	ctx.lastResponseBody = nil
	ctx.tenantARequests = nil
	ctx.tenantBRequests = nil
	ctx.tenantRequestsMu.Unlock()
	ctx.unauthenticatedResponses = nil
	ctx.authenticatedResponses = nil
	ctx.authenticatedResponseBodies = nil
}

// ensureServiceInitialized guarantees the reverseproxy service is initialized and started.
func (ctx *ReverseProxyBDDTestContext) ensureServiceInitialized() error {
	if ctx.app == nil {
		return fmt.Errorf("application not initialized")
	}

	// If service already appears available, still ensure the app is started and routes are registered
	if ctx.service != nil {
		// Verify router has routes; if not, ensure Start is called
		var router *testRouter
		if err := ctx.app.GetService("router", &router); err == nil && router != nil {
			if len(router.routes) == 0 {
				if err := ctx.app.Start(); err != nil {
					ctx.lastError = err
					return fmt.Errorf("failed to start application: %w", err)
				}
			}
		}
		return nil
	}

	// Initialize and start the app if needed
	if err := ctx.app.Init(); err != nil {
		ctx.lastError = err
		return fmt.Errorf("failed to initialize application: %w", err)
	}
	if err := ctx.app.Start(); err != nil {
		ctx.lastError = err
		return fmt.Errorf("failed to start application: %w", err)
	}

	// Retrieve the reverseproxy service
	if err := ctx.app.GetService("reverseproxy.provider", &ctx.service); err != nil {
		ctx.lastError = err
		return fmt.Errorf("failed to get reverseproxy service: %w", err)
	}
	if ctx.service == nil {
		return fmt.Errorf("reverseproxy service is nil after startup")
	}
	return nil
}

// preserveApplicationWithFreshConfiguration resets the application's services and modules
// while keeping the same logger (e.g., MockLogger)
func (ctx *ReverseProxyBDDTestContext) preserveApplicationWithFreshConfiguration() error {
	if ctx.app == nil {
		return fmt.Errorf("no application to preserve")
	}

	// Get the current logger to preserve it
	logger := ctx.app.Logger()

	// Check if the current app is observable
	var app modular.Application
	var err error
	if _, isObservable := ctx.app.(modular.Subject); isObservable {
		// Create new observable application to preserve event handling
		mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
		app = modular.NewObservableApplication(mainConfigProvider, logger)

		// Re-register the event observer if it exists
		if ctx.eventObserver != nil {
			if err := app.(modular.Subject).RegisterObserver(ctx.eventObserver); err != nil {
				return fmt.Errorf("failed to re-register event observer: %w", err)
			}
		}
	} else {
		// Create a regular application
		app, err = modular.NewApplication(
			modular.WithLogger(logger),
		)
		if err != nil {
			return fmt.Errorf("failed to create preserved application: %w", err)
		}
	}

	if app == nil {
		return fmt.Errorf("NewApplication returned nil during preservation")
	}

	// Replace the application with the fresh one that has the same logger
	ctx.app = app
	return nil
}

// makeRequestThroughModule issues an HTTP request through the test router wired by the module.
func (ctx *ReverseProxyBDDTestContext) makeRequestThroughModule(method, urlPath string, body io.Reader) (*http.Response, error) {
	return ctx.makeRequestThroughModuleWithHeaders(method, urlPath, body, nil)
}

func (ctx *ReverseProxyBDDTestContext) makeRequestThroughModuleWithHeaders(method, urlPath string, body io.Reader, headers map[string]string) (*http.Response, error) {
	if err := ctx.ensureServiceInitialized(); err != nil {
		return nil, err
	}

	// Get the router registered in the app
	var router *testRouter
	if err := ctx.app.GetService("router", &router); err != nil {
		return nil, fmt.Errorf("failed to get router service: %w", err)
	}
	if router == nil {
		return nil, fmt.Errorf("router service not available")
	}

	req := httptest.NewRequest(method, urlPath, body)

	// Set custom headers if provided
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	ctx.tenantRequestsMu.Lock()
	ctx.httpRecorder = rec
	ctx.tenantRequestsMu.Unlock()
	resp := rec.Result()
	return resp, nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAModularApplicationWithReverseProxyModuleConfigured() error {
	ctx.resetContext()

	// Create a default test backend for basic scenarios
	defaultTestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("default backend response"))
	}))
	ctx.testServers = append(ctx.testServers, defaultTestServer)

	// Set up basic reverse proxy configuration with a default backend
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"default-backend": defaultTestServer.URL,
		},
		Routes: map[string]string{
			"/test": "default-backend",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"default-backend": {
				URL: defaultTestServer.URL,
			},
		},
		RouteConfigs:   make(map[string]RouteConfig),
		DefaultBackend: "default-backend",
		HealthCheck: HealthCheckConfig{
			Enabled:  false,
			Interval: 30 * time.Second,
			Timeout:  5 * time.Second,
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled: false,
		},
		CacheEnabled: false,
	}

	// Create application directly like the working single backend scenario
	logger := &testLogger{}
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)

	// Register router
	mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}
	ctx.app.RegisterService("router", mockRouter)

	// Create observer for consistency with other scenarios
	ctx.eventObserver = newTestEventObserver()
	_ = ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver)

	// Create module & register
	ctx.module = NewModule()
	ctx.service = ctx.module
	ctx.app.RegisterModule(ctx.module)

	// Register config section directly & init app
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize app: %w", err)
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) setupApplicationWithConfig() error {
	// If an application with a specific logger (like MockLogger) is already set,
	// preserve it and its configuration
	if ctx.app != nil {
		// Check if the existing app has a MockLogger - if so, preserve it
		if _, isMockLogger := ctx.app.Logger().(*MockLogger); isMockLogger {
			// Preserve the existing application with MockLogger but reset its services/modules
			if err := ctx.preserveApplicationWithFreshConfiguration(); err != nil {
				return err
			}
		} else {
			// Create a fresh application with default logger
			ctx.app = nil
		}
	}

	// Create a fresh application if we don't have one or it doesn't have MockLogger
	if ctx.app == nil {
		// Check if we need an observable application based on whether we have an event observer
		var app modular.Application
		var err error
		if ctx.eventObserver != nil {
			// Create an observable application and register the event observer
			mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
			app = modular.NewObservableApplication(mainConfigProvider, &testLogger{})
			if err := app.(modular.Subject).RegisterObserver(ctx.eventObserver); err != nil {
				return fmt.Errorf("failed to register event observer in fresh observable app: %w", err)
			}
		} else {
			// Create a regular application
			app, err = modular.NewApplication(
				modular.WithLogger(&testLogger{}),
			)
			if err != nil {
				return fmt.Errorf("failed to create application: %w", err)
			}
		}
		if app == nil {
			return fmt.Errorf("NewApplication returned nil")
		}
		ctx.app = app
	}

	if ctx.config == nil {
		return fmt.Errorf("configuration not set: config is nil")
	}

	// CRITICAL FIX: Deep copy the config to prevent shared state between tests
	// Use the framework's DeepCopyConfig to ensure complete isolation
	configCopy, err := modular.DeepCopyConfig(ctx.config)
	if err != nil {
		return fmt.Errorf("failed to deep copy config: %w", err)
	}

	// Type assert back to ReverseProxyConfig
	reverseProxyConfigCopy, ok := configCopy.(*ReverseProxyConfig)
	if !ok {
		return fmt.Errorf("deep copy returned unexpected type: %T", configCopy)
	}

	// Register config section with the isolated copy
	reverseproxyConfigProvider := modular.NewStdConfigProvider(reverseProxyConfigCopy)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Register services used by reverse proxy
	if err := ctx.app.RegisterService("logger", &testLogger{}); err != nil {
		return fmt.Errorf("failed to register logger: %w", err)
	}
	// Create and initialize test router with proper setup
	testRouterInstance := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}
	if err := ctx.app.RegisterService("router", testRouterInstance); err != nil {
		return fmt.Errorf("failed to register router: %w", err)
	}
	if err := ctx.app.RegisterService("metrics", &testMetrics{}); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	// Register TenantService to support feature flag functionality
	tenantService := &MockTenantService{
		Configs: make(map[modular.TenantID]map[string]modular.ConfigProvider),
	}
	if err := ctx.app.RegisterService("tenantService", tenantService); err != nil {
		return fmt.Errorf("failed to register tenant service: %w", err)
	}

	// Do not register any feature flag evaluator - let the module handle its own setup
	// This should restore the original behavior before the regression

	// Create and register event observer
	ctx.eventObserver = newTestEventObserver()
	if err := ctx.app.RegisterService("event-bus", &testEventBus{observers: []modular.Observer{ctx.eventObserver}}); err != nil {
		return fmt.Errorf("failed to register event bus: %w", err)
	}

	// Always create a fresh module to avoid state pollution between scenarios
	module := NewModule()

	// Get the registered router service for the constructor
	var router *testRouter
	if err := ctx.app.GetService("router", &router); err != nil {
		return fmt.Errorf("failed to get router service: %w", err)
	}
	if router == nil {
		return fmt.Errorf("router service is nil after retrieval")
	}
	// Safely initialize routes map using the router's mutex
	router.mu.Lock()
	if router.routes == nil {
		router.routes = make(map[string]http.HandlerFunc)
	}
	router.mu.Unlock()

	// Use the constructor to properly initialize the module with dependencies
	constructor := module.Constructor()
	services := map[string]any{
		"router": router,
	}

	// Don't inject feature flag evaluator - let the module handle its own setup

	constructedModule, err := constructor(ctx.app, services)
	if err != nil {
		return fmt.Errorf("failed to construct module: %w", err)
	}

	ctx.module = constructedModule.(*ReverseProxyModule)
	ctx.app.RegisterModule(ctx.module)

	// Initialize and start the application
	fmt.Printf("🔍 DEBUG: Before ctx.app.Init(), ctx.config.CacheTTL=%v\n", ctx.config.CacheTTL)
	if err := ctx.app.Init(); err != nil {
		ctx.lastError = err
		return fmt.Errorf("failed to initialize application: %w", err)
	}
	fmt.Printf("🔍 DEBUG: After ctx.app.Init(), ctx.config.CacheTTL=%v\n", ctx.config.CacheTTL)
	if err := ctx.app.Start(); err != nil {
		ctx.lastError = err
		return fmt.Errorf("failed to start application: %w", err)
	}

	// Retrieve the reverseproxy service
	if err := ctx.app.GetService("reverseproxy.provider", &ctx.service); err != nil {
		ctx.lastError = err
		return fmt.Errorf("failed to get reverseproxy service: %w", err)
	}
	if ctx.service == nil {
		return fmt.Errorf("reverseproxy service is nil after startup")
	}

	// NOTE: We intentionally DO NOT update ctx.config to point to module.config
	// because ctx.config should remain the original config we set up for the test.
	// The module has its own copy (from deepCopyConfig) which may have defaults applied.
	// Tests that need to check the module's actual config should use ctx.module.config directly.

	return nil
}

func (ctx *ReverseProxyBDDTestContext) theReverseProxyModuleIsInitialized() error {
	if ctx.app == nil {
		return fmt.Errorf("application not configured")
	}

	// Initialize the application
	if err := ctx.app.Init(); err != nil {
		ctx.lastError = err
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) theProxyServiceShouldBeAvailable() error {
	if err := ctx.ensureServiceInitialized(); err != nil {
		return err
	}

	if ctx.service == nil {
		return fmt.Errorf("reverse proxy service should be available but is nil")
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) theModuleShouldBeReadyToRouteRequests() error {
	if err := ctx.ensureServiceInitialized(); err != nil {
		return err
	}

	// Check if router has been configured
	var router *testRouter
	if err := ctx.app.GetService("router", &router); err != nil {
		return fmt.Errorf("router service should be available: %w", err)
	}

	return nil
}

// Configuration setup functions for specific scenarios

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyConfiguredForErrorHandling() error {
	ctx.resetContext()

	// Set up application with error handling configuration
	app, err := modular.NewApplication(modular.WithLogger(&testLogger{}))
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}
	ctx.app = app

	// Initialize controlled failure mode
	failureMode := false
	ctx.controlledFailureMode = &failureMode

	// Create a controllable backend that can switch between success and failure
	controllableServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ctx.controlledFailureMode != nil && *ctx.controlledFailureMode {
			// Return error response when failure mode is enabled
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Backend error occurred"))
			return
		}
		// Return success response when failure mode is disabled
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend success"))
	}))
	ctx.testServers = append(ctx.testServers, controllableServer)

	// Configure reverse proxy with error handling features
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"test-backend": controllableServer.URL,
		},
		Routes: map[string]string{
			"/api/test":  "test-backend", // For both normal and error tests
			"/api/error": "test-backend", // Also maps to same backend
		},
		DefaultBackend: "test-backend",
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 2,
			OpenTimeout:      100 * time.Millisecond,
		},
	}

	return ctx.setupApplicationWithConfig()
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyConfiguredForConnectionFailureHandling() error {
	ctx.resetContext()

	// Set up application with connection failure handling
	app, err := modular.NewApplication(modular.WithLogger(&testLogger{}))
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}
	ctx.app = app

	// Configure reverse proxy for connection failure scenarios
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"failing-backend": "http://127.0.0.1:1", // Unreachable port
		},
		Routes: map[string]string{
			"/api/fail": "failing-backend",
		},
		DefaultBackend: "failing-backend",
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 3,
			OpenTimeout:      200 * time.Millisecond,
		},
	}

	return ctx.setupApplicationWithConfig()
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithPerBackendCircuitBreakerSettings() error {
	// This configuration step is already covered by differentBackendsFailAtDifferentRates
	// But we need a specific setup step for the feature scenarios
	return ctx.differentBackendsFailAtDifferentRates()
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithCircuitBreakersInHalfopenState() error {
	// First set up a circuit breaker scenario
	if err := ctx.iHaveAReverseProxyConfiguredForErrorHandling(); err != nil {
		return err
	}

	// Initialize and start to trigger circuit breaker setup
	if err := ctx.ensureServiceInitialized(); err != nil {
		return err
	}

	// Make some failing requests to open the circuit breaker
	for i := 0; i < 5; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", "/api/error", nil)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for circuit breaker to transition to half-open
	time.Sleep(250 * time.Millisecond)

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveMultipleBackendsConfigured() error {
	ctx.resetContext()

	app, err := modular.NewApplication(modular.WithLogger(&testLogger{}))
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}
	ctx.app = app

	// Create multiple test backends
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend1 response"))
	}))
	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend2 response"))
	}))
	backend3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend3 response"))
	}))
	ctx.testServers = append(ctx.testServers, backend1, backend2, backend3)

	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"backend-1": backend1.URL,
			"backend-2": backend2.URL,
			"backend-3": backend3.URL,
		},
		Routes: map[string]string{
			"/api/test": "backend-1",
		},
		DefaultBackend: "backend-1",
	}

	return ctx.setupApplicationWithConfig()
}

// Test logger implementation for BDD tests
type testLogger struct{}

func (l *testLogger) Debug(msg string, keysAndValues ...interface{}) {
	fmt.Printf("DEBUG: %s %v\n", msg, keysAndValues)
}
func (l *testLogger) Info(msg string, keysAndValues ...interface{}) {
	fmt.Printf("INFO: %s %v\n", msg, keysAndValues)
}
func (l *testLogger) Warn(msg string, keysAndValues ...interface{}) {
	fmt.Printf("WARN: %s %v\n", msg, keysAndValues)
}
func (l *testLogger) Error(msg string, keysAndValues ...interface{}) {
	fmt.Printf("ERROR: %s %v\n", msg, keysAndValues)
}
func (l *testLogger) With(keysAndValues ...interface{}) modular.Logger { return l }

// Test helper types

// mockConfigFeeder implements modular.Feeder for testing
type mockConfigFeeder struct {
	configs map[string]interface{}
}

func (m *mockConfigFeeder) Feed(structure interface{}) error {
	// Check if this is the reverseproxy config we want to populate
	if rpConfig, ok := structure.(*ReverseProxyConfig); ok {
		if sourceConfig, exists := m.configs["reverseproxy"]; exists {
			if sourceRpConfig, ok := sourceConfig.(*ReverseProxyConfig); ok {
				// Copy the fields from our source config
				*rpConfig = *sourceRpConfig
				return nil
			}
		}
	}

	return nil
}

// testMetrics implements a basic metrics interface for testing
type testMetrics struct{}

func (m *testMetrics) IncrementCounter(name string, labels ...string)               {}
func (m *testMetrics) AddToGauge(name string, value float64, labels ...string)      {}
func (m *testMetrics) RecordHistogram(name string, value float64, labels ...string) {}

// testEventBus implements a basic event bus interface for testing
type testEventBus struct {
	observers []modular.Observer
}

func (e *testEventBus) Publish(topic string, event interface{}) error {
	return nil
}

func (e *testEventBus) Subscribe(topic string, handler interface{}) error {
	return nil
}

func (e *testEventBus) RegisterObserver(observer modular.Observer, eventTypes ...string) error {
	e.observers = append(e.observers, observer)
	return nil
}

func (e *testEventBus) UnregisterObserver(observer modular.Observer) error {
	for i, obs := range e.observers {
		if obs == observer {
			e.observers = append(e.observers[:i], e.observers[i+1:]...)
			break
		}
	}
	return nil
}

func (e *testEventBus) NotifyObservers(ctx context.Context, event cloudevents.Event) error {
	for _, observer := range e.observers {
		if err := observer.OnEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// TestFeatureFlagEvaluator implements FeatureFlagEvaluator for testing with configurable flags
type TestFeatureFlagEvaluator struct {
	flags map[string]bool
}

func (t *TestFeatureFlagEvaluator) Weight() int {
	return 100 // High priority for test evaluator
}

func (t *TestFeatureFlagEvaluator) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
	if t.flags == nil {
		return false, ErrNoDecision // Let default value handling work properly
	}
	enabled, exists := t.flags[flagID]
	if !exists {
		return false, ErrNoDecision // Flag not found, use default value
	}
	return enabled, nil
}

func (t *TestFeatureFlagEvaluator) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
	result, err := t.EvaluateFlag(ctx, flagID, tenantID, req)
	if err != nil {
		return defaultValue
	}
	return result
}

// deepCopyConfig creates a deep copy of a ReverseProxyConfig to prevent shared state between tests
// NOTE: deepCopyConfig has been removed. Use modular.DeepCopyConfig() instead.
// The framework now exports a generic deep copy function that works for all config types.
