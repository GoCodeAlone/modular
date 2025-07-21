package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/modules/reverseproxy"
)

// TestFeatureFlagEvaluatorIntegration tests the integration between modules
func TestFeatureFlagEvaluatorIntegration(t *testing.T) {
	// Create evaluator
	evaluator := reverseproxy.NewFileBasedFeatureFlagEvaluator()
	evaluator.SetFlag("test-flag", true)
	evaluator.SetTenantFlag("test-tenant", "test-flag", false)
	
	// Test global flag
	req := httptest.NewRequest("GET", "/test", nil)
	enabled := evaluator.EvaluateFlagWithDefault(req.Context(), "test-flag", "", req, false)
	if !enabled {
		t.Error("Expected global flag to be enabled")
	}
	
	// Test tenant override
	enabled = evaluator.EvaluateFlagWithDefault(req.Context(), "test-flag", "test-tenant", req, true)
	if enabled {
		t.Error("Expected tenant flag to be disabled")
	}
	
	// Test non-existent flag with default
	enabled = evaluator.EvaluateFlagWithDefault(req.Context(), "non-existent", "", req, true)
	if !enabled {
		t.Error("Expected default value for non-existent flag")
	}
}

// TestBackendResponse tests backend response parsing
func TestBackendResponse(t *testing.T) {
	// Test parsing a mock backend response
	response := `{"backend":"default","path":"/api/test","method":"GET","feature":"stable"}`
	
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	
	if result["backend"] != "default" {
		t.Errorf("Expected backend 'default', got %v", result["backend"])
	}
	
	if result["feature"] != "stable" {
		t.Errorf("Expected feature 'stable', got %v", result["feature"])
	}
}

// Benchmark feature flag evaluation performance
func BenchmarkFeatureFlagEvaluation(b *testing.B) {
	evaluator := reverseproxy.NewFileBasedFeatureFlagEvaluator()
	evaluator.SetFlag("bench-flag", true)
	
	req := httptest.NewRequest("GET", "/bench", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		evaluator.EvaluateFlagWithDefault(req.Context(), "bench-flag", "", req, false)
	}
}

// Test concurrent access to feature flag evaluator
func TestFeatureFlagEvaluatorConcurrency(t *testing.T) {
	evaluator := reverseproxy.NewFileBasedFeatureFlagEvaluator()
	evaluator.SetFlag("concurrent-flag", true)
	
	// Run multiple goroutines accessing the evaluator
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			req := httptest.NewRequest("GET", "/concurrent", nil)
			for j := 0; j < 100; j++ {
				enabled := evaluator.EvaluateFlagWithDefault(req.Context(), "concurrent-flag", "", req, false)
				if !enabled {
					t.Errorf("Goroutine %d: Expected flag to be enabled", id)
				}
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines to complete with timeout
	timeout := time.After(5 * time.Second)
	completed := 0
	
	for completed < 10 {
		select {
		case <-done:
			completed++
		case <-timeout:
			t.Fatal("Test timed out")
		}
	}
}

// TestTenantSpecificFeatureFlags tests tenant-specific feature flag overrides
func TestTenantSpecificFeatureFlags(t *testing.T) {
	evaluator := reverseproxy.NewFileBasedFeatureFlagEvaluator()
	
	// Set up feature flags
	evaluator.SetFlag("global-feature", false)  // Disabled globally
	evaluator.SetTenantFlag("premium-tenant", "global-feature", true) // Enabled for premium
	evaluator.SetTenantFlag("beta-tenant", "beta-feature", true) // Beta-only feature
	
	req := httptest.NewRequest("GET", "/test", nil)
	
	tests := []struct {
		name     string
		tenantID string
		flagID   string
		expected bool
		desc     string
	}{
		{"GlobalFeatureDisabled", "", "global-feature", false, "Global feature should be disabled"},
		{"PremiumTenantOverride", "premium-tenant", "global-feature", true, "Premium tenant should have global feature enabled"},
		{"BetaTenantSpecific", "beta-tenant", "beta-feature", true, "Beta tenant should have beta feature enabled"},
		{"RegularTenantNoBeta", "regular-tenant", "beta-feature", false, "Regular tenant should not have beta feature"},
		{"NonExistentFlag", "", "non-existent", false, "Non-existent flag should default to false"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For flags that might not exist globally, use EvaluateFlagWithDefault
			if tt.flagID == "beta-feature" && tt.tenantID == "regular-tenant" {
				enabled := evaluator.EvaluateFlagWithDefault(req.Context(), tt.flagID, modular.TenantID(tt.tenantID), req, false)
				if enabled != tt.expected {
					t.Errorf("%s: Expected %v, got %v", tt.desc, tt.expected, enabled)
				}
				return
			}
			
			enabled, err := evaluator.EvaluateFlag(req.Context(), tt.flagID, modular.TenantID(tt.tenantID), req)
			
			// For non-existent flags, we expect an error
			if tt.flagID == "non-existent" {
				if err == nil {
					t.Errorf("%s: Expected error for non-existent flag", tt.desc)
				}
				return
			}
			
			if err != nil {
				t.Errorf("%s: Unexpected error: %v", tt.desc, err)
				return
			}
			
			if enabled != tt.expected {
				t.Errorf("%s: Expected %v, got %v", tt.desc, tt.expected, enabled)
			}
		})
	}
}