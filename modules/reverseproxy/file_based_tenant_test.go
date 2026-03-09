package reverseproxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultipleTenantsSameBackendOverride_FileBasedLoader tests the edge case
// using actual YAML files and FileBasedTenantConfigLoader to exactly reproduce
// the production scenario described in GitHub issue #111.
func TestMultipleTenantsSameBackendOverride_FileBasedLoader(t *testing.T) {
	// Setup mock backend servers
	globalBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"global","url":"` + r.Host + `"}`))
	}))
	defer globalBackend.Close()

	tenant1Backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"tenant1","url":"` + r.Host + `"}`))
	}))
	defer tenant1Backend.Close()

	tenant2Backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"backend":"tenant2","url":"` + r.Host + `"}`))
	}))
	defer tenant2Backend.Close()

	// Create temporary directory for config files
	tmpDir, err := os.MkdirTemp("", "reverseproxy-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config directory for tenant configs
	tenantConfigDir := filepath.Join(tmpDir, "tenants")
	err = os.MkdirAll(tenantConfigDir, 0755)
	require.NoError(t, err)

	// Write global config file
	globalConfigYAML := `
reverseproxy:
  backend_services:
    api: ` + globalBackend.URL + `
  default_backend: api
  tenant_id_header: X-Affiliate-Id
  require_tenant_id: true
`
	globalConfigPath := filepath.Join(tmpDir, "global.yaml")
	err = os.WriteFile(globalConfigPath, []byte(globalConfigYAML), 0644)
	require.NoError(t, err)

	t.Logf("Global backend URL: %s", globalBackend.URL)
	t.Logf("Tenant1 backend URL: %s", tenant1Backend.URL)
	t.Logf("Tenant2 backend URL: %s", tenant2Backend.URL)

	// Write tenant1 config file - overrides "api" backend
	tenant1ConfigYAML := `
reverseproxy:
  backend_services:
    api: ` + tenant1Backend.URL + `
`
	tenant1ConfigPath := filepath.Join(tenantConfigDir, "tenant1.yaml")
	err = os.WriteFile(tenant1ConfigPath, []byte(tenant1ConfigYAML), 0644)
	require.NoError(t, err)

	// Write tenant2 config file - ALSO overrides "api" backend with DIFFERENT URL
	tenant2ConfigYAML := `
reverseproxy:
  backend_services:
    api: ` + tenant2Backend.URL + `
`
	tenant2ConfigPath := filepath.Join(tenantConfigDir, "tenant2.yaml")
	err = os.WriteFile(tenant2ConfigPath, []byte(tenant2ConfigYAML), 0644)
	require.NoError(t, err)

	// Create and populate global config BEFORE wrapping in providers
	// This avoids mutating config after it's been wrapped
	globalCfg := ProvideConfig().(*ReverseProxyConfig)
	globalCfg.BackendServices = map[string]string{
		"api": globalBackend.URL,
	}
	globalCfg.DefaultBackend = "api"
	globalCfg.TenantIDHeader = "X-Affiliate-Id"
	globalCfg.RequireTenantID = true

	// Create a real application (not a mock)
	// Use IsolatedConfigProvider to ensure deep copies are returned on every GetConfig()
	// Note: Using the same globalCfg instance for both providers is safe because
	// IsolatedConfigProvider performs deep copies, preventing shared state issues
	app := modular.NewStdApplication(modular.NewIsolatedConfigProvider(globalCfg), NewMockLogger())

	// Register the reverseproxy config section with IsolatedConfigProvider to prevent sharing
	app.RegisterConfigSection("reverseproxy", modular.NewIsolatedConfigProvider(globalCfg))

	// Register tenant service
	tenantService := modular.NewStandardTenantService(app.Logger())
	err = app.RegisterService("tenantService", tenantService)
	require.NoError(t, err)

	// Register file-based tenant config loader
	tenantConfigLoader := modular.NewFileBasedTenantConfigLoader(modular.TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^tenant\d+\.yaml$`),
		ConfigDir:       tenantConfigDir,
		ConfigFeeders:   []modular.Feeder{},
	})
	err = app.RegisterService("tenantConfigLoader", tenantConfigLoader)
	require.NoError(t, err)

	// Create and register the reverseproxy module
	rpModule := NewModule()
	app.RegisterModule(rpModule)

	// Register a test router
	mockRouter := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}
	err = app.RegisterService("router", mockRouter)
	require.NoError(t, err)

	// THIS IS THE KEY SEQUENCE FROM THE ISSUE:
	// 1. Init() runs - reverseproxy.Init() completes with 0 tenants
	t.Log("Step 1: Calling app.Init() - reverseproxy should initialize with 0 tenants")
	err = app.Init()
	require.NoError(t, err)

	// 2. Between Init() and Start(), framework loads tenant configs
	t.Log("Step 2: Loading tenant configurations from files")
	err = tenantConfigLoader.LoadTenantConfigurations(app, tenantService)
	require.NoError(t, err)

	// Verify tenants were loaded
	tenants := tenantService.GetTenants()
	require.Len(t, tenants, 2, "Should have loaded 2 tenants")
	t.Logf("Loaded tenants: %v", tenants)

	// 3. Start() runs - should create tenant-specific proxies
	t.Log("Step 3: Calling app.Start() - should create tenant proxies")
	err = app.Start()
	require.NoError(t, err)

	// Now verify the proxies are correctly set up
	t.Log("Step 4: Verifying tenant proxy configuration")

	// Get the module to inspect its state
	// The module is just rpModule - we already have direct access to it
	modulePtr := rpModule
	require.NotNil(t, modulePtr)

	// Check tenant1 configuration
	tenant1ID := modular.TenantID("tenant1")
	tenant1Cfg, exists := modulePtr.tenants[tenant1ID]
	require.True(t, exists, "Tenant1 config should exist")
	require.NotNil(t, tenant1Cfg, "Tenant1 config should not be nil")
	t.Logf("Tenant1 config backend URL: %s", tenant1Cfg.BackendServices["api"])
	assert.Equal(t, tenant1Backend.URL, tenant1Cfg.BackendServices["api"],
		"Tenant1 config should have tenant1's backend URL")

	// Check tenant2 configuration
	tenant2ID := modular.TenantID("tenant2")
	tenant2Cfg, exists := modulePtr.tenants[tenant2ID]
	require.True(t, exists, "Tenant2 config should exist")
	require.NotNil(t, tenant2Cfg, "Tenant2 config should not be nil")
	t.Logf("Tenant2 config backend URL: %s", tenant2Cfg.BackendServices["api"])
	assert.Equal(t, tenant2Backend.URL, tenant2Cfg.BackendServices["api"],
		"Tenant2 config should have tenant2's backend URL")

	// Check tenant1 proxy
	tenant1Proxies, exists := modulePtr.tenantBackendProxies[tenant1ID]
	require.True(t, exists, "Tenant1 proxy map should exist")
	require.NotNil(t, tenant1Proxies, "Tenant1 proxies should not be nil")

	tenant1APIProxy, exists := tenant1Proxies["api"]
	require.True(t, exists, "Tenant1 'api' proxy should exist")
	require.NotNil(t, tenant1APIProxy, "Tenant1 'api' proxy should not be nil")
	t.Logf("Tenant1 API proxy: %p", tenant1APIProxy)

	// Check tenant2 proxy
	tenant2Proxies, exists := modulePtr.tenantBackendProxies[tenant2ID]
	require.True(t, exists, "Tenant2 proxy map should exist")
	require.NotNil(t, tenant2Proxies, "Tenant2 proxies should not be nil")

	tenant2APIProxy, exists := tenant2Proxies["api"]
	require.True(t, exists, "Tenant2 'api' proxy should exist")
	require.NotNil(t, tenant2APIProxy, "Tenant2 'api' proxy should not be nil")
	t.Logf("Tenant2 API proxy: %p", tenant2APIProxy)

	// The two proxies should be different instances
	assert.NotEqual(t, tenant1APIProxy, tenant2APIProxy,
		"Tenant1 and tenant2 proxies should be different instances")

	// Step 5: Test actual requests through the proxies
	t.Log("Step 5: Testing actual HTTP requests through tenant proxies")

	// Test tenant1 request
	tenant1Proxy, exists := modulePtr.getProxyForBackendAndTenant("api", tenant1ID)
	require.True(t, exists, "Tenant1 proxy should be retrievable")
	require.NotNil(t, tenant1Proxy, "Tenant1 proxy should not be nil")

	tenant1Req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	tenant1Req.Header.Set("X-Affiliate-Id", string(tenant1ID))
	tenant1W := httptest.NewRecorder()
	tenant1Proxy.ServeHTTP(tenant1W, tenant1Req)

	tenant1Resp := tenant1W.Result()
	assert.Equal(t, http.StatusOK, tenant1Resp.StatusCode)
	tenant1Body, err := io.ReadAll(tenant1Resp.Body)
	require.NoError(t, err)
	t.Logf("Tenant1 response: %s", string(tenant1Body))

	var tenant1Data map[string]interface{}
	err = json.Unmarshal(tenant1Body, &tenant1Data)
	require.NoError(t, err)

	// CRITICAL ASSERTION: Tenant1 should get tenant1's backend
	assert.Equal(t, "tenant1", tenant1Data["backend"],
		"Request with tenant1 ID should route to tenant1's backend")

	// Test tenant2 request
	tenant2Proxy, exists := modulePtr.getProxyForBackendAndTenant("api", tenant2ID)
	require.True(t, exists, "Tenant2 proxy should be retrievable")
	require.NotNil(t, tenant2Proxy, "Tenant2 proxy should not be nil")

	tenant2Req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	tenant2Req.Header.Set("X-Affiliate-Id", string(tenant2ID))
	tenant2W := httptest.NewRecorder()
	tenant2Proxy.ServeHTTP(tenant2W, tenant2Req)

	tenant2Resp := tenant2W.Result()
	assert.Equal(t, http.StatusOK, tenant2Resp.StatusCode)
	tenant2Body, err := io.ReadAll(tenant2Resp.Body)
	require.NoError(t, err)
	t.Logf("Tenant2 response: %s", string(tenant2Body))

	var tenant2Data map[string]interface{}
	err = json.Unmarshal(tenant2Body, &tenant2Data)
	require.NoError(t, err)

	// CRITICAL ASSERTION: Tenant2 should get tenant2's backend, NOT tenant1's
	assert.Equal(t, "tenant2", tenant2Data["backend"],
		"Request with tenant2 ID should route to tenant2's backend (NOT tenant1's backend)")

	// Clean up
	err = app.Stop()
	require.NoError(t, err)
}
