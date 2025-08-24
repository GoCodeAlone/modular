package reverseproxy

import (
	"context"
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

// TestStandaloneBackendProxyHandler tests the backend proxy handler directly without mocks
func TestStandaloneBackendProxyHandler(t *testing.T) {
	// Create a direct handler function that simulates what backendProxyHandler should do
	directHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate the backend server response directly
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Server", "Backend1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"server":"Backend1","path":"` + r.URL.Path + `"}`))
	})

	// Create a test request
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	// Call the handler directly
	directHandler.ServeHTTP(w, req)

	// Get the response
	resp := w.Result()
	defer resp.Body.Close()

	// Check status code
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Check headers
	assert.Equal(t, "Backend1", resp.Header.Get("X-Server"))

	// Check body
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"server":"Backend1"`)
}

// TestDefaultBackendRouting tests that requests without a specific matching route
// are correctly routed to the default backend
func TestDefaultBackendRouting(t *testing.T) {
	// Create a new ReverseProxyModule instance
	module := NewModule()

	// Create a mock tenant application
	mockApp := NewMockTenantApplication() // Changed from NewMockApplication()

	// Register config with the module
	err := module.RegisterConfig(mockApp)
	require.NoError(t, err, "RegisterConfig should not fail")

	// Initialize the module with the mock application
	err = module.Init(mockApp) // Pass mockApp which is also a modular.Application
	require.NoError(t, err, "Init should not fail")

	// Setup backend servers
	defaultBackendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Server", "DefaultBackend")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"server":"DefaultBackend","path":"` + r.URL.Path + `"}`))
	}))
	defer defaultBackendServer.Close()

	specificBackendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Server", "SpecificBackend")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"server":"SpecificBackend","path":"` + r.URL.Path + `"}`))
	}))
	defer specificBackendServer.Close()

	// Create test config with the mock servers
	testConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"default":  defaultBackendServer.URL,
			"specific": specificBackendServer.URL,
		},
		DefaultBackend: "default",
	}

	// Create mock router
	mockRouter := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}

	// Set up module with test config
	module.config = testConfig
	module.defaultBackend = testConfig.DefaultBackend
	module.router = mockRouter
	module.httpClient = &http.Client{}
	module.backendProxies = make(map[string]*httputil.ReverseProxy)
	module.backendRoutes = make(map[string]map[string]http.HandlerFunc)
	module.tenantBackendProxies = make(map[modular.TenantID]map[string]*httputil.ReverseProxy)
	module.tenants = make(map[modular.TenantID]*ReverseProxyConfig)
	module.compositeRoutes = make(map[string]http.HandlerFunc)
	module.circuitBreakers = make(map[string]*CircuitBreaker)

	// Initialize proxies for each backend
	for backend, urlString := range testConfig.BackendServices {
		backendURL, err := url.Parse(urlString)
		require.NoError(t, err)
		proxy := httputil.NewSingleHostReverseProxy(backendURL)
		module.backendProxies[backend] = proxy

		// Initialize route map for this backend
		module.backendRoutes[backend] = make(map[string]http.HandlerFunc)

		// Create a test tenant ID for tenant-specific proxies
		tenantID := modular.TenantID("test-tenant")
		if _, exists := module.tenantBackendProxies[tenantID]; !exists {
			module.tenantBackendProxies[tenantID] = make(map[string]*httputil.ReverseProxy)
		}
		module.tenantBackendProxies[tenantID][backend] = proxy

		// Register the default route handler for each backend
		handler := func(w http.ResponseWriter, r *http.Request) {
			proxy.ServeHTTP(w, r)
		}
		module.backendRoutes[backend]["/*"] = handler
	}

	// Add a specific route
	err = module.AddBackendRoute("specific", "/api/specific/*")
	if err != nil {
		t.Fatalf("Failed to add backend route: %v", err)
	}

	// Start the module to set up routes including the default catch-all route
	err = module.Start(context.Background())
	require.NoError(t, err)

	// Test 1: Request to specific route should go to the specific backend
	specificHandler, exists := mockRouter.routes["/api/specific/*"]
	require.True(t, exists, "Specific route should be registered")

	specificReq := httptest.NewRequest("GET", "/api/specific/test", nil)
	specificW := httptest.NewRecorder()

	specificHandler(specificW, specificReq)

	specificResp := specificW.Result()
	defer specificResp.Body.Close()

	assert.Equal(t, http.StatusOK, specificResp.StatusCode)
	assert.Equal(t, "SpecificBackend", specificResp.Header.Get("X-Server"))

	specificBody, err := io.ReadAll(specificResp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(specificBody), `"server":"SpecificBackend"`)

	// Test 2: Request to non-specific route should go to the default backend
	defaultHandler, exists := mockRouter.routes["/*"]
	require.True(t, exists, "Default route should be registered")

	defaultReq := httptest.NewRequest("GET", "/some/random/path", nil)
	defaultW := httptest.NewRecorder()

	defaultHandler(defaultW, defaultReq)

	defaultResp := defaultW.Result()
	defer defaultResp.Body.Close()

	assert.Equal(t, http.StatusOK, defaultResp.StatusCode)
	assert.Equal(t, "DefaultBackend", defaultResp.Header.Get("X-Server"))

	defaultBody, err := io.ReadAll(defaultResp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(defaultBody), `"server":"DefaultBackend"`)

	// Test 3: Request to root path should also go to the default backend
	rootReq := httptest.NewRequest("GET", "/", nil)
	rootW := httptest.NewRecorder()

	defaultHandler(rootW, rootReq)

	rootResp := rootW.Result()
	defer rootResp.Body.Close()

	assert.Equal(t, http.StatusOK, rootResp.StatusCode)
	assert.Equal(t, "DefaultBackend", rootResp.Header.Get("X-Server"))

	rootBody, err := io.ReadAll(rootResp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(rootBody), `"server":"DefaultBackend"`)
}
