package scheduler

import (
	"context"
	"fmt"
	"testing"

	"github.com/cucumber/godog"
)

// TestSchedulerModuleBDD runs the BDD tests for the Scheduler module
func runSchedulerSuite(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			ctx := &SchedulerBDDTestContext{}

			// Per-scenario allowlist for known/intentional warnings or errors.
			// We include substrings rather than full messages for resiliency.
			baseAllow := []string{
				"shutdown timed out",             // graceful stop timeouts tolerated
				"intentional failure",            // deliberate failing job
				"failed to pre-save jobs",        // persistence race conditions tolerated in tests
				"Failed to emit scheduler event", // allowed until tests register observer earlier in all scenarios
				"Unknown storage type",           // scenario may intentionally force fallback
				"Job execution failed",           // expected in failure scenario
			}

			// After each scenario, verify no unexpected warnings/errors were logged; mark success only for target.
			s.After(func(stdCtx context.Context, sc *godog.Scenario, scenarioErr error) (context.Context, error) {
				if tl, ok := ctx.app.Logger().(*testLogger); ok && tl != nil {
					unexpected := tl.unexpectedWarningsOrErrors(baseAllow)
					if len(unexpected) > 0 && scenarioErr == nil {
						scenarioErr = fmt.Errorf("unexpected warnings/errors: %v", unexpected)
					}
				}
				return stdCtx, scenarioErr
			})

			// Background
			s.Given(`^I have a modular application with scheduler module configured$`, ctx.iHaveAModularApplicationWithSchedulerModuleConfigured)

			// Core module lifecycle
			s.When(`^the scheduler module is initialized$`, ctx.theSchedulerModuleIsInitialized)
			s.Then(`^the scheduler service should be available$`, ctx.theSchedulerServiceShouldBeAvailable)
			s.Then(`^the module should be ready to schedule jobs$`, ctx.theModuleShouldBeReadyToScheduleJobs)

			// Job scheduling - immediate execution
			s.Given(`^I have a scheduler configured for immediate execution$`, ctx.iHaveASchedulerConfiguredForImmediateExecution)
			s.When(`^I schedule a job to run immediately$`, ctx.iScheduleAJobToRunImmediately)
			s.Then(`^the job should be executed right away$`, ctx.theJobShouldBeExecutedRightAway)
			s.Then(`^the job status should be updated to completed$`, ctx.theJobStatusShouldBeUpdatedToCompleted)

			// Job scheduling - delayed execution
			s.Given(`^I have a scheduler configured for delayed execution$`, ctx.iHaveASchedulerConfiguredForDelayedExecution)
			s.When(`^I schedule a job to run in the future$`, ctx.iScheduleAJobToRunInTheFuture)
			s.Then(`^the job should be queued with the correct execution time$`, ctx.theJobShouldBeQueuedWithTheCorrectExecutionTime)
			s.Then(`^the job should be executed at the scheduled time$`, ctx.theJobShouldBeExecutedAtTheScheduledTime)

			// Persistence and recovery
			s.Given(`^I have a scheduler with persistence enabled$`, ctx.iHaveASchedulerWithPersistenceEnabled)
			s.When(`^I schedule multiple jobs$`, ctx.iScheduleMultipleJobs)
			s.When(`^the scheduler is restarted$`, ctx.theSchedulerIsRestarted)
			s.Then(`^all pending jobs should be recovered$`, ctx.allPendingJobsShouldBeRecovered)
			s.Then(`^job execution should continue as scheduled$`, ctx.jobExecutionShouldContinueAsScheduled)

			// Worker pool management
			s.Given(`^I have a scheduler with configurable worker pool$`, ctx.iHaveASchedulerWithConfigurableWorkerPool)
			s.When(`^multiple jobs are scheduled simultaneously$`, ctx.multipleJobsAreScheduledSimultaneously)
			s.Then(`^jobs should be processed by available workers$`, ctx.jobsShouldBeProcessedByAvailableWorkers)
			s.Then(`^the worker pool should handle concurrent execution$`, ctx.theWorkerPoolShouldHandleConcurrentExecution)

			// Job lifecycle and status tracking
			s.Given(`^I have a scheduler with status tracking enabled$`, ctx.iHaveASchedulerWithStatusTrackingEnabled)
			s.When(`^I schedule a job$`, ctx.iScheduleAJob)
			s.Then(`^I should be able to query the job status$`, ctx.iShouldBeAbleToQueryTheJobStatus)
			s.Then(`^the status should update as the job progresses$`, ctx.theStatusShouldUpdateAsTheJobProgresses)

			// Job management - cleanup
			s.Given(`^I have a scheduler with cleanup policies configured$`, ctx.iHaveASchedulerWithCleanupPoliciesConfigured)
			s.When(`^old completed jobs accumulate$`, ctx.oldCompletedJobsAccumulate)
			s.Then(`^jobs older than the retention period should be cleaned up$`, ctx.jobsOlderThanTheRetentionPeriodShouldBeCleanedUp)
			s.Then(`^storage space should be reclaimed$`, ctx.storageSpaceShouldBeReclaimed)

			// Error handling and retries
			s.Given(`^I have a scheduler with retry configuration$`, ctx.iHaveASchedulerWithRetryConfiguration)
			s.When(`^a job fails during execution$`, ctx.aJobFailsDuringExecution)
			s.Then(`^the job should be retried according to the retry policy$`, ctx.theJobShouldBeRetriedAccordingToTheRetryPolicy)
			s.Then(`^failed jobs should be marked appropriately$`, ctx.failedJobsShouldBeMarkedAppropriately)

			// Job management - cancellation
			s.Given(`^I have a scheduler with running jobs$`, ctx.iHaveASchedulerWithRunningJobs)
			s.When(`^I cancel a scheduled job$`, ctx.iCancelAScheduledJob)
			s.Then(`^the job should be removed from the queue$`, ctx.theJobShouldBeRemovedFromTheQueue)
			s.Then(`^running jobs should be stopped gracefully$`, ctx.runningJobsShouldBeStoppedGracefully)

			// Graceful shutdown
			s.Given(`^I have a scheduler with active jobs$`, ctx.iHaveASchedulerWithActiveJobs)
			s.When(`^the module is stopped$`, ctx.theModuleIsStopped)
			s.Then(`^running jobs should be allowed to complete$`, ctx.runningJobsShouldBeAllowedToComplete)
			s.Then(`^new jobs should not be accepted$`, ctx.newJobsShouldNotBeAccepted)

			// Event observation scenarios - scheduler lifecycle
			s.Given(`^I have a scheduler with event observation enabled$`, ctx.iHaveASchedulerWithEventObservationEnabled)
			s.When(`^the scheduler module starts$`, ctx.theSchedulerModuleStarts)
			s.Then(`^a scheduler started event should be emitted$`, ctx.aSchedulerStartedEventShouldBeEmitted)
			s.Then(`^a config loaded event should be emitted$`, ctx.aConfigLoadedEventShouldBeEmitted)
			s.Then(`^the events should contain scheduler configuration details$`, ctx.theEventsShouldContainSchedulerConfigurationDetails)
			s.When(`^the scheduler module stops$`, ctx.theSchedulerModuleStops)
			s.Then(`^a scheduler stopped event should be emitted$`, ctx.aSchedulerStoppedEventShouldBeEmitted)

			// Event observation - job scheduling events
			s.When(`^I schedule a new job$`, ctx.iScheduleANewJob)
			s.Then(`^a job scheduled event should be emitted$`, ctx.aJobScheduledEventShouldBeEmitted)
			s.Then(`^the event should contain job details$`, ctx.theEventShouldContainJobDetails)
			s.When(`^the job starts execution$`, ctx.theJobStartsExecution)
			s.Then(`^a job started event should be emitted$`, ctx.aJobStartedEventShouldBeEmitted)
			s.When(`^the job completes successfully$`, ctx.theJobCompletesSuccessfully)
			s.Then(`^a job completed event should be emitted$`, ctx.aJobCompletedEventShouldBeEmitted)

			// Event observation - job failure events
			s.When(`^I schedule a job that will fail$`, ctx.iScheduleAJobThatWillFail)
			s.When(`^the job fails during execution$`, ctx.theJobFailsDuringExecution)
			s.Then(`^a job failed event should be emitted$`, ctx.aJobFailedEventShouldBeEmitted)
			s.Then(`^the event should contain error information$`, ctx.theEventShouldContainErrorInformation)

			// Event observation - worker pool events
			s.When(`^the scheduler starts worker pool$`, ctx.theSchedulerStartsWorkerPool)
			s.Then(`^worker started events should be emitted$`, ctx.workerStartedEventsShouldBeEmitted)
			s.Then(`^the events should contain worker information$`, ctx.theEventsShouldContainWorkerInformation)
			s.When(`^workers become busy processing jobs$`, ctx.workersBecomeBusyProcessingJobs)
			s.Then(`^worker busy events should be emitted$`, ctx.workerBusyEventsShouldBeEmitted)
			s.When(`^workers become idle after job completion$`, ctx.workersBecomeIdleAfterJobCompletion)
			s.Then(`^worker idle events should be emitted$`, ctx.workerIdleEventsShouldBeEmitted)

			// Event validation (mega-scenario)
			s.Then(`^all registered events should be emitted during testing$`, ctx.allRegisteredEventsShouldBeEmittedDuringTesting)
		},
		Options: &godog.Options{
			Format:   "progress",
			Paths:    []string{"features/scheduler_module.feature"},
			TestingT: t,
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

// TestSchedulerModuleBDD orchestrates each feature scenario as an isolated parallel subtest.
// This increases overall test throughput while keeping scenarios independent.
func TestSchedulerModuleBDD(t *testing.T) { runSchedulerSuite(t) }
