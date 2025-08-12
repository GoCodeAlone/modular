Feature: EventBus Module
  As a developer using the Modular framework
  I want to use the eventbus module for event-driven messaging
  So that I can build decoupled applications with publish-subscribe patterns

  Background:
    Given I have a modular application with eventbus module configured

  Scenario: EventBus module initialization
    When the eventbus module is initialized
    Then the eventbus service should be available
    And the service should be configured with default settings

  Scenario: Basic event publishing and subscribing
    Given I have an eventbus service available
    When I subscribe to topic "user.created" with a handler
    And I publish an event to topic "user.created" with payload "test-user"
    Then the handler should receive the event
    And the payload should match "test-user"

  Scenario: Event publishing to multiple subscribers
    Given I have an eventbus service available
    When I subscribe to topic "order.placed" with handler "handler1"
    And I subscribe to topic "order.placed" with handler "handler2"
    And I publish an event to topic "order.placed" with payload "order-123"
    Then both handlers should receive the event
    And the payload should match "order-123"

  Scenario: Wildcard topic subscriptions
    Given I have an eventbus service available
    When I subscribe to topic "user.*" with a handler
    And I publish an event to topic "user.created" with payload "user-1"
    And I publish an event to topic "user.updated" with payload "user-2"
    Then the handler should receive both events
    And the payloads should match "user-1" and "user-2"

  Scenario: Asynchronous event processing
    Given I have an eventbus service available
    When I subscribe asynchronously to topic "image.uploaded" with a handler
    And I publish an event to topic "image.uploaded" with payload "image-data"
    Then the handler should process the event asynchronously
    And the publishing should not block

  Scenario: Event subscription management
    Given I have an eventbus service available
    When I subscribe to topic "newsletter.sent" with a handler
    And I get the subscription details
    Then the subscription should have a unique ID
    And the subscription topic should be "newsletter.sent"
    And the subscription should not be async by default

  Scenario: Unsubscribing from events
    Given I have an eventbus service available
    When I subscribe to topic "payment.processed" with a handler
    And I unsubscribe from the topic
    And I publish an event to topic "payment.processed" with payload "payment-123"
    Then the handler should not receive the event

  Scenario: Active topics listing
    Given I have an eventbus service available
    When I subscribe to topic "task.started" with a handler
    And I subscribe to topic "task.completed" with a handler
    Then the active topics should include "task.started" and "task.completed"
    And the subscriber count for each topic should be 1

  Scenario: EventBus with memory engine
    Given I have an eventbus configuration with memory engine
    When the eventbus module is initialized
    Then the memory engine should be used
    And events should be processed in-memory

  Scenario: Event handler error handling
    Given I have an eventbus service available
    When I subscribe to topic "error.test" with a failing handler
    And I publish an event to topic "error.test" with payload "error-data"
    Then the eventbus should handle the error gracefully
    And the error should be logged appropriately

  Scenario: Event TTL and retention
    Given I have an eventbus configuration with event TTL
    When events are published with TTL settings
    Then old events should be cleaned up automatically
    And the retention policy should be respected

  Scenario: EventBus shutdown and cleanup
    Given I have a running eventbus service
    When the eventbus is stopped
    Then all subscriptions should be cancelled
    And worker pools should be shut down gracefully
    And no memory leaks should occur

  # Multi-Engine Scenarios
  Scenario: Multi-engine configuration
    Given I have a multi-engine eventbus configuration with memory and custom engines
    When the eventbus module is initialized
    Then both engines should be available
    And the engine router should be configured correctly

  Scenario: Topic routing between engines  
    Given I have a multi-engine eventbus with topic routing configured
    When I publish an event to topic "user.created" 
    And I publish an event to topic "analytics.pageview"
    Then "user.created" should be routed to the memory engine
    And "analytics.pageview" should be routed to the custom engine

  Scenario: Custom engine registration
    Given I register a custom engine type "testengine"
    When I configure eventbus to use the custom engine
    Then the custom engine should be used for event processing
    And events should be handled by the custom implementation

  Scenario: Engine-specific configuration
    Given I have engines with different configuration settings
    When the eventbus is initialized with engine-specific configs
    Then each engine should use its specific configuration
    And engine behavior should reflect the configured settings

  Scenario: Multi-engine subscription management
    Given I have multiple engines running
    When I subscribe to topics on different engines
    And I check subscription counts across engines
    Then each engine should report its subscriptions correctly
    And total subscriber counts should aggregate across engines

  Scenario: Routing rule evaluation
    Given I have routing rules with wildcards and exact matches
    When I publish events with various topic patterns
    Then events should be routed according to the first matching rule
    And fallback routing should work for unmatched topics

  Scenario: Multi-engine error handling
    Given I have multiple engines configured
    When one engine encounters an error
    Then other engines should continue operating normally
    And the error should be isolated to the failing engine

  Scenario: Engine router topic discovery  
    Given I have subscriptions across multiple engines
    When I query for active topics
    Then all topics from all engines should be returned
    And subscriber counts should be aggregated correctly

  # Tenant Isolation Scenarios
  Scenario: Tenant-aware event routing
    Given I have a multi-tenant eventbus configuration
    When tenant "tenant1" publishes an event to "user.login"
    And tenant "tenant2" subscribes to "user.login"
    Then "tenant2" should not receive "tenant1" events
    And event isolation should be maintained between tenants

  Scenario: Tenant-specific engine routing
    Given I have tenant-aware routing configuration
    When "tenant1" is configured to use memory engine
    And "tenant2" is configured to use custom engine
    Then events from each tenant should use their assigned engine
    And tenant configurations should not interfere with each other