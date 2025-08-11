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