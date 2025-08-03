package modular

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Test the new builder API
func TestNewApplication_BasicBuilder(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

	app, err := NewApplication(
		WithLogger(logger),
		WithConfigProvider(NewStdConfigProvider(&struct{}{})),
	)

	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}

	if app == nil {
		t.Fatal("Application is nil")
	}

	if app.Logger() != logger {
		t.Error("Logger not set correctly")
	}
}

func TestNewApplication_WithModules(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

	module1 := &MockModule{name: "module1"}
	module2 := &MockModule{name: "module2"}

	app, err := NewApplication(
		WithLogger(logger),
		WithConfigProvider(NewStdConfigProvider(&struct{}{})),
		WithModules(module1, module2),
	)

	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}

	// Check if modules were registered
	if len(app.(*StdApplication).moduleRegistry) != 2 {
		t.Errorf("Expected 2 modules, got %d", len(app.(*StdApplication).moduleRegistry))
	}
}

func TestNewApplication_WithObserver(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

	observer := func(ctx context.Context, event cloudevents.Event) error {
		return nil
	}

	app, err := NewApplication(
		WithLogger(logger),
		WithConfigProvider(NewStdConfigProvider(&struct{}{})),
		WithObserver(observer),
	)

	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}

	// Should create an ObservableDecorator
	if _, ok := app.(*ObservableDecorator); !ok {
		t.Error("Expected ObservableDecorator when WithObserver is used")
	}
}

func TestNewApplication_WithTenantAware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

	tenantLoader := &MockTenantLoader{}

	app, err := NewApplication(
		WithLogger(logger),
		WithConfigProvider(NewStdConfigProvider(&struct{}{})),
		WithTenantAware(tenantLoader),
	)

	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}

	// Should create a TenantAwareDecorator
	if _, ok := app.(*TenantAwareDecorator); !ok {
		t.Error("Expected TenantAwareDecorator when WithTenantAware is used")
	}
}

func TestNewApplication_WithConfigDecorators(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

	app, err := NewApplication(
		WithLogger(logger),
		WithConfigProvider(NewStdConfigProvider(&struct{}{})),
		WithConfigDecorators(InstanceAwareConfig()),
	)

	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}

	if app == nil {
		t.Fatal("Application is nil")
	}
}

func TestNewApplication_MissingLogger(t *testing.T) {
	_, err := NewApplication(
		WithConfigProvider(NewStdConfigProvider(&struct{}{})),
	)

	if err == nil {
		t.Error("Expected error when logger is not provided")
	}

	if !errors.Is(err, ErrLoggerNotSet) {
		t.Errorf("Expected ErrLoggerNotSet, got %v", err)
	}
}

// Mock types for testing

type MockModule struct {
	name string
}

func (m *MockModule) Name() string {
	return m.name
}

func (m *MockModule) Init(app Application) error {
	return nil
}

type MockTenantLoader struct{}

func (m *MockTenantLoader) LoadTenants() ([]Tenant, error) {
	return []Tenant{
		{ID: "tenant1", Name: "Tenant 1"},
		{ID: "tenant2", Name: "Tenant 2"},
	}, nil
}
