package reverseproxy

import (
	"context"
	"net/http"
	"testing"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/mock"
)

// mockExternalEvaluatorReturnsErrNoDecision simulates an external evaluator (like LaunchDarkly)
// that is configured but returns ErrNoDecision due to initialization failures
type mockExternalEvaluatorReturnsErrNoDecision struct{}

func (m *mockExternalEvaluatorReturnsErrNoDecision) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
	// Simulate external evaluator that's configured but not working (e.g., invalid SDK key)
	return false, ErrNoDecision
}

func (m *mockExternalEvaluatorReturnsErrNoDecision) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
	result, err := m.EvaluateFlag(ctx, flagID, tenantID, req)
	if err != nil {
		return defaultValue
	}
	return result
}

func (m *mockExternalEvaluatorReturnsErrNoDecision) Weight() int {
	return 50 // Higher priority than file evaluator (weight 1000)
}

// TestExternalEvaluatorFallbackFix reproduces and verifies the fix for the bug described in the issue.
//
// The bug: When external evaluator is provided via constructor, it bypassed the aggregator entirely,
// so when it returned ErrNoDecision, there was no fallback to file-based evaluator.
//
// The fix: Always use aggregator pattern, register external evaluator for discovery,
// ensuring proper fallback chain: External → File → Safe defaults.
func TestExternalEvaluatorFallbackFix(t *testing.T) {
	// Create a mock application
	moduleApp := NewMockTenantApplication()

	// Configure reverseproxy with feature flags enabled and a flag set to true
	config := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"primary":     "http://127.0.0.1:18080",
			"alternative": "http://127.0.0.1:18081",
		},
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"my-api": false, // This should route to alternative backend when external evaluator abstains
			},
		},
	}

	// Register the configuration
	moduleApp.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

	// Create the module
	module := NewModule()

	// Create a mock router and set up expected calls
	mockRouter := &MockRouter{}
	// Allow any HandleFunc calls
	mockRouter.On("HandleFunc", mock.Anything, mock.Anything).Return()

	// Create an external evaluator that returns ErrNoDecision (simulating misconfigured LaunchDarkly)
	externalEvaluator := &mockExternalEvaluatorReturnsErrNoDecision{}

	// Provide services via constructor - this is where the bug occurs
	services := map[string]any{
		"router":               mockRouter,
		"featureFlagEvaluator": externalEvaluator, // This bypasses the aggregator
	}

	constructedModule, err := module.Constructor()(moduleApp, services)
	if err != nil {
		t.Fatalf("Failed to construct module: %v", err)
	}
	module = constructedModule.(*ReverseProxyModule)

	// Set up the configuration first
	err = module.RegisterConfig(moduleApp)
	if err != nil {
		t.Fatalf("Failed to register config: %v", err)
	}

	// Initialize the module (this sets up the feature flag evaluator)
	if err := module.Init(moduleApp); err != nil {
		t.Fatalf("Failed to initialize module: %v", err)
	}

	// Start the module (this calls setupFeatureFlagEvaluation)
	if err := module.Start(context.Background()); err != nil {
		t.Fatalf("Failed to start module: %v", err)
	}

	// Test that the fix works correctly
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)

	// Test the evaluateFeatureFlag method - should now use aggregator with fallback
	result := module.evaluateFeatureFlag("my-api", req)

	// Verify the fix: now uses aggregator instead of external evaluator directly
	if _, isAggregator := module.featureFlagEvaluator.(*FeatureFlagAggregator); !isAggregator {
		t.Errorf("Expected module to use aggregator after fix, got: %T", module.featureFlagEvaluator)
	}

	// The aggregator should now call external evaluator, get ErrNoDecision, then fallback to file evaluator
	t.Logf("evaluateFeatureFlag result: %v", result)
	t.Logf("featureFlagEvaluatorProvided: %v", module.featureFlagEvaluatorProvided)
	t.Logf("featureFlagEvaluator type: %T", module.featureFlagEvaluator)

	// Verify the fix: should return file evaluator result (false) instead of hard-coded default (true)
	var fileEvaluator FeatureFlagEvaluator
	if err := module.app.GetService("featureFlagEvaluator.file", &fileEvaluator); err == nil {
		fileResult, fileErr := fileEvaluator.EvaluateFlag(context.Background(), "my-api", "", req)
		t.Logf("File evaluator returns: %v, error: %v", fileResult, fileErr)
		if fileErr == nil && fileResult == result {
			t.Logf("SUCCESS: External evaluator fallback now correctly returns file evaluator result (%v)", fileResult)
		} else {
			t.Errorf("FAIL: Expected aggregator result (%v) to match file evaluator result (%v)", result, fileResult)
		}
	} else {
		t.Logf("File evaluator not registered: %v", err)
	}

	// Verify that external evaluator was registered and discoverable
	var registeredExternalEvaluator FeatureFlagEvaluator
	if err := module.app.GetService("featureFlagEvaluator.external", &registeredExternalEvaluator); err == nil {
		t.Logf("SUCCESS: External evaluator registered and discoverable by aggregator")

		// Test that external evaluator still returns ErrNoDecision when called directly
		_, extErr := registeredExternalEvaluator.EvaluateFlag(context.Background(), "my-api", "", req)
		if extErr == ErrNoDecision {
			t.Logf("SUCCESS: External evaluator correctly returns ErrNoDecision")
		} else {
			t.Errorf("Expected external evaluator to return ErrNoDecision, got: %v", extErr)
		}
	} else {
		t.Errorf("FAIL: External evaluator not registered for discovery: %v", err)
	}
}
