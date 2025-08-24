package reverseproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/CrisisTextLine/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHostnameNotForwarded tests that the reverseproxy module does not forward
// the hostname to the backend service, keeping the original request's Host header.
func TestHostnameNotForwarded(t *testing.T) {
	// Track what Host header the backend receives
	var receivedHost string

	// Create a mock backend server that captures the Host header
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"backend response","host":"` + r.Host + `"}`))
	}))
	defer backendServer.Close()

	// Create a reverse proxy module
	module := NewModule()

	// Set up the module configuration
	backendURL, err := url.Parse(backendServer.URL)
	require.NoError(t, err)

	module.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"test-backend": backendServer.URL,
		},
		DefaultBackend: "test-backend",
		TenantIDHeader: "X-Tenant-ID",
	}

	// Create the reverse proxy directly
	proxy := module.createReverseProxyForBackend(backendURL, "", "")
	require.NotNil(t, proxy)

	// Test Case 1: Request with custom Host header should preserve it
	t.Run("CustomHostHeaderPreserved", func(t *testing.T) {
		// Reset captured values
		receivedHost = ""

		// Create a request with a custom Host header
		req := httptest.NewRequest("GET", "http://original-host.com/api/test", nil)
		req.Host = "original-host.com"

		// Process the request through the proxy
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify the response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify the Host header received by backend
		// The backend should receive the original Host header, not the backend's host
		assert.Equal(t, "original-host.com", receivedHost,
			"Backend should receive original Host header, not be overridden with backend host")

		// Verify response body
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"host":"original-host.com"`)
	})

	// Test Case 2: Request without Host header should get it from URL
	t.Run("NoHostHeaderUsesURLHost", func(t *testing.T) {
		// Reset captured values
		receivedHost = ""

		// Create a request without explicit Host header
		req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
		// Don't set req.Host - let it use the URL host

		// Process the request through the proxy
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify the response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// The backend should receive the Host header from the original request URL
		assert.Equal(t, "example.com", receivedHost,
			"Backend should receive Host header from request URL when no explicit Host is set")
	})

	// Test Case 3: Request with different Host header and URL should preserve Host header
	t.Run("HostHeaderOverridesURLHost", func(t *testing.T) {
		// Reset captured values
		receivedHost = ""

		// Create a request with Host header different from URL host
		req := httptest.NewRequest("GET", "http://url-host.com/api/test", nil)
		req.Host = "header-host.com"

		// Process the request through the proxy
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify the response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// The backend should receive the Host header value, not the URL host
		assert.Equal(t, "header-host.com", receivedHost,
			"Backend should receive Host header value when it differs from URL host")
	})
}

// TestHostnameForwardingWithTenants tests that tenant-specific configurations
// also correctly handle hostname forwarding (i.e., don't forward it)
func TestHostnameForwardingWithTenants(t *testing.T) {
	// Track what Host header the backend receives
	var receivedHost string
	var receivedTenantHeader string

	// Create mock backend servers for different tenants
	globalBackendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host
		receivedTenantHeader = r.Header.Get("X-Tenant-ID")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"global backend","host":"` + r.Host + `"}`))
	}))
	defer globalBackendServer.Close()

	tenantBackendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host
		receivedTenantHeader = r.Header.Get("X-Tenant-ID")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"tenant backend","host":"` + r.Host + `"}`))
	}))
	defer tenantBackendServer.Close()

	// Create a reverse proxy module
	module := NewModule()

	// Set up the module with global configuration
	module.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api": globalBackendServer.URL,
		},
		DefaultBackend: "api",
		TenantIDHeader: "X-Tenant-ID",
	}

	// Set up tenant-specific configuration that overrides the backend URL
	tenantID := modular.TenantID("tenant-123")
	module.tenants = make(map[modular.TenantID]*ReverseProxyConfig)
	module.tenants[tenantID] = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api": tenantBackendServer.URL,
		},
		DefaultBackend: "api",
		TenantIDHeader: "X-Tenant-ID",
	}

	// Test Case 1: Request without tenant header should use global backend
	t.Run("GlobalBackendHostnameNotForwarded", func(t *testing.T) {
		// Reset captured values
		receivedHost = ""
		receivedTenantHeader = ""

		// Create the reverse proxy for global backend
		globalURL, err := url.Parse(globalBackendServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(globalURL, "", "")

		// Create a request without tenant header
		req := httptest.NewRequest("GET", "http://client.example.com/api/test", nil)
		req.Host = "client.example.com"

		// Process the request through the proxy
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify the response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify the Host header received by global backend
		assert.Equal(t, "client.example.com", receivedHost,
			"Global backend should receive original Host header")
		assert.Empty(t, receivedTenantHeader,
			"Global backend should not receive tenant header")
	})

	// Test Case 2: Request with tenant header should use tenant backend
	t.Run("TenantBackendHostnameNotForwarded", func(t *testing.T) {
		// Reset captured values
		receivedHost = ""
		receivedTenantHeader = ""

		// Create the reverse proxy for tenant backend
		tenantURL, err := url.Parse(tenantBackendServer.URL)
		require.NoError(t, err)
		proxy := module.createReverseProxyForBackend(tenantURL, "", "")

		// Create a request with tenant header
		req := httptest.NewRequest("GET", "http://tenant-client.example.com/api/test", nil)
		req.Host = "tenant-client.example.com"
		req.Header.Set("X-Tenant-ID", string(tenantID))

		// Process the request through the proxy
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		// Verify the response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify the Host header received by tenant backend
		assert.Equal(t, "tenant-client.example.com", receivedHost,
			"Tenant backend should receive original Host header")
		assert.Equal(t, string(tenantID), receivedTenantHeader,
			"Tenant backend should receive the tenant header")
	})
}

// TestHostnameForwardingComparisonWithDefault tests that our fix actually changes
// behavior from the default Go reverse proxy behavior
func TestHostnameForwardingComparisonWithDefault(t *testing.T) {
	// Track what Host header the backend receives
	var receivedHostCustom string
	var receivedHostDefault string

	// Create a mock backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This will be called by both proxies, we'll track both
		if r.Header.Get("X-Proxy-Type") == "custom" {
			receivedHostCustom = r.Host
		} else {
			receivedHostDefault = r.Host
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"backend response","host":"` + r.Host + `"}`))
	}))
	defer backendServer.Close()

	backendURL, err := url.Parse(backendServer.URL)
	require.NoError(t, err)

	// Create our custom reverse proxy module
	module := NewModule()
	module.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"test-backend": backendServer.URL,
		},
		DefaultBackend: "test-backend",
		TenantIDHeader: "X-Tenant-ID",
	}
	customProxy := module.createReverseProxyForBackend(backendURL, "", "")

	// Create a default Go reverse proxy for comparison
	defaultProxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = backendURL.Scheme
			req.URL.Host = backendURL.Host
			req.URL.Path = backendURL.Path + req.URL.Path
			// This is the default Go behavior - sets Host header to backend host
			req.Host = backendURL.Host
		},
	}

	// Test with the same request to both proxies
	originalHost := "original-client.example.com"

	// Test our custom proxy
	t.Run("CustomProxyPreservesHost", func(t *testing.T) {
		receivedHostCustom = ""

		req := httptest.NewRequest("GET", "http://"+originalHost+"/api/test", nil)
		req.Host = originalHost
		req.Header.Set("X-Proxy-Type", "custom")

		w := httptest.NewRecorder()
		customProxy.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, originalHost, receivedHostCustom,
			"Custom proxy should preserve original Host header")
	})

	// Test default proxy behavior
	t.Run("DefaultProxyOverridesHost", func(t *testing.T) {
		receivedHostDefault = ""

		req := httptest.NewRequest("GET", "http://"+originalHost+"/api/test", nil)
		req.Host = originalHost
		req.Header.Set("X-Proxy-Type", "default")

		w := httptest.NewRecorder()
		defaultProxy.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, backendURL.Host, receivedHostDefault,
			"Default proxy should override Host header with backend host")
	})

	// Verify that the behaviors are actually different
	assert.NotEqual(t, receivedHostCustom, receivedHostDefault,
		"Custom and default proxy should have different Host header behaviors")
	assert.Equal(t, originalHost, receivedHostCustom,
		"Custom proxy should preserve original host")
	assert.Equal(t, backendURL.Host, receivedHostDefault,
		"Default proxy should use backend host")
}
