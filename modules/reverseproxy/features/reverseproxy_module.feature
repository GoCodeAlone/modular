Feature: Reverse Proxy Module
  As a developer using the Modular framework
  I want to use the reverse proxy module for load balancing and request routing
  So that I can distribute traffic across multiple backend services

  Background:
    Given I have a modular application with reverse proxy module configured

  Scenario: Reverse proxy module initialization
    When the reverse proxy module is initialized
    Then the proxy service should be available
    And the module should be ready to route requests

  Scenario: Single backend proxy routing
    Given I have a reverse proxy configured with a single backend
    When I send a request to the proxy
    Then the request should be forwarded to the backend
    And the response should be returned to the client

  Scenario: Multiple backend load balancing
    Given I have a reverse proxy configured with multiple backends
    When I send multiple requests to the proxy
    Then requests should be distributed across all backends
    And load balancing should be applied

  Scenario: Backend health checking
    Given I have a reverse proxy with health checks enabled
    When a backend becomes unavailable
    Then the proxy should detect the failure
    And route traffic only to healthy backends

  Scenario: Circuit breaker functionality
    Given I have a reverse proxy with circuit breaker enabled
    When a backend fails repeatedly
    Then the circuit breaker should open
    And requests should be handled gracefully

  Scenario: Response caching
    Given I have a reverse proxy with caching enabled
    When I send the same request multiple times
    Then the first request should hit the backend
    And subsequent requests should be served from cache

  Scenario: Tenant-aware routing
    Given I have a tenant-aware reverse proxy configured
    When I send requests with different tenant contexts
    Then requests should be routed based on tenant configuration
    And tenant isolation should be maintained

  Scenario: Composite response handling
    Given I have a reverse proxy configured for composite responses
    When I send a request that requires multiple backend calls
    Then the proxy should call all required backends
    And combine the responses into a single response

  Scenario: Request transformation
    Given I have a reverse proxy with request transformation configured
    When I send a request to the proxy
    Then the request should be transformed before forwarding
    And the backend should receive the transformed request

  Scenario: Graceful shutdown
    Given I have an active reverse proxy with ongoing requests
    When the module is stopped
    Then ongoing requests should be completed
    And new requests should be rejected gracefully

  Scenario: Emit events during proxy lifecycle
    Given I have a reverse proxy with event observation enabled
    When the reverse proxy module starts
    Then a proxy created event should be emitted
    And a proxy started event should be emitted
    And a module started event should be emitted
    And the events should contain proxy configuration details
    When the reverse proxy module stops
    Then a proxy stopped event should be emitted
    And a module stopped event should be emitted

  Scenario: Emit events during request routing
    Given I have a reverse proxy with event observation enabled
    And I have a backend service configured
    When I send a request to the reverse proxy
    Then a request received event should be emitted
    And the event should contain request details
    When the request is successfully proxied to the backend
    Then a request proxied event should be emitted
    And the event should contain backend and response details

  Scenario: Emit events during request failures
    Given I have a reverse proxy with event observation enabled
    And I have an unavailable backend service configured
    When I send a request to the reverse proxy
    Then a request received event should be emitted
    When the request fails to reach the backend
    Then a request failed event should be emitted
    And the event should contain error details

  Scenario: Emit events during backend health management
    Given I have a reverse proxy with event observation enabled
    And I have backends with health checking enabled
    When a backend becomes healthy
    Then a backend healthy event should be emitted
    And the event should contain backend health details
    When a backend becomes unhealthy
    Then a backend unhealthy event should be emitted
    And the event should contain health failure details

  Scenario: Emit events during backend management
    Given I have a reverse proxy with event observation enabled
    When a new backend is added to the configuration
    Then a backend added event should be emitted
    And the event should contain backend configuration
    When a backend is removed from the configuration
    Then a backend removed event should be emitted
    And the event should contain removal details

  Scenario: Emit events during load balancing decisions
    Given I have a reverse proxy with event observation enabled
    And I have multiple backends configured
    When load balancing decisions are made
    Then load balance decision events should be emitted
    And the events should contain selected backend information
    When round-robin load balancing is used
    Then round-robin events should be emitted
    And the events should contain rotation details

  Scenario: Emit events during circuit breaker operations
    Given I have a reverse proxy with event observation enabled
    And I have circuit breaker enabled for backends
    When a circuit breaker opens due to failures
    Then a circuit breaker open event should be emitted
    And the event should contain failure threshold details
    When a circuit breaker transitions to half-open
    Then a circuit breaker half-open event should be emitted
    When a circuit breaker closes after recovery
    Then a circuit breaker closed event should be emitted

  Scenario: Health check DNS resolution
    Given I have a reverse proxy with health checks configured for DNS resolution
    When health checks are performed
    Then DNS resolution should be validated
    And unhealthy backends should be marked as down

  Scenario: Custom health endpoints per backend
    Given I have a reverse proxy with custom health endpoints configured
    When health checks are performed on different backends
    Then each backend should be checked at its custom endpoint
    And health status should be properly tracked

  Scenario: Per-backend health check configuration
    Given I have a reverse proxy with per-backend health check settings
    When health checks run with different intervals and timeouts
    Then each backend should use its specific configuration
    And health check timing should be respected

  Scenario: Recent request threshold behavior
    Given I have a reverse proxy with recent request threshold configured
    When requests are made within the threshold window
    Then health checks should be skipped for recently used backends
    And health checks should resume after threshold expires

  Scenario: Health check expected status codes
    Given I have a reverse proxy with custom expected status codes
    When backends return various HTTP status codes
    Then only configured status codes should be considered healthy
    And other status codes should mark backends as unhealthy

  Scenario: Metrics collection enabled
    Given I have a reverse proxy with metrics enabled
    When requests are processed through the proxy
    Then metrics should be collected and exposed
    And metric values should reflect proxy activity

  Scenario: Metrics endpoint configuration
    Given I have a reverse proxy with custom metrics endpoint
    When the metrics endpoint is accessed
    Then metrics should be available at the configured path
    And metrics data should be properly formatted

  Scenario: Debug endpoints functionality
    Given I have a reverse proxy with debug endpoints enabled
    When debug endpoints are accessed
    Then configuration information should be exposed
    And debug data should be properly formatted

  Scenario: Debug info endpoint
    Given I have a reverse proxy with debug endpoints enabled
    When the debug info endpoint is accessed
    Then general proxy information should be returned
    And configuration details should be included

  Scenario: Debug backends endpoint
    Given I have a reverse proxy with debug endpoints enabled
    When the debug backends endpoint is accessed
    Then backend configuration should be returned
    And backend health status should be included

  Scenario: Debug feature flags endpoint
    Given I have a reverse proxy with debug endpoints and feature flags enabled
    When the debug flags endpoint is accessed
    Then current feature flag states should be returned
    And tenant-specific flags should be included

  Scenario: Debug circuit breakers endpoint
    Given I have a reverse proxy with debug endpoints and circuit breakers enabled
    When the debug circuit breakers endpoint is accessed
    Then circuit breaker states should be returned
    And circuit breaker metrics should be included

  Scenario: Debug health checks endpoint
    Given I have a reverse proxy with debug endpoints and health checks enabled
    When the debug health checks endpoint is accessed
    Then health check status should be returned
    And health check history should be included

  Scenario: Route-level feature flags with alternatives
    Given I have a reverse proxy with route-level feature flags configured
    When requests are made to flagged routes
    Then feature flags should control routing decisions
    And alternative backends should be used when flags are disabled

  Scenario: Backend-level feature flags with alternatives
    Given I have a reverse proxy with backend-level feature flags configured
    When requests target flagged backends
    Then feature flags should control backend selection
    And alternative backends should be used when flags are disabled

  Scenario: Composite route feature flags
    Given I have a reverse proxy with composite route feature flags configured
    When requests are made to composite routes
    Then feature flags should control route availability
    And alternative single backends should be used when disabled

  Scenario: Tenant-specific feature flags
    Given I have a reverse proxy with tenant-specific feature flags configured
    When requests are made with different tenant contexts
    Then feature flags should be evaluated per tenant
    And tenant-specific routing should be applied

  Scenario: Dry run mode with response comparison
    Given I have a reverse proxy with dry run mode enabled
    When requests are processed in dry run mode
    Then requests should be sent to both primary and comparison backends
    And responses should be compared and logged

  Scenario: Dry run with feature flags
    Given I have a reverse proxy with dry run mode and feature flags configured
    When feature flags control routing in dry run mode
    Then appropriate backends should be compared based on flag state
    And comparison results should be logged with flag context

  Scenario: Per-backend path rewriting
    Given I have a reverse proxy with per-backend path rewriting configured
    When requests are routed to different backends
    Then paths should be rewritten according to backend configuration
    And original paths should be properly transformed

  Scenario: Per-endpoint path rewriting
    Given I have a reverse proxy with per-endpoint path rewriting configured
    When requests match specific endpoint patterns
    Then paths should be rewritten according to endpoint configuration
    And endpoint-specific rules should override backend rules

  Scenario: Hostname handling modes
    Given I have a reverse proxy with different hostname handling modes configured
    When requests are forwarded to backends
    Then Host headers should be handled according to configuration
    And custom hostnames should be applied when specified

  Scenario: Header set and remove operations
    Given I have a reverse proxy with header rewriting configured
    When requests are processed through the proxy
    Then specified headers should be added or modified
    And specified headers should be removed from requests

  Scenario: Per-backend circuit breaker configuration
    Given I have a reverse proxy with per-backend circuit breaker settings
    When different backends fail at different rates
    Then each backend should use its specific circuit breaker configuration
    And circuit breaker behavior should be isolated per backend

  Scenario: Circuit breaker half-open state
    Given I have a reverse proxy with circuit breakers in half-open state
    When test requests are sent through half-open circuits
    Then limited requests should be allowed through
    And circuit state should transition based on results

  Scenario: Cache TTL behavior
    Given I have a reverse proxy with specific cache TTL configured
    When cached responses age beyond TTL
    Then expired cache entries should be evicted
    And fresh requests should hit backends after expiration

  Scenario: Global request timeout
    Given I have a reverse proxy with global request timeout configured
    When backend requests exceed the timeout
    Then requests should be terminated after timeout
    And appropriate error responses should be returned

  Scenario: Per-route timeout overrides
    Given I have a reverse proxy with per-route timeout overrides configured
    When requests are made to routes with specific timeouts
    Then route-specific timeouts should override global settings
    And timeout behavior should be applied per route

  Scenario: Backend error response handling
    Given I have a reverse proxy configured for error handling
    When backends return error responses
    Then error responses should be properly handled
    And appropriate client responses should be returned

  Scenario: Connection failure handling
    Given I have a reverse proxy configured for connection failure handling
    When backend connections fail
    Then connection failures should be handled gracefully
    And circuit breakers should respond appropriately
