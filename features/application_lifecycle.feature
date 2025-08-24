Feature: Application Lifecycle Management
  As a developer using the Modular framework
  I want to manage application lifecycle (initialization, startup, shutdown)
  So that I can build robust modular applications

  Background:
    Given I have a new modular application
    And I have a logger configured

  Scenario: Create a new application
    When I create a new standard application
    Then the application should be properly initialized
    And the service registry should be empty
    And the module registry should be empty

  Scenario: Register a simple module
    Given I have a simple test module
    When I register the module with the application
    Then the module should be registered in the module registry
    And the module should not be initialized yet

  Scenario: Initialize application with modules
    Given I have registered a simple test module
    When I initialize the application
    Then the module should be initialized
    And any services provided by the module should be registered

  Scenario: Initialize application with module dependencies
    Given I have a provider module that provides a service
    And I have a consumer module that depends on that service
    When I register both modules with the application
    And I initialize the application
    Then both modules should be initialized in dependency order
    And the consumer module should receive the service from the provider

  Scenario: Start and stop application with startable modules
    Given I have a startable test module
    And the module is registered and initialized
    When I start the application
    Then the startable module should be started
    When I stop the application
    Then the startable module should be stopped

  Scenario: Handle module initialization errors
    Given I have a module that fails during initialization
    When I try to initialize the application
    Then the initialization should fail
    And the error should include details about which module failed

  Scenario: Handle circular dependencies
    Given I have two modules with circular dependencies
    When I try to initialize the application
    Then the initialization should fail
    And the error should indicate circular dependency