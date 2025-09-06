Feature: Feature Flag Aggregator
  As a developer using the reverse proxy module with multiple feature flag evaluators
  I want the aggregator to discover and coordinate evaluators by interface matching
  So that I can have a flexible, priority-based feature flag evaluation system

  Background:
    Given I have a modular application with reverse proxy module configured
    And feature flags are enabled

  Scenario: Interface-based evaluator discovery
    Given I have multiple evaluators implementing FeatureFlagEvaluator with different service names
    And the evaluators are registered with names "customEvaluator", "remoteFlags", and "rules-engine"
    When the feature flag aggregator discovers evaluators
    Then all evaluators should be discovered regardless of their service names
    And each evaluator should be assigned a unique internal name

  Scenario: Weight-based priority ordering
    Given I have three evaluators with weights 10, 50, and 100
    When a feature flag is evaluated
    Then evaluators should be called in ascending weight order
    And the first evaluator returning a decision should determine the result

  Scenario: Automatic name conflict resolution
    Given I have two evaluators registered with the same service name "evaluator"
    When the aggregator discovers evaluators
    Then unique names should be automatically generated
    And both evaluators should be available for evaluation

  Scenario: Built-in file evaluator fallback
    Given I have external evaluators that return ErrNoDecision
    When a feature flag is evaluated
    Then the built-in file evaluator should be called as fallback
    And it should have the lowest priority (weight 1000)

  Scenario: External evaluator priority over file evaluator
    Given I have an external evaluator with weight 50
    And the external evaluator returns true for flag "test-flag"
    When I evaluate flag "test-flag"
    Then the external evaluator result should be returned
    And the file evaluator should not be called

  Scenario: ErrNoDecision handling
    Given I have two evaluators where the first returns ErrNoDecision
    And the second evaluator returns true for flag "test-flag"
    When I evaluate flag "test-flag"
    Then evaluation should continue to the second evaluator
    And the result should be true

  Scenario: ErrEvaluatorFatal handling
    Given I have two evaluators where the first returns ErrEvaluatorFatal
    When I evaluate a feature flag
    Then evaluation should stop immediately
    And no further evaluators should be called

  Scenario: Service registry discovery excludes aggregator itself
    Given the aggregator is registered as "featureFlagEvaluator"
    And external evaluators are also registered
    When evaluator discovery runs
    Then the aggregator should not discover itself
    And only external evaluators should be included

  Scenario: Multiple modules registering evaluators
    Given module A registers an evaluator as "moduleA.flags"
    And module B registers an evaluator as "moduleB.flags"
    When the aggregator discovers evaluators
    Then both evaluators should be discovered
    And their unique names should reflect their origins