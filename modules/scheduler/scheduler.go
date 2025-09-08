package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// Context key types to avoid collisions
type contextKey string

const (
	workerIDKey  contextKey = "worker_id"
	schedulerKey contextKey = "scheduler"
)

// Scheduler errors
var (
	ErrSchedulerShutdownTimeout  = errors.New("scheduler shutdown timed out")
	ErrJobInvalidSchedule        = errors.New("job must have either RunAt or Schedule specified")
	ErrRecurringJobNeedsSchedule = errors.New("recurring jobs must have a Schedule")
	ErrJobIDRequired             = errors.New("job ID must be provided when resuming a job")
	ErrJobNoValidNextRunTime     = errors.New("job has no valid next run time")
	ErrRecurringJobIDRequired    = errors.New("job ID must be provided when resuming a recurring job")
	ErrJobMustBeRecurring        = errors.New("job must be recurring and have a schedule")
)

// JobFunc defines a function that can be executed as a job
type JobFunc func(ctx context.Context) error

// EventEmitter interface for emitting events from the scheduler
type EventEmitter interface {
	EmitEvent(ctx context.Context, event cloudevents.Event) error
}

// JobExecution records details about a single execution of a job
type JobExecution struct {
	JobID     string    `json:"jobId"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime,omitempty"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
}

// JobBackfillPolicy defines how missed executions should be handled
type JobBackfillPolicy struct {
	Strategy            BackfillStrategy `json:"strategy"`
	MaxMissedExecutions int              `json:"maxMissedExecutions,omitempty"`
	MaxBackfillDuration time.Duration    `json:"maxBackfillDuration,omitempty"`
	Priority            int              `json:"priority,omitempty"`
}

// BackfillStrategy defines strategies for backfilling missed executions
type BackfillStrategy string

const (
	// BackfillStrategyAll missed executions
	BackfillStrategyAll BackfillStrategy = "all"
	// BackfillStrategyNone means don't backfill missed executions
	BackfillStrategyNone BackfillStrategy = "none"
	// BackfillStrategyLast means only backfill the last missed execution
	BackfillStrategyLast BackfillStrategy = "last"
	// BackfillStrategyBounded means backfill up to MaxMissedExecutions
	BackfillStrategyBounded BackfillStrategy = "bounded"
	// BackfillStrategyTimeWindow means backfill within MaxBackfillDuration
	BackfillStrategyTimeWindow BackfillStrategy = "time_window"
)

// Job represents a scheduled job
type Job struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Schedule       string                 `json:"schedule,omitempty"`
	RunAt          time.Time              `json:"runAt,omitempty"`
	IsRecurring    bool                   `json:"isRecurring"`
	JobFunc        JobFunc                `json:"-"`
	CreatedAt      time.Time              `json:"createdAt"`
	UpdatedAt      time.Time              `json:"updatedAt"`
	Status         JobStatus              `json:"status"`
	LastRun        *time.Time             `json:"lastRun,omitempty"`
	NextRun        *time.Time             `json:"nextRun,omitempty"`
	MaxConcurrency int                    `json:"maxConcurrency,omitempty"` // T045: Max concurrent executions
	BackfillPolicy *JobBackfillPolicy     `json:"backfillPolicy,omitempty"` // T046: Backfill policy
	Metadata       map[string]interface{} `json:"metadata,omitempty"`       // T046: Job metadata
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
	eventEmitter   EventEmitter
	jobQueue       chan Job
	cronScheduler  *cron.Cron
	cronEntries    map[string]cron.EntryID
	entryMutex     sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	isStarted      bool
	schedulerMutex sync.Mutex

	// T045: Concurrency tracking for maxConcurrency enforcement
	runningJobs  map[string]int // jobID -> current execution count
	runningMutex sync.RWMutex   // protects runningJobs map

	// Catch-up configuration for T037
	catchUpConfig *CatchUpConfig
}

// debugEnabled returns true when SCHEDULER_DEBUG env var is set to a non-empty value
func debugEnabled() bool { return os.Getenv("SCHEDULER_DEBUG") != "" }

// dbg prints verbose scheduler debugging information when SCHEDULER_DEBUG is set
func dbg(format string, args ...interface{}) {
	if !debugEnabled() {
		return
	}
	ts := time.Now().Format(time.RFC3339Nano)
	// Render the message first to avoid placeholder issues
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[SCHEDULER_DEBUG %s] %s\n", ts, msg)
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

// WithEventEmitter sets the event emitter
func WithEventEmitter(emitter EventEmitter) SchedulerOption {
	return func(s *Scheduler) {
		s.eventEmitter = emitter
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
		runningJobs:   make(map[string]int), // T045: Initialize concurrency tracking
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
		//nolint:contextcheck // Context is passed through s.ctx field
		go s.worker(i)

		// Emit worker started event
		s.emitEvent(context.WithValue(ctx, workerIDKey, i), EventTypeWorkerStarted, map[string]interface{}{
			"worker_id":     i,
			"total_workers": s.workerCount,
		})
	}

	// Start cron scheduler
	s.cronScheduler.Start()

	// Start job dispatcher (explicit Add/go because dispatchPendingJobs manages Done)
	s.wg.Add(1)
	go s.dispatchPendingJobs()

	// Immediately check for due jobs (e.g., recovered from persistence) so execution resumes promptly
	dbg("Start: running initial due-jobs dispatch (checkInterval=%s)", s.checkInterval.String())
	s.checkAndDispatchJobs()

	s.isStarted = true

	// Emit scheduler started event
	s.emitEvent(context.WithValue(ctx, schedulerKey, "started"), EventTypeSchedulerStarted, map[string]interface{}{
		"worker_count":   s.workerCount,
		"queue_size":     s.queueSize,
		"check_interval": s.checkInterval.String(),
	})

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

	var shutdownErr error
	select {
	case <-done:
		if s.logger != nil {
			s.logger.Info("Scheduler stopped gracefully")
		}
	case <-ctx.Done():
		if s.logger != nil {
			s.logger.Warn("Scheduler shutdown timed out")
		}
		shutdownErr = ErrSchedulerShutdownTimeout
	case <-cronCtx.Done():
		if s.logger != nil {
			s.logger.Info("Cron scheduler stopped")
		}
	}

	s.isStarted = false

	// Emit scheduler stopped event
	s.emitEvent(context.WithValue(ctx, schedulerKey, "stopped"), EventTypeSchedulerStopped, map[string]interface{}{
		"worker_count": s.workerCount,
	})

	return shutdownErr
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

			// Emit worker stopped event
			s.emitEvent(context.Background(), EventTypeWorkerStopped, map[string]interface{}{
				"worker_id": id,
			})

			return
		case job := <-s.jobQueue:
			dbg("Worker %d: picked job id=%s name=%s nextRun=%v status=%s", id, job.ID, job.Name, job.NextRun, job.Status)
			// Emit worker busy event
			s.emitEvent(context.Background(), EventTypeWorkerBusy, map[string]interface{}{
				"worker_id": id,
				"job_id":    job.ID,
				"job_name":  job.Name,
			})

			s.executeJob(job)

			// Emit worker idle event
			s.emitEvent(context.Background(), EventTypeWorkerIdle, map[string]interface{}{
				"worker_id": id,
			})
			dbg("Worker %d: completed job id=%s", id, job.ID)
		}
	}
}

// executeJob runs a job and records its execution
func (s *Scheduler) executeJob(job Job) {
	// T045: Check maxConcurrency limit before executing
	if job.MaxConcurrency > 0 {
		s.runningMutex.Lock()
		currentCount := s.runningJobs[job.ID]
		if currentCount >= job.MaxConcurrency {
			s.runningMutex.Unlock()
			if s.logger != nil {
				s.logger.Warn("Job execution skipped - max concurrency reached",
					"id", job.ID, "current", currentCount, "max", job.MaxConcurrency)
			}
			// Emit event for maxConcurrency reached
			s.emitEvent(context.Background(), "job.max_concurrency_reached", map[string]interface{}{
				"job_id":          job.ID,
				"job_name":        job.Name,
				"current_count":   currentCount,
				"max_concurrency": job.MaxConcurrency,
			})
			return
		}
		s.runningJobs[job.ID] = currentCount + 1
		s.runningMutex.Unlock()

		// Ensure we decrement the counter when done
		defer func() {
			s.runningMutex.Lock()
			s.runningJobs[job.ID]--
			if s.runningJobs[job.ID] <= 0 {
				delete(s.runningJobs, job.ID)
			}
			s.runningMutex.Unlock()
		}()
	}

	if s.logger != nil {
		s.logger.Debug("Executing job", "id", job.ID, "name", job.Name)
	}

	// Emit job started event
	s.emitEvent(context.Background(), EventTypeJobStarted, map[string]interface{}{
		"job_id":     job.ID,
		"job_name":   job.Name,
		"start_time": time.Now().Format(time.RFC3339),
	})

	// Update job status to running
	job.Status = JobStatusRunning
	job.UpdatedAt = time.Now()
	if err := s.jobStore.UpdateJob(job); err != nil && s.logger != nil {
		s.logger.Warn("Failed to update job status to running", "jobID", job.ID, "error", err)
	}

	// Create execution record
	execution := JobExecution{
		JobID:     job.ID,
		StartTime: time.Now(),
		Status:    string(JobStatusRunning),
	}
	if err := s.jobStore.AddJobExecution(execution); err != nil && s.logger != nil {
		s.logger.Warn("Failed to add job execution record", "jobID", job.ID, "error", err)
	}

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

		// Emit job failed event
		s.emitEvent(context.Background(), EventTypeJobFailed, map[string]interface{}{
			"job_id":   job.ID,
			"job_name": job.Name,
			"error":    err.Error(),
			"end_time": time.Now().Format(time.RFC3339),
		})
	} else {
		execution.Status = string(JobStatusCompleted)
		if s.logger != nil {
			s.logger.Debug("Job execution completed", "id", job.ID, "name", job.Name)
		}

		// Emit job completed event
		s.emitEvent(context.Background(), EventTypeJobCompleted, map[string]interface{}{
			"job_id":   job.ID,
			"job_name": job.Name,
			"end_time": time.Now().Format(time.RFC3339),
			"duration": execution.EndTime.Sub(execution.StartTime).String(),
		})
	}
	if updateErr := s.jobStore.UpdateJobExecution(execution); updateErr != nil && s.logger != nil {
		s.logger.Warn("Failed to update job execution", "jobID", job.ID, "error", updateErr)
	}

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
		if err := s.jobStore.UpdateJob(job); err != nil && s.logger != nil {
			s.logger.Warn("Failed to update completed job", "jobID", job.ID, "error", err)
		}
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

	if err := s.jobStore.UpdateJob(job); err != nil && s.logger != nil {
		s.logger.Warn("Failed to update recurring job", "jobID", job.ID, "error", err)
	}
}

// T046: calculateBackfillJobs determines which missed executions should be backfilled
func (s *Scheduler) calculateBackfillJobs(job Job) []time.Time {
	if job.BackfillPolicy == nil || job.BackfillPolicy.Strategy == BackfillStrategyNone {
		return nil
	}

	// Parse cron schedule to calculate missed executions
	schedule, err := cron.ParseStandard(job.Schedule)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to parse cron schedule for backfill", "schedule", job.Schedule, "error", err)
		}
		return nil
	}

	now := time.Now()
	var missedTimes []time.Time

	// Calculate the time window to check for missed executions
	startTime := now
	if job.LastRun != nil {
		startTime = *job.LastRun
	} else {
		startTime = job.CreatedAt
	}

	// Apply time window limit if configured
	if job.BackfillPolicy.MaxBackfillDuration > 0 {
		earliestTime := now.Add(-job.BackfillPolicy.MaxBackfillDuration)
		if startTime.Before(earliestTime) {
			startTime = earliestTime
		}
	}

	// Find all scheduled times between startTime and now
	currentTime := startTime
	for currentTime.Before(now) {
		nextTime := schedule.Next(currentTime)
		if nextTime.After(now) {
			break
		}

		// Check if this execution was actually missed (within reason)
		if nextTime.Add(5 * time.Minute).Before(now) { // 5-minute grace period
			missedTimes = append(missedTimes, nextTime)
		}

		currentTime = nextTime
	}

	// Apply backfill strategy
	switch job.BackfillPolicy.Strategy {
	case BackfillStrategyLast:
		if len(missedTimes) > 0 {
			return missedTimes[len(missedTimes)-1:]
		}
		return nil

	case BackfillStrategyBounded:
		maxCount := job.BackfillPolicy.MaxMissedExecutions
		if maxCount <= 0 {
			maxCount = 5 // Default limit
		}
		if len(missedTimes) > maxCount {
			return missedTimes[len(missedTimes)-maxCount:]
		}
		return missedTimes

	case BackfillStrategyTimeWindow:
		// Already filtered by time window above
		return missedTimes

	default:
		return nil
	}
}

// T046: processBackfillJobs schedules backfill executions for missed jobs
func (s *Scheduler) processBackfillJobs(job Job, missedTimes []time.Time) {
	if len(missedTimes) == 0 {
		return
	}

	if s.logger != nil {
		s.logger.Info("Processing backfill jobs", "jobID", job.ID, "missedCount", len(missedTimes))
	}

	// Create backfill executions (usually run immediately)
	for _, missedTime := range missedTimes {
		backfillJob := job
		backfillJob.ID = fmt.Sprintf("%s-backfill-%d", job.ID, missedTime.Unix())
		backfillJob.RunAt = time.Now()  // Execute immediately
		backfillJob.IsRecurring = false // Backfill jobs are one-time
		backfillJob.Status = JobStatusPending

		// Add metadata to indicate this is a backfill execution
		if backfillJob.Metadata == nil {
			backfillJob.Metadata = make(map[string]interface{})
		}
		backfillJob.Metadata["is_backfill"] = true
		backfillJob.Metadata["original_schedule_time"] = missedTime.Format(time.RFC3339)
		backfillJob.Metadata["backfill_priority"] = job.BackfillPolicy.Priority

		// Store and queue the backfill job
		err := s.jobStore.AddJob(backfillJob)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to add backfill job", "originalJobID", job.ID, "error", err)
			}
			continue
		}

		// Queue for immediate execution (non-blocking)
		select {
		case s.jobQueue <- backfillJob:
			if s.logger != nil {
				s.logger.Debug("Queued backfill job", "jobID", backfillJob.ID, "originalSchedule", missedTime)
			}
		default:
			if s.logger != nil {
				s.logger.Warn("Job queue full, backfill job will be picked up in next cycle", "jobID", backfillJob.ID)
			}
		}
	}

	// Emit backfill event
	s.emitEvent(context.Background(), "job.backfill_processed", map[string]interface{}{
		"job_id":            job.ID,
		"job_name":          job.Name,
		"missed_count":      len(missedTimes),
		"backfill_strategy": string(job.BackfillPolicy.Strategy),
	})
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

// emitEvent is a helper method to emit events from the scheduler
func (s *Scheduler) emitEvent(ctx context.Context, eventType string, data map[string]interface{}) {
	if s.eventEmitter != nil {
		event := modular.NewCloudEvent(eventType, "scheduler-service", data, nil)
		if err := s.eventEmitter.EmitEvent(ctx, event); err != nil {
			if s.logger != nil {
				s.logger.Warn("Failed to emit scheduler event", "eventType", eventType, "error", err)
			}
		}
	}
}

// checkAndDispatchJobs checks for due jobs and dispatches them
func (s *Scheduler) checkAndDispatchJobs() {
	now := time.Now()
	dbg("Dispatcher: checking due jobs at %s", now.Format(time.RFC3339Nano))
	dueJobs, err := s.jobStore.GetDueJobs(now)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to get due jobs", "error", err)
		}
		dbg("Dispatcher: error retrieving due jobs: %v", err)
		return
	}

	if len(dueJobs) == 0 {
		dbg("Dispatcher: no due jobs found")
	} else {
		for _, j := range dueJobs {
			dbg("Dispatcher: due job id=%s name=%s nextRun=%v", j.ID, j.Name, j.NextRun)
		}
	}

	for _, job := range dueJobs {
		select {
		case s.jobQueue <- job:
			if s.logger != nil {
				s.logger.Debug("Dispatched job", "id", job.ID, "name", job.Name)
			}
			dbg("Dispatcher: queued job id=%s", job.ID)
		default:
			if s.logger != nil {
				s.logger.Warn("Job queue is full, job execution delayed", "id", job.ID, "name", job.Name)
			}
			dbg("Dispatcher: queue full for job id=%s", job.ID)
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
		return "", ErrJobInvalidSchedule
	}

	// For recurring jobs, calculate next run time
	if job.IsRecurring {
		if job.Schedule == "" {
			return "", ErrRecurringJobNeedsSchedule
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
		return "", fmt.Errorf("failed to add job to store: %w", err)
	}

	// Register with cron if recurring
	if job.IsRecurring && s.isStarted {
		s.registerWithCron(job)

		// T046: Process backfill if policy is configured
		if job.BackfillPolicy != nil && job.BackfillPolicy.Strategy != BackfillStrategyNone {
			missedTimes := s.calculateBackfillJobs(job)
			s.processBackfillJobs(job, missedTimes)
		}
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
		return fmt.Errorf("failed to get job for cancellation: %w", err)
	}

	// Update job status
	job.Status = JobStatusCancelled
	job.UpdatedAt = time.Now()
	err = s.jobStore.UpdateJob(job)
	if err != nil {
		return fmt.Errorf("failed to update job status to cancelled: %w", err)
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

	// Emit job cancelled event
	s.emitEvent(context.Background(), EventTypeJobCancelled, map[string]interface{}{
		"job_id":       job.ID,
		"job_name":     job.Name,
		"cancelled_at": time.Now().Format(time.RFC3339),
	})

	return nil
}

// GetJob returns information about a scheduled job
func (s *Scheduler) GetJob(jobID string) (Job, error) {
	job, err := s.jobStore.GetJob(jobID)
	if err != nil {
		return Job{}, fmt.Errorf("failed to get job: %w", err)
	}
	return job, nil
}

// ListJobs returns a list of all scheduled jobs
func (s *Scheduler) ListJobs() ([]Job, error) {
	jobs, err := s.jobStore.GetJobs()
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	return jobs, nil
}

// GetJobHistory returns the execution history for a job
func (s *Scheduler) GetJobHistory(jobID string) ([]JobExecution, error) {
	history, err := s.jobStore.GetJobExecutions(jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job history: %w", err)
	}
	return history, nil
}

// ResumeJob resumes a persisted job
func (s *Scheduler) ResumeJob(job Job) (string, error) {
	if job.ID == "" {
		return "", ErrJobIDRequired
	}

	// Set status to pending
	job.Status = JobStatusPending
	job.UpdatedAt = time.Now()

	// Validate the job has a next run time
	if job.NextRun == nil {
		// If no next run is set, use the original RunAt time if it's in the future
		if !job.RunAt.IsZero() && job.RunAt.After(time.Now()) {
			job.NextRun = &job.RunAt
		} else {
			// Otherwise, job can't be resumed (would run immediately)
			return "", ErrJobNoValidNextRunTime
		}
	}

	// Store the job
	err := s.jobStore.UpdateJob(job)
	if err != nil {
		return "", fmt.Errorf("failed to update job for resume: %w", err)
	}

	return job.ID, nil
}

// ResumeRecurringJob resumes a persisted recurring job, registering it with the cron scheduler
func (s *Scheduler) ResumeRecurringJob(job Job) (string, error) {
	if job.ID == "" {
		return "", ErrRecurringJobIDRequired
	}

	if !job.IsRecurring || job.Schedule == "" {
		return "", ErrJobMustBeRecurring
	}

	// Set status to pending
	job.Status = JobStatusPending
	job.UpdatedAt = time.Now()

	// Calculate next run time
	schedule, err := cron.ParseStandard(job.Schedule)
	if err != nil {
		return "", fmt.Errorf("invalid cron expression '%s': %w", job.Schedule, err)
	}

	next := schedule.Next(time.Now())
	job.NextRun = &next

	// Store the job
	err = s.jobStore.UpdateJob(job)
	if err != nil {
		return "", fmt.Errorf("failed to update job for reschedule: %w", err)
	}

	// Register with cron if running
	if s.isStarted {
		s.registerWithCron(job)
	}

	return job.ID, nil
}

// ApplyOption applies a scheduler option to the scheduler
func (s *Scheduler) ApplyOption(option SchedulerOption) error {
	option(s)
	return nil
}

// IsCatchUpEnabled returns whether catch-up is enabled for this scheduler
func (s *Scheduler) IsCatchUpEnabled() bool {
	if s.catchUpConfig == nil {
		return false
	}
	return s.catchUpConfig.Enabled
}
