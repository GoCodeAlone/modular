package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryPersistenceHandler validates that the memory persistence handler works correctly
func TestMemoryPersistenceHandler(t *testing.T) {
	t.Run("SaveAndLoad", func(t *testing.T) {
		handler := NewMemoryPersistenceHandler()

		// Create test jobs
		job1 := Job{
			ID:      "job1",
			Name:    "Test Job 1",
			RunAt:   time.Now().Add(time.Hour),
			JobFunc: func(ctx context.Context) error { return nil },
		}
		job2 := Job{
			ID:      "job2",
			Name:    "Test Job 2",
			RunAt:   time.Now().Add(2 * time.Hour),
			JobFunc: func(ctx context.Context) error { return nil },
		}

		jobs := []Job{job1, job2}

		// Save jobs
		err := handler.Save(jobs)
		require.NoError(t, err)

		// Verify data was stored
		data := handler.GetStoredData()
		require.NotEmpty(t, data)

		// Load jobs
		loadedJobs, err := handler.Load()
		require.NoError(t, err)
		require.Len(t, loadedJobs, 2)

		// Verify job data (JobFunc will be nil after persistence)
		assert.Equal(t, "job1", loadedJobs[0].ID)
		assert.Equal(t, "Test Job 1", loadedJobs[0].Name)
		assert.Nil(t, loadedJobs[0].JobFunc) // Should be cleared during persistence

		assert.Equal(t, "job2", loadedJobs[1].ID)
		assert.Equal(t, "Test Job 2", loadedJobs[1].Name)
		assert.Nil(t, loadedJobs[1].JobFunc) // Should be cleared during persistence
	})

	t.Run("EmptyHandler", func(t *testing.T) {
		handler := NewMemoryPersistenceHandler()

		// Load from empty handler
		jobs, err := handler.Load()
		require.NoError(t, err)
		assert.Nil(t, jobs)

		// Check no data stored
		data := handler.GetStoredData()
		assert.Nil(t, data)
	})

	t.Run("Clear", func(t *testing.T) {
		handler := NewMemoryPersistenceHandler()

		// Save some data
		jobs := []Job{{ID: "test", Name: "Test Job"}}
		err := handler.Save(jobs)
		require.NoError(t, err)

		// Verify data exists
		data := handler.GetStoredData()
		require.NotEmpty(t, data)

		// Clear data
		handler.Clear()

		// Verify data is gone
		data = handler.GetStoredData()
		assert.Nil(t, data)

		// Verify load returns empty
		loadedJobs, err := handler.Load()
		require.NoError(t, err)
		assert.Nil(t, loadedJobs)
	})
}

// TestSchedulerWithCustomPersistenceHandler validates that the scheduler module works with custom persistence
func TestSchedulerWithCustomPersistenceHandler(t *testing.T) {
	// Create custom persistence handler
	persistenceHandler := NewMemoryPersistenceHandler()

	// Create module with custom persistence
	module := NewModule().(*SchedulerModule)
	app := newMockApp()

	// Configure with custom persistence
	config := &SchedulerConfig{
		WorkerCount:        2,
		QueueSize:          10,
		StorageType:        "memory",
		PersistenceBackend: PersistenceBackendCustom,
		PersistenceHandler: persistenceHandler,
		ShutdownTimeout:    1 * time.Second,
	}
	app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(config))

	// Initialize and start module
	err := module.Init(app)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = module.Start(ctx)
	require.NoError(t, err)

	// Schedule a job
	job := Job{
		Name:  "persistence-test",
		RunAt: time.Now().Add(24 * time.Hour), // Future job to remain pending
		JobFunc: func(ctx context.Context) error {
			return nil
		},
	}

	jobID, err := module.ScheduleJob(job)
	require.NoError(t, err)
	require.NotEmpty(t, jobID)

	// Stop module (should trigger save)
	err = module.Stop(ctx)
	require.NoError(t, err)

	// Verify job was persisted
	savedJobs, err := persistenceHandler.Load()
	require.NoError(t, err)
	require.Len(t, savedJobs, 1)
	assert.Equal(t, "persistence-test", savedJobs[0].Name)
}

// TestSchedulerNoPersistence validates that scheduler works without persistence
func TestSchedulerNoPersistence(t *testing.T) {
	module := NewModule().(*SchedulerModule)
	app := newMockApp()

	// Configure without persistence
	config := &SchedulerConfig{
		WorkerCount:        2,
		QueueSize:          10,
		StorageType:        "memory",
		PersistenceBackend: PersistenceBackendNone,
		ShutdownTimeout:    1 * time.Second,
	}
	app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(config))

	// Initialize and start module
	err := module.Init(app)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = module.Start(ctx)
	require.NoError(t, err)

	// Schedule a job
	job := Job{
		Name:  "no-persistence-test",
		RunAt: time.Now().Add(24 * time.Hour),
		JobFunc: func(ctx context.Context) error {
			return nil
		},
	}

	jobID, err := module.ScheduleJob(job)
	require.NoError(t, err)
	require.NotEmpty(t, jobID)

	// Stop module (should not try to persist)
	err = module.Stop(ctx)
	require.NoError(t, err)

	// This test mainly validates that no errors occur without persistence
}
