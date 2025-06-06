package modular

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Test configuration structs
// YAML expects lowercase names as defaults for keys when unmarshalling,
// so we need tags to explicitly support uppercase
type TestTenantConfig struct {
	Name        string          `yaml:"Name"`
	Environment string          `yaml:"Environment"`
	Features    map[string]bool `yaml:"Features"`
}

type AnotherTestConfig struct {
	APIKey         string `yaml:"ApiKey"`
	MaxConnections int    `yaml:"MaxConnections"`
	Timeout        int    `yaml:"Timeout"`
}

// setupTestConfigFiles creates temporary tenant configuration files for testing
func setupTestConfigFiles(t *testing.T) string {
	// Create a temporary directory for test config files
	tempDir, err := os.MkdirTemp("", "tenant-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create test JSON config file
	tenant1Config := `{
		"TestConfig": {
			"Name": "Tenant1",
			"Environment": "test",
			"Features": {
				"feature1": true,
				"feature2": false
			}
		},
		"ApiConfig": {
			"ApiKey": "tenant1-api-key",
			"MaxConnections": 10,
			"Timeout": 30
		}
	}`

	// Create test YAML config file
	tenant2Config := `
TestConfig:
  Name: Tenant2
  Environment: production
  Features:
    feature1: false
    feature2: true
ApiConfig:
  ApiKey: tenant2-api-key
  MaxConnections: 20
  Timeout: 60
`

	err = os.WriteFile(filepath.Join(tempDir, "tenant1.json"), []byte(tenant1Config), 0600)
	if err != nil {
		t.Fatalf("Failed to write tenant1.json: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "tenant2.yaml"), []byte(tenant2Config), 0600)
	if err != nil {
		t.Fatalf("Failed to write tenant2.yaml: %v", err)
	}

	return tempDir
}

// setupTenantServices creates application and tenant services for testing
func setupTenantServices(t *testing.T) (Application, *StandardTenantService) {
	log := &logger{t}
	app := NewStdApplication(NewStdConfigProvider(nil), log)

	// Register the config sections for the application
	app.RegisterConfigSection("TestConfig", NewStdConfigProvider(&TestTenantConfig{}))
	app.RegisterConfigSection("ApiConfig", NewStdConfigProvider(&AnotherTestConfig{}))

	tenantService := NewStandardTenantService(log)
	return app, tenantService
}

// TestFileBasedTenantConfigLoader tests loading tenant configurations from files
func TestFileBasedTenantConfigLoader(t *testing.T) {
	tempDir := setupTestConfigFiles(t)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp directory: %v", err)
		}
	}()

	app, tenantService := setupTenantServices(t)

	// Create a file-based tenant config loader
	loader := NewFileBasedTenantConfigLoader(TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^tenant\d+\.(json|yaml)$`),
		ConfigDir:       tempDir,
	})

	// Test loading tenant configurations
	err := loader.LoadTenantConfigurations(app, tenantService)
	if err != nil {
		t.Fatalf("Failed to load tenant configurations: %v", err)
	}

	// Verify that both tenants were loaded
	tenants := tenantService.GetTenants()
	if len(tenants) != 2 {
		t.Errorf("Expected 2 tenants, got %d", len(tenants))
	}

	// Verify each tenant's configurations
	verifyTenant1Config(t, tenantService)
	verifyTenant2Config(t, tenantService)
}

// verifyTenant1Config verifies the configuration for tenant1
func verifyTenant1Config(t *testing.T, tenantService *StandardTenantService) {
	tenant1ID := TenantID("tenant1")

	// Check TestConfig
	testConfigProvider, err := tenantService.GetTenantConfig(tenant1ID, "TestConfig")
	if err != nil {
		t.Errorf("Failed to get TestConfig for tenant1: %v", err)
		return
	}

	testConfig, ok := testConfigProvider.GetConfig().(*TestTenantConfig)
	if !ok {
		t.Errorf("Expected *TestTenantConfig, got %T", testConfigProvider.GetConfig())
		return
	}

	if testConfig.Name != "Tenant1" {
		t.Errorf("Expected Name 'Tenant1', got '%s'", testConfig.Name)
	}
	if testConfig.Environment != "test" {
		t.Errorf("Expected Environment 'test', got '%s'", testConfig.Environment)
	}
	if !testConfig.Features["feature1"] {
		t.Errorf("Expected Features['feature1'] to be true")
	}

	// Check ApiConfig
	apiConfigProvider, err := tenantService.GetTenantConfig(tenant1ID, "ApiConfig")
	if err != nil {
		t.Errorf("Failed to get ApiConfig for tenant1: %v", err)
		return
	}

	apiConfig, ok := apiConfigProvider.GetConfig().(*AnotherTestConfig)
	if !ok {
		t.Errorf("Expected *AnotherTestConfig, got %T", apiConfigProvider.GetConfig())
		return
	}

	if apiConfig.APIKey != "tenant1-api-key" {
		t.Errorf("Expected ApiKey 'tenant1-api-key', got '%s'", apiConfig.APIKey)
	}
	if apiConfig.MaxConnections != 10 {
		t.Errorf("Expected MaxConnections 10, got %d", apiConfig.MaxConnections)
	}
}

// verifyTenant2Config verifies the configuration for tenant2
func verifyTenant2Config(t *testing.T, tenantService *StandardTenantService) {
	tenant2ID := TenantID("tenant2")

	testConfigProvider, err := tenantService.GetTenantConfig(tenant2ID, "TestConfig")
	if err != nil {
		t.Errorf("Failed to get TestConfig for tenant2: %v", err)
		return
	}

	testConfig, ok := testConfigProvider.GetConfig().(*TestTenantConfig)
	if !ok {
		t.Errorf("Expected *TestTenantConfig, got %T", testConfigProvider.GetConfig())
		return
	}

	t.Logf("Got TestConfig for tenant2 of type %T", testConfig)
	if testConfig.Name != "Tenant2" {
		t.Errorf("Expected Name 'Tenant2', got '%s'", testConfig.Name)
	}
	if testConfig.Environment != "production" {
		t.Errorf("Expected Environment 'production', got '%s'", testConfig.Environment)
	}
	if testConfig.Features["feature1"] {
		t.Errorf("Expected Features['feature1'] to be false")
	}
}

func TestLoadTenantConfigsEmptyDirectory(t *testing.T) {
	// Create a temporary directory for test config files
	tempDir, err := os.MkdirTemp("", "tenant-empty-dir-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("Failed to remove temp directory: %v", removeErr)
		}
	}()

	// Create an application and tenant service
	log := &logger{t}
	app := NewStdApplication(NewStdConfigProvider(nil), log)
	tenantService := NewStandardTenantService(log)

	// Test loading from empty directory
	params := TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^tenant\d+\.json$`),
		ConfigDir:       tempDir,
	}

	err = LoadTenantConfigs(app, tenantService, params)
	if err != nil {
		t.Fatalf("LoadTenantConfigs failed with empty directory: %v", err)
	}

	// Verify no tenants were loaded
	tenants := tenantService.GetTenants()
	if len(tenants) != 0 {
		t.Errorf("Expected 0 tenants from empty directory, got %d", len(tenants))
	}
}

func TestLoadTenantConfigsNonexistentDirectory(t *testing.T) {
	// Create a path that definitely doesn't exist
	nonExistentDir := "/path/to/nonexistent/directory"

	// Create an application and tenant service
	log := &MockLogger{}
	app := NewStdApplication(NewStdConfigProvider(nil), log)
	tenantService := NewStandardTenantService(log)

	// Test loading from nonexistent directory
	params := TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^tenant\d+\.json$`),
		ConfigDir:       nonExistentDir,
	}

	log.On("Error", "Tenant config directory does not exist", []interface{}{"directory", nonExistentDir}).Return(nil)
	err := LoadTenantConfigs(app, tenantService, params)
	if err == nil || !strings.Contains(err.Error(), "tenant config directory does not exist") {
		t.Errorf("Expected error for nonexistent directory, got: %v", err)
	}

	// Verify no tenants were loaded
	tenants := tenantService.GetTenants()
	if len(tenants) != 0 {
		t.Errorf("Expected 0 tenants from nonexistent directory, got %d", len(tenants))
	}
	log.AssertExpectations(t)
}

func TestTenantConfigProviderSetAndGet(t *testing.T) {
	// Create a TenantConfigProvider
	tcp := NewTenantConfigProvider(nil)

	// Create test configs
	tenant1ID := TenantID("tenant1")
	testConfig1 := &TestTenantConfig{
		Name:        "Tenant 1",
		Environment: "dev",
		Features:    map[string]bool{"feature1": true},
	}

	// Set config for tenant1, section "TestConfig"
	tcp.SetTenantConfig(tenant1ID, "TestConfig", NewStdConfigProvider(testConfig1))

	// Get config for tenant1, section "TestConfig"
	provider, err := tcp.GetTenantConfig(tenant1ID, "TestConfig")
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	config, ok := provider.GetConfig().(*TestTenantConfig)
	if !ok {
		t.Fatalf("Expected *TestTenantConfig, got %T", provider.GetConfig())
	}

	if config.Name != "Tenant 1" {
		t.Errorf("Expected Name 'Tenant 1', got '%s'", config.Name)
	}

	// Test HasTenantConfig
	if !tcp.HasTenantConfig(tenant1ID, "TestConfig") {
		t.Errorf("Expected HasTenantConfig to return true for existing config")
	}

	if tcp.HasTenantConfig(tenant1ID, "NonExistentSection") {
		t.Errorf("Expected HasTenantConfig to return false for non-existent section")
	}

	if tcp.HasTenantConfig(TenantID("nonexistenttenant"), "TestConfig") {
		t.Errorf("Expected HasTenantConfig to return false for non-existent tenant")
	}

	// Test getting non-existent config
	_, err = tcp.GetTenantConfig(TenantID("nonexistenttenant"), "TestConfig")
	if err == nil {
		t.Errorf("Expected error when getting config for non-existent tenant")
	}

	_, err = tcp.GetTenantConfig(tenant1ID, "NonExistentSection")
	if err == nil {
		t.Errorf("Expected error when getting non-existent section")
	}

	// Test nil config provider
	tcp.SetTenantConfig(tenant1ID, "NilSection", nil)
	if tcp.HasTenantConfig(tenant1ID, "NilSection") {
		t.Errorf("Expected HasTenantConfig to return false for nil provider")
	}

	// Test nil config
	nilProviderStruct := &struct{ Config interface{} }{nil}
	nilProvider := NewStdConfigProvider(nilProviderStruct.Config)
	tcp.SetTenantConfig(tenant1ID, "NilConfigSection", nilProvider)
	if tcp.HasTenantConfig(tenant1ID, "NilConfigSection") {
		t.Errorf("Expected HasTenantConfig to return false for provider with nil config")
	}
}

func TestCloneConfigWithValues(t *testing.T) {
	// Test cloning a struct config
	original := &TestTenantConfig{
		Name:        "Original",
		Environment: "test",
		Features:    map[string]bool{"feature1": true},
	}

	loaded := &TestTenantConfig{
		Name:        "Loaded",
		Environment: "prod",
		Features:    map[string]bool{"feature2": true},
	}

	cloned, err := cloneConfigWithValues(original, loaded)
	if err != nil {
		t.Fatalf("Failed to clone config: %v", err)
	}

	clonedConfig, ok := cloned.(*TestTenantConfig)
	if !ok {
		t.Fatalf("Expected *TestTenantConfig, got %T", cloned)
	}

	if clonedConfig.Name != "Loaded" {
		t.Errorf("Expected Name 'Loaded', got '%s'", clonedConfig.Name)
	}

	if clonedConfig.Environment != "prod" {
		t.Errorf("Expected Environment 'prod', got '%s'", clonedConfig.Environment)
	}

	if !clonedConfig.Features["feature2"] {
		t.Errorf("Expected Features['feature2'] to be true")
	}

	// Test nil inputs
	_, err = cloneConfigWithValues(nil, loaded)
	if err == nil {
		t.Errorf("Expected error for nil original config")
	}

	_, err = cloneConfigWithValues(original, nil)
	if err == nil {
		t.Errorf("Expected error for nil loaded config")
	}
}

func TestCopyStructFields(t *testing.T) {
	// Test copying struct to struct
	srcStruct := TestTenantConfig{
		Name:        "Source",
		Environment: "dev",
		Features:    map[string]bool{"feature1": true},
	}

	dstStruct := TestTenantConfig{}

	err := copyStructFields(&dstStruct, &srcStruct)
	if err != nil {
		t.Fatalf("Failed to copy struct fields: %v", err)
	}

	if dstStruct.Name != "Source" {
		t.Errorf("Expected Name 'Source', got '%s'", dstStruct.Name)
	}

	if dstStruct.Environment != "dev" {
		t.Errorf("Expected Environment 'dev', got '%s'", dstStruct.Environment)
	}

	// Test copying map to struct
	srcMap := map[string]interface{}{
		"Name":        "MapSource",
		"Environment": "prod",
		"Features":    map[string]bool{"feature2": true},
	}

	dstStruct = TestTenantConfig{}

	err = copyStructFields(&dstStruct, &srcMap)
	if err != nil {
		t.Fatalf("Failed to copy map to struct: %v", err)
	}

	if dstStruct.Name != "MapSource" {
		t.Errorf("Expected Name 'MapSource', got '%s'", dstStruct.Name)
	}

	// Test error cases
	err = copyStructFields(dstStruct, &srcStruct) // Non-pointer destination
	if err == nil {
		t.Errorf("Expected error with non-pointer destination")
	}

	invalidSrc := 123 // Invalid source type
	err = copyStructFields(&dstStruct, invalidSrc)
	if err == nil {
		t.Errorf("Expected error with invalid source type")
	}
}
