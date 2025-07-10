package goldenmodule

import (
	"context"
	"fmt"
	"github.com/CrisisTextLine/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewGoldenModuleModule(t *testing.T) {
	module := NewGoldenModuleModule()
	assert.NotNil(t, module)
	// Test module properties
	modImpl, ok := module.(*GoldenModuleModule)
	require.True(t, ok) // Use require here as the rest of the test depends on this
	assert.Equal(t, "goldenmodule", modImpl.Name())
	assert.NotNil(t, modImpl.tenantConfigs)
}

func TestModule_RegisterConfig(t *testing.T) {
	module := NewGoldenModuleModule().(*GoldenModuleModule)
	// Create a mock application
	mockApp := NewMockApplication()
	// Test RegisterConfig
	err := module.RegisterConfig(mockApp)
	assert.NoError(t, err)
	assert.NotNil(t, module.config) // Verify config struct was initialized
	// Verify the config section was registered in the mock app
	_, err = mockApp.GetConfigSection(module.Name())
	assert.NoError(t, err, "Config section should be registered")
}

func TestModule_Init(t *testing.T) {
	module := NewGoldenModuleModule().(*GoldenModuleModule)
	// Create a mock application
	mockApp := NewMockApplication()

	// Register mock services if needed for Init
	// mockService := &MockMyService{}
	// mockApp.RegisterService("requiredService", mockService)

	// Test Init
	err := module.Init(mockApp)
	assert.NoError(t, err)
	// Add assertions here to check the state of the module after Init
}

func TestModule_Start(t *testing.T) {
	module := NewGoldenModuleModule().(*GoldenModuleModule)
	// Add setup if needed, e.g., call Init
	// mockApp := NewMockApplication()
	// module.Init(mockApp)

	// Test Start
	err := module.Start(context.Background())
	assert.NoError(t, err)
	// Add assertions here to check the state of the module after Start
}

func TestModule_Stop(t *testing.T) {
	module := NewGoldenModuleModule().(*GoldenModuleModule)
	// Add setup if needed, e.g., call Init and Start
	// mockApp := NewMockApplication()
	// module.Init(mockApp)
	// module.Start(context.Background())

	// Test Stop
	err := module.Stop(context.Background())
	assert.NoError(t, err)
	// Add assertions here to check the state of the module after Stop
}

func TestModule_TenantLifecycle(t *testing.T) {
	module := NewGoldenModuleModule().(*GoldenModuleModule)

	// Initialize base config if needed for tenant fallback
	module.config = &Config{}

	tenantID := modular.TenantID("test-tenant")

	// Test tenant registration
	module.OnTenantRegistered(tenantID)
	// Add assertions: check if tenant-specific resources were created

	// Test loading tenant config (requires a mock TenantService)
	mockTenantService := &MockTenantService{
		Configs: map[modular.TenantID]map[string]modular.ConfigProvider{
			tenantID: {
				module.Name(): modular.NewStdConfigProvider(&Config{ /* Populate with test data */ }),
			},
		},
	}
	err := module.LoadTenantConfig(mockTenantService, tenantID)
	assert.NoError(t, err)
	loadedConfig := module.GetTenantConfig(tenantID)
	require.NotNil(t, loadedConfig, "Loaded tenant config should not be nil")
	// Add assertions to check the loaded config values

	// Test tenant removal
	module.OnTenantRemoved(tenantID)
	_, exists := module.tenantConfigs[tenantID]
	assert.False(t, exists, "Tenant config should be removed")
	// Add assertions: check if tenant-specific resources were cleaned up
}

// MockTenantService for testing LoadTenantConfig
type MockTenantService struct {
	Configs map[modular.TenantID]map[string]modular.ConfigProvider
}

func (m *MockTenantService) GetTenantConfig(tid modular.TenantID, section string) (modular.ConfigProvider, error) {
	if tenantSections, ok := m.Configs[tid]; ok {
		if provider, ok := tenantSections[section]; ok {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("mock config not found for tenant %s, section %s", tid, section)
}
func (m *MockTenantService) GetTenants() []modular.TenantID { return nil } // Not needed for this test
func (m *MockTenantService) RegisterTenant(modular.TenantID, map[string]modular.ConfigProvider) error {
	return nil
}                                                                                      // Not needed
func (m *MockTenantService) RemoveTenant(modular.TenantID) error                       { return nil } // Not needed
func (m *MockTenantService) RegisterTenantAwareModule(modular.TenantAwareModule) error { return nil } // Not needed

// Add more tests for specific module functionality
