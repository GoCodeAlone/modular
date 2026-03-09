Feature: Dynamic Reload Contract
  Modules implementing Reloadable must follow these behavioral contracts.

  Scenario: Successful reload applies changes to all reloadable modules
    Given a reload orchestrator with 3 reloadable modules
    When a reload is requested with configuration changes
    Then all 3 modules should receive the changes
    And a reload completed event should be emitted

  Scenario: Module refusing reload is skipped
    Given a reload orchestrator with a module that cannot reload
    When a reload is requested
    Then the non-reloadable module should be skipped
    And other modules should still be reloaded

  Scenario: Partial failure triggers rollback
    Given a reload orchestrator with 3 modules where the second fails
    When a reload is requested
    Then the first module should be rolled back
    And a reload failed event should be emitted

  Scenario: Circuit breaker activates after repeated failures
    Given a reload orchestrator with a failing module
    When 3 consecutive reloads fail
    Then subsequent reload requests should be rejected
    And the circuit breaker should eventually reset

  Scenario: Empty diff produces noop event
    Given a reload orchestrator with reloadable modules
    When a reload is requested with no changes
    Then a reload noop event should be emitted
    And no modules should be called

  Scenario: Concurrent reload requests are serialized
    Given a reload orchestrator with reloadable modules
    When 10 reload requests are submitted concurrently
    Then all requests should be processed
    And no race conditions should occur
