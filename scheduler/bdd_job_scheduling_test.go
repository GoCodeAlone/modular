package scheduler

import (
	"context"
	"fmt"
	"time"
)

// Job scheduling step implementations

func (ctx *SchedulerBDDTestContext) iHaveASchedulerConfiguredForImmediateExecution() error {
	err := ctx.iHaveAModularApplicationWithSchedulerModuleConfigured()
	if err != nil {
		return err
	}

	// Configure for immediate execution (very short interval)
	ctx.config.CheckInterval = 10 * time.Millisecond

	return ctx.theSchedulerModuleIsInitialized()
}

func (ctx *SchedulerBDDTestContext) iScheduleAJobToRunImmediately() error {
	// Ensure service is available
	if ctx.service == nil {
		err := ctx.theSchedulerServiceShouldBeAvailable()
		if err != nil {
			return err
		}
	}

	// Start the service
	if err := ctx.ensureAppStarted(); err != nil {
		return err
	}

	// Create a test job
	testCtx := ctx // Capture the test context
	testJob := func(jobCtx context.Context) error {
		testCtx.jobCompleted = true
		testCtx.jobResults = append(testCtx.jobResults, "job executed")
		// Simulate brief work so status transitions can be observed
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	// Schedule the job for immediate execution
	job := Job{
		Name:    "test-job",
		RunAt:   time.Now(),
		JobFunc: testJob,
	}
	jobID, err := ctx.service.ScheduleJob(job)
	if err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}
	ctx.jobID = jobID

	return nil
}

func (ctx *SchedulerBDDTestContext) theJobShouldBeExecutedRightAway() error {
	// Verify that the scheduler service is running and the job is scheduled
	if ctx.service == nil {
		return fmt.Errorf("scheduler service not available")
	}

	// For immediate jobs, verify the job ID was generated (indicating job was scheduled)
	if ctx.jobID == "" {
		return fmt.Errorf("job should have been scheduled with a job ID")
	}

	// Poll until the job completes or timeout
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := ctx.service.GetJob(ctx.jobID)
		if err == nil && job.Status == JobStatusCompleted {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("job did not complete within timeout")
}

func (ctx *SchedulerBDDTestContext) theJobStatusShouldBeUpdatedToCompleted() error {
	if ctx.jobID == "" {
		return fmt.Errorf("no job ID to check")
	}
	// Poll for completion and verify history
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := ctx.service.GetJob(ctx.jobID)
		if err == nil && job.Status == JobStatusCompleted {
			history, _ := ctx.service.GetJobHistory(ctx.jobID)
			if len(history) > 0 && history[len(history)-1].Status == string(JobStatusCompleted) {
				return nil
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	job, _ := ctx.service.GetJob(ctx.jobID)
	return fmt.Errorf("expected job to complete, final status: %s", job.Status)
}

func (ctx *SchedulerBDDTestContext) iHaveASchedulerConfiguredForDelayedExecution() error {
	return ctx.iHaveASchedulerConfiguredForImmediateExecution()
}

func (ctx *SchedulerBDDTestContext) iScheduleAJobToRunInTheFuture() error {
	// Ensure service is available
	if ctx.service == nil {
		err := ctx.theSchedulerServiceShouldBeAvailable()
		if err != nil {
			return err
		}
	}

	// Start the service
	if err := ctx.ensureAppStarted(); err != nil {
		return err
	}

	// Create a test job
	testJob := func(ctx context.Context) error { return nil }

	// Schedule the job for near-future execution to keep tests fast
	futureTime := time.Now().Add(150 * time.Millisecond)
	job := Job{
		Name:    "future-job",
		RunAt:   futureTime,
		JobFunc: testJob,
	}
	jobID, err := ctx.service.ScheduleJob(job)
	if err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}
	ctx.jobID = jobID
	ctx.scheduledAt = futureTime

	return nil
}

func (ctx *SchedulerBDDTestContext) theJobShouldBeQueuedWithTheCorrectExecutionTime() error {
	if ctx.jobID == "" {
		return fmt.Errorf("job not scheduled")
	}
	job, err := ctx.service.GetJob(ctx.jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}
	if job.NextRun == nil {
		return fmt.Errorf("expected NextRun to be set")
	}
	// Allow small clock drift
	diff := job.NextRun.Sub(ctx.scheduledAt)
	if diff < -50*time.Millisecond || diff > 50*time.Millisecond {
		return fmt.Errorf("expected NextRun ~ %v, got %v (diff %v)", ctx.scheduledAt, *job.NextRun, diff)
	}
	return nil
}

func (ctx *SchedulerBDDTestContext) theJobShouldBeExecutedAtTheScheduledTime() error {
	// Poll until after scheduled time and verify execution occurred
	deadline := time.Now().Add(time.Until(ctx.scheduledAt) + 2*time.Second)
	for time.Now().Before(deadline) {
		history, _ := ctx.service.GetJobHistory(ctx.jobID)
		if len(history) > 0 {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("expected job to have executed after scheduled time")
}
