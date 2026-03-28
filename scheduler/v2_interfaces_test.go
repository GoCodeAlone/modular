package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface checks
var (
	_ modular.MetricsProvider = (*SchedulerModule)(nil)
	_ modular.Drainable       = (*SchedulerModule)(nil)
)

func TestSchedulerModule_CollectMetrics(t *testing.T) {
	t.Run("not running, no jobs", func(t *testing.T) {
		module := NewModule().(*SchedulerModule)
		app := newMockApp()
		module.RegisterConfig(app)
		require.NoError(t, module.Init(app))

		metrics := module.CollectMetrics(context.Background())
		assert.Equal(t, ModuleName, metrics.Name)
		assert.Equal(t, 0.0, metrics.Values["running"])
		assert.Equal(t, float64(5), metrics.Values["worker_count"]) // default config
		assert.Equal(t, 0.0, metrics.Values["job_count"])
		assert.Equal(t, 0.0, metrics.Values["pending_jobs"])
	})

	t.Run("running with jobs", func(t *testing.T) {
		module := NewModule().(*SchedulerModule)
		app := newMockApp()
		config := &SchedulerConfig{
			WorkerCount:        3,
			QueueSize:          50,
			StorageType:        "memory",
			CheckInterval:      1 * time.Second,
			ShutdownTimeout:    5 * time.Second,
			RetentionDays:      7,
			PersistenceBackend: PersistenceBackendNone,
		}
		app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(config))
		require.NoError(t, module.Init(app))

		ctx := context.Background()
		require.NoError(t, module.Start(ctx))
		defer module.Stop(ctx) //nolint:errcheck

		// Schedule a pending job (far future so it stays pending)
		_, err := module.ScheduleJob(Job{
			Name:    "metrics-test-pending",
			RunAt:   time.Now().Add(24 * time.Hour),
			JobFunc: func(ctx context.Context) error { return nil },
		})
		require.NoError(t, err)

		// Schedule and cancel a job so we have mixed statuses
		cancelID, err := module.ScheduleJob(Job{
			Name:    "metrics-test-cancel",
			RunAt:   time.Now().Add(24 * time.Hour),
			JobFunc: func(ctx context.Context) error { return nil },
		})
		require.NoError(t, err)
		require.NoError(t, module.CancelJob(cancelID))

		metrics := module.CollectMetrics(ctx)
		assert.Equal(t, ModuleName, metrics.Name)
		assert.Equal(t, 1.0, metrics.Values["running"])
		assert.Equal(t, float64(3), metrics.Values["worker_count"])
		assert.Equal(t, 2.0, metrics.Values["job_count"])
		assert.Equal(t, 1.0, metrics.Values["pending_jobs"])
	})
}

func TestSchedulerModule_PreStop(t *testing.T) {
	t.Run("persists jobs on drain", func(t *testing.T) {
		handler := NewMemoryPersistenceHandler()
		module := NewModule().(*SchedulerModule)
		app := newMockApp()
		config := &SchedulerConfig{
			WorkerCount:        2,
			QueueSize:          10,
			StorageType:        "memory",
			CheckInterval:      1 * time.Second,
			ShutdownTimeout:    5 * time.Second,
			RetentionDays:      7,
			PersistenceBackend: PersistenceBackendMemory,
			PersistenceHandler: handler,
		}
		app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(config))
		require.NoError(t, module.Init(app))

		ctx := context.Background()
		require.NoError(t, module.Start(ctx))

		// Schedule a future job
		_, err := module.ScheduleJob(Job{
			Name:    "prestop-test",
			RunAt:   time.Now().Add(24 * time.Hour),
			JobFunc: func(ctx context.Context) error { return nil },
		})
		require.NoError(t, err)

		// Call PreStop — should save jobs and cancel dispatcher
		err = module.PreStop(ctx)
		require.NoError(t, err)

		// Verify persistence handler has data
		data := handler.GetStoredData()
		assert.NotEmpty(t, data, "PreStop should have persisted jobs")

		// Clean up — Stop() should still succeed even after PreStop cancelled the context
		_ = module.Stop(ctx)
	})

	t.Run("no persistence configured", func(t *testing.T) {
		module := NewModule().(*SchedulerModule)
		app := newMockApp()
		module.RegisterConfig(app)
		require.NoError(t, module.Init(app))

		ctx := context.Background()
		require.NoError(t, module.Start(ctx))

		// PreStop with no persistence should succeed (no-op for persistence)
		err := module.PreStop(ctx)
		require.NoError(t, err)

		_ = module.Stop(ctx)
	})
}
