package reverseproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CrisisTextLine/modular"
)

// TestHealthEndpointNotProxied tests that health endpoints are not proxied to backends
func TestHealthEndpointNotProxied(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		config         *ReverseProxyConfig
		expectNotFound bool
		expectProxied  bool
		description    string
	}{
		{
			name: "HealthEndpointNotProxied",
			path: "/health",
			config: &ReverseProxyConfig{
				BackendServices: map[string]string{
					"test": "http://test:8080",
				},
				DefaultBackend: "test",
			},
			expectNotFound: true,
			expectProxied:  false,
			description:    "Health endpoint should not be proxied to backend",
		},
		{
			name: "MetricsEndpointNotProxied",
			path: "/metrics/reverseproxy",
			config: &ReverseProxyConfig{
				BackendServices: map[string]string{
					"test": "http://test:8080",
				},
				DefaultBackend:  "test",
				MetricsEndpoint: "/metrics/reverseproxy",
			},
			expectNotFound: true,
			expectProxied:  false,
			description:    "Metrics endpoint should not be proxied to backend",
		},
		{
			name: "MetricsHealthEndpointNotProxied",
			path: "/metrics/reverseproxy/health",
			config: &ReverseProxyConfig{
				BackendServices: map[string]string{
					"test": "http://test:8080",
				},
				DefaultBackend:  "test",
				MetricsEndpoint: "/metrics/reverseproxy",
			},
			expectNotFound: true,
			expectProxied:  false,
			description:    "Metrics health endpoint should not be proxied to backend",
		},
		{
			name: "DebugEndpointNotProxied",
			path: "/debug/info",
			config: &ReverseProxyConfig{
				BackendServices: map[string]string{
					"test": "http://test:8080",
				},
				DefaultBackend: "test",
			},
			expectNotFound: true,
			expectProxied:  false,
			description:    "Debug endpoint should not be proxied to backend",
		},
		{
			name: "RegularPathIsProxied",
			path: "/api/test",
			config: &ReverseProxyConfig{
				BackendServices: map[string]string{
					"test": "http://test:8080",
				},
				DefaultBackend: "test",
			},
			expectNotFound: false,
			expectProxied:  true,
			description:    "Regular API path should be proxied to backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock router
			mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}

			// Create mock application
			app := NewMockTenantApplication()

			// Register the configuration with the application
			app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(tt.config))

			// Create module
			module := NewModule()

			// Set router via constructor
			services := map[string]any{
				"router": mockRouter,
			}
			constructedModule, err := module.Constructor()(app, services)
			if err != nil {
				t.Fatalf("Failed to construct module: %v", err)
			}
			module = constructedModule.(*ReverseProxyModule)

			// Set the app reference
			module.app = app

			// Initialize the module (this loads config and creates backend proxies)
			if err := module.Init(app); err != nil {
				t.Fatalf("Failed to initialize module: %v", err)
			}

			// Start the module to register routes
			if err := module.Start(context.Background()); err != nil {
				t.Fatalf("Failed to start module: %v", err)
			}

			// Debug: Check if backend proxies were created
			t.Logf("Backend proxies created:")
			for backendID, proxy := range module.backendProxies {
				t.Logf("  - %s: %v", backendID, proxy != nil)
			}

			// Debug: Check default backend
			t.Logf("Default backend: %s", module.defaultBackend)

			// Test the path handling
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			// Debug: Print all registered routes
			t.Logf("Registered routes:")
			for pattern := range mockRouter.routes {
				t.Logf("  - %s", pattern)
			}

			// Find the handler for the catch-all route
			var catchAllHandler http.HandlerFunc
			for pattern, handler := range mockRouter.routes {
				if pattern == "/*" {
					catchAllHandler = handler
					break
				}
			}

			if catchAllHandler == nil {
				t.Fatal("No catch-all route found")
			}

			// Call the handler
			catchAllHandler(w, req)

			// Check the response
			if tt.expectNotFound {
				if w.Code != http.StatusNotFound {
					t.Errorf("Expected status 404 for %s, got %d", tt.path, w.Code)
				}
				t.Logf("SUCCESS: %s - %s", tt.name, tt.description)
			} else if tt.expectProxied {
				// For proxied requests, we expect either a proxy error (connection refused)
				// or a successful proxy attempt (not 404)
				if w.Code == http.StatusNotFound {
					t.Errorf("Expected path %s to be proxied (not 404), got %d", tt.path, w.Code)
				} else {
					t.Logf("SUCCESS: %s - %s (status: %d)", tt.name, tt.description, w.Code)
				}
			}
		})
	}
}

// TestShouldExcludeFromProxy tests the shouldExcludeFromProxy helper function
func TestShouldExcludeFromProxy(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		config   *ReverseProxyConfig
		expected bool
	}{
		{
			name:     "HealthEndpoint",
			path:     "/health",
			config:   &ReverseProxyConfig{},
			expected: true,
		},
		{
			name:     "HealthEndpointWithSlash",
			path:     "/health/",
			config:   &ReverseProxyConfig{},
			expected: true,
		},
		{
			name: "MetricsEndpoint",
			path: "/metrics/reverseproxy",
			config: &ReverseProxyConfig{
				MetricsEndpoint: "/metrics/reverseproxy",
			},
			expected: true,
		},
		{
			name: "MetricsHealthEndpoint",
			path: "/metrics/reverseproxy/health",
			config: &ReverseProxyConfig{
				MetricsEndpoint: "/metrics/reverseproxy",
			},
			expected: true,
		},
		{
			name:     "DebugEndpoint",
			path:     "/debug/info",
			config:   &ReverseProxyConfig{},
			expected: true,
		},
		{
			name:     "DebugFlags",
			path:     "/debug/flags",
			config:   &ReverseProxyConfig{},
			expected: true,
		},
		{
			name:     "RegularAPIPath",
			path:     "/api/v1/test",
			config:   &ReverseProxyConfig{},
			expected: false,
		},
		{
			name:     "RootPath",
			path:     "/",
			config:   &ReverseProxyConfig{},
			expected: false,
		},
		{
			name:     "CustomPath",
			path:     "/custom/endpoint",
			config:   &ReverseProxyConfig{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create module
			module := NewModule()
			module.config = tt.config

			// Test the function
			result := module.shouldExcludeFromProxy(tt.path)

			if result != tt.expected {
				t.Errorf("shouldExcludeFromProxy(%s) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestTenantAwareHealthEndpointHandling tests that health endpoints work correctly with tenant-aware routing
func TestTenantAwareHealthEndpointHandling(t *testing.T) {
	// Create mock router
	mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}

	// Create mock application
	app := NewMockTenantApplication()

	// Create configuration with tenants
	config := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"primary":   "http://primary:8080",
			"secondary": "http://secondary:8080",
		},
		DefaultBackend:  "primary",
		TenantIDHeader:  "X-Tenant-ID",
		RequireTenantID: false,
		MetricsEndpoint: "/metrics/reverseproxy",
	}

	// Register the configuration with the application
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

	// Create module
	module := NewModule()
	module.config = config

	// Set router via constructor
	services := map[string]any{
		"router": mockRouter,
	}
	constructedModule, err := module.Constructor()(app, services)
	if err != nil {
		t.Fatalf("Failed to construct module: %v", err)
	}
	module = constructedModule.(*ReverseProxyModule)

	// Set the app reference
	module.app = app

	// Initialize the module to set up backend proxies
	if err := module.Init(app); err != nil {
		t.Fatalf("Failed to initialize module: %v", err)
	}

	// Add a tenant manually for testing
	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"primary":   "http://tenant-primary:8080",
			"secondary": "http://tenant-secondary:8080",
		},
		DefaultBackend: "secondary",
	}
	module.tenants["test-tenant"] = tenantConfig

	// Start the module to register routes
	if err := module.Start(context.Background()); err != nil {
		t.Fatalf("Failed to start module: %v", err)
	}

	tests := []struct {
		name         string
		path         string
		tenantHeader string
		expectStatus int
		description  string
	}{
		{
			name:         "HealthWithoutTenant",
			path:         "/health",
			tenantHeader: "",
			expectStatus: http.StatusNotFound,
			description:  "Health endpoint without tenant should not be proxied",
		},
		{
			name:         "HealthWithTenant",
			path:         "/health",
			tenantHeader: "test-tenant",
			expectStatus: http.StatusNotFound,
			description:  "Health endpoint with tenant should not be proxied",
		},
		{
			name:         "MetricsWithoutTenant",
			path:         "/metrics/reverseproxy",
			tenantHeader: "",
			expectStatus: http.StatusNotFound,
			description:  "Metrics endpoint without tenant should not be proxied",
		},
		{
			name:         "MetricsWithTenant",
			path:         "/metrics/reverseproxy",
			tenantHeader: "test-tenant",
			expectStatus: http.StatusNotFound,
			description:  "Metrics endpoint with tenant should not be proxied",
		},
		{
			name:         "RegularAPIWithoutTenant",
			path:         "/api/test",
			tenantHeader: "",
			expectStatus: http.StatusBadGateway, // Expected proxy error due to unreachable backend
			description:  "Regular API without tenant should be proxied",
		},
		{
			name:         "RegularAPIWithTenant",
			path:         "/api/test",
			tenantHeader: "test-tenant",
			expectStatus: http.StatusBadGateway, // Expected proxy error due to unreachable backend
			description:  "Regular API with tenant should be proxied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find the handler for the catch-all route
			var catchAllHandler http.HandlerFunc
			for pattern, handler := range mockRouter.routes {
				if pattern == "/*" {
					catchAllHandler = handler
					break
				}
			}

			if catchAllHandler == nil {
				t.Fatal("No catch-all route found")
			}

			// Create request
			req := httptest.NewRequest("GET", tt.path, nil)
			if tt.tenantHeader != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantHeader)
			}
			w := httptest.NewRecorder()

			// Call the handler
			catchAllHandler(w, req)

			// Check the response
			if w.Code != tt.expectStatus {
				// For proxy errors, we might get different status codes depending on the exact error
				// So we'll be more lenient for proxied requests
				if tt.expectStatus == http.StatusBadGateway && w.Code >= 500 {
					t.Logf("SUCCESS: %s - %s (status: %d, expected proxy error)", tt.name, tt.description, w.Code)
				} else if tt.expectStatus == http.StatusNotFound && w.Code == http.StatusNotFound {
					t.Logf("SUCCESS: %s - %s", tt.name, tt.description)
				} else {
					t.Errorf("Expected status %d for %s, got %d", tt.expectStatus, tt.path, w.Code)
				}
			} else {
				t.Logf("SUCCESS: %s - %s", tt.name, tt.description)
			}
		})
	}
}
