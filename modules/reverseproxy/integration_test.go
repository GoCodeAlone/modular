package reverseproxy

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/GoCodeAlone/modular"
)

// Integration tests for the complete feature flag aggregator system

// ExternalEvaluator simulates a third-party feature flag evaluator module
type ExternalEvaluator struct {
	name   string
	weight int
}

func (e *ExternalEvaluator) Name() string {
	return e.name
}

func (e *ExternalEvaluator) Init(app modular.Application) error {
	return nil
}

func (e *ExternalEvaluator) Dependencies() []string {
	return nil
}

func (e *ExternalEvaluator) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:     "featureFlagEvaluator.external",
			Instance: &ExternalEvaluatorService{weight: e.weight},
		},
	}
}

func (e *ExternalEvaluator) RequiresServices() []modular.ServiceDependency {
	return nil
}

// ExternalEvaluatorService implements FeatureFlagEvaluator and WeightedEvaluator
type ExternalEvaluatorService struct {
	weight int
}

func (e *ExternalEvaluatorService) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
	switch flagID {
	case "external-only-flag":
		return true, nil
	case "priority-test-flag":
		return true, nil // External should win over file evaluator due to lower weight
	default:
		return false, ErrNoDecision // Let other evaluators handle
	}
}

func (e *ExternalEvaluatorService) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
	result, err := e.EvaluateFlag(ctx, flagID, tenantID, req)
	if err != nil {
		return defaultValue
	}
	return result
}

func (e *ExternalEvaluatorService) Weight() int {
	return e.weight
}

// TestCompleteFeatureFlagSystem tests the entire aggregator system end-to-end
func TestCompleteFeatureFlagSystem(t *testing.T) {
	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create application
	app := NewMockTenantApplication()
	
	// Register tenant service
	tenantService := modular.NewStandardTenantService(logger)
	err := app.RegisterService("tenantService", tenantService)
	if err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	// Create reverseproxy configuration with file-based flags
	rpConfig := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"file-only-flag":    true,
				"priority-test-flag": false, // External should override this
				"fallback-flag":     true,
			},
		},
		BackendServices: map[string]string{
			"test": "http://localhost:8080",
		},
	}
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(rpConfig))

	// Create and register external evaluator module (simulates third-party module)
	externalModule := &ExternalEvaluator{
		name:   "external-evaluator",
		weight: 50, // Higher priority than file evaluator (weight 1000)
	}
	
	// Create mock router for reverseproxy
	router := &MockRouter{}
	err = app.RegisterService("router", router)
	if err != nil {
		t.Fatalf("Failed to register router: %v", err)
	}

	// Register reverseproxy module
	rpModule := NewModule()
	
	// Register modules
	app.RegisterModule(externalModule)
	app.RegisterModule(rpModule)

	// Initialize application
	err = app.Init()
	if err != nil {
		t.Fatalf("Failed to initialize application: %v", err)
	}

	// Get the reverseproxy module instance to test its evaluator
	var modules []modular.Module
	for _, m := range []modular.Module{externalModule, rpModule} {
		modules = append(modules, m)
	}

	// Find the initialized reverseproxy module
	var initializedRP *ReverseProxyModule
	for _, m := range modules {
		if rp, ok := m.(*ReverseProxyModule); ok {
			initializedRP = rp
			break
		}
	}

	if initializedRP == nil {
		t.Fatal("Could not find initialized ReverseProxyModule")
	}

	// Test the aggregator behavior
	req := httptest.NewRequest("GET", "/test", nil)

	t.Run("External evaluator takes precedence", func(t *testing.T) {
		// External-only flag should work
		result := initializedRP.evaluateFeatureFlag("external-only-flag", req)
		if !result {
			t.Error("Expected external-only-flag to be true from external evaluator")
		}
	})

	t.Run("Priority ordering works", func(t *testing.T) {
		// Priority test flag: external (true) should override file (false)
		result := initializedRP.evaluateFeatureFlag("priority-test-flag", req)
		if !result {
			t.Error("Expected external evaluator to override file evaluator for priority-test-flag")
		}
	})

	t.Run("Fallback to file evaluator", func(t *testing.T) {
		// Fallback flag should work through file evaluator
		result := initializedRP.evaluateFeatureFlag("fallback-flag", req)
		if !result {
			t.Error("Expected fallback-flag to work through file evaluator")
		}
	})

	t.Run("Unknown flags return default", func(t *testing.T) {
		// Unknown flags should return default (true for reverseproxy)
		result := initializedRP.evaluateFeatureFlag("unknown-flag", req)
		if !result {
			t.Error("Expected unknown flag to return default value (true)")
		}
	})
}

// TestBackwardsCompatibility tests that existing evaluator usage still works
func TestBackwardsCompatibility(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create application
	app := NewMockTenantApplication()
	
	// Register tenant service
	tenantService := modular.NewStandardTenantService(logger)
	err := app.RegisterService("tenantService", tenantService)
	if err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	// Create configuration
	rpConfig := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"test-flag": true,
			},
		},
		BackendServices: map[string]string{
			"test": "http://localhost:8080",
		},
	}
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(rpConfig))

	// Test that file-based evaluator can be created directly (backwards compatibility)
	fileEvaluator, err := NewFileBasedFeatureFlagEvaluator(app, logger)
	if err != nil {
		t.Fatalf("Failed to create file-based evaluator: %v", err)
	}

	// Test that it can evaluate flags
	req := httptest.NewRequest("GET", "/test", nil)
	result, err := fileEvaluator.EvaluateFlag(context.Background(), "test-flag", "", req)
	if err != nil {
		t.Fatalf("Failed to evaluate flag: %v", err)
	}
	if !result {
		t.Error("Expected test-flag to be true")
	}

	// Test default value behavior
	defaultResult := fileEvaluator.EvaluateFlagWithDefault(context.Background(), "unknown-flag", "", req, true)
	if !defaultResult {
		t.Error("Expected unknown flag to return default value true")
	}

	// Test that aggregator can be created and works with file evaluator
	err = app.RegisterService("featureFlagEvaluator.file", fileEvaluator)
	if err != nil {
		t.Fatalf("Failed to register file evaluator: %v", err)
	}

	aggregator := NewFeatureFlagAggregator(app, logger)
	
	// Test aggregator with just the file evaluator
	result, err = aggregator.EvaluateFlag(context.Background(), "test-flag", "", req)
	if err != nil {
		t.Fatalf("Aggregator failed to evaluate flag: %v", err)
	}
	if !result {
		t.Error("Expected aggregator to return true for test-flag via file evaluator")
	}
}

// TestServiceExposure tests that the aggregator properly exposes services
func TestServiceExposure(t *testing.T) {
	// Test that a basic module provides the expected service structure
	rpModule := NewModule()
	
	// Before configuration, should provide minimal services
	initialServices := rpModule.ProvidesServices()
	if len(initialServices) != 0 {
		t.Errorf("Expected no services before configuration, got %d", len(initialServices))
	}

	// Test that services are provided after configuration
	rpModule.config = &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{Enabled: true},
	}
	
	// Create a dummy aggregator for testing
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	app := NewMockTenantApplication()
	rpModule.featureFlagEvaluator = NewFeatureFlagAggregator(app, logger)

	// Now should provide services
	services := rpModule.ProvidesServices()
	
	// Should provide both reverseproxy.provider and featureFlagEvaluator services
	var hasProvider, hasEvaluator bool
	for _, svc := range services {
		switch svc.Name {
		case "reverseproxy.provider":
			hasProvider = true
		case "featureFlagEvaluator":
			hasEvaluator = true
		}
	}

	if !hasProvider {
		t.Error("Expected reverseproxy.provider service to be provided")
	}

	if !hasEvaluator {
		t.Error("Expected featureFlagEvaluator service to be provided")
	}
}

// TestNoCyclePrevention tests that the cycle prevention mechanisms work
func TestNoCyclePrevention(t *testing.T) {
	// This test would create a scenario where an external evaluator
	// depends on reverseproxy's featureFlagEvaluator service, creating a potential cycle.
	// The system should handle this by using proper service naming.

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	app := NewMockTenantApplication()

	// Create a potentially problematic external module that tries to depend on reverseproxy
	problematicModule := &ProblematicExternalModule{name: "problematic"}
	rpModule := NewModule()

	// Register tenant service
	tenantService := modular.NewStandardTenantService(logger)
	err := app.RegisterService("tenantService", tenantService)
	if err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	// Create configuration
	rpConfig := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags:   map[string]bool{"test": true},
		},
		BackendServices: map[string]string{
			"test": "http://localhost:8080",
		},
	}
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(rpConfig))

	// Create mock router
	router := &MockRouter{}
	err = app.RegisterService("router", router)
	if err != nil {
		t.Fatalf("Failed to register router: %v", err)
	}

	app.RegisterModule(rpModule)
	app.RegisterModule(problematicModule)

	// This should initialize without cycle errors due to proper service naming
	err = app.Init()
	if err != nil {
		// If there's an error, it shouldn't be a cycle error since we use proper naming
		if errors.Is(err, modular.ErrCircularDependency) {
			t.Errorf("Unexpected cycle error with proper service naming: %v", err)
		}
	}
}

// ProblematicExternalModule tries to create a cycle by depending on featureFlagEvaluator
type ProblematicExternalModule struct {
	name string
}

func (m *ProblematicExternalModule) Name() string {
	return m.name
}

func (m *ProblematicExternalModule) Init(app modular.Application) error {
	return nil
}

func (m *ProblematicExternalModule) Dependencies() []string {
	return nil
}

func (m *ProblematicExternalModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:     "featureFlagEvaluator.problematic",
			Instance: &SimpleEvaluator{},
		},
	}
}

func (m *ProblematicExternalModule) RequiresServices() []modular.ServiceDependency {
	// This module tries to consume the featureFlagEvaluator service
	// In the old system, this could create a cycle
	// In the new system, it should be safe due to aggregator pattern
	return []modular.ServiceDependency{
		{
			Name:               "featureFlagEvaluator",
			Required:           false,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*FeatureFlagEvaluator)(nil)).Elem(),
		},
	}
}

// SimpleEvaluator is a basic evaluator implementation
type SimpleEvaluator struct{}

func (s *SimpleEvaluator) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
	return false, ErrNoDecision
}

func (s *SimpleEvaluator) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
	return defaultValue
}