package eventbus

import (
	"context"
	"testing"
	"time"
)

// TestMemorySubscriptionLifecycle covers double cancel and second unsubscribe no-op behavior for memory engine.
func TestMemorySubscriptionLifecycle(t *testing.T) {
	cfg := &EventBusConfig{Engine: "memory", WorkerCount: 1, DefaultEventBufferSize: 2, MaxEventQueueSize: 10, RetentionDays: 1}
	if err := cfg.ValidateConfig(); err != nil { t.Fatalf("validate: %v", err) }
	router, err := NewEngineRouter(cfg)
	if err != nil { t.Fatalf("router: %v", err) }
	if err := router.Start(context.Background()); err != nil { t.Fatalf("start: %v", err) }

	// Locate memory engine
	var mem *MemoryEventBus
	for _, eng := range router.engines { if m, ok := eng.(*MemoryEventBus); ok { mem = m; break } }
	if mem == nil { t.Fatalf("memory engine missing") }

	delivered, dropped := mem.Stats()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sub, err := mem.Subscribe(ctx, "lifecycle.topic", func(ctx context.Context, e Event) error { return nil })
	if err != nil { t.Fatalf("subscribe: %v", err) }

	// First unsubscribe
	if err := mem.Unsubscribe(ctx, sub); err != nil { t.Fatalf("unsubscribe first: %v", err) }
	// Second unsubscribe on memory engine is a silent no-op (returns nil). Ensure it doesn't error.
	if err := mem.Unsubscribe(ctx, sub); err != nil { t.Fatalf("second unsubscribe should be no-op, got error: %v", err) }

	// Direct double cancel path also returns nil.
	if err := sub.Cancel(); err != nil { t.Fatalf("second direct cancel: %v", err) }

	// Publish events to confirm no delivery after unsubscribe.
	if err := mem.Publish(ctx, Event{Topic: "lifecycle.topic"}); err != nil { t.Fatalf("publish: %v", err) }
	newDelivered, newDropped := mem.Stats()
	if newDelivered != delivered || newDropped != dropped { t.Fatalf("expected stats unchanged after publishing to removed subscription: got %d/%d -> %d/%d", delivered, dropped, newDelivered, newDropped) }
}

// TestEngineRouterDoubleUnsubscribeIdempotent verifies router-level double unsubscribe is idempotent
// (returns nil just like the underlying memory engine). The ErrSubscriptionNotFound branch is
// covered separately using a dummy subscription of an unknown concrete type in
// engine_router_additional_test.go.
func TestEngineRouterDoubleUnsubscribeIdempotent(t *testing.T) {
	cfg := &EventBusConfig{Engine: "memory", WorkerCount: 1, DefaultEventBufferSize: 1, MaxEventQueueSize: 5, RetentionDays: 1}
	if err := cfg.ValidateConfig(); err != nil { t.Fatalf("validate: %v", err) }
	router, err := NewEngineRouter(cfg)
	if err != nil { t.Fatalf("router: %v", err) }
	if err := router.Start(context.Background()); err != nil { t.Fatalf("start: %v", err) }

	sub, err := router.Subscribe(context.Background(), "router.lifecycle", func(ctx context.Context, e Event) error { return nil })
	if err != nil { t.Fatalf("subscribe: %v", err) }
	if err := router.Unsubscribe(context.Background(), sub); err != nil { t.Fatalf("first unsubscribe: %v", err) }
	// Second unsubscribe should traverse all engines, none handle it, yielding ErrSubscriptionNotFound.
	if err := router.Unsubscribe(context.Background(), sub); err != nil {
		 t.Fatalf("second unsubscribe should be idempotent (nil), got %v", err)
	}
}
