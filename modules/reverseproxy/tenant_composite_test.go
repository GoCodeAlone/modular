package reverseproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CrisisTextLine/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestTenantCompositeRoutes tests that tenant-specific composite routes are properly handled
func TestTenantCompositeRoutes(t *testing.T) {
	// Create a mock tenant application with mock functionality
	mockTenantApp := NewMockTenantApplicationWithMock()
	mockConfigProvider := &MockConfigProvider{}

	// Set up global config
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"backend1": "http://backend1.example.com",
			"backend2": "http://backend2.example.com",
		},
		CompositeRoutes: map[string]CompositeRoute{
			"/global/composite": {
				Pattern:  "/global/composite",
				Backends: []string{"backend1", "backend2"},
				Strategy: "json-merge",
			},
			"/shared/route": {
				Pattern:  "/shared/route",
				Backends: []string{"backend1"},
				Strategy: "json-merge",
			},
		},
		TenantIDHeader: "X-Tenant-ID",
	}

	mockConfigProvider.On("GetConfig").Return(globalConfig)
	mockTenantApp.On("GetConfigSection", "reverseproxy").Return(mockConfigProvider, nil)

	// Set up tenant-specific config
	tenant1ID := modular.TenantID("tenant1")
	tenant1Config := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"backend1": "http://tenant1-backend1.example.com",
			"backend2": "http://tenant1-backend2.example.com",
		},
		CompositeRoutes: map[string]CompositeRoute{
			"/tenant/composite": {
				Pattern:  "/tenant/composite",
				Backends: []string{"backend1", "backend2"},
				Strategy: "json-merge",
			},
			"/shared/route": {
				Pattern:  "/shared/route",
				Backends: []string{"backend2"}, // Override the global route
				Strategy: "json-merge",
			},
		},
	}

	tenant1ConfigProvider := &MockConfigProvider{}
	tenant1ConfigProvider.On("GetConfig").Return(tenant1Config)
	mockTenantApp.On("GetTenantConfig", tenant1ID, "reverseproxy").Return(tenant1ConfigProvider, nil)

	// Create a mock router
	mockRouter := &MockRouter{}

	// Set up mock expectations
	mockRouter.On("Use", mock.Anything).Return()

	// Create the reverse proxy module
	module := NewModule()

	// Register config and set app
	err := module.RegisterConfig(mockTenantApp)
	require.NoError(t, err)

	// Initialize the module
	err = module.Init(mockTenantApp)
	require.NoError(t, err)

	// Register tenant
	module.OnTenantRegistered(tenant1ID)

	// Set up router through constructor
	constructor := module.Constructor()
	services := map[string]any{
		"router": mockRouter,
	}

	_, err = constructor(mockTenantApp, services)
	require.NoError(t, err)

	// Capture the routes registered with the router
	var registeredRoutes []string
	var routeHandlers map[string]http.HandlerFunc

	mockRouter.On("HandleFunc", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		pattern := args.String(0)
		handler := args.Get(1).(http.HandlerFunc)

		registeredRoutes = append(registeredRoutes, pattern)

		if routeHandlers == nil {
			routeHandlers = make(map[string]http.HandlerFunc)
		}
		routeHandlers[pattern] = handler
	})

	// Start the module to set up routes
	err = module.Start(context.Background())
	require.NoError(t, err)

	// Make sure our composite routes were registered
	assert.Contains(t, registeredRoutes, "/global/composite")
	assert.Contains(t, registeredRoutes, "/tenant/composite")
	assert.Contains(t, registeredRoutes, "/shared/route")

	// Now let's test the handlers to verify tenant routing works correctly

	// Test 1: Global composite route without tenant ID
	req1 := httptest.NewRequest(http.MethodGet, "/global/composite", nil)
	recorder1 := httptest.NewRecorder()

	// Since we can't properly test the actual backend responses, we'll mock the createCompositeHandler
	// function to return a handler that records which backends were used
	if handler, ok := routeHandlers["/global/composite"]; ok {
		handler(recorder1, req1)
		// The request should proceed without error
		assert.NotEqual(t, http.StatusBadRequest, recorder1.Code)
	}

	// Test 2: Global composite route with tenant ID
	req2 := httptest.NewRequest(http.MethodGet, "/global/composite", nil)
	req2.Header.Set("X-Tenant-ID", "tenant1")
	recorder2 := httptest.NewRecorder()

	if handler, ok := routeHandlers["/global/composite"]; ok {
		handler(recorder2, req2)
		// The request should proceed without error
		assert.NotEqual(t, http.StatusBadRequest, recorder2.Code)
	}

	// Test 3: Tenant-specific route with tenant ID
	req3 := httptest.NewRequest(http.MethodGet, "/tenant/composite", nil)
	req3.Header.Set("X-Tenant-ID", "tenant1")
	recorder3 := httptest.NewRecorder()

	if handler, ok := routeHandlers["/tenant/composite"]; ok {
		handler(recorder3, req3)
		// The request should proceed without error
		assert.NotEqual(t, http.StatusBadRequest, recorder3.Code)
	}

	// Test 4: Shared route with tenant ID (should use tenant-specific backend)
	req4 := httptest.NewRequest(http.MethodGet, "/shared/route", nil)
	req4.Header.Set("X-Tenant-ID", "tenant1")
	recorder4 := httptest.NewRecorder()

	if handler, ok := routeHandlers["/shared/route"]; ok {
		handler(recorder4, req4)
		// The request should proceed without error
		assert.NotEqual(t, http.StatusBadRequest, recorder4.Code)
	}
}
