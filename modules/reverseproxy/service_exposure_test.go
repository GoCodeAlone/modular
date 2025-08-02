package reverseproxy

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/GoCodeAlone/modular"
)

// TestFeatureFlagEvaluatorServiceExposure tests that the module exposes the feature flag evaluator as a service
func TestFeatureFlagEvaluatorServiceExposure(t *testing.T) {
	tests := []struct {
		name          string
		config        *ReverseProxyConfig
		expectService bool
		expectFlags   int
	}{
		{
			name: "FeatureFlagsDisabled",
			config: &ReverseProxyConfig{
				BackendServices: map[string]string{
					"test": "http://test:8080",
				},
				FeatureFlags: FeatureFlagsConfig{
					Enabled: false,
				},
			},
			expectService: false,
		},
		{
			name: "FeatureFlagsEnabledNoDefaults",
			config: &ReverseProxyConfig{
				BackendServices: map[string]string{
					"test": "http://test:8080",
				},
				FeatureFlags: FeatureFlagsConfig{
					Enabled: true,
				},
			},
			expectService: true,
			expectFlags:   0,
		},
		{
			name: "FeatureFlagsEnabledWithDefaults",
			config: &ReverseProxyConfig{
				BackendServices: map[string]string{
					"test": "http://test:8080",
				},
				FeatureFlags: FeatureFlagsConfig{
					Enabled: true,
					Flags: map[string]bool{
						"flag-1": true,
						"flag-2": false,
					},
				},
			},
			expectService: true,
			expectFlags:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock router
			mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}

			// Create mock application
			app := NewMockTenantApplication()

			// Register the configuration with the application
			app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(tt.config))

			// Create module
			module := NewModule()

			// Set the configuration
			module.config = tt.config

			// Set router via constructor
			services := map[string]any{
				"router": mockRouter,
			}
			constructedModule, err := module.Constructor()(app, services)
			if err != nil {
				t.Fatalf("Failed to construct module: %v", err)
			}
			module = constructedModule.(*ReverseProxyModule)

			// Set the app reference
			module.app = app

			// Start the module to trigger feature flag evaluator creation
			if err := module.Start(context.Background()); err != nil {
				t.Fatalf("Failed to start module: %v", err)
			}

			// Test service exposure
			providedServices := module.ProvidesServices()

			if tt.expectService {
				// Should provide exactly one service (featureFlagEvaluator)
				if len(providedServices) != 1 {
					t.Errorf("Expected 1 provided service, got %d", len(providedServices))
					return
				}

				service := providedServices[0]
				if service.Name != "featureFlagEvaluator" {
					t.Errorf("Expected service name 'featureFlagEvaluator', got '%s'", service.Name)
				}

				// Verify the service implements FeatureFlagEvaluator
				if _, ok := service.Instance.(FeatureFlagEvaluator); !ok {
					t.Errorf("Expected service to implement FeatureFlagEvaluator, got %T", service.Instance)
				}

				// Test that it's the FileBasedFeatureFlagEvaluator specifically
				evaluator, ok := service.Instance.(*FileBasedFeatureFlagEvaluator)
				if !ok {
					t.Errorf("Expected service to be *FileBasedFeatureFlagEvaluator, got %T", service.Instance)
					return
				}

				// Test configuration was applied correctly
				req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)

				// Test flags
				if tt.expectFlags > 0 {
					for flagID, expectedValue := range tt.config.FeatureFlags.Flags {
						actualValue, err := evaluator.EvaluateFlag(context.Background(), flagID, "", req)
						if err != nil {
							t.Errorf("Error evaluating flag %s: %v", flagID, err)
						}
						if actualValue != expectedValue {
							t.Errorf("Flag %s: expected %v, got %v", flagID, expectedValue, actualValue)
						}
					}
				}

			} else {
				// Should not provide any services
				if len(providedServices) != 0 {
					t.Errorf("Expected 0 provided services, got %d", len(providedServices))
				}
			}
		})
	}
}

// TestFeatureFlagEvaluatorServiceDependencyResolution tests that external services take precedence
func TestFeatureFlagEvaluatorServiceDependencyResolution(t *testing.T) {
	// Create mock router
	mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}

	// Create external feature flag evaluator
	app := NewMockTenantApplication()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Configure the external evaluator with flags
	externalConfig := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"external-flag": true,
			},
		},
	}
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(externalConfig))

	externalEvaluator, err := NewFileBasedFeatureFlagEvaluator(app, logger)
	if err != nil {
		t.Fatalf("Failed to create feature flag evaluator: %v", err)
	}

	// Create mock application - already created above

	// Create a separate application for the module
	moduleApp := NewMockTenantApplication()

	// Register the module configuration with the module app
	moduleApp.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(&ReverseProxyConfig{
		BackendServices: map[string]string{
			"test": "http://test:8080",
		},
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"internal-flag": true,
			},
		},
	}))

	// Create module
	module := NewModule()

	// Set configuration with feature flags enabled
	module.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"test": "http://test:8080",
		},
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"internal-flag": true,
			},
		},
	}

	// Set router and external evaluator via constructor
	services := map[string]any{
		"router":               mockRouter,
		"featureFlagEvaluator": externalEvaluator,
	}
	constructedModule, err := module.Constructor()(moduleApp, services)
	if err != nil {
		t.Fatalf("Failed to construct module: %v", err)
	}
	module = constructedModule.(*ReverseProxyModule)

	// Set the app reference
	module.app = moduleApp

	// Start the module
	if err := module.Start(context.Background()); err != nil {
		t.Fatalf("Failed to start module: %v", err)
	}

	// Test that the external evaluator is used, not the internal one
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)

	// The external flag should exist
	externalValue, err := module.featureFlagEvaluator.EvaluateFlag(context.Background(), "external-flag", "", req)
	if err != nil {
		t.Errorf("Error evaluating external flag: %v", err)
	}
	if !externalValue {
		t.Error("Expected external flag to be true")
	}

	// The internal flag should not exist (because we're using external evaluator)
	_, err = module.featureFlagEvaluator.EvaluateFlag(context.Background(), "internal-flag", "", req)
	if err == nil {
		t.Error("Expected internal flag to not exist when using external evaluator")
	}

	// The module should still provide the service (it's the external one)
	providedServices := module.ProvidesServices()
	if len(providedServices) != 1 {
		t.Errorf("Expected 1 provided service, got %d", len(providedServices))
		return
	}

	// Verify it's the same instance as the external evaluator
	if providedServices[0].Instance != externalEvaluator {
		t.Error("Expected provided service to be the same instance as external evaluator")
	}
}

// TestFeatureFlagEvaluatorConfigValidation tests configuration validation
func TestFeatureFlagEvaluatorConfigValidation(t *testing.T) {
	// Create mock router
	mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}

	// Create mock application
	app := NewMockTenantApplication()

	// Create module
	module := NewModule()

	// Test with nil config (should not crash)
	module.config = nil

	// Set router via constructor
	services := map[string]any{
		"router": mockRouter,
	}
	constructedModule, err := module.Constructor()(app, services)
	if err != nil {
		t.Fatalf("Failed to construct module: %v", err)
	}
	module = constructedModule.(*ReverseProxyModule)

	// Set the app reference
	module.app = app

	// This should not crash even with nil config
	providedServices := module.ProvidesServices()
	if len(providedServices) != 0 {
		t.Errorf("Expected 0 provided services with nil config, got %d", len(providedServices))
	}
}

// TestServiceProviderInterface tests that the service properly implements the expected interface
func TestServiceProviderInterface(t *testing.T) {
	// Create the evaluator
	app := NewMockTenantApplication()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	evaluator, err := NewFileBasedFeatureFlagEvaluator(app, logger)
	if err != nil {
		t.Fatalf("Failed to create feature flag evaluator: %v", err)
	}

	// Test that it implements FeatureFlagEvaluator
	var _ FeatureFlagEvaluator = evaluator

	// Test using reflection (as the framework would)
	evaluatorType := reflect.TypeOf(evaluator)
	featureFlagInterface := reflect.TypeOf((*FeatureFlagEvaluator)(nil)).Elem()

	if !evaluatorType.Implements(featureFlagInterface) {
		t.Error("FileBasedFeatureFlagEvaluator does not implement FeatureFlagEvaluator interface")
	}
}
