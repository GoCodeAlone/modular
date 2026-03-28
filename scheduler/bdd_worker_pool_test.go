package scheduler

import (
	"fmt"
	"time"
)

// Worker pool management step implementations

func (ctx *SchedulerBDDTestContext) iHaveASchedulerWithConfigurableWorkerPool() error {
	ctx.resetContext()

	// Create scheduler configuration with worker pool settings
	ctx.config = &SchedulerConfig{
		WorkerCount:        5,  // Specific worker count for this test
		QueueSize:          50, // Specific queue size for this test
		CheckInterval:      10 * time.Millisecond,
		ShutdownTimeout:    30 * time.Second,
		StorageType:        "memory",
		RetentionDays:      1,
		PersistenceBackend: PersistenceBackendNone,
	}

	return ctx.setupSchedulerModule()
}

func (ctx *SchedulerBDDTestContext) multipleJobsAreScheduledSimultaneously() error {
	return ctx.iScheduleMultipleJobs()
}

func (ctx *SchedulerBDDTestContext) jobsShouldBeProcessedByAvailableWorkers() error {
	// Ensure service is available
	if ctx.service == nil {
		err := ctx.theSchedulerServiceShouldBeAvailable()
		if err != nil {
			return err
		}
	}

	// Verify worker pool configuration
	if ctx.service.config.WorkerCount != 5 {
		return fmt.Errorf("expected 5 workers, got %d", ctx.service.config.WorkerCount)
	}
	return nil
}

func (ctx *SchedulerBDDTestContext) theWorkerPoolShouldHandleConcurrentExecution() error {
	// Wait up to 2s for multiple jobs to complete
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		jobs, err := ctx.service.ListJobs()
		if err != nil {
			return fmt.Errorf("failed to list jobs: %w", err)
		}
		completed := 0
		for _, j := range jobs {
			if j.Status == JobStatusCompleted {
				completed++
			}
		}
		if completed >= 2 {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("expected at least 2 jobs to complete concurrently")
}
