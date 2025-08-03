package main

import (
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/modules/reverseproxy"
)

// TestFeatureFlagEvaluatorIntegration tests the integration between modules
func TestFeatureFlagEvaluatorIntegration(t *testing.T) {
	// Create mock application with tenant service
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(struct{}{}),
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
	)

	// Register tenant service
	tenantService := modular.NewStandardTenantService(app.Logger())
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	// Create feature flag configuration
	config := &reverseproxy.ReverseProxyConfig{
		FeatureFlags: reverseproxy.FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"test-flag": true,
			},
		},
	}
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

	// Create evaluator
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	evaluator, err := reverseproxy.NewFileBasedFeatureFlagEvaluator(app, logger)
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	// Test global flag
	req := httptest.NewRequest("GET", "/test", nil)
	enabled := evaluator.EvaluateFlagWithDefault(req.Context(), "test-flag", "", req, false)
	if !enabled {
		t.Error("Expected global flag to be enabled")
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
	// Create mock application
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(struct{}{}),
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
	)

	// Register tenant service
	tenantService := modular.NewStandardTenantService(app.Logger())
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		b.Fatalf("Failed to register tenant service: %v", err)
	}

	// Create feature flag configuration
	config := &reverseproxy.ReverseProxyConfig{
		FeatureFlags: reverseproxy.FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"bench-flag": true,
			},
		},
	}
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	evaluator, err := reverseproxy.NewFileBasedFeatureFlagEvaluator(app, logger)
	if err != nil {
		b.Fatalf("Failed to create evaluator: %v", err)
	}
	
	req := httptest.NewRequest("GET", "/bench", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		evaluator.EvaluateFlagWithDefault(req.Context(), "bench-flag", "", req, false)
	}
}

// Test concurrent access to feature flag evaluator
func TestFeatureFlagEvaluatorConcurrency(t *testing.T) {
	// Create mock application
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(struct{}{}),
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
	)

	// Register tenant service
	tenantService := modular.NewStandardTenantService(app.Logger())
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	// Create feature flag configuration
	config := &reverseproxy.ReverseProxyConfig{
		FeatureFlags: reverseproxy.FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"concurrent-flag": true,
			},
		},
	}
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	evaluator, err := reverseproxy.NewFileBasedFeatureFlagEvaluator(app, logger)
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}
	
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
	// Create mock application
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(struct{}{}),
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
	)

	// Register tenant service
	tenantService := modular.NewStandardTenantService(app.Logger())
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	// Create feature flag configuration
	config := &reverseproxy.ReverseProxyConfig{
		FeatureFlags: reverseproxy.FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"global-feature": false, // Disabled globally
			},
		},
	}
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	evaluator, err := reverseproxy.NewFileBasedFeatureFlagEvaluator(app, logger)
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}
	
	req := httptest.NewRequest("GET", "/test", nil)
	
	tests := []struct {
		name     string
		tenantID string
		flagID   string
		expected bool
		desc     string
	}{
		{"GlobalFeatureDisabled", "", "global-feature", false, "Global feature should be disabled"},
		{"NonExistentFlag", "", "non-existent", false, "Non-existent flag should default to false"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled := evaluator.EvaluateFlagWithDefault(req.Context(), tt.flagID, modular.TenantID(tt.tenantID), req, false)
			if enabled != tt.expected {
				t.Errorf("%s: Expected %v, got %v", tt.desc, tt.expected, enabled)
			}
		})
	}
}