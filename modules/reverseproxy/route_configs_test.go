package reverseproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicRouteConfigsFeatureFlagRouting(t *testing.T) {
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
	featureFlagEvaluator := NewFileBasedFeatureFlagEvaluator()

	// Create mock application (needs to be TenantApplication)
	app := NewMockTenantApplication()

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

	t.Run("FeatureFlagDisabled_UsesAlternativeBackend", func(t *testing.T) {
		// Set feature flag to false
		featureFlagEvaluator.SetFlag("avatar-api", false)

		// Start the module
		if err := reverseProxyModule.Start(app.Context()); err != nil {
			t.Fatalf("Failed to start module: %v", err)
		}

		// Feature flag is disabled, should route to alternative backend
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
		if body != "alternative-backend-response" {
			t.Errorf("Expected 'alternative-backend-response', got '%s'", body)
		}
	})

	t.Run("FeatureFlagEnabled_UsesPrimaryBackend", func(t *testing.T) {
		// Enable feature flag
		featureFlagEvaluator.SetFlag("avatar-api", true)

		// Feature flag is enabled, should route to primary backend
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
	featureFlagEvaluator := NewFileBasedFeatureFlagEvaluator()
	featureFlagEvaluator.SetFlag("avatar-api", true)               // Global flag is true
	featureFlagEvaluator.SetTenantFlag("ctl", "avatar-api", false) // Tenant-specific flag is false

	// Create mock application (needs to be TenantApplication)
	app := NewMockTenantApplication()

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
