Feature: Logger Decorator Pattern
  As a developer using the Modular framework
  I want to compose multiple logging behaviors using decorators
  So that I can create flexible and powerful logging systems

  Background:
    Given I have a new modular application
    And I have a test logger configured

  Scenario: Single decorator - prefix logger
    Given I have a base logger
    When I apply a prefix decorator with prefix "[MODULE]"
    And I log an info message "test message"
    Then the logged message should contain "[MODULE] test message"

  Scenario: Single decorator - value injection
    Given I have a base logger
    When I apply a value injection decorator with "service", "test-service" and "version", "1.0.0"
    And I log an info message "test message" with args "key", "value"
    Then the logged args should contain "service": "test-service"
    And the logged args should contain "version": "1.0.0"
    And the logged args should contain "key": "value"

  Scenario: Single decorator - dual writer
    Given I have a primary test logger
    And I have a secondary test logger
    When I apply a dual writer decorator
    And I log an info message "dual message"
    Then both the primary and secondary loggers should receive the message

  Scenario: Single decorator - filter logger
    Given I have a base logger
    When I apply a filter decorator that blocks messages containing "secret"
    And I log an info message "normal message"
    And I log an info message "contains secret data"
    Then the base logger should have received 1 message
    And the logged message should be "normal message"

  Scenario: Multiple decorators chained together
    Given I have a base logger
    When I apply a prefix decorator with prefix "[API]"
    And I apply a value injection decorator with "service", "api-service"
    And I apply a filter decorator that blocks debug level logs
    And I log an info message "processing request"
    And I log a debug message "debug details"
    Then the base logger should have received 1 message
    And the logged message should contain "[API] processing request"
    And the logged args should contain "service": "api-service"

  Scenario: Complex decorator chain - enterprise logging
    Given I have a primary test logger
    And I have an audit test logger
    When I apply a dual writer decorator
    And I apply a value injection decorator with "service", "payment-processor" and "instance", "prod-001"
    And I apply a prefix decorator with prefix "[PAYMENT]"
    And I apply a filter decorator that blocks messages containing "credit_card"
    And I log an info message "payment processed" with args "amount", "99.99"
    And I log an info message "credit_card validation failed"
    Then both the primary and audit loggers should have received 1 message
    And the logged message should contain "[PAYMENT] payment processed"
    And the logged args should contain "service": "payment-processor"
    And the logged args should contain "instance": "prod-001"
    And the logged args should contain "amount": "99.99"

  Scenario: SetLogger with decorators updates service registry
    Given I have an initial test logger in the application
    When I create a decorated logger with prefix "[NEW]"
    And I set the decorated logger on the application
    And I get the logger service from the application
    And I log an info message "service registry test"
    Then the logger service should be the decorated logger
    And the logged message should contain "[NEW] service registry test"

  Scenario: Level modifier decorator promotes warnings to errors
    Given I have a base logger
    When I apply a level modifier decorator that maps "warn" to "error"
    And I log a warn message "high memory usage"
    And I log an info message "normal operation"
    Then the base logger should have received 2 messages
    And the first message should have level "error"
    And the second message should have level "info"

  Scenario: Nested decorators preserve order
    Given I have a base logger
    When I apply a prefix decorator with prefix "[L1]"
    And I apply a value injection decorator with "level", "2"
    And I apply a prefix decorator with prefix "[L3]"
    And I log an info message "nested test"
    Then the logged message should be "[L1] [L3] nested test"
    And the logged args should contain "level": "2"

  Scenario: Filter decorator by key-value pairs
    Given I have a base logger
    When I apply a filter decorator that blocks logs where "env" equals "test"
    And I log an info message "production log" with args "env", "production"
    And I log an info message "test log" with args "env", "test"
    Then the base logger should have received 1 message
    And the logged message should be "production log"

  Scenario: Filter decorator by log level
    Given I have a base logger
    When I apply a filter decorator that allows only "info" and "error" levels
    And I log an info message "info message"
    And I log a debug message "debug message"
    And I log an error message "error message"
    And I log a warn message "warn message"
    Then the base logger should have received 2 messages
    And the messages should have levels "info", "error"