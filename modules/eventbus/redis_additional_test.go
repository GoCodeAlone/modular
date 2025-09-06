package eventbus

import (
	"context"
	"testing"
	"time"
)

// TestNewRedisEventBusInvalidURL covers invalid Redis URL parsing error path.
func TestNewRedisEventBusInvalidURL(t *testing.T) {
	_, err := NewRedisEventBus(map[string]interface{}{"url": ":://bad_url"})
	if err == nil {
		t.Fatalf("expected error for invalid redis url")
	}
}

// TestRedisEventBusStartNotStartedGuard ensures Publish before Start returns ErrEventBusNotStarted.
func TestRedisEventBusPublishBeforeStart(t *testing.T) {
	busAny, err := NewRedisEventBus(map[string]interface{}{"url": "redis://localhost:6379"})
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}
	bus := busAny.(*RedisEventBus)
	if err := bus.Publish(context.Background(), Event{Topic: "t"}); err == nil {
		t.Fatalf("expected ErrEventBusNotStarted")
	}
}

// TestRedisEventBusStartAndStop handles start failure due to connection refusal quickly (short timeout).
func TestRedisEventBusStartFailure(t *testing.T) {
	// Use an un-routable address to force ping failure quickly.
	busAny, err := NewRedisEventBus(map[string]interface{}{"url": "redis://localhost:6390"})
	if err != nil {
		t.Fatalf("constructor should succeed: %v", err)
	}
	bus := busAny.(*RedisEventBus)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := bus.Start(ctx); err == nil {
		t.Fatalf("expected start error due to unreachable redis")
	}
	// Stop should be safe even if not started
	if err := bus.Stop(context.Background()); err != nil {
		t.Fatalf("unexpected stop error: %v", err)
	}
}

// TestRedisSubscribeBeforeStart ensures subscribing before Start errors.
func TestRedisSubscribeBeforeStart(t *testing.T) {
	busAny, err := NewRedisEventBus(map[string]interface{}{"url": "redis://localhost:6379"})
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}
	bus := busAny.(*RedisEventBus)
	if _, err := bus.Subscribe(context.Background(), "topic", func(ctx context.Context, e Event) error { return nil }); err == nil {
		t.Fatalf("expected error when subscribing before start")
	}
	if _, err := bus.SubscribeAsync(context.Background(), "topic", func(ctx context.Context, e Event) error { return nil }); err == nil {
		t.Fatalf("expected error when subscribing async before start")
	}
}

// TestRedisUnsubscribeBeforeStart ensures Unsubscribe before Start errors.
func TestRedisUnsubscribeBeforeStart(t *testing.T) {
	busAny, err := NewRedisEventBus(map[string]interface{}{"url": "redis://localhost:6379"})
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}
	bus := busAny.(*RedisEventBus)
	dummy := &redisSubscription{} // minimal stub
	if err := bus.Unsubscribe(context.Background(), dummy); err == nil {
		t.Fatalf("expected error when unsubscribing before start")
	}
}

// TestRedisSubscriptionCancelIdempotent covers Cancel early return when already cancelled.
func TestRedisSubscriptionCancelIdempotent(t *testing.T) {
	sub := &redisSubscription{cancelled: true, done: make(chan struct{})}
	// Should simply return nil without panic or closing done twice.
	if err := sub.Cancel(); err != nil {
		t.Fatalf("expected nil error for already cancelled subscription, got %v", err)
	}
}
