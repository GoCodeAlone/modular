package modular

import (
	"context"
	"time"
)

// ScheduledJobDefinition represents a job that can be scheduled for execution
type ScheduledJobDefinition struct {
	// ID is the unique identifier for this job
	ID string

	// Name is a human-readable name for the job
	Name string

	// Description provides details about what this job does
	Description string

	// Schedule is the cron expression defining when this job runs
	Schedule string

	// Enabled indicates if this job is currently enabled
	Enabled bool

	// MaxConcurrency limits how many instances of this job can run simultaneously
	MaxConcurrency int

	// JobFunc is the function to execute when the job runs
	JobFunc JobFunc

	// TimeoutDuration specifies how long the job can run before timeout
	TimeoutDuration time.Duration

	// RetryPolicy defines how failed executions should be retried
	RetryPolicy *JobRetryPolicy

	// BackfillPolicy defines how missed executions should be handled
	BackfillPolicy *JobBackfillPolicy

	// Metadata contains additional job-specific metadata
	Metadata map[string]interface{}

	// CreatedAt tracks when this job definition was created
	CreatedAt time.Time

	// UpdatedAt tracks when this job definition was last updated
	UpdatedAt time.Time

	// LastExecutionAt tracks when this job was last executed
	LastExecutionAt *time.Time

	// NextExecutionAt tracks when this job is next scheduled to run
	NextExecutionAt *time.Time

	// ExecutionCount tracks how many times this job has been executed
	ExecutionCount int64

	// SuccessCount tracks how many times this job executed successfully
	SuccessCount int64

	// FailureCount tracks how many times this job failed
	FailureCount int64
}

// JobFunc defines a function that can be executed as a scheduled job
type JobFunc func(ctx context.Context) error

// JobRetryPolicy defines how failed job executions should be retried
type JobRetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// InitialDelay is the delay before the first retry
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// BackoffMultiplier is used for exponential backoff
	BackoffMultiplier float64

	// RetryableErrors lists error types that should trigger retries
	RetryableErrors []string
}

// JobBackfillPolicy defines how missed job executions should be handled
type JobBackfillPolicy struct {
	// Strategy defines the backfill strategy to use
	Strategy BackfillStrategy

	// MaxMissedExecutions limits how many missed executions to backfill
	MaxMissedExecutions int

	// MaxBackfillDuration limits how far back to look for missed executions
	MaxBackfillDuration time.Duration

	// Priority specifies the priority for backfill executions
	Priority int
}

// BackfillStrategy represents different strategies for handling missed executions
type BackfillStrategy string

const (
	// BackfillStrategyNone means don't backfill missed executions
	BackfillStrategyNone BackfillStrategy = "none"

	// BackfillStrategyLast means only backfill the last missed execution
	BackfillStrategyLast BackfillStrategy = "last"

	// BackfillStrategyBounded means backfill up to MaxMissedExecutions
	BackfillStrategyBounded BackfillStrategy = "bounded"

	// BackfillStrategyTimeWindow means backfill within MaxBackfillDuration
	BackfillStrategyTimeWindow BackfillStrategy = "time_window"
)

// JobExecution represents the execution details of a scheduled job
type JobExecution struct {
	// ID is the unique identifier for this execution
	ID string

	// JobID is the ID of the job definition this execution belongs to
	JobID string

	// ScheduledAt is when this execution was originally scheduled
	ScheduledAt time.Time

	// StartedAt is when this execution actually started
	StartedAt *time.Time

	// CompletedAt is when this execution completed (success or failure)
	CompletedAt *time.Time

	// Duration is how long the execution took
	Duration *time.Duration

	// Status indicates the current status of this execution
	Status JobExecutionStatus

	// Error contains error information if the execution failed
	Error string

	// Output contains any output produced by the job
	Output string

	// Metadata contains execution-specific metadata
	Metadata map[string]interface{}

	// RetryCount tracks how many times this execution has been retried
	RetryCount int

	// WorkerID identifies which worker executed this job
	WorkerID string
}

// JobExecutionStatus represents the status of a job execution
type JobExecutionStatus string

const (
	// JobExecutionStatusPending indicates the execution is waiting to start
	JobExecutionStatusPending JobExecutionStatus = "pending"

	// JobExecutionStatusRunning indicates the execution is currently running
	JobExecutionStatusRunning JobExecutionStatus = "running"

	// JobExecutionStatusSuccess indicates the execution completed successfully
	JobExecutionStatusSuccess JobExecutionStatus = "success"

	// JobExecutionStatusFailure indicates the execution failed
	JobExecutionStatusFailure JobExecutionStatus = "failure"

	// JobExecutionStatusTimeout indicates the execution timed out
	JobExecutionStatusTimeout JobExecutionStatus = "timeout"

	// JobExecutionStatusCancelled indicates the execution was cancelled
	JobExecutionStatusCancelled JobExecutionStatus = "cancelled"

	// JobExecutionStatusSkipped indicates the execution was skipped
	JobExecutionStatusSkipped JobExecutionStatus = "skipped"
)
