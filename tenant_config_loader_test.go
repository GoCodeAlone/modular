package modular

import (
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// MockTenantService implements TenantService for testing
type MockTenantService struct {
	tenants       map[TenantID]map[string]ConfigProvider
	registerCalls int
}

func NewMockTenantService() *MockTenantService {
	return &MockTenantService{
		tenants: make(map[TenantID]map[string]ConfigProvider),
	}
}

func (m *MockTenantService) GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error) {
	if configs, ok := m.tenants[tenantID]; ok {
		if provider, ok := configs[section]; ok {
			return provider, nil
		}
	}
	return nil, nil
}

func (m *MockTenantService) GetTenants() []TenantID {
	tenants := make([]TenantID, 0, len(m.tenants))
	for id := range m.tenants {
		tenants = append(tenants, id)
	}
	return tenants
}

func (m *MockTenantService) RegisterTenant(tenantID TenantID, configs map[string]ConfigProvider) error {
	m.registerCalls++
	if _, exists := m.tenants[tenantID]; !exists {
		m.tenants[tenantID] = make(map[string]ConfigProvider)
	}

	for section, provider := range configs {
		m.tenants[tenantID][section] = provider
	}
	return nil
}

func (m *MockTenantService) RegisterTenantAwareModule(_ TenantAwareModule) error {
	// No-op for mock
	return nil
}

// setupTestDirectory creates a temporary directory with test tenant config files
func setupTestDirectory(t *testing.T) (tempDir string, cleanup func()) {
	tempDir, err := os.MkdirTemp("", "tenant_config_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create some test config files
	createTestFile(t, tempDir, "tenant1.json", `{"TestSection": {"key": "value1"}}`)
	createTestFile(t, tempDir, "tenant2.yaml", `TestSection:
  key: value2`)
	createTestFile(t, tempDir, "tenant3.yml", `TestSection:
  key: value3`)
	createTestFile(t, tempDir, "tenant4.toml", `[TestSection]
key = "value4"`)
	// Invalid file should be skipped
	createTestFile(t, tempDir, "invalid.txt", `Not a valid config`)

	cleanup = func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			t.Fatalf("Failed to remove temp directory: %v", err)
			return
		}
	}

	return tempDir, cleanup
}

func createTestFile(t *testing.T, dir, name, content string) {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create test file %s: %v", name, err)
	}
}

// Test the constructor functions
func TestNewFileBasedTenantConfigLoader(t *testing.T) {
	params := TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^tenant\d+\.json$`),
		ConfigDir:       "/test/dir",
	}

	loader := NewFileBasedTenantConfigLoader(params)

	if loader == nil {
		t.Fatal("Expected non-nil loader")
	}

	if loader.configParams.ConfigDir != params.ConfigDir {
		t.Errorf("Expected ConfigDir %s, got %s", params.ConfigDir, loader.configParams.ConfigDir)
	}

	if loader.configParams.ConfigNameRegex.String() != params.ConfigNameRegex.String() {
		t.Errorf("Expected ConfigNameRegex %s, got %s",
			params.ConfigNameRegex.String(), loader.configParams.ConfigNameRegex.String())
	}
}

func TestDefaultTenantConfigLoader(t *testing.T) {
	configDir := "/default/test/dir"
	loader := DefaultTenantConfigLoader(configDir)

	if loader == nil {
		t.Fatal("Expected non-nil loader")
	}

	if loader.configParams.ConfigDir != configDir {
		t.Errorf("Expected ConfigDir %s, got %s", configDir, loader.configParams.ConfigDir)
	}

	expectedRegex := `^\w+\.(json|yaml|yml|toml)$`
	if loader.configParams.ConfigNameRegex.String() != expectedRegex {
		t.Errorf("Expected ConfigNameRegex %s, got %s",
			expectedRegex, loader.configParams.ConfigNameRegex.String())
	}
}

// TestLoadTenantConfigurations tests the full loading process
func TestLoadTenantConfigurations(t *testing.T) {
	// Create a temporary directory with test files
	tempDir, cleanup := setupTestDirectory(t)
	defer cleanup()

	app := NewStdApplication(nil, &logger{t})

	// Register test config sections
	testSection := struct {
		Key string
	}{}
	app.RegisterConfigSection("TestSection", NewStdConfigProvider(&testSection))

	tenantService := NewMockTenantService()

	// Create the loader
	params := TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^tenant\d+\.(json|yaml|yml|toml)$`),
		ConfigDir:       tempDir,
	}
	loader := NewFileBasedTenantConfigLoader(params)

	// Test loading
	err := loader.LoadTenantConfigurations(app, tenantService)

	// Assertions
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check that tenants were registered
	tenants := tenantService.GetTenants()
	expectedCount := 4 // We created 4 valid tenant files
	if len(tenants) != expectedCount {
		t.Errorf("Expected %d tenants, got %d", expectedCount, len(tenants))
	}

	// Check that tenant IDs were parsed correctly
	expectedTenants := map[TenantID]bool{
		"tenant1": true,
		"tenant2": true,
		"tenant3": true,
		"tenant4": true,
	}

	for _, id := range tenants {
		if !expectedTenants[id] {
			t.Errorf("Unexpected tenant ID: %s", id)
		}
	}

	// Verify the mock tenant service received the registration calls
	if tenantService.registerCalls != expectedCount {
		t.Errorf("Expected %d RegisterTenant calls, got %d", expectedCount, tenantService.registerCalls)
	}
}

// Test with no config files present
func TestLoadTenantConfigurationsEmptyDirectory(t *testing.T) {
	// Create an empty temporary directory
	tempDir, err := os.MkdirTemp("", "empty_tenant_config_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func(path string) {
		if removeErr := os.RemoveAll(path); removeErr != nil {
			t.Fatalf("Failed to remove temp directory: %v", removeErr)
		}
	}(tempDir)

	app := NewStdApplication(nil, &logger{t})
	tenantService := NewMockTenantService()

	params := TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^tenant\d+\.json$`),
		ConfigDir:       tempDir,
	}
	loader := NewFileBasedTenantConfigLoader(params)

	err = loader.LoadTenantConfigurations(app, tenantService)

	// Should error with empty directory
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Should be no tenants registered
	if len(tenantService.GetTenants()) > 0 {
		t.Error("Expected no tenants to be registered")
	}
}

// Test with non-existent directory
func TestLoadTenantConfigurationsNonExistentDirectory(t *testing.T) {
	app := NewStdApplication(nil, slog.Default())
	tenantService := NewMockTenantService()

	params := TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^tenant\d+\.json$`),
		ConfigDir:       "/this/directory/should/not/exist",
	}
	loader := NewFileBasedTenantConfigLoader(params)

	err := loader.LoadTenantConfigurations(app, tenantService)

	// Should error with non-existent directory
	if err == nil {
		t.Error("Expected error with non-existent directory")
	}
}
