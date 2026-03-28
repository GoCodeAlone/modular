package scheduler

import (
	"fmt"
	"time"
)

// Job lifecycle and status tracking step implementations

func (ctx *SchedulerBDDTestContext) iHaveASchedulerWithStatusTrackingEnabled() error {
	return ctx.iHaveASchedulerConfiguredForImmediateExecution()
}

func (ctx *SchedulerBDDTestContext) iScheduleAJob() error {
	return ctx.iScheduleAJobToRunImmediately()
}

func (ctx *SchedulerBDDTestContext) iShouldBeAbleToQueryTheJobStatus() error {
	if ctx.jobID == "" {
		return fmt.Errorf("no job to query")
	}
	if _, err := ctx.service.GetJob(ctx.jobID); err != nil {
		return fmt.Errorf("failed to query job: %w", err)
	}
	return nil
}

func (ctx *SchedulerBDDTestContext) theStatusShouldUpdateAsTheJobProgresses() error {
	// Poll until at least one execution entry appears
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		history, _ := ctx.service.GetJobHistory(ctx.jobID)
		if len(history) > 0 {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("expected job history to have entries")
}
