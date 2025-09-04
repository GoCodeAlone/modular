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

  Scenario: Emit events during LetsEncrypt lifecycle
    Given I have a LetsEncrypt module with event observation enabled
    When the LetsEncrypt module starts
    Then a service started event should be emitted
    And the event should contain service configuration details
    When the LetsEncrypt module stops
    Then a service stopped event should be emitted
    And a module stopped event should be emitted

  Scenario: Emit events during certificate lifecycle
    Given I have a LetsEncrypt module with event observation enabled
    When a certificate is requested for domains
    Then a certificate requested event should be emitted
    And the event should contain domain information
    When the certificate is successfully issued
    Then a certificate issued event should be emitted
    And the event should contain domain details

  Scenario: Emit events during certificate renewal
    Given I have a LetsEncrypt module with event observation enabled
    And I have existing certificates that need renewal
    When certificates are renewed
    Then certificate renewed events should be emitted
    And the events should contain renewal details

  Scenario: Emit events during ACME protocol operations
    Given I have a LetsEncrypt module with event observation enabled
    When ACME challenges are processed
    Then ACME challenge events should be emitted
    When ACME authorization is completed
    Then ACME authorization events should be emitted
    When ACME orders are processed
    Then ACME order events should be emitted

  Scenario: Emit events during certificate storage operations
    Given I have a LetsEncrypt module with event observation enabled
    When certificates are stored to disk
    Then storage write events should be emitted
    When certificates are read from storage
    Then storage read events should be emitted
    When storage errors occur
    Then storage error events should be emitted

  Scenario: Emit events during configuration loading
    Given I have a LetsEncrypt module with event observation enabled
    When the module configuration is loaded
    Then a config loaded event should be emitted
    And the event should contain configuration details
    When the configuration is validated
    Then a config validated event should be emitted

  Scenario: Emit events for certificate expiry monitoring
    Given I have a LetsEncrypt module with event observation enabled
    Given I have certificates approaching expiry
    When certificate expiry monitoring runs
    Then certificate expiring events should be emitted
    And the events should contain expiry details
    When certificates have expired
    Then certificate expired events should be emitted

  Scenario: Emit events during certificate revocation
    Given I have a LetsEncrypt module with event observation enabled
    When a certificate is revoked
    Then a certificate revoked event should be emitted
    And the event should contain revocation reason

  Scenario: Emit events during module startup
    Given I have a LetsEncrypt module with event observation enabled
    When the module starts up
    Then a module started event should be emitted
    And the event should contain module information

  Scenario: Emit events for error and warning conditions
    Given I have a LetsEncrypt module with event observation enabled
    When an error condition occurs
    Then an error event should be emitted
    And the event should contain error details
    When a warning condition occurs  
    Then a warning event should be emitted
    And the event should contain warning details

  Scenario: Rate limit warning event
    Given I have a LetsEncrypt module with event observation enabled
    When certificate issuance hits rate limits
    Then a warning event should be emitted
    And the event should contain warning details

  Scenario: Per-domain renewal tracking
    Given I have a LetsEncrypt module with event observation enabled
    And I have existing certificates that need renewal
    When certificates are renewed
    Then certificate renewed events should be emitted
    And there should be a renewal event for each domain

  Scenario: Mixed challenge reconfiguration
    Given I have LetsEncrypt configured for HTTP-01 challenge
    When the module is initialized with HTTP challenge type
    And I reconfigure to DNS-01 challenge with Cloudflare
    Then the DNS challenge handler should be configured
    And the module should be ready for DNS validation

  Scenario: Certificate request failure path
    Given I have a LetsEncrypt module with event observation enabled
    When a certificate request fails
    Then an error event should be emitted
    And the event should contain error details

  Scenario: Event emission coverage
    Given I have a LetsEncrypt module with event observation enabled
    When a certificate is requested for domains
    And the certificate is successfully issued
    And certificates are renewed
    And ACME challenges are processed
    And ACME authorization is completed
    And ACME orders are processed
    And certificates are stored to disk
    And certificates are read from storage
    And storage errors occur
    And the module configuration is loaded
    And the configuration is validated
    And I have certificates approaching expiry
    And certificate expiry monitoring runs
    And certificates have expired
    And a certificate is revoked
  And the LetsEncrypt module starts
  And the LetsEncrypt module stops
    And the module starts up
    And an error condition occurs
    And a warning condition occurs
    Then all registered LetsEncrypt events should have been emitted during testing