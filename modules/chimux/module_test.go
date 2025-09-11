package chimux

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
	"github.com/go-chi/chi/v5"
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
	require.NoError(t, err)

	// Verify the config section was registered in the app
	cfg, err := mockApp.GetConfigSection(module.Name())
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestModule_Init(t *testing.T) {
	module := NewChiMuxModule().(*ChiMuxModule)
	mockApp := NewMockApplication()

	// Register config first
	err := module.RegisterConfig(mockApp)
	require.NoError(t, err)

	// Register observers before Init
	err = module.RegisterObservers(mockApp)
	require.NoError(t, err)

	// Test Init
	err = module.Init(mockApp)
	require.NoError(t, err)

	// Verify config was initialized with default values
	require.NotNil(t, module.config)
	assert.Equal(t, []string{"*"}, module.config.AllowedOrigins)
	assert.Equal(t, []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, module.config.AllowedMethods)
	assert.False(t, module.config.AllowCredentials)
	assert.Equal(t, 300, module.config.MaxAge)
	assert.Equal(t, 60*time.Second, module.config.Timeout)

	// Verify router was created
	assert.NotNil(t, module.router, "Router should be initialized")
	assert.NotNil(t, module.logger, "Logger should be initialized")

	// Verify module provides the router service
	services := module.ProvidesServices()
	assert.GreaterOrEqual(t, len(services), 1, "Should provide at least one service")

	// Check that the main service is provided
	var mainServiceFound bool
	for _, service := range services {
		if service.Name == ServiceName {
			mainServiceFound = true
			assert.Equal(t, module, service.Instance)
			break
		}
	}
	assert.True(t, mainServiceFound, "Main service should be provided")
}

func TestModule_RouterFunctionality(t *testing.T) {
	module := NewChiMuxModule().(*ChiMuxModule)
	mockApp := NewMockApplication()

	// Setup the module
	err := module.RegisterConfig(mockApp)
	require.NoError(t, err)

	// Register observers before Init
	err = module.RegisterObservers(mockApp)
	require.NoError(t, err)

	err = module.Init(mockApp)
	require.NoError(t, err)

	// Define test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test-response"))
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

	// Register observers before Init
	err = module.RegisterObservers(mockApp)
	require.NoError(t, err)

	err = module.Init(mockApp)
	require.NoError(t, err)

	// Create nested routes
	module.Route("/api", func(r chi.Router) {
		r.Get("/users", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("users-list"))
		})

		r.Route("/posts", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("posts-list"))
			})

			r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("post-detail"))
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
	err = mockApp.RegisterService("test.middleware.provider", middlewareProvider)
	require.NoError(t, err)

	// Register observers before Init
	err = module.RegisterObservers(mockApp)
	require.NoError(t, err)

	// Initialize the module
	err = module.Init(mockApp)
	require.NoError(t, err)

	// Create a test route
	module.Get("/secured", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("secured-content"))
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
	assert.NotEmpty(t, deps, "Module should require services")

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
		Timeout:          60 * time.Second,
		BasePath:         "/api/v1", // Set custom base path
	}

	mockApp.RegisterConfigSection(module.Name(), NewStdConfigProvider(customConfig))

	// Get config
	cfg, err := mockApp.GetConfigSection(module.Name())
	require.NoError(t, err)
	module.config = cfg.GetConfig().(*ChiMuxConfig)

	// Register observers before Init
	err = module.RegisterObservers(mockApp)
	require.NoError(t, err)

	// Init the module
	err = module.Init(mockApp)
	require.NoError(t, err)

	// Register a route
	module.Get("/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("users-list"))
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

	// Register observers before Init
	err = module.RegisterObservers(mockApp)
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
		Timeout:  30 * time.Second,
	}

	// Register tenant in mock tenant service
	mockApp.tenantService.Configs[tenantID] = map[string]modular.ConfigProvider{
		module.Name(): NewStdConfigProvider(tenantConfig),
	}

	// Trigger tenant registration
	module.OnTenantRegistered(tenantID)

	// Verify tenant ID is stored (but config not loaded yet)
	_, exists := module.tenantConfigs[tenantID]
	assert.True(t, exists, "Tenant ID should be stored")
	assert.Nil(t, module.tenantConfigs[tenantID], "Tenant config should be nil before loading")

	// Manually load tenant configs (normally done in Start())
	module.loadTenantConfigs()

	// Now verify tenant config is loaded correctly
	storedConfig := module.tenantConfigs[tenantID]
	require.NotNil(t, storedConfig)
	assert.Equal(t, "/tenant", storedConfig.BasePath)
	assert.Equal(t, 30*time.Second, storedConfig.Timeout)

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

	// Register observers before Init
	err = module.RegisterObservers(mockApp)
	require.NoError(t, err)

	err = module.Init(mockApp)
	require.NoError(t, err)

	// Test Start
	err = module.Start(context.Background())
	require.NoError(t, err)

	// Test Stop
	err = module.Stop(context.Background())
	require.NoError(t, err)
}

// TestCORSMiddleware tests that CORS headers are properly applied based on configuration
func TestCORSMiddleware(t *testing.T) {
	module := NewChiMuxModule().(*ChiMuxModule)
	mockApp := NewMockApplication()

	// Setup the module with custom CORS configuration
	err := module.RegisterConfig(mockApp)
	require.NoError(t, err)

	// Modify the config with specific CORS settings
	mockApp.configSections["chimux"] = &mockConfigProvider{
		config: &ChiMuxConfig{
			AllowedOrigins:   []string{"https://example.com", "https://test.com"},
			AllowedMethods:   []string{"GET", "POST", "PUT"},
			AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Custom-Header"},
			AllowCredentials: true,
			MaxAge:           600,
		},
	}

	// Register observers before Init
	err = module.RegisterObservers(mockApp)
	require.NoError(t, err)

	// Initialize the module with the custom config
	err = module.Init(mockApp)
	require.NoError(t, err)

	// Define a simple test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test-response"))
	})

	// Register the handler
	module.Get("/cors-test", testHandler)

	// Test 1: Request with a matching origin
	req1 := httptest.NewRequest("GET", "/cors-test", nil)
	req1.Header.Set("Origin", "https://example.com")
	w1 := httptest.NewRecorder()
	module.ServeHTTP(w1, req1)

	// Verify response and CORS headers
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, "test-response", w1.Body.String())
	assert.Equal(t, "https://example.com", w1.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT", w1.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization, X-Custom-Header", w1.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "true", w1.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "600", w1.Header().Get("Access-Control-Max-Age"))

	// Test 2: Request with non-matching origin
	req2 := httptest.NewRequest("GET", "/cors-test", nil)
	req2.Header.Set("Origin", "https://unknown-origin.com")
	w2 := httptest.NewRecorder()
	module.ServeHTTP(w2, req2)

	// Verify response has no CORS headers for non-matching origin
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Empty(t, w2.Header().Get("Access-Control-Allow-Origin"))

	// Test 3: Preflight OPTIONS request
	req3 := httptest.NewRequest("OPTIONS", "/cors-test", nil)
	req3.Header.Set("Origin", "https://example.com")
	req3.Header.Set("Access-Control-Request-Method", "POST")
	w3 := httptest.NewRecorder()
	module.ServeHTTP(w3, req3)

	// Verify preflight response
	assert.Equal(t, http.StatusNoContent, w3.Code)
	assert.Equal(t, "https://example.com", w3.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT", w3.Header().Get("Access-Control-Allow-Methods"))
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

func TestModule_Dependencies(t *testing.T) {
	module := NewChiMuxModule().(*ChiMuxModule)

	// Verify Dependencies is empty
	assert.Empty(t, module.Dependencies())
}
