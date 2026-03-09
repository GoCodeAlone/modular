package reverseproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoCodeAlone/modular"
)

// TestTenantHeaderEnforcementBDD runs BDD scenarios for tenant header enforcement
func TestTenantHeaderEnforcementBDD(t *testing.T) {
	ctx := &ReverseProxyBDDTestContext{}

	// Run the tenant header enforcement scenarios step by step
	t.Run("Setup reverse proxy with require tenant ID enabled", func(t *testing.T) {
		err := ctx.iHaveAReverseProxyWithRequireTenantIDEnabled()
		if err != nil {
			t.Fatalf("Failed to setup reverse proxy: %v", err)
		}
	})

	t.Run("Requests without tenant header should receive HTTP 400", func(t *testing.T) {
		err := ctx.requestsWithoutTenantHeaderShouldReceive400()
		if err != nil {
			t.Errorf("Tenant header enforcement failed: %v", err)
		}
	})

	t.Run("Requests with valid tenant ID should route correctly", func(t *testing.T) {
		err := ctx.iSendRequestsWithValidTenantIDs()
		if err != nil {
			t.Errorf("Failed to send requests with valid tenant IDs: %v", err)
		}

		err = ctx.requestsWithValidTenantIDShouldRouteCorrectly()
		if err != nil {
			t.Errorf("Valid tenant ID routing failed: %v", err)
		}
	})

	t.Run("Tenant header enforcement should be consistent", func(t *testing.T) {
		err := ctx.tenantHeaderEnforcementShouldBeConsistentAcrossAllRouteTypes()
		if err != nil {
			t.Errorf("Tenant header enforcement consistency failed: %v", err)
		}
	})

	t.Run("Tenant isolation should be maintained", func(t *testing.T) {
		err := ctx.tenantIsolationShouldBeMaintainedForAllRoutingScenarios()
		if err != nil {
			t.Errorf("Tenant isolation failed: %v", err)
		}
	})

	// Cleanup
	ctx.resetContext()
}

// Setup step: Configure reverse proxy with require_tenant_id enabled
func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithRequireTenantIDEnabled() error {
	ctx.resetContext()

	app, err := modular.NewApplication(modular.WithLogger(&testLogger{}))
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}
	ctx.app = app

	// Create tenant-specific backend servers for validation
	tenantAServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-ID", "tenant-a-backend")
		w.Header().Set("X-Tenant-ID", r.Header.Get("X-Tenant-ID"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("tenant-a backend response"))
	}))

	tenantBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-ID", "tenant-b-backend")
		w.Header().Set("X-Tenant-ID", r.Header.Get("X-Tenant-ID"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("tenant-b backend response"))
	}))

	defaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-ID", "default-backend")
		w.Header().Set("X-Tenant-ID", r.Header.Get("X-Tenant-ID"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("default backend response"))
	}))

	compositeServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-ID", "composite-backend-1")
		w.Header().Set("X-Tenant-ID", r.Header.Get("X-Tenant-ID"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("composite backend 1 response"))
	}))

	compositeServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-ID", "composite-backend-2")
		w.Header().Set("X-Tenant-ID", r.Header.Get("X-Tenant-ID"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("composite backend 2 response"))
	}))

	// Store servers for cleanup
	ctx.testServers = append(ctx.testServers, tenantAServer, tenantBServer, defaultServer, compositeServer1, compositeServer2)

	// Configure global reverse proxy with tenant ID requirement enabled
	ctx.config = &ReverseProxyConfig{
		RequireTenantID: true,
		TenantIDHeader:  "X-Tenant-ID",
		BackendServices: map[string]string{
			"default-backend": defaultServer.URL,
		},
		Routes: map[string]string{
			"/api/explicit":   "default-backend",
			"/api/another":    "default-backend",
			"/explicit/route": "default-backend",
		},
		DefaultBackend: "default-backend",
		BackendConfigs: map[string]BackendServiceConfig{
			"default-backend": {URL: defaultServer.URL},
		},
	}

	// Replace the standard app with a mock tenant application for tenant-aware routing
	mockTenantApp := NewMockTenantApplicationWithMock()

	// Register services
	if err := mockTenantApp.RegisterService("logger", &testLogger{}); err != nil {
		return fmt.Errorf("failed to register logger: %w", err)
	}
	if err := mockTenantApp.RegisterService("router", &testRouter{routes: make(map[string]http.HandlerFunc)}); err != nil {
		return fmt.Errorf("failed to register router: %w", err)
	}
	if err := mockTenantApp.RegisterService("metrics", &testMetrics{}); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	// Create event observer
	ctx.eventObserver = newTestEventObserver()
	if err := mockTenantApp.RegisterService("event-bus", &testEventBus{observers: []modular.Observer{ctx.eventObserver}}); err != nil {
		return fmt.Errorf("failed to register event bus: %w", err)
	}

	// Configure tenant-specific routing
	tenantAConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"tenant-a-backend": tenantAServer.URL,
			"default-backend":  tenantAServer.URL, // Override default backend for unmapped routes
		},
		Routes: map[string]string{
			"/api/explicit": "tenant-a-backend",
		},
	}

	tenantBConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"tenant-b-backend": tenantBServer.URL,
			"default-backend":  tenantBServer.URL, // Override default backend for unmapped routes
		},
		Routes: map[string]string{
			"/api/explicit": "tenant-b-backend",
			"/api/another":  "tenant-b-backend",
		},
	}

	// Set up tenant configurations
	tenantAProvider := modular.NewStdConfigProvider(tenantAConfig)
	tenantBProvider := modular.NewStdConfigProvider(tenantBConfig)

	mockTenantApp.On("GetTenantConfig", modular.TenantID("tenant-a"), "reverseproxy").Return(tenantAProvider, nil)
	mockTenantApp.On("GetTenantConfig", modular.TenantID("tenant-b"), "reverseproxy").Return(tenantBProvider, nil)
	mockTenantApp.On("GetTenants").Return([]modular.TenantID{"tenant-a", "tenant-b"})

	// Mock the global config
	globalConfigProvider := modular.NewStdConfigProvider(ctx.config)
	mockTenantApp.On("GetConfigSection", "reverseproxy").Return(globalConfigProvider, nil)

	// Replace the app in context
	ctx.app = mockTenantApp

	// Create and register module
	module := NewModule()
	mockTenantApp.RegisterModule(module)

	// Get router service for constructor
	var router *testRouter
	if err := mockTenantApp.GetService("router", &router); err != nil {
		return fmt.Errorf("failed to get router service: %w", err)
	}

	// Use constructor to create module instance
	constructor := module.Constructor()
	services := map[string]any{"router": router}
	constructedModule, err := constructor(mockTenantApp, services)
	if err != nil {
		return fmt.Errorf("failed to construct module: %w", err)
	}

	ctx.module = constructedModule.(*ReverseProxyModule)

	// Register tenants with the module
	ctx.module.OnTenantRegistered(modular.TenantID("tenant-a"))
	ctx.module.OnTenantRegistered(modular.TenantID("tenant-b"))

	// Initialize and start the module
	if err := ctx.module.Init(mockTenantApp); err != nil {
		return fmt.Errorf("failed to initialize module: %w", err)
	}
	if err := ctx.module.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start module: %w", err)
	}

	// Manually register services
	serviceProviders := ctx.module.ProvidesServices()
	for _, provider := range serviceProviders {
		if err := mockTenantApp.RegisterService(provider.Name, provider.Instance); err != nil {
			return fmt.Errorf("failed to register service %s: %w", provider.Name, err)
		}
	}

	return nil
}

// Test step: Send requests to explicit routes without tenant header
func (ctx *ReverseProxyBDDTestContext) iSendRequestsToExplicitRoutesWithoutTenantHeader() error {
	if err := ctx.ensureServiceInitialized(); err != nil {
		return err
	}

	// Test explicit routes without tenant header
	explicitRoutes := []string{"/api/explicit", "/api/another", "/explicit/route"}

	for _, route := range explicitRoutes {
		resp, err := ctx.makeRequestThroughModule("GET", route, nil)
		if err != nil {
			return fmt.Errorf("failed to make request to explicit route %s: %w", route, err)
		}

		// Store response for validation - we expect 400
		ctx.lastResponse = resp
		if resp.Body != nil {
			body, _ := io.ReadAll(resp.Body)
			ctx.lastResponseBody = body
			resp.Body.Close()
		}
	}

	return nil
}

// Test step: Send requests to composite routes without tenant header
func (ctx *ReverseProxyBDDTestContext) iSendRequestsToCompositeRoutesWithoutTenantHeader() error {
	if err := ctx.ensureServiceInitialized(); err != nil {
		return err
	}

	// Test composite routes without tenant header
	compositeRoutes := []string{"/api/composite", "/composite/multi"}

	for _, route := range compositeRoutes {
		resp, err := ctx.makeRequestThroughModule("GET", route, nil)
		if err != nil {
			return fmt.Errorf("failed to make request to composite route %s: %w", route, err)
		}

		// Store response for validation - we expect 400
		ctx.lastResponse = resp
		if resp.Body != nil {
			body, _ := io.ReadAll(resp.Body)
			ctx.lastResponseBody = body
			resp.Body.Close()
		}
	}

	return nil
}

// Test step: Send requests to default backend without tenant header
func (ctx *ReverseProxyBDDTestContext) iSendRequestsToDefaultBackendWithoutTenantHeader() error {
	if err := ctx.ensureServiceInitialized(); err != nil {
		return err
	}

	// Test requests that fall through to default backend without tenant header
	defaultBackendRoutes := []string{"/unmapped/path", "/some/other/route", "/fallback"}

	for _, route := range defaultBackendRoutes {
		resp, err := ctx.makeRequestThroughModule("GET", route, nil)
		if err != nil {
			return fmt.Errorf("failed to make request to default backend route %s: %w", route, err)
		}

		// Store response for validation - we expect 400
		ctx.lastResponse = resp
		if resp.Body != nil {
			body, _ := io.ReadAll(resp.Body)
			ctx.lastResponseBody = body
			resp.Body.Close()
		}
	}

	return nil
}

// Test step: Send requests with valid tenant IDs
func (ctx *ReverseProxyBDDTestContext) iSendRequestsWithValidTenantIDs() error {
	if err := ctx.ensureServiceInitialized(); err != nil {
		return err
	}

	// Test various route types with valid tenant headers
	testCases := []struct {
		route  string
		tenant string
		desc   string
	}{
		{"/api/explicit", "tenant-a", "explicit route with tenant A"},
		{"/api/another", "tenant-b", "explicit route with tenant B"},
		{"/explicit/route", "tenant-a", "explicit route to default backend"},
		{"/api/composite", "tenant-a", "composite route with tenant A"},
		{"/composite/multi", "tenant-b", "composite route with tenant B"},
		{"/unmapped/path", "tenant-a", "default backend route with tenant A"},
		{"/fallback", "tenant-b", "default backend route with tenant B"},
	}

	for _, tc := range testCases {
		resp, err := ctx.makeRequestThroughModuleWithHeaders("GET", tc.route, nil, map[string]string{
			"X-Tenant-ID": tc.tenant,
		})
		if err != nil {
			return fmt.Errorf("failed to make request with valid tenant ID for %s: %w", tc.desc, err)
		}

		// Store response for validation - we expect 200
		ctx.lastResponse = resp
		if resp.Body != nil {
			body, _ := io.ReadAll(resp.Body)
			ctx.lastResponseBody = body
			resp.Body.Close()
		}

		// Validate successful response
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("request with valid tenant ID should succeed for %s, got status %d", tc.desc, resp.StatusCode)
		}
	}

	return nil
}

// Validation step: Verify requests without tenant header receive HTTP 400
func (ctx *ReverseProxyBDDTestContext) requestsWithoutTenantHeaderShouldReceive400() error {
	if err := ctx.ensureServiceInitialized(); err != nil {
		return err
	}

	// Test all route types to ensure they enforce tenant header requirement
	testRoutes := []struct {
		route string
		desc  string
	}{
		{"/api/explicit", "explicit route"},
		{"/api/another", "another explicit route"},
		{"/explicit/route", "explicit route to default backend"},
		{"/api/composite", "composite route"},
		{"/composite/multi", "multi-backend composite route"},
		{"/unmapped/path", "unmapped path (default backend)"},
		{"/fallback", "fallback route (default backend)"},
	}

	for _, tr := range testRoutes {
		resp, err := ctx.makeRequestThroughModule("GET", tr.route, nil)
		if err != nil {
			return fmt.Errorf("failed to make request to %s: %w", tr.desc, err)
		}

		if resp.Body != nil {
			resp.Body.Close()
		}

		// Verify HTTP 400 response for missing tenant header
		if resp.StatusCode != http.StatusBadRequest {
			return fmt.Errorf("request to %s without tenant header should return 400, got %d", tr.desc, resp.StatusCode)
		}
	}

	return nil
}

// Validation step: Verify requests with valid tenant ID route correctly
func (ctx *ReverseProxyBDDTestContext) requestsWithValidTenantIDShouldRouteCorrectly() error {
	if err := ctx.ensureServiceInitialized(); err != nil {
		return err
	}

	// Test routing with valid tenant IDs to different backend types
	testCases := []struct {
		route             string
		tenant            string
		expectedBackendID string
		desc              string
	}{
		{"/api/explicit", "tenant-a", "tenant-a-backend", "explicit route to tenant A backend"},
		{"/api/another", "tenant-b", "tenant-b-backend", "explicit route to tenant B backend"},
		{"/explicit/route", "tenant-a", "tenant-a-backend", "explicit route uses tenant A's default backend override"},
		{"/unmapped/path", "tenant-b", "tenant-b-backend", "unmapped route uses tenant B's default backend override"},
	}

	for _, tc := range testCases {
		resp, err := ctx.makeRequestThroughModuleWithHeaders("GET", tc.route, nil, map[string]string{
			"X-Tenant-ID": tc.tenant,
		})
		if err != nil {
			return fmt.Errorf("failed to make request for %s: %w", tc.desc, err)
		}
		defer resp.Body.Close()

		// Verify successful routing
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("request for %s should succeed, got status %d", tc.desc, resp.StatusCode)
		}

		// Verify correct backend was hit
		backendID := resp.Header.Get("X-Backend-ID")
		if backendID != tc.expectedBackendID {
			return fmt.Errorf("request for %s should hit %s backend, but hit %s", tc.desc, tc.expectedBackendID, backendID)
		}

		// Verify tenant ID was properly forwarded
		returnedTenantID := resp.Header.Get("X-Tenant-ID")
		if returnedTenantID != tc.tenant {
			return fmt.Errorf("request for %s should preserve tenant ID %s, but got %s", tc.desc, tc.tenant, returnedTenantID)
		}
	}

	// Test composite routes separately since they combine responses
	compositeTestCases := []struct {
		route  string
		tenant string
		desc   string
	}{
		{"/api/composite", "tenant-a", "composite route with tenant A"},
		{"/composite/multi", "tenant-b", "multi-backend composite route with tenant B"},
	}

	for _, tc := range compositeTestCases {
		resp, err := ctx.makeRequestThroughModuleWithHeaders("GET", tc.route, nil, map[string]string{
			"X-Tenant-ID": tc.tenant,
		})
		if err != nil {
			return fmt.Errorf("failed to make composite request for %s: %w", tc.desc, err)
		}
		defer resp.Body.Close()

		// Verify composite request succeeds
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("composite request for %s should succeed, got status %d", tc.desc, resp.StatusCode)
		}

		// Read response body to verify it contains data
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read composite response for %s: %w", tc.desc, err)
		}

		if len(body) == 0 {
			return fmt.Errorf("composite response for %s should contain data", tc.desc)
		}
	}

	return nil
}

// Validation step: Ensure tenant header enforcement is consistent across all route types
func (ctx *ReverseProxyBDDTestContext) tenantHeaderEnforcementShouldBeConsistentAcrossAllRouteTypes() error {
	if err := ctx.ensureServiceInitialized(); err != nil {
		return err
	}

	// Verify the service has tenant ID requirement enabled
	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("service configuration not available")
	}

	if !ctx.service.config.RequireTenantID {
		return fmt.Errorf("require tenant ID should be enabled in configuration")
	}

	if ctx.service.config.TenantIDHeader != "X-Tenant-ID" {
		return fmt.Errorf("tenant ID header should be X-Tenant-ID, got %s", ctx.service.config.TenantIDHeader)
	}

	// Test consistency: All routes should behave the same way regarding tenant header enforcement
	routes := []string{
		"/api/explicit",      // explicit route
		"/api/composite",     // composite route
		"/unmapped/fallback", // default backend route
	}

	for _, route := range routes {
		// Test without tenant header - should get 400
		resp, err := ctx.makeRequestThroughModule("GET", route, nil)
		if err != nil {
			return fmt.Errorf("failed to test %s without tenant header: %w", route, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			return fmt.Errorf("route %s should return 400 without tenant header, got %d", route, resp.StatusCode)
		}

		// Test with valid tenant header - should succeed (200) or handle appropriately
		respWithTenant, err := ctx.makeRequestThroughModuleWithHeaders("GET", route, nil, map[string]string{
			"X-Tenant-ID": "tenant-a",
		})
		if err != nil {
			return fmt.Errorf("failed to test %s with tenant header: %w", route, err)
		}
		respWithTenant.Body.Close()

		// All routes with valid tenant header should not return 400
		if respWithTenant.StatusCode == http.StatusBadRequest {
			return fmt.Errorf("route %s should not return 400 with valid tenant header", route)
		}
	}

	return nil
}

// Validation step: Verify tenant isolation is maintained for all routing scenarios
func (ctx *ReverseProxyBDDTestContext) tenantIsolationShouldBeMaintainedForAllRoutingScenarios() error {
	if err := ctx.ensureServiceInitialized(); err != nil {
		return err
	}

	// Test tenant isolation across different route types and tenants
	testScenarios := []struct {
		route   string
		tenantA string
		tenantB string
		desc    string
	}{
		{"/api/explicit", "tenant-a", "tenant-b", "explicit route isolation"},
		{"/api/another", "tenant-a", "tenant-b", "another explicit route isolation"},
		{"/unmapped/isolated", "tenant-a", "tenant-b", "default backend isolation"},
	}

	for _, scenario := range testScenarios {
		// Make request with tenant A
		respA, err := ctx.makeRequestThroughModuleWithHeaders("GET", scenario.route, nil, map[string]string{
			"X-Tenant-ID": scenario.tenantA,
		})
		if err != nil {
			return fmt.Errorf("failed to make tenant A request for %s: %w", scenario.desc, err)
		}

		bodyA, err := io.ReadAll(respA.Body)
		respA.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read tenant A response for %s: %w", scenario.desc, err)
		}

		// Make request with tenant B
		respB, err := ctx.makeRequestThroughModuleWithHeaders("GET", scenario.route, nil, map[string]string{
			"X-Tenant-ID": scenario.tenantB,
		})
		if err != nil {
			return fmt.Errorf("failed to make tenant B request for %s: %w", scenario.desc, err)
		}

		bodyB, err := io.ReadAll(respB.Body)
		respB.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read tenant B response for %s: %w", scenario.desc, err)
		}

		// Verify both requests succeed
		if respA.StatusCode != http.StatusOK || respB.StatusCode != http.StatusOK {
			return fmt.Errorf("both tenant requests should succeed for %s, got %d and %d", scenario.desc, respA.StatusCode, respB.StatusCode)
		}

		// Verify tenant isolation - responses should indicate proper tenant handling
		tenantAFromResp := respA.Header.Get("X-Tenant-ID")
		tenantBFromResp := respB.Header.Get("X-Tenant-ID")

		if tenantAFromResp != scenario.tenantA {
			return fmt.Errorf("tenant A response for %s should preserve tenant ID %s, got %s", scenario.desc, scenario.tenantA, tenantAFromResp)
		}

		if tenantBFromResp != scenario.tenantB {
			return fmt.Errorf("tenant B response for %s should preserve tenant ID %s, got %s", scenario.desc, scenario.tenantB, tenantBFromResp)
		}

		// Verify responses are different (indicating proper tenant-specific handling)
		if string(bodyA) == string(bodyB) && len(bodyA) > 0 {
			return fmt.Errorf("tenant responses for %s should be different to indicate proper isolation, both returned: %s", scenario.desc, string(bodyA))
		}
	}

	// Test composite routes tenant isolation
	compositeScenarios := []string{"/api/composite", "/composite/multi"}

	for _, route := range compositeScenarios {
		// Test with different tenants
		respA, err := ctx.makeRequestThroughModuleWithHeaders("GET", route, nil, map[string]string{
			"X-Tenant-ID": "tenant-a",
		})
		if err != nil {
			return fmt.Errorf("failed to make tenant A composite request to %s: %w", route, err)
		}
		respA.Body.Close()

		respB, err := ctx.makeRequestThroughModuleWithHeaders("GET", route, nil, map[string]string{
			"X-Tenant-ID": "tenant-b",
		})
		if err != nil {
			return fmt.Errorf("failed to make tenant B composite request to %s: %w", route, err)
		}
		respB.Body.Close()

		// Both composite requests should succeed with proper tenant isolation
		if respA.StatusCode != http.StatusOK || respB.StatusCode != http.StatusOK {
			return fmt.Errorf("composite requests for %s should succeed for both tenants, got %d and %d", route, respA.StatusCode, respB.StatusCode)
		}
	}

	return nil
}
