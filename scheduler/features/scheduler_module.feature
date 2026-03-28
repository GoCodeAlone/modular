Feature: Scheduler Module
  As a developer using the Modular framework
  I want to use the scheduler module for job scheduling and task execution
  So that I can run background tasks and cron jobs reliably

  Background:
    Given I have a modular application with scheduler module configured

  Scenario: Scheduler module initialization
    When the scheduler module is initialized
    Then the scheduler service should be available
    And the module should be ready to schedule jobs

  Scenario: Immediate job execution
    Given I have a scheduler configured for immediate execution
    When I schedule a job to run immediately
    Then the job should be executed right away
    And the job status should be updated to completed

  Scenario: Delayed job execution
    Given I have a scheduler configured for delayed execution
    When I schedule a job to run in the future
    Then the job should be queued with the correct execution time
    And the job should be executed at the scheduled time

  Scenario: Job persistence and recovery
    Given I have a scheduler with persistence enabled
    When I schedule multiple jobs
    And the scheduler is restarted
    Then all pending jobs should be recovered
    And job execution should continue as scheduled

  Scenario: Worker pool management
    Given I have a scheduler with configurable worker pool
    When multiple jobs are scheduled simultaneously
    Then jobs should be processed by available workers
    And the worker pool should handle concurrent execution

  Scenario: Job status tracking
    Given I have a scheduler with status tracking enabled
    When I schedule a job
    Then I should be able to query the job status
    And the status should update as the job progresses

  Scenario: Job cleanup and retention
    Given I have a scheduler with cleanup policies configured
    When old completed jobs accumulate
    Then jobs older than the retention period should be cleaned up
    And storage space should be reclaimed

  Scenario: Error handling and retries
    Given I have a scheduler with retry configuration
    When a job fails during execution
    Then the job should be retried according to the retry policy
    And failed jobs should be marked appropriately

  Scenario: Job cancellation
    Given I have a scheduler with running jobs
    When I cancel a scheduled job
    Then the job should be removed from the queue
    And running jobs should be stopped gracefully

  Scenario: Graceful shutdown with job completion
    Given I have a scheduler with active jobs
    When the module is stopped
    Then running jobs should be allowed to complete
    And new jobs should not be accepted

  Scenario: Emit events during scheduler lifecycle
    Given I have a scheduler with event observation enabled
    When the scheduler module starts
    Then a scheduler started event should be emitted
    And a config loaded event should be emitted
    And the events should contain scheduler configuration details
    When the scheduler module stops
    Then a scheduler stopped event should be emitted

  Scenario: Emit events during job scheduling
    Given I have a scheduler with event observation enabled
    When I schedule a new job
    Then a job scheduled event should be emitted
    And the event should contain job details
    When the job starts execution
    Then a job started event should be emitted
    When the job completes successfully
    Then a job completed event should be emitted

  Scenario: Emit events during job failures
    Given I have a scheduler with event observation enabled
    When I schedule a job that will fail
    Then a job scheduled event should be emitted
    When the job starts execution
    Then a job started event should be emitted
    When the job fails during execution
    Then a job failed event should be emitted
    And the event should contain error information

  Scenario: Emit events during worker pool management
    Given I have a scheduler with event observation enabled
    When the scheduler starts worker pool
    Then worker started events should be emitted
    And the events should contain worker information
    When workers become busy processing jobs
    Then worker busy events should be emitted
    When workers become idle after job completion
    Then worker idle events should be emitted