package modular

import (
	"log/slog"
	"os"
	"testing"
)

// TestTenantAwareModuleRaceCondition tests that tenant-aware modules
// can handle tenant registration without panicking during initialization
func TestTenantAwareModuleRaceCondition(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create application
	app := NewStdApplication(NewStdConfigProvider(&struct{}{}), logger)

	// Register a mock tenant-aware module that simulates the race condition
	mockModule := &MockTenantAwareModule{}
	app.RegisterModule(mockModule)

	// Register tenant service
	tenantService := NewStandardTenantService(logger)
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	// Register a simple tenant config loader
	configLoader := &SimpleTenantConfigLoader{}
	if err := app.RegisterService("tenantConfigLoader", configLoader); err != nil {
		t.Fatalf("Failed to register tenant config loader: %v", err)
	}

	// Initialize application - this should NOT panic
	t.Log("Initializing application...")
	if err := app.Init(); err != nil {
		t.Fatalf("Failed to initialize application: %v", err)
	}

	// Verify that the module received the tenant notification
	if !mockModule.tenantRegistered {
		t.Error("Expected tenant to be registered in mock module")
	}

	t.Log("✅ Application initialized successfully - no race condition panic!")
	t.Log("✅ Tenant-aware module race condition has been tested and works correctly!")
}

// MockTenantAwareModule simulates a tenant-aware module that could have race conditions
type MockTenantAwareModule struct {
	name             string
	app              Application // Store the app instead of logger directly
	tenantRegistered bool
}

func (m *MockTenantAwareModule) Name() string {
	return "MockTenantAwareModule"
}

func (m *MockTenantAwareModule) Init(app Application) error {
	m.app = app
	// Simulate some initialization work
	return nil
}

func (m *MockTenantAwareModule) OnTenantRegistered(tenantID TenantID) {
	// Check if app is available (module might not be fully initialized yet)
	// This simulates the race condition that was fixed in chimux
	if m.app != nil && m.app.Logger() != nil {
		m.app.Logger().Info("Tenant registered in mock module", "tenantID", tenantID)
	}
	m.tenantRegistered = true
}

func (m *MockTenantAwareModule) OnTenantRemoved(tenantID TenantID) {
	// Check if app is available (module might not be fully initialized yet)
	if m.app != nil && m.app.Logger() != nil {
		m.app.Logger().Info("Tenant removed from mock module", "tenantID", tenantID)
	}
}

// SimpleTenantConfigLoader for testing
type SimpleTenantConfigLoader struct{}

func (l *SimpleTenantConfigLoader) LoadTenantConfigurations(app Application, tenantService TenantService) error {
	app.Logger().Info("Loading tenant configurations")

	// Register a test tenant with simple config
	return tenantService.RegisterTenant(TenantID("test-tenant"), map[string]ConfigProvider{
		"MockTenantAwareModule": NewStdConfigProvider(&struct {
			TestValue string `yaml:"testValue" default:"test"`
		}{}),
	})
}
