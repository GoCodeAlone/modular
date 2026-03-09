Feature: Aggregate Health Contract
  The health service must aggregate provider reports correctly.

  Scenario: Single healthy provider produces healthy status
    Given a health service with one healthy provider
    When health is checked
    Then the overall status should be "healthy"
    And readiness should be "healthy"

  Scenario: One unhealthy provider degrades overall health
    Given a health service with one healthy and one unhealthy provider
    When health is checked
    Then the overall health should be "unhealthy"
    And readiness should be "unhealthy"

  Scenario: Optional unhealthy provider does not affect readiness
    Given a health service with one healthy required and one unhealthy optional provider
    When health is checked
    Then the overall health should be "unhealthy"
    But readiness should be "healthy"

  Scenario: Provider panic is recovered gracefully
    Given a health service with a provider that panics
    When health is checked
    Then the panicking provider should report "unhealthy"
    And other providers should still be checked

  Scenario: Temporary error produces degraded status
    Given a health service with a provider returning a temporary error
    When health is checked
    Then the provider status should be "degraded"

  Scenario: Cache returns previous result within TTL
    Given a health service with a 100ms cache TTL
    And a healthy provider
    When health is checked twice within 50ms
    Then the provider should only be called once

  Scenario: Force refresh bypasses cache
    Given a health service with cached results
    When health is checked with force refresh
    Then the provider should be called again

  Scenario: Status change emits event
    Given a health service with a provider that transitions from healthy to unhealthy
    When health is checked after the transition
    Then a health status changed event should be emitted
