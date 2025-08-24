package reverseproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// This test verifies that a backend with empty URL in global config but valid URL in tenant config
// is properly created and routed to
func TestEmptyGlobalBackendWithValidTenantURL(t *testing.T) {
	// Create mock application
	mockApp := &mockTenantApplication{}
	mockApp.On("Logger").Return(&mockLogger{})

	// Create router service
	router := NewMockRouter()

	// Create global config with empty URL for "legacy" backend
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"legacy": "", // Empty URL in global config
		},
		Routes: map[string]string{
			"/": "legacy", // Route that uses the legacy backend
		},
		DefaultBackend: "legacy",
		TenantIDHeader: "X-Tenant-ID",
	}

	// Setup tenant config with valid URL for "legacy" backend
	tenantID := modular.TenantID("sampleaff1")
	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"legacy": "http://localhost:8080", // Valid URL in tenant config
		},
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

	// Expected handler calls for router - need to allow both "/" and "/*" since they're both used
	router.On("HandleFunc", "/", mock.AnythingOfType("http.HandlerFunc")).Return()
	router.On("HandleFunc", "/*", mock.AnythingOfType("http.HandlerFunc")).Return()
	// Expected middleware calls
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

	// Verify that router.HandleFunc was called for route "/*"
	router.AssertCalled(t, "HandleFunc", "/*", mock.AnythingOfType("http.HandlerFunc"))

	// Now test that requests are properly routed to the tenant's backend
	var capturedHandler http.HandlerFunc

	// Get the captured handler from the mock calls
	for _, call := range router.Calls {
		if call.Method == "HandleFunc" && call.Arguments[0].(string) == "/*" {
			capturedHandler = call.Arguments[1].(http.HandlerFunc)
			break
		}
	}

	assert.NotNil(t, capturedHandler, "Handler should have been captured")

	// Create a request with the tenant ID header
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Tenant-ID", string(tenantID))
	rr := httptest.NewRecorder()

	// The handler should not panic - if the backend wasn't properly created, it would
	capturedHandler(rr, req)

	// Verify that the backend proxies were properly created
	assert.Contains(t, module.tenantBackendProxies, tenantID, "Tenant backend proxy should have been created for tenant")
	assert.Contains(t, module.tenantBackendProxies[tenantID], "legacy", "Tenant should have proxy for legacy backend")

	// In a real scenario, this would now route to the tenant's backend URL
	// Since we can't easily mock the actual HTTP response, we'll verify no panic occurred
	// and that the proper proxies were created and accessible
}

// TestAffiliateBackendOverrideRouting tests that when a request includes an affiliate ID header,
// the tenant-specific backend URL is used instead of the default one.
func TestAffiliateBackendOverrideRouting(t *testing.T) {
	// Create a test server for the default backend
	defaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("default-backend-response"))
	}))
	defer defaultServer.Close()

	// Create a test server for the tenant-specific backend
	tenantServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("tenant-specific-backend-response"))
	}))
	defer tenantServer.Close()

	// Create mock application
	mockApp := &mockTenantApplication{}
	mockApp.On("Logger").Return(&mockLogger{})

	// Create router service
	router := NewMockRouter()

	// Create global config with default backend URL
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"legacy":  defaultServer.URL,       // Default legacy backend URL
			"chimera": "https://www.bing.com/", // Not used in this test
		},
		Routes: map[string]string{
			"/": "legacy", // Route that uses the legacy backend
		},
		DefaultBackend:  "legacy",
		TenantIDHeader:  "X-Affiliate-Id",
		RequireTenantID: false, // Set to false to allow testing both with and without tenant ID
	}

	// Setup tenant config with overridden URL for "legacy" backend
	tenantID := modular.TenantID("sampleaff1")
	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"legacy": tenantServer.URL, // Tenant-specific URL for legacy backend
		},
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
	router.On("HandleFunc", "/", mock.AnythingOfType("http.HandlerFunc")).Return()
	router.On("HandleFunc", "/*", mock.AnythingOfType("http.HandlerFunc")).Return()
	// Expected middleware calls
	router.On("Use", mock.Anything).Return()

	// Create the module instance and set up mock handlers
	module := NewModule()
	module.app = mockApp

	// Create a shared map to track requested URLs
	requestedURLs := make(map[string]string)

	// Register tenant before initialization
	module.OnTenantRegistered(tenantID)

	// Initialize module
	err := module.Init(mockApp)
	require.NoError(t, err)

	// Replace the proxy handlers with test handlers
	// This simulates what the actual proxy would do, but in a controlled test environment
	defaultHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := "legacy_"
		requestedURLs[key] = defaultServer.URL
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("default-response"))
	})

	tenantHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := "legacy_" + string(tenantID)
		requestedURLs[key] = tenantServer.URL
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("tenant-response"))
	})

	// Register these handlers directly with the module
	module.backendProxies = map[string]*httputil.ReverseProxy{
		"legacy": {
			Director: func(r *http.Request) {
				// Does nothing but is required to create a valid ReverseProxy
			},
			Transport: &testTransport{handler: defaultHandler},
		},
	}

	module.tenantBackendProxies = make(map[modular.TenantID]map[string]*httputil.ReverseProxy)
	module.tenantBackendProxies[tenantID] = map[string]*httputil.ReverseProxy{
		"legacy": {
			Director: func(r *http.Request) {
				// Does nothing but is required to create a valid ReverseProxy
			},
			Transport: &testTransport{handler: tenantHandler},
		},
	}

	// Register routes with the router
	module.router = router
	err = module.Start(context.Background())
	require.NoError(t, err)

	// Get the captured handler for the root route "/" or "/*"
	var capturedHandler http.HandlerFunc
	for _, call := range router.Calls {
		if call.Method == "HandleFunc" && (call.Arguments[0].(string) == "/" || call.Arguments[0].(string) == "/*") {
			capturedHandler = call.Arguments[1].(http.HandlerFunc)
			break
		}
	}
	assert.NotNil(t, capturedHandler, "Handler should have been captured")

	// Test 1: Request without tenant ID should use the default backend
	t.Run("RequestWithoutTenantID", func(t *testing.T) {
		// Clear the requestedURLs map before each test
		for k := range requestedURLs {
			delete(requestedURLs, k)
		}

		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		// Call the handler directly
		capturedHandler(rr, req)

		// Check the status code
		assert.Equal(t, http.StatusOK, rr.Code, "Request should succeed")

		// Check if the request was directed to the default backend
		assert.Contains(t, requestedURLs, "legacy_", "Request should be directed to legacy backend")
		assert.Equal(t, defaultServer.URL, requestedURLs["legacy_"], "Should use default backend URL")
	})

	// Test 2: Request with tenant ID should be routed to the tenant-specific backend
	t.Run("RequestWithTenantID", func(t *testing.T) {
		// Clear the requestedURLs map before each test
		for k := range requestedURLs {
			delete(requestedURLs, k)
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Affiliate-Id", string(tenantID))
		rr := httptest.NewRecorder()

		// Call the handler directly
		capturedHandler(rr, req)

		// Check the status code
		assert.Equal(t, http.StatusOK, rr.Code, "Request should succeed")

		// Check if the request was directed to the tenant-specific backend
		assert.Contains(t, requestedURLs, "legacy_"+string(tenantID), "Request should be directed to legacy backend with tenant ID")
		assert.Equal(t, tenantServer.URL, requestedURLs["legacy_"+string(tenantID)], "Should use tenant-specific backend URL")
	})
}

// Mock types for testing
type mockRouter struct {
	mock.Mock
}

func NewMockRouter() *mockRouter {
	return new(mockRouter)
}

func (m *mockRouter) Handle(pattern string, handler http.Handler) {
	m.Called(pattern, handler)
}

func (m *mockRouter) HandleFunc(pattern string, handler http.HandlerFunc) {
	fmt.Printf("DEBUG: MockRouter.HandleFunc called with pattern: %s\n", pattern)
	m.Called(pattern, handler)
}

func (m *mockRouter) Mount(pattern string, h http.Handler) {
	m.Called(pattern, h)
}

func (m *mockRouter) Use(middlewares ...func(http.Handler) http.Handler) {
	m.Called(middlewares)
}

func (m *mockRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}

type mockTenantApplication struct {
	mock.Mock
}

func (m *mockTenantApplication) RegisterConfigSection(name string, provider modular.ConfigProvider) {
	m.Called(name, provider)
}

func (m *mockTenantApplication) GetConfigSection(name string) (modular.ConfigProvider, error) {
	args := m.Called(name)
	if err := args.Error(1); err != nil {
		return args.Get(0).(modular.ConfigProvider), fmt.Errorf("mock get config section error: %w", err)
	}
	return args.Get(0).(modular.ConfigProvider), nil
}

func (m *mockTenantApplication) Logger() modular.Logger {
	args := m.Called()
	return args.Get(0).(modular.Logger)
}

func (m *mockTenantApplication) SetLogger(logger modular.Logger) {
	m.Called(logger)
}

func (m *mockTenantApplication) GetTenantConfig(tenantID modular.TenantID, moduleName string) (modular.ConfigProvider, error) {
	args := m.Called(tenantID, moduleName)
	if err := args.Error(1); err != nil {
		return args.Get(0).(modular.ConfigProvider), fmt.Errorf("mock get tenant config error: %w", err)
	}
	return args.Get(0).(modular.ConfigProvider), nil
}

func (m *mockTenantApplication) ConfigProvider() modular.ConfigProvider {
	args := m.Called()
	return args.Get(0).(modular.ConfigProvider)
}

func (m *mockTenantApplication) ConfigSections() map[string]modular.ConfigProvider {
	args := m.Called()
	return args.Get(0).(map[string]modular.ConfigProvider)
}

// Additional methods to implement modular.TenantApplication and modular.Application interfaces
func (m *mockTenantApplication) RegisterModule(module modular.Module) {
	m.Called(module)
}

func (m *mockTenantApplication) SvcRegistry() modular.ServiceRegistry {
	args := m.Called()
	return args.Get(0).(modular.ServiceRegistry)
}

func (m *mockTenantApplication) RegisterService(name string, service interface{}) error {
	args := m.Called(name, service)
	if err := args.Error(0); err != nil {
		return fmt.Errorf("mock error: %w", err)
	}
	return nil
}

func (m *mockTenantApplication) GetService(name string, target interface{}) error {
	args := m.Called(name, target)
	if err := args.Error(0); err != nil {
		return fmt.Errorf("mock error: %w", err)
	}
	return nil
}

func (m *mockTenantApplication) Init() error {
	args := m.Called()
	if err := args.Error(0); err != nil {
		return fmt.Errorf("mock error: %w", err)
	}
	return nil
}

func (m *mockTenantApplication) Start() error {
	args := m.Called()
	if err := args.Error(0); err != nil {
		return fmt.Errorf("mock tenant application start failed: %w", err)
	}
	return nil
}

func (m *mockTenantApplication) Stop() error {
	args := m.Called()
	if err := args.Error(0); err != nil {
		return fmt.Errorf("mock tenant application stop failed: %w", err)
	}
	return nil
}

func (m *mockTenantApplication) Run() error {
	args := m.Called()
	if err := args.Error(0); err != nil {
		return fmt.Errorf("mock tenant application run failed: %w", err)
	}
	return nil
}

func (m *mockTenantApplication) GetTenants() []modular.TenantID {
	args := m.Called()
	return args.Get(0).([]modular.TenantID)
}

func (m *mockTenantApplication) RegisterTenant(tid modular.TenantID, configs map[string]modular.ConfigProvider) error {
	args := m.Called(tid, configs)
	if err := args.Error(0); err != nil {
		return fmt.Errorf("mock register tenant failed: %w", err)
	}
	return nil
}

func (m *mockTenantApplication) RemoveTenant(tid modular.TenantID) error {
	args := m.Called(tid)
	if err := args.Error(0); err != nil {
		return fmt.Errorf("mock remove tenant failed: %w", err)
	}
	return nil
}

func (m *mockTenantApplication) RegisterTenantAwareModule(module modular.TenantAwareModule) error {
	args := m.Called(module)
	if err := args.Error(0); err != nil {
		return fmt.Errorf("mock register tenant aware module failed: %w", err)
	}
	return nil
}

func (m *mockTenantApplication) GetTenantService() (modular.TenantService, error) {
	args := m.Called()
	if err := args.Error(1); err != nil {
		return args.Get(0).(modular.TenantService), fmt.Errorf("mock get tenant service failed: %w", err)
	}
	return args.Get(0).(modular.TenantService), nil
}

func (m *mockTenantApplication) WithTenant(tid modular.TenantID) (*modular.TenantContext, error) {
	args := m.Called(tid)
	if err := args.Error(1); err != nil {
		return args.Get(0).(*modular.TenantContext), fmt.Errorf("mock with tenant failed: %w", err)
	}
	return args.Get(0).(*modular.TenantContext), nil
}

func (m *mockTenantApplication) IsVerboseConfig() bool {
	return false
}

func (m *mockTenantApplication) SetVerboseConfig(verbose bool) {
	// No-op in mock
}

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, args ...interface{}) {}
func (m *mockLogger) Info(msg string, args ...interface{})  {}
func (m *mockLogger) Warn(msg string, args ...interface{})  {}
func (m *mockLogger) Error(msg string, args ...interface{}) {}
func (m *mockLogger) Fatal(msg string, args ...interface{}) {}
