package reverseproxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/GoCodeAlone/modular"
)

// Feature Flag Scenario Step Implementations

// Mock evaluator for testing aggregator discovery
type testFeatureFlagEvaluator struct {
	name      string
	weight    int
	flags     map[string]bool
	callCount int
	returnErr error
}

func (e *testFeatureFlagEvaluator) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
	e.callCount++
	if e.returnErr != nil {
		return false, e.returnErr
	}
	if val, exists := e.flags[flagID]; exists {
		return val, nil
	}
	return false, ErrFeatureFlagNotFound
}

func (e *testFeatureFlagEvaluator) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
	result, err := e.EvaluateFlag(ctx, flagID, tenantID, req)
	if err != nil {
		return defaultValue
	}
	return result
}

func (e *testFeatureFlagEvaluator) Weight() int {
	return e.weight
}

// Step 1: I evaluate a feature flag
func (ctx *ReverseProxyBDDTestContext) iEvaluateAFeatureFlag() error {
	// Set up a proper test environment with feature flag evaluator
	ctx.resetContext()

	// Create test backend servers for feature flag testing
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("primary backend response"))
	}))
	ctx.testServers = append(ctx.testServers, primaryServer)

	altServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("alternative backend response"))
	}))
	ctx.testServers = append(ctx.testServers, altServer)

	// Create configuration with feature flags
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"primary-backend": primaryServer.URL,
			"alt-backend":     altServer.URL,
		},
		Routes: map[string]string{
			"/api/test": "primary-backend",
		},
		RouteConfigs: map[string]RouteConfig{
			"/api/test": {
				FeatureFlagID:      "test-feature-enabled",
				AlternativeBackend: "alt-backend",
			},
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"primary-backend": {URL: primaryServer.URL},
			"alt-backend":     {URL: altServer.URL},
		},
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"test-feature-enabled": true, // Feature enabled by default
			},
		},
	}

	// Setup application with feature flag evaluator
	if err := ctx.setupApplicationWithConfig(); err != nil {
		return fmt.Errorf("failed to setup application: %w", err)
	}

	// Create a test evaluator
	testEvaluator := &testFeatureFlagEvaluator{
		name:   "testEvaluator",
		weight: 100,
		flags:  map[string]bool{"test-feature-enabled": true},
	}

	// Register the evaluator with the application
	if ctx.app != nil {
		ctx.app.RegisterService("testEvaluator", testEvaluator)
	}

	// Create feature flag service for evaluation
	if ctx.service != nil && ctx.app != nil {
		aggregator := NewFeatureFlagAggregator(ctx.app, slog.Default())
		ctx.featureFlagService = &FileBasedFeatureFlagEvaluator{
			app:    ctx.app,
			logger: slog.Default(),
		}

		// Set up aggregator as the main evaluator
		ctx.app.RegisterService("featureFlagAggregator", aggregator)
	}

	// Evaluate a test feature flag
	if ctx.featureFlagService != nil {
		result, err := ctx.featureFlagService.EvaluateFlag(
			context.Background(),
			"test-feature-enabled",
			"",
			nil,
		)

		// Store the result and error for later verification
		ctx.lastError = err
		if err == nil {
			// Store result in context for verification
			if ctx.config.FeatureFlags.Flags == nil {
				ctx.config.FeatureFlags.Flags = make(map[string]bool)
			}
			ctx.config.FeatureFlags.Flags["last-evaluated"] = result
		}
	}

	return nil
}

// Step 2: the aggregator discovers evaluators
func (ctx *ReverseProxyBDDTestContext) theAggregatorDiscoversEvaluators() error {
	// Set up test context with multiple evaluators
	ctx.resetContext()

	// Create a test app with service registry
	testApp := NewMockTenantApplication()
	ctx.app = testApp

	// Register the tenant service so feature flag evaluators can access it
	tenantService := &MockTenantService{
		Configs: make(map[modular.TenantID]map[string]modular.ConfigProvider),
	}
	testApp.RegisterService("tenantService", tenantService)

	// Create and register multiple test evaluators with different service names
	eval1 := &testFeatureFlagEvaluator{
		name:   "customEvaluator",
		weight: 50,
		flags:  map[string]bool{"test-flag": true},
	}

	eval2 := &testFeatureFlagEvaluator{
		name:   "remoteFlags",
		weight: 75,
		flags:  map[string]bool{"test-flag": false},
	}

	eval3 := &testFeatureFlagEvaluator{
		name:   "rules-engine",
		weight: 25,
		flags:  map[string]bool{"test-flag": true},
	}

	// Register evaluators with the application
	testApp.RegisterService("customEvaluator", eval1)
	testApp.RegisterService("remoteFlags", eval2)
	testApp.RegisterService("rules-engine", eval3)

	// Create aggregator and test discovery
	logger := slog.Default()
	aggregator := NewFeatureFlagAggregator(testApp, logger)
	evaluators := aggregator.discoverEvaluators()

	// Store for later verification
	ctx.featureFlagService = &FileBasedFeatureFlagEvaluator{
		app:    testApp,
		logger: logger,
	}

	// Verify discovery worked
	if len(evaluators) < 3 {
		return fmt.Errorf("expected at least 3 evaluators discovered, got %d", len(evaluators))
	}

	// Store aggregator in context for further testing
	testApp.RegisterService("featureFlagAggregator", aggregator)

	return nil
}

// Note: Steps 3-6 are already implemented in other BDD test files:
// - alternativeBackendsShouldBeUsedWhenFlagsAreDisabled is in bdd_feature_flags_test.go
// - alternativeSingleBackendsShouldBeUsedWhenDisabled is in bdd_feature_flags_test.go
// - tenantSpecificRoutingShouldBeApplied is in bdd_feature_flags_test.go
// - comparisonResultsShouldBeLoggedWithFlagContext is in bdd_dryrun_comparison_test.go
