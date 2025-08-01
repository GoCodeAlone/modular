package reverseproxy

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/GoCodeAlone/modular"
)

// TestFileBasedFeatureFlagEvaluator_WithMockApp tests the feature flag evaluator with a mock application
func TestFileBasedFeatureFlagEvaluator_WithMockApp(t *testing.T) {
	// Create mock application
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	config := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"test-flag-1": true,
				"test-flag-2": false,
			},
		},
	}

	app := NewMockTenantApplication()
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

	// Create tenant service (optional for this test)
	tenantService := modular.NewStandardTenantService(logger)
	err := app.RegisterService("tenantService", tenantService)
	if err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	evaluator, err := NewFileBasedFeatureFlagEvaluator(app, logger)
	if err != nil {
		t.Fatalf("Failed to create feature flag evaluator: %v", err)
	}

	req := httptest.NewRequest("GET", "/test", nil)

	// Test enabled flag
	enabled, err := evaluator.EvaluateFlag(context.Background(), "test-flag-1", "", req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !enabled {
		t.Error("Expected flag to be enabled")
	}

	// Test disabled flag
	enabled, err = evaluator.EvaluateFlag(context.Background(), "test-flag-2", "", req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if enabled {
		t.Error("Expected flag to be disabled")
	}

	// Test non-existent flag
	_, err = evaluator.EvaluateFlag(context.Background(), "non-existent-flag", "", req)
	if err == nil {
		t.Error("Expected error for non-existent flag")
	}
}

// TestFileBasedFeatureFlagEvaluator_WithDefault tests the evaluator with default values
func TestFileBasedFeatureFlagEvaluator_WithDefault(t *testing.T) {
	// Create mock application
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	config := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"existing-flag": true,
			},
		},
	}

	app := NewMockTenantApplication()
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

	tenantService := modular.NewStandardTenantService(logger)
	err := app.RegisterService("tenantService", tenantService)
	if err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	evaluator, err := NewFileBasedFeatureFlagEvaluator(app, logger)
	if err != nil {
		t.Fatalf("Failed to create feature flag evaluator: %v", err)
	}

	req := httptest.NewRequest("GET", "/test", nil)

	// Test existing flag with default
	result := evaluator.EvaluateFlagWithDefault(context.Background(), "existing-flag", "", req, false)
	if !result {
		t.Error("Expected existing flag to return true")
	}

	// Test non-existent flag with default
	result = evaluator.EvaluateFlagWithDefault(context.Background(), "non-existent-flag", "", req, true)
	if !result {
		t.Error("Expected non-existent flag to return default value true")
	}

	result = evaluator.EvaluateFlagWithDefault(context.Background(), "non-existent-flag", "", req, false)
	if result {
		t.Error("Expected non-existent flag to return default value false")
	}
}

// TestFileBasedFeatureFlagEvaluator_Disabled tests when feature flags are disabled
func TestFileBasedFeatureFlagEvaluator_Disabled(t *testing.T) {
	// Create mock application with disabled feature flags
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	config := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: false, // Disabled
			Flags: map[string]bool{
				"test-flag": true,
			},
		},
	}

	app := NewMockTenantApplication()
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

	tenantService := modular.NewStandardTenantService(logger)
	err := app.RegisterService("tenantService", tenantService)
	if err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	evaluator, err := NewFileBasedFeatureFlagEvaluator(app, logger)
	if err != nil {
		t.Fatalf("Failed to create feature flag evaluator: %v", err)
	}

	req := httptest.NewRequest("GET", "/test", nil)

	// Test that flags return error when disabled
	_, err = evaluator.EvaluateFlag(context.Background(), "test-flag", "", req)
	if err == nil {
		t.Error("Expected error when feature flags are disabled")
	}

	// Test that flags return default when disabled
	result := evaluator.EvaluateFlagWithDefault(context.Background(), "test-flag", "", req, false)
	if result {
		t.Error("Expected default value when feature flags are disabled")
	}
}
