package eventbus

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestCustomMemoryErrorPaths covers Publish/Subscribe before Start and nil handler validation.
func TestCustomMemoryErrorPaths(t *testing.T) {
	ctx := context.Background()
	ebRaw, err := NewCustomMemoryEventBus(map[string]interface{}{"enableMetrics": false})
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	eb := ebRaw.(*CustomMemoryEventBus)

	// Publish before Start
	if err := eb.Publish(ctx, Event{Topic: "x"}); !errors.Is(err, ErrEventBusNotStarted) {
		t.Fatalf("expected ErrEventBusNotStarted publish, got %v", err)
	}
	// Subscribe before Start
	if _, err := eb.Subscribe(ctx, "x", func(context.Context, Event) error { return nil }); !errors.Is(err, ErrEventBusNotStarted) {
		t.Fatalf("expected ErrEventBusNotStarted subscribe, got %v", err)
	}
	if _, err := eb.SubscribeAsync(ctx, "x", func(context.Context, Event) error { return nil }); !errors.Is(err, ErrEventBusNotStarted) {
		t.Fatalf("expected ErrEventBusNotStarted subscribe async, got %v", err)
	}

	// Start now
	if err := eb.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Nil handler
	if _, err := eb.Subscribe(ctx, "y", nil); !errors.Is(err, ErrEventHandlerNil) {
		t.Fatalf("expected ErrEventHandlerNil got %v", err)
	}

	// Basic successful subscription after start
	sub, err := eb.Subscribe(ctx, "y", func(context.Context, Event) error { return nil })
	if err != nil {
		t.Fatalf("subscribe after start: %v", err)
	}
	if sub.Topic() != "y" {
		t.Fatalf("unexpected topic %s", sub.Topic())
	}

	// Publish should succeed now
	if err := eb.Publish(ctx, Event{Topic: "y"}); err != nil {
		t.Fatalf("publish after start: %v", err)
	}

	// Allow processing
	time.Sleep(20 * time.Millisecond)
	_ = eb.Stop(ctx)
}
