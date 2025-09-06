package eventbus

import (
	"context"
	"testing"
	"time"
)

// TestMemoryPublishRotationAndDrops exercises:
// 1. RotateSubscriberOrder branch in memory.Publish (ensures rotation logic executes)
// 2. Async worker pool saturation drop path (queueEventHandler default case increments droppedCount)
// 3. DeliveryMode "timeout" with zero PublishBlockTimeout immediate drop branch
// 4. Module level GetRouter / Stats / PerEngineStats accessors (light touch)
func TestMemoryPublishRotationAndDrops(t *testing.T) {
	cfg := &EventBusConfig{
		Engine:                 "memory",
		WorkerCount:            1,
		DefaultEventBufferSize: 1,
		MaxEventQueueSize:      10,
		RetentionDays:          1,
		RotateSubscriberOrder:  true,
		DeliveryMode:           "timeout", // exercise timeout mode with zero timeout
		PublishBlockTimeout:    0,         // immediate drop for full buffers
	}
	if err := cfg.ValidateConfig(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	router, err := NewEngineRouter(cfg)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Extract memory engine
	var mem *MemoryEventBus
	for _, eng := range router.engines {
		if m, ok := eng.(*MemoryEventBus); ok {
			mem = m
			break
		}
	}
	if mem == nil {
		t.Fatalf("memory engine missing")
	}

	// Create multiple async subscriptions so rotation has >1 subscriber list.
	ctx := context.Background()
	for i := 0; i < 3; i++ { // 3 subs ensures rotation slice logic triggers when >1
		_, err := mem.SubscribeAsync(ctx, "rotate.topic", func(ctx context.Context, e Event) error { time.Sleep(5 * time.Millisecond); return nil })
		if err != nil {
			t.Fatalf("subscribe async %d: %v", i, err)
		}
	}

	// Also create a synchronous subscriber with tiny buffer to force timeout-mode drops when saturated.
	_, err = mem.Subscribe(ctx, "rotate.topic", func(ctx context.Context, e Event) error { time.Sleep(2 * time.Millisecond); return nil })
	if err != nil {
		t.Fatalf("sync subscribe: %v", err)
	}

	// Fire a burst of events; limited worker pool + small buffers -> some drops.
	for i := 0; i < 50; i++ { // ample attempts to cause rotation & drops
		_ = mem.Publish(ctx, Event{Topic: "rotate.topic"})
	}

	// Allow processing/draining
	time.Sleep(100 * time.Millisecond)

	delivered, dropped := mem.Stats()
	if delivered == 0 {
		t.Fatalf("expected some delivered events (rotation path), got 0")
	}
	if dropped == 0 {
		t.Fatalf("expected some dropped events from timeout + saturation, got 0")
	}

	// Touch module-level accessors via a lightweight module wrapper to bump coverage on module.go convenience methods.
	mod := &EventBusModule{router: router}
	if mod.GetRouter() == nil {
		t.Fatalf("expected router from module accessor")
	}
	td, _ := mod.Stats()
	if td == 0 {
		t.Fatalf("expected non-zero delivered via module stats")
	}
	per := mod.PerEngineStats()
	if len(per) == 0 {
		t.Fatalf("expected per-engine stats via module accessor")
	}
}
