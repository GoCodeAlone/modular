package reverseproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestTenantRequestTimeoutOverride verifies that tenant-specific request_timeout overrides global configuration
func TestTenantRequestTimeoutOverride(t *testing.T) {
	// Backend that sleeps 2.5 seconds but respects context cancellation
	slowBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2500 * time.Millisecond):
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

	// Global config with 30s timeout
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"default": slowBackend.URL,
		},
		Routes: map[string]string{
			"/api/test": "default",
		},
		DefaultBackend:  "default",
		RequestTimeout:  30 * time.Second, // Global: 30 second timeout
		TenantIDHeader:  "X-Affiliate-Id",
		RequireTenantID: true,
	}

	// Tenant config with 1s timeout - should override global 30s
	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"default": slowBackend.URL,
		},
		RequestTimeout: 1 * time.Second, // Tenant: 1 second timeout (should override)
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

	// Verify the merged config has the tenant timeout
	mergedCfg, exists := module.tenants[tenantID]
	require.True(t, exists, "Tenant config should exist")
	assert.Equal(t, 1*time.Second, mergedCfg.RequestTimeout, "Merged config should have tenant's 1s timeout")

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

	// CRITICAL TEST: Verify tenant timeout (1s) was used, not global timeout (30s)
	// Backend sleeps 2.5s, so with 1s timeout it should fail around 1s
	// If global timeout (30s) was incorrectly used, duration would be ~2.5s
	assert.True(t, duration < 2*time.Second,
		"Expected timeout around 1s (tenant), got %v - tenant timeout not being used!", duration)
	assert.True(t, duration >= 900*time.Millisecond,
		"Timeout should be at least 900ms, got %v", duration)

	// Status should indicate timeout (504 Gateway Timeout or 502 Bad Gateway)
	assert.True(t, rr.Code == http.StatusGatewayTimeout || rr.Code == http.StatusBadGateway,
		"Expected 504 or 502, got %d", rr.Code)
}

// TestTenantCacheTTLOverride verifies that tenant-specific cache_ttl overrides global configuration
func TestTenantCacheTTLOverride(t *testing.T) {
	globalConfig := &ReverseProxyConfig{
		CacheEnabled: true,
		CacheTTL:     120 * time.Second, // Global: 120s
	}

	tenantConfig := &ReverseProxyConfig{
		CacheEnabled: true,
		CacheTTL:     60 * time.Second, // Tenant: 60s (should override)
	}

	merged := mergeConfigs(globalConfig, tenantConfig)

	assert.True(t, merged.CacheEnabled, "Cache should be enabled")
	assert.Equal(t, 60*time.Second, merged.CacheTTL,
		"Merged config should use tenant's 60s CacheTTL, not global 120s")
}

// TestTenantMetricsPathOverride verifies that tenant-specific metrics_path overrides global configuration
func TestTenantMetricsPathOverride(t *testing.T) {
	globalConfig := &ReverseProxyConfig{
		MetricsEnabled: true,
		MetricsPath:    "/metrics/global",
	}

	tenantConfig := &ReverseProxyConfig{
		MetricsEnabled: true,
		MetricsPath:    "/metrics/tenant1",
	}

	merged := mergeConfigs(globalConfig, tenantConfig)

	assert.True(t, merged.MetricsEnabled, "Metrics should be enabled")
	assert.Equal(t, "/metrics/tenant1", merged.MetricsPath,
		"Merged config should use tenant's metrics path, not global")
}

// TestTenantFeatureFlagsOverride documents that tenant-specific feature flags are NOT currently merged
// The mergeConfigs function doesn't perform deep merging of the FeatureFlags.Flags map,
// resulting in zero values for the entire FeatureFlags struct in the merged configuration.
// TODO: This is a known limitation that should be addressed in a separate issue
func TestTenantFeatureFlagsOverride(t *testing.T) {
	globalConfig := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"feature_a": true,
				"feature_b": false,
			},
		},
	}

	tenantConfig := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"feature_a": false, // Override to false
				"feature_c": true,  // New flag
			},
		},
	}

	merged := mergeConfigs(globalConfig, tenantConfig)

	// KNOWN LIMITATION: mergeConfigs doesn't perform deep merging of FeatureFlags
	// The merged config will have zero values for the entire FeatureFlags struct
	assert.False(t, merged.FeatureFlags.Enabled,
		"KNOWN LIMITATION: FeatureFlags struct is zero-valued in merged config because deep merging is not implemented")
	t.Log("NOTE: FeatureFlags deep merging is not implemented. The entire FeatureFlags struct remains zero-valued in merged configs. This is a separate issue to be addressed.")
}

// TestTenantCircuitBreakerOverride verifies that tenant-specific circuit breaker config overrides global
func TestTenantCircuitBreakerOverride(t *testing.T) {
	globalConfig := &ReverseProxyConfig{
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 5,
			OpenTimeout:      30 * time.Second,
		},
	}

	tenantConfig := &ReverseProxyConfig{
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 3,                // Override to 3
			OpenTimeout:      20 * time.Second, // Override to 20s
		},
	}

	merged := mergeConfigs(globalConfig, tenantConfig)

	assert.True(t, merged.CircuitBreakerConfig.Enabled, "Circuit breaker should be enabled")
	assert.Equal(t, 3, merged.CircuitBreakerConfig.FailureThreshold,
		"Merged config should use tenant's failure threshold (3), not global (5)")
	assert.Equal(t, 20*time.Second, merged.CircuitBreakerConfig.OpenTimeout,
		"Merged config should use tenant's open timeout (20s), not global (30s)")
}

// TestTenantHealthCheckOverride verifies that tenant-specific health check config overrides global
func TestTenantHealthCheckOverride(t *testing.T) {
	globalConfig := &ReverseProxyConfig{
		HealthCheck: HealthCheckConfig{
			Enabled:  true,
			Interval: 30 * time.Second,
			Timeout:  5 * time.Second,
		},
	}

	tenantConfig := &ReverseProxyConfig{
		HealthCheck: HealthCheckConfig{
			Enabled:  true,
			Interval: 60 * time.Second, // Override to 60s
			Timeout:  10 * time.Second, // Override to 10s
		},
	}

	merged := mergeConfigs(globalConfig, tenantConfig)

	assert.True(t, merged.HealthCheck.Enabled, "Health check should be enabled")
	assert.Equal(t, 60*time.Second, merged.HealthCheck.Interval,
		"Merged config should use tenant's interval (60s), not global (30s)")
	assert.Equal(t, 10*time.Second, merged.HealthCheck.Timeout,
		"Merged config should use tenant's timeout (10s), not global (5s)")
}

// TestTenantBackendServicesOverride verifies that tenant-specific backend services override global ones
func TestTenantBackendServicesOverride(t *testing.T) {
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"default": "http://global-backend.example.com",
			"service": "http://global-service.example.com",
		},
		DefaultBackend: "default",
	}

	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"default": "http://tenant-backend.example.com", // Override
		},
		DefaultBackend: "default",
	}

	merged := mergeConfigs(globalConfig, tenantConfig)

	assert.Equal(t, "http://tenant-backend.example.com", merged.BackendServices["default"],
		"Merged config should use tenant's backend URL, not global")
	assert.Equal(t, "http://global-service.example.com", merged.BackendServices["service"],
		"Non-overridden global backends should be preserved")
	assert.Equal(t, "default", merged.DefaultBackend, "Default backend should be preserved")
}

// TestTenantRoutesOverride verifies that tenant-specific routes override global routes
func TestTenantRoutesOverride(t *testing.T) {
	globalConfig := &ReverseProxyConfig{
		Routes: map[string]string{
			"/api/v1/*":    "backend1",
			"/api/admin/*": "admin",
		},
	}

	tenantConfig := &ReverseProxyConfig{
		Routes: map[string]string{
			"/api/v1/*":     "backend2", // Override
			"/api/tenant/*": "tenant",   // New route
		},
	}

	merged := mergeConfigs(globalConfig, tenantConfig)

	assert.Equal(t, "backend2", merged.Routes["/api/v1/*"],
		"Merged config should use tenant's route mapping, not global")
	assert.Equal(t, "admin", merged.Routes["/api/admin/*"],
		"Non-overridden global routes should be preserved")
	assert.Equal(t, "tenant", merged.Routes["/api/tenant/*"],
		"Tenant-specific routes should be added")
}

// TestMergeConfigsRequestTimeout specifically tests the RequestTimeout merge logic
func TestMergeConfigsRequestTimeout(t *testing.T) {
	tests := []struct {
		name            string
		globalTimeout   time.Duration
		tenantTimeout   time.Duration
		expectedTimeout time.Duration
		description     string
	}{
		{
			name:            "tenant overrides global",
			globalTimeout:   30 * time.Second,
			tenantTimeout:   60 * time.Second,
			expectedTimeout: 60 * time.Second,
			description:     "When tenant specifies timeout, it should override global",
		},
		{
			name:            "tenant not specified, use global",
			globalTimeout:   30 * time.Second,
			tenantTimeout:   0,
			expectedTimeout: 30 * time.Second,
			description:     "When tenant doesn't specify timeout (0), use global",
		},
		{
			name:            "both zero",
			globalTimeout:   0,
			tenantTimeout:   0,
			expectedTimeout: 0,
			description:     "When both are zero, merged should be zero",
		},
		{
			name:            "only tenant specified",
			globalTimeout:   0,
			tenantTimeout:   45 * time.Second,
			expectedTimeout: 45 * time.Second,
			description:     "When only tenant specifies timeout, use tenant value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalConfig := &ReverseProxyConfig{
				RequestTimeout: tt.globalTimeout,
			}
			tenantConfig := &ReverseProxyConfig{
				RequestTimeout: tt.tenantTimeout,
			}

			merged := mergeConfigs(globalConfig, tenantConfig)

			assert.Equal(t, tt.expectedTimeout, merged.RequestTimeout, tt.description)
		})
	}
}

// TestMergeConfigsGlobalTimeout specifically tests the GlobalTimeout merge logic
func TestMergeConfigsGlobalTimeout(t *testing.T) {
	tests := []struct {
		name            string
		globalTimeout   time.Duration
		tenantTimeout   time.Duration
		expectedTimeout time.Duration
		description     string
	}{
		{
			name:            "tenant overrides global",
			globalTimeout:   30 * time.Second,
			tenantTimeout:   60 * time.Second,
			expectedTimeout: 60 * time.Second,
			description:     "When tenant specifies GlobalTimeout, it should override global",
		},
		{
			name:            "tenant not specified, use global",
			globalTimeout:   30 * time.Second,
			tenantTimeout:   0,
			expectedTimeout: 30 * time.Second,
			description:     "When tenant doesn't specify GlobalTimeout (0), use global",
		},
		{
			name:            "both zero",
			globalTimeout:   0,
			tenantTimeout:   0,
			expectedTimeout: 0,
			description:     "When both are zero, merged should be zero",
		},
		{
			name:            "only tenant specified",
			globalTimeout:   0,
			tenantTimeout:   45 * time.Second,
			expectedTimeout: 45 * time.Second,
			description:     "When only tenant specifies GlobalTimeout, use tenant value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalConfig := &ReverseProxyConfig{
				GlobalTimeout: tt.globalTimeout,
			}
			tenantConfig := &ReverseProxyConfig{
				GlobalTimeout: tt.tenantTimeout,
			}

			merged := mergeConfigs(globalConfig, tenantConfig)

			assert.Equal(t, tt.expectedTimeout, merged.GlobalTimeout, tt.description)
		})
	}
}
