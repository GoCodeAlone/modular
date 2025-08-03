package reverseproxy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestMergeConfigs tests the configuration merging functionality
func TestMergeConfigs(t *testing.T) {
	// Create a global config with multiple backends
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"legacy":   "http://legacy-global.example.com",   // Will be overridden by tenant
			"chimera":  "http://chimera-global.example.com",  // Should be preserved
			"internal": "http://internal-global.example.com", // Should be preserved
		},
		DefaultBackend: "chimera",
		Routes: map[string]string{
			"/api/v1/*":       "legacy",
			"/api/internal/*": "internal",
		},
		CompositeRoutes: map[string]CompositeRoute{
			"/api/compose": {
				Pattern:  "/api/compose",
				Backends: []string{"legacy", "chimera"},
				Strategy: "merge",
			},
		},
		TenantIDHeader:  "X-Global-Tenant-ID", // Will be overridden
		RequireTenantID: false,                // Will be overridden
		CacheEnabled:    true,
		CacheTTL:        120 * time.Second,
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 5,
			OpenTimeout:      30 * time.Second,
		},
		BackendCircuitBreakers: map[string]CircuitBreakerConfig{
			"legacy": {
				Enabled:          true,
				FailureThreshold: 10,
				OpenTimeout:      60 * time.Second,
			},
		},
	}

	// Create a tenant config that overrides some settings but not others
	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"legacy": "http://legacy-tenant.example.com", // Override the legacy backend
			// chimera is not defined, should be inherited from global
			"tenant": "http://tenant-specific.example.com", // New backend only in tenant config
		},
		DefaultBackend: "legacy", // Override default backend
		Routes: map[string]string{
			"/api/tenant/*": "tenant", // New route
			"/api/v1/*":     "legacy", // Override route
		},
		TenantIDHeader:  "X-Tenant-ID",    // Override header
		RequireTenantID: true,             // Override requirement
		CacheEnabled:    true,             // Same as global but explicitly set
		CacheTTL:        60 * time.Second, // Override TTL
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:          true,             // Same as global
			FailureThreshold: 3,                // Override threshold
			OpenTimeout:      20 * time.Second, // Override timeout
		},
		BackendCircuitBreakers: map[string]CircuitBreakerConfig{
			"tenant": { // New backend circuit breaker
				Enabled:          true,
				FailureThreshold: 8,
				OpenTimeout:      40 * time.Second,
			},
		},
	}

	// Merge the configurations
	mergedConfig := mergeConfigs(globalConfig, tenantConfig)

	// TEST 1: BackendServices should include both global and tenant backends with tenant overrides
	assert.Len(t, mergedConfig.BackendServices, 4, "Merged config should have 4 backend services")
	assert.Equal(t, "http://legacy-tenant.example.com", mergedConfig.BackendServices["legacy"], "Legacy backend should be overridden by tenant config")
	assert.Equal(t, "http://chimera-global.example.com", mergedConfig.BackendServices["chimera"], "Chimera backend should be preserved from global config")
	assert.Equal(t, "http://internal-global.example.com", mergedConfig.BackendServices["internal"], "Internal backend should be preserved from global config")
	assert.Equal(t, "http://tenant-specific.example.com", mergedConfig.BackendServices["tenant"], "Tenant-specific backend should be added")

	// TEST 2: DefaultBackend should be overridden by tenant
	assert.Equal(t, "legacy", mergedConfig.DefaultBackend, "Default backend should be overridden by tenant config")

	// TEST 3: Routes should combine global and tenant with tenant overrides
	assert.Len(t, mergedConfig.Routes, 3, "Merged config should have 3 routes")
	assert.Equal(t, "legacy", mergedConfig.Routes["/api/v1/*"], "API v1 route should point to legacy backend")
	assert.Equal(t, "internal", mergedConfig.Routes["/api/internal/*"], "Internal route should be preserved")
	assert.Equal(t, "tenant", mergedConfig.Routes["/api/tenant/*"], "Tenant route should be added")

	// TEST 4: CompositeRoutes should be preserved
	assert.Len(t, mergedConfig.CompositeRoutes, 1, "Composite routes should be preserved")
	assert.Equal(t, []string{"legacy", "chimera"}, mergedConfig.CompositeRoutes["/api/compose"].Backends)

	// TEST 5: TenantIDHeader should be overridden
	assert.Equal(t, "X-Tenant-ID", mergedConfig.TenantIDHeader, "TenantIDHeader should be overridden")

	// TEST 6: RequireTenantID should be overridden
	assert.True(t, mergedConfig.RequireTenantID, "RequireTenantID should be overridden to true")

	// TEST 7: CacheTTL should be overridden
	assert.Equal(t, 60*time.Second, mergedConfig.CacheTTL, "CacheTTL should be overridden")

	// TEST 8: CircuitBreakerConfig should be overridden
	assert.Equal(t, 3, mergedConfig.CircuitBreakerConfig.FailureThreshold, "CircuitBreaker threshold should be overridden")
	assert.Equal(t, 20*time.Second, mergedConfig.CircuitBreakerConfig.OpenTimeout, "CircuitBreaker timeout should be overridden")

	// TEST 9: BackendCircuitBreakers should be merged
	assert.Len(t, mergedConfig.BackendCircuitBreakers, 2, "BackendCircuitBreakers should be merged")
	assert.Equal(t, 10, mergedConfig.BackendCircuitBreakers["legacy"].FailureThreshold, "Legacy circuit breaker should be preserved")
	assert.Equal(t, 8, mergedConfig.BackendCircuitBreakers["tenant"].FailureThreshold, "Tenant circuit breaker should be added")
}

// TestPartialTenantConfig tests merging when tenant config only specifies partial values
func TestPartialTenantConfig(t *testing.T) {
	// Create a global config
	globalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"service1": "http://global-service1.example.com",
			"service2": "http://global-service2.example.com",
		},
		DefaultBackend:  "service1",
		TenantIDHeader:  "X-Tenant-ID",
		RequireTenantID: true,
		CacheEnabled:    true,
		CacheTTL:        60 * time.Second,
	}

	// Tenant config only overrides specific fields
	tenantConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			// Only override service1
			"service1": "http://tenant-service1.example.com",
		},
		// DefaultBackend not specified, should use global
		// Routes not specified, should use global
	}

	// Merge the configurations
	mergedConfig := mergeConfigs(globalConfig, tenantConfig)

	// Test that service1 is overridden but service2 is preserved
	assert.Equal(t, "http://tenant-service1.example.com", mergedConfig.BackendServices["service1"],
		"Service1 should be overridden by tenant config")
	assert.Equal(t, "http://global-service2.example.com", mergedConfig.BackendServices["service2"],
		"Service2 should be preserved from global config")

	// Test that other fields keep global values when not specified in tenant config
	assert.Equal(t, "service1", mergedConfig.DefaultBackend,
		"DefaultBackend should preserve global value when not specified in tenant config")
	assert.Equal(t, "X-Tenant-ID", mergedConfig.TenantIDHeader,
		"TenantIDHeader should preserve global value when not specified in tenant config")
	assert.True(t, mergedConfig.RequireTenantID,
		"RequireTenantID should preserve global value when not specified in tenant config")
	assert.True(t, mergedConfig.CacheEnabled,
		"CacheEnabled should preserve global value when not specified in tenant config")
	assert.Equal(t, 60*time.Second, mergedConfig.CacheTTL,
		"CacheTTL should preserve global value when not specified in tenant config")
}
