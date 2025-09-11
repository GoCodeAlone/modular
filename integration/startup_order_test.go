package integration

import (
	"log/slog"
	"os"
	"strings"
	"testing"

	modular "github.com/GoCodeAlone/modular"
)

// TestStartupDependencyResolution tests T023: Integration startup dependency resolution
// This test verifies that modules are initialized in the correct dependency order
// and that dependency resolution works correctly during application startup.
func TestStartupDependencyResolution(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Track initialization order
	var initOrder []string

	// Create modules with clear dependency chain: A -> B -> C
	moduleA := &testOrderModule{name: "moduleA", deps: []string{}, initOrder: &initOrder}
	moduleB := &testOrderModule{name: "moduleB", deps: []string{"moduleA"}, initOrder: &initOrder}
	moduleC := &testOrderModule{name: "moduleC", deps: []string{"moduleB"}, initOrder: &initOrder}

	// Create application
	app := modular.NewStdApplication(modular.NewStdConfigProvider(&struct{}{}), logger)

	// Register modules in intentionally wrong order to test dependency resolution
	app.RegisterModule(moduleC) // Should init last
	app.RegisterModule(moduleA) // Should init first
	app.RegisterModule(moduleB) // Should init second

	// Initialize application - dependency resolver should order correctly
	err := app.Init()
	if err != nil {
		t.Fatalf("Application initialization failed: %v", err)
	}

	// Verify correct initialization order
	expectedOrder := []string{"moduleA", "moduleB", "moduleC"}
	if len(initOrder) != len(expectedOrder) {
		t.Fatalf("Expected %d modules initialized, got %d", len(expectedOrder), len(initOrder))
	}

	for i, expected := range expectedOrder {
		if initOrder[i] != expected {
			t.Errorf("Expected module %s at position %d, got %s", expected, i, initOrder[i])
		}
	}

	t.Logf("✅ Modules initialized in correct dependency order: %s", strings.Join(initOrder, " -> "))

	// Test service dependency resolution
	var serviceA *testOrderService
	err = app.GetService("serviceA", &serviceA)
	if err != nil {
		t.Errorf("Failed to resolve serviceA: %v", err)
	}

	var serviceB *testOrderService
	err = app.GetService("serviceB", &serviceB)
	if err != nil {
		t.Errorf("Failed to resolve serviceB: %v", err)
	}

	// Verify services are properly resolved
	if serviceA == nil || serviceB == nil {
		t.Error("Service resolution failed - nil services returned")
	}

	t.Log("✅ Service dependency resolution completed successfully")
}

// testOrderModule tracks initialization order for dependency testing
type testOrderModule struct {
	name      string
	deps      []string
	initOrder *[]string
}

func (m *testOrderModule) Name() string {
	return m.name
}

func (m *testOrderModule) Init(app modular.Application) error {
	// Record initialization order
	*m.initOrder = append(*m.initOrder, m.name)

	// Register a service for this module
	service := &testOrderService{moduleName: m.name}
	return app.RegisterService("service"+strings.TrimPrefix(m.name, "module"), service)
}

func (m *testOrderModule) Dependencies() []string {
	return m.deps
}

// testOrderService provides a simple service for dependency testing
type testOrderService struct {
	moduleName string
}

func (s *testOrderService) GetModuleName() string {
	return s.moduleName
}
