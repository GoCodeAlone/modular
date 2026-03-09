package reverseproxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
)

// TestDebugAuthScenarios tests the authenticated debug endpoints BDD scenarios
func TestDebugAuthScenarios(t *testing.T) {
	t.Run("UnauthenticatedRequestsShouldReceive401", func(t *testing.T) {
		ctx := setupAuthenticatedDebugContext(t)
		defer ctx.resetContext()

		endpoints := []string{
			"/debug/info",
			"/debug/backends",
			"/debug/flags",
			"/debug/circuit-breakers",
			"/debug/health-checks",
		}

		for _, endpoint := range endpoints {
			resp, err := ctx.makeRequestThroughModule("GET", endpoint, nil)
			if err != nil {
				t.Fatalf("Failed to make unauthenticated request to %s: %v", endpoint, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("Expected HTTP 401 for unauthenticated request to %s, got %d", endpoint, resp.StatusCode)
			}

			wwwAuth := resp.Header.Get("WWW-Authenticate")
			if wwwAuth != "Bearer" {
				t.Errorf("Expected WWW-Authenticate: Bearer header for %s, got: %s", endpoint, wwwAuth)
			}
		}
	})

	t.Run("AuthenticatedRequestsShouldSucceed", func(t *testing.T) {
		ctx := setupAuthenticatedDebugContext(t)
		defer ctx.resetContext()

		endpoints := []string{
			"/debug/info",
			"/debug/backends",
			"/debug/flags",
			"/debug/circuit-breakers",
			"/debug/health-checks",
		}

		headers := map[string]string{
			"Authorization": "Bearer test-auth-token-12345",
			"X-Tenant-ID":   "tenant-a",
		}

		for _, endpoint := range endpoints {
			resp, err := ctx.makeRequestThroughModuleWithHeaders("GET", endpoint, nil, headers)
			if err != nil {
				t.Fatalf("Failed to make authenticated request to %s: %v", endpoint, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected HTTP 200 for authenticated request to %s, got %d", endpoint, resp.StatusCode)
			}

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				t.Errorf("Expected JSON content type for %s, got: %s", endpoint, contentType)
			}

			// Read and validate response body
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body from %s: %v", endpoint, err)
			}

			var data map[string]interface{}
			if err := json.Unmarshal(body, &data); err != nil {
				t.Errorf("Failed to parse JSON response from %s: %v", endpoint, err)
				continue
			}

			// Check for expected fields based on endpoint
			switch endpoint {
			case "/debug/info":
				if _, ok := data["module_name"]; !ok {
					t.Errorf("Debug info endpoint missing module_name field")
				}
				if _, ok := data["backendServices"]; !ok {
					t.Errorf("Debug info endpoint missing backendServices field")
				}
			case "/debug/backends":
				if _, ok := data["backendServices"]; !ok {
					t.Errorf("Debug backends endpoint missing backendServices field")
				}
			case "/debug/flags":
				// Feature flags endpoint should have timestamp
				if _, ok := data["timestamp"]; !ok {
					t.Errorf("Debug flags endpoint missing timestamp field")
				}
			case "/debug/circuit-breakers":
				if _, ok := data["circuit_breakers"]; !ok {
					t.Errorf("Debug circuit-breakers endpoint missing circuit_breakers field")
				}
			case "/debug/health-checks":
				if _, ok := data["health_checks"]; !ok {
					t.Errorf("Debug health-checks endpoint missing health_checks field")
				}
			}
		}
	})

	t.Run("TenantSpecificRoutesShouldBeIncluded", func(t *testing.T) {
		ctx := setupAuthenticatedDebugContext(t)
		defer ctx.resetContext()

		headers := map[string]string{
			"Authorization": "Bearer test-auth-token-12345",
			"X-Tenant-ID":   "tenant-a",
		}

		resp, err := ctx.makeRequestThroughModuleWithHeaders("GET", "/debug/info", nil, headers)
		if err != nil {
			t.Fatalf("Failed to make authenticated request: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Verify tenant information is included
		if tenant, ok := data["tenant"]; ok {
			if tenant != "tenant-a" {
				t.Errorf("Expected tenant 'tenant-a', got %v", tenant)
			}
		}

		// Verify tenant-specific routes are present in backend services
		if backendServices, ok := data["backendServices"]; ok {
			services := backendServices.(map[string]interface{})
			if _, hasTenantA := services["tenant-a-service"]; !hasTenantA {
				t.Errorf("Missing tenant-a-service in backend services")
			}
			if _, hasTenantB := services["tenant-b-service"]; !hasTenantB {
				t.Errorf("Missing tenant-b-service in backend services")
			}
		} else {
			t.Errorf("Missing backendServices in debug info")
		}

		// Verify routes configuration
		if routes, ok := data["routes"]; ok {
			routesMap := routes.(map[string]interface{})
			if _, hasTenantARoute := routesMap["/api/tenant-a/*"]; !hasTenantARoute {
				t.Errorf("Missing tenant-a route in routes configuration")
			}
		}
	})

	t.Run("CircuitBreakerStateShouldBeIncluded", func(t *testing.T) {
		ctx := setupAuthenticatedDebugContext(t)
		defer ctx.resetContext()

		// Make some failing requests to trigger circuit breaker state
		for i := 0; i < 3; i++ {
			resp, err := ctx.makeRequestThroughModule("GET", "/api/fail/test", nil)
			if err == nil && resp != nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
			time.Sleep(10 * time.Millisecond)
		}

		headers := map[string]string{
			"Authorization": "Bearer test-auth-token-12345",
			"X-Tenant-ID":   "tenant-a",
		}

		// Check circuit breaker endpoint
		resp, err := ctx.makeRequestThroughModuleWithHeaders("GET", "/debug/circuit-breakers", nil, headers)
		if err != nil {
			t.Fatalf("Failed to make authenticated request: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Verify circuit breaker data structure
		if cbData, ok := data["circuit_breakers"]; ok {
			cbMap := cbData.(map[string]interface{})

			// If we have circuit breaker data, validate structure
			if len(cbMap) > 0 {
				for cbName, cbInfo := range cbMap {
					if cb, ok := cbInfo.(map[string]interface{}); ok {
						if _, hasState := cb["state"]; !hasState {
							t.Errorf("Circuit breaker %s missing state field", cbName)
						}
					}
				}
			}
		} else {
			t.Errorf("Missing circuit_breakers field in debug response")
		}
	})

	t.Run("InvalidTokenShouldReceive403", func(t *testing.T) {
		ctx := setupAuthenticatedDebugContext(t)
		defer ctx.resetContext()

		headers := map[string]string{
			"Authorization": "Bearer invalid-token",
		}

		resp, err := ctx.makeRequestThroughModuleWithHeaders("GET", "/debug/info", nil, headers)
		if err != nil {
			t.Fatalf("Failed to make request with invalid token: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected HTTP 403 for invalid token, got %d", resp.StatusCode)
		}
	})
}

// setupAuthenticatedDebugContext creates a reverse proxy context with authenticated debug endpoints
func setupAuthenticatedDebugContext(t *testing.T) *ReverseProxyBDDTestContext {
	ctx := &ReverseProxyBDDTestContext{}

	// Create new application
	app, err := modular.NewApplication(modular.WithLogger(&testLogger{}))
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	ctx.app = app

	// Create test backends
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend1 response"))
	}))
	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend2 response"))
	}))
	failingBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("service unavailable"))
	}))

	ctx.testServers = []*httptest.Server{backend1, backend2, failingBackend}

	// Configure reverse proxy with authenticated debug endpoints
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"tenant-a-service": backend1.URL,
			"tenant-b-service": backend2.URL,
			"failing-service":  failingBackend.URL,
		},
		Routes: map[string]string{
			"/api/tenant-a/*": "tenant-a-service",
			"/api/tenant-b/*": "tenant-b-service",
			"/api/fail/*":     "failing-service",
		},
		TenantIDHeader: "X-Tenant-ID",
		DebugEndpoints: DebugEndpointsConfig{
			Enabled:     true,
			BasePath:    "/debug",
			RequireAuth: true,
			AuthToken:   "test-auth-token-12345",
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 2,
			OpenTimeout:      100 * time.Millisecond,
		},
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"tenant-routing":  true,
				"circuit-breaker": true,
			},
		},
	}

	if err := ctx.setupApplicationWithConfig(); err != nil {
		t.Fatalf("Failed to setup application: %v", err)
	}

	if err := ctx.ensureServiceInitialized(); err != nil {
		t.Fatalf("Failed to initialize service: %v", err)
	}

	return ctx
}
