Feature: Cache Module
  As a developer using the Modular framework
  I want to use the cache module for data caching
  So that I can improve application performance with fast data access

  Background:
    Given I have a modular application with cache module configured

  Scenario: Cache module initialization
    When the cache module is initialized
    Then the cache service should be available

  Scenario: Set and get cache item
    Given I have a cache service available
    When I set a cache item with key "test-key" and value "test-value"
    And I get the cache item with key "test-key"
    Then the cached value should be "test-value"
    And the cache hit should be successful

  Scenario: Set cache item with TTL
    Given I have a cache service available
    When I set a cache item with key "ttl-key" and value "ttl-value" with TTL 2 seconds
    And I get the cache item with key "ttl-key" immediately
    Then the cached value should be "ttl-value"
    When I wait for 3 seconds
    And I get the cache item with key "ttl-key"
    Then the cache hit should be unsuccessful

  Scenario: Get non-existent cache item
    Given I have a cache service available
    When I get the cache item with key "non-existent-key"
    Then the cache hit should be unsuccessful
    And no value should be returned

  Scenario: Delete cache item
    Given I have a cache service available
    And I have set a cache item with key "delete-key" and value "delete-value"
    When I delete the cache item with key "delete-key"
    And I get the cache item with key "delete-key"
    Then the cache hit should be unsuccessful

  Scenario: Flush all cache items
    Given I have a cache service available
    And I have set multiple cache items
    When I flush all cache items
    And I get any of the previously set cache items
    Then the cache hit should be unsuccessful

  Scenario: Set multiple cache items
    Given I have a cache service available
    When I set multiple cache items with different keys and values
    Then all items should be stored successfully
    And I should be able to retrieve all items

  Scenario: Get multiple cache items
    Given I have a cache service available
    And I have set multiple cache items with keys "multi1", "multi2", "multi3"
    When I get multiple cache items with the same keys
    Then I should receive all the cached values
    And the values should match what was stored

  Scenario: Delete multiple cache items
    Given I have a cache service available
    And I have set multiple cache items with keys "del1", "del2", "del3"
    When I delete multiple cache items with the same keys
    And I get multiple cache items with the same keys
    Then I should receive no cached values

  Scenario: Cache with default TTL
    Given I have a cache service with default TTL configured
    When I set a cache item without specifying TTL
    Then the item should use the default TTL from configuration