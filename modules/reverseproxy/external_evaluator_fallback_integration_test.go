package reverseproxy

import (
	"context"
	"net/http"
	"testing"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/mock"
)

// TestExternalEvaluatorFallbackIntegration tests the complete scenario described in the problem statement:
// 1. Configure reverseproxy with feature flags in YAML
// 2. Configure external evaluator (like LaunchDarkly) but with invalid/missing SDK key
// 3. External evaluator registers as FeatureFlagEvaluator service but returns ErrNoDecision
// 4. Verify requests route based on YAML flag values (file evaluator fallback)
func TestExternalEvaluatorFallbackIntegration(t *testing.T) {
	testCases := []struct {
		name           string
		yamlFlagValue  bool
		expectedResult bool
		description    string
	}{
		{
			name:           "YAML_true_should_fallback_to_true",
			yamlFlagValue:  true,
			expectedResult: true,
			description:    "When external evaluator returns ErrNoDecision and YAML has true, should route to primary backend",
		},
		{
			name:           "YAML_false_should_fallback_to_false",
			yamlFlagValue:  false,
			expectedResult: false,
			description:    "When external evaluator returns ErrNoDecision and YAML has false, should route to alternative backend",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock application
			moduleApp := NewMockTenantApplication()

			// Configure reverseproxy with feature flags in YAML (step 1)
			config := &ReverseProxyConfig{
				BackendServices: map[string]string{
					"primary":     "http://primary.example.com",
					"alternative": "http://alternative.example.com",
				},
				FeatureFlags: FeatureFlagsConfig{
					Enabled: true,
					Flags: map[string]bool{
						"my-api": tc.yamlFlagValue, // This should be respected when external evaluator fails
					},
				},
			}

			// Register the configuration
			moduleApp.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

			// Create the module
			module := NewModule()

			// Create a mock router and set up expected calls
			mockRouter := &MockRouter{}
			mockRouter.On("HandleFunc", mock.Anything, mock.Anything).Return()

			// Configure external evaluator (like LaunchDarkly) but with invalid/missing SDK key (step 2)
			// This simulates the real-world scenario where LaunchDarkly is configured but not working
			externalEvaluator := &mockExternalEvaluatorReturnsErrNoDecision{}

			// External evaluator registers as FeatureFlagEvaluator service (step 3)
			services := map[string]any{
				"router":               mockRouter,
				"featureFlagEvaluator": externalEvaluator,
			}

			constructedModule, err := module.Constructor()(moduleApp, services)
			if err != nil {
				t.Fatalf("Failed to construct module: %v", err)
			}
			module = constructedModule.(*ReverseProxyModule)

			// Set up the configuration and initialize
			err = module.RegisterConfig(moduleApp)
			if err != nil {
				t.Fatalf("Failed to register config: %v", err)
			}

			if err := module.Init(moduleApp); err != nil {
				t.Fatalf("Failed to initialize module: %v", err)
			}

			if err := module.Start(context.Background()); err != nil {
				t.Fatalf("Failed to start module: %v", err)
			}

			// Test the fixed behavior (step 4)
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/test", nil)

			// This should now properly fallback to YAML config instead of using hard-coded default
			result := module.evaluateFeatureFlag("my-api", req)

			// Verify the fix works correctly
			if result != tc.expectedResult {
				t.Errorf("%s: Expected %v, got %v", tc.description, tc.expectedResult, result)
			}

			// Verify the implementation details of the fix
			t.Run("Implementation_Verification", func(t *testing.T) {
				// 1. Verify aggregator is used instead of external evaluator directly
				if _, isAggregator := module.featureFlagEvaluator.(*FeatureFlagAggregator); !isAggregator {
					t.Errorf("Expected aggregator to be used, got: %T", module.featureFlagEvaluator)
				}

				// 2. Verify external evaluator was registered for discovery
				var registeredExternalEvaluator FeatureFlagEvaluator
				if err := module.app.GetService("featureFlagEvaluator.external", &registeredExternalEvaluator); err != nil {
					t.Errorf("Expected external evaluator to be registered for aggregation: %v", err)
				}

				// 3. Verify file evaluator is available as fallback
				var fileEvaluator FeatureFlagEvaluator
				if err := module.app.GetService("featureFlagEvaluator.file", &fileEvaluator); err != nil {
					t.Errorf("Expected file evaluator to be available as fallback: %v", err)
				} else {
					// File evaluator should return the YAML config value
					fileResult, fileErr := fileEvaluator.EvaluateFlag(context.Background(), "my-api", "", req)
					if fileErr != nil {
						t.Errorf("File evaluator should handle flag from YAML: %v", fileErr)
					}
					if fileResult != tc.expectedResult {
						t.Errorf("File evaluator should return YAML value %v, got %v", tc.expectedResult, fileResult)
					}
				}

				// 4. Verify external evaluator returns ErrNoDecision (simulating misconfigured LaunchDarkly)
				if registeredExternalEvaluator != nil {
					_, extErr := registeredExternalEvaluator.EvaluateFlag(context.Background(), "my-api", "", req)
					if extErr != ErrNoDecision {
						t.Errorf("Expected external evaluator to return ErrNoDecision, got: %v", extErr)
					}
				}
			})

			t.Logf("✅ %s: External evaluator returned ErrNoDecision, successfully fell back to YAML config (%v)", tc.name, tc.expectedResult)
		})
	}
}
