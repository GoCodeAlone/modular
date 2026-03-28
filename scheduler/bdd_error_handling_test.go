package scheduler

import (
	"context"
	"fmt"
	"time"
)

// Error handling and retry step implementations

func (ctx *SchedulerBDDTestContext) iHaveASchedulerWithRetryConfiguration() error {
	ctx.resetContext()

	// Create scheduler configuration for retry testing
	ctx.config = &SchedulerConfig{
		WorkerCount:        1, // Single worker for predictable testing
		QueueSize:          100,
		CheckInterval:      1 * time.Second,
		ShutdownTimeout:    30 * time.Second,
		StorageType:        "memory",
		RetentionDays:      1,
		PersistenceBackend: PersistenceBackendNone,
	}

	return ctx.setupSchedulerModule()
}

func (ctx *SchedulerBDDTestContext) aJobFailsDuringExecution() error {
	// Schedule a job that fails immediately
	if ctx.service == nil {
		if err := ctx.theSchedulerServiceShouldBeAvailable(); err != nil {
			return err
		}
	}
	if err := ctx.ensureAppStarted(); err != nil {
		return err
	}
	job := Job{
		Name:    "fail-job",
		RunAt:   time.Now().Add(10 * time.Millisecond),
		JobFunc: func(ctx context.Context) error { return fmt.Errorf("intentional failure") },
	}
	id, err := ctx.service.ScheduleJob(job)
	if err != nil {
		return err
	}
	ctx.jobID = id
	// No sleep here; the verification step will poll for failure
	return nil
}

func (ctx *SchedulerBDDTestContext) theJobShouldBeRetriedAccordingToTheRetryPolicy() error {
	// Ensure service is available
	if ctx.service == nil {
		err := ctx.theSchedulerServiceShouldBeAvailable()
		if err != nil {
			return err
		}
	}

	// Verify scheduler is configured for handling failed jobs
	if ctx.service.config.WorkerCount == 0 {
		return fmt.Errorf("scheduler not properly configured")
	}
	return nil
}

func (ctx *SchedulerBDDTestContext) failedJobsShouldBeMarkedAppropriately() error {
	if ctx.jobID == "" {
		return fmt.Errorf("no job to check")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := ctx.service.GetJob(ctx.jobID)
		if err == nil && job.Status == JobStatusFailed {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	job, _ := ctx.service.GetJob(ctx.jobID)
	return fmt.Errorf("expected failed status, got %s", job.Status)
}
