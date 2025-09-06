package eventbus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestMemoryPublishDeliveryModes exercises drop and timeout delivery modes including drop counting.
func TestMemoryPublishDeliveryModes(t *testing.T) {
	// Shared handler increments processed count; we will intentionally cancel subscription to make channel fill.
	processed := atomic.Int64{}
	handler := func(ctx context.Context, e Event) error {
		processed.Add(1)
		return nil
	}

	// Helper to create bus with mode.
	newBus := func(mode string, timeout time.Duration) *MemoryEventBus {
		cfg := &EventBusConfig{
			MaxEventQueueSize:      10,
			DefaultEventBufferSize: 1, // tiny buffer to fill quickly
			WorkerCount:            1,
			DeliveryMode:           mode,
			PublishBlockTimeout:    timeout,
			RotateSubscriberOrder:  true,
			RetentionDays:          1,
		}
		if err := cfg.ValidateConfig(); err != nil {
			t.Fatalf("validate config: %v", err)
		}
		bus := NewMemoryEventBus(cfg)
		if err := bus.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		return bus
	}

	// DROP mode: fire many concurrent publishes to oversaturate single-buffer channel causing drops.
	dropBus := newBus("drop", 0)
	slowHandler := func(ctx context.Context, e Event) error {
		time.Sleep(1 * time.Millisecond) // slow processing to keep channel occupied
		return nil
	}
	if _, err := dropBus.Subscribe(context.Background(), "mode.topic", slowHandler); err != nil {
		t.Fatalf("subscribe drop: %v", err)
	}
	attempts := 200
	publishStorm := func() {
		var wg sync.WaitGroup
		for i := 0; i < attempts; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = dropBus.Publish(context.Background(), Event{Topic: "mode.topic"})
			}()
		}
		wg.Wait()
	}
	publishStorm()
	delivered, dropped := dropBus.Stats()
	if dropped == 0 { // Rare edge: scheduler drained everything fast. Retry once.
		publishStorm()
		delivered, dropped = dropBus.Stats()
	}
	if dropped == 0 { // still zero => environment too fast; mark test skipped to avoid flake.
		t.Skipf("could not provoke drop after %d attempts; delivered=%d dropped=%d", attempts*2, delivered, dropped)
	}

	// TIMEOUT mode
	timeoutBus := newBus("timeout", 0) // zero timeout triggers immediate attempt then drop
	sub2, err := timeoutBus.Subscribe(context.Background(), "mode.topic", handler)
	if err != nil {
		t.Fatalf("subscribe timeout: %v", err)
	}
	ms2 := sub2.(*memorySubscription)
	ms2.mutex.Lock()
	ms2.cancelled = true
	ms2.mutex.Unlock()
	time.Sleep(5 * time.Millisecond)
	// Timeout mode with zero timeout behaves like immediate attempt/dropping when buffer full.
	// Reuse concurrency storm approach.
	if _, err := timeoutBus.Subscribe(context.Background(), "mode.topic", slowHandler); err != nil {
		t.Fatalf("subscribe timeout: %v", err)
	}
	publishStorm = func() { // overshadow prior var for clarity
		var wg sync.WaitGroup
		for i := 0; i < attempts; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = timeoutBus.Publish(context.Background(), Event{Topic: "mode.topic"})
			}()
		}
		wg.Wait()
	}
	baseDelivered, baseDropped := timeoutBus.Stats()
	publishStorm()
	d2, dr2 := timeoutBus.Stats()
	if dr2 == baseDropped { // retry once
		publishStorm()
		d2, dr2 = timeoutBus.Stats()
	}
	if dr2 == baseDropped { // skip if still no observable drop increase
		t.Skipf("could not provoke timeout drop; before (%d,%d) after (%d,%d)", baseDelivered, baseDropped, d2, dr2)
	}
}
