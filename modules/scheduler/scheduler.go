package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// JobFunc defines a function that can be executed as a job
type JobFunc func(ctx context.Context) error

// JobExecution records details about a single execution of a job
type JobExecution struct {
	JobID     string    `json:"jobId"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime,omitempty"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
}

// Job represents a scheduled job
type Job struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Schedule    string     `json:"schedule,omitempty"`
	RunAt       time.Time  `json:"runAt,omitempty"`
	IsRecurring bool       `json:"isRecurring"`
	JobFunc     JobFunc    `json:"-"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	Status      JobStatus  `json:"status"`
	LastRun     *time.Time `json:"lastRun,omitempty"`
	NextRun     *time.Time `json:"nextRun,omitempty"`
}

// JobStatus represents the status of a job
type JobStatus string

const (
	// JobStatusPending indicates a job is waiting to be executed
	JobStatusPending JobStatus = "pending"
	// JobStatusRunning indicates a job is currently executing
	JobStatusRunning JobStatus = "running"
	// JobStatusCompleted indicates a job has completed successfully
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed indicates a job has failed
	JobStatusFailed JobStatus = "failed"
	// JobStatusCancelled indicates a job has been cancelled
	JobStatusCancelled JobStatus = "cancelled"
)

// Scheduler handles scheduling and executing jobs
type Scheduler struct {
	jobStore       JobStore
	workerCount    int
	queueSize      int
	checkInterval  time.Duration
	logger         modular.Logger
	jobQueue       chan Job
	cronScheduler  *cron.Cron
	cronEntries    map[string]cron.EntryID
	entryMutex     sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	isStarted      bool
	schedulerMutex sync.Mutex
}

// SchedulerOption defines a function that can configure a scheduler
type SchedulerOption func(*Scheduler)

// WithWorkerCount sets the number of workers
func WithWorkerCount(count int) SchedulerOption {
	return func(s *Scheduler) {
		if count > 0 {
			s.workerCount = count
		}
	}
}

// WithQueueSize sets the job queue size
func WithQueueSize(size int) SchedulerOption {
	return func(s *Scheduler) {
		if size > 0 {
			s.queueSize = size
		}
	}
}

// WithCheckInterval sets how often to check for scheduled jobs
func WithCheckInterval(interval time.Duration) SchedulerOption {
	return func(s *Scheduler) {
		if interval > 0 {
			s.checkInterval = interval
		}
	}
}

// WithLogger sets the logger
func WithLogger(logger modular.Logger) SchedulerOption {
	return func(s *Scheduler) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// NewScheduler creates a new scheduler
func NewScheduler(jobStore JobStore, opts ...SchedulerOption) *Scheduler {
	s := &Scheduler{
		jobStore:      jobStore,
		workerCount:   5, // Default
		queueSize:     100,
		checkInterval: time.Second,
		cronEntries:   make(map[string]cron.EntryID),
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Create cron scheduler
	s.cronScheduler = cron.New()

	return s
}

// Start starts the scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	s.schedulerMutex.Lock()
	defer s.schedulerMutex.Unlock()

	if s.isStarted {
		return nil
	}

	if s.logger != nil {
		s.logger.Info("Starting scheduler", "workers", s.workerCount, "queueSize", s.queueSize)
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.jobQueue = make(chan Job, s.queueSize)

	// Start worker goroutines
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}

	// Start cron scheduler
	s.cronScheduler.Start()

	// Start job dispatcher
	s.wg.Add(1)
	go s.dispatchPendingJobs()

	s.isStarted = true
	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop(ctx context.Context) error {
	s.schedulerMutex.Lock()
	defer s.schedulerMutex.Unlock()

	if !s.isStarted {
		return nil
	}

	if s.logger != nil {
		s.logger.Info("Stopping scheduler")
	}

	// Cancel the context to signal workers to stop
	if s.cancel != nil {
		s.cancel()
	}

	// Stop the cron scheduler
	cronCtx := s.cronScheduler.Stop()

	// Wait for all workers to finish with a timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if s.logger != nil {
			s.logger.Info("Scheduler stopped gracefully")
		}
	case <-ctx.Done():
		if s.logger != nil {
			s.logger.Warn("Scheduler shutdown timed out")
		}
		return fmt.Errorf("scheduler shutdown timed out")
	case <-cronCtx.Done():
		if s.logger != nil {
			s.logger.Info("Cron scheduler stopped")
		}
	}

	s.isStarted = false
	return nil
}

// worker processes jobs from the queue
func (s *Scheduler) worker(id int) {
	defer s.wg.Done()

	if s.logger != nil {
		s.logger.Debug("Starting worker", "id", id)
	}

	for {
		select {
		case <-s.ctx.Done():
			if s.logger != nil {
				s.logger.Debug("Worker stopping", "id", id)
			}
			return
		case job := <-s.jobQueue:
			s.executeJob(job)
		}
	}
}

// executeJob runs a job and records its execution
func (s *Scheduler) executeJob(job Job) {
	if s.logger != nil {
		s.logger.Debug("Executing job", "id", job.ID, "name", job.Name)
	}

	// Update job status to running
	job.Status = JobStatusRunning
	job.UpdatedAt = time.Now()
	s.jobStore.UpdateJob(job)

	// Create execution record
	execution := JobExecution{
		JobID:     job.ID,
		StartTime: time.Now(),
		Status:    string(JobStatusRunning),
	}
	s.jobStore.AddJobExecution(execution)

	// Execute the job
	jobCtx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	var err error
	if job.JobFunc != nil {
		err = job.JobFunc(jobCtx)
	}

	// Update execution record
	execution.EndTime = time.Now()
	if err != nil {
		execution.Status = string(JobStatusFailed)
		execution.Error = err.Error()
		if s.logger != nil {
			s.logger.Error("Job execution failed", "id", job.ID, "name", job.Name, "error", err)
		}
	} else {
		execution.Status = string(JobStatusCompleted)
		if s.logger != nil {
			s.logger.Debug("Job execution completed", "id", job.ID, "name", job.Name)
		}
	}
	s.jobStore.UpdateJobExecution(execution)

	// Update job status and run times
	now := time.Now()
	job.LastRun = &now
	if err != nil {
		job.Status = JobStatusFailed
	} else {
		job.Status = JobStatusCompleted
	}

	// For non-recurring jobs, we're done
	if !job.IsRecurring {
		s.jobStore.UpdateJob(job)
		return
	}

	// For recurring jobs, calculate next run time
	schedule, err := cron.ParseStandard(job.Schedule)
	if err == nil {
		nextRun := schedule.Next(now)
		job.NextRun = &nextRun
		job.Status = JobStatusPending
	} else {
		if s.logger != nil {
			s.logger.Error("Failed to parse cron schedule", "schedule", job.Schedule, "error", err)
		}
	}

	s.jobStore.UpdateJob(job)
}

// dispatchPendingJobs checks for and dispatches pending jobs
func (s *Scheduler) dispatchPendingJobs() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkAndDispatchJobs()
		}
	}
}

// checkAndDispatchJobs checks for due jobs and dispatches them
func (s *Scheduler) checkAndDispatchJobs() {
	now := time.Now()
	dueJobs, err := s.jobStore.GetDueJobs(now)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to get due jobs", "error", err)
		}
		return
	}

	for _, job := range dueJobs {
		select {
		case s.jobQueue <- job:
			if s.logger != nil {
				s.logger.Debug("Dispatched job", "id", job.ID, "name", job.Name)
			}
		default:
			if s.logger != nil {
				s.logger.Warn("Job queue is full, job execution delayed", "id", job.ID, "name", job.Name)
			}
			// If queue is full, we'll try again next tick
		}
	}
}

// ScheduleJob schedules a new job
func (s *Scheduler) ScheduleJob(job Job) (string, error) {
	// Generate ID if not provided
	if job.ID == "" {
		job.ID = uuid.New().String()
	}

	// Set default values
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now
	job.Status = JobStatusPending

	// Validate job has either run time or schedule
	if job.RunAt.IsZero() && job.Schedule == "" {
		return "", fmt.Errorf("job must have either RunAt or Schedule specified")
	}

	// For recurring jobs, calculate next run time
	if job.IsRecurring {
		if job.Schedule == "" {
			return "", fmt.Errorf("recurring jobs must have a Schedule")
		}

		// Parse cron expression to verify and get next run
		schedule, err := cron.ParseStandard(job.Schedule)
		if err != nil {
			return "", fmt.Errorf("invalid cron expression '%s': %w", job.Schedule, err)
		}
		next := schedule.Next(now)
		job.NextRun = &next
	} else {
		job.NextRun = &job.RunAt
	}

	// Store the job
	err := s.jobStore.AddJob(job)
	if err != nil {
		return "", err
	}

	// Register with cron if recurring
	if job.IsRecurring && s.isStarted {
		s.registerWithCron(job)
	}

	return job.ID, nil
}

// registerWithCron registers a recurring job with the cron scheduler
func (s *Scheduler) registerWithCron(job Job) {
	s.entryMutex.Lock()
	defer s.entryMutex.Unlock()

	// Remove any existing entry
	if entryID, exists := s.cronEntries[job.ID]; exists {
		s.cronScheduler.Remove(entryID)
		delete(s.cronEntries, job.ID)
	}

	// Add to cron scheduler
	entryID, err := s.cronScheduler.AddFunc(job.Schedule, func() {
		retrievedJob, err := s.jobStore.GetJob(job.ID)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to retrieve job for cron execution", "id", job.ID, "error", err)
			}
			return
		}

		// Only queue if job still exists and isn't already running
		if retrievedJob.Status != JobStatusRunning {
			select {
			case s.jobQueue <- retrievedJob:
				if s.logger != nil {
					s.logger.Debug("Queued job from cron", "id", job.ID, "name", job.Name)
				}
			default:
				if s.logger != nil {
					s.logger.Warn("Job queue is full, cron job execution delayed", "id", job.ID, "name", job.Name)
				}
			}
		}
	})

	if err == nil {
		s.cronEntries[job.ID] = entryID
	} else if s.logger != nil {
		s.logger.Error("Failed to add job to cron scheduler", "id", job.ID, "error", err)
	}
}

// ScheduleRecurring schedules a recurring job using a cron expression
func (s *Scheduler) ScheduleRecurring(name string, cronExpr string, jobFunc JobFunc) (string, error) {
	job := Job{
		Name:        name,
		Schedule:    cronExpr,
		IsRecurring: true,
		JobFunc:     jobFunc,
	}
	return s.ScheduleJob(job)
}

// CancelJob cancels a scheduled job
func (s *Scheduler) CancelJob(jobID string) error {
	job, err := s.jobStore.GetJob(jobID)
	if err != nil {
		return err
	}

	// Update job status
	job.Status = JobStatusCancelled
	job.UpdatedAt = time.Now()
	err = s.jobStore.UpdateJob(job)
	if err != nil {
		return err
	}

	// Remove from cron if it's recurring
	if job.IsRecurring {
		s.entryMutex.Lock()
		if entryID, exists := s.cronEntries[jobID]; exists {
			s.cronScheduler.Remove(entryID)
			delete(s.cronEntries, jobID)
		}
		s.entryMutex.Unlock()
	}

	return nil
}

// GetJob returns information about a scheduled job
func (s *Scheduler) GetJob(jobID string) (Job, error) {
	return s.jobStore.GetJob(jobID)
}

// ListJobs returns a list of all scheduled jobs
func (s *Scheduler) ListJobs() ([]Job, error) {
	return s.jobStore.GetJobs()
}

// GetJobHistory returns the execution history for a job
func (s *Scheduler) GetJobHistory(jobID string) ([]JobExecution, error) {
	return s.jobStore.GetJobExecutions(jobID)
}