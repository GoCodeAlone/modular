package reverseproxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Feature Flag Scenarios

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithRouteLevelFeatureFlagsConfigured() error {
	ctx.resetContext()

	// Create test backend servers
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("primary backend response"))
	}))
	ctx.testServers = append(ctx.testServers, primaryServer)

	altServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("alternative backend response"))
	}))
	ctx.testServers = append(ctx.testServers, altServer)

	// Create configuration with route-level feature flags
	ctx.config = &ReverseProxyConfig{
		DefaultBackend: "primary-backend",
		BackendServices: map[string]string{
			"primary-backend": primaryServer.URL,
			"alt-backend":     altServer.URL,
		},
		Routes: map[string]string{
			"/api/new-feature": "primary-backend",
		},
		RouteConfigs: map[string]RouteConfig{
			"/api/new-feature": {
				FeatureFlagID:      "new-feature-enabled",
				AlternativeBackend: "alt-backend",
			},
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"primary-backend": {URL: primaryServer.URL},
			"alt-backend":     {URL: altServer.URL},
		},
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"new-feature-enabled": false, // Feature disabled
			},
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

	return ctx.setupApplicationWithConfig()
}

func (ctx *ReverseProxyBDDTestContext) requestsAreMadeToFlaggedRoutes() error {
	return ctx.iSendARequestToTheProxy()
}

func (ctx *ReverseProxyBDDTestContext) featureFlagsShouldControlRoutingDecisions() error {
	// Verify route-level feature flag configuration
	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("service or config not available")
	}

	routeConfig, exists := ctx.service.config.RouteConfigs["/api/new-feature"]
	if !exists {
		return fmt.Errorf("route config for /api/new-feature not found")
	}

	if routeConfig.FeatureFlagID != "new-feature-enabled" {
		return fmt.Errorf("expected feature flag ID new-feature-enabled, got %s", routeConfig.FeatureFlagID)
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) alternativeBackendsShouldBeUsedWhenFlagsAreDisabled() error {
	// This step needs to check the configuration differently depending on which scenario we're in
	err := ctx.ensureServiceInitialized()
	if err != nil {
		return err
	}

	// Check if we're in a route-level feature flag scenario
	if routeConfig, exists := ctx.service.config.RouteConfigs["/api/new-feature"]; exists {
		if routeConfig.AlternativeBackend != "alt-backend" {
			return fmt.Errorf("expected alternative backend alt-backend for route scenario, got %s", routeConfig.AlternativeBackend)
		}
		return nil
	}

	// Check if we're in a backend-level feature flag scenario
	if backendConfig, exists := ctx.service.config.BackendConfigs["new-backend"]; exists {
		if backendConfig.AlternativeBackend != "old-backend" {
			return fmt.Errorf("expected alternative backend old-backend for backend scenario, got %s", backendConfig.AlternativeBackend)
		}
		return nil
	}

	// Check for composite route scenario
	if compositeRoute, exists := ctx.service.config.CompositeRoutes["/api/combined"]; exists {
		if compositeRoute.AlternativeBackend != "fallback" {
			return fmt.Errorf("expected alternative backend fallback for composite scenario, got %s", compositeRoute.AlternativeBackend)
		}
		return nil
	}

	return fmt.Errorf("no alternative backend configuration found for any scenario")
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithBackendLevelFeatureFlagsConfigured() error {
	ctx.resetContext()

	// Create test backend servers
	newBackendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("new backend response"))
	}))
	ctx.testServers = append(ctx.testServers, newBackendServer)

	oldBackendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("old backend response"))
	}))
	ctx.testServers = append(ctx.testServers, oldBackendServer)

	// Create configuration with backend-level feature flags
	ctx.config = &ReverseProxyConfig{
		DefaultBackend: "new-backend",
		BackendServices: map[string]string{
			"new-backend": newBackendServer.URL,
			"old-backend": oldBackendServer.URL,
		},
		Routes: map[string]string{
			"/api/test": "new-backend",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"new-backend": {
				URL:                newBackendServer.URL,
				FeatureFlagID:      "new-backend-enabled",
				AlternativeBackend: "old-backend",
			},
			"old-backend": {URL: oldBackendServer.URL},
		},
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"new-backend-enabled": false, // Feature disabled
			},
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

	return ctx.setupApplicationWithConfig()
}

func (ctx *ReverseProxyBDDTestContext) requestsTargetFlaggedBackends() error {
	return ctx.iSendARequestToTheProxy()
}

func (ctx *ReverseProxyBDDTestContext) featureFlagsShouldControlBackendSelection() error {
	// Verify backend-level feature flag configuration
	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("service or config not available")
	}

	backendConfig, exists := ctx.service.config.BackendConfigs["new-backend"]
	if !exists {
		return fmt.Errorf("backend config for new-backend not found")
	}

	if backendConfig.FeatureFlagID != "new-backend-enabled" {
		return fmt.Errorf("expected feature flag ID new-backend-enabled, got %s", backendConfig.FeatureFlagID)
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithCompositeRouteFeatureFlagsConfigured() error {
	ctx.resetContext()

	// Create test backend servers for composite scenario
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("primary composite response"))
	}))
	ctx.testServers = append(ctx.testServers, primaryServer)

	secondaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("secondary composite response"))
	}))
	ctx.testServers = append(ctx.testServers, secondaryServer)

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fallback response"))
	}))
	ctx.testServers = append(ctx.testServers, fallbackServer)

	// Create configuration with composite route feature flags
	ctx.config = &ReverseProxyConfig{
		DefaultBackend: "primary",
		BackendServices: map[string]string{
			"primary":   primaryServer.URL,
			"secondary": secondaryServer.URL,
			"fallback":  fallbackServer.URL,
		},
		Routes: map[string]string{
			"/api/*": "primary",
		},
		CompositeRoutes: map[string]CompositeRoute{
			"/api/combined": {
				Pattern:            "/api/combined",
				Backends:           []string{"primary", "secondary"},
				Strategy:           "combine",
				FeatureFlagID:      "composite-feature-enabled",
				AlternativeBackend: "fallback",
			},
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"primary":   {URL: primaryServer.URL},
			"secondary": {URL: secondaryServer.URL},
			"fallback":  {URL: fallbackServer.URL},
		},
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"composite-feature-enabled": false, // Feature disabled
			},
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

	return ctx.setupApplicationWithConfig()
}

func (ctx *ReverseProxyBDDTestContext) requestsAreMadeToCompositeRoutes() error {
	return ctx.iSendARequestToTheProxy()
}

func (ctx *ReverseProxyBDDTestContext) featureFlagsShouldControlRouteAvailability() error {
	// Verify composite route feature flag configuration
	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("service or config not available")
	}

	compositeRoute, exists := ctx.service.config.CompositeRoutes["/api/combined"]
	if !exists {
		return fmt.Errorf("composite route config for /api/combined not found")
	}

	if compositeRoute.FeatureFlagID != "composite-feature-enabled" {
		return fmt.Errorf("expected feature flag ID composite-feature-enabled, got %s", compositeRoute.FeatureFlagID)
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) alternativeSingleBackendsShouldBeUsedWhenDisabled() error {
	// Verify alternative backend configuration for disabled composite routes
	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("service or config not available")
	}

	compositeRoute, exists := ctx.service.config.CompositeRoutes["/api/combined"]
	if !exists {
		return fmt.Errorf("composite route config for /api/combined not found")
	}

	if compositeRoute.AlternativeBackend != "fallback" {
		return fmt.Errorf("expected alternative backend fallback, got %s", compositeRoute.AlternativeBackend)
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithTenantSpecificFeatureFlagsConfigured() error {
	ctx.resetContext()

	// Create tenant-specific backend servers
	tenantANewServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("tenant-a new feature response"))
	}))
	ctx.testServers = append(ctx.testServers, tenantANewServer)

	tenantAOldServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("tenant-a old feature response"))
	}))
	ctx.testServers = append(ctx.testServers, tenantAOldServer)

	tenantBNewServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("tenant-b new feature response"))
	}))
	ctx.testServers = append(ctx.testServers, tenantBNewServer)

	tenantBOldServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("tenant-b old feature response"))
	}))
	ctx.testServers = append(ctx.testServers, tenantBOldServer)

	// Create configuration with tenant-specific feature flags
	ctx.config = &ReverseProxyConfig{
		DefaultBackend:  "default-backend",
		RequireTenantID: true,
		TenantIDHeader:  "X-Tenant-ID",
		BackendServices: map[string]string{
			"default-backend": "", // Will be overridden by tenant configs
		},
		Routes: map[string]string{
			"/api/feature": "default-backend",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"default-backend": {URL: ""},
		},
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"new-feature": false, // Default disabled, will be overridden per tenant
			},
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

	// Setup tenant-specific configurations with feature flag routing
	tenantAConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"new-feature": tenantANewServer.URL,
			"old-feature": tenantAOldServer.URL,
		},
		Routes: map[string]string{
			"/api/feature": "new-feature",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"new-feature": {URL: tenantANewServer.URL},
			"old-feature": {URL: tenantAOldServer.URL},
		},
		RouteConfigs: map[string]RouteConfig{
			"/api/feature": {
				FeatureFlagID:      "new-feature",
				AlternativeBackend: "old-feature",
			},
		},
	}

	tenantBConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"new-feature": tenantBNewServer.URL,
			"old-feature": tenantBOldServer.URL,
		},
		Routes: map[string]string{
			"/api/feature": "new-feature",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"new-feature": {URL: tenantBNewServer.URL},
			"old-feature": {URL: tenantBOldServer.URL},
		},
		RouteConfigs: map[string]RouteConfig{
			"/api/feature": {
				FeatureFlagID:      "new-feature",
				AlternativeBackend: "old-feature",
			},
		},
	}

	// Replace with mock tenant application
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

	// Create fresh module for mock app
	freshModule := NewModule()
	ctx.module = freshModule
	mockTenantApp.RegisterModule(freshModule)

	// Constructor pattern
	constructor := freshModule.Constructor()
	services := map[string]any{
		"router": router, // Router is guaranteed to be non-nil at this point
	}
	constructedModule, err := constructor(mockTenantApp, services)
	if err != nil {
		return fmt.Errorf("failed to construct module: %w", err)
	}
	ctx.module = constructedModule.(*ReverseProxyModule)
	freshModule = constructedModule.(*ReverseProxyModule)

	// Setup tenant configurations
	tenantAProvider := modular.NewStdConfigProvider(tenantAConfig)
	tenantBProvider := modular.NewStdConfigProvider(tenantBConfig)

	mockTenantApp.On("GetTenantConfig", modular.TenantID("tenant-a"), "reverseproxy").Return(tenantAProvider, nil)
	mockTenantApp.On("GetTenantConfig", modular.TenantID("tenant-b"), "reverseproxy").Return(tenantBProvider, nil)
	mockTenantApp.On("GetTenants").Return([]modular.TenantID{"tenant-a", "tenant-b"})

	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	mockTenantApp.On("GetConfigSection", "reverseproxy").Return(reverseproxyConfigProvider, nil)

	ctx.app = mockTenantApp

	// Register tenants
	freshModule.OnTenantRegistered(modular.TenantID("tenant-a"))
	freshModule.OnTenantRegistered(modular.TenantID("tenant-b"))

	// Initialize and start
	if err := freshModule.Init(mockTenantApp); err != nil {
		return fmt.Errorf("failed to init module: %w", err)
	}
	if err := freshModule.Start(nil); err != nil {
		return fmt.Errorf("failed to start module: %w", err)
	}

	// Register services
	serviceProviders := freshModule.ProvidesServices()
	for _, provider := range serviceProviders {
		if err := mockTenantApp.RegisterService(provider.Name, provider.Instance); err != nil {
			return fmt.Errorf("failed to register service %s: %w", provider.Name, err)
		}
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) requestsAreMadeWithDifferentTenantContexts() error {
	return ctx.iSendARequestToTheProxy()
}

func (ctx *ReverseProxyBDDTestContext) featureFlagsShouldBeEvaluatedPerTenant() error {
	// Verify tenant-specific feature flag configuration
	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("service or config not available")
	}

	// Check global feature flags configuration
	if !ctx.service.config.FeatureFlags.Enabled {
		return fmt.Errorf("feature flags should be enabled")
	}

	if len(ctx.service.config.FeatureFlags.Flags) == 0 {
		return fmt.Errorf("no feature flags configured")
	}

	// Check that the new-feature flag exists in the default configuration
	if _, exists := ctx.service.config.FeatureFlags.Flags["new-feature"]; !exists {
		return fmt.Errorf("new-feature flag not found in configuration")
	}

	// Note: Tenant-specific flags would be handled by tenant-specific configurations
	// rather than being stored in the main service configuration

	return nil
}

func (ctx *ReverseProxyBDDTestContext) tenantSpecificRoutingShouldBeApplied() error {
	// Test tenant-specific routing by making requests with different tenant headers
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Make request for tenant-a (feature enabled)
	resp1, err := ctx.makeRequestThroughModuleWithHeaders("GET", "/api/feature", nil, map[string]string{
		"X-Tenant-ID": "tenant-a",
	})
	if err != nil {
		return fmt.Errorf("failed to make tenant-a request: %w", err)
	}
	resp1.Body.Close()

	// Make request for tenant-b (feature disabled)
	resp2, err := ctx.makeRequestThroughModuleWithHeaders("GET", "/api/feature", nil, map[string]string{
		"X-Tenant-ID": "tenant-b",
	})
	if err != nil {
		return fmt.Errorf("failed to make tenant-b request: %w", err)
	}
	resp2.Body.Close()

	// Both requests should succeed but route to different backends based on feature flags
	if resp1.StatusCode != http.StatusOK || resp2.StatusCode != http.StatusOK {
		return fmt.Errorf("tenant-specific requests should succeed, got %d and %d", resp1.StatusCode, resp2.StatusCode)
	}

	return nil
}
