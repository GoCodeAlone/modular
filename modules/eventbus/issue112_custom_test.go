package eventbus

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCustomBusWithDrop starts a CustomMemoryEventBus, holds its single handler
// open, fills the (size-1) subscriber buffer, then publishes more so the engine
// must drop on a full channel. Returns the engine and a cleanup func that
// releases the handler and stops the bus.
func newCustomBusWithDrop(t *testing.T) (*CustomMemoryEventBus, func()) {
	t.Helper()
	ebus, err := NewCustomMemoryEventBus(map[string]interface{}{
		"defaultEventBufferSize": 1,
		"maxEventQueueSize":      100,
		"enableMetrics":          false,
	})
	require.NoError(t, err)
	cbus := ebus.(*CustomMemoryEventBus)
	require.NoError(t, cbus.Start(context.Background()))

	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	_, err = cbus.Subscribe(context.Background(), "full.topic", func(_ context.Context, _ Event) error {
		once.Do(func() { close(started) })
		<-release
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, cbus.Publish(context.Background(), evt112("full.topic")))
	<-started
	// buffer (cap 1) takes one; the rest hit the full-channel drop path.
	for range 10 {
		_ = cbus.Publish(context.Background(), evt112("full.topic"))
	}

	cleanup := func() {
		close(release)
		_ = cbus.Stop(context.Background())
	}
	return cbus, cleanup
}

// TestIssue112_CustomBus_StatsCountsDroppedOnFullChannel gates issue #112 part 2:
// CustomMemoryEventBus must expose Stats() and count silent full-channel drops.
func TestIssue112_CustomBus_StatsCountsDroppedOnFullChannel(t *testing.T) {
	cbus, cleanup := newCustomBusWithDrop(t)
	defer cleanup()

	_, dropped := cbus.Stats()
	assert.Greater(t, dropped, uint64(0),
		"custom engine must count publish-time full-channel drops in Stats()")
}

// TestIssue112_CustomBus_UnsubscribeCountsBufferedAsDropped gates issue #112
// part 3 on the custom engine: buffered events abandoned at Unsubscribe must be
// counted as dropped, and Stats() conservation must hold.
func TestIssue112_CustomBus_UnsubscribeCountsBufferedAsDropped(t *testing.T) {
	ebus, err := NewCustomMemoryEventBus(map[string]interface{}{
		"defaultEventBufferSize": 16,
		"maxEventQueueSize":      100,
		"enableMetrics":          false,
	})
	require.NoError(t, err)
	cbus := ebus.(*CustomMemoryEventBus)
	ctx := context.Background()
	require.NoError(t, cbus.Start(ctx))
	defer cbus.Stop(context.Background()) //nolint:errcheck

	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	sub, err := cbus.Subscribe(ctx, "drain.topic", func(_ context.Context, _ Event) error {
		once.Do(func() { close(started) })
		<-release
		return nil
	})
	require.NoError(t, err)
	cs := sub.(*customMemorySubscription)

	require.NoError(t, cbus.Publish(ctx, evt112("drain.topic")))
	<-started

	const buffered = 5
	for range buffered {
		require.NoError(t, cbus.Publish(ctx, evt112("drain.topic")))
	}

	require.NoError(t, cbus.Unsubscribe(ctx, sub))
	close(release)

	select {
	case <-cs.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("custom handler goroutine did not exit after release")
	}

	delivered, dropped := cbus.Stats()
	const total = uint64(1 + buffered)
	assert.Equal(t, uint64(1), delivered, "e1 should be delivered")
	assert.Equal(t, uint64(buffered), dropped, "buffered events must be counted as dropped at Unsubscribe")
	assert.Equal(t, total, delivered+dropped, "conservation: delivered+dropped == enqueued")
}

// TestIssue112_Router_CollectStatsIncludesCustomEngine gates the module-wiring
// half of issue #112 part 2 (design D3): the router must aggregate Stats() from
// ANY engine implementing the statsProvider interface, not only *MemoryEventBus.
func TestIssue112_Router_CollectStatsIncludesCustomEngine(t *testing.T) {
	cbus, cleanup := newCustomBusWithDrop(t)
	defer cleanup()

	router := &EngineRouter{engines: map[string]EventBus{"custom": cbus}}

	_, dropped := router.CollectStats()
	assert.Greater(t, dropped, uint64(0),
		"CollectStats must include the custom engine's drops via the statsProvider interface")

	per := router.CollectPerEngineStats()
	require.Contains(t, per, "custom", "per-engine stats must include the custom engine")
	assert.Greater(t, per["custom"].Dropped, uint64(0))
}
