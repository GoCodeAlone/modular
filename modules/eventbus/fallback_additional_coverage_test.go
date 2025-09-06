package eventbus

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// failingEngine is a minimal engine that always errors to exercise router error wrapping paths.
type failingEngine struct{}

func (f *failingEngine) Start(ctx context.Context) error { return nil }
func (f *failingEngine) Stop(ctx context.Context) error  { return nil }
func (f *failingEngine) Publish(ctx context.Context, e Event) error {
	return errors.New("fail publish")
}
func (f *failingEngine) Subscribe(ctx context.Context, topic string, h EventHandler) (Subscription, error) {
	return nil, errors.New("fail subscribe")
}
func (f *failingEngine) SubscribeAsync(ctx context.Context, topic string, h EventHandler) (Subscription, error) {
	return nil, errors.New("fail subscribe async")
}
func (f *failingEngine) Unsubscribe(ctx context.Context, s Subscription) error {
	return errors.New("fail unsubscribe")
}
func (f *failingEngine) Topics() []string                 { return nil }
func (f *failingEngine) SubscriberCount(topic string) int { return 0 }

// TestEngineRouterFailingEngineErrors ensures router surfaces engine errors.
func TestEngineRouterFailingEngineErrors(t *testing.T) {
	// Temporarily register a custom type name to avoid polluting global registry unpredictably.
	RegisterEngine("failing_tmp", func(cfg map[string]interface{}) (EventBus, error) { return &failingEngine{}, nil })
	cfg := &EventBusConfig{Engine: "failing_tmp", MaxEventQueueSize: 1, DefaultEventBufferSize: 1, WorkerCount: 1, RetentionDays: 1}
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
	if _, err := router.Subscribe(context.Background(), "x", func(ctx context.Context, e Event) error { return nil }); err == nil {
		t.Fatalf("expected subscribe error")
	}
	if err := router.Publish(context.Background(), Event{Topic: "x"}); err == nil {
		t.Fatalf("expected publish error")
	}
}

// TestMemoryBlockModeContextCancel hits Publish block mode path where context cancellation causes drop.
func TestMemoryBlockModeContextCancel(t *testing.T) {
	cfg := &EventBusConfig{MaxEventQueueSize: 10, DefaultEventBufferSize: 1, WorkerCount: 1, RetentionDays: 1, DeliveryMode: "block"}
	bus := NewMemoryEventBus(cfg)
	if err := bus.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Slow handler to ensure queue stays busy.
	sub, err := bus.Subscribe(context.Background(), "slow.topic", func(ctx context.Context, e Event) error { time.Sleep(50 * time.Millisecond); return nil })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	// Fill buffer with one event.
	if err := bus.Publish(context.Background(), Event{Topic: "slow.topic"}); err != nil {
		t.Fatalf("prime publish: %v", err)
	}
	// Context with deadline that will expire quickly forcing the block select to cancel.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_ = bus.Publish(ctx, Event{Topic: "slow.topic"}) // expected to drop due to context
	// Ensure cancellation of subscription to avoid leakage.
	_ = bus.Unsubscribe(context.Background(), sub)
}

// TestMemoryRotateSubscriberOrder ensures rotated path executes when flag enabled and >1 subs.
func TestMemoryRotateSubscriberOrder(t *testing.T) {
	cfg := &EventBusConfig{MaxEventQueueSize: 10, DefaultEventBufferSize: 1, WorkerCount: 1, RetentionDays: 1, RotateSubscriberOrder: true}
	bus := NewMemoryEventBus(cfg)
	if err := bus.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	var recv1 int64
	var recv2 int64
	_, _ = bus.Subscribe(context.Background(), "rot.topic", func(ctx context.Context, e Event) error { atomic.AddInt64(&recv1, 1); return nil })
	_, _ = bus.Subscribe(context.Background(), "rot.topic", func(ctx context.Context, e Event) error { atomic.AddInt64(&recv2, 1); return nil })
	for i := 0; i < 5; i++ {
		_ = bus.Publish(context.Background(), Event{Topic: "rot.topic"})
	}
	time.Sleep(40 * time.Millisecond)
	if atomic.LoadInt64(&recv1)+atomic.LoadInt64(&recv2) == 0 {
		t.Fatalf("expected deliveries with rotation enabled")
	}
}
