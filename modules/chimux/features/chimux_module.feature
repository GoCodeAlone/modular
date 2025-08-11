Feature: ChiMux Module
  As a developer using the Modular framework
  I want to use the chimux module for HTTP routing
  So that I can build web applications with flexible routing and middleware

  Background:
    Given I have a modular application with chimux module configured

  Scenario: ChiMux module initialization
    When the chimux module is initialized
    Then the router service should be available
    And the Chi router service should be available
    And the basic router service should be available

  Scenario: Register basic routes
    Given I have a router service available
    When I register a GET route "/test" with handler
    And I register a POST route "/data" with handler
    Then the routes should be registered successfully

  Scenario: CORS configuration
    Given I have a chimux configuration with CORS settings
    When the chimux module is initialized with CORS
    Then the CORS middleware should be configured
    And allowed origins should include the configured values

  Scenario: Middleware discovery and application
    Given I have middleware provider services available
    When the chimux module discovers middleware providers
    Then the middleware should be applied to the router
    And requests should pass through the middleware chain

  Scenario: Base path configuration
    Given I have a chimux configuration with base path "/api/v1"
    When I register routes with the configured base path
    Then all routes should be prefixed with the base path

  Scenario: Request timeout configuration
    Given I have a chimux configuration with timeout settings
    When the chimux module applies timeout configuration
    Then the timeout middleware should be configured
    And requests should respect the timeout settings

  Scenario: Chi router advanced features
    Given I have access to the Chi router service
    When I use Chi-specific routing features
    Then I should be able to create route groups
    And I should be able to mount sub-routers

  Scenario: Multiple HTTP methods support
    Given I have a basic router service available
    When I register routes for different HTTP methods
    Then GET routes should be handled correctly
    And POST routes should be handled correctly
    And PUT routes should be handled correctly
    And DELETE routes should be handled correctly

  Scenario: Route parameters and wildcards
    Given I have a router service available
    When I register parameterized routes
    Then route parameters should be extracted correctly
    And wildcard routes should match appropriately

  Scenario: Middleware ordering
    Given I have multiple middleware providers
    When middleware is applied to the router
    Then middleware should be applied in the correct order
    And request processing should follow the middleware chain