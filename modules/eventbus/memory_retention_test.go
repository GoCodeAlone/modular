package eventbus

import (
	"context"
	"testing"
	"time"
)

// TestMemoryCleanupOldEvents exercises startRetentionTimer() and cleanupOldEvents() paths.
func TestMemoryCleanupOldEvents(t *testing.T) {
	cfg := &EventBusConfig{
		MaxEventQueueSize:      100,
		DefaultEventBufferSize: 4,
		WorkerCount:            1,
		RetentionDays:          1,
		DeliveryMode:           "drop",
		RotateSubscriberOrder:  true,
	}
	if err := cfg.ValidateConfig(); err != nil { // ensure defaults applied sensibly
		t.Fatalf("validate config: %v", err)
	}
	bus := NewMemoryEventBus(cfg)
	// Mark as started so the retention timer restart logic would be considered if it fired.
	bus.isStarted = true

	// Invoke startRetentionTimer directly (covers its body). We won't wait 24h for callback.
	bus.startRetentionTimer()
	if bus.retentionTimer == nil {
		t.Fatal("expected retention timer to be created")
	}

	// Seed event history with one old and one fresh event.
	oldEvent := Event{Topic: "orders.created", CreatedAt: time.Now().AddDate(0, 0, -3)}
	freshEvent := Event{Topic: "orders.created", CreatedAt: time.Now()}
	bus.storeEventHistory(oldEvent)
	bus.storeEventHistory(freshEvent)

	// Sanity precondition.
	if got := len(bus.eventHistory["orders.created"]); got != 2 {
		t.Fatalf("expected 2 events pre-cleanup, got %d", got)
	}

	// Run cleanup directly; old event should be dropped.
	bus.cleanupOldEvents()
	events := bus.eventHistory["orders.created"]
	if len(events) != 1 {
		t.Fatalf("expected 1 event post-cleanup, got %d", len(events))
	}
	if !events[0].CreatedAt.After(time.Now().AddDate(0, 0, -2)) { // loose assertion
		t.Fatalf("expected remaining event to be the fresh one: %+v", events[0])
	}
}

// TestMemoryRetentionTimerRestartPath calls startRetentionTimer twice with different isStarted states
// to cover the conditional restart logic indirectly (first while started, then after stop flag cleared).
func TestMemoryRetentionTimerRestartPath(t *testing.T) {
	cfg := &EventBusConfig{MaxEventQueueSize: 10, DefaultEventBufferSize: 1, WorkerCount: 1, RetentionDays: 1}
	bus := NewMemoryEventBus(cfg)
	bus.isStarted = true
	bus.startRetentionTimer()
	if bus.retentionTimer == nil {
		t.Fatalf("expected first timer")
	}
	// Simulate stop before timer callback would re-arm; mark not started and invoke startRetentionTimer again.
	bus.isStarted = false
	bus.startRetentionTimer() // should still create a timer object (restart logic gated inside callback)
	if bus.retentionTimer == nil {
		t.Fatalf("expected second timer creation even when not started")
	}
}

// TestMemoryRetentionIntegration ensures that published events get stored then can be cleaned.
func TestMemoryRetentionIntegration(t *testing.T) {
	cfg := &EventBusConfig{MaxEventQueueSize: 10, DefaultEventBufferSize: 2, WorkerCount: 1, RetentionDays: 1, RotateSubscriberOrder: true}
	if err := cfg.ValidateConfig(); err != nil {
		t.Fatalf("validate config: %v", err)
	}
	bus := NewMemoryEventBus(cfg)
	if err := bus.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Publish a couple of events to build up some history.
	for i := 0; i < 3; i++ {
		if err := bus.Publish(context.Background(), Event{Topic: "retention.topic"}); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	// Inject an old event manually to ensure cleanup path removes it.
	old := Event{Topic: "retention.topic", CreatedAt: time.Now().AddDate(0, 0, -5)}
	bus.storeEventHistory(old)
	if l := len(bus.eventHistory["retention.topic"]); l < 4 { // 3 recent + 1 old
		t.Fatalf("expected >=4 events, have %d", l)
	}
	bus.cleanupOldEvents()
	for _, e := range bus.eventHistory["retention.topic"] {
		if e.CreatedAt.Before(time.Now().AddDate(0, 0, -cfg.RetentionDays)) {
			t.Fatalf("found non-cleaned old event: %+v", e)
		}
	}
}
