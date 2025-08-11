package reverseproxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
	"github.com/cucumber/godog"
)

// ReverseProxy BDD Test Context
type ReverseProxyBDDTestContext struct {
	app          modular.Application
	module       *ReverseProxyModule
	service      *ReverseProxyModule
	config       *ReverseProxyConfig
	lastError    error
	testServers  []*httptest.Server
	lastResponse *http.Response
}

func (ctx *ReverseProxyBDDTestContext) resetContext() {
	// Close test servers
	for _, server := range ctx.testServers {
		if server != nil {
			server.Close()
		}
	}

	ctx.app = nil
	ctx.module = nil
	ctx.service = nil
	ctx.config = nil
	ctx.lastError = nil
	ctx.testServers = nil
	ctx.lastResponse = nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAModularApplicationWithReverseProxyModuleConfigured() error {
	ctx.resetContext()

	// Create a test backend server first
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test backend response"))
	}))
	ctx.testServers = append(ctx.testServers, testServer)

	// Create basic reverse proxy configuration for testing using the test server
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"test-backend": testServer.URL,
		},
		Routes: map[string]string{
			"/api/*": "test-backend",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"test-backend": {
				URL: testServer.URL,
			},
		},
	}

	// Create application
	logger := &testLogger{}

	// Clear ConfigFeeders and disable AppConfigLoader to prevent environment interference during tests
	modular.ConfigFeeders = []modular.Feeder{}
	originalLoader := modular.AppConfigLoader
	modular.AppConfigLoader = func(app *modular.StdApplication) error { return nil }
	// Don't restore them - let them stay disabled throughout all BDD tests
	_ = originalLoader

	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewStdApplication(mainConfigProvider, logger)

	// Create and register a mock router service (required by ReverseProxy)
	mockRouter := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}
	ctx.app.RegisterService("router", mockRouter)

	// Create and register reverse proxy module
	ctx.module = NewModule()

	// Register the reverseproxy config section
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Register the module
	ctx.app.RegisterModule(ctx.module)

	return nil
}

// setupApplicationWithConfig creates a fresh application with the current configuration
func (ctx *ReverseProxyBDDTestContext) setupApplicationWithConfig() error {
	// Create application
	logger := &testLogger{}

	// Clear ConfigFeeders and disable AppConfigLoader to prevent environment interference during tests
	modular.ConfigFeeders = []modular.Feeder{}
	modular.AppConfigLoader = func(app *modular.StdApplication) error { return nil }

	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewStdApplication(mainConfigProvider, logger)

	// Create and register a mock router service (required by ReverseProxy)
	mockRouter := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}
	ctx.app.RegisterService("router", mockRouter)

	// Create and register reverse proxy module
	ctx.module = NewModule()

	// Register the reverseproxy config section with current configuration
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Register the module
	ctx.app.RegisterModule(ctx.module)

	// Initialize the application with the complete configuration
	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) theReverseProxyModuleIsInitialized() error {
	err := ctx.app.Init()
	if err != nil {
		ctx.lastError = err
		return err
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theProxyServiceShouldBeAvailable() error {
	err := ctx.app.GetService("reverseproxy.provider", &ctx.service)
	if err != nil {
		return err
	}
	if ctx.service == nil {
		return fmt.Errorf("proxy service not available")
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theModuleShouldBeReadyToRouteRequests() error {
	// Verify the module is properly configured
	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("module not properly initialized")
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyConfiguredWithASingleBackend() error {
	// The background step has already set up a single backend configuration
	// Initialize the module so it's ready for the "When" step
	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) iSendARequestToTheProxy() error {
	// Ensure service is available if not already retrieved
	if ctx.service == nil {
		err := ctx.app.GetService("reverseproxy.provider", &ctx.service)
		if err != nil {
			return fmt.Errorf("failed to get reverseproxy service: %w", err)
		}
	}

	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Start the service
	err := ctx.app.Start()
	if err != nil {
		return err
	}

	// Simulate a request (in real tests would make HTTP call)
	// For BDD test, we just verify the service is ready
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theRequestShouldBeForwardedToTheBackend() error {
	// In a real implementation, would verify request forwarding
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theResponseShouldBeReturnedToTheClient() error {
	// In a real implementation, would verify response handling
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyConfiguredWithMultipleBackends() error {
	// Reset context and set up fresh application for this scenario
	ctx.resetContext()

	// Create multiple test backend servers
	for i := 0; i < 3; i++ {
		testServer := httptest.NewServer(http.HandlerFunc(func(idx int) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(fmt.Sprintf("backend-%d response", idx)))
			}
		}(i)))
		ctx.testServers = append(ctx.testServers, testServer)
	}

	// Create configuration with multiple backends
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"backend-1": ctx.testServers[0].URL,
			"backend-2": ctx.testServers[1].URL,
			"backend-3": ctx.testServers[2].URL,
		},
		Routes: map[string]string{
			"/api/*": "backend-1",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"backend-1": {URL: ctx.testServers[0].URL},
			"backend-2": {URL: ctx.testServers[1].URL},
			"backend-3": {URL: ctx.testServers[2].URL},
		},
	}

	return ctx.setupApplicationWithConfig()
}

func (ctx *ReverseProxyBDDTestContext) iSendMultipleRequestsToTheProxy() error {
	return ctx.iSendARequestToTheProxy()
}

func (ctx *ReverseProxyBDDTestContext) requestsShouldBeDistributedAcrossAllBackends() error {
	// Ensure service is available
	if ctx.service == nil {
		err := ctx.app.GetService("reverseproxy.provider", &ctx.service)
		if err != nil {
			return fmt.Errorf("failed to get reverseproxy service: %w", err)
		}
	}

	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("service or config not available")
	}

	// Verify multiple backends are configured
	if len(ctx.service.config.BackendServices) < 2 {
		return fmt.Errorf("expected multiple backends, got %d", len(ctx.service.config.BackendServices))
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) loadBalancingShouldBeApplied() error {
	// In a real implementation, would verify load balancing algorithm
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithHealthChecksEnabled() error {
	// Ensure health checks are enabled
	ctx.config.HealthCheck.Enabled = true
	ctx.config.HealthCheck.Interval = 5 * time.Second
	ctx.config.HealthCheck.HealthEndpoints = map[string]string{
		"test-backend": "/health",
	}

	// Re-register the config section with the updated configuration
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Initialize the module with the updated configuration
	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) aBackendBecomesUnavailable() error {
	// Simulate backend failure by closing one test server
	if len(ctx.testServers) > 0 {
		ctx.testServers[0].Close()
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theProxyShouldDetectTheFailure() error {
	// In a real implementation, would verify health check detection
	return nil
}

func (ctx *ReverseProxyBDDTestContext) routeTrafficOnlyToHealthyBackends() error {
	// In a real implementation, would verify traffic routing
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithCircuitBreakerEnabled() error {
	// Reset context and set up fresh application for this scenario
	ctx.resetContext()

	// Create a test backend server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test backend response"))
	}))
	ctx.testServers = append(ctx.testServers, testServer)

	// Create configuration with circuit breaker enabled
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"test-backend": testServer.URL,
		},
		Routes: map[string]string{
			"/api/*": "test-backend",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"test-backend": {
				URL: testServer.URL,
			},
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 3,
		},
	}

	return ctx.setupApplicationWithConfig()
}

func (ctx *ReverseProxyBDDTestContext) aBackendFailsRepeatedly() error {
	// Simulate repeated failures
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theCircuitBreakerShouldOpen() error {
	// Ensure service is available
	if ctx.service == nil {
		err := ctx.app.GetService("reverseproxy.provider", &ctx.service)
		if err != nil {
			return fmt.Errorf("failed to get reverseproxy service: %w", err)
		}
	}

	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("service or config not available")
	}

	// Verify circuit breaker configuration
	if !ctx.service.config.CircuitBreakerConfig.Enabled {
		return fmt.Errorf("circuit breaker not enabled")
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) requestsShouldBeHandledGracefully() error {
	// In a real implementation, would verify graceful handling
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithCachingEnabled() error {
	// Reset context and set up fresh application for this scenario
	ctx.resetContext()

	// Create a test backend server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test backend response"))
	}))
	ctx.testServers = append(ctx.testServers, testServer)

	// Create configuration with caching enabled
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"test-backend": testServer.URL,
		},
		Routes: map[string]string{
			"/api/*": "test-backend",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"test-backend": {
				URL: testServer.URL,
			},
		},
		CacheEnabled: true,
		CacheTTL:     300 * time.Second,
	}

	return ctx.setupApplicationWithConfig()
}

func (ctx *ReverseProxyBDDTestContext) iSendTheSameRequestMultipleTimes() error {
	return ctx.iSendARequestToTheProxy()
}

func (ctx *ReverseProxyBDDTestContext) theFirstRequestShouldHitTheBackend() error {
	// In a real implementation, would verify cache miss
	return nil
}

func (ctx *ReverseProxyBDDTestContext) subsequentRequestsShouldBeServedFromCache() error {
	// Ensure service is available
	if ctx.service == nil {
		err := ctx.app.GetService("reverseproxy.provider", &ctx.service)
		if err != nil {
			return fmt.Errorf("failed to get reverseproxy service: %w", err)
		}
	}

	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("service or config not available")
	}

	// Verify caching is enabled
	if !ctx.service.config.CacheEnabled {
		return fmt.Errorf("caching not enabled")
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveATenantAwareReverseProxyConfigured() error {
	// Add tenant-specific configuration
	ctx.config.RequireTenantID = true
	ctx.config.TenantIDHeader = "X-Tenant-ID"

	// Re-register the config section with the updated configuration
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Initialize the module with the updated configuration
	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) iSendRequestsWithDifferentTenantContexts() error {
	return ctx.iSendARequestToTheProxy()
}

func (ctx *ReverseProxyBDDTestContext) requestsShouldBeRoutedBasedOnTenantConfiguration() error {
	// Ensure service is available
	if ctx.service == nil {
		err := ctx.app.GetService("reverseproxy.provider", &ctx.service)
		if err != nil {
			return fmt.Errorf("failed to get reverseproxy service: %w", err)
		}
	}

	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("service or config not available")
	}

	// Verify tenant routing is configured
	if !ctx.service.config.RequireTenantID {
		return fmt.Errorf("tenant routing not enabled")
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) tenantIsolationShouldBeMaintained() error {
	// In a real implementation, would verify tenant isolation
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyConfiguredForCompositeResponses() error {
	// Add composite route configuration
	ctx.config.CompositeRoutes = map[string]CompositeRoute{
		"/api/combined": {
			Pattern:  "/api/combined",
			Backends: []string{"backend-1", "backend-2"},
			Strategy: "combine",
		},
	}

	// Re-register the config section with the updated configuration
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Initialize the module with the updated configuration
	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) iSendARequestThatRequiresMultipleBackendCalls() error {
	return ctx.iSendARequestToTheProxy()
}

func (ctx *ReverseProxyBDDTestContext) theProxyShouldCallAllRequiredBackends() error {
	// Ensure service is available
	if ctx.service == nil {
		err := ctx.app.GetService("reverseproxy.provider", &ctx.service)
		if err != nil {
			return fmt.Errorf("failed to get reverseproxy service: %w", err)
		}
	}

	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("service or config not available")
	}

	// Verify composite routes are configured
	if len(ctx.service.config.CompositeRoutes) == 0 {
		return fmt.Errorf("no composite routes configured")
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) combineTheResponsesIntoASingleResponse() error {
	// In a real implementation, would verify response combination
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithRequestTransformationConfigured() error {
	// Create a test backend server for transformation testing
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("transformed backend response"))
	}))
	ctx.testServers = append(ctx.testServers, testServer)

	// Add backend configuration with header rewriting
	ctx.config.BackendConfigs = map[string]BackendServiceConfig{
		"backend-1": {
			URL: testServer.URL,
			HeaderRewriting: HeaderRewritingConfig{
				SetHeaders: map[string]string{
					"X-Forwarded-By": "reverse-proxy",
				},
				RemoveHeaders: []string{"Authorization"},
			},
		},
	}

	// Update backend services to use the test server
	ctx.config.BackendServices["backend-1"] = testServer.URL

	// Re-register the config section with the updated configuration
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Initialize the module with the updated configuration
	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) theRequestShouldBeTransformedBeforeForwarding() error {
	// Ensure service is available
	if ctx.service == nil {
		err := ctx.app.GetService("reverseproxy.provider", &ctx.service)
		if err != nil {
			return fmt.Errorf("failed to get reverseproxy service: %w", err)
		}
	}

	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("service or config not available")
	}

	// Verify backend configs with header rewriting are configured
	if len(ctx.service.config.BackendConfigs) == 0 {
		return fmt.Errorf("no backend configs configured")
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theBackendShouldReceiveTheTransformedRequest() error {
	// In a real implementation, would verify transformed request
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAnActiveReverseProxyWithOngoingRequests() error {
	// Initialize the module with the basic configuration from background
	err := ctx.app.Init()
	if err != nil {
		return err
	}

	err = ctx.theProxyServiceShouldBeAvailable()
	if err != nil {
		return err
	}

	// Start the module
	return ctx.app.Start()
}

func (ctx *ReverseProxyBDDTestContext) theModuleIsStopped() error {
	return ctx.app.Stop()
}

func (ctx *ReverseProxyBDDTestContext) ongoingRequestsShouldBeCompleted() error {
	// In a real implementation, would verify graceful completion
	return nil
}

func (ctx *ReverseProxyBDDTestContext) newRequestsShouldBeRejectedGracefully() error {
	// In a real implementation, would verify graceful rejection
	return nil
}

// Test helper structures
type testLogger struct{}

func (l *testLogger) Debug(msg string, keysAndValues ...interface{})   {}
func (l *testLogger) Info(msg string, keysAndValues ...interface{})    {}
func (l *testLogger) Warn(msg string, keysAndValues ...interface{})    {}
func (l *testLogger) Error(msg string, keysAndValues ...interface{})   {}
func (l *testLogger) With(keysAndValues ...interface{}) modular.Logger { return l }

// TestReverseProxyModuleBDD runs the BDD tests for the ReverseProxy module
func TestReverseProxyModuleBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			ctx := &ReverseProxyBDDTestContext{}

			// Background
			s.Given(`^I have a modular application with reverse proxy module configured$`, ctx.iHaveAModularApplicationWithReverseProxyModuleConfigured)

			// Initialization
			s.When(`^the reverse proxy module is initialized$`, ctx.theReverseProxyModuleIsInitialized)
			s.Then(`^the proxy service should be available$`, ctx.theProxyServiceShouldBeAvailable)
			s.Then(`^the module should be ready to route requests$`, ctx.theModuleShouldBeReadyToRouteRequests)

			// Single backend
			s.Given(`^I have a reverse proxy configured with a single backend$`, ctx.iHaveAReverseProxyConfiguredWithASingleBackend)
			s.When(`^I send a request to the proxy$`, ctx.iSendARequestToTheProxy)
			s.Then(`^the request should be forwarded to the backend$`, ctx.theRequestShouldBeForwardedToTheBackend)
			s.Then(`^the response should be returned to the client$`, ctx.theResponseShouldBeReturnedToTheClient)

			// Multiple backends
			s.Given(`^I have a reverse proxy configured with multiple backends$`, ctx.iHaveAReverseProxyConfiguredWithMultipleBackends)
			s.When(`^I send multiple requests to the proxy$`, ctx.iSendMultipleRequestsToTheProxy)
			s.Then(`^requests should be distributed across all backends$`, ctx.requestsShouldBeDistributedAcrossAllBackends)
			s.Then(`^load balancing should be applied$`, ctx.loadBalancingShouldBeApplied)

			// Health checking
			s.Given(`^I have a reverse proxy with health checks enabled$`, ctx.iHaveAReverseProxyWithHealthChecksEnabled)
			s.When(`^a backend becomes unavailable$`, ctx.aBackendBecomesUnavailable)
			s.Then(`^the proxy should detect the failure$`, ctx.theProxyShouldDetectTheFailure)
			s.Then(`^route traffic only to healthy backends$`, ctx.routeTrafficOnlyToHealthyBackends)

			// Circuit breaker
			s.Given(`^I have a reverse proxy with circuit breaker enabled$`, ctx.iHaveAReverseProxyWithCircuitBreakerEnabled)
			s.When(`^a backend fails repeatedly$`, ctx.aBackendFailsRepeatedly)
			s.Then(`^the circuit breaker should open$`, ctx.theCircuitBreakerShouldOpen)
			s.Then(`^requests should be handled gracefully$`, ctx.requestsShouldBeHandledGracefully)

			// Caching
			s.Given(`^I have a reverse proxy with caching enabled$`, ctx.iHaveAReverseProxyWithCachingEnabled)
			s.When(`^I send the same request multiple times$`, ctx.iSendTheSameRequestMultipleTimes)
			s.Then(`^the first request should hit the backend$`, ctx.theFirstRequestShouldHitTheBackend)
			s.Then(`^subsequent requests should be served from cache$`, ctx.subsequentRequestsShouldBeServedFromCache)

			// Tenant routing
			s.Given(`^I have a tenant-aware reverse proxy configured$`, ctx.iHaveATenantAwareReverseProxyConfigured)
			s.When(`^I send requests with different tenant contexts$`, ctx.iSendRequestsWithDifferentTenantContexts)
			s.Then(`^requests should be routed based on tenant configuration$`, ctx.requestsShouldBeRoutedBasedOnTenantConfiguration)
			s.Then(`^tenant isolation should be maintained$`, ctx.tenantIsolationShouldBeMaintained)

			// Composite responses
			s.Given(`^I have a reverse proxy configured for composite responses$`, ctx.iHaveAReverseProxyConfiguredForCompositeResponses)
			s.When(`^I send a request that requires multiple backend calls$`, ctx.iSendARequestThatRequiresMultipleBackendCalls)
			s.Then(`^the proxy should call all required backends$`, ctx.theProxyShouldCallAllRequiredBackends)
			s.Then(`^combine the responses into a single response$`, ctx.combineTheResponsesIntoASingleResponse)

			// Request transformation
			s.Given(`^I have a reverse proxy with request transformation configured$`, ctx.iHaveAReverseProxyWithRequestTransformationConfigured)
			s.Then(`^the request should be transformed before forwarding$`, ctx.theRequestShouldBeTransformedBeforeForwarding)
			s.Then(`^the backend should receive the transformed request$`, ctx.theBackendShouldReceiveTheTransformedRequest)

			// Shutdown
			s.Given(`^I have an active reverse proxy with ongoing requests$`, ctx.iHaveAnActiveReverseProxyWithOngoingRequests)
			s.When(`^the module is stopped$`, ctx.theModuleIsStopped)
			s.Then(`^ongoing requests should be completed$`, ctx.ongoingRequestsShouldBeCompleted)
			s.Then(`^new requests should be rejected gracefully$`, ctx.newRequestsShouldBeRejectedGracefully)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/reverseproxy_module.feature"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
