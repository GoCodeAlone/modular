package chimux

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChiMuxModule(t *testing.T) {
	module := NewChiMuxModule()
	assert.NotNil(t, module)

	modImpl, ok := module.(*ChiMuxModule)
	require.True(t, ok)
	assert.Equal(t, "chimux", modImpl.Name())
	assert.NotNil(t, modImpl.tenantConfigs)
}

func TestModule_RegisterConfig(t *testing.T) {
	module := NewChiMuxModule().(*ChiMuxModule)
	mockApp := NewMockApplication()

	err := module.RegisterConfig(mockApp)
	assert.NoError(t, err)

	// Verify config was initialized with default values
	require.NotNil(t, module.config)
	assert.Equal(t, []string{"*"}, module.config.AllowedOrigins)
	assert.Equal(t, []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, module.config.AllowedMethods)
	assert.False(t, module.config.AllowCredentials)
	assert.Equal(t, 300, module.config.MaxAge)
	assert.Equal(t, 60000, module.config.Timeout)

	// Verify the config section was registered in the app
	cfg, err := mockApp.GetConfigSection(module.Name())
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestModule_Init(t *testing.T) {
	module := NewChiMuxModule().(*ChiMuxModule)
	mockApp := NewMockApplication()

	// Register config first
	err := module.RegisterConfig(mockApp)
	require.NoError(t, err)

	// Test Init
	err = module.Init(mockApp)
	assert.NoError(t, err)

	// Verify router was created
	assert.NotNil(t, module.router, "Router should be initialized")
	assert.NotNil(t, module.logger, "Logger should be initialized")

	// Verify module provides the router service
	services := module.ProvidesServices()
	assert.Len(t, services, 1)
	assert.Equal(t, ServiceName, services[0].Name)
	assert.Equal(t, module, services[0].Instance)
}

func TestModule_RouterFunctionality(t *testing.T) {
	module := NewChiMuxModule().(*ChiMuxModule)
	mockApp := NewMockApplication()

	// Setup the module
	err := module.RegisterConfig(mockApp)
	require.NoError(t, err)
	err = module.Init(mockApp)
	require.NoError(t, err)

	// Define test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test-response"))
	})

	// Register a route
	module.Get("/test", testHandler)

	// Test route handling with a request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	module.router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test-response", w.Body.String())
}

func TestModule_NestedRoutes(t *testing.T) {
	module := NewChiMuxModule().(*ChiMuxModule)
	mockApp := NewMockApplication()

	// Setup the module
	err := module.RegisterConfig(mockApp)
	require.NoError(t, err)
	err = module.Init(mockApp)
	require.NoError(t, err)

	// Create nested routes
	module.Route("/api", func(r Router) {
		r.Get("/users", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("users-list"))
		})

		r.Route("/posts", func(r Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("posts-list"))
			})

			r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("post-detail"))
			})
		})
	})

	// Test first route
	req1 := httptest.NewRequest("GET", "/api/users", nil)
	w1 := httptest.NewRecorder()
	module.router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, "users-list", w1.Body.String())

	// Test nested route
	req2 := httptest.NewRequest("GET", "/api/posts", nil)
	w2 := httptest.NewRecorder()
	module.router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, "posts-list", w2.Body.String())

	// Test parameter route
	req3 := httptest.NewRequest("GET", "/api/posts/123", nil)
	w3 := httptest.NewRecorder()
	module.router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	assert.Equal(t, "post-detail", w3.Body.String())
}

func TestModule_CustomMiddleware(t *testing.T) {
	module := NewChiMuxModule().(*ChiMuxModule)
	mockApp := NewMockApplication()

	// Setup the module
	err := module.RegisterConfig(mockApp)
	require.NoError(t, err)

	// Create a middleware provider
	middlewareProvider := &TestMiddlewareProvider{}
	mockApp.RegisterService("test.middleware.provider", middlewareProvider)

	// Initialize the module
	err = module.Init(mockApp)
	require.NoError(t, err)

	// Create a test route
	module.Get("/secured", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("secured-content"))
	})

	// Test with valid auth header
	req1 := httptest.NewRequest("GET", "/secured", nil)
	req1.Header.Set("Authorization", "valid-token")
	w1 := httptest.NewRecorder()
	module.router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, "secured-content", w1.Body.String())

	// Test with invalid auth header
	req2 := httptest.NewRequest("GET", "/secured", nil)
	req2.Header.Set("Authorization", "invalid-token")
	w2 := httptest.NewRecorder()
	module.router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)

	// Test with missing auth header
	req3 := httptest.NewRequest("GET", "/secured", nil)
	w3 := httptest.NewRecorder()
	module.router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusUnauthorized, w3.Code)
}

func TestModule_InterfaceBasedMatching(t *testing.T) {
	module := NewChiMuxModule().(*ChiMuxModule)

	// Verify the module requests middleware providers using interface-based matching
	deps := module.RequiresServices()
	require.NotEmpty(t, deps)

	// Find the middleware.provider dependency
	var middlewareDep modular.ServiceDependency
	for _, dep := range deps {
		if dep.Name == "middleware.provider" {
			middlewareDep = dep
			break
		}
	}

	require.NotNil(t, middlewareDep)
	assert.True(t, middlewareDep.MatchByInterface)
	assert.Equal(t, reflect.TypeOf((*MiddlewareProvider)(nil)).Elem(), middlewareDep.SatisfiesInterface)
	assert.False(t, middlewareDep.Required)
}

func TestModule_BasePath(t *testing.T) {
	module := NewChiMuxModule().(*ChiMuxModule)
	mockApp := NewMockApplication()

	// Set base path in config
	customConfig := &ChiMuxConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Authorization"},
		AllowCredentials: false,
		MaxAge:           300,
		Timeout:          60000,
		BasePath:         "/api/v1", // Set custom base path
	}

	mockApp.RegisterConfigSection(module.Name(), NewStdConfigProvider(customConfig))

	// Get config
	cfg, err := mockApp.GetConfigSection(module.Name())
	require.NoError(t, err)
	module.config = cfg.GetConfig().(*ChiMuxConfig)

	// Init the module
	err = module.Init(mockApp)
	require.NoError(t, err)

	// Register a route
	module.Get("/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("users-list"))
	})

	// Test the route with base path prefix
	// NOTE: Use the module itself as the handler, not the underlying router
	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	w := httptest.NewRecorder()
	module.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "users-list", w.Body.String())

	// Test without base path (should fail)
	req2 := httptest.NewRequest("GET", "/users", nil)
	w2 := httptest.NewRecorder()
	module.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestModule_TenantLifecycle(t *testing.T) {
	module := NewChiMuxModule().(*ChiMuxModule)
	mockApp := NewMockApplication()

	// Setup the module
	err := module.RegisterConfig(mockApp)
	require.NoError(t, err)
	err = module.Init(mockApp)
	require.NoError(t, err)

	// Base config should exist
	require.NotNil(t, module.config)

	// Test tenant registration
	tenantID := modular.TenantID("test-tenant")

	// Create tenant-specific config
	tenantConfig := &ChiMuxConfig{
		BasePath: "/tenant",
		Timeout:  30000,
	}

	// Register tenant in mock tenant service
	mockApp.tenantService.Configs[tenantID] = map[string]modular.ConfigProvider{
		module.Name(): NewStdConfigProvider(tenantConfig),
	}

	// Trigger tenant registration
	module.OnTenantRegistered(tenantID)

	// Verify tenant config is stored
	storedConfig, exists := module.tenantConfigs[tenantID]
	assert.True(t, exists, "Tenant config should be stored")
	require.NotNil(t, storedConfig)
	assert.Equal(t, "/tenant", storedConfig.BasePath)
	assert.Equal(t, 30000, storedConfig.Timeout)

	// Verify GetTenantConfig works for existing tenant
	retrievedConfig := module.GetTenantConfig(tenantID)
	assert.Equal(t, tenantConfig, retrievedConfig)

	// Test tenant removal
	module.OnTenantRemoved(tenantID)
	_, exists = module.tenantConfigs[tenantID]
	assert.False(t, exists, "Tenant config should be removed")

	// GetTenantConfig should fall back to base config after removal
	fallbackConfig := module.GetTenantConfig(tenantID)
	assert.Equal(t, module.config, fallbackConfig)
}

func TestModule_Start_Stop(t *testing.T) {
	module := NewChiMuxModule().(*ChiMuxModule)
	mockApp := NewMockApplication()

	// Setup the module
	err := module.RegisterConfig(mockApp)
	require.NoError(t, err)
	err = module.Init(mockApp)
	require.NoError(t, err)

	// Test Start
	err = module.Start(context.Background())
	assert.NoError(t, err)

	// Test Stop
	err = module.Stop(context.Background())
	assert.NoError(t, err)
}

// TestMiddlewareProvider implements a test middleware provider
type TestMiddlewareProvider struct{}

func (p *TestMiddlewareProvider) ProvideMiddleware() []Middleware {
	return []Middleware{
		// Auth middleware
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				token := r.Header.Get("Authorization")
				if token == "" {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				if token != "valid-token" {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
			})
		},
	}
}
