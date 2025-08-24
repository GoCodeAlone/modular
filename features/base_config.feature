Feature: Base Configuration Support
  As a developer using the Modular framework
  I want to use base configuration files with environment-specific overrides
  So that I can manage configuration for multiple environments efficiently

  Background:
    Given I have a base config structure with environment "prod"

  Scenario: Basic base config with environment overrides
    Given the base config contains:
      """
      app_name: "MyApp"
      environment: "base"
      database:
        host: "localhost"
        port: 5432
        name: "myapp"
        username: "user"
        password: "password"
      features:
        logging: true
        metrics: false
        caching: true
      """
    And the environment config contains:
      """
      environment: "production"
      database:
        host: "prod-db.example.com"
        password: "prod-secret"
      features:
        metrics: true
      """
    When I set the environment to "prod" and load the configuration
    Then the configuration loading should succeed
    And the configuration should have app name "MyApp"
    And the configuration should have environment "production"
    And the configuration should have database host "prod-db.example.com"
    And the configuration should have database password "prod-secret"
    And the feature "logging" should be enabled
    And the feature "metrics" should be enabled
    And the feature "caching" should be enabled

  Scenario: Base config only (no environment overrides)
    Given the base config contains:
      """
      app_name: "BaseApp"
      environment: "development"
      database:
        host: "localhost"
        port: 5432
      features:
        logging: true
        metrics: false
      """
    When I set the environment to "nonexistent" and load the configuration
    Then the configuration loading should succeed
    And the configuration should have app name "BaseApp"
    And the configuration should have environment "development"
    And the configuration should have database host "localhost"
    And the feature "logging" should be enabled
    And the feature "metrics" should be disabled

  Scenario: Environment overrides only (no base config)
    Given the environment config contains:
      """
      app_name: "ProdApp"
      environment: "production"
      database:
        host: "prod-db.example.com"
        port: 3306
      features:
        logging: false
        metrics: true
      """
    When I set the environment to "prod" and load the configuration
    Then the configuration loading should succeed
    And the configuration should have app name "ProdApp"
    And the configuration should have environment "production"
    And the configuration should have database host "prod-db.example.com"
    And the feature "logging" should be disabled
    And the feature "metrics" should be enabled

  Scenario: Deep merge of nested configurations
    Given the base config contains:
      """
      database:
        host: "base-host"
        port: 5432
        name: "base-db"
        username: "base-user"
        password: "base-pass"
      features:
        feature1: true
        feature2: false
        feature3: true
      """
    And the environment config contains:
      """
      database:
        host: "prod-host"
        password: "prod-pass"
      features:
        feature2: true
        feature4: true
      """
    When I set the environment to "prod" and load the configuration
    Then the configuration loading should succeed
    And the configuration should have database host "prod-host"
    And the configuration should have database password "prod-pass"
    And the feature "feature1" should be enabled
    And the feature "feature2" should be enabled
    And the feature "feature3" should be enabled
    And the feature "feature4" should be enabled