package reverseproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModule(t *testing.T) {
	// Test module creation
	module := NewModule()

	// Assertions
	assert.NotNil(t, module)
	assert.Equal(t, "reverseproxy", module.Name())
}

func TestModule_Init(t *testing.T) {
	// Create a new module
	module := NewModule()

	// Create config directly
	testConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api1": "http://api1.example.com",
			"api2": "http://api2.example.com",
		},
		DefaultBackend: "api1",
		TenantIDHeader: "X-Tenant-ID",
	}

	// Directly set the config for testing
	module.config = testConfig
	module.defaultBackend = testConfig.DefaultBackend

	// Verify the module was configured properly
	assert.NotNil(t, module.config)
	assert.Equal(t, "api1", module.defaultBackend)
	assert.Equal(t, "http://api1.example.com", module.config.BackendServices["api1"])
	assert.Equal(t, "http://api2.example.com", module.config.BackendServices["api2"])
}

func TestModule_Start(t *testing.T) {
	// Setup
	module := NewModule()

	// Create test config
	testConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api1": "http://api1.example.com",
			"api2": "http://api2.example.com",
		},
		DefaultBackend: "api1",
		CompositeRoutes: map[string]CompositeRoute{
			"/api/test": {
				Pattern:  "/api/test",
				Backends: []string{"api1", "api2"},
				Strategy: "merge",
			},
		},
	}

	// Create a mock app with our test config
	mockApp := NewMockTenantApplication()
	mockApp.configSections["reverseproxy"] = &mockConfigProvider{
		config: testConfig,
	}

	// Create a test router
	mockRouter := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}

	// Initialize module
	err := module.RegisterConfig(mockApp)
	require.NoError(t, err)

	// Directly set config and routes
	module.config = testConfig
	module.defaultBackend = testConfig.DefaultBackend

	// Set up backend routes manually
	module.backendProxies = make(map[string]*httputil.ReverseProxy)
	for backend, urlString := range testConfig.BackendServices {
		backendURL, urlErr := url.Parse(urlString)
		require.NoError(t, urlErr)

		proxy := httputil.NewSingleHostReverseProxy(backendURL)
		module.backendProxies[backend] = proxy

		if _, ok := module.backendRoutes[backend]; !ok {
			module.backendRoutes[backend] = make(map[string]http.HandlerFunc)
		}

		// Add a simple handler for the catch-all route
		module.backendRoutes[backend]["/*"] = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}
	}

	// Setup composite route manually
	module.compositeRoutes = make(map[string]http.HandlerFunc)
	module.compositeRoutes["/api/test"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	module.router = mockRouter

	// Test Start
	err = module.Start(context.Background())
	require.NoError(t, err)

	// Verify routes were registered
	assert.NotEmpty(t, mockRouter.routes, "Should register routes with the router")

	// Verify the composite route was registered
	_, exists := mockRouter.routes["/api/test"]
	assert.True(t, exists, "Composite route should be registered")

	// Verify the default backend routes were registered
	_, exists = mockRouter.routes["/*"]
	assert.True(t, exists, "Default backend route should be registered")
}

func TestModule_Stop(t *testing.T) {
	// Setup
	module := NewModule()

	// Test Stop
	err := module.Stop(context.Background())
	assert.NoError(t, err)
	// Stop is currently a no-op, so there's not much to test here
}

func TestProvideConfig(t *testing.T) {
	// Test that ProvideConfig returns a valid config
	config := ProvideConfig()
	assert.NotNil(t, config)

	// Check that it's the right type
	_, ok := config.(*ReverseProxyConfig)
	assert.True(t, ok)
}

func TestOnTenantRegistered(t *testing.T) {
	// Setup
	module := NewModule()

	mockApp := NewMockTenantApplication()
	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api1": "http://tenant1-api1.example.com",
		},
	}
	tenantID := modular.TenantID("tenant1")

	// Register tenant config
	err := mockApp.RegisterTenant(tenantID, map[string]modular.ConfigProvider{
		"reverseproxy": NewStdConfigProvider(tenantConfig),
	})
	require.NoError(t, err)

	err = module.RegisterConfig(mockApp)
	require.NoError(t, err)

	// Test tenant registration
	module.OnTenantRegistered(tenantID)

	// Verify tenant was registered
	_, exists := module.tenants[tenantID]
	assert.True(t, exists)
}

func TestOnTenantRemoved(t *testing.T) {
	// Setup
	module := NewModule()

	mockApp := NewMockTenantApplication()

	err := module.RegisterConfig(mockApp)
	require.NoError(t, err)

	// Register tenant first
	tenantID := modular.TenantID("tenant1")
	module.tenants[tenantID] = &ReverseProxyConfig{}

	// Test tenant removal
	module.OnTenantRemoved(tenantID)

	// Verify tenant was removed
	_, exists := module.tenants[tenantID]
	assert.False(t, exists)
}

// TestRegisterCustomEndpoint tests the RegisterCustomEndpoint method
// with mock HTTP servers to simulate actual backend services.
func TestRegisterCustomEndpoint(t *testing.T) {
	// Setup mock backend servers
	service1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if headers were forwarded correctly
		authHeader := r.Header.Get("Authorization")
		customHeader := r.Header.Get("X-Custom-Header")

		// Check the request path
		switch r.URL.Path {
		case "/api/data":
			w.Header().Set("Content-Type", "application/json")
			// Include received headers in the response for verification
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"service":"service1","data":{"id":123,"name":"Test Item"},"received_headers":{"auth":"%s","custom":"%s"}}`, authHeader, customHeader)
		case "/api/timeout":
			// Simulate a timeout
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusGatewayTimeout)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer service1.Close()

	service2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check the request method
		if r.Method != r.Header.Get("X-Expected-Method") {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Check the request path
		switch r.URL.Path {
		case "/api/more-data":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"service":"service2","metadata":{"tags":["important","featured"],"views":1024}}`))
		case "/api/error":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"Internal server error"}`))
		case "/api/redirect":
			// Test handling of redirects
			w.Header().Set("Location", "/api/more-data")
			w.WriteHeader(http.StatusTemporaryRedirect)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer service2.Close()

	// Setup
	module := NewModule()

	// Create test config with mock server URLs
	testConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"service1": service1.URL,
			"service2": service2.URL,
		},
		DefaultBackend: "service1",
		TenantIDHeader: "X-Tenant-ID",
	}

	// Create mock app and router
	mockApp := NewMockTenantApplication()
	module.app = mockApp
	mockRouter := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}

	// Setup the module with test config and real HTTP client
	module.config = testConfig
	module.router = mockRouter
	module.httpClient = &http.Client{
		Timeout: 100 * time.Millisecond, // Short timeout to test timeout handling
	}
	module.backendProxies = make(map[string]*httputil.ReverseProxy)

	// Setup proxy for each mock backend
	for backend, urlString := range testConfig.BackendServices {
		backendURL, err := url.Parse(urlString)
		require.NoError(t, err)
		module.backendProxies[backend] = httputil.NewSingleHostReverseProxy(backendURL)
	}

	// Track the number of times the transformer is called
	transformerCallCount := 0

	// Track responses received by the transformer
	var capturedResponses map[string]*http.Response

	// Create a response transformer function that captures and validates the responses
	customTransformer := func(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*CompositeResponse, error) {
		transformerCallCount++
		capturedResponses = responses

		// Verify we received responses from both backends
		require.Contains(t, responses, "service1", "Should receive response from service1")
		require.Contains(t, responses, "service2", "Should receive response from service2")

		// Create a combined response
		service1Body, err := io.ReadAll(responses["service1"].Body)
		require.NoError(t, err)

		service2Body, err := io.ReadAll(responses["service2"].Body)
		require.NoError(t, err)

		// Parse the JSON responses
		var service1Data, service2Data map[string]interface{}
		err = json.Unmarshal(service1Body, &service1Data)
		require.NoError(t, err)

		err = json.Unmarshal(service2Body, &service2Data)
		require.NoError(t, err)

		// Combine the responses
		combined := map[string]interface{}{
			"service1":  service1Data,
			"service2":  service2Data,
			"timestamp": time.Now().Unix(),
		}

		combinedJSON, err := json.Marshal(combined)
		require.NoError(t, err)

		return &CompositeResponse{
			StatusCode: http.StatusOK,
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
			Body:       combinedJSON,
		}, nil
	}

	// Create endpoint mapping for successful case
	successMapping := EndpointMapping{
		Endpoints: []BackendEndpointRequest{
			{
				Backend: "service1",
				Path:    "/api/data",
				Method:  "GET",
				Headers: map[string]string{
					"X-Custom-Header": "test-value",
				},
			},
			{
				Backend: "service2",
				Path:    "/api/more-data",
				Method:  "GET",
				Headers: map[string]string{
					"X-Expected-Method": "GET", // Used by our mock server to validate
				},
			},
		},
		ResponseTransformer: customTransformer,
	}

	// Test registering a custom endpoint
	testPattern := "/api/custom"
	module.RegisterCustomEndpoint(testPattern, successMapping)

	// Verify that the endpoint was registered
	assert.Len(t, module.compositeRoutes, 1, "Should have registered one route")

	handler, exists := module.compositeRoutes[testPattern]
	assert.True(t, exists, "Custom endpoint handler should be registered")
	assert.NotNil(t, handler, "Custom endpoint handler should not be nil")

	// Test the registered handler with a successful request
	req := httptest.NewRequest("GET", "http://example.com"+testPattern, nil)
	req.Header.Set("Authorization", "Bearer test-token") // Add auth header to test header forwarding
	w := httptest.NewRecorder()

	// Call the handler
	handler(w, req)

	// Verify the transformer was called
	assert.Equal(t, 1, transformerCallCount, "Response transformer should have been called once")

	// Check the transformer received the expected responses
	assert.Equal(t, http.StatusOK, capturedResponses["service1"].StatusCode)
	assert.Equal(t, http.StatusOK, capturedResponses["service2"].StatusCode)

	// Check the response returned to the client
	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return status code from transformer")
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "Should set Content-Type header")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Parse the response body
	var responseData map[string]interface{}
	err = json.Unmarshal(body, &responseData)
	require.NoError(t, err)

	// Verify the combined response contains data from both services
	assert.Contains(t, responseData, "service1")
	assert.Contains(t, responseData, "service2")
	assert.Contains(t, responseData, "timestamp")

	// Verify the response data contains the expected fields from both services
	service1Response := responseData["service1"].(map[string]interface{})
	assert.Equal(t, "service1", service1Response["service"])

	// Verify that headers were correctly forwarded
	receivedHeaders := service1Response["received_headers"].(map[string]interface{})
	assert.Equal(t, "Bearer test-token", receivedHeaders["auth"])
	assert.Equal(t, "test-value", receivedHeaders["custom"])

	service2Response := responseData["service2"].(map[string]interface{})
	assert.Equal(t, "service2", service2Response["service"])

	// Reset tracking variables for error test case
	transformerCallCount = 0
	capturedResponses = nil

	// Create a mapping that includes an endpoint that will return an error
	errorMapping := EndpointMapping{
		Endpoints: []BackendEndpointRequest{
			{
				Backend: "service1",
				Path:    "/api/data",
				Method:  "GET",
			},
			{
				Backend: "service2",
				Path:    "/api/error",
				Method:  "GET",
				Headers: map[string]string{
					"X-Expected-Method": "GET", // Used by our mock server to validate
				},
			},
		},
		ResponseTransformer: customTransformer,
	}

	// Register the error endpoint
	errorPattern := "/api/error-test"
	module.RegisterCustomEndpoint(errorPattern, errorMapping)

	// Get the handler
	errorHandler, exists := module.compositeRoutes[errorPattern]
	assert.True(t, exists, "Error endpoint handler should be registered")

	// Test the error handler
	errReq := httptest.NewRequest("GET", "http://example.com"+errorPattern, nil)
	errW := httptest.NewRecorder()

	// Call the handler
	errorHandler(errW, errReq)

	// Verify the transformer was called
	assert.Equal(t, 1, transformerCallCount, "Response transformer should have been called once")

	// Check the transformer received responses with expected status codes
	assert.Equal(t, http.StatusOK, capturedResponses["service1"].StatusCode)
	assert.Equal(t, http.StatusInternalServerError, capturedResponses["service2"].StatusCode)

	// Test timeout behavior
	transformerCallCount = 0
	capturedResponses = nil

	// Create a mapping that includes an endpoint that will timeout
	timeoutMapping := EndpointMapping{
		Endpoints: []BackendEndpointRequest{
			{
				Backend: "service1",
				Path:    "/api/timeout",
				Method:  "GET",
			},
			{
				Backend: "service2",
				Path:    "/api/more-data",
				Method:  "GET",
				Headers: map[string]string{
					"X-Expected-Method": "GET",
				},
			},
		},
		ResponseTransformer: func(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*CompositeResponse, error) {
			transformerCallCount++
			capturedResponses = responses

			// Check that service1 is missing or has a timeout error
			_, hasService1 := responses["service1"]
			if hasService1 {
				assert.Equal(t, http.StatusGatewayTimeout, responses["service1"].StatusCode)
			}

			// Check that service2 succeeded
			assert.Contains(t, responses, "service2")
			assert.Equal(t, http.StatusOK, responses["service2"].StatusCode)

			// Return only the successful response
			service2Body, err := io.ReadAll(responses["service2"].Body)
			require.NoError(t, err)

			return &CompositeResponse{
				StatusCode: http.StatusPartialContent,
				Headers:    http.Header{"Content-Type": []string{"application/json"}},
				Body:       service2Body,
			}, nil
		},
	}

	// Register the timeout endpoint
	timeoutPattern := "/api/timeout-test"
	module.RegisterCustomEndpoint(timeoutPattern, timeoutMapping)

	timeoutHandler, exists := module.compositeRoutes[timeoutPattern]
	assert.True(t, exists)

	// Test the timeout handler
	timeoutReq := httptest.NewRequest("GET", "http://example.com"+timeoutPattern, nil)
	timeoutW := httptest.NewRecorder()

	// Call the handler
	timeoutHandler(timeoutW, timeoutReq)

	// Verify transformer was called
	assert.Equal(t, 1, transformerCallCount)

	// Check that the partial response was returned
	timeoutResp := timeoutW.Result()
	assert.Equal(t, http.StatusPartialContent, timeoutResp.StatusCode)

	// Test redirect handling
	transformerCallCount = 0
	capturedResponses = nil

	// Create a mapping that includes an endpoint that will redirect
	redirectMapping := EndpointMapping{
		Endpoints: []BackendEndpointRequest{
			{
				Backend: "service1",
				Path:    "/api/data",
				Method:  "GET",
			},
			{
				Backend: "service2",
				Path:    "/api/redirect",
				Method:  "GET",
				Headers: map[string]string{
					"X-Expected-Method": "GET",
				},
			},
		},
		ResponseTransformer: customTransformer,
	}

	// Register the redirect endpoint
	redirectPattern := "/api/redirect-test"
	module.RegisterCustomEndpoint(redirectPattern, redirectMapping)

	redirectHandler, exists := module.compositeRoutes[redirectPattern]
	assert.True(t, exists)

	// Test the redirect handler
	redirectReq := httptest.NewRequest("GET", "http://example.com"+redirectPattern, nil)
	redirectW := httptest.NewRecorder()

	// Call the handler
	redirectHandler(redirectW, redirectReq)

	// Verify transformer was called
	assert.Equal(t, 1, transformerCallCount)

	// Test tenant awareness by registering a tenant-specific config
	// Create a tenant mock server
	tenantService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/data" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"service":"tenant-service","data":{"tenant_id":"test-tenant"}}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer tenantService.Close()

	// Register tenant configuration
	tenantID := modular.TenantID("test-tenant")
	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"service1": tenantService.URL,
		},
	}
	module.tenants[tenantID] = tenantConfig

	// Create tenant-specific proxies
	tenantProxies := make(map[modular.TenantID]*httputil.ReverseProxy)
	tenantURL, err := url.Parse(tenantService.URL)
	require.NoError(t, err)
	tenantProxies[tenantID] = httputil.NewSingleHostReverseProxy(tenantURL)
	module.tenantBackendProxies = make(map[modular.TenantID]map[string]*httputil.ReverseProxy)

	// Initialize tenant proxies
	if _, exists := module.tenantBackendProxies[tenantID]; !exists {
		module.tenantBackendProxies[tenantID] = make(map[string]*httputil.ReverseProxy)
	}
	module.tenantBackendProxies[tenantID]["service1"] = httputil.NewSingleHostReverseProxy(tenantURL)

	// Reset tracking variables for tenant test
	transformerCallCount = 0
	capturedResponses = nil

	// Make a request with a tenant header
	tenantReq := httptest.NewRequest("GET", "http://example.com"+testPattern, nil)
	tenantReq.Header.Set("X-Tenant-ID", string(tenantID))
	tenantW := httptest.NewRecorder()

	// Call the handler
	handler(tenantW, tenantReq)

	// Verify the transformer was called
	assert.Equal(t, 1, transformerCallCount, "Response transformer should have been called with tenant header")

	// Check the tenant-specific response
	tenantResp := capturedResponses["service1"]
	assert.NotNil(t, tenantResp, "Should receive response from tenant-specific service")
}

// TestAddBackendRoute verifies that the AddBackendRoute method correctly registers
// a dedicated route for a specific backend and properly handles tenant-specific configurations
func TestAddBackendRoute(t *testing.T) {
	// Setup mock backend servers
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"default-backend","path":"` + r.URL.Path + `"}`))
	}))
	defer backendServer.Close()

	// Setup tenant-specific backend server
	tenantBackendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"tenant-backend","path":"` + r.URL.Path + `"}`))
	}))
	defer tenantBackendServer.Close()

	// Create module
	module := NewModule()

	// Create test config with the mock server
	testConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"twitter": backendServer.URL,
			"github":  backendServer.URL + "/github", // Different path prefix
		},
		DefaultBackend: "twitter",
		TenantIDHeader: "X-Tenant-ID",
	}

	// Create mock app and router
	mockApp := NewMockTenantApplication()
	module.app = mockApp
	mockRouter := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}

	// Set up module with test config
	module.config = testConfig
	module.router = mockRouter
	module.httpClient = &http.Client{Timeout: 100 * time.Millisecond}
	module.backendProxies = make(map[string]*httputil.ReverseProxy)
	module.backendRoutes = make(map[string]map[string]http.HandlerFunc)
	module.compositeRoutes = make(map[string]http.HandlerFunc)

	// Set up backend proxies
	// This is the key part that was missing: We need to initialize the proxies before calling AddBackendRoute
	twitterURL, err := url.Parse(backendServer.URL)
	require.NoError(t, err)
	module.backendProxies["twitter"] = httputil.NewSingleHostReverseProxy(twitterURL)

	githubURL, err := url.Parse(backendServer.URL + "/github")
	require.NoError(t, err)
	module.backendProxies["github"] = httputil.NewSingleHostReverseProxy(githubURL)

	// Initialize tenant maps
	tenantID := modular.TenantID("test-tenant")
	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"twitter": tenantBackendServer.URL,
		},
	}
	module.tenants = make(map[modular.TenantID]*ReverseProxyConfig)
	module.tenants[tenantID] = tenantConfig
	module.tenantBackendProxies = make(map[modular.TenantID]map[string]*httputil.ReverseProxy)

	// Initialize tenant proxies
	tenantTwitterURL, err := url.Parse(tenantBackendServer.URL)
	require.NoError(t, err)

	// Create map for this tenant if it doesn't exist
	if _, exists := module.tenantBackendProxies[tenantID]; !exists {
		module.tenantBackendProxies[tenantID] = make(map[string]*httputil.ReverseProxy)
	}

	// Add the tenant-specific proxy
	module.tenantBackendProxies[tenantID]["twitter"] = httputil.NewSingleHostReverseProxy(tenantTwitterURL)

	// Test 1: Add a route for the Twitter backend
	twitterPattern := "/api/twitter/*"
	err = module.AddBackendRoute("twitter", twitterPattern)
	require.NoError(t, err)

	// Verify that the route was registered
	handler, exists := mockRouter.routes[twitterPattern]
	assert.True(t, exists, "Twitter route pattern should be registered")
	require.NotNil(t, handler, "Handler should not be nil")

	// Test the handler with a direct request
	req := httptest.NewRequest("GET", "http://example.com/api/twitter/users/12345", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	// Verify response
	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Check that we're getting the expected response
	var responseData map[string]interface{}
	err = json.Unmarshal(body, &responseData)
	require.NoError(t, err)
	assert.Equal(t, "default-backend", responseData["service"])

	// Test 2: Add a route for the GitHub backend
	githubPattern := "/api/github/*"
	err = module.AddBackendRoute("github", githubPattern)
	require.NoError(t, err)

	// Verify that the route was registered
	githubHandler, githubExists := mockRouter.routes[githubPattern]
	assert.True(t, githubExists, "GitHub route pattern should be registered")
	require.NotNil(t, githubHandler, "GitHub handler should not be nil")

	// Test 3: Test with tenant header to verify tenant-specific routing
	if handler != nil { // Only proceed if handler exists
		tenantReq := httptest.NewRequest("GET", "http://example.com/api/twitter/users/12345", nil)
		tenantReq.Header.Set("X-Tenant-ID", string(tenantID))
		tenantW := httptest.NewRecorder()

		handler(tenantW, tenantReq)

		// Verify tenant response
		tenantResp := tenantW.Result()
		assert.Equal(t, http.StatusOK, tenantResp.StatusCode)
		tenantBody, err := io.ReadAll(tenantResp.Body)
		require.NoError(t, err)

		var tenantResponseData map[string]interface{}
		err = json.Unmarshal(tenantBody, &tenantResponseData)
		require.NoError(t, err)

		// The response should now come from the tenant-specific backend
		assert.Equal(t, "tenant-backend", tenantResponseData["service"])
	}

	// Test 4: Test with a non-existent backend
	err = module.AddBackendRoute("nonexistent", "/api/nonexistent/*")
	require.Error(t, err, "AddBackendRoute should return an error for non-existent backend")

	// This should log an error but not panic, and no route should be registered
	_, nonexistentExists := mockRouter.routes["/api/nonexistent/*"]
	assert.False(t, nonexistentExists, "No route should be registered for non-existent backend")

	// Test 5: Test with invalid URL
	invalidConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"invalid": "://invalid-url",
		},
	}
	module.config = invalidConfig

	// This should log an error but not panic
	err = module.AddBackendRoute("invalid", "/api/invalid/*")
	require.Error(t, err, "AddBackendRoute should return an error for invalid URL")
	_, invalidExists := mockRouter.routes["/api/invalid/*"]
	assert.False(t, invalidExists, "No route should be registered for invalid URL")
}

// TestTenantConfigMerging tests that tenant-specific configurations are properly merged with global config
func TestTenantConfigMerging(t *testing.T) {
	// Setup
	module := NewModule()

	// Create a global config with multiple backend services
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"legacy":  "http://legacy-global.example.com",
			"chimera": "http://chimera-global.example.com",
		},
		DefaultBackend:  "chimera",
		TenantIDHeader:  "X-Tenant-ID",
		RequireTenantID: false,
		CacheEnabled:    true,
		CacheTTL:        120 * time.Second,
	}

	// Create mock app with global config
	mockApp := NewMockTenantApplication()
	mockApp.configSections["reverseproxy"] = &mockConfigProvider{
		config: globalConfig,
	}

	// Set up tenant configurations
	tenant1ID := modular.TenantID("tenant1")
	tenant1Config := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"legacy": "http://legacy-tenant1.example.com", // Override legacy
			// chimera not specified, should inherit from global
		},
		DefaultBackend: "legacy", // Override default backend
	}

	// Register tenant config
	err := mockApp.RegisterTenant(tenant1ID, map[string]modular.ConfigProvider{
		"reverseproxy": NewStdConfigProvider(tenant1Config),
	})
	require.NoError(t, err)

	// Initialize module
	err = module.RegisterConfig(mockApp)
	require.NoError(t, err)
	module.config = globalConfig // Set global config directly

	// Register tenant
	module.OnTenantRegistered(tenant1ID)

	// Load tenant configs - this should merge them with global config
	module.loadTenantConfigs()

	// Verify tenant config was properly merged with global config
	tenantCfg, exists := module.tenants[tenant1ID]
	assert.True(t, exists, "Tenant configuration should exist")
	assert.NotNil(t, tenantCfg, "Tenant configuration should not be nil")

	// Check that the tenant config has both services: the overridden legacy and inherited chimera
	assert.Equal(t, "http://legacy-tenant1.example.com", tenantCfg.BackendServices["legacy"],
		"Legacy backend should be overridden in tenant config")
	assert.Equal(t, "http://chimera-global.example.com", tenantCfg.BackendServices["chimera"],
		"Chimera backend should be inherited from global config")

	// Check that the tenant's default backend was overridden
	assert.Equal(t, "legacy", tenantCfg.DefaultBackend, "Default backend should be overridden")

	// Check that other settings were inherited from global config
	assert.Equal(t, "X-Tenant-ID", tenantCfg.TenantIDHeader, "TenantIDHeader should be inherited")
	assert.False(t, tenantCfg.RequireTenantID, "RequireTenantID should be inherited")
	assert.True(t, tenantCfg.CacheEnabled, "CacheEnabled should be inherited")
	assert.Equal(t, 120*time.Second, tenantCfg.CacheTTL, "CacheTTL should be inherited")

	// Test with a second tenant that has more comprehensive overrides
	tenant2ID := modular.TenantID("tenant2")
	tenant2Config := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"legacy":      "http://legacy-tenant2.example.com",
			"tenant-only": "http://tenant2-specific.example.com", // Tenant-specific service
		},
		TenantIDHeader:  "X-Custom-Tenant-Header", // Override header
		RequireTenantID: true,                     // Override requirement
		CacheEnabled:    true,                     // Same as global but explicitly set
		CacheTTL:        60 * time.Second,         // Override TTL
	}

	// Register second tenant
	err = mockApp.RegisterTenant(tenant2ID, map[string]modular.ConfigProvider{
		"reverseproxy": NewStdConfigProvider(tenant2Config),
	})
	require.NoError(t, err)

	// Register and load second tenant
	module.OnTenantRegistered(tenant2ID)
	module.loadTenantConfigs()

	// Verify second tenant's config was properly merged
	tenant2Cfg, exists := module.tenants[tenant2ID]
	assert.True(t, exists)
	assert.NotNil(t, tenant2Cfg)

	// Check services - should have both global and tenant-specific ones
	assert.Len(t, tenant2Cfg.BackendServices, 3, "Should have 3 backend services")
	assert.Equal(t, "http://legacy-tenant2.example.com", tenant2Cfg.BackendServices["legacy"])
	assert.Equal(t, "http://chimera-global.example.com", tenant2Cfg.BackendServices["chimera"])
	assert.Equal(t, "http://tenant2-specific.example.com", tenant2Cfg.BackendServices["tenant-only"])

	// Check that overridden settings were applied
	assert.Equal(t, "X-Custom-Tenant-Header", tenant2Cfg.TenantIDHeader)
	assert.True(t, tenant2Cfg.RequireTenantID)
	assert.Equal(t, 60*time.Second, tenant2Cfg.CacheTTL, "CacheTTL should be overridden to 60")

	// Skip the proxy testing part which causes network errors in the test environment
	// This part of the test was just to validate that the configuration is properly used,
	// but we've already verified the configuration merging
}

// Simple test router implementation for tests
type testRouter struct {
	mu     sync.RWMutex
	routes map[string]http.HandlerFunc
}

func (tr *testRouter) Handle(pattern string, handler http.Handler) {
	if tr == nil {
		panic("testRouter is nil")
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.routes == nil {
		tr.routes = make(map[string]http.HandlerFunc)
	}
	if handler == nil {
		panic(fmt.Sprintf("handler is nil for pattern '%s'", pattern))
	}
	tr.routes[pattern] = handler.ServeHTTP
}

func (tr *testRouter) HandleFunc(pattern string, handler http.HandlerFunc) {
	if tr == nil {
		panic("testRouter is nil")
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.routes == nil {
		tr.routes = make(map[string]http.HandlerFunc)
	}
	if handler == nil {
		panic(fmt.Sprintf("handler is nil for pattern '%s'", pattern))
	}
	tr.routes[pattern] = handler
}

func (tr *testRouter) Mount(pattern string, h http.Handler) {
	if tr == nil {
		panic("testRouter is nil")
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.routes == nil {
		tr.routes = make(map[string]http.HandlerFunc)
	}
	if h == nil {
		panic(fmt.Sprintf("handler is nil for pattern '%s'", pattern))
	}
	tr.routes[pattern] = h.ServeHTTP
}

func (tr *testRouter) Use(middlewares ...func(http.Handler) http.Handler) {
	// No-op for test router
}

func (tr *testRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if tr == nil {
		panic("testRouter is nil")
	}
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	if tr.routes == nil {
		tr.routes = make(map[string]http.HandlerFunc)
	}

	// Try exact path match first
	if handler, ok := tr.routes[r.URL.Path]; ok {
		handler(w, r)
		return
	}

	// Try wildcard pattern matching - use deterministic order
	var patterns []string
	for pattern := range tr.routes {
		if strings.HasSuffix(pattern, "/*") {
			patterns = append(patterns, pattern)
		}
	}
	// Sort patterns to ensure deterministic matching (longer patterns first for more specific matches)
	sort.Slice(patterns, func(i, j int) bool {
		return len(patterns[i]) > len(patterns[j])
	})

	for _, pattern := range patterns {
		handler := tr.routes[pattern]
		prefix := strings.TrimSuffix(pattern, "/*")
		if strings.HasPrefix(r.URL.Path, prefix+"/") || r.URL.Path == prefix {
			handler(w, r)
			return
		}
	}

	// Try global wildcard
	if handler, ok := tr.routes["/*"]; ok {
		handler(w, r)
		return
	}

	http.NotFound(w, r)
}

// TestTenantSpecificBackendProxiesCreated tests that tenant-specific backend proxies
// are created when a tenant configuration overrides a backend service URL.
// This reproduces GitHub issue #111.
//
// The bug occurs in the standard startup flow when using FileBasedTenantConfigLoader:
//  1. Init() runs - no tenants yet (they haven't been loaded from files)
//  2. Framework loads tenant configs - calls OnTenantRegistered() for each tenant
//  3. Start() runs - should create tenant proxies but was NOT doing so (BUG)
//
// This test validates that Start() now properly creates tenant proxies for tenants
// that were registered after Init() but before Start() - which is the standard flow
// for FileBasedTenantConfigLoader and affects all users of that loader.
func TestTenantSpecificBackendProxiesCreated(t *testing.T) {
	// Setup mock backend servers
	globalBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"global"}`))
	}))
	defer globalBackend.Close()

	acmeBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"acme"}`))
	}))
	defer acmeBackend.Close()

	// Create module
	module := NewModule()

	// Create global config with a backend service
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api": globalBackend.URL,
		},
		DefaultBackend:  "api",
		TenantIDHeader:  "X-Tenant-Id",
		RequireTenantID: true,
		Routes: map[string]string{
			"/api/test": "api",
		},
	}

	// Create mock app with global config
	mockApp := NewMockTenantApplication()
	mockApp.configSections["reverseproxy"] = &mockConfigProvider{
		config: globalConfig,
	}

	// Set up tenant configuration that overrides the "api" backend
	tenantID := modular.TenantID("acme")
	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api": acmeBackend.URL, // Override with tenant-specific URL
		},
	}

	// Register tenant config
	err := mockApp.RegisterTenant(tenantID, map[string]modular.ConfigProvider{
		"reverseproxy": NewStdConfigProvider(tenantConfig),
	})
	require.NoError(t, err)

	// Set up router service
	mockRouter := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}

	// Initialize proxy maps
	module.tenants = make(map[modular.TenantID]*ReverseProxyConfig)
	module.tenantBackendProxies = make(map[modular.TenantID]map[string]*httputil.ReverseProxy)
	module.backendProxies = make(map[string]*httputil.ReverseProxy)
	module.backendRoutes = make(map[string]map[string]http.HandlerFunc)
	module.compositeRoutes = make(map[string]http.HandlerFunc)
	module.router = mockRouter

	// Initialize module FIRST (without tenant registered yet)
	err = module.Init(mockApp)
	require.NoError(t, err)

	// NOW register tenant AFTER Init has run
	// This simulates the scenario where tenants are dynamically added
	module.OnTenantRegistered(tenantID)

	// Call Start() - this should load tenant configs and CREATE proxies
	// BUT this is where the BUG is - Start() loads configs but doesn't create proxies!
	err = module.Start(context.Background())
	require.NoError(t, err)

	// Verify tenant config was merged
	tenantCfg, exists := module.tenants[tenantID]
	require.True(t, exists, "Tenant config should exist")
	require.NotNil(t, tenantCfg, "Tenant config should not be nil")
	assert.Equal(t, acmeBackend.URL, tenantCfg.BackendServices["api"],
		"Tenant config should have the overridden backend URL")

	// THIS IS THE BUG: After Init(), tenant-specific proxies should be created
	// The test will verify whether they exist
	tenantProxies, tenantProxiesExist := module.tenantBackendProxies[tenantID]

	// THIS IS THE KEY ASSERTION - tenant proxies should exist after Init()
	require.True(t, tenantProxiesExist, "Tenant proxy map should exist after Init()")
	require.NotNil(t, tenantProxies, "Tenant proxies should not be nil")

	// The tenant-specific 'api' proxy should exist
	apiProxy, apiProxyExists := tenantProxies["api"]
	require.True(t, apiProxyExists, "Tenant-specific 'api' proxy should exist")
	require.NotNil(t, apiProxy, "Tenant-specific 'api' proxy should not be nil")

	// Now test the proxy resolution
	// Without tenant ID - should use global proxy
	globalProxy, exists := module.getProxyForBackendAndTenant("api", "")
	assert.True(t, exists, "Global proxy should exist")
	assert.NotNil(t, globalProxy, "Global proxy should not be nil")

	// With tenant ID - should use tenant-specific proxy
	tenantProxy, exists := module.getProxyForBackendAndTenant("api", tenantID)
	assert.True(t, exists, "Tenant proxy should exist after manual creation")
	assert.NotNil(t, tenantProxy, "Tenant proxy should not be nil")

	// The two proxies should be different (pointing to different backends)
	assert.NotEqual(t, globalProxy, tenantProxy,
		"Global and tenant proxies should be different instances")

	// Test actual requests
	// Create a test handler that uses the proxy resolution
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		tenantID := modular.TenantID(r.Header.Get("X-Tenant-Id"))
		proxy, exists := module.getProxyForBackendAndTenant("api", tenantID)
		if !exists {
			http.Error(w, "Backend not found", http.StatusNotFound)
			return
		}
		proxy.ServeHTTP(w, r)
	}

	// Test request without tenant ID (should route to global)
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	w := httptest.NewRecorder()
	testHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"backend":"global"`,
		"Request without tenant ID should route to global backend")

	// Test request with tenant ID (should route to tenant-specific backend)
	tenantReq := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	tenantReq.Header.Set("X-Tenant-Id", string(tenantID))
	tenantW := httptest.NewRecorder()
	testHandler(tenantW, tenantReq)

	tenantResp := tenantW.Result()
	assert.Equal(t, http.StatusOK, tenantResp.StatusCode)
	tenantBody, err := io.ReadAll(tenantResp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(tenantBody), `"backend":"acme"`,
		"Request with tenant ID should route to tenant-specific backend")
}

// TestMultipleTenantsSameBackendOverride tests the edge case where multiple tenants
// override the SAME backend ID with different URLs.
// This reproduces the edge case bug found in GitHub issue #111 comment.
//
// The bug: When multiple tenants override the same backend ID (e.g., "api") with
// different URLs, both tenants end up with proxies pointing to the same URL due to
// improper variable scoping/capture in the createTenantProxies() loop.
//
// Expected behavior: tenant1 requests → tenant1's backend, tenant2 requests → tenant2's backend
// Actual buggy behavior: Both tenant1 and tenant2 requests → same backend (last processed tenant's URL)
func TestMultipleTenantsSameBackendOverride(t *testing.T) {
	// Setup mock backend servers
	globalBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"global","url":"` + r.Host + `"}`))
	}))
	defer globalBackend.Close()

	tenant1Backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"tenant1","url":"` + r.Host + `"}`))
	}))
	defer tenant1Backend.Close()

	tenant2Backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"tenant2","url":"` + r.Host + `"}`))
	}))
	defer tenant2Backend.Close()

	// Create module
	module := NewModule()

	// Create global config with a backend service
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api": globalBackend.URL,
		},
		DefaultBackend:  "api",
		TenantIDHeader:  "X-Affiliate-Id",
		RequireTenantID: true,
		Routes: map[string]string{
			"/api/*": "api",
		},
	}

	// Create mock app with global config
	mockApp := NewMockTenantApplication()
	mockApp.configSections["reverseproxy"] = &mockConfigProvider{
		config: globalConfig,
	}

	// Set up tenant1 configuration that overrides the "api" backend
	tenant1ID := modular.TenantID("tenant1")
	tenant1Config := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api": tenant1Backend.URL, // Override with tenant1-specific URL
		},
	}

	// Register tenant1 config
	err := mockApp.RegisterTenant(tenant1ID, map[string]modular.ConfigProvider{
		"reverseproxy": NewStdConfigProvider(tenant1Config),
	})
	require.NoError(t, err)

	// Set up tenant2 configuration that ALSO overrides the "api" backend
	tenant2ID := modular.TenantID("tenant2")
	tenant2Config := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api": tenant2Backend.URL, // Override with tenant2-specific URL (DIFFERENT from tenant1)
		},
	}

	// Register tenant2 config
	err = mockApp.RegisterTenant(tenant2ID, map[string]modular.ConfigProvider{
		"reverseproxy": NewStdConfigProvider(tenant2Config),
	})
	require.NoError(t, err)

	// Set up router service
	mockRouter := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}

	// Initialize proxy maps
	module.tenants = make(map[modular.TenantID]*ReverseProxyConfig)
	module.tenantBackendProxies = make(map[modular.TenantID]map[string]*httputil.ReverseProxy)
	module.backendProxies = make(map[string]*httputil.ReverseProxy)
	module.backendRoutes = make(map[string]map[string]http.HandlerFunc)
	module.compositeRoutes = make(map[string]http.HandlerFunc)
	module.router = mockRouter

	// Initialize module FIRST
	err = module.Init(mockApp)
	require.NoError(t, err)

	// Register tenants AFTER Init (simulating FileBasedTenantConfigLoader flow)
	module.OnTenantRegistered(tenant1ID)
	module.OnTenantRegistered(tenant2ID)

	// Call Start() - this should create tenant-specific proxies for BOTH tenants
	err = module.Start(context.Background())
	require.NoError(t, err)

	// Log the tenant configs and proxy URLs for debugging
	t.Logf("Tenant1 config backend URL: %s", module.tenants[tenant1ID].BackendServices["api"])
	t.Logf("Tenant2 config backend URL: %s", module.tenants[tenant2ID].BackendServices["api"])

	// Verify tenant configs were merged
	tenant1Cfg, exists := module.tenants[tenant1ID]
	require.True(t, exists, "Tenant1 config should exist")
	require.NotNil(t, tenant1Cfg, "Tenant1 config should not be nil")
	assert.Equal(t, tenant1Backend.URL, tenant1Cfg.BackendServices["api"],
		"Tenant1 config should have tenant1's backend URL")

	tenant2Cfg, exists := module.tenants[tenant2ID]
	require.True(t, exists, "Tenant2 config should exist")
	require.NotNil(t, tenant2Cfg, "Tenant2 config should not be nil")
	assert.Equal(t, tenant2Backend.URL, tenant2Cfg.BackendServices["api"],
		"Tenant2 config should have tenant2's backend URL")

	// Verify tenant-specific proxies exist
	tenant1Proxies, exists := module.tenantBackendProxies[tenant1ID]
	require.True(t, exists, "Tenant1 proxy map should exist")
	require.NotNil(t, tenant1Proxies, "Tenant1 proxies should not be nil")

	tenant2Proxies, exists := module.tenantBackendProxies[tenant2ID]
	require.True(t, exists, "Tenant2 proxy map should exist")
	require.NotNil(t, tenant2Proxies, "Tenant2 proxies should not be nil")

	// Verify each tenant has an "api" proxy
	tenant1APIProxy, exists := tenant1Proxies["api"]
	require.True(t, exists, "Tenant1 'api' proxy should exist")
	require.NotNil(t, tenant1APIProxy, "Tenant1 'api' proxy should not be nil")

	tenant2APIProxy, exists := tenant2Proxies["api"]
	require.True(t, exists, "Tenant2 'api' proxy should exist")
	require.NotNil(t, tenant2APIProxy, "Tenant2 'api' proxy should not be nil")

	// Log proxy details for debugging
	if tenant1APIProxy != nil {
		t.Logf("Tenant1 proxy Director: %p", tenant1APIProxy)
	}
	if tenant2APIProxy != nil {
		t.Logf("Tenant2 proxy Director: %p", tenant2APIProxy)
	}

	// The two tenant proxies should be different instances
	assert.NotEqual(t, tenant1APIProxy, tenant2APIProxy,
		"Tenant1 and tenant2 proxies should be different instances")

	// Create a test handler that uses the actual routing logic from the module
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		tenantIDStr := r.Header.Get("X-Affiliate-Id")
		tenantID := modular.TenantID(tenantIDStr)

		proxy, exists := module.getProxyForBackendAndTenant("api", tenantID)
		if !exists {
			http.Error(w, "Backend not found", http.StatusNotFound)
			return
		}
		proxy.ServeHTTP(w, r)
	}

	// Test request with tenant1 header
	tenant1Req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	tenant1Req.Header.Set("X-Affiliate-Id", string(tenant1ID))
	tenant1W := httptest.NewRecorder()
	testHandler(tenant1W, tenant1Req)

	tenant1Resp := tenant1W.Result()
	assert.Equal(t, http.StatusOK, tenant1Resp.StatusCode)
	tenant1Body, err := io.ReadAll(tenant1Resp.Body)
	require.NoError(t, err)

	t.Logf("Tenant1 response: %s", string(tenant1Body))

	var tenant1Data map[string]interface{}
	err = json.Unmarshal(tenant1Body, &tenant1Data)
	require.NoError(t, err)

	// THIS IS THE KEY ASSERTION: Tenant1 should get tenant1's backend
	assert.Equal(t, "tenant1", tenant1Data["backend"],
		"Request with tenant1 ID should route to tenant1's backend")

	// Test request with tenant2 header
	tenant2Req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	tenant2Req.Header.Set("X-Affiliate-Id", string(tenant2ID))
	tenant2W := httptest.NewRecorder()
	testHandler(tenant2W, tenant2Req)

	tenant2Resp := tenant2W.Result()
	assert.Equal(t, http.StatusOK, tenant2Resp.StatusCode)
	tenant2Body, err := io.ReadAll(tenant2Resp.Body)
	require.NoError(t, err)

	t.Logf("Tenant2 response: %s", string(tenant2Body))

	var tenant2Data map[string]interface{}
	err = json.Unmarshal(tenant2Body, &tenant2Data)
	require.NoError(t, err)

	// THIS IS THE BUG CHECK: Tenant2 should get tenant2's backend, NOT tenant1's
	assert.Equal(t, "tenant2", tenant2Data["backend"],
		"Request with tenant2 ID should route to tenant2's backend (NOT tenant1's backend)")

	// If the bug exists, both tenants will route to the same backend (the last tenant processed)
	// The test will fail with a message like:
	// "Request with tenant1 ID should route to tenant1's backend" but got "tenant2"
}

// TestMultipleTenantsSameBackendOverride_Concurrent tests the edge case with concurrent
// requests to ensure proper proxy isolation under concurrent load.
// This runs the multi-tenant backend override test multiple times to catch race conditions.
func TestMultipleTenantsSameBackendOverride_Concurrent(t *testing.T) {
	const iterations = 20

	for i := 0; i < iterations; i++ {
		t.Run(fmt.Sprintf("Iteration_%d", i), func(t *testing.T) {
			t.Parallel() // Run iterations in parallel to stress test

			// Setup mock backend servers
			globalBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"backend":"global"}`))
			}))
			defer globalBackend.Close()

			tenant1Backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"backend":"tenant1"}`))
			}))
			defer tenant1Backend.Close()

			tenant2Backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"backend":"tenant2"}`))
			}))
			defer tenant2Backend.Close()

			tenant3Backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"backend":"tenant3"}`))
			}))
			defer tenant3Backend.Close()

			// Create module
			module := NewModule()

			// Create global config
			globalConfig := &ReverseProxyConfig{
				BackendServices: map[string]string{
					"api": globalBackend.URL,
				},
				DefaultBackend:  "api",
				TenantIDHeader:  "X-Affiliate-Id",
				RequireTenantID: true,
			}

			// Create mock app
			mockApp := NewMockTenantApplication()
			mockApp.configSections["reverseproxy"] = &mockConfigProvider{config: globalConfig}

			// Register 3 tenants with different backend URLs for the SAME backend ID
			tenants := []struct {
				id  modular.TenantID
				url string
			}{
				{"tenant1", tenant1Backend.URL},
				{"tenant2", tenant2Backend.URL},
				{"tenant3", tenant3Backend.URL},
			}

			for _, tenant := range tenants {
				err := mockApp.RegisterTenant(tenant.id, map[string]modular.ConfigProvider{
					"reverseproxy": NewStdConfigProvider(&ReverseProxyConfig{
						BackendServices: map[string]string{
							"api": tenant.url,
						},
					}),
				})
				require.NoError(t, err)
			}

			// Setup module
			mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}
			module.tenants = make(map[modular.TenantID]*ReverseProxyConfig)
			module.tenantBackendProxies = make(map[modular.TenantID]map[string]*httputil.ReverseProxy)
			module.backendProxies = make(map[string]*httputil.ReverseProxy)
			module.backendRoutes = make(map[string]map[string]http.HandlerFunc)
			module.compositeRoutes = make(map[string]http.HandlerFunc)
			module.router = mockRouter

			// Init, register tenants, and start
			err := module.Init(mockApp)
			require.NoError(t, err)

			for _, tenant := range tenants {
				module.OnTenantRegistered(tenant.id)
			}

			err = module.Start(context.Background())
			require.NoError(t, err)

			// Test each tenant concurrently
			var wg sync.WaitGroup
			for _, tenant := range tenants {
				wg.Add(1)
				go func(tid modular.TenantID, expectedBackend string) {
					defer wg.Done()

					// Make multiple requests per tenant
					for j := 0; j < 5; j++ {
						proxy, exists := module.getProxyForBackendAndTenant("api", tid)
						require.True(t, exists, "Proxy should exist for tenant %s", tid)
						require.NotNil(t, proxy, "Proxy should not be nil for tenant %s", tid)

						// Test actual request
						req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
						req.Header.Set("X-Affiliate-Id", string(tid))
						w := httptest.NewRecorder()

						proxy.ServeHTTP(w, req)

						resp := w.Result()
						body, err := io.ReadAll(resp.Body)
						require.NoError(t, err)

						var data map[string]interface{}
						err = json.Unmarshal(body, &data)
						require.NoError(t, err)

						// Critical assertion: Each tenant must get its own backend
						assert.Equal(t, expectedBackend, data["backend"],
							"Tenant %s request %d should route to %s backend", tid, j, expectedBackend)
					}
				}(tenant.id, string(tenant.id))
			}

			wg.Wait()
		})
	}
}

// TestSetupResponseCacheWithTenantOverrides tests that cache is initialized
// when any tenant has caching enabled, even if global caching is disabled
func TestSetupResponseCacheWithTenantOverrides(t *testing.T) {
	tests := []struct {
		name                     string
		globalCacheEnabled       bool
		tenantCacheEnabled       map[string]bool
		expectedCacheInitialized bool
	}{
		{
			name:                     "Global cache enabled, no tenants",
			globalCacheEnabled:       true,
			tenantCacheEnabled:       map[string]bool{},
			expectedCacheInitialized: true,
		},
		{
			name:                     "Global cache disabled, no tenants",
			globalCacheEnabled:       false,
			tenantCacheEnabled:       map[string]bool{},
			expectedCacheInitialized: false,
		},
		{
			name:               "Global cache disabled, one tenant has cache enabled",
			globalCacheEnabled: false,
			tenantCacheEnabled: map[string]bool{
				"tenant1": true,
			},
			expectedCacheInitialized: true,
		},
		{
			name:               "Global cache disabled, multiple tenants mixed settings",
			globalCacheEnabled: false,
			tenantCacheEnabled: map[string]bool{
				"tenant1": false,
				"tenant2": true,
				"tenant3": false,
			},
			expectedCacheInitialized: true,
		},
		{
			name:               "Global cache disabled, all tenants have cache disabled",
			globalCacheEnabled: false,
			tenantCacheEnabled: map[string]bool{
				"tenant1": false,
				"tenant2": false,
			},
			expectedCacheInitialized: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create module
			module := NewModule()

			// Set up global config
			module.config = &ReverseProxyConfig{
				CacheEnabled: tt.globalCacheEnabled,
				CacheTTL:     60 * time.Second,
			}

			// Set up tenant configs
			module.tenants = make(map[modular.TenantID]*ReverseProxyConfig)
			for tenantID, cacheEnabled := range tt.tenantCacheEnabled {
				module.tenants[modular.TenantID(tenantID)] = &ReverseProxyConfig{
					CacheEnabled: cacheEnabled,
					CacheTTL:     30 * time.Second,
				}
			}

			// Set app to nil - setupResponseCache handles nil app gracefully
			module.app = nil

			// Call setupResponseCache
			err := module.setupResponseCache()
			require.NoError(t, err)

			// Verify cache initialization
			if tt.expectedCacheInitialized {
				assert.NotNil(t, module.responseCache, "Cache should be initialized")
			} else {
				assert.Nil(t, module.responseCache, "Cache should not be initialized")
			}
		})
	}
}
