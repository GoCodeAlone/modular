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

  Scenario: Emit events during database operations
    Given I have a database service with event observation enabled
    When I execute a database query
    Then a query executed event should be emitted
    And the event should contain query performance metrics
    When I start a database transaction
    Then a transaction started event should be emitted
    When the query fails with an error
    Then a query error event should be emitted
    And the event should contain error details

  Scenario: Emit events during database lifecycle
    Given I have a database service with event observation enabled
    When the database module starts
    Then a configuration loaded event should be emitted
    And a database connected event should be emitted
    When the database module stops
    Then a database disconnected event should be emitted

  Scenario: Emit connection error events
    Given I have a database service with event observation enabled
    When a database connection fails with invalid credentials
    Then a connection error event should be emitted
    And the event should contain connection failure details

  Scenario: Emit transaction commit events
    Given I have a database service with event observation enabled
    And I have started a database transaction
    When I commit the transaction successfully
    Then a transaction committed event should be emitted
    And the event should contain transaction details

  Scenario: Emit transaction rollback events
    Given I have a database service with event observation enabled
    And I have started a database transaction
    When I rollback the transaction
    Then a transaction rolled back event should be emitted
    And the event should contain rollback details

  Scenario: Emit migration started events
    Given I have a database service with event observation enabled
    When a database migration is initiated
    Then a migration started event should be emitted
    And the event should contain migration metadata

  Scenario: Emit migration completed events
    Given I have a database service with event observation enabled
    When a database migration completes successfully
    Then a migration completed event should be emitted
    And the event should contain migration results

  Scenario: Emit migration failed events
    Given I have a database service with event observation enabled
    When a database migration fails with errors
    Then a migration failed event should be emitted
    And the event should contain failure details