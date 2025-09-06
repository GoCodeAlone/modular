package eventbus

import (
	"context"
	"testing"
)

// foreignSub implements Subscription but is not the concrete type expected by CustomMemoryEventBus.
type foreignSub struct{}

func (f foreignSub) Topic() string { return "valid.topic" }
func (f foreignSub) ID() string    { return "foreign" }
func (f foreignSub) IsAsync() bool { return false }
func (f foreignSub) Cancel() error { return nil }

// TestCustomMemoryInvalidUnsubscribe exercises the ErrInvalidSubscriptionType branch.
func TestCustomMemoryInvalidUnsubscribe(t *testing.T) {
	busRaw, err := NewCustomMemoryEventBus(map[string]interface{}{"enableMetrics": false})
	if err != nil {
		t.Fatalf("create bus: %v", err)
	}
	bus := busRaw.(*CustomMemoryEventBus)
	if err := bus.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Create a valid subscription to ensure bus started logic executed (not strictly required for invalid path).
	sub, err := bus.Subscribe(context.Background(), "valid.topic", func(ctx context.Context, e Event) error { return nil })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if sub == nil {
		t.Fatalf("expected non-nil subscription")
	}

	if err := bus.Unsubscribe(context.Background(), foreignSub{}); err == nil || err != ErrInvalidSubscriptionType {
		t.Fatalf("expected ErrInvalidSubscriptionType, got %v", err)
	}
}
