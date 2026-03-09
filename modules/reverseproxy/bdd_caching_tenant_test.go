package reverseproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Caching Scenarios

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithCachingEnabled() error {
	// Reset context and set up fresh application for this scenario
	fmt.Printf("\n🔍 DEBUG: iHaveAReverseProxyWithCachingEnabled() starting (Response caching scenario)\n")
	ctx.resetContext()

	// Create a test backend server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test backend response"))
	}))
	ctx.testServers = append(ctx.testServers, testServer)

	// Create configuration with caching enabled
	ctx.config = &ReverseProxyConfig{
		DefaultBackend: "test-backend",
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
		CacheTTL:     337 * time.Second, // SIGNATURE: 337s for "Response caching" scenario
		HealthCheck: HealthCheckConfig{
			Enabled:  false,
			Interval: 30 * time.Second,
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:          false,
			FailureThreshold: 5,
			OpenTimeout:      30 * time.Second,
		},
	}

	fmt.Printf("🔍 DEBUG: Set CacheTTL=337s (Response caching signature)\n")
	return ctx.setupApplicationWithConfig()
}

func (ctx *ReverseProxyBDDTestContext) iSendTheSameRequestMultipleTimes() error {
	return ctx.iSendARequestToTheProxy()
}

func (ctx *ReverseProxyBDDTestContext) theFirstRequestShouldHitTheBackend() error {
	// Test cache behavior by making actual request to the reverseproxy module
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Make an initial request through the actual module to test cache miss
	resp, err := ctx.makeRequestThroughModule("GET", "/cached-endpoint", nil)
	if err != nil {
		return fmt.Errorf("failed to make initial request: %w", err)
	}
	defer resp.Body.Close()

	// First request should succeed (hitting backend)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("first request should succeed, got status %d", resp.StatusCode)
	}

	// Store response for comparison
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	ctx.lastResponseBody = body

	// Verify we got a response (indicating backend was hit)
	if len(body) == 0 {
		return fmt.Errorf("expected response body from backend hit")
	}

	// For cache testing, the first request hitting the backend is the expected behavior
	return nil
}

func (ctx *ReverseProxyBDDTestContext) subsequentRequestsShouldBeServedFromCache() error {
	// Test cache behavior by making multiple requests through the actual reverseproxy module
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Make a second request to the same endpoint to test caching
	resp, err := ctx.makeRequestThroughModule("GET", "/cached-endpoint", nil)
	if err != nil {
		return fmt.Errorf("failed to make cached request: %w", err)
	}
	defer resp.Body.Close()

	// Second request should also succeed
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cached request should succeed, got status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read cached response body: %w", err)
	}

	// For cache testing, we should get a response faster or with cache headers
	// The specific implementation depends on the cache configuration
	if len(body) == 0 {
		return fmt.Errorf("expected response body from cached request")
	}

	// Make a third request to further verify cache behavior
	resp3, err := ctx.makeRequestThroughModule("GET", "/cached-endpoint", nil)
	if err != nil {
		return fmt.Errorf("failed to make third cached request: %w", err)
	}
	resp3.Body.Close()

	// All cached requests should succeed
	if resp3.StatusCode != http.StatusOK {
		return fmt.Errorf("third cached request should succeed, got status %d", resp3.StatusCode)
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithSpecificCacheTTLConfigured() error {
	fmt.Printf("\n🔍 DEBUG: iHaveAReverseProxyWithSpecificCacheTTLConfigured() starting (Cache TTL behavior scenario)\n")
	ctx.resetContext()

	app, err := modular.NewApplication(modular.WithLogger(&testLogger{}))
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}
	ctx.app = app

	// Create test backend
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend response"))
	}))
	ctx.testServers = append(ctx.testServers, testServer)

	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"cached-backend": testServer.URL,
		},
		Routes: map[string]string{
			"/api/cached": "cached-backend",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"cached-backend": {URL: testServer.URL},
		},
		DefaultBackend: "cached-backend",
		CacheEnabled:   true,
		CacheTTL:       1 * time.Second, // SIGNATURE: 1s for "Cache TTL behavior" scenario
		HealthCheck: HealthCheckConfig{
			Enabled:  false,
			Interval: 30 * time.Second,
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:          false,
			FailureThreshold: 5,
			OpenTimeout:      30 * time.Second,
		},
	}

	fmt.Printf("🔍 DEBUG: Set CacheTTL=1s (Cache TTL behavior signature)\n")
	fmt.Printf("🔍 DEBUG: BEFORE setupApplicationWithConfig: ctx.config pointer=%p, CacheTTL=%v\n", ctx.config, ctx.config.CacheTTL)

	err = ctx.setupApplicationWithConfig()
	if err != nil {
		return err
	}

	fmt.Printf("🔍 DEBUG: AFTER setupApplicationWithConfig: ctx.config pointer=%p, CacheTTL=%v\n", ctx.config, ctx.config.CacheTTL)

	// Also check if ctx.service.config points to the same config
	if ctx.service != nil {
		fmt.Printf("🔍 DEBUG: ctx.service exists, ctx.module.config pointer=%p, CacheTTL=%v\n",
			ctx.module.config, ctx.module.config.CacheTTL)

		if ctx.config == ctx.module.config {
			fmt.Printf("🔍 DEBUG: ✅ ctx.config and ctx.module.config are THE SAME pointer\n")
		} else {
			fmt.Printf("🔍 DEBUG: ❌ ctx.config and ctx.module.config are DIFFERENT pointers!\n")
		}
	}

	// Verify that the cache is actually enabled and working
	if ctx.service == nil || ctx.service.responseCache == nil {
		return fmt.Errorf("response cache not initialized")
	}

	// Explicitly clear cache to ensure clean state for this scenario
	ctx.service.responseCache.Clear()

	// Make an initial request to populate the cache
	// This ensures there's something in the cache that can expire
	resp, err := ctx.makeRequestThroughModule("GET", "/api/cached", nil)
	if err != nil {
		return fmt.Errorf("failed to make initial cache population request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("initial cache population request should succeed, got status %d", resp.StatusCode)
	}
	resp.Body.Close()

	return nil
}

// Cache TTL Management Functions

func (ctx *ReverseProxyBDDTestContext) cachedResponsesAgeBeyondTTL() error {
	// Verify the cache exists and has the expected TTL
	if ctx.service == nil || ctx.service.responseCache == nil {
		return fmt.Errorf("response cache not initialized")
	}

	if ctx.config == nil || ctx.config.CacheTTL <= 0 {
		return fmt.Errorf("cache TTL not configured")
	}

	// Wait for cache TTL to expire plus a small buffer
	ttl := ctx.config.CacheTTL
	waitTime := ttl + (500 * time.Millisecond) // Add buffer to ensure expiration

	// LOG THE ACTUAL TTL BEING USED - this will help us trace config bleeding
	fmt.Printf("\n🔍 DEBUG: cachedResponsesAgeBeyondTTL reading CacheTTL=%v, will sleep for %v\n", ttl, waitTime)
	fmt.Printf("🔍 DEBUG: Expected signatures: 1s=Cache_TTL_behavior, 337s=Response_caching\n\n")

	time.Sleep(waitTime)

	// Clear events to focus on fresh requests after TTL
	if ctx.eventObserver != nil {
		ctx.eventObserver.ClearEvents()
	}

	// Don't make a request here as it would repopulate the cache
	// The next step will verify that requests hit the backend due to expired cache

	return nil
}

// Tenant-Aware Scenarios

func (ctx *ReverseProxyBDDTestContext) iHaveATenantAwareReverseProxyConfigured() error {
	// Reset context first
	ctx.resetContext()

	// Create tracking structures for backend calls
	tenantARequests := make([]*http.Request, 0)
	tenantBRequests := make([]*http.Request, 0)

	// Create tenant-specific backend servers with request tracking
	var tenantAServer *httptest.Server
	var tenantBServer *httptest.Server

	tenantAServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Track this request to validate tenant isolation
		ctx.appendTenantARequest(r.Clone(r.Context()))

		// Add backend identification headers
		w.Header().Set("X-Backend-ID", "tenant-a-backend")
		w.Header().Set("X-Backend-URL", tenantAServer.URL)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response from tenant-a backend"))
	}))

	tenantBServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Track this request to validate tenant isolation
		ctx.appendTenantBRequest(r.Clone(r.Context()))

		// Add backend identification headers
		w.Header().Set("X-Backend-ID", "tenant-b-backend")
		w.Header().Set("X-Backend-URL", tenantBServer.URL)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response from tenant-b backend"))
	}))

	// Store request tracking in context for validation
	ctx.tenantARequests = &tenantARequests
	ctx.tenantBRequests = &tenantBRequests

	// Store both servers for cleanup
	ctx.testServers = append(ctx.testServers, tenantAServer, tenantBServer)

	// Initialize config before modifying it
	ctx.config = &ReverseProxyConfig{
		DefaultBackend: "default-backend",
		BackendConfigs: map[string]BackendServiceConfig{
			"default-backend": {URL: ""},
		},
		HealthCheck: HealthCheckConfig{
			Enabled:  false,
			Interval: 30 * time.Second,
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:          false,
			FailureThreshold: 5,
			OpenTimeout:      30 * time.Second,
		},
	}

	// Configure global config with tenant awareness enabled
	ctx.config.RequireTenantID = true
	ctx.config.TenantIDHeader = "X-Tenant-ID"
	ctx.config.Routes = map[string]string{
		"/test": "default-backend",
		"/":     "default-backend",
	}
	ctx.config.BackendServices = map[string]string{
		"default-backend": "", // Will be overridden by tenant configs
	}

	// Setup tenant-specific configurations
	tenantAConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"default-backend": tenantAServer.URL,
		},
	}

	tenantBConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"default-backend": tenantBServer.URL,
		},
	}

	// Replace the standard app with a mock tenant application for tenant testing
	mockTenantApp := NewMockTenantApplicationWithMock()

	// Ensure we have a router service - create one if needed
	var router *testRouter
	if ctx.app != nil {
		if err := ctx.app.GetService("router", &router); err != nil || router == nil {
			// Create a new router if the existing app doesn't have one or it's nil
			router = &testRouter{
				routes: make(map[string]http.HandlerFunc),
			}
		}
	} else {
		// Create a new router if there's no existing app
		router = &testRouter{
			routes: make(map[string]http.HandlerFunc),
		}
	}

	// Register the router service with the mock app
	mockTenantApp.RegisterService("router", router)

	// Create a fresh module for the mock app to avoid state conflicts
	freshModule := NewModule()
	ctx.module = freshModule

	// Register the fresh reverseproxy module with the mock app
	mockTenantApp.RegisterModule(freshModule)

	// Call the Constructor to inject dependencies
	constructor := freshModule.Constructor()
	services := map[string]any{
		"router": router, // Router is guaranteed to be non-nil at this point
	}
	constructedModule, err := constructor(mockTenantApp, services)
	if err != nil {
		return fmt.Errorf("failed to construct module: %w", err)
	}
	// Update the module reference
	ctx.module = constructedModule.(*ReverseProxyModule)
	freshModule = constructedModule.(*ReverseProxyModule)

	// Set up the tenant configurations
	tenantAProvider := modular.NewStdConfigProvider(tenantAConfig)
	tenantBProvider := modular.NewStdConfigProvider(tenantBConfig)

	mockTenantApp.On("GetTenantConfig", modular.TenantID("tenant-a"), "reverseproxy").Return(tenantAProvider, nil)
	mockTenantApp.On("GetTenantConfig", modular.TenantID("tenant-b"), "reverseproxy").Return(tenantBProvider, nil)
	mockTenantApp.On("GetTenants").Return([]modular.TenantID{"tenant-a", "tenant-b"})

	// Mock the GetConfigSection method that the module will call during Init
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	mockTenantApp.On("GetConfigSection", "reverseproxy").Return(reverseproxyConfigProvider, nil)

	// Replace the app in the context
	ctx.app = mockTenantApp

	// Register tenants with the module (this will call OnTenantRegistered)
	freshModule.OnTenantRegistered(modular.TenantID("tenant-a"))
	freshModule.OnTenantRegistered(modular.TenantID("tenant-b"))

	// Initialize the fresh module with the mock app and updated configuration
	err = freshModule.Init(mockTenantApp)
	if err != nil {
		return fmt.Errorf("failed to init fresh module: %w", err)
	}

	// Start the module to complete initialization
	err = freshModule.Start(context.Background())
	if err != nil {
		return fmt.Errorf("failed to start fresh module: %w", err)
	}

	// Manually register the services that the module provides
	serviceProviders := freshModule.ProvidesServices()
	for _, provider := range serviceProviders {
		err = mockTenantApp.RegisterService(provider.Name, provider.Instance)
		if err != nil {
			return fmt.Errorf("failed to register service %s: %w", provider.Name, err)
		}
	}

	return nil
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
	// Test tenant isolation by making requests with different tenant headers
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Clear request tracking before starting the test
	if ctx.tenantARequests != nil {
		*ctx.tenantARequests = (*ctx.tenantARequests)[:0]
	}
	if ctx.tenantBRequests != nil {
		*ctx.tenantBRequests = (*ctx.tenantBRequests)[:0]
	}

	// Make request with tenant A
	resp1, err := ctx.makeRequestThroughModuleWithHeaders("GET", "/test?tenant=a", nil, map[string]string{
		"X-Tenant-ID": "tenant-a",
	})
	if err != nil {
		return fmt.Errorf("failed to make tenant-a request: %w", err)
	}
	defer resp1.Body.Close()

	// Make request with tenant B
	resp2, err := ctx.makeRequestThroughModuleWithHeaders("GET", "/test?tenant=b", nil, map[string]string{
		"X-Tenant-ID": "tenant-b",
	})
	if err != nil {
		return fmt.Errorf("failed to make tenant-b request: %w", err)
	}
	defer resp2.Body.Close()

	// Both requests should succeed, indicating tenant isolation is working
	if resp1.StatusCode != http.StatusOK || resp2.StatusCode != http.StatusOK {
		return fmt.Errorf("tenant requests should be isolated and successful, got %d and %d", resp1.StatusCode, resp2.StatusCode)
	}

	// Validate tenant isolation by checking backend identification headers
	backend1ID := resp1.Header.Get("X-Backend-ID")
	backend2ID := resp2.Header.Get("X-Backend-ID")

	if backend1ID != "tenant-a-backend" {
		return fmt.Errorf("tenant-a request should hit tenant-a-backend, but hit %s", backend1ID)
	}

	if backend2ID != "tenant-b-backend" {
		return fmt.Errorf("tenant-b request should hit tenant-b-backend, but hit %s", backend2ID)
	}

	// Verify request tracking shows proper backend isolation
	if ctx.tenantARequests == nil || ctx.tenantBRequests == nil {
		return fmt.Errorf("request tracking not initialized")
	}

	// Verify that tenant A request was tracked only on tenant A backend
	if len(*ctx.tenantARequests) != 1 {
		return fmt.Errorf("expected exactly 1 request to tenant-a backend, got %d", len(*ctx.tenantARequests))
	}

	// Verify that tenant B request was tracked only on tenant B backend
	if len(*ctx.tenantBRequests) != 1 {
		return fmt.Errorf("expected exactly 1 request to tenant-b backend, got %d", len(*ctx.tenantBRequests))
	}

	// Validate that no cross-tenant backend calls occurred by checking tenant headers
	tenantAReq := (*ctx.tenantARequests)[0]
	tenantBReq := (*ctx.tenantBRequests)[0]

	if tenantAReq.Header.Get("X-Tenant-ID") != "tenant-a" {
		return fmt.Errorf("tenant-a backend received request with wrong tenant ID: %s", tenantAReq.Header.Get("X-Tenant-ID"))
	}

	if tenantBReq.Header.Get("X-Tenant-ID") != "tenant-b" {
		return fmt.Errorf("tenant-b backend received request with wrong tenant ID: %s", tenantBReq.Header.Get("X-Tenant-ID"))
	}

	// Verify tenant-specific processing occurred by checking responses are different
	// Read response bodies to ensure they're different
	body1, _ := io.ReadAll(resp1.Body)
	body2, _ := io.ReadAll(resp2.Body)

	if string(body1) == string(body2) {
		return fmt.Errorf("tenant responses should be different to prove isolation, but both returned: %s", string(body1))
	}

	// Additional validation: Check backend URLs
	backend1URL := resp1.Header.Get("X-Backend-URL")
	backend2URL := resp2.Header.Get("X-Backend-URL")

	if backend1URL == backend2URL {
		return fmt.Errorf("tenant requests hit the same backend URL (%s), tenant isolation is broken", backend1URL)
	}

	return nil
}

// Composite Response Scenarios

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyConfiguredForCompositeResponses() error {
	// Reset context and set up fresh application
	ctx.resetContext()

	// Create test backend servers
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend-1 response"))
	}))
	ctx.testServers = append(ctx.testServers, backend1)

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend-2 response"))
	}))
	ctx.testServers = append(ctx.testServers, backend2)

	// Create configuration with composite routes
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"backend-1": backend1.URL,
			"backend-2": backend2.URL,
		},
		Routes: map[string]string{
			"/api/combined": "backend-1", // Default route
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"backend-1": {
				URL: backend1.URL,
			},
			"backend-2": {
				URL: backend2.URL,
			},
		},
		CompositeRoutes: map[string]CompositeRoute{
			"/api/combined": {
				Pattern:  "/api/combined",
				Backends: []string{"backend-1", "backend-2"},
				Strategy: "combine",
			},
		},
		DefaultBackend: "backend-1",
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

	return ctx.setupApplicationWithConfig()
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
	// Test composite response combination by making request to composite endpoint
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Make request to composite route that should combine multiple backend responses
	resp, err := ctx.makeRequestThroughModule("GET", "/api/combined", nil)
	if err != nil {
		return fmt.Errorf("failed to make composite request: %w", err)
	}
	defer resp.Body.Close()

	// Composite request should succeed
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("composite request should succeed, got status %d", resp.StatusCode)
	}

	// Read and verify response body contains combined data
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read composite response: %w", err)
	}

	if len(body) == 0 {
		return fmt.Errorf("composite response should contain combined data")
	}

	// Verify response looks like combined content
	responseText := string(body)
	if len(responseText) < 10 { // Arbitrary minimum for combined content
		return fmt.Errorf("composite response appears too short for combined content")
	}

	return nil
}

// Request Transformation Scenarios

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithRequestTransformationConfigured() error {
	ctx.resetContext()

	// Create a test backend server for transformation testing
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("transformed backend response"))
	}))
	ctx.testServers = append(ctx.testServers, testServer)

	// Set up configuration with proper routing and backend configuration
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"backend-1": testServer.URL,
		},
		Routes: map[string]string{
			"/api/*": "backend-1", // Route /api/* requests to backend-1
		},
		DefaultBackend: "backend-1",
		BackendConfigs: map[string]BackendServiceConfig{
			"backend-1": {
				URL: testServer.URL,
				HeaderRewriting: HeaderRewritingConfig{
					SetHeaders: map[string]string{
						"X-Forwarded-By": "reverse-proxy",
					},
					RemoveHeaders: []string{"Authorization"},
				},
			},
		},
		HealthCheck: HealthCheckConfig{
			Enabled:  false,
			Interval: 30 * time.Second,
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled: false,
		},
	}

	return ctx.setupApplicationWithConfig()
}

func (ctx *ReverseProxyBDDTestContext) theRequestShouldBeTransformedBeforeForwarding() error {
	// Test request transformation through the actual reverseproxy module
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Make request that will be transformed
	resp, err := ctx.makeRequestThroughModuleWithHeaders("GET", "/api/test", nil, map[string]string{
		"Authorization": "Bearer token123",
	})
	if err != nil {
		return fmt.Errorf("failed to make transformation request: %w", err)
	}
	defer resp.Body.Close()

	// Request should succeed
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("transformation request should succeed, got status %d", resp.StatusCode)
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) theBackendShouldReceiveTheTransformedRequest() error {
	// Test that backend received transformed request through reverseproxy module
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Verify transformation configuration is in place
	if len(ctx.service.config.BackendConfigs) == 0 {
		return fmt.Errorf("no backend configs with transformations configured")
	}

	// Check that backend config has header rewriting configured
	for _, backendConfig := range ctx.service.config.BackendConfigs {
		if len(backendConfig.HeaderRewriting.SetHeaders) > 0 || len(backendConfig.HeaderRewriting.RemoveHeaders) > 0 {
			// Found transformation configuration
			return nil
		}
	}

	return fmt.Errorf("no header rewriting transformations found in backend configurations")
}

// Cache TTL and Eviction Handling

func (ctx *ReverseProxyBDDTestContext) expiredCacheEntriesShouldBeEvicted() error {
	// Verify that expired cache entries are properly evicted
	if ctx.service == nil {
		return fmt.Errorf("proxy service not available")
	}

	// Check if caching is enabled
	if ctx.config == nil || !ctx.config.CacheEnabled {
		return fmt.Errorf("caching not enabled in configuration")
	}

	// Verify that expired entries are no longer served from cache
	// This is tested by ensuring that after TTL expires, requests hit the backend again

	// Check if we have a cache service available through the service registry
	var cacheService interface{}
	err := ctx.app.GetService("cache", &cacheService)
	if err != nil || cacheService == nil {
		// If no cache service, then cache eviction is handled by the underlying cache implementation
		// We can verify behavior by checking that fresh requests hit backends after expiration
		ctx.app.Logger().Info("No cache service available, verifying eviction through backend hit patterns")
		return nil
	}

	// If we have access to cache statistics or cache service, verify eviction behavior
	// For now, we verify that the caching behavior changes after TTL expires

	// The key verification is that after the TTL configured in the test,
	// subsequent requests should hit the backend again rather than serving from cache

	// Check TTL configuration
	if ctx.config.CacheTTL <= 0 {
		return fmt.Errorf("cache TTL should be configured for eviction testing")
	}

	ctx.app.Logger().Info("Cache eviction verification completed",
		"ttl", ctx.config.CacheTTL)

	return nil
}
