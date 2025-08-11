Feature: LetsEncrypt Module
  As a developer using the Modular framework
  I want to use the LetsEncrypt module for automatic SSL certificate management
  So that I can secure my applications with automatically renewed certificates

  Background:
    Given I have a modular application with LetsEncrypt module configured

  Scenario: LetsEncrypt module initialization
    When the LetsEncrypt module is initialized
    Then the certificate service should be available
    And the module should be ready to manage certificates

  Scenario: HTTP-01 challenge configuration
    Given I have LetsEncrypt configured for HTTP-01 challenge
    When the module is initialized with HTTP challenge type
    Then the HTTP challenge handler should be configured
    And the module should be ready for domain validation

  Scenario: DNS-01 challenge configuration
    Given I have LetsEncrypt configured for DNS-01 challenge with Cloudflare
    When the module is initialized with DNS challenge type
    Then the DNS challenge handler should be configured
    And the module should be ready for DNS validation

  Scenario: Certificate storage configuration
    Given I have LetsEncrypt configured with custom certificate paths
    When the module initializes certificate storage
    Then the certificate and key directories should be created
    And the storage paths should be properly configured

  Scenario: Staging environment configuration
    Given I have LetsEncrypt configured for staging environment
    When the module is initialized
    Then the module should use the staging CA directory
    And certificate requests should use staging endpoints

  Scenario: Production environment configuration
    Given I have LetsEncrypt configured for production environment
    When the module is initialized
    Then the module should use the production CA directory
    And certificate requests should use production endpoints

  Scenario: Multiple domain certificate request
    Given I have LetsEncrypt configured for multiple domains
    When a certificate is requested for multiple domains
    Then the certificate should include all specified domains
    And the subject alternative names should be properly set

  Scenario: Certificate service dependency injection
    Given I have LetsEncrypt module registered
    When other modules request the certificate service
    Then they should receive the LetsEncrypt certificate service
    And the service should provide certificate retrieval functionality

  Scenario: Error handling for invalid configuration
    Given I have LetsEncrypt configured with invalid settings
    When the module is initialized
    Then appropriate configuration errors should be reported
    And the module should fail gracefully

  Scenario: Graceful module shutdown
    Given I have an active LetsEncrypt module
    When the module is stopped
    Then certificate renewal processes should be stopped
    And resources should be cleaned up properly