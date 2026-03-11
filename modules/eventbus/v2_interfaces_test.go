package eventbus

import (
	"context"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper that creates, inits, and starts an EventBusModule.
func setupModule(t *testing.T) *EventBusModule {
	t.Helper()
	module := NewModule().(*EventBusModule)
	app := newMockApp()
	require.NoError(t, module.RegisterConfig(app))
	require.NoError(t, module.Init(app))
	require.NoError(t, module.Start(context.Background()))
	return module
}

func TestEventBusModule_CollectMetrics(t *testing.T) {
	module := setupModule(t)
	defer func() { _ = module.Stop(context.Background()) }()

	ctx := context.Background()

	// Before any activity the counters should be zero.
	metrics := module.CollectMetrics(ctx)
	assert.Equal(t, module.Name(), metrics.Name)
	assert.Equal(t, float64(0), metrics.Values["delivered_count"])
	assert.Equal(t, float64(0), metrics.Values["dropped_count"])
	assert.Equal(t, float64(0), metrics.Values["topic_count"])
	assert.Equal(t, float64(0), metrics.Values["subscriber_count"])

	// Subscribe + publish so counters move.
	received := make(chan struct{}, 1)
	sub, err := module.Subscribe(ctx, "metrics.test", func(_ context.Context, _ Event) error {
		received <- struct{}{}
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, module.Publish(ctx, "metrics.test", map[string]any{"k": "v"}))

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event delivery")
	}

	metrics = module.CollectMetrics(ctx)
	assert.Equal(t, float64(1), metrics.Values["delivered_count"])
	assert.Equal(t, float64(0), metrics.Values["dropped_count"])
	assert.Equal(t, float64(1), metrics.Values["topic_count"])
	assert.Equal(t, float64(1), metrics.Values["subscriber_count"])

	_ = module.Unsubscribe(ctx, sub)
}

func TestEventBusModule_CollectMetrics_InterfaceCompliance(t *testing.T) {
	var _ modular.MetricsProvider = (*EventBusModule)(nil)
}

func TestEventBusModule_PreStop(t *testing.T) {
	module := setupModule(t)
	defer func() { _ = module.Stop(context.Background()) }()

	// PreStop should succeed without error.
	err := module.PreStop(context.Background())
	assert.NoError(t, err)

	// Module should still be operational after PreStop (Stop handles actual shutdown).
	ctx := context.Background()
	received := make(chan struct{}, 1)
	sub, err := module.Subscribe(ctx, "prestop.test", func(_ context.Context, _ Event) error {
		received <- struct{}{}
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, module.Publish(ctx, "prestop.test", map[string]any{"a": 1}))

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out — module should still work after PreStop")
	}

	_ = module.Unsubscribe(ctx, sub)
}

func TestEventBusModule_PreStop_InterfaceCompliance(t *testing.T) {
	var _ modular.Drainable = (*EventBusModule)(nil)
}
