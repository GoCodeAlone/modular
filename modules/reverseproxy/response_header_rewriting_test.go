package reverseproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResponseHeaderRewriting tests basic response header rewriting functionality
func TestResponseHeaderRewriting(t *testing.T) {
	t.Run("Backend Response Headers Modified", func(t *testing.T) {
		// Create mock backend server that sets various headers including CORS
		backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Backend sets its own headers
			w.Header().Set("X-Backend-Header", "backend-value")
			w.Header().Set("Access-Control-Allow-Origin", "http://backend.example.com")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
			w.Header().Set("X-Remove-Me", "should-be-removed")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}))
		defer backendServer.Close()

		// Create a reverse proxy module
		module := NewModule()

		// Configure response header rewriting
		module.config = &ReverseProxyConfig{
			BackendServices: map[string]string{
				"api": backendServer.URL,
			},
			BackendConfigs: map[string]BackendServiceConfig{
				"api": {
					URL: backendServer.URL,
					ResponseHeaderRewriting: ResponseHeaderRewritingConfig{
						SetHeaders: map[string]string{
							"X-Proxy-Header":               "proxy-value",
							"Access-Control-Allow-Origin":  "*",
							"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE",
						},
						RemoveHeaders: []string{"X-Remove-Me"},
					},
				},
			},
			TenantIDHeader: "X-Tenant-ID",
		}

		// Create the reverse proxy for API backend
		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(context.Background(), apiURL, "api", "")

		// Create a request
		req := httptest.NewRequest("GET", "http://client.example.com/api/products", nil)
		req.Host = "client.example.com"

		// Process the request through the proxy
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify the response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify response headers were modified
		assert.Equal(t, "proxy-value", resp.Header.Get("X-Proxy-Header"),
			"Proxy should set X-Proxy-Header")
		assert.Equal(t, "backend-value", resp.Header.Get("X-Backend-Header"),
			"Backend header should be preserved")
		assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"),
			"CORS Origin header should be overridden by proxy")
		assert.Equal(t, "GET, POST, PUT, DELETE", resp.Header.Get("Access-Control-Allow-Methods"),
			"CORS Methods header should be overridden by proxy")
		assert.Empty(t, resp.Header.Get("X-Remove-Me"),
			"X-Remove-Me header should be removed")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"),
			"Content-Type should be preserved")
	})

	t.Run("Global Response Header Configuration", func(t *testing.T) {
		// Create mock backend server
		backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Backend", "backend")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))
		defer backendServer.Close()

		// Create a reverse proxy module with global response header config
		module := NewModule()
		module.config = &ReverseProxyConfig{
			BackendServices: map[string]string{
				"api": backendServer.URL,
			},
			ResponseHeaderConfig: ResponseHeaderRewritingConfig{
				SetHeaders: map[string]string{
					"X-Global-Header": "global-value",
				},
			},
			TenantIDHeader: "X-Tenant-ID",
		}

		// Create the reverse proxy
		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(context.Background(), apiURL, "api", "")

		// Make a request
		req := httptest.NewRequest("GET", "http://client.example.com/test", nil)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify global header was added
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "global-value", resp.Header.Get("X-Global-Header"))
		assert.Equal(t, "backend", resp.Header.Get("X-Backend"))
	})
}

// TestEndpointResponseHeaderRewriting tests endpoint-specific response header overrides
func TestEndpointResponseHeaderRewriting(t *testing.T) {
	t.Run("Endpoint Override Backend Configuration", func(t *testing.T) {
		// Create mock backend server
		backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Backend", "backend")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))
		defer backendServer.Close()

		// Create a reverse proxy module with endpoint-specific config
		module := NewModule()
		module.config = &ReverseProxyConfig{
			BackendServices: map[string]string{
				"api": backendServer.URL,
			},
			BackendConfigs: map[string]BackendServiceConfig{
				"api": {
					URL: backendServer.URL,
					ResponseHeaderRewriting: ResponseHeaderRewritingConfig{
						SetHeaders: map[string]string{
							"X-Level": "backend",
						},
					},
					Endpoints: map[string]EndpointConfig{
						"/special": {
							Pattern: "/special",
							ResponseHeaderRewriting: ResponseHeaderRewritingConfig{
								SetHeaders: map[string]string{
									"X-Level":    "endpoint",
									"X-Endpoint": "special",
								},
							},
						},
					},
				},
			},
			TenantIDHeader: "X-Tenant-ID",
		}

		// Create the reverse proxy with endpoint
		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(context.Background(), apiURL, "api", "/special")

		// Make a request
		req := httptest.NewRequest("GET", "http://client.example.com/special", nil)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify endpoint-specific headers override backend headers
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "endpoint", resp.Header.Get("X-Level"),
			"Endpoint configuration should override backend configuration")
		assert.Equal(t, "special", resp.Header.Get("X-Endpoint"),
			"Endpoint-specific header should be set")
	})
}

// TestDynamicResponseHeaderModifier tests the custom response header modifier callback
func TestDynamicResponseHeaderModifier(t *testing.T) {
	t.Run("Custom Modifier Applied", func(t *testing.T) {
		// Create mock backend server
		backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "http://backend1.example.com")
			w.Header().Add("Access-Control-Allow-Origin", "http://backend2.example.com")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))
		defer backendServer.Close()

		// Create a reverse proxy module
		module := NewModule()
		module.config = &ReverseProxyConfig{
			BackendServices: map[string]string{
				"api": backendServer.URL,
			},
			TenantIDHeader: "X-Tenant-ID",
		}

		// Set a custom response header modifier that consolidates CORS headers
		module.SetResponseHeaderModifier(func(resp *http.Response, backendID string, tenantID modular.TenantID) error {
			// Remove duplicate CORS headers and set a single consolidated one
			resp.Header.Del("Access-Control-Allow-Origin")
			resp.Header.Set("Access-Control-Allow-Origin", "*")
			resp.Header.Set("X-Modified-By", "custom-modifier")
			resp.Header.Set("X-Backend-ID", backendID)
			if tenantID != "" {
				resp.Header.Set("X-Tenant-ID", string(tenantID))
			}
			return nil
		})

		// Create the reverse proxy
		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(context.Background(), apiURL, "api", "")

		// Make a request without tenant ID
		req := httptest.NewRequest("GET", "http://client.example.com/test", nil)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify custom modifier was applied
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"),
			"Custom modifier should consolidate CORS headers")
		assert.Equal(t, "custom-modifier", resp.Header.Get("X-Modified-By"))
		assert.Equal(t, "api", resp.Header.Get("X-Backend-ID"))
		assert.Empty(t, resp.Header.Get("X-Tenant-ID"), "No tenant ID should be set")

		// Make a request with tenant ID
		req2 := httptest.NewRequest("GET", "http://client.example.com/test", nil)
		req2.Header.Set("X-Tenant-ID", "tenant1")
		w2 := httptest.NewRecorder()
		proxy.ServeHTTP(w2, req2)

		// Verify tenant ID was passed to modifier
		resp2 := w2.Result()
		assert.Equal(t, "tenant1", resp2.Header.Get("X-Tenant-ID"))
	})

	t.Run("Modifier Error Handling", func(t *testing.T) {
		// Create mock backend server
		backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))
		defer backendServer.Close()

		// Create a reverse proxy module
		module := NewModule()
		module.config = &ReverseProxyConfig{
			BackendServices: map[string]string{
				"api": backendServer.URL,
			},
			TenantIDHeader: "X-Tenant-ID",
		}

		// Create a mock app with a logger to capture error logs
		mockApp := NewMockApplication()
		module.app = mockApp

		// Set a custom response header modifier that returns an error
		module.SetResponseHeaderModifier(func(resp *http.Response, backendID string, tenantID modular.TenantID) error {
			return assert.AnError
		})

		// Create the reverse proxy
		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(context.Background(), apiURL, "api", "")

		// Make a request - should still succeed even if modifier errors
		req := httptest.NewRequest("GET", "http://client.example.com/test", nil)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Response should indicate an error from the modifier
		resp := w.Result()
		// The modifier error should cause the proxy to return an error
		// (the exact status depends on implementation, but it should not be 200)
		assert.NotEqual(t, http.StatusOK, resp.StatusCode,
			"Modifier error should cause proxy to return error status")
	})
}

// TestTenantSpecificResponseHeaderRewriting tests tenant-specific response header configuration
func TestTenantSpecificResponseHeaderRewriting(t *testing.T) {
	t.Run("Tenant-Specific Response Headers", func(t *testing.T) {
		// Create mock backend server
		backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Backend", "backend")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))
		defer backendServer.Close()

		// Create a reverse proxy module with tenant-specific config
		module := NewModule()
		module.config = &ReverseProxyConfig{
			BackendServices: map[string]string{
				"api": backendServer.URL,
			},
			BackendConfigs: map[string]BackendServiceConfig{
				"api": {
					URL: backendServer.URL,
					ResponseHeaderRewriting: ResponseHeaderRewritingConfig{
						SetHeaders: map[string]string{
							"X-Config": "global",
						},
					},
				},
			},
			TenantIDHeader: "X-Tenant-ID",
		}

		// Add tenant-specific configuration
		module.tenants = map[modular.TenantID]*ReverseProxyConfig{
			"tenant1": {
				BackendServices: map[string]string{
					"api": backendServer.URL,
				},
				BackendConfigs: map[string]BackendServiceConfig{
					"api": {
						URL: backendServer.URL,
						ResponseHeaderRewriting: ResponseHeaderRewritingConfig{
							SetHeaders: map[string]string{
								"X-Config":     "tenant1",
								"X-Tenant-For": "tenant1",
							},
						},
					},
				},
				TenantIDHeader: "X-Tenant-ID",
			},
		}

		// Create the reverse proxy
		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(context.Background(), apiURL, "api", "")

		// Test global config (no tenant)
		req := httptest.NewRequest("GET", "http://client.example.com/test", nil)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "global", resp.Header.Get("X-Config"))
		assert.Empty(t, resp.Header.Get("X-Tenant-For"))

		// Test tenant-specific config
		req2 := httptest.NewRequest("GET", "http://client.example.com/test", nil)
		req2.Header.Set("X-Tenant-ID", "tenant1")
		w2 := httptest.NewRecorder()
		proxy.ServeHTTP(w2, req2)

		resp2 := w2.Result()
		assert.Equal(t, http.StatusOK, resp2.StatusCode)
		assert.Equal(t, "tenant1", resp2.Header.Get("X-Config"),
			"Tenant-specific configuration should override global")
		assert.Equal(t, "tenant1", resp2.Header.Get("X-Tenant-For"))
	})
}

// TestCORSHeaderConsolidation tests the use case of consolidating CORS headers
func TestCORSHeaderConsolidation(t *testing.T) {
	t.Run("CORS Header Consolidation Use Case", func(t *testing.T) {
		// Create mock backend that sends duplicate/conflicting CORS headers
		backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate a misconfigured backend with multiple CORS headers
			w.Header().Add("Access-Control-Allow-Origin", "http://old-domain.com")
			w.Header().Add("Access-Control-Allow-Methods", "GET")
			w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))
		defer backendServer.Close()

		// Create a reverse proxy module
		module := NewModule()
		module.config = &ReverseProxyConfig{
			BackendServices: map[string]string{
				"legacy-api": backendServer.URL,
			},
			BackendConfigs: map[string]BackendServiceConfig{
				"legacy-api": {
					URL: backendServer.URL,
					// Use response header rewriting to consolidate and override CORS headers
					ResponseHeaderRewriting: ResponseHeaderRewritingConfig{
						SetHeaders: map[string]string{
							"Access-Control-Allow-Origin":  "*",
							"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
							"Access-Control-Allow-Headers": "Content-Type, Authorization",
							"Access-Control-Max-Age":       "86400",
						},
					},
				},
			},
			TenantIDHeader: "X-Tenant-ID",
		}

		// Create the reverse proxy
		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(context.Background(), apiURL, "legacy-api", "")

		// Make a request
		req := httptest.NewRequest("GET", "http://client.example.com/api/data", nil)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify CORS headers are consolidated
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify that the proxy's CORS configuration overrides backend
		assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"),
			"Proxy should override backend CORS origin")
		assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", resp.Header.Get("Access-Control-Allow-Methods"),
			"Proxy should override backend CORS methods")
		assert.Equal(t, "Content-Type, Authorization", resp.Header.Get("Access-Control-Allow-Headers"),
			"Proxy should override backend CORS headers")
		assert.Equal(t, "86400", resp.Header.Get("Access-Control-Max-Age"),
			"Proxy should set CORS max age")

		// Verify there are no duplicate CORS headers (copyResponseHeaders should handle this)
		// Get all values to check for duplicates
		originValues := resp.Header.Values("Access-Control-Allow-Origin")
		assert.Equal(t, 1, len(originValues), "Should have exactly one CORS origin header")
	})
}

// TestResponseHeaderRewritingPriority tests the priority of response header configurations
func TestResponseHeaderRewritingPriority(t *testing.T) {
	t.Run("Priority: Endpoint > Backend > Global", func(t *testing.T) {
		// Create mock backend server
		backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))
		defer backendServer.Close()

		// Create a reverse proxy module with all three levels of config
		module := NewModule()
		module.config = &ReverseProxyConfig{
			BackendServices: map[string]string{
				"api": backendServer.URL,
			},
			// Global config
			ResponseHeaderConfig: ResponseHeaderRewritingConfig{
				SetHeaders: map[string]string{
					"X-Priority-Test": "global",
					"X-Global-Only":   "global",
				},
			},
			BackendConfigs: map[string]BackendServiceConfig{
				"api": {
					URL: backendServer.URL,
					// Backend config
					ResponseHeaderRewriting: ResponseHeaderRewritingConfig{
						SetHeaders: map[string]string{
							"X-Priority-Test": "backend",
							"X-Backend-Only":  "backend",
						},
					},
					Endpoints: map[string]EndpointConfig{
						"/endpoint": {
							Pattern: "/endpoint",
							// Endpoint config
							ResponseHeaderRewriting: ResponseHeaderRewritingConfig{
								SetHeaders: map[string]string{
									"X-Priority-Test": "endpoint",
									"X-Endpoint-Only": "endpoint",
								},
							},
						},
					},
				},
			},
			TenantIDHeader: "X-Tenant-ID",
		}

		// Create the reverse proxy with endpoint
		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(context.Background(), apiURL, "api", "/endpoint")

		// Make a request
		req := httptest.NewRequest("GET", "http://client.example.com/endpoint", nil)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify priority: endpoint > backend > global
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "endpoint", resp.Header.Get("X-Priority-Test"),
			"Endpoint config should have highest priority")
		assert.Equal(t, "endpoint", resp.Header.Get("X-Endpoint-Only"))
		assert.Equal(t, "backend", resp.Header.Get("X-Backend-Only"))
		assert.Equal(t, "global", resp.Header.Get("X-Global-Only"))
	})
}
