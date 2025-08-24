Feature: JSONSchema Module
  As a developer using the Modular framework
  I want to use the jsonschema module for JSON Schema validation
  So that I can validate JSON data against predefined schemas

  Background:
    Given I have a modular application with jsonschema module configured

  Scenario: JSONSchema module initialization
    When the jsonschema module is initialized
    Then the jsonschema service should be available

  Scenario: Schema compilation from string
    Given I have a jsonschema service available
    When I compile a schema from a JSON string
    Then the schema should be compiled successfully

  Scenario: Valid JSON validation
    Given I have a jsonschema service available
    And I have a compiled schema for user data
    When I validate valid user JSON data
    Then the validation should pass

  Scenario: Invalid JSON validation
    Given I have a jsonschema service available
    And I have a compiled schema for user data
    When I validate invalid user JSON data
    Then the validation should fail with appropriate errors

  Scenario: Validation of different data types
    Given I have a jsonschema service available
    And I have a compiled schema
    When I validate data from bytes
    And I validate data from reader
    And I validate data from interface
    Then all validation methods should work correctly

  Scenario: Schema error handling
    Given I have a jsonschema service available
    When I try to compile an invalid schema
    Then a schema compilation error should be returned

  Scenario: Emit events during schema compilation
    Given I have a jsonschema service with event observation enabled
    When I compile a valid schema
    Then a schema compiled event should be emitted
    And the event should contain the source information
    When I try to compile an invalid schema
    Then a schema error event should be emitted

  Scenario: Emit events during JSON validation
    Given I have a jsonschema service with event observation enabled
    And I have a compiled schema for user data
    When I validate valid user JSON data with bytes method
    Then a validate bytes event should be emitted
    And a validation success event should be emitted
    When I validate invalid user JSON data with bytes method
    Then a validate bytes event should be emitted
    And a validation failed event should be emitted

  Scenario: Emit events for different validation methods
    Given I have a jsonschema service with event observation enabled
    And I have a compiled schema for user data
    When I validate data using the reader method
    Then a validate reader event should be emitted
    When I validate data using the interface method
    Then a validate interface event should be emitted