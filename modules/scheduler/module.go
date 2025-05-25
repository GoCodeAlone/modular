package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
)

// ModuleName is the name of this module
const ModuleName = "scheduler"

// ServiceName is the name of the service provided by this module
const ServiceName = "scheduler.provider"

// SchedulerModule represents the scheduler module
type SchedulerModule struct {
	name          string
	config        *SchedulerConfig
	logger        modular.Logger
	scheduler     *Scheduler
	jobStore      JobStore
	running       bool
	schedulerLock sync.Mutex
}

// NewModule creates a new instance of the scheduler module
func NewModule() modular.Module {
	return &SchedulerModule{
		name: ModuleName,
	}
}

// Name returns the name of the module
func (m *SchedulerModule) Name() string {
	return m.name
}

// RegisterConfig registers the module's configuration structure
func (m *SchedulerModule) RegisterConfig(app modular.Application) error {
	// Register the configuration with default values
	defaultConfig := &SchedulerConfig{
		WorkerCount:       5,
		QueueSize:         100,
		ShutdownTimeout:   30,
		StorageType:       "memory",
		CheckInterval:     1,
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
		WithCheckInterval(time.Duration(m.config.CheckInterval)*time.Second),
		WithLogger(m.logger),
	)

	// Load persisted jobs if enabled
	if m.config.EnablePersistence {
		m.logger.Info("Job persistence is enabled, but loading functionality is not implemented yet",
			"file", m.config.PersistenceFile)
		// TODO: Implement job persistence loading from file
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

	m.running = true
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
	shutdownCtx, cancel := context.WithTimeout(ctx, time.Duration(m.config.ShutdownTimeout)*time.Second)
	defer cancel()

	// Stop the scheduler
	err := m.scheduler.Stop(shutdownCtx)
	if err != nil {
		return err
	}

	// Save pending jobs if persistence is enabled
	if m.config.EnablePersistence {
		m.logger.Info("Persistence enabled, but saving jobs is not implemented yet",
			"file", m.config.PersistenceFile)
		// TODO: Implement job persistence saving to file
	}

	m.running = false
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
	return m.scheduler.ScheduleJob(job)
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
