package chimux_test

import (
	"testing"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/modules/chimux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTenantAwareModule simulates a module that changes initialization order
type MockTenantAwareModule struct {
	name        string
	initialized bool
}

func NewMockTenantAwareModule(name string) *MockTenantAwareModule {
	return &MockTenantAwareModule{name: name}
}

func (m *MockTenantAwareModule) Name() string {
	return m.name
}

func (m *MockTenantAwareModule) RegisterConfig(app modular.Application) error {
	return nil
}

func (m *MockTenantAwareModule) Init(app modular.Application) error {
	m.initialized = true
	return nil
}

func (m *MockTenantAwareModule) OnTenantRegistered(tenantID modular.TenantID) {
	// This simulates other tenant-aware modules that can trigger race conditions
}

func (m *MockTenantAwareModule) OnTenantRemoved(tenantID modular.TenantID) {
	// No-op for this test
}

// TestChimuxTenantRaceConditionFixed demonstrates that the race condition is resolved
func TestChimuxTenantRaceConditionFixed(t *testing.T) {
	t.Run("Chimux handles OnTenantRegistered gracefully when called before Init", func(t *testing.T) {
		// Create chimux module but DO NOT call Init
		module := chimux.NewChiMuxModule().(*chimux.ChiMuxModule)

		// This should NOT panic anymore due to the defensive nil check
		// In the real scenario, this happens during application Init when
		// tenant service registration triggers immediate tenant callbacks
		assert.NotPanics(t, func() {
			module.OnTenantRegistered(modular.TenantID("test-tenant"))
			// With the fix, this handles nil logger gracefully
		}, "Should not panic when OnTenantRegistered is called before Init due to defensive nil check")
	})
}

// TestChimuxTenantRaceConditionWithComplexDependencies simulates the real scenario
func TestChimuxTenantRaceConditionWithComplexDependencies(t *testing.T) {
	t.Run("Simulate complex module dependency graph causing race condition", func(t *testing.T) {
		// This test simulates what happens when modules like reverseproxy + launchdarkly
		// or eventlogger/eventbus change the initialization order

		logger := &chimux.MockLogger{}

		// Create a simplified application that shows the race condition
		app := modular.NewObservableApplication(modular.NewStdConfigProvider(&struct{}{}), logger)

		// Register modules in an order that will trigger the race condition
		chimuxModule := chimux.NewChiMuxModule()
		app.RegisterModule(chimuxModule)

		// Register mock tenant-aware modules that could affect initialization order
		mockModule1 := NewMockTenantAwareModule("reverseproxy-mock")
		mockModule2 := NewMockTenantAwareModule("launchdarkly-mock")
		app.RegisterModule(mockModule1)
		app.RegisterModule(mockModule2)

		// Create and register tenant service and config loader
		// This is what triggers the race condition in real scenarios
		tenantService := modular.NewStandardTenantService(logger)
		app.RegisterService("tenantService", tenantService)

		// Register a mock tenant config loader
		tenantConfigLoader := &MockTenantConfigLoader{}
		app.RegisterService("tenantConfigLoader", tenantConfigLoader)

		// Register a tenant before initialization to simulate the race condition
		tenantService.RegisterTenant("test-tenant", nil)

		// This Init call should NOT trigger the race condition anymore
		// After our fix, it should work properly
		err := app.Init()
		require.NoError(t, err, "Application initialization should not panic due to race condition")
	})
}

// MockTenantConfigLoader for testing
type MockTenantConfigLoader struct{}

func (m *MockTenantConfigLoader) LoadTenantConfigurations(app modular.TenantApplication, tenantService modular.TenantService) error {
	// Simple mock - just return success
	return nil
}

func TestChimuxInitializationLifecycle(t *testing.T) {
	t.Run("Verify chimux initialization state", func(t *testing.T) {
		module := chimux.NewChiMuxModule().(*chimux.ChiMuxModule)
		mockApp := chimux.NewMockApplication()

		// Before Init - router should be nil
		assert.Nil(t, module.ChiRouter(), "Router should be nil before Init")

		// Register config
		err := module.RegisterConfig(mockApp)
		require.NoError(t, err)

		// Before Init - router should still be nil
		assert.Nil(t, module.ChiRouter(), "Router should still be nil after RegisterConfig")

		// Register observers before Init
		err = module.RegisterObservers(mockApp)
		require.NoError(t, err)

		// Init should create the router
		err = module.Init(mockApp)
		require.NoError(t, err)

		// After Init - router should be available
		assert.NotNil(t, module.ChiRouter(), "Router should be available after Init")

		// Now tenant registration should be safe
		require.NotPanics(t, func() {
			module.OnTenantRegistered(modular.TenantID("test-tenant"))
		}, "OnTenantRegistered should not panic after proper initialization")
	})
}
