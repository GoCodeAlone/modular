// Package scheduler defines interfaces for job scheduling and execution
package scheduler

import (
	"context"
	"time"
)

// SchedulerService defines additional service interface methods for the scheduler
type SchedulerService interface {
	// TriggerJob manually triggers execution of a job
	TriggerJob(ctx context.Context, jobID string, options *TriggerOptions) (*JobExecution, error)

	// GetExecutions returns execution history for a job
	GetExecutions(ctx context.Context, jobID string, limit int) ([]*JobExecution, error)

	// PauseJob pauses execution of a job
	PauseJob(ctx context.Context, jobID string) error

	// ResumeJob resumes execution of a paused job
	ResumeJob(ctx context.Context, jobID string) error

	// GetStatistics returns scheduler performance statistics
	GetStatistics(ctx context.Context) (*SchedulerStatistics, error)
}

// JobExecutor defines the interface for executing jobs
type JobExecutor interface {
	// Execute executes a job and returns the result
	Execute(ctx context.Context, job *Job, execution *JobExecution) (*ExecutionResult, error)

	// CanExecute returns true if this executor can handle the given job
	CanExecute(job *Job) bool

	// Name returns the name of this executor
	Name() string
}

// ExtendedJobStore extends the existing JobStore with additional capabilities
type ExtendedJobStore interface {
	JobStore

	// Store persists a job definition (alias for AddJob for consistency)
	Store(ctx context.Context, job *Job) error

	// Get retrieves a job definition by ID (alias for GetJob)
	Get(ctx context.Context, jobID string) (*Job, error)

	// List retrieves all job definitions (alias for GetJobs)
	List(ctx context.Context) ([]*Job, error)

	// Delete removes a job definition (alias for DeleteJob)
	Delete(ctx context.Context, jobID string) error

	// Update updates an existing job definition (alias for UpdateJob)
	Update(ctx context.Context, job *Job) error
}

// ExecutionStore defines the interface for job execution persistence
type ExecutionStore interface {
	// Store persists a job execution record
	Store(ctx context.Context, execution *JobExecution) error

	// Get retrieves a job execution by ID
	Get(ctx context.Context, executionID string) (*JobExecution, error)

	// GetByJob retrieves executions for a specific job
	GetByJob(ctx context.Context, jobID string, limit int, offset int) ([]*JobExecution, error)

	// Update updates an existing execution record
	Update(ctx context.Context, execution *JobExecution) error

	// Cleanup removes old execution records based on retention policy
	Cleanup(ctx context.Context, retentionPeriod time.Duration) error
}

// CronParser defines the interface for parsing cron expressions
type CronParser interface {
	// Parse parses a cron expression and returns the next execution time
	Parse(cronExpr string) (CronSchedule, error)

	// Validate validates a cron expression without parsing
	Validate(cronExpr string) error

	// Next returns the next execution time for the given cron expression
	Next(cronExpr string, from time.Time) (time.Time, error)
}

// CronSchedule represents a parsed cron schedule
type CronSchedule interface {
	// Next returns the next execution time after the given time
	Next(time.Time) time.Time

	// String returns the string representation of the cron expression
	String() string
}

// Extended types that don't conflict with existing ones

// ExecutionResult represents the result of a job execution
type ExecutionResult struct {
	Success bool                   `json:"success"`
	Output  string                 `json:"output,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Metrics map[string]float64     `json:"metrics,omitempty"`
	Logs    []string               `json:"logs,omitempty"`
}

// RetryPolicy defines how jobs should be retried on failure
type RetryPolicy struct {
	MaxRetries    int           `json:"max_retries"`
	InitialDelay  time.Duration `json:"initial_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	BackoffFactor float64       `json:"backoff_factor"`
	RetryOnErrors []string      `json:"retry_on_errors,omitempty"`
	SkipOnErrors  []string      `json:"skip_on_errors,omitempty"`
}

// BackfillPolicy defines how missed executions should be handled
type BackfillPolicy struct {
	Enabled         bool             `json:"enabled"`
	MaxBackfillJobs int              `json:"max_backfill_jobs"`
	BackfillWindow  time.Duration    `json:"backfill_window"`
	Strategy        BackfillStrategy `json:"strategy"`
}

// NotificationPolicy defines how job execution events should be reported
type NotificationPolicy struct {
	OnSuccess  bool     `json:"on_success"`
	OnFailure  bool     `json:"on_failure"`
	OnRetry    bool     `json:"on_retry"`
	Recipients []string `json:"recipients,omitempty"`
	Channels   []string `json:"channels,omitempty"`
}

// TriggerOptions provides options for manually triggering jobs
type TriggerOptions struct {
	Force       bool                   `json:"force"`          // Force execution even if at max concurrency
	Data        map[string]interface{} `json:"data,omitempty"` // Override job data
	Tags        []string               `json:"tags,omitempty"` // Additional tags for this execution
	TriggeredBy string                 `json:"triggered_by,omitempty"`
}

// SchedulerStatistics provides statistics about scheduler performance
type SchedulerStatistics struct {
	TotalJobs            int64               `json:"total_jobs"`
	RunningJobs          int64               `json:"running_jobs"`
	QueuedJobs           int64               `json:"queued_jobs"`
	CompletedJobs        int64               `json:"completed_jobs"`
	FailedJobs           int64               `json:"failed_jobs"`
	AverageExecutionTime time.Duration       `json:"average_execution_time"`
	JobsByStatus         map[JobStatus]int64 `json:"jobs_by_status"`
	LastUpdateTime       time.Time           `json:"last_update_time"`
}

// Constants for new enums

// BackfillStrategy defines strategies for backfilling missed executions
type BackfillStrategy string

const (
	BackfillStrategyAll  BackfillStrategy = "all"  // Backfill all missed executions
	BackfillStrategyLast BackfillStrategy = "last" // Only backfill the most recent missed execution
	BackfillStrategyNone BackfillStrategy = "none" // No backfilling
)
