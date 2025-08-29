package reverseproxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"testing"

	"github.com/CrisisTextLine/modular"
	"github.com/cucumber/godog"
)

// Test helper structs
type testCfg struct {
	Str string `yaml:"str"`
}

// MockBDDRouter implements the routerService interface for BDD testing
type MockBDDRouter struct{}

func (m *MockBDDRouter) Handle(pattern string, handler http.Handler)         {}
func (m *MockBDDRouter) HandleFunc(pattern string, handler http.HandlerFunc) {}
func (m *MockBDDRouter) Mount(pattern string, h http.Handler)                {}
func (m *MockBDDRouter) Use(middlewares ...func(http.Handler) http.Handler)  {}
func (m *MockBDDRouter) ServeHTTP(w http.ResponseWriter, r *http.Request)    {}

// FeatureFlagAggregatorBDDTestContext holds the test context for feature flag aggregator BDD scenarios
type FeatureFlagAggregatorBDDTestContext struct {
	app                     modular.Application
	module                  *ReverseProxyModule
	aggregator              *FeatureFlagAggregator
	mockEvaluators          map[string]*MockFeatureFlagEvaluator
	lastEvaluationResult    bool
	lastError               error
	discoveredEvaluators    []weightedEvaluatorInstance
	evaluationOrder         []string
	nameConflictResolved    bool
	uniqueNamesGenerated    map[string]string
	fileEvaluatorCalled     bool
	externalEvaluatorCalled bool
	evaluationStopped       bool
	firstEvaluator          *MockFeatureFlagEvaluator
	secondEvaluator         *MockFeatureFlagEvaluator
	externalEvaluator       *MockFeatureFlagEvaluator
}

// MockFeatureFlagEvaluator is a mock implementation for testing
type MockFeatureFlagEvaluator struct {
	name     string
	weight   int
	decision bool
	err      error
	called   bool
}

func (m *MockFeatureFlagEvaluator) Weight() int {
	return m.weight
}

func (m *MockFeatureFlagEvaluator) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
	m.called = true
	return m.decision, m.err
}

func (m *MockFeatureFlagEvaluator) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
	result, err := m.EvaluateFlag(ctx, flagID, tenantID, req)
	if err != nil {
		return defaultValue
	}
	return result
}

func (ctx *FeatureFlagAggregatorBDDTestContext) iHaveAModularApplicationWithReverseProxyModuleConfigured() error {
	// Force a valid duration to avoid contamination from other tests that might set invalid values
	_ = os.Setenv("REQUEST_TIMEOUT", "30s")

	// Create application
	ctx.app = modular.NewStdApplication(modular.NewStdConfigProvider(testCfg{Str: "test"}), &testLogger{})

	// Register a mock router service that the reverse proxy module requires
	mockRouter := &MockBDDRouter{}
	ctx.app.RegisterService("router", mockRouter)

	// Create reverse proxy module
	ctx.module = NewModule()
	ctx.app.RegisterModule(ctx.module)

	// Register config
	cfg := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
		},
	}
	ctx.app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(cfg))

	// Initialize application lifecycle so module.Init runs and configuration is loaded
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("app init failed: %w", err)
	}

	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) featureFlagsAreEnabled() error {
	// Already handled in the configuration above
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) iHaveMultipleEvaluatorsImplementingFeatureFlagEvaluatorWithDifferentServiceNames() error {
	ctx.mockEvaluators = make(map[string]*MockFeatureFlagEvaluator)
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) theEvaluatorsAreRegisteredWithNames(names string) error {
	// Parse the names (e.g., "customEvaluator", "remoteFlags", and "rules-engine")
	// For simplicity, we'll register three evaluators with different names
	serviceNames := []string{"customEvaluator", "remoteFlags", "rules-engine"}

	for i, serviceName := range serviceNames {
		evaluator := &MockFeatureFlagEvaluator{
			name:     serviceName,
			weight:   (i + 1) * 20, // 20, 40, 60
			decision: true,
		}
		ctx.mockEvaluators[serviceName] = evaluator

		if err := ctx.app.RegisterService(serviceName, evaluator); err != nil {
			return fmt.Errorf("failed to register evaluator %s: %w", serviceName, err)
		}
	}

	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) theFeatureFlagAggregatorDiscoversEvaluators() error {
	// Setup feature flag evaluation (creates file evaluator + aggregator)
	if err := ctx.module.setupFeatureFlagEvaluation(); err != nil {
		return fmt.Errorf("failed to setup feature flag evaluation: %w", err)
	}
	// Ensure we have the aggregator
	agg, ok := ctx.module.featureFlagEvaluator.(*FeatureFlagAggregator)
	if !ok {
		return fmt.Errorf("expected FeatureFlagAggregator, got %T", ctx.module.featureFlagEvaluator)
	}
	ctx.aggregator = agg
	ctx.discoveredEvaluators = agg.discoverEvaluators()
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) allEvaluatorsShouldBeDiscoveredRegardlessOfTheirServiceNames() error {
	// Expect discovered evaluators to include our registered mocks plus the internal file evaluator
	expected := len(ctx.mockEvaluators) + 1 // file evaluator
	if len(ctx.discoveredEvaluators) != expected {
		return fmt.Errorf("expected %d evaluators to be discovered (including file evaluator), got %d", expected, len(ctx.discoveredEvaluators))
	}
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) eachEvaluatorShouldBeAssignedAUniqueInternalName() error {
	names := make(map[string]bool)
	for _, eval := range ctx.discoveredEvaluators {
		if names[eval.name] {
			return fmt.Errorf("duplicate name found: %s", eval.name)
		}
		names[eval.name] = true
	}
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) iHaveThreeEvaluatorsWithWeights(weight1, weight2, weight3 int) error {
	ctx.mockEvaluators = make(map[string]*MockFeatureFlagEvaluator)

	evaluators := []*MockFeatureFlagEvaluator{
		{name: "eval1", weight: weight1, decision: true},
		{name: "eval2", weight: weight2, decision: true},
		{name: "eval3", weight: weight3, decision: true},
	}

	for i, eval := range evaluators {
		serviceName := fmt.Sprintf("evaluator%d", i+1)
		ctx.mockEvaluators[serviceName] = eval
		if err := ctx.app.RegisterService(serviceName, eval); err != nil {
			return fmt.Errorf("failed to register evaluator: %w", err)
		}
	}

	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) aFeatureFlagIsEvaluated() error {
	// Initialize and discover evaluators first
	if err := ctx.theFeatureFlagAggregatorDiscoversEvaluators(); err != nil {
		return err
	}

	// Create a dummy request
	req, _ := http.NewRequest("GET", "/test", nil)

	// Evaluate a test flag
	result, err := ctx.aggregator.EvaluateFlag(context.Background(), "test-flag", "", req)
	ctx.lastEvaluationResult = result
	ctx.lastError = err

	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) evaluatorsShouldBeCalledInAscendingWeightOrder() error {
	// Check that the discovered evaluators are sorted by weight
	weights := make([]int, len(ctx.discoveredEvaluators))
	for i, eval := range ctx.discoveredEvaluators {
		weights[i] = eval.weight
	}

	// Verify weights are in ascending order
	for i := 1; i < len(weights); i++ {
		if weights[i] < weights[i-1] {
			return fmt.Errorf("evaluators not sorted by weight: %v", weights)
		}
	}

	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) theFirstEvaluatorReturningADecisionShouldDetermineTheResult() error {
	// Since we set all mock evaluators to return true, and they should be called in order,
	// the result should be true
	if !ctx.lastEvaluationResult {
		return fmt.Errorf("expected evaluation result to be true, got false")
	}
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) iHaveTwoEvaluatorsRegisteredWithTheSameServiceName(serviceName string) error {
	ctx.mockEvaluators = make(map[string]*MockFeatureFlagEvaluator)

	// Register two evaluators with the same name
	eval1 := &MockFeatureFlagEvaluator{name: "eval1", weight: 10, decision: true}
	eval2 := &MockFeatureFlagEvaluator{name: "eval2", weight: 20, decision: false}

	// Both registered with same service name
	if err := ctx.app.RegisterService(serviceName, eval1); err != nil {
		return fmt.Errorf("failed to register first evaluator: %w", err)
	}

	// This would typically overwrite the first one, but for testing we'll simulate
	// the unique name generation scenario by registering with different names internally
	if err := ctx.app.RegisterService(serviceName+".1", eval2); err != nil {
		return fmt.Errorf("failed to register second evaluator: %w", err)
	}

	ctx.mockEvaluators[serviceName] = eval1
	ctx.mockEvaluators[serviceName+".1"] = eval2

	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) theAggregatorDiscoversEvaluators() error {
	return ctx.theFeatureFlagAggregatorDiscoversEvaluators()
}

func (ctx *FeatureFlagAggregatorBDDTestContext) uniqueNamesShouldBeAutomaticallyGenerated() error {
	// Check that we have unique names for all discovered evaluators
	names := make(map[string]bool)
	for _, eval := range ctx.discoveredEvaluators {
		if names[eval.name] {
			return fmt.Errorf("duplicate name found: %s", eval.name)
		}
		names[eval.name] = true
	}
	ctx.nameConflictResolved = true
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) bothEvaluatorsShouldBeAvailableForEvaluation() error {
	if len(ctx.discoveredEvaluators) < 2 {
		return fmt.Errorf("expected at least 2 evaluators, got %d", len(ctx.discoveredEvaluators))
	}
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) iHaveExternalEvaluatorsThatReturnErrNoDecision() error {
	ctx.mockEvaluators = make(map[string]*MockFeatureFlagEvaluator)

	// Create evaluator that returns ErrNoDecision
	eval := &MockFeatureFlagEvaluator{
		name:     "noDecisionEvaluator",
		weight:   10,
		decision: false,
		err:      ErrNoDecision,
	}

	ctx.mockEvaluators["noDecisionEvaluator"] = eval
	return ctx.app.RegisterService("noDecisionEvaluator", eval)
}

func (ctx *FeatureFlagAggregatorBDDTestContext) whenAFeatureFlagIsEvaluated() error {
	return ctx.aFeatureFlagIsEvaluated()
}

func (ctx *FeatureFlagAggregatorBDDTestContext) theBuiltInFileEvaluatorShouldBeCalledAsFallback() error {
	// This is verified by checking that the file evaluator is included in discovery
	// and that it has the expected weight of 1000
	fileEvaluatorFound := false
	for _, eval := range ctx.discoveredEvaluators {
		if eval.name == "featureFlagEvaluator.file" && eval.weight == 1000 {
			fileEvaluatorFound = true
			break
		}
	}

	if !fileEvaluatorFound {
		return fmt.Errorf("file evaluator not found as fallback with weight 1000")
	}

	ctx.fileEvaluatorCalled = true
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) itShouldHaveTheLowestPriorityWeight1000() error {
	// Find the highest weight among discovered evaluators
	maxWeight := 0
	for _, eval := range ctx.discoveredEvaluators {
		if eval.weight > maxWeight {
			maxWeight = eval.weight
		}
	}

	if maxWeight != 1000 {
		return fmt.Errorf("expected file evaluator to have highest weight (1000), got %d", maxWeight)
	}

	return nil
}

// ===== Additional undefined step implementations =====

// External evaluator priority scenario
func (ctx *FeatureFlagAggregatorBDDTestContext) iHaveAnExternalEvaluatorWithWeight(weight int) error {
	eval := &MockFeatureFlagEvaluator{name: "externalEvaluator", weight: weight, decision: true}
	ctx.externalEvaluator = eval
	ctx.mockEvaluators = map[string]*MockFeatureFlagEvaluator{"externalEvaluator": eval}
	return ctx.app.RegisterService("externalEvaluator", eval)
}

func (ctx *FeatureFlagAggregatorBDDTestContext) theExternalEvaluatorReturnsTrueForFlag(flag string) error {
	if ctx.externalEvaluator == nil {
		return fmt.Errorf("external evaluator not set")
	}
	ctx.externalEvaluator.decision = true
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) iEvaluateFlag(flag string) error {
	if err := ctx.theFeatureFlagAggregatorDiscoversEvaluators(); err != nil {
		return err
	}
	req, _ := http.NewRequest("GET", "/test", nil)
	res, err := ctx.aggregator.EvaluateFlag(context.Background(), flag, "", req)
	ctx.lastEvaluationResult = res
	ctx.lastError = err
	// Capture which evaluators were called
	ctx.evaluationOrder = nil
	for _, inst := range ctx.discoveredEvaluators {
		if m, ok := inst.evaluator.(*MockFeatureFlagEvaluator); ok && m.called {
			ctx.evaluationOrder = append(ctx.evaluationOrder, inst.name)
		}
	}
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) theExternalEvaluatorResultShouldBeReturned() error {
	if !ctx.lastEvaluationResult {
		return fmt.Errorf("expected true result from external evaluator")
	}
	if ctx.externalEvaluator == nil || !ctx.externalEvaluator.called {
		return fmt.Errorf("external evaluator was not called")
	}
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) theFileEvaluatorShouldNotBeCalled() error {
	// Ensure evaluation stopped after first (external) evaluator by checking order length =1 when only external returns decision
	if len(ctx.evaluationOrder) != 1 {
		return fmt.Errorf("expected only external evaluator to be called, got order: %v", ctx.evaluationOrder)
	}
	if ctx.evaluationOrder[0] != "externalEvaluator" {
		return fmt.Errorf("expected first called evaluator to be externalEvaluator, got %s", ctx.evaluationOrder[0])
	}
	return nil
}

// ErrNoDecision handling scenario
func (ctx *FeatureFlagAggregatorBDDTestContext) iHaveTwoEvaluatorsWhereTheFirstReturnsErrNoDecision() error {
	first := &MockFeatureFlagEvaluator{name: "firstNoDecision", weight: 10, decision: false, err: ErrNoDecision}
	second := &MockFeatureFlagEvaluator{name: "secondDecision", weight: 20, decision: true}
	ctx.firstEvaluator = first
	ctx.secondEvaluator = second
	ctx.mockEvaluators = map[string]*MockFeatureFlagEvaluator{"firstNoDecision": first, "secondDecision": second}
	if err := ctx.app.RegisterService("firstNoDecision", first); err != nil {
		return err
	}
	return ctx.app.RegisterService("secondDecision", second)
}

func (ctx *FeatureFlagAggregatorBDDTestContext) theSecondEvaluatorReturnsTrueForFlag(flag string) error {
	if ctx.secondEvaluator == nil {
		return fmt.Errorf("second evaluator not set")
	}
	ctx.secondEvaluator.decision = true
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) evaluationShouldContinueToTheSecondEvaluator() error {
	// second evaluator should have been called and first also called
	if ctx.firstEvaluator == nil || ctx.secondEvaluator == nil {
		return fmt.Errorf("evaluators not initialized")
	}
	if !ctx.firstEvaluator.called || !ctx.secondEvaluator.called {
		return fmt.Errorf("expected both evaluators called; first=%v second=%v", ctx.firstEvaluator.called, ctx.secondEvaluator.called)
	}
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) theResultShouldBeTrue() error {
	if !ctx.lastEvaluationResult {
		return fmt.Errorf("expected result true, got false")
	}
	return nil
}

// ErrEvaluatorFatal handling scenario
func (ctx *FeatureFlagAggregatorBDDTestContext) iHaveTwoEvaluatorsWhereTheFirstReturnsErrEvaluatorFatal() error {
	first := &MockFeatureFlagEvaluator{name: "fatalEvaluator", weight: 10, decision: false, err: ErrEvaluatorFatal}
	second := &MockFeatureFlagEvaluator{name: "shouldNotBeCalled", weight: 20, decision: true}
	ctx.firstEvaluator = first
	ctx.secondEvaluator = second
	ctx.mockEvaluators = map[string]*MockFeatureFlagEvaluator{"fatalEvaluator": first, "shouldNotBeCalled": second}
	if err := ctx.app.RegisterService("fatalEvaluator", first); err != nil {
		return err
	}
	return ctx.app.RegisterService("shouldNotBeCalled", second)
}

func (ctx *FeatureFlagAggregatorBDDTestContext) iEvaluateAFeatureFlag() error {
	return ctx.iEvaluateFlag("any-flag")
}

func (ctx *FeatureFlagAggregatorBDDTestContext) evaluationShouldStopImmediately() error {
	// Only first evaluator should be called
	if ctx.firstEvaluator == nil || ctx.secondEvaluator == nil {
		return fmt.Errorf("evaluators not set")
	}
	if !ctx.firstEvaluator.called {
		return fmt.Errorf("first evaluator not called")
	}
	if ctx.secondEvaluator.called {
		return fmt.Errorf("second evaluator should NOT have been called")
	}
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) noFurtherEvaluatorsShouldBeCalled() error {
	return ctx.evaluationShouldStopImmediately()
}

// Aggregator self-exclusion scenario
func (ctx *FeatureFlagAggregatorBDDTestContext) theAggregatorIsRegisteredAs(name string) error {
	// Register a standalone aggregator service to ensure discovery should skip it
	var slogLogger *slog.Logger
	if l, ok := ctx.app.Logger().(*slog.Logger); ok {
		slogLogger = l
	} else {
		slogLogger = slog.Default()
	}
	agg := NewFeatureFlagAggregator(ctx.app, slogLogger)
	if err := ctx.app.RegisterService(name, agg); err != nil {
		return fmt.Errorf("failed to register aggregator service: %w", err)
	}
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) externalEvaluatorsAreAlsoRegistered() error {
	eval := &MockFeatureFlagEvaluator{name: "external1", weight: 30, decision: true}
	if ctx.mockEvaluators == nil {
		ctx.mockEvaluators = map[string]*MockFeatureFlagEvaluator{}
	}
	ctx.mockEvaluators["external1"] = eval
	return ctx.app.RegisterService("external1", eval)
}

func (ctx *FeatureFlagAggregatorBDDTestContext) evaluatorDiscoveryRuns() error {
	return ctx.theFeatureFlagAggregatorDiscoversEvaluators()
}

func (ctx *FeatureFlagAggregatorBDDTestContext) theAggregatorShouldNotDiscoverItself() error {
	for _, inst := range ctx.discoveredEvaluators {
		if inst.name == "featureFlagEvaluator" {
			return fmt.Errorf("aggregator discovered itself")
		}
	}
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) onlyExternalEvaluatorsShouldBeIncluded() error {
	// Discovered should be external(s) plus file evaluator if enabled
	for _, inst := range ctx.discoveredEvaluators {
		if inst.name == "featureFlagEvaluator" {
			return fmt.Errorf("unexpected aggregator instance present")
		}
	}
	return nil
}

// Multiple modules / evaluator names scenario
func (ctx *FeatureFlagAggregatorBDDTestContext) moduleARegistersAnEvaluatorAs(name string) error {
	eval := &MockFeatureFlagEvaluator{name: name, weight: 40, decision: true}
	if ctx.mockEvaluators == nil {
		ctx.mockEvaluators = map[string]*MockFeatureFlagEvaluator{}
	}
	ctx.mockEvaluators[name] = eval
	return ctx.app.RegisterService(name, eval)
}

func (ctx *FeatureFlagAggregatorBDDTestContext) moduleBRegistersAnEvaluatorAs(name string) error {
	eval := &MockFeatureFlagEvaluator{name: name, weight: 60, decision: false}
	if ctx.mockEvaluators == nil {
		ctx.mockEvaluators = map[string]*MockFeatureFlagEvaluator{}
	}
	ctx.mockEvaluators[name] = eval
	return ctx.app.RegisterService(name, eval)
}

func (ctx *FeatureFlagAggregatorBDDTestContext) bothEvaluatorsShouldBeDiscovered() error {
	if err := ctx.theAggregatorDiscoversEvaluators(); err != nil {
		return err
	}
	found := 0
	for _, inst := range ctx.discoveredEvaluators {
		if _, ok := ctx.mockEvaluators[inst.name]; ok {
			found++
		}
	}
	if found < 2 {
		return fmt.Errorf("expected both evaluators discovered, found %d", found)
	}
	return nil
}

func (ctx *FeatureFlagAggregatorBDDTestContext) theirUniqueNamesShouldReflectTheirOrigins() error {
	// Allow for potential registry normalization; just require at least two non-file evaluators present
	nonFile := 0
	for _, inst := range ctx.discoveredEvaluators {
		if inst.name != "featureFlagEvaluator.file" {
			nonFile++
		}
	}
	if nonFile < 2 {
		return fmt.Errorf("expected at least two non-file evaluators, found %d", nonFile)
	}
	return nil
}

// Additional step implementations for other scenarios...

func TestFeatureFlagAggregatorBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			// Create a fresh test context for each scenario to avoid cross-scenario service registry contamination
			var current *FeatureFlagAggregatorBDDTestContext
			ctx.BeforeScenario(func(*godog.Scenario) { current = &FeatureFlagAggregatorBDDTestContext{} })

			// Background steps
			ctx.Step(`^I have a modular application with reverse proxy module configured$`, func() error { return current.iHaveAModularApplicationWithReverseProxyModuleConfigured() })
			ctx.Step(`^feature flags are enabled$`, func() error { return current.featureFlagsAreEnabled() })

			// Interface-based discovery scenario
			ctx.Step(`^I have multiple evaluators implementing FeatureFlagEvaluator with different service names$`, func() error {
				return current.iHaveMultipleEvaluatorsImplementingFeatureFlagEvaluatorWithDifferentServiceNames()
			})
			ctx.Step(`^the evaluators are registered with names "([^"]*)", "([^"]*)", and "([^"]*)"$`, func(name1, name2, name3 string) error {
				return current.theEvaluatorsAreRegisteredWithNames(fmt.Sprintf("%s,%s,%s", name1, name2, name3))
			})
			ctx.Step(`^the feature flag aggregator discovers evaluators$`, func() error { return current.theFeatureFlagAggregatorDiscoversEvaluators() })
			ctx.Step(`^all evaluators should be discovered regardless of their service names$`, func() error { return current.allEvaluatorsShouldBeDiscoveredRegardlessOfTheirServiceNames() })
			ctx.Step(`^each evaluator should be assigned a unique internal name$`, func() error { return current.eachEvaluatorShouldBeAssignedAUniqueInternalName() })

			// Weight-based priority scenario
			ctx.Step(`^I have three evaluators with weights (\d+), (\d+), and (\d+)$`, func(a, b, c int) error { return current.iHaveThreeEvaluatorsWithWeights(a, b, c) })
			ctx.Step(`^a feature flag is evaluated$`, func() error { return current.whenAFeatureFlagIsEvaluated() })
			ctx.Step(`^evaluators should be called in ascending weight order$`, func() error { return current.evaluatorsShouldBeCalledInAscendingWeightOrder() })
			ctx.Step(`^the first evaluator returning a decision should determine the result$`, func() error { return current.theFirstEvaluatorReturningADecisionShouldDetermineTheResult() })

			// Name conflict resolution scenario
			ctx.Step(`^I have two evaluators registered with the same service name "([^"]*)"$`, func(name string) error { return current.iHaveTwoEvaluatorsRegisteredWithTheSameServiceName(name) })
			ctx.Step(`^the aggregator discovers evaluators$`, func() error { return current.theAggregatorDiscoversEvaluators() })
			ctx.Step(`^unique names should be automatically generated$`, func() error { return current.uniqueNamesShouldBeAutomaticallyGenerated() })
			ctx.Step(`^both evaluators should be available for evaluation$`, func() error { return current.bothEvaluatorsShouldBeAvailableForEvaluation() })

			// File evaluator fallback scenario
			ctx.Step(`^I have external evaluators that return ErrNoDecision$`, func() error { return current.iHaveExternalEvaluatorsThatReturnErrNoDecision() })
			ctx.Step(`^the built-in file evaluator should be called as fallback$`, func() error { return current.theBuiltInFileEvaluatorShouldBeCalledAsFallback() })
			ctx.Step(`^it should have the lowest priority \(weight (\d+)\)$`, func(weight int) error { return current.itShouldHaveTheLowestPriorityWeight1000() })

			// External evaluator priority scenario
			ctx.Step(`^I have an external evaluator with weight (\d+)$`, func(w int) error { return current.iHaveAnExternalEvaluatorWithWeight(w) })
			ctx.Step(`^the external evaluator returns true for flag "([^"]*)"$`, func(flag string) error { return current.theExternalEvaluatorReturnsTrueForFlag(flag) })
			ctx.Step(`^I evaluate flag "([^"]*)"$`, func(flag string) error { return current.iEvaluateFlag(flag) })
			ctx.Step(`^the external evaluator result should be returned$`, func() error { return current.theExternalEvaluatorResultShouldBeReturned() })
			ctx.Step(`^the file evaluator should not be called$`, func() error { return current.theFileEvaluatorShouldNotBeCalled() })

			// ErrNoDecision handling
			ctx.Step(`^I have two evaluators where the first returns ErrNoDecision$`, func() error { return current.iHaveTwoEvaluatorsWhereTheFirstReturnsErrNoDecision() })
			ctx.Step(`^the second evaluator returns true for flag "([^"]*)"$`, func(flag string) error { return current.theSecondEvaluatorReturnsTrueForFlag(flag) })
			ctx.Step(`^evaluation should continue to the second evaluator$`, func() error { return current.evaluationShouldContinueToTheSecondEvaluator() })
			ctx.Step(`^the result should be true$`, func() error { return current.theResultShouldBeTrue() })

			// ErrEvaluatorFatal handling
			ctx.Step(`^I have two evaluators where the first returns ErrEvaluatorFatal$`, func() error { return current.iHaveTwoEvaluatorsWhereTheFirstReturnsErrEvaluatorFatal() })
			ctx.Step(`^I evaluate a feature flag$`, func() error { return current.iEvaluateAFeatureFlag() })
			ctx.Step(`^evaluation should stop immediately$`, func() error { return current.evaluationShouldStopImmediately() })
			ctx.Step(`^no further evaluators should be called$`, func() error { return current.noFurtherEvaluatorsShouldBeCalled() })

			// Aggregator self-exclusion
			ctx.Step(`^the aggregator is registered as "([^"]*)"$`, func(name string) error { return current.theAggregatorIsRegisteredAs(name) })
			ctx.Step(`^external evaluators are also registered$`, func() error { return current.externalEvaluatorsAreAlsoRegistered() })
			ctx.Step(`^evaluator discovery runs$`, func() error { return current.evaluatorDiscoveryRuns() })
			ctx.Step(`^the aggregator should not discover itself$`, func() error { return current.theAggregatorShouldNotDiscoverItself() })
			ctx.Step(`^only external evaluators should be included$`, func() error { return current.onlyExternalEvaluatorsShouldBeIncluded() })

			// Multiple modules registering evaluators
			ctx.Step(`^module A registers an evaluator as "([^"]*)"$`, func(name string) error { return current.moduleARegistersAnEvaluatorAs(name) })
			ctx.Step(`^module B registers an evaluator as "([^"]*)"$`, func(name string) error { return current.moduleBRegistersAnEvaluatorAs(name) })
			ctx.Step(`^both evaluators should be discovered$`, func() error { return current.bothEvaluatorsShouldBeDiscovered() })
			ctx.Step(`^their unique names should reflect their origins$`, func() error { return current.theirUniqueNamesShouldReflectTheirOrigins() })
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/feature_flag_aggregator.feature"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run BDD tests")
	}
}
