package goldenmodule

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGoldenModuleModule(t *testing.T) {
	module := NewGoldenModuleModule()
	assert.NotNil(t, module)

	// Test module properties
	modImpl, ok := module.(*GoldenModuleModule)
	require.True(t, ok)
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
	assert.NotNil(t, module.config)
}

func TestModule_Init(t *testing.T) {
	module := NewGoldenModuleModule().(*GoldenModuleModule)

	// Create a mock application
	mockApp := NewMockApplication()

	// Test Init
	err := module.Init(mockApp)
	assert.NoError(t, err)
}
func TestModule_Start(t *testing.T) {
	module := NewGoldenModuleModule().(*GoldenModuleModule)

	// Test Start
	err := module.Start(context.Background())
	assert.NoError(t, err)
}
func TestModule_Stop(t *testing.T) {
	module := NewGoldenModuleModule().(*GoldenModuleModule)

	// Test Stop
	err := module.Stop(context.Background())
	assert.NoError(t, err)
}
func TestModule_TenantLifecycle(t *testing.T) {
	module := NewGoldenModuleModule().(*GoldenModuleModule)

	// Test tenant registration
	tenantID := modular.TenantID("test-tenant")
	module.OnTenantRegistered(tenantID)

	// Test tenant removal
	module.OnTenantRemoved(tenantID)
	_, exists := module.tenantConfigs[tenantID]
	assert.False(t, exists)
}
