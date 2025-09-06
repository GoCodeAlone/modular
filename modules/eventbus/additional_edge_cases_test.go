package eventbus

import (
	"context"
	"errors"
	"testing"
)

// bogusSub implements Subscription but is not a *memorySubscription to trigger type error.
type bogusSub struct{}

func (b bogusSub) Topic() string { return "t" }
func (b bogusSub) ID() string    { return "id" }
func (b bogusSub) IsAsync() bool { return false }
func (b bogusSub) Cancel() error { return nil }

// TestMemoryEventBusEdgeCases covers small edge branches not yet exercised to
// push overall coverage safely above threshold.
func TestMemoryEventBusEdgeCases(t *testing.T) {
	cfg := &EventBusConfig{Engine: "memory", MaxEventQueueSize: 5, DefaultEventBufferSize: 1, WorkerCount: 1, RetentionDays: 1}
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

	// 1. Publish to topic with no subscribers (early return path)
	if err := router.Publish(context.Background(), Event{Topic: "no.subscribers"}); err != nil {
		t.Fatalf("publish no subs: %v", err)
	}

	// Find memory engine instance (only engine configured here)
	var mem *MemoryEventBus
	for _, eng := range router.engines { // access internal map within same package
		if m, ok := eng.(*MemoryEventBus); ok {
			mem = m
			break
		}
	}
	if mem == nil {
		t.Fatalf("expected memory engine present")
	}

	// 2. Subscribe with nil handler triggers ErrEventHandlerNil
	if _, err := mem.Subscribe(context.Background(), "x", nil); !errors.Is(err, ErrEventHandlerNil) {
		if err == nil {
			// Should never be nil
			t.Fatalf("expected error ErrEventHandlerNil, got nil")
		}
		t.Fatalf("expected ErrEventHandlerNil, got %v", err)
	}

	// 3. Unsubscribe invalid subscription type -> ErrInvalidSubscriptionType
	if err := mem.Unsubscribe(context.Background(), bogusSub{}); !errors.Is(err, ErrInvalidSubscriptionType) {
		t.Fatalf("expected ErrInvalidSubscriptionType, got %v", err)
	}

	// 4. Stats after Stop should stay stable and not panic
	delBefore, dropBefore := mem.Stats()
	if err := mem.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	delAfter, dropAfter := mem.Stats()
	if delAfter != delBefore || dropAfter != dropBefore {
		t.Fatalf("stats changed after stop")
	}
}
