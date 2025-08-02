package reverseproxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPerBackendPathRewriting tests path rewriting configuration per backend
func TestPerBackendPathRewriting(t *testing.T) {
	// Track what path each backend receives
	var apiReceivedPath, userReceivedPath string

	// Create mock backend servers
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiReceivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]string{"service": "api", "path": r.URL.Path}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	userServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userReceivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]string{"service": "user", "path": r.URL.Path}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer userServer.Close()

	// Create a reverse proxy module
	module := NewModule()

	// Configure per-backend path rewriting
	module.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api":  apiServer.URL,
			"user": userServer.URL,
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"api": {
				URL: apiServer.URL,
				PathRewriting: PathRewritingConfig{
					StripBasePath:   "/api/v1",
					BasePathRewrite: "/internal/api",
				},
			},
			"user": {
				URL: userServer.URL,
				PathRewriting: PathRewritingConfig{
					StripBasePath:   "/user/v1",
					BasePathRewrite: "/internal/user",
				},
			},
		},
		TenantIDHeader: "X-Tenant-ID",
	}

	t.Run("API Backend Path Rewriting", func(t *testing.T) {
		// Reset received path
		apiReceivedPath = ""

		// Create the reverse proxy for API backend
		apiURL, err := url.Parse(apiServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(apiURL, "api", "")

		// Create a request that should be rewritten
		req := httptest.NewRequest("GET", "http://client.example.com/api/v1/products/123", nil)
		req.Host = "client.example.com"

		// Process the request through the proxy
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify the response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// The API backend should receive the path rewritten as /internal/api/products/123
		assert.Equal(t, "/internal/api/products/123", apiReceivedPath,
			"API backend should receive path with /api/v1 stripped and /internal/api prepended")
	})

	t.Run("User Backend Path Rewriting", func(t *testing.T) {
		// Reset received path
		userReceivedPath = ""

		// Create the reverse proxy for User backend
		userURL, err := url.Parse(userServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(userURL, "user", "")

		// Create a request that should be rewritten
		req := httptest.NewRequest("GET", "http://client.example.com/user/v1/profile/456", nil)
		req.Host = "client.example.com"

		// Process the request through the proxy
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify the response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// The User backend should receive the path rewritten as /internal/user/profile/456
		assert.Equal(t, "/internal/user/profile/456", userReceivedPath,
			"User backend should receive path with /user/v1 stripped and /internal/user prepended")
	})
}

// TestPerBackendHostnameHandling tests hostname handling configuration per backend
func TestPerBackendHostnameHandling(t *testing.T) {
	// Track what hostname each backend receives
	var apiReceivedHost, userReceivedHost string

	// Create mock backend servers
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiReceivedHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]string{"service": "api", "host": r.Host}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	userServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userReceivedHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]string{"service": "user", "host": r.Host}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer userServer.Close()

	// Create a reverse proxy module
	module := NewModule()

	// Configure per-backend hostname handling
	module.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api":  apiServer.URL,
			"user": userServer.URL,
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"api": {
				URL: apiServer.URL,
				HeaderRewriting: HeaderRewritingConfig{
					HostnameHandling: HostnamePreserveOriginal, // Default behavior
				},
			},
			"user": {
				URL: userServer.URL,
				HeaderRewriting: HeaderRewritingConfig{
					HostnameHandling: HostnameUseBackend, // Use backend hostname
				},
			},
		},
		TenantIDHeader: "X-Tenant-ID",
	}

	t.Run("API Backend Preserves Original Hostname", func(t *testing.T) {
		// Reset received host
		apiReceivedHost = ""

		// Create the reverse proxy for API backend
		apiURL, err := url.Parse(apiServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(apiURL, "api", "")

		// Create a request with original hostname
		req := httptest.NewRequest("GET", "http://client.example.com/api/products", nil)
		req.Host = "client.example.com"

		// Process the request through the proxy
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify the response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// The API backend should receive the original hostname
		assert.Equal(t, "client.example.com", apiReceivedHost,
			"API backend should receive original client hostname")
	})

	t.Run("User Backend Uses Backend Hostname", func(t *testing.T) {
		// Reset received host
		userReceivedHost = ""

		// Create the reverse proxy for User backend
		userURL, err := url.Parse(userServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(userURL, "user", "")

		// Create a request with original hostname
		req := httptest.NewRequest("GET", "http://client.example.com/user/profile", nil)
		req.Host = "client.example.com"

		// Process the request through the proxy
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify the response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// The User backend should receive the backend hostname
		expectedHost := userURL.Host
		assert.Equal(t, expectedHost, userReceivedHost,
			"User backend should receive backend hostname")
	})
}

// TestPerBackendCustomHostname tests custom hostname configuration per backend
func TestPerBackendCustomHostname(t *testing.T) {
	// Track what hostname the backend receives
	var receivedHost string

	// Create mock backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]string{"service": "api", "host": r.Host}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer backendServer.Close()

	// Create a reverse proxy module
	module := NewModule()

	// Configure custom hostname handling
	module.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api": backendServer.URL,
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"api": {
				URL: backendServer.URL,
				HeaderRewriting: HeaderRewritingConfig{
					HostnameHandling: HostnameUseCustom,
					CustomHostname:   "custom.internal.com",
				},
			},
		},
		TenantIDHeader: "X-Tenant-ID",
	}

	t.Run("Backend Uses Custom Hostname", func(t *testing.T) {
		// Reset received host
		receivedHost = ""

		// Create the reverse proxy for API backend
		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(apiURL, "api", "")

		// Create a request with original hostname
		req := httptest.NewRequest("GET", "http://client.example.com/api/products", nil)
		req.Host = "client.example.com"

		// Process the request through the proxy
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify the response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// The backend should receive the custom hostname
		assert.Equal(t, "custom.internal.com", receivedHost,
			"Backend should receive custom hostname")
	})
}

// TestPerBackendHeaderRewriting tests header rewriting configuration per backend
func TestPerBackendHeaderRewriting(t *testing.T) {
	// Track what headers the backend receives
	var receivedHeaders map[string]string

	// Create mock backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = make(map[string]string)
		for name, values := range r.Header {
			if len(values) > 0 {
				receivedHeaders[name] = values[0]
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"service": "api",
			"headers": receivedHeaders,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer backendServer.Close()

	// Create a reverse proxy module
	module := NewModule()

	// Configure header rewriting
	module.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api": backendServer.URL,
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"api": {
				URL: backendServer.URL,
				HeaderRewriting: HeaderRewritingConfig{
					SetHeaders: map[string]string{
						"X-API-Key":     "secret-key",
						"X-Custom-Auth": "bearer-token",
					},
					RemoveHeaders: []string{"X-Client-Version"},
				},
			},
		},
		TenantIDHeader: "X-Tenant-ID",
	}

	t.Run("Backend Receives Modified Headers", func(t *testing.T) {
		// Reset received headers
		receivedHeaders = nil

		// Create the reverse proxy for API backend
		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(apiURL, "api", "")

		// Create a request with original headers
		req := httptest.NewRequest("GET", "http://client.example.com/api/products", nil)
		req.Host = "client.example.com"
		req.Header.Set("X-Client-Version", "1.0.0")
		req.Header.Set("X-Original-Header", "original-value")

		// Process the request through the proxy
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify the response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// The backend should receive the modified headers
		assert.Equal(t, "secret-key", receivedHeaders["X-Api-Key"],
			"Backend should receive set X-API-Key header")
		assert.Equal(t, "bearer-token", receivedHeaders["X-Custom-Auth"],
			"Backend should receive set X-Custom-Auth header")
		assert.Equal(t, "original-value", receivedHeaders["X-Original-Header"],
			"Backend should receive original header that wasn't modified")
		assert.Empty(t, receivedHeaders["X-Client-Version"],
			"Backend should not receive removed X-Client-Version header")
	})
}

// TestPerEndpointConfiguration tests endpoint-specific configuration
func TestPerEndpointConfiguration(t *testing.T) {
	// Track what the backend receives
	var receivedPath, receivedHost string

	// Create mock backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]string{
			"service": "api",
			"path":    r.URL.Path,
			"host":    r.Host,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer backendServer.Close()

	// Create a reverse proxy module
	module := NewModule()

	// Configure endpoint-specific configuration
	module.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api": backendServer.URL,
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"api": {
				URL: backendServer.URL,
				PathRewriting: PathRewritingConfig{
					StripBasePath: "/api/v1",
				},
				HeaderRewriting: HeaderRewritingConfig{
					HostnameHandling: HostnamePreserveOriginal,
				},
				Endpoints: map[string]EndpointConfig{
					"users": {
						Pattern: "/users/*",
						PathRewriting: PathRewritingConfig{
							BasePathRewrite: "/internal/users",
						},
						HeaderRewriting: HeaderRewritingConfig{
							HostnameHandling: HostnameUseCustom,
							CustomHostname:   "users.internal.com",
						},
					},
				},
			},
		},
		TenantIDHeader: "X-Tenant-ID",
	}

	t.Run("Users Endpoint Uses Specific Configuration", func(t *testing.T) {
		// Reset received values
		receivedPath = ""
		receivedHost = ""

		// Create the reverse proxy for API backend with users endpoint
		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(apiURL, "api", "users")

		// Create a request to users endpoint
		req := httptest.NewRequest("GET", "http://client.example.com/api/v1/users/123", nil)
		req.Host = "client.example.com"

		// Process the request through the proxy
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify the response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// The backend should receive endpoint-specific configuration
		assert.Equal(t, "/internal/users/users/123", receivedPath,
			"Backend should receive endpoint-specific path rewriting")
		assert.Equal(t, "users.internal.com", receivedHost,
			"Backend should receive endpoint-specific hostname")
	})
}

// TestHeaderRewritingEdgeCases tests edge cases for header rewriting functionality
func TestHeaderRewritingEdgeCases(t *testing.T) {
	// Track received headers
	var receivedHeaders http.Header
	var receivedHost string

	// Mock backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture all headers
		receivedHeaders = r.Header.Clone()
		// Capture the Host field separately
		receivedHost = r.Host
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"backend response"}`))
	}))
	defer backendServer.Close()

	// Create module
	module := NewModule()

	t.Run("NilHeaderConfiguration", func(t *testing.T) {
		// Reset received headers
		receivedHeaders = nil

		// Create proxy with nil header configuration
		config := &ReverseProxyConfig{
			BackendServices: map[string]string{
				"api": backendServer.URL,
			},
			BackendConfigs: map[string]BackendServiceConfig{
				"api": {
					URL:             backendServer.URL,
					HeaderRewriting: HeaderRewritingConfig{
						// All fields are nil/empty
					},
				},
			},
			TenantIDHeader: "X-Tenant-ID",
		}
		module.config = config

		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)

		// Create proxy
		proxy := module.createReverseProxyForBackend(apiURL, "api", "")

		// Create request with headers
		req := httptest.NewRequest("GET", "http://client.example.com/api/test", nil)
		req.Host = "client.example.com"
		req.Header.Set("X-Original-Header", "original-value")

		// Process request
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Original headers should be preserved
		assert.Equal(t, "original-value", receivedHeaders.Get("X-Original-Header"))
		// Host should be preserved (original behavior)
		assert.Equal(t, "client.example.com", receivedHost)
	})

	t.Run("EmptyHeaderMaps", func(t *testing.T) {
		// Reset received headers
		receivedHeaders = nil

		// Create proxy with empty header maps
		config := &ReverseProxyConfig{
			BackendServices: map[string]string{
				"api": backendServer.URL,
			},
			BackendConfigs: map[string]BackendServiceConfig{
				"api": {
					URL: backendServer.URL,
					HeaderRewriting: HeaderRewritingConfig{
						HostnameHandling: HostnamePreserveOriginal,
						SetHeaders:       make(map[string]string), // Empty map
						RemoveHeaders:    make([]string, 0),       // Empty slice
					},
				},
			},
			TenantIDHeader: "X-Tenant-ID",
		}
		module.config = config

		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)

		// Create proxy
		proxy := module.createReverseProxyForBackend(apiURL, "api", "")

		// Create request with headers
		req := httptest.NewRequest("GET", "http://client.example.com/api/test", nil)
		req.Host = "client.example.com"
		req.Header.Set("X-Original-Header", "original-value")

		// Process request
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Original headers should be preserved
		assert.Equal(t, "original-value", receivedHeaders.Get("X-Original-Header"))
		assert.Equal(t, "client.example.com", receivedHost)
	})

	t.Run("CaseInsensitiveHeaderRemoval", func(t *testing.T) {
		// Reset received headers
		receivedHeaders = nil

		// Create proxy with case-insensitive header removal
		config := &ReverseProxyConfig{
			BackendServices: map[string]string{
				"api": backendServer.URL,
			},
			BackendConfigs: map[string]BackendServiceConfig{
				"api": {
					URL: backendServer.URL,
					HeaderRewriting: HeaderRewritingConfig{
						RemoveHeaders: []string{
							"x-remove-me",       // lowercase
							"X-REMOVE-ME-TOO",   // uppercase
							"X-Remove-Me-Three", // mixed case
						},
					},
				},
			},
			TenantIDHeader: "X-Tenant-ID",
		}
		module.config = config

		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)

		// Create proxy
		proxy := module.createReverseProxyForBackend(apiURL, "api", "")

		// Create request with headers in different cases
		req := httptest.NewRequest("GET", "http://client.example.com/api/test", nil)
		req.Host = "client.example.com"
		req.Header.Set("X-Remove-Me", "should-be-removed")
		req.Header.Set("x-remove-me-too", "should-be-removed-too")
		req.Header.Set("X-remove-me-three", "should-be-removed-three")
		req.Header.Set("X-Keep-Me", "should-be-kept")

		// Process request
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Headers should be removed (case-insensitive)
		assert.Empty(t, receivedHeaders.Get("X-Remove-Me"))
		assert.Empty(t, receivedHeaders.Get("X-Remove-Me-Too"))
		assert.Empty(t, receivedHeaders.Get("X-Remove-Me-Three"))

		// Other headers should be kept
		assert.Equal(t, "should-be-kept", receivedHeaders.Get("X-Keep-Me"))
	})

	t.Run("HeaderOverrideAndRemoval", func(t *testing.T) {
		// Reset received headers
		receivedHeaders = nil
		receivedHost = ""

		// Create proxy that both sets and removes headers
		config := &ReverseProxyConfig{
			BackendServices: map[string]string{
				"api": backendServer.URL,
			},
			BackendConfigs: map[string]BackendServiceConfig{
				"api": {
					URL: backendServer.URL,
					HeaderRewriting: HeaderRewritingConfig{
						SetHeaders: map[string]string{
							"X-Override-Me": "new-value",
							"X-New-Header":  "new-header-value",
						},
						RemoveHeaders: []string{
							"X-Remove-Me",
							"X-Override-Me", // Try to remove a header we're also setting
						},
					},
				},
			},
			TenantIDHeader: "X-Tenant-ID",
		}
		module.config = config

		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)

		// Create proxy
		proxy := module.createReverseProxyForBackend(apiURL, "api", "")

		// Create request
		req := httptest.NewRequest("GET", "http://client.example.com/api/test", nil)
		req.Host = "client.example.com"
		req.Header.Set("X-Override-Me", "original-value")
		req.Header.Set("X-Remove-Me", "remove-this")

		// Process request
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Set headers should be applied first, then removal
		// Since X-Override-Me is in the removal list, it should be removed even if set
		assert.Empty(t, receivedHeaders.Get("X-Override-Me"),
			"Header should be removed since it's in the removal list")
		assert.Equal(t, "new-header-value", receivedHeaders.Get("X-New-Header"))
		// Removed headers should be gone
		assert.Empty(t, receivedHeaders.Get("X-Remove-Me"))
	})

	t.Run("HostnameHandlingModes", func(t *testing.T) {
		testCases := []struct {
			name             string
			hostnameHandling HostnameHandlingMode
			customHostname   string
			expectedHost     string
		}{
			{
				name:             "PreserveOriginal",
				hostnameHandling: HostnamePreserveOriginal,
				customHostname:   "",
				expectedHost:     "client.example.com",
			},
			{
				name:             "UseBackend",
				hostnameHandling: HostnameUseBackend,
				customHostname:   "",
				expectedHost:     "backend.example.com", // This will be the backend server's host
			},
			{
				name:             "UseCustom",
				hostnameHandling: HostnameUseCustom,
				customHostname:   "custom.example.com",
				expectedHost:     "custom.example.com",
			},
			{
				name:             "UseCustomWithEmptyCustom",
				hostnameHandling: HostnameUseCustom,
				customHostname:   "",
				expectedHost:     "client.example.com", // Should fallback to original
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Reset received headers
				receivedHeaders = nil

				// Create proxy with specific hostname handling
				config := &ReverseProxyConfig{
					BackendServices: map[string]string{
						"api": backendServer.URL,
					},
					BackendConfigs: map[string]BackendServiceConfig{
						"api": {
							URL: backendServer.URL,
							HeaderRewriting: HeaderRewritingConfig{
								HostnameHandling: tc.hostnameHandling,
								CustomHostname:   tc.customHostname,
							},
						},
					},
					TenantIDHeader: "X-Tenant-ID",
				}
				module.config = config

				apiURL, err := url.Parse(backendServer.URL)
				require.NoError(t, err)

				// Create proxy
				proxy := module.createReverseProxyForBackend(apiURL, "api", "")

				// Create request
				req := httptest.NewRequest("GET", "http://client.example.com/api/test", nil)
				req.Host = "client.example.com"

				// Process request
				w := httptest.NewRecorder()
				proxy.ServeHTTP(w, req)

				// Verify response
				resp := w.Result()
				assert.Equal(t, http.StatusOK, resp.StatusCode)

				// Check hostname handling
				if tc.hostnameHandling == HostnameUseBackend {
					// For backend hostname, we expect the host from the backend URL
					backendURL, _ := url.Parse(backendServer.URL)
					assert.Equal(t, backendURL.Host, receivedHost)
				} else {
					assert.Equal(t, tc.expectedHost, receivedHost)
				}
			})
		}
	})

	t.Run("MultipleHeaderValues", func(t *testing.T) {
		// Reset received headers
		receivedHeaders = nil

		// Create proxy
		config := &ReverseProxyConfig{
			BackendServices: map[string]string{
				"api": backendServer.URL,
			},
			BackendConfigs: map[string]BackendServiceConfig{
				"api": {
					URL: backendServer.URL,
					HeaderRewriting: HeaderRewritingConfig{
						SetHeaders: map[string]string{
							"X-Multiple": "value1,value2,value3",
						},
					},
				},
			},
			TenantIDHeader: "X-Tenant-ID",
		}
		module.config = config

		apiURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)

		// Create proxy
		proxy := module.createReverseProxyForBackend(apiURL, "api", "")

		// Create request
		req := httptest.NewRequest("GET", "http://client.example.com/api/test", nil)
		req.Host = "client.example.com"

		// Process request
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Check multiple values
		assert.Equal(t, "value1,value2,value3", receivedHeaders.Get("X-Multiple"))
	})
}
