package eventbus

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkerPoolBufferSize verifies that the worker pool task queue has capacity
// equal to MaxEventQueueSize, not WorkerCount.  With a single slow worker and
// MaxEventQueueSize=50 we publish 40 events; all 40 should be delivered without
// being dropped, because the larger queue absorbs the burst while the single
// worker drains it.
func TestWorkerPoolBufferSize(t *testing.T) {
	module := NewModule().(*EventBusModule)
	app := newMockApp()
	cfg := &EventBusConfig{
		Engine:                 "memory",
		WorkerCount:            1, // only one worker — intentionally slow
		DefaultEventBufferSize: 64,
		MaxEventQueueSize:      50, // queue depth for the worker pool task queue
		DeliveryMode:           "drop",
	}
	app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(cfg))
	require.NoError(t, module.Init(app))
	ctx := context.Background()
	require.NoError(t, module.Start(ctx))
	defer module.Stop(ctx) //nolint:errcheck

	var received int64
	_, err := module.SubscribeAsync(ctx, "burst.topic", func(ctx context.Context, e Event) error {
		// Intentionally slow handler so the single worker is always busy
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt64(&received, 1)
		return nil
	})
	require.NoError(t, err)

	// Publish fewer events than MaxEventQueueSize so none should be dropped
	const publish = 40
	for i := 0; i < publish; i++ {
		require.NoError(t, module.Publish(ctx, "burst.topic", map[string]int{"i": i}))
	}

	// Wait long enough for the single worker to drain all 40 tasks (40 * 10ms = 400ms)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&received) >= publish {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	delivered, dropped := module.Stats()
	assert.Equal(t, int64(publish), atomic.LoadInt64(&received),
		"all published events should be delivered (delivered=%d dropped=%d)", delivered, dropped)
	assert.Zero(t, dropped,
		"no events should be dropped when publish count < MaxEventQueueSize (delivered=%d dropped=%d)", delivered, dropped)
}

// TestEventHistoryCap verifies that storeEventHistory does not grow the per-topic
// history slice beyond MaxEventQueueSize entries, preventing OOM under high volume.
func TestEventHistoryCap(t *testing.T) {
	const maxQueue = 10
	bus := NewMemoryEventBus(&EventBusConfig{
		MaxEventQueueSize:      maxQueue,
		DefaultEventBufferSize: 64,
		WorkerCount:            2,
		RetentionDays:          7,
		DeliveryMode:           "drop",
	})
	ctx := context.Background()
	require.NoError(t, bus.Start(ctx))
	defer bus.Stop(ctx) //nolint:errcheck

	// Subscribe so Publish has somewhere to route
	_, err := bus.Subscribe(ctx, "hist.topic", func(ctx context.Context, e Event) error { return nil })
	require.NoError(t, err)

	// Publish twice the cap using a CloudEvent
	for i := 0; i < maxQueue*2; i++ {
		evt := cloudevents.NewEvent()
		evt.SetType("hist.topic")
		evt.SetSource("test")
		require.NoError(t, bus.Publish(ctx, evt))
	}

	// Inspect history directly (package-internal access)
	bus.historyMutex.RLock()
	histLen := len(bus.eventHistory["hist.topic"])
	bus.historyMutex.RUnlock()

	assert.LessOrEqual(t, histLen, maxQueue,
		"event history length %d must not exceed MaxEventQueueSize %d", histLen, maxQueue)
}
