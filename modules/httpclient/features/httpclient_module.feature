Feature: HTTPClient Module
  As a developer using the Modular framework
  I want to use the httpclient module for making HTTP requests
  So that I can interact with external APIs with reliable HTTP client functionality

  Background:
    Given I have a modular application with httpclient module configured

  Scenario: HTTPClient module initialization
    When the httpclient module is initialized
    Then the httpclient service should be available
    And the client should be configured with default settings

  Scenario: Basic HTTP GET request
    Given I have an httpclient service available
    When I make a GET request to a test endpoint
    Then the request should be successful
    And the response should be received

  Scenario: HTTP client with custom timeouts
    Given I have an httpclient configuration with custom timeouts
    When the httpclient module is initialized
    Then the client should have the configured request timeout
    And the client should have the configured TLS timeout
    And the client should have the configured idle connection timeout

  Scenario: HTTP client with connection pooling
    Given I have an httpclient configuration with connection pooling
    When the httpclient module is initialized
    Then the client should have the configured max idle connections
    And the client should have the configured max idle connections per host
    And connection reuse should be enabled

  Scenario: HTTP POST request with data
    Given I have an httpclient service available
    When I make a POST request with JSON data
    Then the request should be successful
    And the request body should be sent correctly

  Scenario: HTTP client with custom headers
    Given I have an httpclient service available
    When I set a request modifier for custom headers
    And I make a request with the modified client
    Then the custom headers should be included in the request

  Scenario: HTTP client with authentication
    Given I have an httpclient service available
    When I set a request modifier for authentication
    And I make a request to a protected endpoint
    Then the authentication headers should be included
    And the request should be authenticated

  Scenario: HTTP client with verbose logging
    Given I have an httpclient configuration with verbose logging enabled
    When the httpclient module is initialized
    And I make HTTP requests
    Then request and response details should be logged
    And the logs should include headers and timing information

  Scenario: HTTP client with timeout handling
    Given I have an httpclient service available
    When I make a request with a custom timeout
    And the request takes longer than the timeout
    Then the request should timeout appropriately
    And a timeout error should be returned

  Scenario: HTTP client with compression
    Given I have an httpclient configuration with compression enabled
    When the httpclient module is initialized
    And I make requests to endpoints that support compression
    Then the client should handle gzip compression
    And compressed responses should be automatically decompressed

  Scenario: HTTP client with keep-alive disabled
    Given I have an httpclient configuration with keep-alive disabled
    When the httpclient module is initialized
    Then each request should use a new connection
    And connections should not be reused

  Scenario: HTTP client error handling
    Given I have an httpclient service available
    When I make a request to an invalid endpoint
    Then an appropriate error should be returned
    And the error should contain meaningful information

  Scenario: HTTP client with retry logic
    Given I have an httpclient service available
    When I make a request that initially fails
    And retry logic is configured
    Then the client should retry the request
    And eventually succeed or return the final error

  Scenario: Emit events during httpclient lifecycle
    Given I have an httpclient with event observation enabled
    When the httpclient module starts
    Then a client started event should be emitted
    And a config loaded event should be emitted
    And the events should contain client configuration details

  Scenario: Emit events during request modifier management
    Given I have an httpclient with event observation enabled
    When I add a request modifier
    Then a modifier added event should be emitted
    When I remove a request modifier
    Then a modifier removed event should be emitted

  Scenario: Emit events during timeout changes
    Given I have an httpclient with event observation enabled
    When I change the client timeout
    Then a timeout changed event should be emitted
    And the event should contain the new timeout value