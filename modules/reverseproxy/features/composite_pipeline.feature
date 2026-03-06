Feature: Pipeline and Fan-Out-Merge Composite Strategies
  As a developer building a multi-backend application
  I want to chain backend requests and merge responses by ID
  So that I can aggregate data from multiple services into unified responses

  Background:
    Given I have a modular application with reverse proxy module configured

  Scenario: Pipeline strategy chains requests through multiple backends
    Given I have a pipeline composite route with two backends
    When I send a request to the pipeline route
    Then the first backend should be called with the original request
    And the second backend should receive data derived from the first response
    And the final response should contain merged data from all stages

  Scenario: Fan-out-merge strategy merges responses by ID
    Given I have a fan-out-merge composite route with two backends
    When I send a request to the fan-out-merge route
    Then both backends should be called in parallel
    And the responses should be merged by matching IDs
    And items with matching ancillary data should be enriched

  Scenario: Pipeline with empty response using skip policy
    Given I have a pipeline route with skip-empty policy
    When I send a request and a backend returns an empty response
    Then the empty response should be excluded from the result
    And the non-empty responses should still be present

  Scenario: Fan-out-merge with empty response using fail policy
    Given I have a fan-out-merge route with fail-on-empty policy
    When I send a request and a backend returns an empty response
    Then the request should fail with a bad gateway error

  Scenario: Pipeline filters results using ancillary data
    Given I have a pipeline route that filters by ancillary backend data
    When I send a request to fetch filtered results
    Then only items matching the ancillary criteria should be returned
