// Package scheduler provides job scheduling and task execution capabilities for the modular framework.
//
// This module implements a flexible job scheduler that supports both immediate and scheduled
// job execution, configurable worker pools, job persistence, and comprehensive job lifecycle
// management. It's designed for reliable background task processing in web applications and services.
//
// # Features
//
// The scheduler module provides the following capabilities:
//   - Immediate and scheduled job execution
//   - Configurable worker pools for concurrent processing
//   - Job persistence with multiple storage backends
//   - Job status tracking and lifecycle management
//   - Automatic job cleanup and retention policies
//   - Service interface for dependency injection
//   - Thread-safe operations for concurrent access
//
// # Service Registration
//
// The module registers a scheduler service for dependency injection:
//
//	// Get the scheduler service
//	scheduler := app.GetService("scheduler.provider").(*SchedulerModule)
//
//	// Schedule immediate job
//	job := scheduler.ScheduleJob("process-data", processDataFunc, time.Now())
//
//	// Schedule delayed job
//	futureTime := time.Now().Add(time.Hour)
//	job := scheduler.ScheduleJob("cleanup", cleanupFunc, futureTime)
//
// # Usage Examples
//
// Basic job scheduling:
//
//	// Define a job function
//	emailJob := func(ctx context.Context) error {
//	    return sendEmail("user@example.com", "Welcome!")
//	}
//
//	// Schedule immediate execution
//	job := scheduler.ScheduleJob("send-welcome-email", emailJob, time.Now())
//
//	// Schedule for later
//	scheduledTime := time.Now().Add(time.Minute * 30)
//	job := scheduler.ScheduleJob("send-reminder", reminderJob, scheduledTime)
//
// Job with custom options:
//
//	// Create scheduler with custom options
//	customScheduler := NewScheduler(
//	    jobStore,
//	    WithWorkerCount(10),
//	    WithQueueSize(500),
//	    WithCheckInterval(time.Second * 5),
//	)
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Module errors
var (
	ErrJobStoreNotPersistable = errors.New("job store does not implement PersistableJobStore interface")
)

// ModuleName is the unique identifier for the scheduler module.
const ModuleName = "scheduler"

// ServiceName is the name of the service provided by this module.
// Other modules can use this name to request the scheduler service through dependency injection.
const ServiceName = "scheduler.provider"

// SchedulerModule provides job scheduling and task execution capabilities.
// It manages a pool of worker goroutines that execute scheduled jobs and
// provides persistence and lifecycle management for jobs.
//
// The module implements the following interfaces:
//   - modular.Module: Basic module lifecycle
//   - modular.Configurable: Configuration management
//   - modular.ServiceAware: Service dependency management
//   - modular.Startable: Startup logic
//   - modular.Stoppable: Shutdown logic
//
// Job execution is thread-safe and supports concurrent job processing.
type SchedulerModule struct {
	name          string
	config        *SchedulerConfig
	logger        modular.Logger
	scheduler     *Scheduler
	jobStore      JobStore
	running       bool
	schedulerLock sync.Mutex
	subject       modular.Subject // Added for event observation
}

// NewModule creates a new instance of the scheduler module.
// This is the primary constructor for the scheduler module and should be used
// when registering the module with the application.
//
// Example:
//
//	app.RegisterModule(scheduler.NewModule())
func NewModule() modular.Module {
	return &SchedulerModule{
		name: ModuleName,
	}
}

// Name returns the unique identifier for this module.
// This name is used for service registration, dependency resolution,
// and configuration section identification.
func (m *SchedulerModule) Name() string {
	return m.name
}

// RegisterConfig registers the module's configuration structure.
// This method is called during application initialization to register
// the default configuration values for the scheduler module.
//
// Default configuration:
//   - WorkerCount: 5 worker goroutines
//   - QueueSize: 100 job queue capacity
//   - ShutdownTimeout: 30s for graceful shutdown
//   - StorageType: "memory" storage backend
//   - CheckInterval: 1s for job polling
//   - RetentionDays: 7 days for completed job retention
func (m *SchedulerModule) RegisterConfig(app modular.Application) error {
	// If a non-nil config provider is already registered (e.g., tests), don't override it
	if existing, err := app.GetConfigSection(m.Name()); err == nil && existing != nil {
		return nil
	}

	// Register the configuration with default values
	defaultConfig := &SchedulerConfig{
		WorkerCount:       5,
		QueueSize:         100,
		ShutdownTimeout:   30 * time.Second,
		StorageType:       "memory",
		CheckInterval:     1 * time.Second, // Fast for unit tests
		RetentionDays:     7,
		PersistenceFile:   "scheduler_jobs.json",
		EnablePersistence: false,
	}

	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(defaultConfig))
	return nil
}

// Init initializes the module
func (m *SchedulerModule) Init(app modular.Application) error {
	// Retrieve the registered config section for access
	cfg, err := app.GetConfigSection(m.name)
	if err != nil {
		return fmt.Errorf("failed to get config section '%s': %w", m.name, err)
	}

	m.config = cfg.GetConfig().(*SchedulerConfig)
	m.logger = app.Logger()

	// Emit config loaded event
	m.emitEvent(context.Background(), EventTypeConfigLoaded, map[string]interface{}{
		"worker_count":       m.config.WorkerCount,
		"queue_size":         m.config.QueueSize,
		"shutdown_timeout":   m.config.ShutdownTimeout.String(),
		"storage_type":       m.config.StorageType,
		"check_interval":     m.config.CheckInterval.String(),
		"retention_days":     m.config.RetentionDays,
		"enable_persistence": m.config.EnablePersistence,
	})

	// Initialize job store based on configuration
	switch m.config.StorageType {
	case "memory":
		m.jobStore = NewMemoryJobStore(time.Duration(m.config.RetentionDays) * 24 * time.Hour)
		m.logger.Info("Using memory job store")
	default:
		m.jobStore = NewMemoryJobStore(time.Duration(m.config.RetentionDays) * 24 * time.Hour)
		m.logger.Warn("Unknown storage type, using memory job store", "specified", m.config.StorageType)
	}

	// Initialize the scheduler
	m.scheduler = NewScheduler(
		m.jobStore,
		WithWorkerCount(m.config.WorkerCount),
		WithQueueSize(m.config.QueueSize),
		WithCheckInterval(m.config.CheckInterval),
		WithLogger(m.logger),
		WithEventEmitter(m),
	)

	// Load persisted jobs if enabled
	if m.config.EnablePersistence {
		err := m.loadPersistedJobs()
		if err != nil {
			m.logger.Error("Failed to load persisted jobs", "error", err, "file", m.config.PersistenceFile)
			// Non-fatal error, continue with initialization
		}
	}

	m.logger.Info("Scheduler module initialized")
	return nil
}

// Start performs startup logic for the module
func (m *SchedulerModule) Start(ctx context.Context) error {
	m.logger.Info("Starting scheduler module")

	m.schedulerLock.Lock()
	defer m.schedulerLock.Unlock()

	if m.running {
		return nil
	}

	// Start the scheduler
	err := m.scheduler.Start(ctx)
	if err != nil {
		return err
	}

	// Ensure a scheduler started event is emitted at module level as well
	m.emitEvent(ctx, EventTypeSchedulerStarted, map[string]interface{}{
		"worker_count":   m.config.WorkerCount,
		"queue_size":     m.config.QueueSize,
		"check_interval": m.config.CheckInterval.String(),
	})

	m.running = true

	// Emit module started event
	m.emitEvent(ctx, EventTypeModuleStarted, map[string]interface{}{
		"worker_count": m.config.WorkerCount,
		"queue_size":   m.config.QueueSize,
		"storage_type": m.config.StorageType,
	})

	m.logger.Info("Scheduler started successfully")
	return nil
}

// Stop performs shutdown logic for the module
func (m *SchedulerModule) Stop(ctx context.Context) error {
	m.logger.Info("Stopping scheduler module")

	m.schedulerLock.Lock()
	defer m.schedulerLock.Unlock()

	if !m.running {
		return nil
	}

	// Create a context with timeout for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, m.config.ShutdownTimeout)
	defer cancel()

	// Save pending jobs before stopping to ensure recovery even if jobs execute during shutdown
	if m.config.EnablePersistence {
		if preSaveErr := m.savePersistedJobs(); preSaveErr != nil {
			if m.logger != nil {
				m.logger.Warn("Pre-stop save of jobs failed", "error", preSaveErr, "file", m.config.PersistenceFile)
			}
		}
	}

	// Stop the scheduler
	err := m.scheduler.Stop(shutdownCtx)

	// Save pending jobs if persistence is enabled (even if stop errored)
	if m.config.EnablePersistence {
		if saveErr := m.savePersistedJobs(); saveErr != nil {
			if m.logger != nil {
				m.logger.Error("Failed to save jobs to persistence file", "error", saveErr, "file", m.config.PersistenceFile)
			}
		}
	}

	if err != nil {
		return err
	}

	m.running = false

	// Emit module stopped event
	m.emitEvent(ctx, EventTypeModuleStopped, map[string]interface{}{
		"worker_count": m.config.WorkerCount,
		"jobs_saved":   m.config.EnablePersistence,
	})

	m.logger.Info("Scheduler stopped")
	return nil
}

// Dependencies returns the names of modules this module depends on
func (m *SchedulerModule) Dependencies() []string {
	return nil
}

// ProvidesServices declares services provided by this module
func (m *SchedulerModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        ServiceName,
			Description: "Job scheduling service",
			Instance:    m,
		},
	}
}

// RequiresServices declares services required by this module
func (m *SchedulerModule) RequiresServices() []modular.ServiceDependency {
	return nil
}

// Constructor provides a dependency injection constructor for the module
func (m *SchedulerModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		return m, nil
	}
}

// ScheduleJob schedules a new job
func (m *SchedulerModule) ScheduleJob(job Job) (string, error) {
	jobID, err := m.scheduler.ScheduleJob(job)
	if err != nil {
		return "", err
	}

	// Emit job scheduled event
	m.emitEvent(context.Background(), EventTypeJobScheduled, map[string]interface{}{
		"job_id":        jobID,
		"job_name":      job.Name,
		"schedule_time": job.RunAt.Format(time.RFC3339),
		"is_recurring":  job.IsRecurring,
	})

	return jobID, nil
}

// ScheduleRecurring schedules a recurring job using a cron expression
func (m *SchedulerModule) ScheduleRecurring(name string, cronExpr string, jobFunc JobFunc) (string, error) {
	return m.scheduler.ScheduleRecurring(name, cronExpr, jobFunc)
}

// CancelJob cancels a scheduled job
func (m *SchedulerModule) CancelJob(jobID string) error {
	return m.scheduler.CancelJob(jobID)
}

// GetJob returns information about a scheduled job
func (m *SchedulerModule) GetJob(jobID string) (Job, error) {
	return m.scheduler.GetJob(jobID)
}

// ListJobs returns a list of all scheduled jobs
func (m *SchedulerModule) ListJobs() ([]Job, error) {
	return m.scheduler.ListJobs()
}

// GetJobHistory returns the execution history for a job
func (m *SchedulerModule) GetJobHistory(jobID string) ([]JobExecution, error) {
	return m.scheduler.GetJobHistory(jobID)
}

// loadPersistedJobs loads jobs from the persistence file
func (m *SchedulerModule) loadPersistedJobs() error {
	m.logger.Info("Loading persisted jobs", "file", m.config.PersistenceFile)

	// Use the job store's persistence methods if available
	if persistable, ok := m.jobStore.(PersistableJobStore); ok {
		jobs, err := persistable.LoadFromFile(m.config.PersistenceFile)
		if err != nil {
			return fmt.Errorf("failed to load jobs from persistence file: %w", err)
		}
		if debugEnabled() {
			dbg("LoadPersisted: loaded %d jobs from %s", len(jobs), m.config.PersistenceFile)
		}

		// Reinsert all relevant jobs into the fresh job store so the dispatcher can pick them up
		for _, job := range jobs {
			// Debug before normalization
			if debugEnabled() {
				preNR := "<nil>"
				if job.NextRun != nil {
					preNR = job.NextRun.Format(time.RFC3339Nano)
				}
				runAtStr := job.RunAt.Format(time.RFC3339Nano)
				dbg("LoadPersisted: job=%s name=%s status=%s runAt=%s nextRun=%s", job.ID, job.Name, job.Status, runAtStr, preNR)
			}
			// Skip already completed or cancelled jobs
			if job.Status == JobStatusCompleted || job.Status == JobStatusCancelled {
				continue
			}

			// Normalize NextRun so due jobs are picked up promptly after restart
			now := time.Now()
			if job.NextRun == nil {
				if !job.RunAt.IsZero() {
					// If run time already passed, schedule immediately; otherwise keep original RunAt
					if !job.RunAt.After(now) {
						nr := now
						job.NextRun = &nr
					} else {
						j := job.RunAt
						job.NextRun = &j
					}
				} else {
					// No scheduling info — set to now to avoid being stuck
					nr := now
					job.NextRun = &nr
				}
			} else if job.NextRun.Before(now) {
				// If persisted NextRun is in the past, schedule immediately
				nr := now
				job.NextRun = &nr
			} else {
				// If NextRun is very near-future (within 750ms), pull it to now to avoid timing flakes on restart
				if job.NextRun.Sub(now) <= 750*time.Millisecond {
					nr := now
					job.NextRun = &nr
				}
			}

			// Normalize status back to pending for rescheduled work
			job.Status = JobStatusPending
			job.UpdatedAt = time.Now()

			// Debug after normalization
			if debugEnabled() {
				postNR := "<nil>"
				if job.NextRun != nil {
					postNR = job.NextRun.Format(time.RFC3339Nano)
				}
				dbg("LoadPersisted: normalized job=%s status=%s nextRun=%s (now=%s)", job.ID, job.Status, postNR, now.Format(time.RFC3339Nano))
			}

			// Persist normalized job back into the store
			if err := m.scheduler.jobStore.UpdateJob(job); err != nil {
				// If job wasn't present (unexpected), attempt to add it
				if addErr := m.scheduler.jobStore.AddJob(job); addErr != nil {
					m.logger.Warn("Failed to persist normalized job to store", "jobID", job.ID, "updateErr", err, "addErr", addErr)
				}
			}
		}

		m.logger.Info("Loaded persisted jobs", "count", len(jobs))
		return nil
	}

	m.logger.Warn("Job store does not support persistence")
	return ErrJobStoreNotPersistable
}

// savePersistedJobs saves jobs to the persistence file
func (m *SchedulerModule) savePersistedJobs() error {
	m.logger.Info("Saving jobs to persistence file", "file", m.config.PersistenceFile)

	// Use the job store's persistence methods if available
	if persistable, ok := m.jobStore.(PersistableJobStore); ok {
		jobs, err := m.scheduler.ListJobs()
		if err != nil {
			return fmt.Errorf("failed to list jobs for persistence: %w", err)
		}

		err = persistable.SaveToFile(jobs, m.config.PersistenceFile)
		if err != nil {
			return fmt.Errorf("failed to save jobs to persistence file: %w", err)
		}

		m.logger.Info("Saved jobs to persistence file", "count", len(jobs))
		if debugEnabled() {
			dbg("SavePersisted: saved %d jobs to %s", len(jobs), m.config.PersistenceFile)
		}
		return nil
	}

	m.logger.Warn("Job store does not support persistence")
	return ErrJobStoreNotPersistable
}

// RegisterObservers implements the ObservableModule interface.
// This allows the scheduler module to register as an observer for events it's interested in.
func (m *SchedulerModule) RegisterObservers(subject modular.Subject) error {
	m.subject = subject
	return nil
}

// EmitEvent implements the ObservableModule interface.
// This allows the scheduler module to emit events that other modules or observers can receive.
func (m *SchedulerModule) EmitEvent(ctx context.Context, event cloudevents.Event) error {
	if m.subject == nil {
		return ErrNoSubjectForEventEmission
	}
	if err := m.subject.NotifyObservers(ctx, event); err != nil {
		return fmt.Errorf("failed to notify observers: %w", err)
	}
	return nil
}

// emitEvent is a helper method to create and emit CloudEvents for the scheduler module.
// This centralizes the event creation logic and ensures consistent event formatting.
// If no subject is available for event emission, it silently skips the event emission
// to avoid noisy error messages in tests and non-observable applications.
func (m *SchedulerModule) emitEvent(ctx context.Context, eventType string, data map[string]interface{}) {
	// Skip event emission if no subject is available (non-observable application)
	if m.subject == nil {
		return
	}

	event := modular.NewCloudEvent(eventType, "scheduler-service", data, nil)

	if emitErr := m.EmitEvent(ctx, event); emitErr != nil {
		// If no subject is registered, quietly skip to allow non-observable apps to run cleanly
		if errors.Is(emitErr, ErrNoSubjectForEventEmission) {
			return
		}
		// Use structured logger to avoid noisy stdout during tests
		if m.logger != nil {
			m.logger.Warn("Failed to emit scheduler event", "eventType", eventType, "error", emitErr)
		} else {
			// Fallback to stdout only when no logger is available
			fmt.Printf("Failed to emit scheduler event %s: %v\n", eventType, emitErr)
		}
	}
}

// GetRegisteredEventTypes implements the ObservableModule interface.
// Returns all event types that this scheduler module can emit.
func (m *SchedulerModule) GetRegisteredEventTypes() []string {
	return []string{
		EventTypeConfigLoaded,
		EventTypeConfigValidated,
		EventTypeJobScheduled,
		EventTypeJobStarted,
		EventTypeJobCompleted,
		EventTypeJobFailed,
		EventTypeJobCancelled,
		EventTypeJobRemoved,
		EventTypeSchedulerStarted,
		EventTypeSchedulerStopped,
		EventTypeSchedulerPaused,
		EventTypeSchedulerResumed,
		EventTypeWorkerStarted,
		EventTypeWorkerStopped,
		EventTypeWorkerBusy,
		EventTypeWorkerIdle,
		EventTypeModuleStarted,
		EventTypeModuleStopped,
		EventTypeError,
		EventTypeWarning,
	}
}
