Feature: Configuration Management
  As a developer using the Modular framework
  I want to manage configuration loading, validation, and feeding
  So that I can configure my modular applications properly

  Background:
    Given I have a new modular application
    And I have a logger configured

  Scenario: Register module configuration
    Given I have a module with configuration requirements
    When I register the module's configuration
    Then the configuration should be registered successfully
    And the configuration should be available for the module

  Scenario: Load configuration from environment variables
    Given I have environment variables set for module configuration
    And I have a module that requires configuration
    When I load configuration using environment feeder
    Then the module configuration should be populated from environment
    And the configuration should pass validation

  Scenario: Load configuration from YAML file
    Given I have a YAML configuration file
    And I have a module that requires configuration
    When I load configuration using YAML feeder
    Then the module configuration should be populated from YAML
    And the configuration should pass validation

  Scenario: Load configuration from JSON file
    Given I have a JSON configuration file
    And I have a module that requires configuration
    When I load configuration using JSON feeder
    Then the module configuration should be populated from JSON
    And the configuration should pass validation

  Scenario: Configuration validation with valid data
    Given I have a module with configuration validation rules
    And I have valid configuration data
    When I validate the configuration
    Then the validation should pass
    And no validation errors should be reported

  Scenario: Configuration validation with invalid data
    Given I have a module with configuration validation rules
    And I have invalid configuration data
    When I validate the configuration
    Then the validation should fail
    And appropriate validation errors should be reported

  Scenario: Configuration with default values
    Given I have a module with default configuration values
    When I load configuration without providing all values
    Then the missing values should use defaults
    And the configuration should be complete

  Scenario: Required configuration fields
    Given I have a module with required configuration fields
    When I load configuration without required values
    Then the configuration loading should fail
    And the error should indicate missing required fields

  Scenario: Configuration field tracking
    Given I have a module with configuration field tracking enabled
    When I load configuration from multiple sources
    Then I should be able to track which fields were set
    And I should know the source of each configuration value