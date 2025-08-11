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