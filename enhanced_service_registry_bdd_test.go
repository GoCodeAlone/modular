package modular

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/cucumber/godog"
)

// BDD Test Context for Enhanced Service Registry
type EnhancedServiceRegistryBDDContext struct {
	app                Application
	modules            map[string]Module
	services           map[string]any
	lastError          error
	retrievedServices  []*ServiceRegistryEntry
	servicesByModule   []string
	serviceEntry       *ServiceRegistryEntry
	serviceEntryExists bool
}

// Test interface for interface-based discovery tests
type TestServiceInterface interface {
	DoSomething() string
}

// Mock implementation of TestServiceInterface
type EnhancedMockTestService struct {
	identifier string
}

func (m *EnhancedMockTestService) DoSomething() string {
	return fmt.Sprintf("Service: %s", m.identifier)
}

// Test modules for BDD scenarios

// SingleServiceModule provides one service
type SingleServiceModule struct {
	name        string
	serviceName string
	service     any
}

func (m *SingleServiceModule) Name() string               { return m.name }
func (m *SingleServiceModule) Init(app Application) error { return nil }
func (m *SingleServiceModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     m.serviceName,
		Instance: m.service,
	}}
}

// ConflictingServiceModule provides a service that might conflict with others
type ConflictingServiceModule struct {
	name        string
	serviceName string
	service     any
}

func (m *ConflictingServiceModule) Name() string               { return m.name }
func (m *ConflictingServiceModule) Init(app Application) error { return nil }
func (m *ConflictingServiceModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     m.serviceName,
		Instance: m.service,
	}}
}

// MultiServiceModule provides multiple services
type MultiServiceModule struct {
	name     string
	services []ServiceProvider
}

func (m *MultiServiceModule) Name() string               { return m.name }
func (m *MultiServiceModule) Init(app Application) error { return nil }
func (m *MultiServiceModule) ProvidesServices() []ServiceProvider {
	return m.services
}

// BDD Step implementations

func (ctx *EnhancedServiceRegistryBDDContext) iHaveAModularApplicationWithEnhancedServiceRegistry() error {
	// Use the builder pattern for cleaner application creation
	app, err := NewApplication(
		WithLogger(&testLogger{}),
		WithConfigProvider(NewStdConfigProvider(testCfg{Str: "test"})),
	)
	if err != nil {
		return err
	}
	ctx.app = app
	ctx.modules = make(map[string]Module)
	ctx.services = make(map[string]any)
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iHaveAModuleThatProvidesAService(moduleName, serviceName string) error {
	service := &EnhancedMockTestService{identifier: fmt.Sprintf("%s:%s", moduleName, serviceName)}
	module := &SingleServiceModule{
		name:        moduleName,
		serviceName: serviceName,
		service:     service,
	}

	ctx.modules[moduleName] = module
	ctx.services[serviceName] = service
	ctx.app.RegisterModule(module)
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iRegisterTheModuleAndInitializeTheApplication() error {
	err := ctx.app.Init()
	ctx.lastError = err
	return err
}

func (ctx *EnhancedServiceRegistryBDDContext) theServiceShouldBeRegisteredWithModuleAssociation() error {
	// Check that services exist in the registry
	for serviceName := range ctx.services {
		var service *EnhancedMockTestService
		err := ctx.app.GetService(serviceName, &service)
		if err != nil {
			return fmt.Errorf("service %s not found: %w", serviceName, err)
		}
	}
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iShouldBeAbleToRetrieveTheServiceEntryWithModuleInformation() error {
	for serviceName := range ctx.services {
		entry, exists := ctx.app.GetServiceEntry(serviceName)
		if !exists {
			return fmt.Errorf("service entry for %s not found", serviceName)
		}

		if entry.OriginalName != serviceName {
			return fmt.Errorf("expected original name %s, got %s", serviceName, entry.OriginalName)
		}

		if entry.ModuleName == "" {
			return fmt.Errorf("module name should not be empty for service %s", serviceName)
		}
	}
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iHaveTwoModulesThatBothProvideService(moduleA, moduleB, serviceName string) error {
	serviceA := &EnhancedMockTestService{identifier: fmt.Sprintf("%s:%s", moduleA, serviceName)}
	serviceB := &EnhancedMockTestService{identifier: fmt.Sprintf("%s:%s", moduleB, serviceName)}

	moduleObjA := &ConflictingServiceModule{
		name:        moduleA,
		serviceName: serviceName,
		service:     serviceA,
	}

	moduleObjB := &ConflictingServiceModule{
		name:        moduleB,
		serviceName: serviceName,
		service:     serviceB,
	}

	ctx.modules[moduleA] = moduleObjA
	ctx.modules[moduleB] = moduleObjB
	ctx.services[serviceName+".A"] = serviceA // Expected resolved names
	ctx.services[serviceName+".B"] = serviceB

	ctx.app.RegisterModule(moduleObjA)
	ctx.app.RegisterModule(moduleObjB)
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iRegisterBothModulesAndInitializeTheApplication() error {
	return ctx.iRegisterTheModuleAndInitializeTheApplication()
}

func (ctx *EnhancedServiceRegistryBDDContext) theFirstModuleShouldKeepTheOriginalServiceName() error {
	// The first module should keep the original name
	var service EnhancedMockTestService
	err := ctx.app.GetService("duplicateService", &service)
	if err != nil {
		return fmt.Errorf("original service name not found: %w", err)
	}
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) theSecondModuleShouldGetAModuleSuffixedName() error {
	// Check if a module-suffixed version exists
	var service EnhancedMockTestService
	err := ctx.app.GetService("duplicateService.ModuleB", &service)
	if err != nil {
		return fmt.Errorf("module-suffixed service name not found: %w", err)
	}
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) bothServicesShouldBeAccessibleThroughTheirResolvedNames() error {
	// Both services should be accessible
	var serviceA, serviceB EnhancedMockTestService

	errA := ctx.app.GetService("duplicateService", &serviceA)
	errB := ctx.app.GetService("duplicateService.ModuleB", &serviceB)

	if errA != nil || errB != nil {
		return fmt.Errorf("not all services accessible: %v, %v", errA, errB)
	}

	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iHaveMultipleModulesProvidingServicesThatImplement(interfaceName string) error {
	// Create modules that provide services implementing TestServiceInterface
	for i, moduleName := range []string{"InterfaceModuleA", "InterfaceModuleB", "InterfaceModuleC"} {
		service := &EnhancedMockTestService{identifier: fmt.Sprintf("service%d", i+1)}
		module := &SingleServiceModule{
			name:        moduleName,
			serviceName: fmt.Sprintf("interfaceService%d", i+1),
			service:     service,
		}

		ctx.modules[moduleName] = module
		ctx.app.RegisterModule(module)
	}
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iQueryForServicesByInterfaceType() error {
	// Initialize the application first
	err := ctx.app.Init()
	if err != nil {
		ctx.lastError = err
		return err
	}

	// Query for services implementing TestServiceInterface
	interfaceType := reflect.TypeOf((*TestServiceInterface)(nil)).Elem()
	ctx.retrievedServices = ctx.app.GetServicesByInterface(interfaceType)
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iShouldGetAllServicesImplementingThatInterface() error {
	if len(ctx.retrievedServices) != 3 {
		return fmt.Errorf("expected 3 services implementing interface, got %d", len(ctx.retrievedServices))
	}
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) eachServiceShouldIncludeItsModuleAssociationInformation() error {
	for _, entry := range ctx.retrievedServices {
		if entry.ModuleName == "" {
			return fmt.Errorf("service %s missing module name", entry.ActualName)
		}
		if entry.ModuleType == nil {
			return fmt.Errorf("service %s missing module type", entry.ActualName)
		}
	}
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iHaveModulesProvidingDifferentServices(moduleA, moduleB, moduleC string) error {
	modules := []struct {
		name    string
		service string
	}{
		{moduleA, "serviceA"},
		{moduleB, "serviceB"},
		{moduleB, "serviceBExtra"}, // ModuleB provides 2 services
		{moduleC, "serviceC"},
	}

	for _, m := range modules {
		service := &EnhancedMockTestService{identifier: m.service}

		// Check if module already exists
		if existingModule, exists := ctx.modules[m.name]; exists {
			// Add to existing multi-service module
			if multiModule, ok := existingModule.(*MultiServiceModule); ok {
				multiModule.services = append(multiModule.services, ServiceProvider{
					Name:     m.service,
					Instance: service,
				})
			}
		} else {
			// Create new module
			module := &MultiServiceModule{
				name: m.name,
				services: []ServiceProvider{{
					Name:     m.service,
					Instance: service,
				}},
			}
			ctx.modules[m.name] = module
			ctx.app.RegisterModule(module)
		}
	}
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iQueryForServicesProvidedBy(moduleName string) error {
	// Initialize first if not done
	if ctx.lastError == nil {
		err := ctx.app.Init()
		if err != nil {
			ctx.lastError = err
			return err
		}
	}

	ctx.servicesByModule = ctx.app.GetServicesByModule(moduleName)
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iShouldGetOnlyTheServicesRegisteredBy(moduleName string) error {
	expectedCount := 0
	if moduleName == "ModuleB" {
		expectedCount = 2 // ModuleB provides 2 services
	} else {
		expectedCount = 1
	}

	if len(ctx.servicesByModule) != expectedCount {
		return fmt.Errorf("expected %d services for %s, got %d", expectedCount, moduleName, len(ctx.servicesByModule))
	}

	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) theServiceNamesShouldReflectAnyConflictResolutionApplied() error {
	// All service names should be retrievable
	for _, serviceName := range ctx.servicesByModule {
		entry, exists := ctx.app.GetServiceEntry(serviceName)
		if !exists {
			return fmt.Errorf("service entry for %s not found", serviceName)
		}

		// Check that we have both original and actual names
		if entry.OriginalName == "" || entry.ActualName == "" {
			return fmt.Errorf("service %s missing name information", serviceName)
		}
	}
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iHaveAServiceRegisteredByModule(serviceName, moduleName string) error {
	service := &EnhancedMockTestService{identifier: serviceName}
	module := &SingleServiceModule{
		name:        moduleName,
		serviceName: serviceName,
		service:     service,
	}

	ctx.modules[moduleName] = module
	ctx.services[serviceName] = service
	ctx.app.RegisterModule(module)

	// Initialize to register the service
	err := ctx.app.Init()
	ctx.lastError = err
	return err
}

func (ctx *EnhancedServiceRegistryBDDContext) iRetrieveTheServiceEntryByName() error {
	// Use the last registered service name
	var serviceName string
	for name := range ctx.services {
		serviceName = name
		break // Use the first service
	}

	entry, exists := ctx.app.GetServiceEntry(serviceName)
	ctx.serviceEntry = entry
	ctx.serviceEntryExists = exists
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) theEntryShouldContainTheOriginalNameActualNameModuleNameAndModuleType() error {
	if !ctx.serviceEntryExists {
		return fmt.Errorf("service entry does not exist")
	}

	if ctx.serviceEntry.OriginalName == "" {
		return fmt.Errorf("original name is empty")
	}
	if ctx.serviceEntry.ActualName == "" {
		return fmt.Errorf("actual name is empty")
	}
	if ctx.serviceEntry.ModuleName == "" {
		return fmt.Errorf("module name is empty")
	}
	if ctx.serviceEntry.ModuleType == nil {
		return fmt.Errorf("module type is nil")
	}

	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iShouldBeAbleToAccessTheActualServiceInstance() error {
	if !ctx.serviceEntryExists {
		return fmt.Errorf("service entry does not exist")
	}

	// Try to cast to expected type
	if _, ok := ctx.serviceEntry.Service.(*EnhancedMockTestService); !ok {
		return fmt.Errorf("service instance is not of expected type")
	}

	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iHaveServicesRegisteredThroughBothOldAndNewPatterns() error {
	// Register through old pattern (direct registry access)
	oldService := &EnhancedMockTestService{identifier: "oldPattern"}
	err := ctx.app.RegisterService("oldService", oldService)
	if err != nil {
		return err
	}

	// Register through new pattern (module-based)
	return ctx.iHaveAServiceRegisteredByModule("newService", "NewModule")
}

func (ctx *EnhancedServiceRegistryBDDContext) iAccessServicesThroughTheBackwardsCompatibleInterface() error {
	var oldService, newService EnhancedMockTestService

	errOld := ctx.app.GetService("oldService", &oldService)
	errNew := ctx.app.GetService("newService", &newService)

	if errOld != nil || errNew != nil {
		return fmt.Errorf("not all services accessible: old=%v, new=%v", errOld, errNew)
	}

	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) allServicesShouldBeAccessibleRegardlessOfRegistrationMethod() error {
	return ctx.iAccessServicesThroughTheBackwardsCompatibleInterface()
}

func (ctx *EnhancedServiceRegistryBDDContext) theServiceRegistryMapShouldContainAllServices() error {
	registry := ctx.app.SvcRegistry()

	if _, exists := registry["oldService"]; !exists {
		return fmt.Errorf("old service not found in registry map")
	}
	if _, exists := registry["newService"]; !exists {
		return fmt.Errorf("new service not found in registry map")
	}

	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iHaveThreeModulesProvidingServicesImplementingTheSameInterface() error {
	for i, moduleName := range []string{"ConflictModuleA", "ConflictModuleB", "ConflictModuleC"} {
		service := &EnhancedMockTestService{identifier: fmt.Sprintf("conflict%d", i+1)}
		module := &ConflictingServiceModule{
			name:        moduleName,
			serviceName: "conflictService", // Same name for all
			service:     service,
		}

		ctx.modules[moduleName] = module
		ctx.app.RegisterModule(module)
	}
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) allModulesAttemptToRegisterWithTheSameServiceName() error {
	// This is already handled in the previous step
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) theApplicationInitializes() error {
	err := ctx.app.Init()
	ctx.lastError = err
	return err
}

func (ctx *EnhancedServiceRegistryBDDContext) eachServiceShouldGetAUniqueNameThroughAutomaticConflictResolution() error {
	// Check that we can access services with resolved names
	var service1, service2, service3 EnhancedMockTestService

	err1 := ctx.app.GetService("conflictService", &service1)                 // First should keep original name
	err2 := ctx.app.GetService("conflictService.ConflictModuleB", &service2) // Second gets module suffix
	err3 := ctx.app.GetService("conflictService.ConflictModuleC", &service3) // Third gets module suffix

	if err1 != nil || err2 != nil || err3 != nil {
		return fmt.Errorf("not all conflict-resolved services accessible: %v, %v, %v", err1, err2, err3)
	}

	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) allServicesShouldBeDiscoverableByInterface() error {
	interfaceType := reflect.TypeOf((*TestServiceInterface)(nil)).Elem()
	services := ctx.app.GetServicesByInterface(interfaceType)

	if len(services) != 3 {
		return fmt.Errorf("expected 3 services discoverable by interface, got %d", len(services))
	}

	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) iHaveAModuleThatProvidesMultipleServicesWithPotentialNameConflicts() error {
	services := []ServiceProvider{
		{Name: "commonService", Instance: &EnhancedMockTestService{identifier: "service1"}},
		{Name: "commonService.extra", Instance: &EnhancedMockTestService{identifier: "service2"}},
		{Name: "commonService", Instance: &EnhancedMockTestService{identifier: "service3"}}, // Conflict with first
	}

	module := &MultiServiceModule{
		name:     "ConflictingModule",
		services: services,
	}

	ctx.modules["ConflictingModule"] = module
	ctx.app.RegisterModule(module)
	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) theModuleRegistersServicesWithSimilarNames() error {
	return ctx.theApplicationInitializes()
}

func (ctx *EnhancedServiceRegistryBDDContext) theEnhancedRegistryShouldResolveAllConflictsIntelligently() error {
	// Try to access services - the registry should have resolved conflicts
	var service1, service2, service3 EnhancedMockTestService

	// First service should keep original name
	err1 := ctx.app.GetService("commonService", &service1)
	if err1 != nil {
		return fmt.Errorf("first service not accessible: %v", err1)
	}

	// Second service should keep its original name (no conflict)
	err2 := ctx.app.GetService("commonService.extra", &service2)
	if err2 != nil {
		return fmt.Errorf("second service not accessible: %v", err2)
	}

	// Third service should get conflict resolution (likely a counter)
	err3 := ctx.app.GetService("commonService.2", &service3)
	if err3 != nil {
		return fmt.Errorf("third service not accessible with resolved name: %v", err3)
	}

	return nil
}

func (ctx *EnhancedServiceRegistryBDDContext) eachServiceShouldMaintainItsModuleAssociation() error {
	services := ctx.app.GetServicesByModule("ConflictingModule")

	if len(services) != 3 {
		return fmt.Errorf("expected 3 services for ConflictingModule, got %d", len(services))
	}

	// Check that all services have proper module association
	for _, serviceName := range services {
		entry, exists := ctx.app.GetServiceEntry(serviceName)
		if !exists {
			return fmt.Errorf("service entry for %s not found", serviceName)
		}

		if entry.ModuleName != "ConflictingModule" {
			return fmt.Errorf("service %s has wrong module name: %s", serviceName, entry.ModuleName)
		}
	}

	return nil
}

// Test function for BDD scenarios
func TestEnhancedServiceRegistryBDD(t *testing.T) {
	testContext := &EnhancedServiceRegistryBDDContext{}

	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			// Background step
			ctx.Step(`^I have a modular application with enhanced service registry$`, testContext.iHaveAModularApplicationWithEnhancedServiceRegistry)

			// Service registration with module tracking
			ctx.Step(`^I have a module "([^"]*)" that provides a service "([^"]*)"$`, testContext.iHaveAModuleThatProvidesAService)
			ctx.Step(`^I register the module and initialize the application$`, testContext.iRegisterTheModuleAndInitializeTheApplication)
			ctx.Step(`^the service should be registered with module association$`, testContext.theServiceShouldBeRegisteredWithModuleAssociation)
			ctx.Step(`^I should be able to retrieve the service entry with module information$`, testContext.iShouldBeAbleToRetrieveTheServiceEntryWithModuleInformation)

			// Automatic conflict resolution
			ctx.Step(`^I have two modules "([^"]*)" and "([^"]*)" that both provide service "([^"]*)"$`, testContext.iHaveTwoModulesThatBothProvideService)
			ctx.Step(`^I register both modules and initialize the application$`, testContext.iRegisterBothModulesAndInitializeTheApplication)
			ctx.Step(`^the first module should keep the original service name$`, testContext.theFirstModuleShouldKeepTheOriginalServiceName)
			ctx.Step(`^the second module should get a module-suffixed name$`, testContext.theSecondModuleShouldGetAModuleSuffixedName)
			ctx.Step(`^both services should be accessible through their resolved names$`, testContext.bothServicesShouldBeAccessibleThroughTheirResolvedNames)

			// Interface-based service discovery
			ctx.Step(`^I have multiple modules providing services that implement "([^"]*)"$`, testContext.iHaveMultipleModulesProvidingServicesThatImplement)
			ctx.Step(`^I query for services by interface type$`, testContext.iQueryForServicesByInterfaceType)
			ctx.Step(`^I should get all services implementing that interface$`, testContext.iShouldGetAllServicesImplementingThatInterface)
			ctx.Step(`^each service should include its module association information$`, testContext.eachServiceShouldIncludeItsModuleAssociationInformation)

			// Get services by module
			ctx.Step(`^I have modules "([^"]*)", "([^"]*)", and "([^"]*)" providing different services$`, testContext.iHaveModulesProvidingDifferentServices)
			ctx.Step(`^I query for services provided by "([^"]*)"$`, testContext.iQueryForServicesProvidedBy)
			ctx.Step(`^I should get only the services registered by "([^"]*)"$`, testContext.iShouldGetOnlyTheServicesRegisteredBy)
			ctx.Step(`^the service names should reflect any conflict resolution applied$`, testContext.theServiceNamesShouldReflectAnyConflictResolutionApplied)

			// Service entry detailed information
			ctx.Step(`^I have a service "([^"]*)" registered by module "([^"]*)"$`, testContext.iHaveAServiceRegisteredByModule)
			ctx.Step(`^I retrieve the service entry by name$`, testContext.iRetrieveTheServiceEntryByName)
			ctx.Step(`^the entry should contain the original name, actual name, module name, and module type$`, testContext.theEntryShouldContainTheOriginalNameActualNameModuleNameAndModuleType)
			ctx.Step(`^I should be able to access the actual service instance$`, testContext.iShouldBeAbleToAccessTheActualServiceInstance)

			// Backwards compatibility
			ctx.Step(`^I have services registered through both old and new patterns$`, testContext.iHaveServicesRegisteredThroughBothOldAndNewPatterns)
			ctx.Step(`^I access services through the backwards-compatible interface$`, testContext.iAccessServicesThroughTheBackwardsCompatibleInterface)
			ctx.Step(`^all services should be accessible regardless of registration method$`, testContext.allServicesShouldBeAccessibleRegardlessOfRegistrationMethod)
			ctx.Step(`^the service registry map should contain all services$`, testContext.theServiceRegistryMapShouldContainAllServices)

			// Multiple interface implementations conflict resolution
			ctx.Step(`^I have three modules providing services implementing the same interface$`, testContext.iHaveThreeModulesProvidingServicesImplementingTheSameInterface)
			ctx.Step(`^all modules attempt to register with the same service name$`, testContext.allModulesAttemptToRegisterWithTheSameServiceName)
			ctx.Step(`^the application initializes$`, testContext.theApplicationInitializes)
			ctx.Step(`^each service should get a unique name through automatic conflict resolution$`, testContext.eachServiceShouldGetAUniqueNameThroughAutomaticConflictResolution)
			ctx.Step(`^all services should be discoverable by interface$`, testContext.allServicesShouldBeDiscoverableByInterface)

			// Enhanced service registry edge cases
			ctx.Step(`^I have a module that provides multiple services with potential name conflicts$`, testContext.iHaveAModuleThatProvidesMultipleServicesWithPotentialNameConflicts)
			ctx.Step(`^the module registers services with similar names$`, testContext.theModuleRegistersServicesWithSimilarNames)
			ctx.Step(`^the enhanced registry should resolve all conflicts intelligently$`, testContext.theEnhancedRegistryShouldResolveAllConflictsIntelligently)
			ctx.Step(`^each service should maintain its module association$`, testContext.eachServiceShouldMaintainItsModuleAssociation)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/enhanced_service_registry.feature"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run BDD tests")
	}
}
