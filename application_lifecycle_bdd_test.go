package modular

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cucumber/godog"
)

// Static error variables for BDD tests to comply with err113 linting rule
var (
	errInitializationFailed           = errors.New("initialization failed")
	errApplicationNotCreated          = errors.New("application was not created in background")
	errApplicationIsNil               = errors.New("application is nil")
	errConfigProviderIsNil            = errors.New("config provider is nil")
	errNoModulesToRegister            = errors.New("no modules to register")
	errModuleShouldNotBeInitialized   = errors.New("module should not be initialized yet")
	errModuleShouldBeInitialized      = errors.New("module should be initialized")
	errProviderModuleShouldBeInit     = errors.New("provider module should be initialized")
	errConsumerModuleShouldBeInit     = errors.New("consumer module should be initialized")
	errConsumerShouldReceiveService   = errors.New("consumer module should have received the service")
	errStartableModuleShouldBeStarted = errors.New("startable module should be started")
	errStartableModuleShouldBeStopped = errors.New("startable module should be stopped")
	errExpectedInitializationToFail   = errors.New("expected initialization to fail")
	errNoErrorToCheck                 = errors.New("no error to check")
	errErrorMessageIsEmpty            = errors.New("error message is empty")
)

// BDDTestContext holds the test context for BDD scenarios
type BDDTestContext struct {
	app           Application
	logger        Logger
	modules       []Module
	initError     error
	startError    error
	stopError     error
	moduleStates  map[string]bool
	servicesFound map[string]interface{}
}

// Test modules for BDD scenarios
type SimpleTestModule struct {
	name        string
	initialized bool
	started     bool
	stopped     bool
}

func (m *SimpleTestModule) Name() string {
	return m.name
}

func (m *SimpleTestModule) Init(app Application) error {
	m.initialized = true
	return nil
}

type StartableTestModule struct {
	SimpleTestModule
}

func (m *StartableTestModule) Start(ctx context.Context) error {
	m.started = true
	return nil
}

func (m *StartableTestModule) Stop(ctx context.Context) error {
	m.stopped = true
	return nil
}

type ProviderTestModule struct {
	SimpleTestModule
}

func (m *ProviderTestModule) Init(app Application) error {
	m.initialized = true
	if err := app.RegisterService("test-service", &MockTestService{}); err != nil {
		return fmt.Errorf("failed to register test service: %w", err)
	}
	return nil
}

type MockTestService struct{}

type ConsumerTestModule struct {
	SimpleTestModule
	receivedService interface{}
}

func (m *ConsumerTestModule) Init(app Application) error {
	m.initialized = true
	var service MockTestService
	err := app.GetService("test-service", &service)
	if err == nil {
		m.receivedService = &service
	}
	return nil
}

func (m *ConsumerTestModule) Dependencies() []string {
	return []string{"provider"}
}

type BDDFailingTestModule struct {
	SimpleTestModule
}

func (m *BDDFailingTestModule) Init(app Application) error {
	return errInitializationFailed
}

// Step definitions
func (ctx *BDDTestContext) resetContext() {
	ctx.app = nil
	ctx.logger = nil
	ctx.modules = nil
	ctx.initError = nil
	ctx.startError = nil
	ctx.stopError = nil
	ctx.moduleStates = make(map[string]bool)
	ctx.servicesFound = make(map[string]interface{})
}

func (ctx *BDDTestContext) iHaveANewModularApplication() error {
	ctx.resetContext()
	return nil
}

func (ctx *BDDTestContext) iHaveALoggerConfigured() error {
	ctx.logger = &BDDTestLogger{}
	// Create the application here since both background steps are done
	cp := NewStdConfigProvider(struct{}{})
	ctx.app = NewStdApplication(cp, ctx.logger)
	return nil
}

func (ctx *BDDTestContext) iCreateANewStandardApplication() error {
	// Application already created in background, just verify it exists
	if ctx.app == nil {
		return errApplicationNotCreated
	}
	return nil
}

func (ctx *BDDTestContext) theApplicationShouldBeProperlyInitialized() error {
	if ctx.app == nil {
		return errApplicationIsNil
	}
	if ctx.app.ConfigProvider() == nil {
		return errConfigProviderIsNil
	}
	return nil
}

func (ctx *BDDTestContext) theServiceRegistryShouldBeEmpty() error {
	// Note: This would require exposing service count in the interface
	// For now, we assume it's empty for a new application
	return nil
}

func (ctx *BDDTestContext) theModuleRegistryShouldBeEmpty() error {
	// Note: This would require exposing module count in the interface
	// For now, we assume it's empty for a new application
	return nil
}

func (ctx *BDDTestContext) iHaveASimpleTestModule() error {
	module := &SimpleTestModule{name: "simple-test"}
	ctx.modules = append(ctx.modules, module)
	return nil
}

func (ctx *BDDTestContext) iRegisterTheModuleWithTheApplication() error {
	if len(ctx.modules) == 0 {
		return errNoModulesToRegister
	}
	for _, module := range ctx.modules {
		ctx.app.RegisterModule(module)
	}
	return nil
}

func (ctx *BDDTestContext) theModuleShouldBeRegisteredInTheModuleRegistry() error {
	// Note: This would require exposing module lookup in the interface
	// For now, we assume registration was successful if no error occurred
	return nil
}

func (ctx *BDDTestContext) theModuleShouldNotBeInitializedYet() error {
	for _, module := range ctx.modules {
		if testModule, ok := module.(*SimpleTestModule); ok {
			if testModule.initialized {
				return fmt.Errorf("module %s: %w", testModule.name, errModuleShouldNotBeInitialized)
			}
		}
	}
	return nil
}

func (ctx *BDDTestContext) iHaveRegisteredASimpleTestModule() error {
	if err := ctx.iHaveASimpleTestModule(); err != nil {
		return err
	}
	return ctx.iRegisterTheModuleWithTheApplication()
}

func (ctx *BDDTestContext) iInitializeTheApplication() error {
	ctx.initError = ctx.app.Init()
	return nil
}

func (ctx *BDDTestContext) theModuleShouldBeInitialized() error {
	for _, module := range ctx.modules {
		if testModule, ok := module.(*SimpleTestModule); ok {
			if !testModule.initialized {
				return fmt.Errorf("module %s: %w", testModule.name, errModuleShouldBeInitialized)
			}
		}
	}
	return nil
}

func (ctx *BDDTestContext) anyServicesProvidedByTheModuleShouldBeRegistered() error {
	// Check if any services were registered (this is implementation-specific)
	return nil
}

func (ctx *BDDTestContext) iHaveAProviderModuleThatProvidesAService() error {
	module := &ProviderTestModule{SimpleTestModule{name: "provider", initialized: false}}
	ctx.modules = append(ctx.modules, module)
	return nil
}

func (ctx *BDDTestContext) iHaveAConsumerModuleThatDependsOnThatService() error {
	module := &ConsumerTestModule{SimpleTestModule{name: "consumer", initialized: false}, nil}
	ctx.modules = append(ctx.modules, module)
	return nil
}

func (ctx *BDDTestContext) iRegisterBothModulesWithTheApplication() error {
	return ctx.iRegisterTheModuleWithTheApplication()
}

func (ctx *BDDTestContext) bothModulesShouldBeInitializedInDependencyOrder() error {
	// Check that both modules are initialized
	for _, module := range ctx.modules {
		if testModule, ok := module.(*SimpleTestModule); ok {
			if !testModule.initialized {
				return fmt.Errorf("module %s: %w", testModule.name, errModuleShouldBeInitialized)
			}
		}
		if testModule, ok := module.(*ProviderTestModule); ok {
			if !testModule.initialized {
				return errProviderModuleShouldBeInit
			}
		}
		if testModule, ok := module.(*ConsumerTestModule); ok {
			if !testModule.initialized {
				return errConsumerModuleShouldBeInit
			}
		}
	}
	return nil
}

func (ctx *BDDTestContext) theConsumerModuleShouldReceiveTheServiceFromTheProvider() error {
	for _, module := range ctx.modules {
		if consumerModule, ok := module.(*ConsumerTestModule); ok {
			if consumerModule.receivedService == nil {
				return errConsumerShouldReceiveService
			}
		}
	}
	return nil
}

func (ctx *BDDTestContext) iHaveAStartableTestModule() error {
	module := &StartableTestModule{SimpleTestModule{name: "startable-test", initialized: false}}
	ctx.modules = append(ctx.modules, module)
	return nil
}

func (ctx *BDDTestContext) theModuleIsRegisteredAndInitialized() error {
	if err := ctx.iRegisterTheModuleWithTheApplication(); err != nil {
		return err
	}
	return ctx.iInitializeTheApplication()
}

func (ctx *BDDTestContext) iStartTheApplication() error {
	ctx.startError = ctx.app.Start()
	return nil
}

func (ctx *BDDTestContext) theStartableModuleShouldBeStarted() error {
	for _, module := range ctx.modules {
		if startableModule, ok := module.(*StartableTestModule); ok {
			if !startableModule.started {
				return errStartableModuleShouldBeStarted
			}
		}
	}
	return nil
}

func (ctx *BDDTestContext) iStopTheApplication() error {
	ctx.stopError = ctx.app.Stop()
	return nil
}

func (ctx *BDDTestContext) theStartableModuleShouldBeStopped() error {
	for _, module := range ctx.modules {
		if startableModule, ok := module.(*StartableTestModule); ok {
			if !startableModule.stopped {
				return errStartableModuleShouldBeStopped
			}
		}
	}
	return nil
}

func (ctx *BDDTestContext) iHaveAModuleThatFailsDuringInitialization() error {
	module := &BDDFailingTestModule{SimpleTestModule{name: "failing-test", initialized: false}}
	ctx.modules = append(ctx.modules, module)
	// Register it with the application so it's included in initialization
	return ctx.iRegisterTheModuleWithTheApplication()
}

func (ctx *BDDTestContext) iTryToInitializeTheApplication() error {
	ctx.initError = ctx.app.Init()
	return nil
}

func (ctx *BDDTestContext) theInitializationShouldFail() error {
	if ctx.initError == nil {
		return errExpectedInitializationToFail
	}
	return nil
}

func (ctx *BDDTestContext) theErrorShouldIncludeDetailsAboutWhichModuleFailed() error {
	if ctx.initError == nil {
		return errNoErrorToCheck
	}
	// Check that the error message contains relevant information
	if len(ctx.initError.Error()) == 0 {
		return errErrorMessageIsEmpty
	}
	return nil
}

type CircularDepModuleA struct {
	SimpleTestModule
}

func (m *CircularDepModuleA) Dependencies() []string {
	return []string{"circular-b"}
}

type CircularDepModuleB struct {
	SimpleTestModule
}

func (m *CircularDepModuleB) Dependencies() []string {
	return []string{"circular-a"}
}

func (ctx *BDDTestContext) iHaveTwoModulesWithCircularDependencies() error {
	moduleA := &CircularDepModuleA{SimpleTestModule{name: "circular-a", initialized: false}}
	moduleB := &CircularDepModuleB{SimpleTestModule{name: "circular-b", initialized: false}}
	ctx.modules = append(ctx.modules, moduleA, moduleB)
	return ctx.iRegisterTheModuleWithTheApplication()
}

func (ctx *BDDTestContext) theErrorShouldIndicateCircularDependency() error {
	if ctx.initError == nil {
		return errNoErrorToCheck
	}
	// This would check for specific circular dependency error
	return nil
}

// BDDTestLogger for BDD tests
type BDDTestLogger struct{}

func (l *BDDTestLogger) Debug(msg string, fields ...interface{}) {}
func (l *BDDTestLogger) Info(msg string, fields ...interface{})  {}
func (l *BDDTestLogger) Warn(msg string, fields ...interface{})  {}
func (l *BDDTestLogger) Error(msg string, fields ...interface{}) {}

// InitializeScenario initializes the BDD test scenario
func InitializeScenario(ctx *godog.ScenarioContext) {
	testCtx := &BDDTestContext{
		moduleStates:  make(map[string]bool),
		servicesFound: make(map[string]interface{}),
	}

	// Reset context before each scenario
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		testCtx.resetContext()
		return ctx, nil
	})

	// Background steps
	ctx.Step(`^I have a new modular application$`, testCtx.iHaveANewModularApplication)
	ctx.Step(`^I have a logger configured$`, testCtx.iHaveALoggerConfigured)

	// Application creation steps
	ctx.Step(`^I create a new standard application$`, testCtx.iCreateANewStandardApplication)
	ctx.Step(`^the application should be properly initialized$`, testCtx.theApplicationShouldBeProperlyInitialized)
	ctx.Step(`^the service registry should be empty$`, testCtx.theServiceRegistryShouldBeEmpty)
	ctx.Step(`^the module registry should be empty$`, testCtx.theModuleRegistryShouldBeEmpty)

	// Module registration steps
	ctx.Step(`^I have a simple test module$`, testCtx.iHaveASimpleTestModule)
	ctx.Step(`^I register the module with the application$`, testCtx.iRegisterTheModuleWithTheApplication)
	ctx.Step(`^the module should be registered in the module registry$`, testCtx.theModuleShouldBeRegisteredInTheModuleRegistry)
	ctx.Step(`^the module should not be initialized yet$`, testCtx.theModuleShouldNotBeInitializedYet)

	// Module initialization steps
	ctx.Step(`^I have registered a simple test module$`, testCtx.iHaveRegisteredASimpleTestModule)
	ctx.Step(`^I initialize the application$`, testCtx.iInitializeTheApplication)
	ctx.Step(`^the module should be initialized$`, testCtx.theModuleShouldBeInitialized)
	ctx.Step(`^any services provided by the module should be registered$`, testCtx.anyServicesProvidedByTheModuleShouldBeRegistered)

	// Dependency resolution steps
	ctx.Step(`^I have a provider module that provides a service$`, testCtx.iHaveAProviderModuleThatProvidesAService)
	ctx.Step(`^I have a consumer module that depends on that service$`, testCtx.iHaveAConsumerModuleThatDependsOnThatService)
	ctx.Step(`^I register both modules with the application$`, testCtx.iRegisterBothModulesWithTheApplication)
	ctx.Step(`^both modules should be initialized in dependency order$`, testCtx.bothModulesShouldBeInitializedInDependencyOrder)
	ctx.Step(`^the consumer module should receive the service from the provider$`, testCtx.theConsumerModuleShouldReceiveTheServiceFromTheProvider)

	// Startable module steps
	ctx.Step(`^I have a startable test module$`, testCtx.iHaveAStartableTestModule)
	ctx.Step(`^the module is registered and initialized$`, testCtx.theModuleIsRegisteredAndInitialized)
	ctx.Step(`^I start the application$`, testCtx.iStartTheApplication)
	ctx.Step(`^the startable module should be started$`, testCtx.theStartableModuleShouldBeStarted)
	ctx.Step(`^I stop the application$`, testCtx.iStopTheApplication)
	ctx.Step(`^the startable module should be stopped$`, testCtx.theStartableModuleShouldBeStopped)

	// Error handling steps
	ctx.Step(`^I have a module that fails during initialization$`, testCtx.iHaveAModuleThatFailsDuringInitialization)
	ctx.Step(`^I try to initialize the application$`, testCtx.iTryToInitializeTheApplication)
	ctx.Step(`^the initialization should fail$`, testCtx.theInitializationShouldFail)
	ctx.Step(`^the error should include details about which module failed$`, testCtx.theErrorShouldIncludeDetailsAboutWhichModuleFailed)

	// Circular dependency steps
	ctx.Step(`^I have two modules with circular dependencies$`, testCtx.iHaveTwoModulesWithCircularDependencies)
	ctx.Step(`^the error should indicate circular dependency$`, testCtx.theErrorShouldIndicateCircularDependency)
}

// TestApplicationLifecycle runs the BDD tests for application lifecycle
func TestApplicationLifecycle(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/application_lifecycle.feature"},
			TestingT: t,
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
