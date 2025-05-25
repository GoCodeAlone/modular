package scheduler

import (
	"time"
)

// JobStore defines the interface for job storage implementations
type JobStore interface {
	// AddJob stores a new job
	AddJob(job Job) error

	// UpdateJob updates an existing job
	UpdateJob(job Job) error

	// GetJob retrieves a job by ID
	GetJob(jobID string) (Job, error)

	// GetJobs returns all jobs
	GetJobs() ([]Job, error)

	// GetPendingJobs returns all pending jobs
	GetPendingJobs() ([]Job, error)

	// GetDueJobs returns jobs that are due to run at or before the given time
	GetDueJobs(before time.Time) ([]Job, error)

	// DeleteJob removes a job
	DeleteJob(jobID string) error

	// AddJobExecution records a job execution
	AddJobExecution(execution JobExecution) error

	// UpdateJobExecution updates a job execution
	UpdateJobExecution(execution JobExecution) error

	// GetJobExecutions retrieves execution history for a job
	GetJobExecutions(jobID string) ([]JobExecution, error)

	// CleanupOldExecutions removes execution records older than retention period
	CleanupOldExecutions(before time.Time) error
}

// PersistableJobStore extends JobStore with persistence capabilities
type PersistableJobStore interface {
	JobStore
	
	// LoadFromFile loads jobs from a file
	LoadFromFile(filePath string) ([]Job, error)
	
	// SaveToFile saves jobs to a file
	SaveToFile(jobs []Job, filePath string) error
}
