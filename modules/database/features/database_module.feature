Feature: Database Module
  As a developer using the Modular framework
  I want to use the database module for data persistence
  So that I can build applications with database connectivity

  Background:
    Given I have a modular application with database module configured

  Scenario: Database module initialization
    When the database module is initialized
    Then the database service should be available
    And database connections should be configured

  Scenario: Execute SQL query
    Given I have a database connection
    When I execute a simple SQL query
    Then the query should execute successfully
    And I should receive the expected results

  Scenario: Execute SQL query with parameters
    Given I have a database connection
    When I execute a parameterized SQL query
    Then the query should execute successfully with parameters
    And the parameters should be properly escaped

  Scenario: Handle database connection errors
    Given I have an invalid database configuration
    When I try to execute a query
    Then the operation should fail gracefully
    And an appropriate database error should be returned

  Scenario: Database transaction management
    Given I have a database connection
    When I start a database transaction
    Then I should be able to execute queries within the transaction
    And I should be able to commit or rollback the transaction

  Scenario: Connection pool management
    Given I have database connection pooling configured
    When I make multiple concurrent database requests
    Then the connection pool should handle the requests efficiently
    And connections should be reused properly

  Scenario: Health check functionality
    Given I have a database module configured
    When I perform a health check
    Then the health check should report database status
    And indicate whether the database is accessible