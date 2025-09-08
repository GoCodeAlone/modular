package integration

import (
	"context"
	"log/slog"
	"os"
	"testing"

	modular "github.com/GoCodeAlone/modular"
)

// TestGracefulShutdownOrdering tests T027: Integration graceful shutdown ordering
// This test verifies that modules are stopped in reverse dependency order during shutdown.
func TestGracefulShutdownOrdering(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	// Track shutdown events
	var shutdownEvents []string
	
	// Create modules with dependencies: A -> B -> C (A depends on nothing, B depends on A, C depends on B)
	moduleA := &testShutdownModule{name: "moduleA", deps: []string{}, events: &shutdownEvents}
	moduleB := &testShutdownModule{name: "moduleB", deps: []string{"moduleA"}, events: &shutdownEvents}
	moduleC := &testShutdownModule{name: "moduleC", deps: []string{"moduleB"}, events: &shutdownEvents}
	
	// Create application
	app := modular.NewStdApplication(modular.NewStdConfigProvider(&struct{}{}), logger)
	
	// Register modules
	app.RegisterModule(moduleA)
	app.RegisterModule(moduleB)
	app.RegisterModule(moduleC)
	
	// Initialize application - should succeed
	err := app.Init()
	if err != nil {
		t.Fatalf("Application initialization failed: %v", err)
	}
	
	// Start application
	err = app.Start()
	if err != nil {
		t.Fatalf("Application start failed: %v", err)
	}
	
	// Stop application - should shutdown in reverse order
	err = app.Stop()
	if err != nil {
		t.Fatalf("Application stop failed: %v", err)
	}
	
	// Verify shutdown happens in reverse order of initialization
	// Expected: Init order A->B->C, Start order A->B->C, Stop order C->B->A
	expectedShutdownOrder := []string{
		"moduleA.Init",
		"moduleB.Init",
		"moduleC.Init",
		"moduleA.Start",
		"moduleB.Start", 
		"moduleC.Start",
		"moduleC.Stop",  // Reverse order
		"moduleB.Stop",  // Reverse order
		"moduleA.Stop",  // Reverse order
	}
	
	if len(shutdownEvents) != len(expectedShutdownOrder) {
		t.Fatalf("Expected %d events, got %d: %v", len(expectedShutdownOrder), len(shutdownEvents), shutdownEvents)
	}
	
	for i, expected := range expectedShutdownOrder {
		if shutdownEvents[i] != expected {
			t.Errorf("Expected event %s at position %d, got %s", expected, i, shutdownEvents[i])
		}
	}
	
	t.Logf("✅ Graceful shutdown completed in reverse order: %v", shutdownEvents)
}

// testShutdownModule implements all necessary interfaces for dependency ordering and lifecycle testing
type testShutdownModule struct {
	name    string
	deps    []string
	events  *[]string
	started bool
}

func (m *testShutdownModule) Name() string {
	return m.name
}

func (m *testShutdownModule) Dependencies() []string {
	return m.deps
}

func (m *testShutdownModule) Init(app modular.Application) error {
	*m.events = append(*m.events, m.name+".Init")
	return nil
}

func (m *testShutdownModule) Start(ctx context.Context) error {
	*m.events = append(*m.events, m.name+".Start")
	m.started = true
	return nil
}

func (m *testShutdownModule) Stop(ctx context.Context) error {
	*m.events = append(*m.events, m.name+".Stop")
	m.started = false
	return nil
}