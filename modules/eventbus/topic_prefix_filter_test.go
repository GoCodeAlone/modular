package eventbus

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestTopicPrefixFilter ensures filtering works when configured.
func TestTopicPrefixFilter(t *testing.T) {
	ctx := context.Background()
	ebRaw, err := NewCustomMemoryEventBus(map[string]interface{}{})
	if err != nil {
		t.Fatalf("create bus: %v", err)
	}
	// inject a topic prefix filter manually since constructor only reads config at creation
	bus := ebRaw.(*CustomMemoryEventBus)
	bus.eventFilters = append(bus.eventFilters, &TopicPrefixFilter{AllowedPrefixes: []string{"allow."}, name: "topicPrefix"})

	if err := bus.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	var received int64
	sub, err := bus.Subscribe(ctx, "allow.something", func(ctx context.Context, e Event) error { atomic.AddInt64(&received, 1); return nil })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	_ = sub // ensure retained

	// allowed topic
	if err := bus.Publish(ctx, Event{Topic: "allow.something"}); err != nil {
		t.Fatalf("publish allow: %v", err)
	}
	// disallowed topic (different prefix) should be dropped
	if err := bus.Publish(ctx, Event{Topic: "deny.something"}); err != nil {
		t.Fatalf("publish deny: %v", err)
	}

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&received) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt64(&received) != 1 {
		t.Fatalf("expected only 1 allowed event processed got %d", atomic.LoadInt64(&received))
	}

	// sanity: publishing more allowed events increments counter
	// publish another allowed event on subscribed topic to guarantee delivery
	if err := bus.Publish(ctx, Event{Topic: "allow.something"}); err != nil {
		t.Fatalf("publish allow2: %v", err)
	}
	deadline = time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&received) == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt64(&received) != 2 {
		t.Fatalf("expected 2 total allowed events got %d", atomic.LoadInt64(&received))
	}

	_ = bus.Stop(ctx)
}
