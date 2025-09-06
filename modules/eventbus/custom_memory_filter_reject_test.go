package eventbus

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestCustomMemoryFilterReject ensures events not matching TopicPrefixFilter are skipped without metrics increment.
func TestCustomMemoryFilterReject(t *testing.T) {
	busRaw, err := NewCustomMemoryEventBus(map[string]interface{}{
		"enableMetrics":          true,
		"defaultEventBufferSize": 1,
	})
	if err != nil {
		t.Fatalf("create bus: %v", err)
	}
	bus := busRaw.(*CustomMemoryEventBus)

	// Inject a filter allowing only topics starting with "allow.".
	bus.eventFilters = []EventFilter{&TopicPrefixFilter{AllowedPrefixes: []string{"allow."}, name: "topicPrefix"}}
	if err := bus.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Subscribe to both allowed and denied topics; only allowed should receive events.
	var allowedCount int64
	var deniedCount int64
	_, err = bus.Subscribe(context.Background(), "allow.test", func(ctx context.Context, e Event) error { atomic.AddInt64(&allowedCount, 1); return nil })
	if err != nil {
		t.Fatalf("subscribe allow: %v", err)
	}
	_, err = bus.Subscribe(context.Background(), "deny.test", func(ctx context.Context, e Event) error { atomic.AddInt64(&deniedCount, 1); return nil })
	if err != nil {
		t.Fatalf("subscribe deny: %v", err)
	}

	// Publish one denied event and one allowed; denied should be filtered out early.
	_ = bus.Publish(context.Background(), Event{Topic: "deny.test"})
	_ = bus.Publish(context.Background(), Event{Topic: "allow.test"})

	// Wait briefly for allowed delivery.
	time.Sleep(20 * time.Millisecond)

	if atomic.LoadInt64(&allowedCount) != 1 {
		t.Fatalf("expected allowedCount=1 got %d", atomic.LoadInt64(&allowedCount))
	}
	if atomic.LoadInt64(&deniedCount) != 0 {
		t.Fatalf("expected deniedCount=0 got %d", atomic.LoadInt64(&deniedCount))
	}

	metrics := bus.GetMetrics()
	if metrics.TotalEvents != 1 {
		t.Fatalf("expected metrics.TotalEvents=1 got %d", metrics.TotalEvents)
	}
	if metrics.EventsPerTopic["deny.test"] != 0 {
		t.Fatalf("deny.test should not be counted")
	}
	if metrics.EventsPerTopic["allow.test"] != 1 {
		t.Fatalf("allow.test metrics missing")
	}
}
