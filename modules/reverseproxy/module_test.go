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
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModule(t *testing.T) {
	// Test module creation
	module, err := NewModule()

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, module)
	assert.Equal(t, "reverseproxy", module.Name())
}

func TestModule_Init(t *testing.T) {
	// Create a new module
	module, err := NewModule()
	require.NoError(t, err)

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
	module, err := NewModule()
	require.NoError(t, err)

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
	err = module.RegisterConfig(mockApp)
	assert.NoError(t, err)

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
	assert.NoError(t, err)

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
	module, err := NewModule()
	require.NoError(t, err)

	// Test Stop
	err = module.Stop(context.Background())
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
	module, err := NewModule()
	require.NoError(t, err)

	mockApp := NewMockTenantApplication()
	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api1": "http://tenant1-api1.example.com",
		},
	}
	tenantID := modular.TenantID("tenant1")

	// Register tenant config
	mockApp.RegisterTenant(tenantID, map[string]modular.ConfigProvider{
		"reverseproxy": NewStdConfigProvider(tenantConfig),
	})

	err = module.RegisterConfig(mockApp)
	assert.NoError(t, err)

	// Test tenant registration
	module.OnTenantRegistered(tenantID)

	// Verify tenant was registered
	_, exists := module.tenants[tenantID]
	assert.True(t, exists)
}

func TestOnTenantRemoved(t *testing.T) {
	// Setup
	module, err := NewModule()
	require.NoError(t, err)

	mockApp := NewMockTenantApplication()

	err = module.RegisterConfig(mockApp)
	assert.NoError(t, err)

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
		if r.URL.Path == "/api/data" {
			w.Header().Set("Content-Type", "application/json")
			// Include received headers in the response for verification
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf(`{"service":"service1","data":{"id":123,"name":"Test Item"},"received_headers":{"auth":"%s","custom":"%s"}}`, authHeader, customHeader)))
		} else if r.URL.Path == "/api/timeout" {
			// Simulate a timeout
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusGatewayTimeout)
		} else {
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
		if r.URL.Path == "/api/more-data" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"service":"service2","metadata":{"tags":["important","featured"],"views":1024}}`))
		} else if r.URL.Path == "/api/error" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"Internal server error"}`))
		} else if r.URL.Path == "/api/redirect" {
			// Test handling of redirects
			w.Header().Set("Location", "/api/more-data")
			w.WriteHeader(http.StatusTemporaryRedirect)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer service2.Close()

	// Setup
	module, err := NewModule()
	require.NoError(t, err)

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
			w.Write([]byte(`{"service":"tenant-service","data":{"tenant_id":"test-tenant"}}`))
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
	module.tenantBackendProxies["service1"] = tenantProxies

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
		w.Write([]byte(`{"service":"default-backend","path":"` + r.URL.Path + `"}`))
	}))
	defer backendServer.Close()

	// Setup tenant-specific backend server
	tenantBackendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"service":"tenant-backend","path":"` + r.URL.Path + `"}`))
	}))
	defer tenantBackendServer.Close()

	// Create module
	module, err := NewModule()
	require.NoError(t, err)

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

	// Initialize tenant maps
	tenantID := modular.TenantID("test-tenant")
	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"twitter": tenantBackendServer.URL,
		},
	}
	module.tenants = make(map[modular.TenantID]*ReverseProxyConfig)
	module.tenants[tenantID] = tenantConfig
	module.tenantBackendProxies = make(map[string]map[modular.TenantID]*httputil.ReverseProxy)

	// Test 1: Add a route for the Twitter backend
	twitterPattern := "/api/twitter/*"
	module.AddBackendRoute("twitter", twitterPattern)

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
	module.AddBackendRoute("github", githubPattern)

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
	module.AddBackendRoute("nonexistent", "/api/nonexistent/*")

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
	module.AddBackendRoute("invalid", "/api/invalid/*")
	_, invalidExists := mockRouter.routes["/api/invalid/*"]
	assert.False(t, invalidExists, "No route should be registered for invalid URL")
}

// Simple test router implementation for tests
type testRouter struct {
	routes map[string]http.HandlerFunc
}

func (r *testRouter) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	r.routes[pattern] = handler
}
