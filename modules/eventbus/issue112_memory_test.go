package eventbus

import (
	"context"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// evt112 builds a minimal CloudEvent for the given topic.
func evt112(topic string) Event {
	e := cloudevents.NewEvent()
	e.SetID(uuid.NewString())
	e.SetType(topic)
	e.SetSource("issue112-test")
	return e
}

// TestIssue112_TimeoutModePublishDoesNotHang is the regression gate for issue
// #112 part 1: in deliveryMode "timeout", once the timer fires the legacy
// `if !deadline.Stop() { <-deadline.C }` drain blocks the publisher forever.
//
// CI-safe: the publish loop runs in a goroutine and the test asserts it returns
// within a bound. On buggy code the goroutine never returns and the bound trips
// (test fails) instead of deadlocking the whole CI run.
func TestIssue112_TimeoutModePublishDoesNotHang(t *testing.T) {
	bus := NewMemoryEventBus(&EventBusConfig{
		MaxEventQueueSize:      100,
		DefaultEventBufferSize: 1,
		WorkerCount:            1,
		DeliveryMode:           "timeout",
		PublishBlockTimeout:    50 * time.Millisecond,
		RetentionDays:          1,
	})
	ctx := context.Background()
	require.NoError(t, bus.Start(ctx))

	release := make(chan struct{})
	var once sync.Once
	relFn := func() { once.Do(func() { close(release) }) }
	// LIFO: release the handler before Stop waits on the worker.
	defer bus.Stop(context.Background()) //nolint:errcheck
	defer relFn()

	_, err := bus.Subscribe(ctx, "hang.topic", func(_ context.Context, _ Event) error {
		<-release // hold the single sync handler so the buffer fills
		return nil
	})
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// First publish is consumed by the handler (which then blocks); the
		// next fills the buffer; subsequent ones hit the full-buffer timeout
		// path that contains the deadlock.
		for range 5 {
			_ = bus.Publish(ctx, evt112("hang.topic"))
		}
	}()

	select {
	case <-done:
		// All publishes returned — timer drain is race-free.
	case <-time.After(3 * time.Second):
		t.Fatal("Publish hung in timeout delivery mode (issue #112 P1): timer-drain deadlock")
	}
}

// TestIssue112_MemoryBus_UnsubscribeCountsBufferedAsDropped is the regression
// gate for issue #112 part 3 on the memory engine: events still buffered in a
// subscriber channel at Unsubscribe must be counted as dropped (at-most-once
// teardown contract), so Stats() conservation holds.
func TestIssue112_MemoryBus_UnsubscribeCountsBufferedAsDropped(t *testing.T) {
	bus := NewMemoryEventBus(&EventBusConfig{
		MaxEventQueueSize:      100,
		DefaultEventBufferSize: 16,
		WorkerCount:            1,
		DeliveryMode:           "drop",
		RetentionDays:          1,
	})
	ctx := context.Background()
	require.NoError(t, bus.Start(ctx))
	defer bus.Stop(context.Background()) //nolint:errcheck

	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	sub, err := bus.Subscribe(ctx, "drain.topic", func(_ context.Context, _ Event) error {
		once.Do(func() { close(started) })
		<-release
		return nil
	})
	require.NoError(t, err)
	ms := sub.(*memorySubscription)

	// e1 is dequeued; the handler signals started then blocks.
	require.NoError(t, bus.Publish(ctx, evt112("drain.topic")))
	<-started

	// e2..e(1+buffered) sit in the buffer (buffer=16, none dropped at publish).
	const buffered = 5
	for range buffered {
		require.NoError(t, bus.Publish(ctx, evt112("drain.topic")))
	}

	// Unsubscribe sets cancelled + closes done, then waits (<=100ms) on finished.
	// The handler is still blocked, so the wait times out and Unsubscribe returns.
	require.NoError(t, bus.Unsubscribe(ctx, sub))

	// Release the handler: it returns (e1 delivered), the loop's top-of-iteration
	// cancelled fast-path fires, and the deferred drain counts the buffered
	// events as dropped.
	close(release)

	select {
	case <-ms.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("handler goroutine did not exit after release")
	}

	delivered, dropped := bus.Stats()
	const total = uint64(1 + buffered)
	assert.Equal(t, uint64(1), delivered, "e1 should be delivered")
	assert.Equal(t, uint64(buffered), dropped, "buffered events must be counted as dropped at Unsubscribe")
	assert.Equal(t, total, delivered+dropped, "conservation: delivered+dropped == enqueued")
}

// TestIssue112_MemoryBus_StopCountsWorkerPoolAsDropped is the regression gate
// for the async leak (adversarial-review C1): events dequeued into the worker
// pool but not yet executed when Stop cancels must be counted as dropped, so
// conservation holds for async subscriptions too.
func TestIssue112_MemoryBus_StopCountsWorkerPoolAsDropped(t *testing.T) {
	bus := NewMemoryEventBus(&EventBusConfig{
		MaxEventQueueSize:      100,
		DefaultEventBufferSize: 64,
		WorkerCount:            1, // single slow worker so the pool backs up
		DeliveryMode:           "drop",
		RetentionDays:          1,
	})
	ctx := context.Background()
	require.NoError(t, bus.Start(ctx))

	const n = 30
	_, err := bus.SubscribeAsync(ctx, "wp.topic", func(_ context.Context, _ Event) error {
		time.Sleep(15 * time.Millisecond)
		return nil
	})
	require.NoError(t, err)

	for range n {
		require.NoError(t, bus.Publish(ctx, evt112("wp.topic")))
	}
	// Let handleEvents move events from the channel into the worker pool and let
	// the single worker start chewing through them.
	time.Sleep(20 * time.Millisecond)

	require.NoError(t, bus.Stop(context.Background()))

	delivered, dropped := bus.Stats()
	assert.Equal(t, uint64(n), delivered+dropped,
		"conservation: every async-queued event must be delivered or dropped (delivered=%d dropped=%d)", delivered, dropped)
	assert.Greater(t, dropped, uint64(0),
		"a slow worker + immediate Stop must leave some queued events counted as dropped")
}
