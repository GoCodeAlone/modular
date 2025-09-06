package eventbus

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestCustomMemoryUnsubscribe ensures Unsubscribe detaches subscription and halts delivery.
func TestCustomMemoryUnsubscribe(t *testing.T) {
	ctx := context.Background()
	ebRaw, err := NewCustomMemoryEventBus(map[string]interface{}{})
	if err != nil {
		t.Fatalf("create bus: %v", err)
	}
	eb := ebRaw.(*CustomMemoryEventBus)
	if err := eb.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	var count int64
	sub, err := eb.Subscribe(ctx, "beta.topic", func(ctx context.Context, e Event) error { atomic.AddInt64(&count, 1); return nil })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// initial event to ensure live
	if err := eb.Publish(ctx, Event{Topic: "beta.topic"}); err != nil {
		t.Fatalf("publish1: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&count) == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if atomic.LoadInt64(&count) != 1 {
		t.Fatalf("expected first event processed, got %d", atomic.LoadInt64(&count))
	}

	// unsubscribe and publish some more events which should not be processed
	if err := eb.Unsubscribe(ctx, sub); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	for i := 0; i < 3; i++ {
		_ = eb.Publish(ctx, Event{Topic: "beta.topic"})
	}
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt64(&count) != 1 {
		t.Fatalf("expected no further events after unsubscribe, got %d", atomic.LoadInt64(&count))
	}

	// confirm subscriber count for topic now zero
	if c := eb.SubscriberCount("beta.topic"); c != 0 {
		t.Fatalf("expected 0 subscribers got %d", c)
	}
	_ = eb.Stop(ctx)
}
