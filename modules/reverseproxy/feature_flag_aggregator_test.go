package reverseproxy

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/CrisisTextLine/modular"
)

// Mock evaluators for testing

// mockHighPriorityEvaluator always returns true with weight 10 (high priority)
type mockHighPriorityEvaluator struct{}

func (m *mockHighPriorityEvaluator) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
	if flagID == "high-priority-flag" {
		return true, nil
	}
	return false, ErrNoDecision // Abstain for other flags
}

func (m *mockHighPriorityEvaluator) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
	result, err := m.EvaluateFlag(ctx, flagID, tenantID, req)
	if err != nil {
		return defaultValue
	}
	return result
}

func (m *mockHighPriorityEvaluator) Weight() int {
	return 10 // High priority
}

// mockMediumPriorityEvaluator returns true for medium flags with weight 50
type mockMediumPriorityEvaluator struct{}

func (m *mockMediumPriorityEvaluator) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
	if flagID == "medium-priority-flag" {
		return true, nil
	}
	return false, ErrNoDecision
}

func (m *mockMediumPriorityEvaluator) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
	result, err := m.EvaluateFlag(ctx, flagID, tenantID, req)
	if err != nil {
		return defaultValue
	}
	return result
}

func (m *mockMediumPriorityEvaluator) Weight() int {
	return 50 // Medium priority
}

// mockFatalErrorEvaluator returns ErrEvaluatorFatal for certain flags
type mockFatalErrorEvaluator struct{}

func (m *mockFatalErrorEvaluator) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
	if flagID == "fatal-flag" {
		return false, ErrEvaluatorFatal
	}
	return false, ErrNoDecision
}

func (m *mockFatalErrorEvaluator) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
	result, err := m.EvaluateFlag(ctx, flagID, tenantID, req)
	if err != nil {
		return defaultValue
	}
	return result
}

func (m *mockFatalErrorEvaluator) Weight() int {
	return 20 // Higher than medium, lower than high
}

// mockNonFatalErrorEvaluator returns a non-fatal error
type mockNonFatalErrorEvaluator struct{}

func (m *mockNonFatalErrorEvaluator) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
	if flagID == "error-flag" {
		return false, errors.New("non-fatal error")
	}
	return false, ErrNoDecision
}

func (m *mockNonFatalErrorEvaluator) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
	result, err := m.EvaluateFlag(ctx, flagID, tenantID, req)
	if err != nil {
		return defaultValue
	}
	return result
}

func (m *mockNonFatalErrorEvaluator) Weight() int {
	return 30
}

// Test aggregator priority ordering
func TestFeatureFlagAggregator_PriorityOrdering(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	// Create mock application with service registry
	app := NewMockTenantApplication()
	
	// Register mock evaluators with different priorities
	highPriority := &mockHighPriorityEvaluator{}
	mediumPriority := &mockMediumPriorityEvaluator{}
	
	err := app.RegisterService("featureFlagEvaluator.high", highPriority)
	if err != nil {
		t.Fatalf("Failed to register high priority evaluator: %v", err)
	}
	
	err = app.RegisterService("featureFlagEvaluator.medium", mediumPriority)
	if err != nil {
		t.Fatalf("Failed to register medium priority evaluator: %v", err)
	}
	
	// Create file evaluator configuration
	config := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"fallback-flag": true,
			},
		},
	}
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))
	
	// Register tenant service
	tenantService := modular.NewStandardTenantService(logger)
	err = app.RegisterService("tenantService", tenantService)
	if err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}
	
	// Create and register file evaluator
	fileEvaluator, err := NewFileBasedFeatureFlagEvaluator(app, logger)
	if err != nil {
		t.Fatalf("Failed to create file evaluator: %v", err)
	}
	err = app.RegisterService("featureFlagEvaluator.file", fileEvaluator)
	if err != nil {
		t.Fatalf("Failed to register file evaluator: %v", err)
	}
	
	// Create aggregator
	aggregator := NewFeatureFlagAggregator(app, logger)
	
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.Background()
	
	// Test high priority flag - should be handled by high priority evaluator
	result, err := aggregator.EvaluateFlag(ctx, "high-priority-flag", "", req)
	if err != nil {
		t.Errorf("Expected no error for high priority flag, got %v", err)
	}
	if !result {
		t.Error("Expected high priority flag to be true")
	}
	
	// Test medium priority flag - high priority should abstain, medium should handle
	result, err = aggregator.EvaluateFlag(ctx, "medium-priority-flag", "", req)
	if err != nil {
		t.Errorf("Expected no error for medium priority flag, got %v", err)
	}
	if !result {
		t.Error("Expected medium priority flag to be true")
	}
	
	// Test fallback flag - should fall through to file evaluator
	result, err = aggregator.EvaluateFlag(ctx, "fallback-flag", "", req)
	if err != nil {
		t.Errorf("Expected no error for fallback flag, got %v", err)
	}
	if !result {
		t.Error("Expected fallback flag to be true")
	}
}

// Test aggregator error handling
func TestFeatureFlagAggregator_ErrorHandling(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	app := NewMockTenantApplication()
	
	// Register evaluators with different error behaviors
	fatalEvaluator := &mockFatalErrorEvaluator{}
	nonFatalEvaluator := &mockNonFatalErrorEvaluator{}
	
	err := app.RegisterService("featureFlagEvaluator.fatal", fatalEvaluator)
	if err != nil {
		t.Fatalf("Failed to register fatal evaluator: %v", err)
	}
	
	err = app.RegisterService("featureFlagEvaluator.nonFatal", nonFatalEvaluator)
	if err != nil {
		t.Fatalf("Failed to register non-fatal evaluator: %v", err)
	}
	
	aggregator := NewFeatureFlagAggregator(app, logger)
	
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.Background()
	
	// Test fatal error - should stop evaluation chain
	_, err = aggregator.EvaluateFlag(ctx, "fatal-flag", "", req)
	if err == nil {
		t.Error("Expected fatal error to be propagated")
	}
	if !errors.Is(err, ErrEvaluatorFatal) {
		t.Errorf("Expected ErrEvaluatorFatal, got %v", err)
	}
	
	// Test non-fatal error - should continue to next evaluator
	// Since no evaluator handles "error-flag" successfully, should get no decision error
	_, err = aggregator.EvaluateFlag(ctx, "error-flag", "", req)
	if err == nil {
		t.Error("Expected error when no evaluator provides decision")
	}
	// Should not be the non-fatal error, should be "no decision" error
	if errors.Is(err, ErrEvaluatorFatal) {
		t.Error("Should not have fatal error for non-fatal evaluator error")
	}
}

// Test aggregator with no evaluators
func TestFeatureFlagAggregator_NoEvaluators(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	app := NewMockTenantApplication()
	aggregator := NewFeatureFlagAggregator(app, logger)
	
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.Background()
	
	// Should return error when no evaluators are available
	_, err := aggregator.EvaluateFlag(ctx, "any-flag", "", req)
	if err == nil {
		t.Error("Expected error when no evaluators available")
	}
}

// Test default value behavior
func TestFeatureFlagAggregator_DefaultValue(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	app := NewMockTenantApplication()
	aggregator := NewFeatureFlagAggregator(app, logger)
	
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.Background()
	
	// Should return default value when no evaluators provide decision
	result := aggregator.EvaluateFlagWithDefault(ctx, "any-flag", "", req, true)
	if !result {
		t.Error("Expected default value true to be returned")
	}
	
	result = aggregator.EvaluateFlagWithDefault(ctx, "any-flag", "", req, false)
	if result {
		t.Error("Expected default value false to be returned")
	}
}

// Test self-ingestion prevention
func TestFeatureFlagAggregator_PreventSelfIngestion(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	app := NewMockTenantApplication()
	aggregator := NewFeatureFlagAggregator(app, logger)
	
	// Register the aggregator as a service (simulating what ReverseProxyModule does)
	err := app.RegisterService("featureFlagEvaluator", aggregator)
	if err != nil {
		t.Fatalf("Failed to register aggregator: %v", err)
	}
	
	// The aggregator should not discover itself in the evaluators list
	evaluators := aggregator.discoverEvaluators()
	for _, eval := range evaluators {
		if eval.evaluator == aggregator {
			t.Error("Aggregator should not include itself in evaluators list")
		}
	}
}