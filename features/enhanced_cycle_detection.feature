Feature: Enhanced Cycle Detection
  As a developer using the Modular framework
  I want enhanced cycle detection with clear error messages including interface dependencies
  So that I can easily understand and fix circular dependency issues

  Background:
    Given I have a modular application

  Scenario: Cycle detection with interface-based dependencies
    Given I have two modules with circular interface dependencies
    When I try to initialize the application
    Then the initialization should fail with a circular dependency error
    And the error message should include both module names
    And the error message should indicate interface-based dependencies
    And the error message should show the complete dependency cycle

  Scenario: Enhanced error message format
    Given I have modules A and B where A requires interface TestInterface and B provides TestInterface
    And module B also requires interface TestInterface creating a cycle
    When I try to initialize the application
    Then the error message should contain "cycle: moduleA →(interface:TestInterface) moduleB → moduleB →(interface:TestInterface) moduleA"
    And the error message should clearly show the interface causing the cycle

  Scenario: Mixed dependency types in cycle detection
    Given I have modules with both named service dependencies and interface dependencies
    And the dependencies form a circular chain
    When I try to initialize the application
    Then the error message should distinguish between interface and named dependencies
    And both dependency types should be included in the cycle description

  Scenario: No false positive cycle detection
    Given I have modules with valid linear dependencies
    When I initialize the application
    Then the initialization should succeed
    And no circular dependency error should be reported

  Scenario: Self-dependency detection
    Given I have a module that depends on a service it also provides
    When I try to initialize the application
    Then a self-dependency cycle should be detected
    And the error message should clearly indicate the self-dependency

  Scenario: Complex multi-module cycles
    Given I have modules A, B, and C where A depends on B, B depends on C, and C depends on A
    When I try to initialize the application
    Then the complete cycle path should be shown in the error message
    And all three modules should be mentioned in the cycle description

  Scenario: Interface name disambiguation
    Given I have multiple interfaces with similar names causing cycles
    When cycle detection runs
    Then interface names in error messages should be fully qualified
    And there should be no ambiguity about which interface caused the cycle