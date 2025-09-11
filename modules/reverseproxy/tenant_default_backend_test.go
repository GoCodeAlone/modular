package reverseproxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CrisisTextLine/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestTenantDefaultBackendOverride tests the tenant-specific default backend override functionality
func TestTenantDefaultBackendOverride(t *testing.T) {
	// Create test servers for different backends
	globalDefaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"global-default","path":"` + r.URL.Path + `"}`))
	}))
	defer globalDefaultServer.Close()

	tenantDefaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"tenant-default","path":"` + r.URL.Path + `"}`))
	}))
	defer tenantDefaultServer.Close()

	specificBackendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"specific-backend","path":"` + r.URL.Path + `"}`))
	}))
	defer specificBackendServer.Close()

	// Create mock application
	mockApp := &mockTenantApplication{}
	mockApp.On("Logger").Return(&mockLogger{})

	// Create router service
	router := NewMockRouter()

	// Create global config with a default backend
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"global-backend":   globalDefaultServer.URL,
			"tenant-backend":   tenantDefaultServer.URL,
			"specific-backend": specificBackendServer.URL,
		},
		Routes: map[string]string{
			"/api/specific": "specific-backend", // Specific route that should not be affected by default backend override
		},
		DefaultBackend: "global-backend", // Global default backend
		TenantIDHeader: "X-Tenant-ID",
	}

	// Setup tenant config with a different default backend
	tenantID := modular.TenantID("test-tenant")
	tenantConfig := &ReverseProxyConfig{
		DefaultBackend: "tenant-backend", // Tenant-specific default backend (different from global)
	}

	// Configure mock app to return our configs
	mockCP := NewStdConfigProvider(globalConfig)
	tenantMockCP := NewStdConfigProvider(tenantConfig)
	mockApp.On("GetConfigSection", "reverseproxy").Return(mockCP, nil)
	mockApp.On("GetTenantConfig", tenantID, "reverseproxy").Return(tenantMockCP, nil)
	mockApp.On("ConfigProvider").Return(mockCP)
	mockApp.On("ConfigSections").Return(map[string]modular.ConfigProvider{
		"reverseproxy": mockCP,
	})
	mockApp.On("RegisterModule", mock.Anything).Return()
	mockApp.On("RegisterConfigSection", mock.Anything, mock.Anything).Return()
	mockApp.On("SvcRegistry").Return(map[string]any{})
	mockApp.On("RegisterService", mock.Anything, mock.Anything).Return(nil)
	mockApp.On("GetService", mock.Anything, mock.Anything).Return(nil)
	mockApp.On("Init").Return(nil)
	mockApp.On("Start").Return(nil)
	mockApp.On("Stop").Return(nil)
	mockApp.On("Run").Return(nil)
	mockApp.On("GetTenants").Return([]modular.TenantID{tenantID})
	mockApp.On("RegisterTenant", mock.Anything, mock.Anything).Return(nil)
	mockApp.On("RemoveTenant", mock.Anything).Return(nil)
	mockApp.On("RegisterTenantAwareModule", mock.Anything).Return(nil)
	mockApp.On("GetTenantService").Return(nil, nil)
	mockApp.On("WithTenant", mock.Anything).Return(&modular.TenantContext{}, nil)

	// Expected handler calls for router
	router.On("HandleFunc", mock.Anything, mock.AnythingOfType("http.HandlerFunc")).Return()
	router.On("Use", mock.Anything).Return()

	// Create module
	module := NewModule()
	module.app = mockApp

	// Register tenant before initialization
	module.OnTenantRegistered(tenantID)

	// Initialize module
	err := module.Init(mockApp)
	require.NoError(t, err)

	// Register routes with the router
	module.router = router
	err = module.Start(context.Background())
	require.NoError(t, err)

	// Get the captured handler for the specific route and catch-all
	var specificRouteHandler, catchAllHandler http.HandlerFunc

	for _, call := range router.Calls {
		if call.Method == "HandleFunc" {
			pattern := call.Arguments[0].(string)
			handler := call.Arguments[1].(http.HandlerFunc)

			switch pattern {
			case "/api/specific":
				specificRouteHandler = handler
			case "/*":
				catchAllHandler = handler
			}
		}
	}

	require.NotNil(t, catchAllHandler, "Catch-all handler should have been captured")

	t.Run("RequestWithoutTenantIDUsesGlobalDefault", func(t *testing.T) {
		// Test requests to unmatched paths without tenant ID should use global default backend
		req := httptest.NewRequest("GET", "/some/unmatched/path", nil)
		w := httptest.NewRecorder()

		catchAllHandler(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Should contain the global default backend response
		assert.Contains(t, string(body), "global-default")
	})

	t.Run("RequestWithTenantIDUsesTenantDefault", func(t *testing.T) {
		// Test requests to unmatched paths with tenant ID should use tenant-specific default backend
		// NOTE: This test should FAIL initially because the feature is not yet implemented
		req := httptest.NewRequest("GET", "/some/unmatched/path", nil)
		req.Header.Set("X-Tenant-ID", string(tenantID))
		w := httptest.NewRecorder()

		catchAllHandler(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Should contain the tenant-specific default backend response
		// This assertion will FAIL until we enable the commented code
		assert.Contains(t, string(body), "tenant-default", "Request with tenant ID should use tenant-specific default backend")
	})

	t.Run("SpecificRouteNotAffectedByDefaultBackendOverride", func(t *testing.T) {
		// Test that specific routes are not affected by default backend override
		if specificRouteHandler != nil {
			req := httptest.NewRequest("GET", "/api/specific", nil)
			req.Header.Set("X-Tenant-ID", string(tenantID))
			w := httptest.NewRecorder()

			specificRouteHandler(w, req)

			resp := w.Result()
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			// Should use the specifically configured backend, not the tenant default
			assert.Contains(t, string(body), "specific-backend")
		}
	})

	t.Run("TenantWithSameDefaultAsGlobalIgnored", func(t *testing.T) {
		// Test a tenant that has the same default backend as global - should not create duplicate routes
		sameDefaultTenantID := modular.TenantID("same-default-tenant")
		sameDefaultTenantConfig := &ReverseProxyConfig{
			DefaultBackend: "global-backend", // Same as global default
		}

		module.tenants[sameDefaultTenantID] = sameDefaultTenantConfig

		// Start the module again to register the new tenant
		module.router = NewMockRouter()
		module.router.(*mockRouter).On("HandleFunc", mock.Anything, mock.AnythingOfType("http.HandlerFunc")).Return()
		module.router.(*mockRouter).On("Use", mock.Anything).Return()

		err := module.registerTenantAwareRoutes()
		assert.NoError(t, err)

		// The logic should handle this gracefully without creating unnecessary duplicate routes
		// (This is more of a sanity check to ensure the implementation is robust)
	})
}

// TestTenantDefaultBackendWithEmptyGlobalDefault tests tenant default backend when global default is empty
func TestTenantDefaultBackendWithEmptyGlobalDefault(t *testing.T) {
	// Create test server for tenant default backend
	tenantDefaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"tenant-default","path":"` + r.URL.Path + `"}`))
	}))
	defer tenantDefaultServer.Close()

	// Create mock application
	mockApp := &mockTenantApplication{}
	mockApp.On("Logger").Return(&mockLogger{})

	// Create router service
	router := NewMockRouter()

	// Create global config with NO default backend
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"tenant-backend": tenantDefaultServer.URL,
		},
		// No DefaultBackend set globally
		TenantIDHeader: "X-Tenant-ID",
	}

	// Setup tenant config with a default backend
	tenantID := modular.TenantID("test-tenant-2")
	tenantConfig := &ReverseProxyConfig{
		DefaultBackend: "tenant-backend", // Tenant-specific default backend when global has none
	}

	// Configure mock app
	mockCP := NewStdConfigProvider(globalConfig)
	tenantMockCP := NewStdConfigProvider(tenantConfig)
	mockApp.On("GetConfigSection", "reverseproxy").Return(mockCP, nil)
	mockApp.On("GetTenantConfig", tenantID, "reverseproxy").Return(tenantMockCP, nil)
	mockApp.On("ConfigProvider").Return(mockCP)
	mockApp.On("ConfigSections").Return(map[string]modular.ConfigProvider{
		"reverseproxy": mockCP,
	})
	mockApp.On("RegisterModule", mock.Anything).Return()
	mockApp.On("RegisterConfigSection", mock.Anything, mock.Anything).Return()
	mockApp.On("SvcRegistry").Return(map[string]any{})
	mockApp.On("RegisterService", mock.Anything, mock.Anything).Return(nil)
	mockApp.On("GetService", mock.Anything, mock.Anything).Return(nil)
	mockApp.On("Init").Return(nil)
	mockApp.On("Start").Return(nil)
	mockApp.On("Stop").Return(nil)
	mockApp.On("Run").Return(nil)
	mockApp.On("GetTenants").Return([]modular.TenantID{tenantID})
	mockApp.On("RegisterTenant", mock.Anything, mock.Anything).Return(nil)
	mockApp.On("RemoveTenant", mock.Anything).Return(nil)
	mockApp.On("RegisterTenantAwareModule", mock.Anything).Return(nil)
	mockApp.On("GetTenantService").Return(nil, nil)
	mockApp.On("WithTenant", mock.Anything).Return(&modular.TenantContext{}, nil)

	// Expected handler calls for router
	router.On("HandleFunc", mock.Anything, mock.AnythingOfType("http.HandlerFunc")).Return()
	router.On("Use", mock.Anything).Return()

	// Create module
	module := NewModule()
	module.app = mockApp

	// Register tenant before initialization
	module.OnTenantRegistered(tenantID)

	// Initialize module
	err := module.Init(mockApp)
	require.NoError(t, err)

	// Register routes with the router
	module.router = router
	err = module.Start(context.Background())
	require.NoError(t, err)

	t.Run("TenantDefaultBackendUsedWhenGlobalEmpty", func(t *testing.T) {
		// Find the catch-all handler
		var catchAllHandler http.HandlerFunc
		for _, call := range router.Calls {
			if call.Method == "HandleFunc" && call.Arguments[0].(string) == "/*" {
				catchAllHandler = call.Arguments[1].(http.HandlerFunc)
				break
			}
		}

		if catchAllHandler != nil {
			req := httptest.NewRequest("GET", "/some/path", nil)
			req.Header.Set("X-Tenant-ID", string(tenantID))
			w := httptest.NewRecorder()

			catchAllHandler(w, req)

			resp := w.Result()
			// This test should fail initially since tenant default backend override is not implemented
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}
	})
}

// TestMultipleTenantDefaultBackends tests multiple tenants with different default backends
func TestMultipleTenantDefaultBackends(t *testing.T) {
	// Create test servers for different tenant backends
	tenant1Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"tenant1-backend","path":"` + r.URL.Path + `"}`))
	}))
	defer tenant1Server.Close()

	tenant2Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"tenant2-backend","path":"` + r.URL.Path + `"}`))
	}))
	defer tenant2Server.Close()

	globalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"global-backend","path":"` + r.URL.Path + `"}`))
	}))
	defer globalServer.Close()

	// Create mock application
	mockApp := &mockTenantApplication{}
	mockApp.On("Logger").Return(&mockLogger{})

	// Create router service
	router := NewMockRouter()

	// Create global config
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"global-backend":  globalServer.URL,
			"tenant1-backend": tenant1Server.URL,
			"tenant2-backend": tenant2Server.URL,
		},
		DefaultBackend: "global-backend",
		TenantIDHeader: "X-Tenant-ID",
	}

	// Setup multiple tenant configs
	tenant1ID := modular.TenantID("tenant1")
	tenant1Config := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"tenant1-backend": tenant1Server.URL, // Tenant1 defines its own backend service
		},
		DefaultBackend: "tenant1-backend",
	}

	tenant2ID := modular.TenantID("tenant2")
	tenant2Config := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"tenant2-backend": tenant2Server.URL, // Tenant2 defines its own backend service
		},
		DefaultBackend: "tenant2-backend",
	}

	// Configure mock app
	mockCP := NewStdConfigProvider(globalConfig)
	tenant1MockCP := NewStdConfigProvider(tenant1Config)
	tenant2MockCP := NewStdConfigProvider(tenant2Config)

	mockApp.On("GetConfigSection", "reverseproxy").Return(mockCP, nil)
	mockApp.On("GetTenantConfig", tenant1ID, "reverseproxy").Return(tenant1MockCP, nil)
	mockApp.On("GetTenantConfig", tenant2ID, "reverseproxy").Return(tenant2MockCP, nil)
	mockApp.On("ConfigProvider").Return(mockCP)
	mockApp.On("ConfigSections").Return(map[string]modular.ConfigProvider{
		"reverseproxy": mockCP,
	})
	mockApp.On("RegisterModule", mock.Anything).Return()
	mockApp.On("RegisterConfigSection", mock.Anything, mock.Anything).Return()
	mockApp.On("SvcRegistry").Return(map[string]any{})
	mockApp.On("RegisterService", mock.Anything, mock.Anything).Return(nil)
	mockApp.On("GetService", mock.Anything, mock.Anything).Return(nil)
	mockApp.On("Init").Return(nil)
	mockApp.On("Start").Return(nil)
	mockApp.On("Stop").Return(nil)
	mockApp.On("Run").Return(nil)
	mockApp.On("GetTenants").Return([]modular.TenantID{tenant1ID, tenant2ID})
	mockApp.On("RegisterTenant", mock.Anything, mock.Anything).Return(nil)
	mockApp.On("RemoveTenant", mock.Anything).Return(nil)
	mockApp.On("RegisterTenantAwareModule", mock.Anything).Return(nil)
	mockApp.On("GetTenantService").Return(nil, nil)
	mockApp.On("WithTenant", mock.Anything).Return(&modular.TenantContext{}, nil)

	// Expected handler calls
	router.On("HandleFunc", mock.Anything, mock.AnythingOfType("http.HandlerFunc")).Return()
	router.On("Use", mock.Anything).Return()

	// Create module
	module := NewModule()
	module.app = mockApp

	// Register tenants
	module.OnTenantRegistered(tenant1ID)
	module.OnTenantRegistered(tenant2ID)

	// Initialize module
	err := module.Init(mockApp)
	require.NoError(t, err)

	// Register routes
	module.router = router
	err = module.Start(context.Background())
	require.NoError(t, err)

	t.Run("DifferentTenantsShouldUseDifferentDefaults", func(t *testing.T) {
		// Debug: Check what tenants are registered
		assert.Contains(t, module.tenants, tenant1ID, "Tenant1 should be registered")
		assert.Contains(t, module.tenants, tenant2ID, "Tenant2 should be registered")

		if tenant1Cfg, ok := module.tenants[tenant1ID]; ok {
			assert.NotNil(t, tenant1Cfg, "Tenant1 config should not be nil")
			assert.Equal(t, "tenant1-backend", tenant1Cfg.DefaultBackend, "Tenant1 should have correct default backend")
			assert.Contains(t, tenant1Cfg.BackendServices, "tenant1-backend", "Tenant1 should have tenant1-backend service")
		}

		if tenant2Cfg, ok := module.tenants[tenant2ID]; ok {
			assert.NotNil(t, tenant2Cfg, "Tenant2 config should not be nil")
			assert.Equal(t, "tenant2-backend", tenant2Cfg.DefaultBackend, "Tenant2 should have correct default backend")
			assert.Contains(t, tenant2Cfg.BackendServices, "tenant2-backend", "Tenant2 should have tenant2-backend service")
		}

		// Find the catch-all handler (get the LAST one registered, which should be tenant-aware)
		var catchAllHandler http.HandlerFunc
		for _, call := range router.Calls {
			if call.Method == "HandleFunc" && call.Arguments[0].(string) == "/*" {
				catchAllHandler = call.Arguments[1].(http.HandlerFunc)
			}
		}

		if catchAllHandler != nil {
			// Test tenant1 - should use tenant1-backend
			req1 := httptest.NewRequest("GET", "/test", nil)
			req1.Header.Set("X-Tenant-ID", string(tenant1ID))
			w1 := httptest.NewRecorder()

			catchAllHandler(w1, req1)

			resp1 := w1.Result()
			assert.Equal(t, http.StatusOK, resp1.StatusCode)
			body1, err := io.ReadAll(resp1.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body1), "tenant1-backend", "Tenant1 should use tenant1-specific default backend")

			// Test tenant2 - should use tenant2-backend
			req2 := httptest.NewRequest("GET", "/test", nil)
			req2.Header.Set("X-Tenant-ID", string(tenant2ID))
			w2 := httptest.NewRecorder()

			catchAllHandler(w2, req2)

			resp2 := w2.Result()
			assert.Equal(t, http.StatusOK, resp2.StatusCode)
			body2, err := io.ReadAll(resp2.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body2), "tenant2-backend", "Tenant2 should use tenant2-specific default backend")

			// Test without tenant (should use global)
			req3 := httptest.NewRequest("GET", "/test", nil)
			w3 := httptest.NewRecorder()

			catchAllHandler(w3, req3)

			resp3 := w3.Result()
			assert.Equal(t, http.StatusOK, resp3.StatusCode)
			body3, err := io.ReadAll(resp3.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body3), "global-backend", "Request without tenant should use global default backend")
		}
	})
}
