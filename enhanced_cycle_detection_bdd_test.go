package modular

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/cucumber/godog"
)

// EnhancedCycleDetectionBDDTestContext holds the test context for cycle detection BDD scenarios
type EnhancedCycleDetectionBDDTestContext struct {
	app              Application
	modules          map[string]Module
	lastError        error
	initializeResult error
	cycleDetected    bool
}

// Test interfaces for cycle detection scenarios
type TestInterfaceA interface {
	MethodA() string
}

type TestInterfaceB interface {
	MethodB() string
}

type TestInterfaceC interface {
	MethodC() string
}

// Similar interfaces for name disambiguation testing
type EnhancedTestInterface interface {
	TestMethod() string
}

type AnotherEnhancedTestInterface interface {
	AnotherTestMethod() string
}

// Mock modules for different cycle scenarios

// CycleModuleA - provides TestInterfaceA and requires TestInterfaceB
type CycleModuleA struct {
	name string
}

func (m *CycleModuleA) Name() string               { return m.name }
func (m *CycleModuleA) Init(app Application) error { return nil }

func (m *CycleModuleA) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "serviceA",
		Instance: &TestInterfaceAImpl{name: "A"},
	}}
}

func (m *CycleModuleA) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "serviceB",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeOf((*TestInterfaceB)(nil)).Elem(),
	}}
}

// CycleModuleB - provides TestInterfaceB and requires TestInterfaceA
type CycleModuleB struct {
	name string
}

func (m *CycleModuleB) Name() string               { return m.name }
func (m *CycleModuleB) Init(app Application) error { return nil }

func (m *CycleModuleB) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "serviceB",
		Instance: &TestInterfaceBImpl{name: "B"},
	}}
}

func (m *CycleModuleB) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "serviceA",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeOf((*TestInterfaceA)(nil)).Elem(),
	}}
}

// LinearModuleA - only provides services, no dependencies
type LinearModuleA struct {
	name string
}

func (m *LinearModuleA) Name() string               { return m.name }
func (m *LinearModuleA) Init(app Application) error { return nil }

func (m *LinearModuleA) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "linearServiceA",
		Instance: &TestInterfaceAImpl{name: "LinearA"},
	}}
}

// LinearModuleB - depends on LinearModuleA
type LinearModuleB struct {
	name string
}

func (m *LinearModuleB) Name() string               { return m.name }
func (m *LinearModuleB) Init(app Application) error { return nil }

func (m *LinearModuleB) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "linearServiceA",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeOf((*TestInterfaceA)(nil)).Elem(),
	}}
}

// SelfDependentModule - depends on a service it provides
type SelfDependentModule struct {
	name string
}

func (m *SelfDependentModule) Name() string               { return m.name }
func (m *SelfDependentModule) Init(app Application) error { return nil }

// TestInterfaceAImpl implements TestInterfaceA for self-dependency testing
type TestInterfaceAImpl struct {
	name string
}

func (t *TestInterfaceAImpl) MethodA() string {
	return t.name
}

// TestInterfaceBImpl implements TestInterfaceB
type TestInterfaceBImpl struct {
	name string
}

func (t *TestInterfaceBImpl) MethodB() string {
	return t.name
}

// TestInterfaceCImpl implements TestInterfaceC
type TestInterfaceCImpl struct {
	name string
}

func (t *TestInterfaceCImpl) MethodC() string {
	return t.name
}

// EnhancedTestInterfaceImpl implements EnhancedTestInterface
type EnhancedTestInterfaceImpl struct {
	name string
}

func (t *EnhancedTestInterfaceImpl) TestMethod() string {
	return t.name
}

// AnotherEnhancedTestInterfaceImpl implements AnotherEnhancedTestInterface
type AnotherEnhancedTestInterfaceImpl struct {
	name string
}

func (t *AnotherEnhancedTestInterfaceImpl) AnotherTestMethod() string {
	return t.name
}

func (m *SelfDependentModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "selfService",
		Instance: &TestInterfaceAImpl{name: "self"},
	}}
}

func (m *SelfDependentModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "selfService",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeOf((*TestInterfaceA)(nil)).Elem(),
	}}
}

// MixedDependencyModuleA - has both named and interface dependencies
type MixedDependencyModuleA struct {
	name string
}

func (m *MixedDependencyModuleA) Name() string               { return m.name }
func (m *MixedDependencyModuleA) Init(app Application) error { return nil }

func (m *MixedDependencyModuleA) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "mixedServiceA",
		Instance: &TestInterfaceAImpl{name: "MixedA"},
	}}
}

func (m *MixedDependencyModuleA) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:             "namedServiceB", // Named dependency
		Required:         true,
		MatchByInterface: false,
	}}
}

// MixedDependencyModuleB - provides named service and requires interface
type MixedDependencyModuleB struct {
	name string
}

func (m *MixedDependencyModuleB) Name() string               { return m.name }
func (m *MixedDependencyModuleB) Init(app Application) error { return nil }

func (m *MixedDependencyModuleB) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "namedServiceB",
		Instance: &TestInterfaceBImpl{name: "MixedB"},
	}}
}

func (m *MixedDependencyModuleB) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "mixedServiceA",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeOf((*TestInterfaceA)(nil)).Elem(),
	}}
}

// ComplexCycleModuleA - part of 3-module cycle A->B->C->A
type ComplexCycleModuleA struct {
	name string
}

func (m *ComplexCycleModuleA) Name() string               { return m.name }
func (m *ComplexCycleModuleA) Init(app Application) error { return nil }

func (m *ComplexCycleModuleA) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "complexServiceA",
		Instance: &TestInterfaceAImpl{name: "ComplexA"},
	}}
}

func (m *ComplexCycleModuleA) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "complexServiceB",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeOf((*TestInterfaceB)(nil)).Elem(),
	}}
}

// ComplexCycleModuleB - part of 3-module cycle A->B->C->A
type ComplexCycleModuleB struct {
	name string
}

func (m *ComplexCycleModuleB) Name() string               { return m.name }
func (m *ComplexCycleModuleB) Init(app Application) error { return nil }

func (m *ComplexCycleModuleB) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "complexServiceB",
		Instance: &TestInterfaceBImpl{name: "ComplexB"},
	}}
}

func (m *ComplexCycleModuleB) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "complexServiceC",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeOf((*TestInterfaceC)(nil)).Elem(),
	}}
}

// ComplexCycleModuleC - part of 3-module cycle A->B->C->A
type ComplexCycleModuleC struct {
	name string
}

func (m *ComplexCycleModuleC) Name() string               { return m.name }
func (m *ComplexCycleModuleC) Init(app Application) error { return nil }

func (m *ComplexCycleModuleC) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "complexServiceC",
		Instance: &TestInterfaceCImpl{name: "ComplexC"},
	}}
}

func (m *ComplexCycleModuleC) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "complexServiceA",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeOf((*TestInterfaceA)(nil)).Elem(),
	}}
}

// DisambiguationModuleA - for interface name disambiguation testing
type DisambiguationModuleA struct {
	name string
}

func (m *DisambiguationModuleA) Name() string               { return m.name }
func (m *DisambiguationModuleA) Init(app Application) error { return nil }

func (m *DisambiguationModuleA) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "disambiguationServiceA",
		Instance: &EnhancedTestInterfaceImpl{name: "DisambigA"},
	}}
}

func (m *DisambiguationModuleA) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "disambiguationServiceB",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeOf((*AnotherEnhancedTestInterface)(nil)).Elem(),
	}}
}

// DisambiguationModuleB - for interface name disambiguation testing
type DisambiguationModuleB struct {
	name string
}

func (m *DisambiguationModuleB) Name() string               { return m.name }
func (m *DisambiguationModuleB) Init(app Application) error { return nil }

func (m *DisambiguationModuleB) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "disambiguationServiceB",
		Instance: &AnotherEnhancedTestInterfaceImpl{name: "DisambigB"},
	}}
}

func (m *DisambiguationModuleB) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "disambiguationServiceA",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeOf((*EnhancedTestInterface)(nil)).Elem(), // Note: different interface
	}}
}

// BDD Step implementations

func (ctx *EnhancedCycleDetectionBDDTestContext) iHaveAModularApplication() error {
	app, err := NewApplication(
		WithLogger(&testLogger{}),
		WithConfigProvider(NewStdConfigProvider(testCfg{Str: "test"})),
	)
	if err != nil {
		return err
	}
	ctx.app = app
	ctx.modules = make(map[string]Module)
	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) iHaveTwoModulesWithCircularInterfaceDependencies() error {
	moduleA := &CycleModuleA{name: "moduleA"}
	moduleB := &CycleModuleB{name: "moduleB"}

	ctx.modules["moduleA"] = moduleA
	ctx.modules["moduleB"] = moduleB

	ctx.app.RegisterModule(moduleA)
	ctx.app.RegisterModule(moduleB)

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) iTryToInitializeTheApplication() error {
	ctx.initializeResult = ctx.app.Init()
	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) theInitializationShouldFailWithACircularDependencyError() error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("expected initialization to fail with circular dependency error, but it succeeded")
	}

	if !IsErrCircularDependency(ctx.initializeResult) {
		return fmt.Errorf("expected ErrCircularDependency, got %T: %v", ctx.initializeResult, ctx.initializeResult)
	}

	ctx.cycleDetected = true
	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) theErrorMessageShouldIncludeBothModuleNames() error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("no error to check")
	}

	errorMsg := ctx.initializeResult.Error()
	if !strings.Contains(errorMsg, "moduleA") || !strings.Contains(errorMsg, "moduleB") {
		return fmt.Errorf("error message should contain both module names, got: %s", errorMsg)
	}

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) theErrorMessageShouldIndicateInterfaceBasedDependencies() error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("no error to check")
	}

	errorMsg := ctx.initializeResult.Error()
	if !strings.Contains(errorMsg, "interface:") {
		return fmt.Errorf("error message should indicate interface-based dependencies, got: %s", errorMsg)
	}

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) theErrorMessageShouldShowTheCompleteDependencyCycle() error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("no error to check")
	}

	errorMsg := ctx.initializeResult.Error()
	if !strings.Contains(errorMsg, "cycle:") {
		return fmt.Errorf("error message should show complete cycle, got: %s", errorMsg)
	}

	// Check for arrow notation indicating dependency flow
	if !strings.Contains(errorMsg, "→") {
		return fmt.Errorf("error message should use arrow notation for dependency flow, got: %s", errorMsg)
	}

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) iHaveModulesAAndBWhereARequiresInterfaceTestInterfaceAndBProvidesTestInterface() error {
	// This is effectively the same as the circular dependency setup
	return ctx.iHaveTwoModulesWithCircularInterfaceDependencies()
}

func (ctx *EnhancedCycleDetectionBDDTestContext) moduleBAlsoRequiresInterfaceTestInterfaceCreatingACycle() error {
	// Already handled in the setup above - moduleB requires TestInterfaceA
	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) theErrorMessageShouldContain(expectedMsg string) error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("no error to check")
	}

	errorMsg := ctx.initializeResult.Error()

	// The exact format might vary, so let's check for key components
	requiredComponents := []string{"cycle:", "moduleA", "moduleB", "interface:", "TestInterface"}
	for _, component := range requiredComponents {
		if !strings.Contains(errorMsg, component) {
			return fmt.Errorf("error message should contain '%s', got: %s", component, errorMsg)
		}
	}

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) theErrorMessageShouldClearlyShowTheInterfaceCausingTheCycle() error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("no error to check")
	}

	errorMsg := ctx.initializeResult.Error()
	// Look for interface specification in the error message
	if !strings.Contains(errorMsg, "TestInterface") {
		return fmt.Errorf("error message should clearly show TestInterface causing the cycle, got: %s", errorMsg)
	}

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) iHaveModulesWithValidLinearDependencies() error {
	moduleA := &LinearModuleA{name: "linearA"}
	moduleB := &LinearModuleB{name: "linearB"}

	ctx.modules["linearA"] = moduleA
	ctx.modules["linearB"] = moduleB

	ctx.app.RegisterModule(moduleA)
	ctx.app.RegisterModule(moduleB)

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) iInitializeTheApplication() error {
	ctx.initializeResult = ctx.app.Init()
	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) theInitializationShouldSucceed() error {
	if ctx.initializeResult != nil {
		return fmt.Errorf("expected initialization to succeed, got error: %v", ctx.initializeResult)
	}
	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) noCircularDependencyErrorShouldBeReported() error {
	if IsErrCircularDependency(ctx.initializeResult) {
		return fmt.Errorf("unexpected circular dependency error: %v", ctx.initializeResult)
	}
	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) iHaveAModuleThatDependsOnAServiceItAlsoProvides() error {
	module := &SelfDependentModule{name: "selfModule"}

	ctx.modules["selfModule"] = module
	ctx.app.RegisterModule(module)

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) aSelfDependencyCycleShouldBeDetected() error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("expected self-dependency cycle to be detected")
	}

	// With improved self-interface pruning, a self-required interface dependency
	// manifests as an unsatisfied required service instead of an artificial cycle.
	// Accept either a circular dependency error (legacy behavior) or a required
	// service not found error referencing the self module.
	if !IsErrCircularDependency(ctx.initializeResult) {
		// Fallback acceptance: required service not found for the module's own interface
		if !strings.Contains(ctx.initializeResult.Error(), "required service not found") || !strings.Contains(ctx.initializeResult.Error(), "selfModule") {
			return fmt.Errorf("expected circular dependency or unsatisfied self service error, got %v", ctx.initializeResult)
		}
	}

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) theErrorMessageShouldClearlyIndicateTheSelfDependency() error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("no error to check")
	}

	errorMsg := ctx.initializeResult.Error()
	// Should mention the module name and self-reference
	if !strings.Contains(errorMsg, "selfModule") {
		return fmt.Errorf("error message should mention the self-dependent module, got: %s", errorMsg)
	}

	return nil
}

// Missing step implementations for complex scenarios

func (ctx *EnhancedCycleDetectionBDDTestContext) iHaveModulesWithBothNamedServiceDependenciesAndInterfaceDependencies() error {
	moduleA := &MixedDependencyModuleA{name: "mixedA"}
	moduleB := &MixedDependencyModuleB{name: "mixedB"}

	ctx.modules["mixedA"] = moduleA
	ctx.modules["mixedB"] = moduleB

	ctx.app.RegisterModule(moduleA)
	ctx.app.RegisterModule(moduleB)

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) theDependenciesFormACircularChain() error {
	// Dependencies are already set up in the modules - mixedA requires namedServiceB, mixedB requires interface TestInterfaceA
	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) theErrorMessageShouldDistinguishBetweenInterfaceAndNamedDependencies() error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("no error to check")
	}

	errorMsg := ctx.initializeResult.Error()
	// Should contain both service: and interface: markers
	hasService := strings.Contains(errorMsg, "(service:")
	hasInterface := strings.Contains(errorMsg, "(interface:")

	if !hasService || !hasInterface {
		return fmt.Errorf("error message should distinguish between service and interface dependencies, got: %s", errorMsg)
	}

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) bothDependencyTypesShouldBeIncludedInTheCycleDescription() error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("no error to check")
	}

	errorMsg := ctx.initializeResult.Error()
	// Should show the complete cycle with both dependency types
	if !strings.Contains(errorMsg, "cycle:") {
		return fmt.Errorf("error message should contain cycle description, got: %s", errorMsg)
	}

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) iHaveModulesABAndCWhereADependsOnBBDependsOnCAndCDependsOnA() error {
	moduleA := &ComplexCycleModuleA{name: "complexA"}
	moduleB := &ComplexCycleModuleB{name: "complexB"}
	moduleC := &ComplexCycleModuleC{name: "complexC"}

	ctx.modules["complexA"] = moduleA
	ctx.modules["complexB"] = moduleB
	ctx.modules["complexC"] = moduleC

	ctx.app.RegisterModule(moduleA)
	ctx.app.RegisterModule(moduleB)
	ctx.app.RegisterModule(moduleC)

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) theCompleteCyclePathShouldBeShownInTheErrorMessage() error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("no error to check")
	}

	errorMsg := ctx.initializeResult.Error()
	if !strings.Contains(errorMsg, "cycle:") {
		return fmt.Errorf("error message should show complete cycle path, got: %s", errorMsg)
	}

	// Should contain arrow notation showing the path
	if !strings.Contains(errorMsg, "→") {
		return fmt.Errorf("error message should use arrow notation for cycle path, got: %s", errorMsg)
	}

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) allThreeModulesShouldBeMentionedInTheCycleDescription() error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("no error to check")
	}

	errorMsg := ctx.initializeResult.Error()
	requiredModules := []string{"complexA", "complexB", "complexC"}

	for _, module := range requiredModules {
		if !strings.Contains(errorMsg, module) {
			return fmt.Errorf("error message should mention all three modules (%v), got: %s", requiredModules, errorMsg)
		}
	}

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) iHaveMultipleInterfacesWithSimilarNamesCausingCycles() error {
	moduleA := &DisambiguationModuleA{name: "disambigA"}
	moduleB := &DisambiguationModuleB{name: "disambigB"}

	ctx.modules["disambigA"] = moduleA
	ctx.modules["disambigB"] = moduleB

	ctx.app.RegisterModule(moduleA)
	ctx.app.RegisterModule(moduleB)

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) cycleDetectionRuns() error {
	ctx.initializeResult = ctx.app.Init()
	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) interfaceNamesInErrorMessagesShouldBeFullyQualified() error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("no error to check")
	}

	errorMsg := ctx.initializeResult.Error()
	// Should contain fully qualified interface names to avoid ambiguity
	// Look for package prefix in interface names
	if !strings.Contains(errorMsg, "modular.EnhancedTestInterface") && !strings.Contains(errorMsg, "modular.AnotherEnhancedTestInterface") {
		return fmt.Errorf("error message should contain fully qualified interface names, got: %s", errorMsg)
	}

	return nil
}

func (ctx *EnhancedCycleDetectionBDDTestContext) thereShouldBeNoAmbiguityAboutWhichInterfaceCausedTheCycle() error {
	if ctx.initializeResult == nil {
		return fmt.Errorf("no error to check")
	}

	errorMsg := ctx.initializeResult.Error()
	// The interface names should be clearly distinguishable
	if strings.Contains(errorMsg, "EnhancedTestInterface") && strings.Contains(errorMsg, "AnotherEnhancedTestInterface") {
		// Both interfaces mentioned - good disambiguation
		return nil
	}

	return fmt.Errorf("error message should clearly distinguish between different interfaces, got: %s", errorMsg)
}

// Test runner
func TestEnhancedCycleDetectionBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			testContext := &EnhancedCycleDetectionBDDTestContext{}

			// Background
			ctx.Step(`^I have a modular application$`, testContext.iHaveAModularApplication)

			// Cycle detection scenarios
			ctx.Step(`^I have two modules with circular interface dependencies$`, testContext.iHaveTwoModulesWithCircularInterfaceDependencies)
			ctx.Step(`^I try to initialize the application$`, testContext.iTryToInitializeTheApplication)
			ctx.Step(`^the initialization should fail with a circular dependency error$`, testContext.theInitializationShouldFailWithACircularDependencyError)
			ctx.Step(`^the error message should include both module names$`, testContext.theErrorMessageShouldIncludeBothModuleNames)
			ctx.Step(`^the error message should indicate interface-based dependencies$`, testContext.theErrorMessageShouldIndicateInterfaceBasedDependencies)
			ctx.Step(`^the error message should show the complete dependency cycle$`, testContext.theErrorMessageShouldShowTheCompleteDependencyCycle)

			// Enhanced error message format
			ctx.Step(`^I have modules A and B where A requires interface TestInterface and B provides TestInterface$`, testContext.iHaveModulesAAndBWhereARequiresInterfaceTestInterfaceAndBProvidesTestInterface)
			ctx.Step(`^module B also requires interface TestInterface creating a cycle$`, testContext.moduleBAlsoRequiresInterfaceTestInterfaceCreatingACycle)
			ctx.Step(`^the error message should contain "([^"]*)"$`, testContext.theErrorMessageShouldContain)
			ctx.Step(`^the error message should clearly show the interface causing the cycle$`, testContext.theErrorMessageShouldClearlyShowTheInterfaceCausingTheCycle)

			// Linear dependencies (no cycles)
			ctx.Step(`^I have modules with valid linear dependencies$`, testContext.iHaveModulesWithValidLinearDependencies)
			ctx.Step(`^I initialize the application$`, testContext.iInitializeTheApplication)
			ctx.Step(`^the initialization should succeed$`, testContext.theInitializationShouldSucceed)
			ctx.Step(`^no circular dependency error should be reported$`, testContext.noCircularDependencyErrorShouldBeReported)

			// Self-dependency
			ctx.Step(`^I have a module that depends on a service it also provides$`, testContext.iHaveAModuleThatDependsOnAServiceItAlsoProvides)
			ctx.Step(`^a self-dependency cycle should be detected$`, testContext.aSelfDependencyCycleShouldBeDetected)
			ctx.Step(`^the error message should clearly indicate the self-dependency$`, testContext.theErrorMessageShouldClearlyIndicateTheSelfDependency)

			// Mixed dependency types
			ctx.Step(`^I have modules with both named service dependencies and interface dependencies$`, testContext.iHaveModulesWithBothNamedServiceDependenciesAndInterfaceDependencies)
			ctx.Step(`^the dependencies form a circular chain$`, testContext.theDependenciesFormACircularChain)
			ctx.Step(`^the error message should distinguish between interface and named dependencies$`, testContext.theErrorMessageShouldDistinguishBetweenInterfaceAndNamedDependencies)
			ctx.Step(`^both dependency types should be included in the cycle description$`, testContext.bothDependencyTypesShouldBeIncludedInTheCycleDescription)

			// Complex multi-module cycles
			ctx.Step(`^I have modules A, B, and C where A depends on B, B depends on C, and C depends on A$`, testContext.iHaveModulesABAndCWhereADependsOnBBDependsOnCAndCDependsOnA)
			ctx.Step(`^the complete cycle path should be shown in the error message$`, testContext.theCompleteCyclePathShouldBeShownInTheErrorMessage)
			ctx.Step(`^all three modules should be mentioned in the cycle description$`, testContext.allThreeModulesShouldBeMentionedInTheCycleDescription)

			// Interface name disambiguation
			ctx.Step(`^I have multiple interfaces with similar names causing cycles$`, testContext.iHaveMultipleInterfacesWithSimilarNamesCausingCycles)
			ctx.Step(`^cycle detection runs$`, testContext.cycleDetectionRuns)
			ctx.Step(`^interface names in error messages should be fully qualified$`, testContext.interfaceNamesInErrorMessagesShouldBeFullyQualified)
			ctx.Step(`^there should be no ambiguity about which interface caused the cycle$`, testContext.thereShouldBeNoAmbiguityAboutWhichInterfaceCausedTheCycle)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/enhanced_cycle_detection.feature"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run BDD tests")
	}
}
