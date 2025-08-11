Feature: Event Logger Module
  As a developer using the Modular framework
  I want to use the event logger module for structured event logging
  So that I can track and monitor application events across multiple output targets

  Background:
    Given I have a modular application with event logger module configured

  Scenario: Event logger module initialization
    When the event logger module is initialized
    Then the event logger service should be available
    And the module should register as an observer

  Scenario: Log events to console output
    Given I have an event logger with console output configured
    When I emit a test event with type "test.event" and data "test-data"
    Then the event should be logged to console output
    And the log entry should contain the event type and data

  Scenario: Log events to file output
    Given I have an event logger with file output configured
    When I emit multiple events with different types
    Then all events should be logged to the file
    And the file should contain structured log entries

  Scenario: Filter events by type
    Given I have an event logger with event type filters configured
    When I emit events with different types
    Then only filtered event types should be logged
    And non-matching events should be ignored

  Scenario: Log level filtering
    Given I have an event logger with INFO log level configured
    When I emit events with different log levels
    Then only INFO and higher level events should be logged
    And DEBUG events should be filtered out

  Scenario: Event buffer management
    Given I have an event logger with buffer size configured
    When I emit more events than the buffer can hold
    Then older events should be dropped
    And buffer overflow should be handled gracefully

  Scenario: Multiple output targets
    Given I have an event logger with multiple output targets configured
    When I emit an event
    Then the event should be logged to all configured targets
    And each target should receive the same event data

  Scenario: Event metadata inclusion
    Given I have an event logger with metadata inclusion enabled
    When I emit an event with metadata
    Then the logged event should include the metadata
    And CloudEvent fields should be preserved

  Scenario: Graceful shutdown with event flushing
    Given I have an event logger with pending events
    When the module is stopped
    Then all pending events should be flushed
    And output targets should be closed properly

  Scenario: Error handling for output target failures
    Given I have an event logger with faulty output target
    When I emit events
    Then errors should be handled gracefully
    And other output targets should continue working