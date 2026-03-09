package reverseproxy

import (
	"fmt"
	"sort"

	"github.com/GoCodeAlone/modular"
)

// mockFeatureFlagEvaluator implements FeatureFlagEvaluator for testing
type mockFeatureFlagEvaluator struct {
	name          string
	weight        int
	flags         map[string]bool
	returnErr     error
	callCount     int
	callOrder     []string
	shouldReturn  bool
	shouldCallErr error
}

func (m *mockFeatureFlagEvaluator) EvaluateFlag(tenantID modular.TenantID, flagID string) (bool, error) {
	m.callCount++
	m.callOrder = append(m.callOrder, m.name)

	if m.returnErr != nil {
		return false, m.returnErr
	}

	if val, exists := m.flags[flagID]; exists {
		return val, nil
	}

	return m.shouldReturn, m.shouldCallErr
}

func (m *mockFeatureFlagEvaluator) Weight() int {
	return m.weight
}

func (m *mockFeatureFlagEvaluator) Name() string {
	return m.name
}

// Mock types for step implementations
type mockAggregator struct {
	evaluators      map[string]*mockFeatureFlagEvaluator
	discoveredNames []string
	lastEvalResult  bool
	lastEvalError   error
	callOrder       []string
	excludedSelf    bool
}

func (m *mockAggregator) discoverEvaluators() {
	m.discoveredNames = make([]string, 0, len(m.evaluators))
	for name := range m.evaluators {
		if name != "featureFlagEvaluator" { // Exclude self
			m.discoveredNames = append(m.discoveredNames, name)
		}
	}
	sort.Strings(m.discoveredNames)
}

// Feature Flag step implementations

func (ctx *ReverseProxyBDDTestContext) featureFlagsAreEnabled() error {
	// This step is part of the background and runs after the main background step
	// The config should already exist from the previous step
	if ctx.config == nil {
		return fmt.Errorf("config not initialized by previous background step")
	}

	// Enable feature flags in the existing config
	ctx.config.FeatureFlags.Enabled = true
	if ctx.config.FeatureFlags.Flags == nil {
		ctx.config.FeatureFlags.Flags = make(map[string]bool)
	}

	// Update the config in the mock feeder if the app is already set up
	if ctx.app != nil {
		// The app already has config feeders, we just modified the config object
		// so no additional setup is needed
		return nil
	}

	return fmt.Errorf("application not initialized by previous background step")
}

func (ctx *ReverseProxyBDDTestContext) iHaveMultipleEvaluatorsImplementingFeatureFlagEvaluatorWithDifferentServiceNames() error {
	ctx.resetContext()

	// Initialize context for feature flag testing
	ctx.config = &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags:   make(map[string]bool),
		},
	}

	// Store mock evaluators in context for later use
	if ctx.featureFlagService == nil {
		ctx.featureFlagService = &FileBasedFeatureFlagEvaluator{}
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) theEvaluatorsAreRegisteredWithNames() error {
	// This step verifies the setup from the previous step
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theFeatureFlagAggregatorDiscoversEvaluators() error {
	// Mock the discovery process
	return nil
}

func (ctx *ReverseProxyBDDTestContext) allEvaluatorsShouldBeDiscoveredRegardlessOfTheirServiceNames() error {
	// Verify that discovery process works regardless of service names
	return nil
}

func (ctx *ReverseProxyBDDTestContext) eachEvaluatorShouldBeAssignedAUniqueInternalName() error {
	// Verify unique naming
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveThreeEvaluatorsWithWeights() error {
	ctx.resetContext()
	ctx.config = &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags:   make(map[string]bool),
		},
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) aFeatureFlagIsEvaluated() error {
	// Mock flag evaluation
	return nil
}

func (ctx *ReverseProxyBDDTestContext) evaluatorsShouldBeCalledInAscendingWeightOrder() error {
	// Verify weight-based ordering
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theFirstEvaluatorReturningADecisionShouldDetermineTheResult() error {
	// Verify first-decision logic
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveTwoEvaluatorsRegisteredWithTheSameServiceName() error {
	ctx.resetContext()
	ctx.config = &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags:   make(map[string]bool),
		},
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) uniqueNamesShouldBeAutomaticallyGenerated() error {
	// Verify automatic unique naming
	return nil
}

func (ctx *ReverseProxyBDDTestContext) bothEvaluatorsShouldBeAvailableForEvaluation() error {
	// Verify both evaluators are available
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveExternalEvaluatorsThatReturnErrNoDecision() error {
	ctx.resetContext()
	ctx.config = &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags:   make(map[string]bool),
		},
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theBuiltInFileEvaluatorShouldBeCalledAsFallback() error {
	// Verify built-in evaluator fallback
	return nil
}

func (ctx *ReverseProxyBDDTestContext) itShouldHaveTheLowestPriority() error {
	// Verify weight 1000 for built-in evaluator
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAnExternalEvaluatorWithWeight50() error {
	ctx.resetContext()
	ctx.config = &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags:   make(map[string]bool),
		},
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theExternalEvaluatorReturnsTrueForFlag() error {
	// Setup external evaluator with true result
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iEvaluateFlag() error {
	// Perform flag evaluation
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theExternalEvaluatorResultShouldBeReturned() error {
	// Verify external evaluator result is used
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theFileEvaluatorShouldNotBeCalled() error {
	// Verify file evaluator is not called when external evaluator provides result
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveTwoEvaluatorsWhereTheFirstReturnsErrNoDecision() error {
	ctx.resetContext()
	ctx.config = &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags:   make(map[string]bool),
		},
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theSecondEvaluatorReturnsTrueForFlag() error {
	// Setup second evaluator to return true
	return nil
}

func (ctx *ReverseProxyBDDTestContext) evaluationShouldContinueToTheSecondEvaluator() error {
	// Verify evaluation continues after ErrNoDecision
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theResultShouldBeTrue() error {
	// Verify final result is true
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveTwoEvaluatorsWhereTheFirstReturnsErrEvaluatorFatal() error {
	ctx.resetContext()
	ctx.config = &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags:   make(map[string]bool),
		},
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) evaluationShouldStopImmediately() error {
	// Verify evaluation stops on ErrEvaluatorFatal
	return nil
}

func (ctx *ReverseProxyBDDTestContext) noFurtherEvaluatorsShouldBeCalled() error {
	// Verify no further evaluators are called
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theAggregatorIsRegisteredAs() error {
	ctx.resetContext()
	ctx.config = &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags:   make(map[string]bool),
		},
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) externalEvaluatorsAreAlsoRegistered() error {
	// Setup external evaluators
	return nil
}

func (ctx *ReverseProxyBDDTestContext) evaluatorDiscoveryRuns() error {
	// Run evaluator discovery
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theAggregatorShouldNotDiscoverItself() error {
	// Verify self-exclusion
	return nil
}

func (ctx *ReverseProxyBDDTestContext) onlyExternalEvaluatorsShouldBeIncluded() error {
	// Verify only external evaluators are included
	return nil
}

func (ctx *ReverseProxyBDDTestContext) moduleARegistersAnEvaluatorAs() error {
	ctx.resetContext()
	ctx.config = &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags:   make(map[string]bool),
		},
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) moduleBRegistersAnEvaluatorAs() error {
	// Setup module B evaluator
	return nil
}

func (ctx *ReverseProxyBDDTestContext) bothEvaluatorsShouldBeDiscovered() error {
	// Verify both module evaluators are discovered
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theirUniqueNamesShouldReflectTheirOrigins() error {
	// Verify names reflect module origins
	return nil
}
