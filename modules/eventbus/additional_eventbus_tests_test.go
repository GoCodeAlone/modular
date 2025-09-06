package eventbus

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// Test basic publish/subscribe lifecycle using memory engine ensuring message receipt and stats increments.
func TestEventBusPublishSubscribeBasic(t *testing.T) {
	m := NewModule().(*EventBusModule)
	app := newMockApp()
	// Register default config section as RegisterConfig would
	if err := m.RegisterConfig(app); err != nil {
		t.Fatalf("register config: %v", err)
	}
	if err := m.Init(app); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer m.Stop(context.Background())

	received := make(chan struct{}, 1)
	_, err := m.Subscribe(context.Background(), "test.topic", func(ctx context.Context, e Event) error {
		if e.Topic != "test.topic" {
			t.Errorf("unexpected topic %s", e.Topic)
		}
		received <- struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := m.Publish(context.Background(), "test.topic", "payload"); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event delivery")
	}

	del, _ := m.Stats()
	if del == 0 {
		t.Fatalf("expected delivered stats > 0")
	}
}

// Test unsubscribe removes subscription and no further deliveries occur.
func TestEventBusUnsubscribe(t *testing.T) {
	m := NewModule().(*EventBusModule)
	app := newMockApp()
	if err := m.RegisterConfig(app); err != nil {
		t.Fatalf("register config: %v", err)
	}
	if err := m.Init(app); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer m.Stop(context.Background())

	count := 0
	sub, err := m.Subscribe(context.Background(), "once.topic", func(ctx context.Context, e Event) error { count++; return nil })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := m.Publish(context.Background(), "once.topic", 1); err != nil {
		t.Fatalf("publish1: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if count != 1 {
		t.Fatalf("expected 1 delivery got %d", count)
	}

	if err := m.Unsubscribe(context.Background(), sub); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	if err := m.Publish(context.Background(), "once.topic", 2); err != nil {
		t.Fatalf("publish2: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if count != 1 {
		t.Fatalf("expected no additional deliveries after unsubscribe")
	}
}

// Test async subscription processes events without blocking publisher.
func TestEventBusAsyncSubscription(t *testing.T) {
	m := NewModule().(*EventBusModule)
	app := newMockApp()
	if err := m.RegisterConfig(app); err != nil {
		t.Fatalf("register config: %v", err)
	}
	if err := m.Init(app); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer m.Stop(context.Background())

	received := make(chan struct{}, 1)
	_, err := m.SubscribeAsync(context.Background(), "async.topic", func(ctx context.Context, e Event) error { received <- struct{}{}; return nil })
	if err != nil {
		t.Fatalf("subscribe async: %v", err)
	}

	start := time.Now()
	if err := m.Publish(context.Background(), "async.topic", 123); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// We expect Publish to return quickly (well under 100ms) even if handler not yet executed.
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("publish blocked unexpectedly long")
	}

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for async delivery")
	}
}

// Removed local mockApp (reuse the one defined in module_test.go)

// TestMemoryEventBus_RotationFairness ensures subscriber ordering rotates when enabled.
func TestMemoryEventBus_RotationFairness(t *testing.T) {
	ctx := context.Background()
	cfg := &EventBusConfig{WorkerCount: 1, DefaultEventBufferSize: 1, RotateSubscriberOrder: true, DeliveryMode: "drop"}
	bus := NewMemoryEventBus(cfg)
	if err := bus.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer bus.Stop(ctx)

	orderCh := make(chan string, 16)
	mkHandler := func(id string) EventHandler {
		return func(ctx context.Context, evt Event) error { orderCh <- id; return nil }
	}
	for i := 0; i < 3; i++ {
		_, err := bus.Subscribe(ctx, "rot.topic", mkHandler(string(rune('A'+i))))
		if err != nil {
			t.Fatalf("subscribe %d: %v", i, err)
		}
	}

	firsts := make(map[string]int)
	for i := 0; i < 9; i++ {
		_ = bus.Publish(ctx, Event{Topic: "rot.topic"})
		select {
		case id := <-orderCh:
			firsts[id]++
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timeout waiting for first handler")
		}
		// Drain remaining handlers for this publish (best-effort)
		for j := 0; j < 2; j++ {
			select {
			case <-orderCh:
			default:
			}
		}
	}
	if len(firsts) < 2 {
		t.Fatalf("expected rotation to vary first subscriber, got %v", firsts)
	}
}

// TestMemoryEventBus_PublishTimeoutImmediateDrop covers timeout mode with zero timeout resulting in immediate drop when subscriber buffer full.
func TestMemoryEventBus_PublishTimeoutImmediateDrop(t *testing.T) {
	ctx := context.Background()
	cfg := &EventBusConfig{WorkerCount: 1, DefaultEventBufferSize: 1, DeliveryMode: "timeout", PublishBlockTimeout: 0}
	bus := NewMemoryEventBus(cfg)
	if err := bus.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer bus.Stop(ctx)

	// Manually construct a subscription with a full channel (no handler goroutine)
	sub := &memorySubscription{
		id:       "manual",
		topic:    "t",
		handler:  func(ctx context.Context, e Event) error { return nil },
		isAsync:  false,
		eventCh:  make(chan Event, 1),
		done:     make(chan struct{}),
		finished: make(chan struct{}),
	}
	// Fill the channel to force publish path into drop branch
	sub.eventCh <- Event{Topic: "t"}
	bus.topicMutex.Lock()
	bus.subscriptions["t"] = map[string]*memorySubscription{sub.id: sub}
	bus.topicMutex.Unlock()

	before := atomic.LoadUint64(&bus.droppedCount)
	_ = bus.Publish(ctx, Event{Topic: "t"})
	after := atomic.LoadUint64(&bus.droppedCount)
	if after != before+1 {
		t.Fatalf("expected exactly one drop, before=%d after=%d", before, after)
	}
}

// TestMemoryEventBus_AsyncWorkerSaturation ensures async drops when worker count is zero (no workers to consume tasks).
func TestMemoryEventBus_AsyncWorkerSaturation(t *testing.T) {
	ctx := context.Background()
	cfg := &EventBusConfig{WorkerCount: 0, DefaultEventBufferSize: 1}
	bus := NewMemoryEventBus(cfg)
	if err := bus.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer bus.Stop(ctx)

	_, err := bus.SubscribeAsync(ctx, "a", func(ctx context.Context, e Event) error { return nil })
	if err != nil {
		t.Fatalf("subscribe async: %v", err)
	}
	before := atomic.LoadUint64(&bus.droppedCount)
	for i := 0; i < 5; i++ {
		_ = bus.Publish(ctx, Event{Topic: "a"})
	}
	after := atomic.LoadUint64(&bus.droppedCount)
	if after <= before {
		t.Fatalf("expected drops due to saturated worker pool, before=%d after=%d", before, after)
	}
}

// TestMemoryEventBus_RetentionCleanup verifies old events pruned.
func TestMemoryEventBus_RetentionCleanup(t *testing.T) {
	ctx := context.Background()
	cfg := &EventBusConfig{WorkerCount: 1, DefaultEventBufferSize: 1, RetentionDays: 1}
	bus := NewMemoryEventBus(cfg)
	if err := bus.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer bus.Stop(ctx)

	old := Event{Topic: "old", CreatedAt: time.Now().AddDate(0, 0, -2)}
	recent := Event{Topic: "recent", CreatedAt: time.Now()}
	bus.storeEventHistory(old)
	bus.storeEventHistory(recent)
	bus.cleanupOldEvents()
	bus.historyMutex.RLock()
	defer bus.historyMutex.RUnlock()
	for _, evs := range bus.eventHistory {
		for _, e := range evs {
			if e.Topic == "old" {
				t.Fatalf("old event not cleaned up")
			}
		}
	}
}
