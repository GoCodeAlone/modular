package modular

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/cucumber/godog"
)

// BDD Step implementations for service registry scenarios

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

func (ctx *EnhancedServiceRegistryBDDContext) iQueryForServicesByInterfaceType() error {
	// Initialize the application first
	err := ctx.app.Init()
	if err != nil {
		ctx.lastError = err
		return err
	}

	// Query for services implementing TestServiceInterface
	interfaceType := reflect.TypeFor[TestServiceInterface]()
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

func (ctx *EnhancedServiceRegistryBDDContext) allModulesAttemptToRegisterWithTheSameServiceName() error {
	// This is already handled in the previous step
	return nil
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
	interfaceType := reflect.TypeFor[TestServiceInterface]()
	services := ctx.app.GetServicesByInterface(interfaceType)

	if len(services) != 3 {
		return fmt.Errorf("expected 3 services discoverable by interface, got %d", len(services))
	}

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

	// Third service should get conflict resolution with module name
	err3 := ctx.app.GetService("commonService.ConflictingModule", &service3)
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
