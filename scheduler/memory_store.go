package scheduler

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Memory store errors
var (
	ErrJobAlreadyExists  = errors.New("job already exists")
	ErrJobNotFound       = errors.New("job not found")
	ErrNoExecutionsFound = errors.New("no executions found for job")
	ErrExecutionNotFound = errors.New("execution not found")
)

// MemoryJobStore implements JobStore using in-memory storage
type MemoryJobStore struct {
	jobs            map[string]Job
	jobsMutex       sync.RWMutex
	executions      map[string][]JobExecution
	executionsMutex sync.RWMutex
	retentionPeriod time.Duration
}

// NewMemoryJobStore creates a new memory job store
func NewMemoryJobStore(retentionPeriod time.Duration) *MemoryJobStore {
	return &MemoryJobStore{
		jobs:            make(map[string]Job),
		executions:      make(map[string][]JobExecution),
		retentionPeriod: retentionPeriod,
	}
}

// AddJob stores a new job
func (s *MemoryJobStore) AddJob(job Job) error {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	// Check if job already exists
	if _, exists := s.jobs[job.ID]; exists {
		return fmt.Errorf("%w: %s", ErrJobAlreadyExists, job.ID)
	}

	s.jobs[job.ID] = job
	return nil
}

// UpdateJob updates an existing job
func (s *MemoryJobStore) UpdateJob(job Job) error {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	// Check if job exists
	if _, exists := s.jobs[job.ID]; !exists {
		return fmt.Errorf("%w: %s", ErrJobNotFound, job.ID)
	}

	s.jobs[job.ID] = job
	return nil
}

// GetJob retrieves a job by ID
func (s *MemoryJobStore) GetJob(jobID string) (Job, error) {
	s.jobsMutex.RLock()
	defer s.jobsMutex.RUnlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return Job{}, fmt.Errorf("%w: %s", ErrJobNotFound, jobID)
	}

	return job, nil
}

// GetJobs returns all jobs
func (s *MemoryJobStore) GetJobs() ([]Job, error) {
	s.jobsMutex.RLock()
	defer s.jobsMutex.RUnlock()

	jobs := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetPendingJobs returns all pending jobs
func (s *MemoryJobStore) GetPendingJobs() ([]Job, error) {
	s.jobsMutex.RLock()
	defer s.jobsMutex.RUnlock()

	pendingJobs := make([]Job, 0)
	for _, job := range s.jobs {
		if job.Status == JobStatusPending {
			pendingJobs = append(pendingJobs, job)
		}
	}

	return pendingJobs, nil
}

// GetDueJobs returns jobs that are due to run at or before the given time
func (s *MemoryJobStore) GetDueJobs(before time.Time) ([]Job, error) {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	dueJobs := make([]Job, 0)
	for id, job := range s.jobs {
		if job.Status == JobStatusPending && job.NextRun != nil && !job.NextRun.After(before) {
			// Update status to prevent duplicate execution
			job.Status = JobStatusRunning
			job.UpdatedAt = time.Now()
			s.jobs[id] = job

			dueJobs = append(dueJobs, job)
		}
	}

	return dueJobs, nil
}

// DeleteJob removes a job
func (s *MemoryJobStore) DeleteJob(jobID string) error {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	if _, exists := s.jobs[jobID]; !exists {
		return fmt.Errorf("%w: %s", ErrJobNotFound, jobID)
	}

	delete(s.jobs, jobID)
	return nil
}

// AddJobExecution records a job execution
func (s *MemoryJobStore) AddJobExecution(execution JobExecution) error {
	s.executionsMutex.Lock()
	defer s.executionsMutex.Unlock()

	if _, exists := s.executions[execution.JobID]; !exists {
		s.executions[execution.JobID] = make([]JobExecution, 0)
	}

	s.executions[execution.JobID] = append(s.executions[execution.JobID], execution)
	return nil
}

// UpdateJobExecution updates a job execution
func (s *MemoryJobStore) UpdateJobExecution(execution JobExecution) error {
	s.executionsMutex.Lock()
	defer s.executionsMutex.Unlock()

	executions, exists := s.executions[execution.JobID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrNoExecutionsFound, execution.JobID)
	}

	// Find the execution by start time
	for i, exec := range executions {
		if exec.StartTime.Equal(execution.StartTime) {
			executions[i] = execution
			s.executions[execution.JobID] = executions
			return nil
		}
	}

	return fmt.Errorf("%w: start time %v for job ID %s", ErrExecutionNotFound, execution.StartTime, execution.JobID)
}

// GetJobExecutions retrieves execution history for a job
func (s *MemoryJobStore) GetJobExecutions(jobID string) ([]JobExecution, error) {
	s.executionsMutex.RLock()
	defer s.executionsMutex.RUnlock()

	executions, exists := s.executions[jobID]
	if !exists {
		return []JobExecution{}, nil
	}

	// Return a copy to prevent modification of internal state
	result := make([]JobExecution, len(executions))
	copy(result, executions)
	return result, nil
}

// CleanupOldExecutions removes execution records older than retention period
func (s *MemoryJobStore) CleanupOldExecutions(before time.Time) error {
	s.executionsMutex.Lock()
	defer s.executionsMutex.Unlock()

	for jobID, executions := range s.executions {
		filtered := make([]JobExecution, 0, len(executions))
		for _, exec := range executions {
			if exec.StartTime.After(before) {
				filtered = append(filtered, exec)
			}
		}
		s.executions[jobID] = filtered
	}

	return nil
}

// LoadJobs loads jobs using the configured persistence handler
func (s *MemoryJobStore) LoadJobs(handler PersistenceHandler) ([]Job, error) {
	if handler == nil {
		return nil, nil
	}

	// Load jobs from handler
	jobs, err := handler.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load jobs from persistence handler: %w", err)
	}

	if len(jobs) == 0 {
		return nil, nil
	}

	// Load jobs into store
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	for _, job := range jobs {
		// Skip if job already exists
		if _, exists := s.jobs[job.ID]; exists {
			continue
		}

		// Clear job function as it can't be persisted
		// It will be reinitialized when job is resumed
		job.JobFunc = nil

		// Add to store
		s.jobs[job.ID] = job
	}

	return jobs, nil
}

// SaveJobs saves jobs using the configured persistence handler
func (s *MemoryJobStore) SaveJobs(jobs []Job, handler PersistenceHandler) error {
	if handler == nil {
		return nil // No persistence handler, nothing to do
	}

	// Save jobs using the handler
	err := handler.Save(jobs)
	if err != nil {
		return fmt.Errorf("failed to save jobs using persistence handler: %w", err)
	}

	return nil
}
