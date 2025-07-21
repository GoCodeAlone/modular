package reverseproxy

import (
	"net/http/httptest"
	"testing"
)

// TestFileBasedFeatureFlagEvaluator tests the file-based feature flag evaluator
func TestFileBasedFeatureFlagEvaluator(t *testing.T) {
	evaluator := NewFileBasedFeatureFlagEvaluator()

	// Test setting and evaluating global flags
	evaluator.SetFlag("test-flag-1", true)
	evaluator.SetFlag("test-flag-2", false)

	req := httptest.NewRequest("GET", "/test", nil)

	// Test enabled flag
	enabled, err := evaluator.EvaluateFlag(req.Context(), "test-flag-1", "", req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !enabled {
		t.Error("Expected flag to be enabled")
	}

	// Test disabled flag
	enabled, err = evaluator.EvaluateFlag(req.Context(), "test-flag-2", "", req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if enabled {
		t.Error("Expected flag to be disabled")
	}

	// Test non-existent flag (should return error)
	enabled, err = evaluator.EvaluateFlag(req.Context(), "non-existent", "", req)
	if err == nil {
		t.Error("Expected error for non-existent flag")
	}
	if enabled {
		t.Error("Expected non-existent flag to be disabled")
	}
}

// TestFileBasedFeatureFlagEvaluator_TenantSpecific tests tenant-specific feature flags
func TestFileBasedFeatureFlagEvaluator_TenantSpecific(t *testing.T) {
	evaluator := NewFileBasedFeatureFlagEvaluator()

	// Set global and tenant-specific flags
	evaluator.SetFlag("global-flag", true)
	evaluator.SetTenantFlag("tenant1", "global-flag", false) // Override global
	evaluator.SetTenantFlag("tenant1", "tenant-flag", true)

	req := httptest.NewRequest("GET", "/test", nil)

	// Test global flag for tenant1 (should be overridden)
	enabled, err := evaluator.EvaluateFlag(req.Context(), "global-flag", "tenant1", req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if enabled {
		t.Error("Expected tenant1 to have global-flag disabled")
	}

	// Test global flag for tenant2 (should use global value)
	enabled, err = evaluator.EvaluateFlag(req.Context(), "global-flag", "tenant2", req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !enabled {
		t.Error("Expected tenant2 to have global-flag enabled")
	}

	// Test tenant-specific flag
	enabled, err = evaluator.EvaluateFlag(req.Context(), "tenant-flag", "tenant1", req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !enabled {
		t.Error("Expected tenant1 to have tenant-flag enabled")
	}
}

// TestFileBasedFeatureFlagEvaluator_WithDefault tests the EvaluateFlagWithDefault method
func TestFileBasedFeatureFlagEvaluator_WithDefault(t *testing.T) {
	evaluator := NewFileBasedFeatureFlagEvaluator()

	req := httptest.NewRequest("GET", "/test", nil)

	// Test non-existent flag with default true
	enabled := evaluator.EvaluateFlagWithDefault(req.Context(), "non-existent", "", req, true)
	if !enabled {
		t.Error("Expected default value true for non-existent flag")
	}

	// Test non-existent flag with default false
	enabled = evaluator.EvaluateFlagWithDefault(req.Context(), "non-existent", "", req, false)
	if enabled {
		t.Error("Expected default value false for non-existent flag")
	}

	// Test existing flag (should ignore default)
	evaluator.SetFlag("existing-flag", false)
	enabled = evaluator.EvaluateFlagWithDefault(req.Context(), "existing-flag", "", req, true)
	if enabled {
		t.Error("Expected actual flag value to override default")
	}
}

// TestReverseProxyModule_FeatureFlagEvaluation tests that the module correctly evaluates feature flags
func TestReverseProxyModule_FeatureFlagEvaluation(t *testing.T) {
	// Create a mock application
	app := &MockTenantApplication{}

	// Create and configure the module
	module := NewModule()
	module.app = app
	module.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"backend1": "http://backend1.example.com",
			"backend2": "http://backend2.example.com",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"backend1": {
				FeatureFlagID:      "backend1-flag",
				AlternativeBackend: "backend2",
			},
		},
	}

	// Create and set up feature flag evaluator
	evaluator := NewFileBasedFeatureFlagEvaluator()
	evaluator.SetFlag("backend1-flag", false) // Disable backend1
	module.featureFlagEvaluator = evaluator

	// Test evaluateFeatureFlag method
	req := httptest.NewRequest("GET", "/test", nil)

	// Test enabled flag
	evaluator.SetFlag("enabled-flag", true)
	if !module.evaluateFeatureFlag("enabled-flag", req) {
		t.Error("Expected enabled flag to return true")
	}

	// Test disabled flag
	evaluator.SetFlag("disabled-flag", false)
	if module.evaluateFeatureFlag("disabled-flag", req) {
		t.Error("Expected disabled flag to return false")
	}

	// Test empty flag ID (should default to true)
	if !module.evaluateFeatureFlag("", req) {
		t.Error("Expected empty flag ID to default to true")
	}

	// Test with nil evaluator (should default to true)
	module.featureFlagEvaluator = nil
	if !module.evaluateFeatureFlag("any-flag", req) {
		t.Error("Expected nil evaluator to default to true")
	}
}

// TestReverseProxyModule_GetAlternativeBackend tests the alternative backend selection logic
func TestReverseProxyModule_GetAlternativeBackend(t *testing.T) {
	module := NewModule()
	module.defaultBackend = "default-backend"

	// Test with specified alternative
	alt := module.getAlternativeBackend("custom-backend")
	if alt != "custom-backend" {
		t.Errorf("Expected 'custom-backend', got '%s'", alt)
	}

	// Test with empty alternative (should use default)
	alt = module.getAlternativeBackend("")
	if alt != "default-backend" {
		t.Errorf("Expected 'default-backend', got '%s'", alt)
	}
}
