package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/require"
)

type blockingPersistenceHandler struct {
	entered chan struct{}
	release chan struct{}
	saves   atomic.Int32
}

func newBlockingPersistenceHandler() *blockingPersistenceHandler {
	return &blockingPersistenceHandler{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (h *blockingPersistenceHandler) Save([]Job) error {
	h.saves.Add(1)
	select {
	case <-h.entered:
	default:
		close(h.entered)
	}
	<-h.release
	return nil
}

func (h *blockingPersistenceHandler) Load() ([]Job, error) {
	return nil, nil
}

func TestSchedulerLifecyclePersistenceDoesNotStarveMetrics(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(context.Context, *SchedulerModule) error
	}{
		{name: "Stop", call: func(ctx context.Context, module *SchedulerModule) error {
			return module.Stop(ctx)
		}},
		{name: "PreStop", call: func(ctx context.Context, module *SchedulerModule) error {
			return module.PreStop(ctx)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := newBlockingPersistenceHandler()
			module := NewModule().(*SchedulerModule)
			app := newMockApp()
			config := &SchedulerConfig{
				WorkerCount:        1,
				QueueSize:          10,
				StorageType:        "memory",
				CheckInterval:      time.Hour,
				ShutdownTimeout:    time.Second,
				RetentionDays:      7,
				PersistenceBackend: PersistenceBackendCustom,
				PersistenceHandler: handler,
			}
			app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(config))
			require.NoError(t, module.Init(app))
			require.NoError(t, module.Start(context.Background()))
			_, err := module.ScheduleJob(Job{
				Name:    "persist-later",
				RunAt:   time.Now().Add(time.Hour),
				JobFunc: func(context.Context) error { return nil },
			})
			require.NoError(t, err)

			errCh := make(chan error, 1)
			go func() {
				errCh <- tc.call(context.Background(), module)
			}()

			select {
			case <-handler.entered:
			case <-time.After(time.Second):
				close(handler.release)
				t.Fatal("lifecycle call did not reach persistence handler")
			}

			metricsDone := make(chan struct{})
			go func() {
				_ = module.CollectMetrics(context.Background())
				close(metricsDone)
			}()

			select {
			case <-metricsDone:
			case <-time.After(100 * time.Millisecond):
				close(handler.release)
				t.Fatal("CollectMetrics blocked behind lifecycle persistence")
			}

			close(handler.release)
			require.NoError(t, <-errCh)
		})
	}
}

func TestSchedulerStopClaimsShutdownBeforePersistence(t *testing.T) {
	handler := newBlockingPersistenceHandler()
	module := NewModule().(*SchedulerModule)
	app := newMockApp()
	config := &SchedulerConfig{
		WorkerCount:        1,
		QueueSize:          10,
		StorageType:        "memory",
		CheckInterval:      time.Hour,
		ShutdownTimeout:    time.Second,
		RetentionDays:      7,
		PersistenceBackend: PersistenceBackendCustom,
		PersistenceHandler: handler,
	}
	app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(config))
	require.NoError(t, module.Init(app))
	require.NoError(t, module.Start(context.Background()))
	_, err := module.ScheduleJob(Job{
		Name:    "persist-later",
		RunAt:   time.Now().Add(time.Hour),
		JobFunc: func(context.Context) error { return nil },
	})
	require.NoError(t, err)

	firstStop := make(chan error, 1)
	go func() {
		firstStop <- module.Stop(context.Background())
	}()

	select {
	case <-handler.entered:
	case <-time.After(time.Second):
		close(handler.release)
		t.Fatal("first Stop did not reach persistence handler")
	}

	secondStop := make(chan error, 1)
	go func() {
		secondStop <- module.Stop(context.Background())
	}()

	select {
	case err := <-secondStop:
		require.NoError(t, err)
	case <-time.After(100 * time.Millisecond):
		close(handler.release)
		t.Fatal("second Stop blocked behind first Stop persistence")
	}

	if saves := handler.saves.Load(); saves != 1 {
		close(handler.release)
		t.Fatalf("expected one persistence save before release, got %d", saves)
	}

	close(handler.release)
	require.NoError(t, <-firstStop)
}
