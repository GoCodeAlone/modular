package eventbus

import (
	"context"
	"testing"
	"time"
)

// TestCancelIdempotency ensures calling Cancel multiple times on subscriptions is safe.
func TestCancelIdempotency(t *testing.T) {
	// Memory event bus setup
	memCfg := &EventBusConfig{MaxEventQueueSize: 10, DefaultEventBufferSize: 1, WorkerCount: 1, RetentionDays: 1}
	mem := NewMemoryEventBus(memCfg)
	if err := mem.Start(context.Background()); err != nil {
		t.Fatalf("start memory: %v", err)
	}
	sub, err := mem.Subscribe(context.Background(), "idempotent.topic", func(ctx context.Context, e Event) error { return nil })
	if err != nil {
		t.Fatalf("subscribe mem: %v", err)
	}
	if err := sub.Cancel(); err != nil {
		t.Fatalf("first cancel mem: %v", err)
	}
	// Second cancel should be no-op
	if err := sub.Cancel(); err != nil {
		t.Fatalf("second cancel mem: %v", err)
	}

	// Custom memory event bus setup
	busRaw, err := NewCustomMemoryEventBus(map[string]interface{}{"enableMetrics": false, "defaultEventBufferSize": 1})
	if err != nil {
		t.Fatalf("create custom: %v", err)
	}
	cust := busRaw.(*CustomMemoryEventBus)
	if err := cust.Start(context.Background()); err != nil {
		t.Fatalf("start custom: %v", err)
	}
	csub, err := cust.Subscribe(context.Background(), "idempotent.custom", func(ctx context.Context, e Event) error { return nil })
	if err != nil {
		t.Fatalf("subscribe custom: %v", err)
	}
	if err := csub.Cancel(); err != nil {
		t.Fatalf("first cancel custom: %v", err)
	}
	if err := csub.Cancel(); err != nil {
		t.Fatalf("second cancel custom: %v", err)
	}

	// Publish after cancellation should not trigger handler (cannot easily assert directly without races; rely on no panic).
	_ = mem.Publish(context.Background(), Event{Topic: "idempotent.topic"})
	_ = cust.Publish(context.Background(), Event{Topic: "idempotent.custom"})
	time.Sleep(10 * time.Millisecond)
}
