package modular

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TenantAwareConfigTestModule is a test module that implements TenantAwareModule
type TenantAwareConfigTestModule struct {
	name                  string
	tenantConfigs         map[TenantID]*TestTenantConfig
	tenantRegisteredCalls int
	tenantRemovedCalls    int
}

func NewTenantAwareConfigTestModule(name string) *TenantAwareConfigTestModule {
	return &TenantAwareConfigTestModule{
		name:          name,
		tenantConfigs: make(map[TenantID]*TestTenantConfig),
	}
}

func (m *TenantAwareConfigTestModule) Name() string {
	return m.name
}

func (m *TenantAwareConfigTestModule) Init(app Application) error {
	return nil
}

func (m *TenantAwareConfigTestModule) Start(ctx context.Context) error {
	return nil
}

func (m *TenantAwareConfigTestModule) Stop(ctx context.Context) error {
	return nil
}

func (m *TenantAwareConfigTestModule) Dependencies() []string {
	return []string{}
}

func (m *TenantAwareConfigTestModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{}
}

func (m *TenantAwareConfigTestModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{}
}

func (m *TenantAwareConfigTestModule) RegisterConfig(app Application) {
	// Register a test config section
	app.RegisterConfigSection("TestConfig", NewStdConfigProvider(&TestTenantConfig{}))
}

func (m *TenantAwareConfigTestModule) OnTenantRegistered(tenantID TenantID) {
	m.tenantRegisteredCalls++
}

func (m *TenantAwareConfigTestModule) OnTenantRemoved(tenantID TenantID) {
	m.tenantRemovedCalls++
}

func (m *TenantAwareConfigTestModule) LoadTenantConfig(tenantService TenantService, tenantID TenantID) error {
	config, err := tenantService.GetTenantConfig(tenantID, "TestConfig")
	if err != nil {
		return err
	}

	if testConfig, ok := config.GetConfig().(*TestTenantConfig); ok {
		m.tenantConfigs[tenantID] = testConfig
	}

	return nil
}

func TestTenantAwareConfigModule(t *testing.T) {
	// Create a temporary directory for tenant config files
	tempDir, err := os.MkdirTemp("", "tenant-aware-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create tenant config files
	tenant1Config := `{
		"TestConfig": {
			"Name": "Tenant1",
			"Environment": "test",
			"Features": {
				"feature1": true,
				"feature2": false
			}
		}
	}`

	tenant2Config := `{
		"TestConfig": {
			"Name": "Tenant2",
			"Environment": "production",
			"Features": {
				"feature1": false,
				"feature2": true
			}
		}
	}`

	err = os.WriteFile(filepath.Join(tempDir, "tenant1.json"), []byte(tenant1Config), 0644)
	if err != nil {
		t.Fatalf("Failed to write tenant1.json: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "tenant2.json"), []byte(tenant2Config), 0644)
	if err != nil {
		t.Fatalf("Failed to write tenant2.json: %v", err)
	}

	// Create test app with tenant-aware module
	log := &logger{t}
	app := NewStdApplication(NewStdConfigProvider(nil), log)

	tm := NewTenantAwareConfigTestModule("TestModule")
	app.RegisterModule(tm)

	// Create tenant service
	tenantService := NewStandardTenantService(log)

	// Register the tenant service with the module
	app.RegisterService("tenantService", tenantService)

	// Setup TenantConfigLoader
	loader := NewFileBasedTenantConfigLoader(TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile("^tenant[0-9]+\\.json$"),
		ConfigDir:       tempDir,
	})
	app.RegisterService("tenantConfigLoader", loader)

	// Register the tenant-aware module with the tenant service
	tenantService.RegisterTenantAwareModule(tm)

	app.RegisterConfigSection("TestConfig", NewStdConfigProvider(&TestTenantConfig{}))

	// Initialize the module to register its config section
	err = tm.Init(app)
	if err != nil {
		t.Fatalf("Failed to initialize test module: %v", err)
	}

	err = app.(*StdApplication).initTenantConfigurations()
	if err != nil {
		t.Fatalf("Failed to initialize tenant configurations: %v", err)
	}

	// Verify tenants were registered
	if tm.tenantRegisteredCalls != 2 {
		t.Errorf("Expected 2 tenant registered calls, got %d", tm.tenantRegisteredCalls)
	}

	// Load tenant configs in the module
	tenants := tenantService.GetTenants()
	for _, tenantID := range tenants {
		err = tm.LoadTenantConfig(tenantService, tenantID)
		if err != nil {
			t.Errorf("Failed to load tenant config for tenant %s: %v", tenantID, err)
		}
	}

	// Verify tenant configs were loaded correctly
	tenant1ID := TenantID("tenant1")
	tenant1Cfg, exists := tm.tenantConfigs[tenant1ID]
	if !exists {
		t.Errorf("Expected tenant1 config to be loaded")
	} else {
		if tenant1Cfg.Name != "Tenant1" {
			t.Errorf("Expected tenant1 Name to be 'Tenant1', got '%s'", tenant1Cfg.Name)
		}
		if tenant1Cfg.Environment != "test" {
			t.Errorf("Expected tenant1 Environment to be 'test', got '%s'", tenant1Cfg.Environment)
		}
		if !tenant1Cfg.Features["feature1"] {
			t.Errorf("Expected tenant1 Features['feature1'] to be true")
		}
	}

	tenant2ID := TenantID("tenant2")
	tenant2Cfg, exists := tm.tenantConfigs[tenant2ID]
	if !exists {
		t.Errorf("Expected tenant2 config to be loaded")
	} else {
		if tenant2Cfg.Name != "Tenant2" {
			t.Errorf("Expected tenant2 Name to be 'Tenant2', got '%s'", tenant2Cfg.Name)
		}
		if tenant2Cfg.Environment != "production" {
			t.Errorf("Expected tenant2 Environment to be 'production', got '%s'", tenant2Cfg.Environment)
		}
		if tenant2Cfg.Features["feature1"] {
			t.Errorf("Expected tenant2 Features['feature1'] to be false")
		}
	}

	// Test tenant removal
	err = tenantService.RemoveTenant(tenant1ID)
	if err != nil {
		t.Errorf("Failed to remove tenant: %v", err)
	}

	if tm.tenantRemovedCalls != 1 {
		t.Errorf("Expected 1 tenant removed call, got %d", tm.tenantRemovedCalls)
	}
}

func TestTenantContextWithConfig(t *testing.T) {
	// Create a tenant service with predefined configs
	log := &logger{t}
	tenantService := NewStandardTenantService(log)

	// Register a tenant with config
	tenant1ID := TenantID("tenant1")
	testConfig := &TestTenantConfig{
		Name:        "Tenant1",
		Environment: "test",
		Features:    map[string]bool{"feature1": true},
	}

	configs := map[string]ConfigProvider{
		"TestConfig": NewStdConfigProvider(testConfig),
	}

	err := tenantService.RegisterTenant(tenant1ID, configs)
	if err != nil {
		t.Fatalf("Failed to register tenant: %v", err)
	}

	// Create a tenant context
	ctx := context.Background()
	tenantCtx := NewTenantContext(ctx, tenant1ID)

	// Test getting tenant config via context
	retrievedID, ok := GetTenantIDFromContext(tenantCtx)
	if !ok {
		t.Fatal("Failed to get tenant ID from context")
	}

	if retrievedID != tenant1ID {
		t.Errorf("Expected tenant ID %s, got %s", tenant1ID, retrievedID)
	}

	configProvider, err := tenantService.GetTenantConfig(retrievedID, "TestConfig")
	if err != nil {
		t.Fatalf("Failed to get tenant config: %v", err)
	}

	cfg, ok := configProvider.GetConfig().(*TestTenantConfig)
	if !ok {
		t.Fatalf("Expected *TestTenantConfig, got %T", configProvider.GetConfig())
	}

	if cfg.Name != "Tenant1" {
		t.Errorf("Expected Name 'Tenant1', got '%s'", cfg.Name)
	}
}

func TestTenantConfigMerging(t *testing.T) {
	// Create a tenant service
	log := &logger{t}
	tenantService := NewStandardTenantService(log)

	// Register a tenant with initial config
	tenant1ID := TenantID("tenant1")
	initialConfig := &TestTenantConfig{
		Name:        "Initial",
		Environment: "dev",
		Features:    map[string]bool{"feature1": true},
	}

	configs := map[string]ConfigProvider{
		"TestConfig": NewStdConfigProvider(initialConfig),
	}

	err := tenantService.RegisterTenant(tenant1ID, configs)
	if err != nil {
		t.Fatalf("Failed to register tenant: %v", err)
	}

	// Verify initial config
	configProvider, err := tenantService.GetTenantConfig(tenant1ID, "TestConfig")
	if err != nil {
		t.Fatalf("Failed to get tenant config: %v", err)
	}

	cfg, ok := configProvider.GetConfig().(*TestTenantConfig)
	if !ok {
		t.Fatalf("Expected *TestTenantConfig, got %T", configProvider.GetConfig())
	}

	if cfg.Name != "Initial" {
		t.Errorf("Expected Name 'Initial', got '%s'", cfg.Name)
	}

	// Update tenant config
	updatedConfig := &TestTenantConfig{
		Name:        "Updated",
		Environment: "prod",
		Features:    map[string]bool{"feature2": true},
	}

	updatedConfigs := map[string]ConfigProvider{
		"TestConfig": NewStdConfigProvider(updatedConfig),
	}

	// Register same tenant again with new config (should merge)
	err = tenantService.RegisterTenant(tenant1ID, updatedConfigs)
	if err != nil {
		t.Fatalf("Failed to update tenant config: %v", err)
	}

	// Verify updated config
	configProvider, err = tenantService.GetTenantConfig(tenant1ID, "TestConfig")
	if err != nil {
		t.Fatalf("Failed to get updated tenant config: %v", err)
	}

	updatedCfg, ok := configProvider.GetConfig().(*TestTenantConfig)
	if !ok {
		t.Fatalf("Expected *TestTenantConfig, got %T", configProvider.GetConfig())
	}

	if updatedCfg.Name != "Updated" {
		t.Errorf("Expected updated Name 'Updated', got '%s'", updatedCfg.Name)
	}

	if updatedCfg.Environment != "prod" {
		t.Errorf("Expected Environment 'prod', got '%s'", updatedCfg.Environment)
	}

	// Add a new config section
	anotherConfig := &AnotherTestConfig{
		ApiKey:         "test-key",
		MaxConnections: 5,
		Timeout:        30,
	}

	additionalConfigs := map[string]ConfigProvider{
		"ApiConfig": NewStdConfigProvider(anotherConfig),
	}

	// Register same tenant again with additional config section
	err = tenantService.RegisterTenant(tenant1ID, additionalConfigs)
	if err != nil {
		t.Fatalf("Failed to add config section: %v", err)
	}

	// Verify both config sections exist
	apiConfigProvider, err := tenantService.GetTenantConfig(tenant1ID, "ApiConfig")
	if err != nil {
		t.Fatalf("Failed to get ApiConfig: %v", err)
	}

	apiCfg, ok := apiConfigProvider.GetConfig().(*AnotherTestConfig)
	if !ok {
		t.Fatalf("Expected *AnotherTestConfig, got %T", apiConfigProvider.GetConfig())
	}

	if apiCfg.ApiKey != "test-key" {
		t.Errorf("Expected ApiKey 'test-key', got '%s'", apiCfg.ApiKey)
	}

	// Original TestConfig section should still exist
	_, err = tenantService.GetTenantConfig(tenant1ID, "TestConfig")
	if err != nil {
		t.Errorf("TestConfig section should still exist after adding ApiConfig")
	}
}
