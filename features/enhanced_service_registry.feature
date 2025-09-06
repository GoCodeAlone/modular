Feature: Enhanced Service Registry API
  As a developer using the Modular framework
  I want to use the enhanced service registry with interface-based discovery and automatic conflict resolution
  So that I can build more flexible and maintainable modular applications

  Background:
    Given I have a modular application with enhanced service registry

  Scenario: Service registration with module tracking
    Given I have a module "TestModule" that provides a service "testService"
    When I register the module and initialize the application
    Then the service should be registered with module association
    And I should be able to retrieve the service entry with module information

  Scenario: Automatic conflict resolution with module suffixes
    Given I have two modules "ModuleA" and "ModuleB" that both provide service "duplicateService"
    When I register both modules and initialize the application
    Then the first module should keep the original service name
    And the second module should get a module-suffixed name
    And both services should be accessible through their resolved names

  Scenario: Interface-based service discovery
    Given I have multiple modules providing services that implement "TestInterface"
    When I query for services by interface type
    Then I should get all services implementing that interface
    And each service should include its module association information

  Scenario: Get services provided by specific module
    Given I have modules "ModuleA", "ModuleB", and "ModuleC" providing different services
    When I query for services provided by "ModuleB"
    Then I should get only the services registered by "ModuleB"
    And the service names should reflect any conflict resolution applied

  Scenario: Service entry with detailed information
    Given I have a service "detailedService" registered by module "DetailModule"
    When I retrieve the service entry by name
    Then the entry should contain the original name, actual name, module name, and module type
    And I should be able to access the actual service instance

  Scenario: Backwards compatibility with existing service registry
    Given I have services registered through both old and new patterns
    When I access services through the backwards-compatible interface
    Then all services should be accessible regardless of registration method
    And the service registry map should contain all services

  Scenario: Multiple interface implementations conflict resolution
    Given I have three modules providing services implementing the same interface
    And all modules attempt to register with the same service name
    When the application initializes
    Then each service should get a unique name through automatic conflict resolution
    And all services should be discoverable by interface

  Scenario: Enhanced service registry handles edge cases
    Given I have a module that provides multiple services with potential name conflicts
    When the module registers services with similar names
    Then the enhanced registry should resolve all conflicts intelligently
    And each service should maintain its module association