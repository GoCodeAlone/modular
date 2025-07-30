package reverseproxy

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/CrisisTextLine/modular"
)

func TestBasicRouteConfigsFeatureFlagRouting(t *testing.T) {
	t.Run("FeatureFlagDisabled_UsesAlternativeBackend", func(t *testing.T) {
		testRouteConfigWithFlag(t, false, "alternative-backend-response")
	})

	t.Run("FeatureFlagEnabled_UsesPrimaryBackend", func(t *testing.T) {
		testRouteConfigWithFlag(t, true, "primary-backend-response")
	})
}

func testRouteConfigWithFlag(t *testing.T, flagEnabled bool, expectedResponse string) {
	// Create mock backends
	primaryBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("primary-backend-response"))
	}))
	defer primaryBackend.Close()

	alternativeBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("alternative-backend-response"))
	}))
	defer alternativeBackend.Close()

	// Create mock router
	mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}

	// Create feature flag evaluator
	app := NewMockTenantApplication()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Register configuration for the feature flag evaluator
	flagConfig := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"avatar-api": flagEnabled,
			},
		},
	}
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(flagConfig))

	// Register tenant service for proper configuration management
	tenantService := modular.NewStandardTenantService(logger)
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	featureFlagEvaluator, err := NewFileBasedFeatureFlagEvaluator(app, logger)
	if err != nil {
		t.Fatalf("Failed to create feature flag evaluator: %v", err)
	}

	// Create reverse proxy module
	module := NewModule()

	// Register config first (this sets the app reference)
	if err := module.RegisterConfig(app); err != nil {
		t.Fatalf("Failed to register config: %v", err)
	}

	// Configure the module
	config := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"chimera": primaryBackend.URL,
			"default": alternativeBackend.URL,
		},
		Routes: map[string]string{
			"/api/v1/avatar/*": "chimera",
		},
		RouteConfigs: map[string]RouteConfig{
			"/api/v1/avatar/*": {
				FeatureFlagID:      "avatar-api",
				AlternativeBackend: "default",
			},
		},
		DefaultBackend:  "default",
		TenantIDHeader:  "X-Affiliate-Id",
		RequireTenantID: false,
	}

	// Replace config with our configured one (keep feature flags separate)
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

	// Initialize with services
	services := map[string]any{
		"router":               mockRouter,
		"featureFlagEvaluator": featureFlagEvaluator,
	}

	constructedModule, err := module.Constructor()(app, services)
	if err != nil {
		t.Fatalf("Failed to construct module: %v", err)
	}

	reverseProxyModule := constructedModule.(*ReverseProxyModule)

	// Initialize the module
	if err := reverseProxyModule.Init(app); err != nil {
		t.Fatalf("Failed to initialize module: %v", err)
	}

	// Start the module
	if err := reverseProxyModule.Start(app.Context()); err != nil {
		t.Fatalf("Failed to start module: %v", err)
	}

	// Test the route behavior
	handler := mockRouter.routes["/api/v1/avatar/*"]
	if handler == nil {
		t.Fatal("Handler not registered for /api/v1/avatar/*")
	}

	req := httptest.NewRequest("POST", "/api/v1/avatar/upload", nil)
	recorder := httptest.NewRecorder()

	handler(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	body := recorder.Body.String()
	if body != expectedResponse {
		t.Errorf("Expected '%s', got '%s'", expectedResponse, body)
	}
}
func TestRouteConfigsWithTenantSpecificFlags(t *testing.T) {
	// Create mock backends
	primaryBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("primary-backend-response"))
	}))
	defer primaryBackend.Close()

	alternativeBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("alternative-backend-response"))
	}))
	defer alternativeBackend.Close()

	// Create mock router
	mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}

	// Create feature flag evaluator with tenant-specific flags
	app := NewMockTenantApplication()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Register tenant service
	tenantService := modular.NewStandardTenantService(logger)
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	// Register global configuration with default flags
	globalConfig := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"avatar-api": true, // Global default is true
			},
		},
	}
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(globalConfig))

	// Register tenant "ctl" with overridden flag
	tenantConfig := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"avatar-api": false, // Tenant-specific override to false
			},
		},
	}
	if err := tenantService.RegisterTenant("ctl", map[string]modular.ConfigProvider{
		"reverseproxy": modular.NewStdConfigProvider(tenantConfig),
	}); err != nil {
		t.Fatalf("Failed to register tenant: %v", err)
	}

	featureFlagEvaluator, err := NewFileBasedFeatureFlagEvaluator(app, logger)
	if err != nil {
		t.Fatalf("Failed to create feature flag evaluator: %v", err)
	}

	// Create mock application (needs to be TenantApplication) - already created above

	// Create reverse proxy module and register config
	module := NewModule()
	if err := module.RegisterConfig(app); err != nil {
		t.Fatalf("Failed to register config: %v", err)
	}

	// Configure the module
	config := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"chimera": primaryBackend.URL,
			"default": alternativeBackend.URL,
		},
		Routes: map[string]string{
			"/api/v1/avatar/*": "chimera",
		},
		RouteConfigs: map[string]RouteConfig{
			"/api/v1/avatar/*": {
				FeatureFlagID:      "avatar-api",
				AlternativeBackend: "default",
			},
		},
		DefaultBackend:  "default",
		TenantIDHeader:  "X-Affiliate-Id",
		RequireTenantID: false,
	}

	// Replace config with our configured one
	app.RegisterConfigSection("reverseproxy", NewStdConfigProvider(config))

	// Initialize with services
	services := map[string]any{
		"router":               mockRouter,
		"featureFlagEvaluator": featureFlagEvaluator,
	}

	constructedModule, err := module.Constructor()(app, services)
	if err != nil {
		t.Fatalf("Failed to construct module: %v", err)
	}

	reverseProxyModule := constructedModule.(*ReverseProxyModule)

	// Initialize the module
	if err := reverseProxyModule.Init(app); err != nil {
		t.Fatalf("Failed to initialize module: %v", err)
	}

	// Start the module
	if err := reverseProxyModule.Start(app.Context()); err != nil {
		t.Fatalf("Failed to start module: %v", err)
	}

	t.Run("RequestWithoutTenantID_UsesGlobalFlag", func(t *testing.T) {
		// No tenant ID, should use global flag (true) -> primary backend
		handler := mockRouter.routes["/api/v1/avatar/*"]
		if handler == nil {
			t.Fatal("Handler not registered for /api/v1/avatar/*")
		}

		req := httptest.NewRequest("POST", "/api/v1/avatar/upload", nil)
		recorder := httptest.NewRecorder()

		handler(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		body := recorder.Body.String()
		if body != "primary-backend-response" {
			t.Errorf("Expected 'primary-backend-response', got '%s'", body)
		}
	})

	t.Run("RequestWithTenantID_UsesTenantSpecificFlag", func(t *testing.T) {
		// Tenant ID "ctl" has flag set to false -> alternative backend
		handler := mockRouter.routes["/api/v1/avatar/*"]
		if handler == nil {
			t.Fatal("Handler not registered for /api/v1/avatar/*")
		}

		req := httptest.NewRequest("POST", "/api/v1/avatar/upload", nil)
		req.Header.Set("X-Affiliate-Id", "ctl")
		recorder := httptest.NewRecorder()

		handler(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		body := recorder.Body.String()
		if body != "alternative-backend-response" {
			t.Errorf("Expected 'alternative-backend-response', got '%s'", body)
		}
	})
}
