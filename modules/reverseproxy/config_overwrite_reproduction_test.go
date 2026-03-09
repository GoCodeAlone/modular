package reverseproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigOverwriteReproduction reproduces the exact issue where CacheTTL gets overwritten
// This test FAILS when the bug is present and PASSES when fixed
func TestConfigOverwriteReproduction(t *testing.T) {
	// Create a test backend server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend response"))
	}))
	defer testServer.Close()

	// Create the config with a specific CacheTTL value
	originalConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"cached-backend": testServer.URL,
		},
		Routes: map[string]string{
			"/api/cached": "cached-backend",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"cached-backend": {URL: testServer.URL},
		},
		DefaultBackend: "cached-backend",
		CacheEnabled:   true,
		CacheTTL:       1 * time.Second, // CRITICAL: This should NOT change
		HealthCheck: HealthCheckConfig{
			Enabled:  false,
			Interval: 30 * time.Second,
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:          false,
			FailureThreshold: 5,
			OpenTimeout:      30 * time.Second,
		},
	}

	// Capture the original value and pointer
	expectedCacheTTL := originalConfig.CacheTTL
	originalPointer := originalConfig
	originalCacheTTLAddress := &originalConfig.CacheTTL

	t.Logf("BEFORE setup: CacheTTL=%v, pointer=%p, TTL_address=%p",
		originalConfig.CacheTTL, originalConfig, originalCacheTTLAddress)

	// Create application
	app, err := modular.NewApplication(modular.WithLogger(&testLogger{}))
	require.NoError(t, err)

	// Register config section
	reverseproxyConfigProvider := modular.NewStdConfigProvider(originalConfig)
	app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Register required services
	require.NoError(t, app.RegisterService("logger", &testLogger{}))
	testRouterInstance := &testRouter{routes: make(map[string]http.HandlerFunc)}
	require.NoError(t, app.RegisterService("router", testRouterInstance))
	require.NoError(t, app.RegisterService("metrics", &testMetrics{}))

	tenantService := &MockTenantService{
		Configs: make(map[modular.TenantID]map[string]modular.ConfigProvider),
	}
	require.NoError(t, app.RegisterService("tenantService", tenantService))

	eventObserver := newTestEventObserver()
	require.NoError(t, app.RegisterService("event-bus", &testEventBus{observers: []modular.Observer{eventObserver}}))

	// Create and register module
	module := NewModule()
	constructor := module.Constructor()
	services := map[string]any{"router": testRouterInstance}

	constructedModule, err := constructor(app, services)
	require.NoError(t, err)

	module = constructedModule.(*ReverseProxyModule)
	app.RegisterModule(module)

	// THIS IS THE CRITICAL POINT - Init() should NOT modify our config
	t.Logf("BEFORE Init(): CacheTTL=%v", originalConfig.CacheTTL)

	err = app.Init()
	require.NoError(t, err)

	t.Logf("AFTER Init(): CacheTTL=%v", originalConfig.CacheTTL)

	// THE BUG: originalConfig.CacheTTL gets modified during Init()
	// Check if config was modified
	if originalConfig.CacheTTL != expectedCacheTTL {
		t.Errorf("❌ BUG REPRODUCED: CacheTTL was modified!")
		t.Errorf("   Expected: %v", expectedCacheTTL)
		t.Errorf("   Got:      %v", originalConfig.CacheTTL)
		t.Errorf("   Config pointer: %p (original: %p)", originalConfig, originalPointer)
		t.Errorf("   TTL address: %p (original: %p)", &originalConfig.CacheTTL, originalCacheTTLAddress)

		// Check if it's a specific known bad value
		if originalConfig.CacheTTL == 300*time.Second {
			t.Errorf("   Got 300s - this is the 'Response caching' scenario config!")
		} else if originalConfig.CacheTTL == 120*time.Second {
			t.Errorf("   Got 120s - this is from config_merge_test.go or module_test.go!")
		}
	} else {
		t.Logf("✅ Config preserved: CacheTTL=%v (unchanged)", originalConfig.CacheTTL)
	}

	// Also verify the module received the correct config
	assert.Equal(t, expectedCacheTTL, module.config.CacheTTL,
		"Module should have received the original CacheTTL value")

	// Start the app to complete setup
	err = app.Start()
	require.NoError(t, err)

	// Final check after Start()
	assert.Equal(t, expectedCacheTTL, originalConfig.CacheTTL,
		"CacheTTL should still be unchanged after Start()")
	assert.Equal(t, expectedCacheTTL, module.config.CacheTTL,
		"Module's CacheTTL should match original value")
}

// TestConfigOverwriteWithPriorTests simulates running after other tests that set different CacheTTL
func TestConfigOverwriteWithPriorTests(t *testing.T) {
	// First, simulate a test that uses 300s CacheTTL
	t.Run("PriorTest_With300s", func(t *testing.T) {
		config := &ReverseProxyConfig{
			CacheTTL:     300 * time.Second,
			CacheEnabled: true,
			BackendServices: map[string]string{
				"backend": "http://localhost:8000",
			},
		}

		// Create and initialize app with 300s config
		app, err := modular.NewApplication(modular.WithLogger(&testLogger{}))
		require.NoError(t, err)

		provider := modular.NewStdConfigProvider(config)
		app.RegisterConfigSection("reverseproxy", provider)

		t.Logf("Prior test set CacheTTL=300s, config=%p", config)
	})

	// Now run a test that should use 1s CacheTTL
	t.Run("CurrentTest_With1s", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		config := &ReverseProxyConfig{
			CacheTTL:     1 * time.Second, // Should be 1s
			CacheEnabled: true,
			BackendServices: map[string]string{
				"backend": testServer.URL,
			},
			DefaultBackend: "backend",
		}

		originalTTL := config.CacheTTL
		t.Logf("Current test set CacheTTL=1s, config=%p", config)

		// Setup app
		app, err := modular.NewApplication(modular.WithLogger(&testLogger{}))
		require.NoError(t, err)

		provider := modular.NewStdConfigProvider(config)
		app.RegisterConfigSection("reverseproxy", provider)

		require.NoError(t, app.RegisterService("logger", &testLogger{}))
		require.NoError(t, app.RegisterService("router", &testRouter{routes: make(map[string]http.HandlerFunc)}))
		require.NoError(t, app.RegisterService("metrics", &testMetrics{}))

		module := NewModule()
		app.RegisterModule(module)

		err = app.Init()
		require.NoError(t, err)

		// Check if prior test's config leaked in
		if config.CacheTTL != originalTTL {
			t.Errorf("❌ Config leaked from prior test!")
			t.Errorf("   Expected: %v", originalTTL)
			t.Errorf("   Got:      %v", config.CacheTTL)
			if config.CacheTTL == 300*time.Second {
				t.Errorf("   This is the 300s value from the prior test!")
			}
		} else {
			t.Logf("✅ No leakage: CacheTTL=%v", config.CacheTTL)
		}
	})
}

// TestConfigProviderBehavior tests how NewStdConfigProvider handles config objects
func TestConfigProviderBehavior(t *testing.T) {
	t.Run("ConfigProviderWithPointer", func(t *testing.T) {
		config := &ReverseProxyConfig{
			CacheTTL: 1 * time.Second,
		}

		provider := modular.NewStdConfigProvider(config)

		// Modify the original config
		config.CacheTTL = 2 * time.Second

		t.Logf("Original config pointer: %p, CacheTTL=%v", config, config.CacheTTL)

		// ConfigProvider stores references, so modifications will be visible
		t.Logf("⚠️  ConfigProvider stores pointer reference, not a copy!")
		t.Logf("   Modifications to original config will affect the provider")

		// This is expected behavior - just documenting it
		assert.Equal(t, 2*time.Second, config.CacheTTL, "Config should be modified")

		_ = provider // Use the provider variable
	})
}
