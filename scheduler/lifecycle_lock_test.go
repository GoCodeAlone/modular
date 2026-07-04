package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/require"
)

type blockingPersistenceHandler struct {
	entered chan struct{}
	release chan struct{}
}

func newBlockingPersistenceHandler() *blockingPersistenceHandler {
	return &blockingPersistenceHandler{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (h *blockingPersistenceHandler) Save([]Job) error {
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
