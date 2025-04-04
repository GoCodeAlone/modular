package modular

import (
	"context"
	"fmt"
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

func (m *TenantAwareConfigTestModule) Init(Application) error {
	return nil
}

func (m *TenantAwareConfigTestModule) Start(context.Context) error {
	return nil
}

func (m *TenantAwareConfigTestModule) Stop(context.Context) error {
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

func (m *TenantAwareConfigTestModule) OnTenantRegistered(TenantID) {
	m.tenantRegisteredCalls++
}

func (m *TenantAwareConfigTestModule) OnTenantRemoved(TenantID) {
	m.tenantRemovedCalls++
}

func (m *TenantAwareConfigTestModule) LoadTenantConfig(tenantService TenantService, tenantID TenantID) error {
	config, err := tenantService.GetTenantConfig(tenantID, "TestConfig")
	if err != nil {
		return fmt.Errorf("failed to get tenant config: %w", err)
	}

	if testConfig, ok := config.GetConfig().(*TestTenantConfig); ok {
		m.tenantConfigs[tenantID] = testConfig
	} else {
		return fmt.Errorf("%w: %s - %+v", ErrConfigCastFailed, tenantID, config.GetConfig())
	}

	return nil
}

// setupTempConfigDir creates a temporary directory with tenant config files and returns the directory path
func setupTempConfigDir(t *testing.T) string {
	t.Helper()

	// Create a temporary directory for tenant config files
	tempDir, err := os.MkdirTemp("", "tenant-aware-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

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

	err = os.WriteFile(filepath.Join(tempDir, "tenant1.json"), []byte(tenant1Config), 0600)
	if err != nil {
		t.Fatalf("Failed to write tenant1.json: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "tenant2.json"), []byte(tenant2Config), 0600)
	if err != nil {
		t.Fatalf("Failed to write tenant2.json: %v", err)
	}

	return tempDir
}

// setupTestApp creates and initializes the test application with necessary services
func setupTestApp(t *testing.T, tempDir string) (Application, *TenantAwareConfigTestModule, *StandardTenantService) {
	t.Helper()

	// Create test app with tenant-aware module
	log := &logger{t}
	app := NewStdApplication(NewStdConfigProvider(nil), log)

	tm := NewTenantAwareConfigTestModule("TestModule")
	app.RegisterModule(tm)

	// Create tenant service
	tenantService := NewStandardTenantService(log)

	// Register the tenant service with the module
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	// Setup TenantConfigLoader
	loader := NewFileBasedTenantConfigLoader(TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^tenant[0-9]+\.json$`),
		ConfigDir:       tempDir,
	})
	if err := app.RegisterService("tenantConfigLoader", loader); err != nil {
		t.Fatalf("Failed to register tenant config loader: %v", err)
	}

	// Register the tenant-aware module with the tenant service
	tenantService.RegisterTenantAwareModule(tm)

	app.RegisterConfigSection("TestConfig", NewStdConfigProvider(&TestTenantConfig{}))

	// Initialize the module to register its config section
	if err := tm.Init(app); err != nil {
		t.Fatalf("Failed to initialize test module: %v", err)
	}

	return app, tm, tenantService
}

// verifyTenantConfig checks if tenant configs were loaded correctly
func verifyTenantConfig(t *testing.T, tm *TenantAwareConfigTestModule, tenantID TenantID, expectedName, expectedEnv string, featureKey string, featureValue bool) {
	t.Helper()

	tenantCfg, exists := tm.tenantConfigs[tenantID]
	if !exists {
		t.Errorf("Expected %s config to be loaded", tenantID)
		return
	}

	if tenantCfg.Name != expectedName {
		t.Errorf("Expected %s Name to be '%s', got '%s'", tenantID, expectedName, tenantCfg.Name)
	}

	if tenantCfg.Environment != expectedEnv {
		t.Errorf("Expected %s Environment to be '%s', got '%s'", tenantID, expectedEnv, tenantCfg.Environment)
	}

	if tenantCfg.Features[featureKey] != featureValue {
		t.Errorf("Expected %s Features['%s'] to be %v", tenantID, featureKey, featureValue)
	}
}

// loadTenantConfigs loads tenant configs in the module
func loadTenantConfigs(t *testing.T, tm *TenantAwareConfigTestModule, tenantService *StandardTenantService, app Application) {
	t.Helper()

	tenants := tenantService.GetTenants()
	for _, tenantID := range tenants {
		cp, err := tenantService.GetTenantConfig(tenantID, "TestConfig")
		if err != nil {
			t.Errorf("Failed to get tenant config for tenant %s: %v", tenantID, err)
		} else if cp == nil {
			t.Errorf("Expected non-nil config provider for tenant %s", tenantID)
		} else if cp.GetConfig() == nil {
			t.Errorf("Expected non-nil config for tenant %s", tenantID)
		} else {
			// Use Debug level for logging config contents
			app.Logger().Debug("Tenant config loaded", "tenantID", tenantID)
			tm.tenantConfigs[tenantID] = cp.GetConfig().(*TestTenantConfig)
		}
	}
}

func TestTenantAwareConfigModule(t *testing.T) {
	// Setup
	tempDir := setupTempConfigDir(t)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Fatalf("Failed to remove temp directory: %v", err)
		}
	}()

	app, tm, tenantService := setupTestApp(t, tempDir)

	// Initialize tenant configurations
	if err := app.(*StdApplication).initTenantConfigurations(); err != nil {
		t.Fatalf("Failed to initialize tenant configurations: %v", err)
	}

	// Verify tenants were registered
	if tm.tenantRegisteredCalls != 2 {
		t.Errorf("Expected 2 tenant registered calls, got %d", tm.tenantRegisteredCalls)
	}

	// Load tenant configs in the module
	loadTenantConfigs(t, tm, tenantService, app)

	// Verify tenant configs were loaded correctly
	verifyTenantConfig(t, tm, TenantID("tenant1"), "Tenant1", "test", "feature1", true)
	verifyTenantConfig(t, tm, TenantID("tenant2"), "Tenant2", "production", "feature1", false)

	// Test tenant removal
	if err := tenantService.RemoveTenant(TenantID("tenant1")); err != nil {
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

// TestTenantAwareConfig tests the TenantAwareConfig functionality
func TestTenantAwareConfig(t *testing.T) {
	// Create test logger
	log := &logger{t}

	// Create default config
	defaultCfg := &TestTenantConfig{
		Name:        "Default",
		Environment: "default",
		Features:    map[string]bool{"default": true},
	}
	defaultProvider := NewStdConfigProvider(defaultCfg)

	// Create tenant service
	tenantService := NewStandardTenantService(log)

	// Register tenants with configs
	tenant1ID := TenantID("tenant1")
	tenant1Cfg := &TestTenantConfig{
		Name:        "Tenant1",
		Environment: "test",
		Features:    map[string]bool{"feature1": true},
	}
	tenant1Configs := map[string]ConfigProvider{
		"TestConfig": NewStdConfigProvider(tenant1Cfg),
	}
	err := tenantService.RegisterTenant(tenant1ID, tenant1Configs)
	if err != nil {
		t.Fatalf("Failed to register tenant1: %v", err)
	}

	// Register another tenant
	tenant2ID := TenantID("tenant2")
	tenant2Cfg := &TestTenantConfig{
		Name:        "Tenant2",
		Environment: "production",
		Features:    map[string]bool{"feature2": true},
	}
	tenant2Configs := map[string]ConfigProvider{
		"TestConfig": NewStdConfigProvider(tenant2Cfg),
	}
	err = tenantService.RegisterTenant(tenant2ID, tenant2Configs)
	if err != nil {
		t.Fatalf("Failed to register tenant2: %v", err)
	}

	// Create TenantAwareConfig
	tacConfig := NewTenantAwareConfig(defaultProvider, tenantService, "TestConfig")

	// Test 1: GetConfig should return default config
	config := tacConfig.GetConfig()
	testCfg, ok := config.(*TestTenantConfig)
	if !ok {
		t.Fatalf("Expected *TestTenantConfig, got %T", config)
	}
	if testCfg.Name != "Default" {
		t.Errorf("Expected Name 'Default', got '%s'", testCfg.Name)
	}

	// Test 2: GetConfigWithContext with no tenant ID should return default config
	ctx := context.Background()
	config = tacConfig.GetConfigWithContext(ctx)
	testCfg, ok = config.(*TestTenantConfig)
	if !ok {
		t.Fatalf("Expected *TestTenantConfig, got %T", config)
	}
	if testCfg.Name != "Default" {
		t.Errorf("Expected Name 'Default', got '%s'", testCfg.Name)
	}

	// Test 3: GetConfigWithContext with tenant1 ID should return tenant1 config
	ctx1 := NewTenantContext(ctx, tenant1ID)
	config = tacConfig.GetConfigWithContext(ctx1)
	testCfg, ok = config.(*TestTenantConfig)
	if !ok {
		t.Fatalf("Expected *TestTenantConfig, got %T", config)
	}
	if testCfg.Name != "Tenant1" {
		t.Errorf("Expected Name 'Tenant1', got '%s'", testCfg.Name)
	}
	if testCfg.Environment != "test" {
		t.Errorf("Expected Environment 'test', got '%s'", testCfg.Environment)
	}
	if !testCfg.Features["feature1"] {
		t.Errorf("Expected Features['feature1'] to be true")
	}

	// Test 4: GetConfigWithContext with tenant2 ID should return tenant2 config
	ctx2 := NewTenantContext(ctx, tenant2ID)
	config = tacConfig.GetConfigWithContext(ctx2)
	testCfg, ok = config.(*TestTenantConfig)
	if !ok {
		t.Fatalf("Expected *TestTenantConfig, got %T", config)
	}
	if testCfg.Name != "Tenant2" {
		t.Errorf("Expected Name 'Tenant2', got '%s'", testCfg.Name)
	}
	if testCfg.Environment != "production" {
		t.Errorf("Expected Environment 'production', got '%s'", testCfg.Environment)
	}
	if !testCfg.Features["feature2"] {
		t.Errorf("Expected Features['feature2'] to be true")
	}

	// Test 5: GetConfigWithContext with non-existent tenant ID should return default config
	nonExistentCtx := NewTenantContext(ctx, TenantID("non-existent"))
	config = tacConfig.GetConfigWithContext(nonExistentCtx)
	testCfg, ok = config.(*TestTenantConfig)
	if !ok {
		t.Fatalf("Expected *TestTenantConfig, got %T", config)
	}
	if testCfg.Name != "Default" {
		t.Errorf("Expected Name 'Default', got '%s'", testCfg.Name)
	}

	// Test 6: nil default config provider
	nilConfig := NewTenantAwareConfig(nil, tenantService, "TestConfig")
	config = nilConfig.GetConfig()
	if config != nil {
		t.Errorf("Expected nil config, got %+v", config)
	}

	// Test 7: GetConfigWithContext with tenant ID but invalid section
	wrongSectionConfig := NewTenantAwareConfig(defaultProvider, tenantService, "NonExistentSection")
	config = wrongSectionConfig.GetConfigWithContext(ctx1)
	testCfg, ok = config.(*TestTenantConfig)
	if !ok {
		t.Fatalf("Expected *TestTenantConfig, got %T", config)
	}
	if testCfg.Name != "Default" {
		t.Errorf("Expected to fall back to default config, got Name '%s'", testCfg.Name)
	}
}

// TestTenantAwareConfigNilCases tests edge cases with nil values
func TestTenantAwareConfigNilCases(t *testing.T) {
	// Create default config and tenant service
	defaultCfg := &TestTenantConfig{Name: "Default"}
	defaultProvider := NewStdConfigProvider(defaultCfg)

	// Test with nil tenant service but valid default config
	tacNilTenantService := NewTenantAwareConfig(defaultProvider, nil, "TestConfig")

	// GetConfig should still work with nil tenant service
	config := tacNilTenantService.GetConfig()
	testCfg, ok := config.(*TestTenantConfig)
	if !ok {
		t.Fatalf("Expected *TestTenantConfig, got %T", config)
	}
	if testCfg.Name != "Default" {
		t.Errorf("Expected Name 'Default', got '%s'", testCfg.Name)
	}

	// GetConfigWithContext should fall back to default when tenant service is nil
	ctx := NewTenantContext(context.Background(), TenantID("tenant1"))
	config = tacNilTenantService.GetConfigWithContext(ctx)
	testCfg, ok = config.(*TestTenantConfig)
	if !ok {
		t.Fatalf("Expected *TestTenantConfig, got %T", config)
	}
	if testCfg.Name != "Default" {
		t.Errorf("Expected Name 'Default', got '%s'", testCfg.Name)
	}

	// Test fully nil case
	tacAllNil := NewTenantAwareConfig(nil, nil, "")
	if config = tacAllNil.GetConfig(); config != nil {
		t.Errorf("Expected nil config, got %+v", config)
	}
	if config = tacAllNil.GetConfigWithContext(ctx); config != nil {
		t.Errorf("Expected nil config, got %+v", config)
	}
}
