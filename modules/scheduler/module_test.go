package scheduler

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"testing/synctest"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockApp struct {
	configSections map[string]modular.ConfigProvider
	logger         modular.Logger
	configProvider modular.ConfigProvider
	modules        []modular.Module
	services       modular.ServiceRegistry
}

func newMockApp() *mockApp {
	return &mockApp{
		configSections: make(map[string]modular.ConfigProvider),
		logger:         &mockLogger{},
		configProvider: &mockConfigProvider{},
		services:       make(modular.ServiceRegistry),
	}
}

func (a *mockApp) RegisterConfigSection(name string, provider modular.ConfigProvider) {
	a.configSections[name] = provider
}

func (a *mockApp) GetConfigSection(name string) (modular.ConfigProvider, error) {
	return a.configSections[name], nil
}

func (a *mockApp) ConfigSections() map[string]modular.ConfigProvider {
	return a.configSections
}

func (a *mockApp) Logger() modular.Logger {
	return a.logger
}

func (a *mockApp) SetLogger(logger modular.Logger) {
	a.logger = logger
}

func (a *mockApp) ConfigProvider() modular.ConfigProvider {
	return a.configProvider
}

func (a *mockApp) SvcRegistry() modular.ServiceRegistry {
	return a.services
}

func (a *mockApp) RegisterModule(module modular.Module) {
	a.modules = append(a.modules, module)
}

func (a *mockApp) RegisterService(name string, service any) error {
	a.services[name] = service
	return nil
}

func (a *mockApp) GetService(name string, target any) error {
	return nil
}

// New interface-introspection methods added to Application; provide minimal mock implementations
func (a *mockApp) GetServicesByModule(moduleName string) []string { return nil }
func (a *mockApp) GetServiceEntry(serviceName string) (*modular.ServiceRegistryEntry, bool) {
	return nil, false
}
func (a *mockApp) GetServicesByInterface(interfaceType reflect.Type) []*modular.ServiceRegistryEntry {
	return nil
}

// ServiceIntrospector returns nil for tests
func (a *mockApp) ServiceIntrospector() modular.ServiceIntrospector { return nil }

func (a *mockApp) Init() error {
	return nil
}

func (a *mockApp) Start() error {
	return nil
}

func (a *mockApp) Stop() error {
	return nil
}

func (a *mockApp) Run() error {
	return nil
}

func (a *mockApp) IsVerboseConfig() bool {
	return false
}

func (a *mockApp) SetVerboseConfig(verbose bool) {
	// No-op in mock
}

type mockLogger struct{}

func (l *mockLogger) Debug(msg string, args ...interface{}) {}
func (l *mockLogger) Info(msg string, args ...interface{})  {}
func (l *mockLogger) Warn(msg string, args ...interface{})  {}
func (l *mockLogger) Error(msg string, args ...interface{}) {}

type mockConfigProvider struct{}

func (m *mockConfigProvider) GetConfig() interface{} {
	return nil
}

func TestSchedulerModule(t *testing.T) {
	module := NewModule()
	assert.Equal(t, "scheduler", module.Name())

	// Test configuration registration
	app := newMockApp()
	err := module.(*SchedulerModule).RegisterConfig(app)
	require.NoError(t, err)

	// Test initialization
	err = module.(*SchedulerModule).Init(app)
	require.NoError(t, err)

	// Test services provided
	services := module.(*SchedulerModule).ProvidesServices()
	assert.Equal(t, 1, len(services))
	assert.Equal(t, ServiceName, services[0].Name)

	// Test module lifecycle
	ctx := context.Background()
	err = module.(*SchedulerModule).Start(ctx)
	require.NoError(t, err)

	err = module.(*SchedulerModule).Stop(ctx)
	require.NoError(t, err)
}

func TestSchedulerOperations(t *testing.T) {
	// Create the module
	module := NewModule().(*SchedulerModule)

	// Initialize with mock app
	app := newMockApp()
	module.RegisterConfig(app)
	module.Init(app)

	// Start the module
	ctx := context.Background()
	err := module.Start(ctx)
	require.NoError(t, err)

	t.Run("ScheduleOneTimeJob", func(t *testing.T) {
		executed := make(chan bool, 1)

		job := Job{
			Name:  "test-job",
			RunAt: time.Now().Add(100 * time.Millisecond),
			JobFunc: func(ctx context.Context) error {
				executed <- true
				return nil
			},
		}

		jobID, err := module.ScheduleJob(job)
		require.NoError(t, err)
		assert.NotEmpty(t, jobID)

		// Wait for job execution
		select {
		case <-executed:
			// Job executed successfully
		case <-time.After(2 * time.Second):
			t.Fatal("Job did not execute within timeout")
		}

		// Verify job can be retrieved
		retrievedJob, err := module.GetJob(jobID)
		require.NoError(t, err)
		assert.Equal(t, "test-job", retrievedJob.Name)
	})

	t.Run("ScheduleRecurringJob", func(t *testing.T) {
		executionCount := 0
		var mu sync.Mutex

		// Use a cron expression that runs every minute, but test differently
		jobID, err := module.ScheduleRecurring("recurring-test", "* * * * *", func(ctx context.Context) error {
			mu.Lock()
			executionCount++
			mu.Unlock()
			return nil
		})
		require.NoError(t, err)
		assert.NotEmpty(t, jobID)

		// Verify the job was created as recurring
		job, err := module.GetJob(jobID)
		require.NoError(t, err)
		assert.True(t, job.IsRecurring)
		assert.Equal(t, "* * * * *", job.Schedule)
		assert.Equal(t, "recurring-test", job.Name)

		// Cancel the job (we don't need to wait for execution for this test)
		err = module.CancelJob(jobID)
		require.NoError(t, err)

		// Verify job was cancelled
		job, err = module.GetJob(jobID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusCancelled, job.Status)
	})

	t.Run("CancelJob", func(t *testing.T) {
		executed := make(chan bool, 1)

		job := Job{
			Name:  "cancel-test",
			RunAt: time.Now().Add(1 * time.Second),
			JobFunc: func(ctx context.Context) error {
				executed <- true
				return nil
			},
		}

		jobID, err := module.ScheduleJob(job)
		require.NoError(t, err)

		// Cancel the job before it runs
		err = module.CancelJob(jobID)
		require.NoError(t, err)

		// Verify job was cancelled
		job, err = module.GetJob(jobID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusCancelled, job.Status)

		// Wait to ensure job doesn't execute
		select {
		case <-executed:
			t.Fatal("Cancelled job should not execute")
		case <-time.After(1500 * time.Millisecond):
			// Expected - job was cancelled
		}
	})

	t.Run("ListJobs", func(t *testing.T) {
		// Schedule a few jobs
		job1 := Job{
			Name:    "list-test-1",
			RunAt:   time.Now().Add(10 * time.Second),
			JobFunc: func(ctx context.Context) error { return nil },
		}
		job2 := Job{
			Name:    "list-test-2",
			RunAt:   time.Now().Add(20 * time.Second),
			JobFunc: func(ctx context.Context) error { return nil },
		}

		jobID1, err := module.ScheduleJob(job1)
		require.NoError(t, err)
		jobID2, err := module.ScheduleJob(job2)
		require.NoError(t, err)

		// List all jobs
		jobs, err := module.ListJobs()
		require.NoError(t, err)

		// Should contain our jobs
		foundJob1 := false
		foundJob2 := false
		for _, job := range jobs {
			if job.ID == jobID1 {
				foundJob1 = true
				assert.Equal(t, "list-test-1", job.Name)
			}
			if job.ID == jobID2 {
				foundJob2 = true
				assert.Equal(t, "list-test-2", job.Name)
			}
		}
		assert.True(t, foundJob1, "Job 1 should be in the list")
		assert.True(t, foundJob2, "Job 2 should be in the list")
	})

	t.Run("JobHistory", func(t *testing.T) {
		executed := make(chan bool, 1)

		job := Job{
			Name:  "history-test",
			RunAt: time.Now().Add(100 * time.Millisecond),
			JobFunc: func(ctx context.Context) error {
				executed <- true
				return nil
			},
		}

		jobID, err := module.ScheduleJob(job)
		require.NoError(t, err)

		// Wait for execution
		select {
		case <-executed:
		case <-time.After(2 * time.Second):
			t.Fatal("Job did not execute within timeout")
		}

		// Poll for job completion and history persistence
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			to, _ := module.GetJob(jobID)
			if to.Status == JobStatusCompleted {
				history, err := module.GetJobHistory(jobID)
				require.NoError(t, err)
				if len(history) == 1 && history[0].Status == string(JobStatusCompleted) {
					assert.Equal(t, jobID, history[0].JobID)
					return
				}
			}
			time.Sleep(25 * time.Millisecond)
		}
		to, _ := module.GetJob(jobID)
		t.Fatalf("Job history not stable; final status=%v", to.Status)
	})

	t.Run("JobFailure", func(t *testing.T) {
		executed := make(chan bool, 1)
		completedCh := make(chan bool, 1)

		job := Job{
			Name:  "failure-test",
			RunAt: time.Now().Add(100 * time.Millisecond),
			JobFunc: func(ctx context.Context) error {
				executed <- true
				return fmt.Errorf("intentional test failure")
			},
		}

		jobID, err := module.ScheduleJob(job)
		require.NoError(t, err)

		// Wait for execution
		select {
		case <-executed:
			// Give time for job execution to complete and update status
			go func() {
				time.Sleep(500 * time.Millisecond)
				completedCh <- true
			}()
		case <-time.After(2 * time.Second):
			t.Fatal("Job did not execute within timeout")
		}

		// Wait for job execution to complete
		select {
		case <-completedCh:
		case <-time.After(1 * time.Second):
			t.Fatal("Job execution didn't complete in time")
		}

		// Get job history
		history, err := module.GetJobHistory(jobID)
		require.NoError(t, err)
		assert.Len(t, history, 1, "Should have one execution record")
		assert.Equal(t, "failed", history[0].Status)
		assert.Contains(t, history[0].Error, "intentional test failure")
	})

	// Stop the module
	err = module.Stop(ctx)
	require.NoError(t, err)
}

// TestSchedulerImmediateJobSynctest demonstrates deterministic timing using synctest.
// It schedules a job 1s in the future and advances virtual time instantly.
func TestSchedulerImmediateJobSynctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Use standard module setup
		module := NewModule().(*SchedulerModule)
		app := newMockApp()
		module.RegisterConfig(app)
		module.Init(app)
		ctx := context.Background()
		executed := make(chan struct{}, 1)
		// Schedule job BEFORE starting so Start's initial due-job dispatch sees it.
		job := Job{
			Name:  "synctest-job",
			RunAt: time.Now(), // due immediately
			JobFunc: func(ctx context.Context) error {
				executed <- struct{}{}
				return nil
			},
		}
		if _, err := module.ScheduleJob(job); err != nil {
			t.Fatalf("schedule: %v", err)
		}
		if err := module.Start(ctx); err != nil {
			t.Fatalf("start: %v", err)
		}
		// Wait until goroutines settle.
		synctest.Wait()
		select {
		case <-executed:
		default:
			t.Fatalf("job did not execute in virtual time")
		}
		if err := module.Stop(ctx); err != nil {
			t.Fatalf("stop: %v", err)
		}
	})
}

func TestSchedulerConfiguration(t *testing.T) {
	module := NewModule().(*SchedulerModule)
	app := newMockApp()

	// Test with custom configuration
	config := &SchedulerConfig{
		WorkerCount:       10,
		QueueSize:         200,
		StorageType:       "memory",
		CheckInterval:     2 * time.Second,
		EnablePersistence: false,
	}
	app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(config))

	err := module.Init(app)
	require.NoError(t, err)

	// Verify configuration was applied
	assert.NotNil(t, module.scheduler)
	assert.Equal(t, config.WorkerCount, module.config.WorkerCount)
	assert.Equal(t, config.QueueSize, module.config.QueueSize)
}

func TestSchedulerServiceProvider(t *testing.T) {
	module := NewModule().(*SchedulerModule)
	app := newMockApp()

	module.RegisterConfig(app)
	module.Init(app)

	// Test service provides
	services := module.ProvidesServices()
	assert.Len(t, services, 1)
	assert.Equal(t, ServiceName, services[0].Name)
	assert.Equal(t, "Job scheduling service", services[0].Description)

	// Test required services
	required := module.RequiresServices()
	assert.Empty(t, required)
}

func TestJobPersistence(t *testing.T) {
	// Create temporary file for job persistence
	tempFile := fmt.Sprintf("/tmp/job-persistence-test-%d.json", time.Now().UnixNano())

	t.Run("SaveAndLoadJobs", func(t *testing.T) {
		// Create module with persistence enabled
		module := NewModule().(*SchedulerModule)
		app := newMockApp()

		// Override config for persistence and reduce shutdown timeout for test
		config := &SchedulerConfig{
			WorkerCount:       2,
			QueueSize:         10,
			StorageType:       "memory",
			EnablePersistence: true,
			PersistenceFile:   tempFile,
			ShutdownTimeout:   1 * time.Second, // Short timeout for test
		}
		app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(config))

		err := module.Init(app)
		require.NoError(t, err)

		// Start with a timeout context
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = module.Start(ctx)
		require.NoError(t, err)

		// Schedule some test jobs
		job1 := Job{
			Name:  "persistence-test-1",
			RunAt: time.Now().Add(24 * time.Hour), // Future job
			JobFunc: func(ctx context.Context) error {
				return nil
			},
		}

		job2 := Job{
			Name:        "persistence-test-2",
			Schedule:    "0 */2 * * *", // Every 2 hours
			IsRecurring: true,
			JobFunc: func(ctx context.Context) error {
				return nil
			},
		}

		jobID1, err := module.ScheduleJob(job1)
		require.NoError(t, err)

		jobID2, err := module.ScheduleRecurring(job2.Name, job2.Schedule, job2.JobFunc)
		require.NoError(t, err)

		// Stop the module (should save jobs)
		err = module.Stop(ctx)
		require.NoError(t, err)

		// Verify the file was created
		_, err = os.Stat(tempFile)
		require.NoError(t, err, "Persistence file should exist")

		// Create a new module to load the jobs
		newModule := NewModule().(*SchedulerModule)
		app = newMockApp()
		app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(config))

		err = newModule.Init(app)
		require.NoError(t, err)

		// No need to start the module to verify jobs were loaded

		// Verify jobs were loaded
		job, err := newModule.GetJob(jobID1)
		require.NoError(t, err)
		assert.Equal(t, "persistence-test-1", job.Name)
		assert.False(t, job.IsRecurring)

		job, err = newModule.GetJob(jobID2)
		require.NoError(t, err)
		assert.Equal(t, "persistence-test-2", job.Name)
		assert.True(t, job.IsRecurring)
		assert.Equal(t, "0 */2 * * *", job.Schedule)

		// Delete temp file
		err = os.Remove(tempFile)
		if err != nil && !os.IsNotExist(err) {
			t.Logf("Failed to remove temp file %s: %v", tempFile, err)
		}
	})
}
