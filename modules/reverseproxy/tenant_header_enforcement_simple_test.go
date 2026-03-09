package reverseproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoCodeAlone/modular"
)

// TestTenantHeaderEnforcementSimple tests tenant header enforcement across all route types
func TestTenantHeaderEnforcementSimple(t *testing.T) {
	// Create test backend servers
	tenantAServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-ID", "tenant-a-backend")
		w.Header().Set("X-Tenant-ID", r.Header.Get("X-Tenant-ID"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("tenant-a backend response"))
	}))
	defer tenantAServer.Close()

	defaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-ID", "default-backend")
		w.Header().Set("X-Tenant-ID", r.Header.Get("X-Tenant-ID"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("default backend response"))
	}))
	defer defaultServer.Close()

	compositeServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-ID", "composite-backend-1")
		w.Header().Set("X-Tenant-ID", r.Header.Get("X-Tenant-ID"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("composite backend 1 response"))
	}))
	defer compositeServer1.Close()

	compositeServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-ID", "composite-backend-2")
		w.Header().Set("X-Tenant-ID", r.Header.Get("X-Tenant-ID"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("composite backend 2 response"))
	}))
	defer compositeServer2.Close()

	// Create configuration with tenant ID requirement enabled
	config := &ReverseProxyConfig{
		RequireTenantID: true,
		TenantIDHeader:  "X-Tenant-ID",
		BackendServices: map[string]string{
			"tenant-a-backend":    tenantAServer.URL,
			"default-backend":     defaultServer.URL,
			"composite-backend-1": compositeServer1.URL,
			"composite-backend-2": compositeServer2.URL,
		},
		Routes: map[string]string{
			"/api/explicit":   "tenant-a-backend",
			"/explicit/route": "default-backend",
		},
		CompositeRoutes: map[string]CompositeRoute{
			"/api/composite": {
				Pattern:  "/api/composite",
				Backends: []string{"composite-backend-1", "composite-backend-2"},
				Strategy: "combine",
			},
		},
		DefaultBackend: "default-backend",
		BackendConfigs: map[string]BackendServiceConfig{
			"tenant-a-backend":    {URL: tenantAServer.URL},
			"default-backend":     {URL: defaultServer.URL},
			"composite-backend-1": {URL: compositeServer1.URL},
			"composite-backend-2": {URL: compositeServer2.URL},
		},
	}

	// Create application and module
	app, err := modular.NewApplication(modular.WithLogger(&testLogger{}))
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}

	// Register required services
	router := &testRouter{routes: make(map[string]http.HandlerFunc)}
	if err := app.RegisterService("router", router); err != nil {
		t.Fatalf("Failed to register router: %v", err)
	}
	if err := app.RegisterService("logger", &testLogger{}); err != nil {
		t.Fatalf("Failed to register logger: %v", err)
	}
	if err := app.RegisterService("metrics", &testMetrics{}); err != nil {
		t.Fatalf("Failed to register metrics: %v", err)
	}

	// Setup config feeders
	configFeeders := []modular.Feeder{
		&mockConfigFeeder{
			configs: map[string]interface{}{
				"reverseproxy": config,
			},
		},
	}
	if stdApp, ok := app.(*modular.StdApplication); ok {
		stdApp.SetConfigFeeders(configFeeders)
	}

	// Create and register reverse proxy module
	module := NewModule()
	app.RegisterModule(module)

	constructor := module.Constructor()
	services := map[string]any{"router": router}
	constructedModule, err := constructor(app, services)
	if err != nil {
		t.Fatalf("Failed to construct module: %v", err)
	}

	rpModule := constructedModule.(*ReverseProxyModule)

	// Initialize and start the application
	if err := app.Init(); err != nil {
		t.Fatalf("Failed to initialize application: %v", err)
	}
	if err := app.Start(); err != nil {
		t.Fatalf("Failed to start application: %v", err)
	}

	// Test cases for tenant header enforcement
	testCases := []struct {
		name           string
		route          string
		tenantHeader   string
		expectedStatus int
		description    string
	}{
		// Without tenant header - should all return 400
		{"Explicit route without tenant", "/api/explicit", "", http.StatusBadRequest, "explicit route should enforce tenant header"},
		{"Explicit to default without tenant", "/explicit/route", "", http.StatusBadRequest, "explicit route to default should enforce tenant header"},
		{"Composite route without tenant", "/api/composite", "", http.StatusBadRequest, "composite route should enforce tenant header"},
		{"Default backend without tenant", "/unmapped/route", "", http.StatusBadRequest, "default backend should enforce tenant header"},

		// With valid tenant header - should all succeed
		{"Explicit route with tenant", "/api/explicit", "tenant-a", http.StatusOK, "explicit route should work with tenant header"},
		{"Explicit to default with tenant", "/explicit/route", "tenant-a", http.StatusOK, "explicit route to default should work with tenant header"},
		{"Default backend with tenant", "/unmapped/route", "tenant-a", http.StatusOK, "default backend should work with tenant header"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req := httptest.NewRequest("GET", tc.route, nil)
			if tc.tenantHeader != "" {
				req.Header.Set("X-Tenant-ID", tc.tenantHeader)
			}

			// Make request through router
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			// Check status code
			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("%s: expected status %d, got %d", tc.description, tc.expectedStatus, resp.StatusCode)
			}

			// For successful requests, verify tenant header forwarding
			if tc.expectedStatus == http.StatusOK && tc.tenantHeader != "" {
				returnedTenantID := resp.Header.Get("X-Tenant-ID")
				if returnedTenantID != tc.tenantHeader {
					t.Errorf("%s: expected tenant ID %s in response, got %s", tc.description, tc.tenantHeader, returnedTenantID)
				}
			}
		})
	}

	// Test composite route with tenant header (separate test due to different handling)
	t.Run("Composite route with tenant header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/composite", nil)
		req.Header.Set("X-Tenant-ID", "tenant-a")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		resp := rec.Result()
		defer resp.Body.Close()

		// Composite routes might have different success criteria
		if resp.StatusCode == http.StatusBadRequest {
			t.Error("Composite route should not return 400 with valid tenant header")
		}

		// Read response body to verify it contains data (if successful)
		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("Failed to read composite response: %v", err)
			} else if len(body) == 0 {
				t.Error("Composite response should contain data")
			}
		}
	})

	// Test tenant isolation
	t.Run("Tenant isolation", func(t *testing.T) {
		// Request with tenant A
		reqA := httptest.NewRequest("GET", "/api/explicit", nil)
		reqA.Header.Set("X-Tenant-ID", "tenant-a")

		recA := httptest.NewRecorder()
		router.ServeHTTP(recA, reqA)

		respA := recA.Result()
		defer respA.Body.Close()

		// Request with tenant B
		reqB := httptest.NewRequest("GET", "/api/explicit", nil)
		reqB.Header.Set("X-Tenant-ID", "tenant-b")

		recB := httptest.NewRecorder()
		router.ServeHTTP(recB, reqB)

		respB := recB.Result()
		defer respB.Body.Close()

		// Both should succeed (or handle appropriately)
		if respA.StatusCode == http.StatusBadRequest {
			t.Error("Request with tenant-a should not return 400")
		}
		if respB.StatusCode == http.StatusBadRequest {
			t.Error("Request with tenant-b should not return 400")
		}

		// Verify tenant headers are preserved
		if respA.StatusCode == http.StatusOK {
			tenantA := respA.Header.Get("X-Tenant-ID")
			if tenantA != "tenant-a" {
				t.Errorf("Expected tenant-a in response, got %s", tenantA)
			}
		}

		if respB.StatusCode == http.StatusOK {
			tenantB := respB.Header.Get("X-Tenant-ID")
			if tenantB != "tenant-b" {
				t.Errorf("Expected tenant-b in response, got %s", tenantB)
			}
		}
	})

	// Verify configuration
	t.Run("Configuration verification", func(t *testing.T) {
		if !rpModule.config.RequireTenantID {
			t.Error("RequireTenantID should be enabled")
		}
		if rpModule.config.TenantIDHeader != "X-Tenant-ID" {
			t.Errorf("Expected TenantIDHeader to be X-Tenant-ID, got %s", rpModule.config.TenantIDHeader)
		}
	})
}
