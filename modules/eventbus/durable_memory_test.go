package eventbus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: build and start a DurableMemoryEventBus-backed EventBusModule.
func newDurableModule(t *testing.T, maxDepth int) *EventBusModule {
	t.Helper()
	module := NewModule().(*EventBusModule)
	app := newMockApp()
	cfg := &EventBusConfig{
		Engine:               "durable-memory",
		MaxEventQueueSize:    1000,
		MaxDurableQueueDepth: maxDepth,
	}
	app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(cfg))
	require.NoError(t, module.Init(app))
	require.NoError(t, module.Start(context.Background()))
	t.Cleanup(func() { _ = module.Stop(context.Background()) })
	return module
}

// helper: publish N events and return count of failures.
func publishN(t *testing.T, module *EventBusModule, topic string, n int) int {
	t.Helper()
	var failed int
	for i := 0; i < n; i++ {
		if err := module.Publish(context.Background(), topic, map[string]int{"i": i}); err != nil {
			failed++
		}
	}
	return failed
}

// TestDurableMemoryNoEventLoss verifies that every published event is delivered
// even when the subscriber is slow (handler sleeps).
// With MemoryEventBus/drop mode this test would see substantial drops; with the
// durable engine the publisher blocks until the subscriber processes each batch.
func TestDurableMemoryNoEventLoss(t *testing.T) {
	const (
		topic     = "durable.no-loss"
		total     = 200
		queueDepth = 20 // intentionally small to exercise backpressure
	)

	module := newDurableModule(t, queueDepth)

	var received int64
	_, err := module.Subscribe(context.Background(), topic, func(_ context.Context, _ Event) error {
		// Simulate a moderately slow subscriber.
		time.Sleep(2 * time.Millisecond)
		atomic.AddInt64(&received, 1)
		return nil
	})
	require.NoError(t, err)

	// Publish all events — will block under backpressure but must never drop.
	failed := publishN(t, module, topic, total)
	require.Zero(t, failed, "no publish should fail")

	// Wait for all events to be processed.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&received) >= total {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	assert.Equal(t, int64(total), atomic.LoadInt64(&received),
		"all %d published events must be delivered", total)
}

// TestDurableMemoryBackpressureNotDrop verifies that when the queue is full the
// publisher is blocked (not errored or dropped) until the subscriber drains events.
func TestDurableMemoryBackpressureNotDrop(t *testing.T) {
	const (
		topic      = "durable.backpressure"
		queueDepth = 5
		publish    = 10
	)

	// Use the lower-level engine directly for fine-grained timing control.
	bus := &DurableMemoryEventBus{
		config: &EventBusConfig{
			MaxEventQueueSize:    100,
			MaxDurableQueueDepth: queueDepth,
		},
		subscriptions: make(map[string]map[string]*durableSub),
	}
	ctx := context.Background()
	require.NoError(t, bus.Start(ctx))
	defer bus.Stop(ctx) //nolint:errcheck

	// A gate channel: handler blocks until we release it.
	gate := make(chan struct{})
	var received int64

	_, err := bus.Subscribe(ctx, topic, func(_ context.Context, _ Event) error {
		<-gate
		atomic.AddInt64(&received, 1)
		return nil
	})
	require.NoError(t, err)

	// Publish in background; it will fill the queue then block.
	publishDone := make(chan struct{})
	go func() {
		defer close(publishDone)
		for i := 0; i < publish; i++ {
			evt := cloudevents.NewEvent()
			evt.SetType(topic)
			evt.SetSource("test")
			_ = bus.Publish(ctx, evt)
		}
	}()

	// Give publisher time to fill the queue and block.
	time.Sleep(50 * time.Millisecond)
	// Publisher should still be running (blocked on backpressure).
	select {
	case <-publishDone:
		t.Fatal("publisher finished before subscriber drained — expected backpressure")
	default:
	}

	// Release the gate so the subscriber can drain.
	close(gate)

	// Publisher must now finish.
	select {
	case <-publishDone:
	case <-time.After(5 * time.Second):
		t.Fatal("publisher did not finish after subscriber was released")
	}

	// All events must have been delivered.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&received) >= publish {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	assert.Equal(t, int64(publish), atomic.LoadInt64(&received),
		"all events must be delivered via backpressure")
}

// TestDurableMemoryUnsubscribeDuringPublish verifies that cancelling a subscription
// while publishing is safe and does not deadlock or panic.
func TestDurableMemoryUnsubscribeDuringPublish(t *testing.T) {
	module := newDurableModule(t, 10)

	var received int64
	sub, err := module.Subscribe(context.Background(), "durable.unsub", func(_ context.Context, _ Event) error {
		atomic.AddInt64(&received, 1)
		return nil
	})
	require.NoError(t, err)

	// Publish a burst then unsubscribe mid-flight.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = module.Publish(context.Background(), "durable.unsub", map[string]int{"i": i})
		}
	}()

	time.Sleep(5 * time.Millisecond)
	require.NoError(t, module.Unsubscribe(context.Background(), sub))

	wg.Wait()
	// No assertion on received count — the test validates no deadlock/panic.
}

// TestDurableMemoryGracefulShutdown verifies that Stop waits for in-flight
// handlers to complete before returning.
func TestDurableMemoryGracefulShutdown(t *testing.T) {
	bus := &DurableMemoryEventBus{
		config: &EventBusConfig{
			MaxEventQueueSize:    100,
			MaxDurableQueueDepth: 50,
		},
		subscriptions: make(map[string]map[string]*durableSub),
	}
	ctx := context.Background()
	require.NoError(t, bus.Start(ctx))

	var received int64
	_, err := bus.Subscribe(ctx, "durable.shutdown", func(_ context.Context, _ Event) error {
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt64(&received, 1)
		return nil
	})
	require.NoError(t, err)

	// Publish a few events, then stop with a generous deadline.
	for i := 0; i < 5; i++ {
		evt := cloudevents.NewEvent()
		evt.SetType("durable.shutdown")
		evt.SetSource("test")
		require.NoError(t, bus.Publish(ctx, evt))
	}

	// Stop should return promptly (handler takes 10ms × 5 = ~50ms).
	stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, bus.Stop(stopCtx))
	// At least one event should have been delivered before/during shutdown.
	assert.Positive(t, atomic.LoadInt64(&received))
}

// TestDurableMemoryWildcardTopics verifies wildcard subscriptions work correctly.
func TestDurableMemoryWildcardTopics(t *testing.T) {
	module := newDurableModule(t, 50)
	ctx := context.Background()

	var exact, wildcard int64

	_, err := module.Subscribe(ctx, "durable.wc.alpha", func(_ context.Context, _ Event) error {
		atomic.AddInt64(&exact, 1)
		return nil
	})
	require.NoError(t, err)

	_, err = module.Subscribe(ctx, "durable.wc.*", func(_ context.Context, _ Event) error {
		atomic.AddInt64(&wildcard, 1)
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, module.Publish(ctx, "durable.wc.alpha", nil))
	require.NoError(t, module.Publish(ctx, "durable.wc.beta", nil))

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int64(1), atomic.LoadInt64(&exact), "exact sub should get 1 event")
	assert.Equal(t, int64(2), atomic.LoadInt64(&wildcard), "wildcard sub should get 2 events")
}

// TestDurableMemoryEngineRegistered verifies that "durable-memory" is a valid engine
// name via the normal module init path.
func TestDurableMemoryEngineRegistered(t *testing.T) {
	module := NewModule().(*EventBusModule)
	app := newMockApp()
	app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(&EventBusConfig{
		Engine: "durable-memory",
	}))
	require.NoError(t, module.Init(app))
	require.NoError(t, module.Start(context.Background()))
	require.NoError(t, module.Stop(context.Background()))
}
