package reverseproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestTimeoutNonTenantPath verifies that timeout is enforced for non-tenant paths
func TestTimeoutNonTenantPath(t *testing.T) {
	// Backend that sleeps 3 seconds but respects context cancellation
	slowBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(3 * time.Second):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "delayed"})
		case <-r.Context().Done():
			// Context cancelled - don't write anything, just return
			return
		}
	}))
	defer slowBackend.Close()

	// Create mock application
	mockApp := &mockTenantApplication{}
	mockApp.On("Logger").Return(&mockLogger{})

	// Create router service
	router := NewMockRouter()

	// Config with 1s timeout, NO tenant requirement
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"default": slowBackend.URL,
		},
		Routes: map[string]string{
			"/api/test": "default",
		},
		DefaultBackend:  "default",
		RequestTimeout:  1 * time.Second, // 1 second timeout
		RequireTenantID: false,           // Non-tenant path
	}

	// Configure mock app
	mockCP := NewStdConfigProvider(globalConfig)
	mockApp.On("GetConfigSection", "reverseproxy").Return(mockCP, nil)
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
	mockApp.On("GetTenants").Return([]modular.TenantID{})
	mockApp.On("RegisterTenant", mock.Anything, mock.Anything).Return(nil)
	mockApp.On("RemoveTenant", mock.Anything).Return(nil)
	mockApp.On("RegisterTenantAwareModule", mock.Anything).Return(nil)
	mockApp.On("GetTenantService").Return(nil, nil)

	router.On("HandleFunc", "/api/test", mock.AnythingOfType("http.HandlerFunc")).Return()
	router.On("HandleFunc", "/*", mock.AnythingOfType("http.HandlerFunc")).Return()
	router.On("Use", mock.Anything).Return()

	// Create and initialize module
	module := NewModule()
	module.app = mockApp

	err := module.Init(mockApp)
	require.NoError(t, err)

	module.router = router
	err = module.Start(context.Background())
	require.NoError(t, err)

	// Get the captured handler
	var capturedHandler http.HandlerFunc
	for _, call := range router.Calls {
		if call.Method == "HandleFunc" && call.Arguments[0].(string) == "/api/test" {
			capturedHandler = call.Arguments[1].(http.HandlerFunc)
			break
		}
	}
	require.NotNil(t, capturedHandler, "Handler should have been captured")

	// Make request
	start := time.Now()
	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()
	capturedHandler(rr, req)
	duration := time.Since(start)

	// Verify timeout occurred (should be around 1 second, not 3)
	assert.True(t, duration < 2*time.Second, fmt.Sprintf("Expected timeout around 1s, got %v", duration))
	assert.True(t, duration >= 900*time.Millisecond, fmt.Sprintf("Timeout should be at least 900ms, got %v", duration))

	// Status should indicate timeout (504 Gateway Timeout or 502 Bad Gateway)
	assert.True(t, rr.Code == http.StatusGatewayTimeout || rr.Code == http.StatusBadGateway,
		fmt.Sprintf("Expected 504 or 502, got %d", rr.Code))
}

// TestTimeoutTenantSpecificPath verifies that timeout is enforced for tenant-specific paths
func TestTimeoutTenantSpecificPath(t *testing.T) {
	// Backend that sleeps 3 seconds but respects context cancellation
	slowBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(3 * time.Second):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "delayed"})
		case <-r.Context().Done():
			// Context cancelled - don't write anything, just return
			return
		}
	}))
	defer slowBackend.Close()

	// Create mock application
	mockApp := &mockTenantApplication{}
	mockApp.On("Logger").Return(&mockLogger{})

	// Create router service
	router := NewMockRouter()

	tenantID := modular.TenantID("tenant1")

	// Config with 1s timeout AND tenant requirement
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"default": slowBackend.URL,
		},
		Routes: map[string]string{
			"/api/test": "default",
		},
		DefaultBackend:  "default",
		RequestTimeout:  1 * time.Second, // 1 second timeout
		TenantIDHeader:  "X-Affiliate-Id",
		RequireTenantID: true, // Tenant-specific path
	}

	// Tenant config
	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"default": slowBackend.URL,
		},
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

	router.On("HandleFunc", "/api/test", mock.AnythingOfType("http.HandlerFunc")).Return()
	router.On("HandleFunc", "/*", mock.AnythingOfType("http.HandlerFunc")).Return()
	router.On("Use", mock.Anything).Return()

	// Create and initialize module
	module := NewModule()
	module.app = mockApp

	// Register tenant before initialization
	module.OnTenantRegistered(tenantID)

	err := module.Init(mockApp)
	require.NoError(t, err)

	module.router = router
	err = module.Start(context.Background())
	require.NoError(t, err)

	// Get the captured handler
	var capturedHandler http.HandlerFunc
	for _, call := range router.Calls {
		if call.Method == "HandleFunc" && call.Arguments[0].(string) == "/api/test" {
			capturedHandler = call.Arguments[1].(http.HandlerFunc)
			break
		}
	}
	require.NotNil(t, capturedHandler, "Handler should have been captured")

	// Make request with tenant header
	start := time.Now()
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Affiliate-Id", string(tenantID))
	rr := httptest.NewRecorder()
	capturedHandler(rr, req)
	duration := time.Since(start)

	// Verify timeout occurred (should be around 1 second, not 3)
	// This is the key test - before the fix, this would take ~3 seconds
	assert.True(t, duration < 2*time.Second, fmt.Sprintf("Expected timeout around 1s, got %v", duration))
	assert.True(t, duration >= 900*time.Millisecond, fmt.Sprintf("Timeout should be at least 900ms, got %v", duration))

	// Status should indicate timeout (504 Gateway Timeout or 502 Bad Gateway)
	assert.True(t, rr.Code == http.StatusGatewayTimeout || rr.Code == http.StatusBadGateway,
		fmt.Sprintf("Expected 504 or 502, got %d", rr.Code))
}
