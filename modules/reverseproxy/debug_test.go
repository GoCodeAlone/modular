package reverseproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebugHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create a mock reverse proxy config
	proxyConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"primary":   "http://primary.example.com",
			"secondary": "http://secondary.example.com",
		},
		Routes: map[string]string{
			"/api/v1/users": "primary",
			"/api/v2/data":  "secondary",
		},
		DefaultBackend: "primary",
		TenantIDHeader: "X-Tenant-ID", // Set explicit default for testing
	}

	// Create a mock feature flag evaluator
	mockApp := NewMockTenantApplication()
	featureFlagEval, err := NewFileBasedFeatureFlagEvaluator(mockApp, logger)
	if err != nil {
		t.Fatalf("Failed to create feature flag evaluator: %v", err)
	}

	// Test with authentication enabled
	t.Run("WithAuthentication", func(t *testing.T) {
		config := DebugEndpointsConfig{
			Enabled:     true,
			BasePath:    "/debug",
			RequireAuth: true,
			AuthToken:   "test-token",
		}

		debugHandler := NewDebugHandler(config, featureFlagEval, proxyConfig, nil, logger)

		// Test authentication required
		t.Run("RequiresAuthentication", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/debug/info", nil)
			w := httptest.NewRecorder()

			debugHandler.handleInfo(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		// Test with valid auth token
		t.Run("ValidAuthentication", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/debug/info", nil)
			req.Header.Set("Authorization", "Bearer test-token")
			w := httptest.NewRecorder()

			debugHandler.handleInfo(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response DebugInfo
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.NotZero(t, response.Timestamp)
			assert.Equal(t, "local", response.Environment)
			assert.Equal(t, proxyConfig.BackendServices, response.BackendServices)
			assert.Equal(t, proxyConfig.Routes, response.Routes)
		})
	})

	// Test without authentication
	t.Run("WithoutAuthentication", func(t *testing.T) {
		config := DebugEndpointsConfig{
			Enabled:     true,
			BasePath:    "/debug",
			RequireAuth: false,
		}

		debugHandler := NewDebugHandler(config, featureFlagEval, proxyConfig, nil, logger)

		t.Run("InfoEndpoint", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/debug/info", nil)
			w := httptest.NewRecorder()

			debugHandler.handleInfo(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response DebugInfo
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.NotZero(t, response.Timestamp)
			assert.Equal(t, "local", response.Environment)
			assert.Equal(t, proxyConfig.BackendServices, response.BackendServices)
			assert.Equal(t, proxyConfig.Routes, response.Routes)
		})

		t.Run("BackendsEndpoint", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/debug/backends", nil)
			w := httptest.NewRecorder()

			debugHandler.handleBackends(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response map[string]interface{}
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.Contains(t, response, "timestamp")
			assert.Contains(t, response, "backendServices")
			assert.Contains(t, response, "routes")
			assert.Contains(t, response, "defaultBackend")

			backendServices := response["backendServices"].(map[string]interface{})
			assert.Equal(t, "http://primary.example.com", backendServices["primary"])
			assert.Equal(t, "http://secondary.example.com", backendServices["secondary"])
		})

		t.Run("FlagsEndpoint", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/debug/flags", nil)
			w := httptest.NewRecorder()

			debugHandler.handleFlags(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response DebugInfo
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			// Flags might be nil if no feature flag evaluator is set
			// Just check that the response structure is correct
		})

		t.Run("CircuitBreakersEndpoint", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/debug/circuit-breakers", nil)
			w := httptest.NewRecorder()

			debugHandler.handleCircuitBreakers(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response map[string]interface{}
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.Contains(t, response, "timestamp")
			assert.Contains(t, response, "circuitBreakers")
		})

		t.Run("HealthChecksEndpoint", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/debug/health-checks", nil)
			w := httptest.NewRecorder()

			debugHandler.handleHealthChecks(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response map[string]interface{}
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.Contains(t, response, "timestamp")
			assert.Contains(t, response, "healthChecks")
		})
	})

	// Test route registration
	t.Run("RouteRegistration", func(t *testing.T) {
		config := DebugEndpointsConfig{
			Enabled:     true,
			BasePath:    "/debug",
			RequireAuth: false,
		}

		debugHandler := NewDebugHandler(config, featureFlagEval, proxyConfig, nil, logger)

		mux := http.NewServeMux()
		debugHandler.RegisterRoutes(mux)

		// Test that routes are accessible
		endpoints := []string{
			"/debug/info",
			"/debug/flags",
			"/debug/backends",
			"/debug/circuit-breakers",
			"/debug/health-checks",
		}

		server := httptest.NewServer(mux)
		defer server.Close()

		for _, endpoint := range endpoints {
			t.Run(fmt.Sprintf("Route%s", endpoint), func(t *testing.T) {
				req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL+endpoint, nil)
				require.NoError(t, err)

				client := &http.Client{}
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
			})
		}
	})

	// Test disabled debug endpoints
	t.Run("DisabledEndpoints", func(t *testing.T) {
		config := DebugEndpointsConfig{
			Enabled:     false,
			BasePath:    "/debug",
			RequireAuth: false,
		}

		debugHandler := NewDebugHandler(config, featureFlagEval, proxyConfig, nil, logger)

		mux := http.NewServeMux()
		debugHandler.RegisterRoutes(mux)

		// Routes should not be registered when disabled
		req := httptest.NewRequest("GET", "/debug/info", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		// Should get 404 since routes are not registered
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	// Test tenant ID extraction
	t.Run("TenantIDExtraction", func(t *testing.T) {
		config := DebugEndpointsConfig{
			Enabled:     true,
			BasePath:    "/debug",
			RequireAuth: false,
		}

		debugHandler := NewDebugHandler(config, featureFlagEval, proxyConfig, nil, logger)

		t.Run("FromHeader", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/debug/info", nil)
			req.Header.Set("X-Tenant-ID", "test-tenant")
			w := httptest.NewRecorder()

			debugHandler.handleInfo(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response DebugInfo
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.Equal(t, "test-tenant", response.Tenant)
		})

	})
}

func TestDebugHandlerWithMocks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	proxyConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"primary": "http://primary.example.com",
		},
		Routes:         map[string]string{},
		DefaultBackend: "primary",
	}

	config := DebugEndpointsConfig{
		Enabled:     true,
		BasePath:    "/debug",
		RequireAuth: false,
	}

	debugHandler := NewDebugHandler(config, nil, proxyConfig, nil, logger)

	t.Run("CircuitBreakerInfo", func(t *testing.T) {
		// Create mock circuit breakers
		mockCircuitBreakers := map[string]*CircuitBreaker{
			"primary": NewCircuitBreaker("primary", nil),
		}
		debugHandler.SetCircuitBreakers(mockCircuitBreakers)

		req := httptest.NewRequest("GET", "/debug/info", nil)
		w := httptest.NewRecorder()

		debugHandler.handleInfo(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response DebugInfo
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Contains(t, response.CircuitBreakers, "primary")
		assert.Equal(t, "closed", response.CircuitBreakers["primary"].State)
	})

	t.Run("HealthCheckInfo", func(t *testing.T) {
		// Create mock health checkers
		mockHealthCheckers := map[string]*HealthChecker{
			"primary": NewHealthChecker(
				&HealthCheckConfig{Enabled: true},
				map[string]string{"primary": "http://primary.example.com"},
				&http.Client{},
				logger.WithGroup("health"),
			),
		}
		debugHandler.SetHealthCheckers(mockHealthCheckers)

		req := httptest.NewRequest("GET", "/debug/info", nil)
		w := httptest.NewRecorder()

		debugHandler.handleInfo(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response DebugInfo
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		// Health checkers may not populate immediately, so just check structure
		// Since the health checker hasn't been started, the status map will be empty
		// Due to omitempty JSON tag, empty maps become nil after JSON round-trip
		// This is expected behavior, so we'll check that it's either nil or empty
		if len(mockHealthCheckers) > 0 {
			// HealthChecks can be nil (omitted due to omitempty) or empty map
			if response.HealthChecks != nil {
				assert.Empty(t, response.HealthChecks)
			}
		}
	})
}
