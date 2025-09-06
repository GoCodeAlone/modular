package eventbus

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestCustomMemorySubscriptionAndMetrics covers Subscribe, SubscribeAsync, ProcessedEvents, IsAsync, Topic, Publish metrics, GetMetrics, and Stop.
func TestCustomMemorySubscriptionAndMetrics(t *testing.T) {
	ctx := context.Background()
	ebRaw, err := NewCustomMemoryEventBus(map[string]interface{}{
		"enableMetrics":          true,
		"metricsInterval":        "100ms", // fast tick so metricsCollector branch executes at least once
		"defaultEventBufferSize": 5,
	})
	if err != nil {
		t.Fatalf("failed creating custom memory bus: %v", err)
	}
	eb := ebRaw.(*CustomMemoryEventBus)

	if err := eb.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// synchronous subscription
	var syncCount int64
	subSync, err := eb.Subscribe(ctx, "alpha.topic", func(ctx context.Context, e Event) error {
		atomic.AddInt64(&syncCount, 1)
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe sync failed: %v", err)
	}
	if subSync.Topic() != "alpha.topic" {
		t.Fatalf("expected topic alpha.topic got %s", subSync.Topic())
	}
	if subSync.IsAsync() {
		t.Fatalf("expected sync subscription")
	}

	// async subscription
	var asyncCount int64
	subAsync, err := eb.SubscribeAsync(ctx, "alpha.topic", func(ctx context.Context, e Event) error {
		atomic.AddInt64(&asyncCount, 1)
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe async failed: %v", err)
	}
	if !subAsync.IsAsync() {
		t.Fatalf("expected async subscription")
	}

	// publish several events
	totalEvents := 4
	for i := 0; i < totalEvents; i++ {
		if err := eb.Publish(ctx, Event{Topic: "alpha.topic"}); err != nil {
			t.Fatalf("publish failed: %v", err)
		}
	}

	// wait for async handler to process
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&syncCount) == int64(totalEvents) && atomic.LoadInt64(&asyncCount) == int64(totalEvents) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt64(&syncCount) != int64(totalEvents) || atomic.LoadInt64(&asyncCount) != int64(totalEvents) {
		t.Fatalf("handlers did not process all events: sync=%d async=%d", atomic.LoadInt64(&syncCount), atomic.LoadInt64(&asyncCount))
	}

	// validate ProcessedEvents counters on underlying subscription concrete types
	if cs, ok := subSync.(*customMemorySubscription); ok {
		if ce := cs.ProcessedEvents(); ce != int64(totalEvents) {
			t.Fatalf("expected sync processed %d got %d", totalEvents, ce)
		}
	} else {
		t.Fatalf("expected customMemorySubscription concrete type for sync subscription")
	}
	if ca, ok := subAsync.(*customMemorySubscription); ok {
		if ce := ca.ProcessedEvents(); ce != int64(totalEvents) {
			t.Fatalf("expected async processed %d got %d", totalEvents, ce)
		}
	} else {
		t.Fatalf("expected customMemorySubscription concrete type for async subscription")
	}

	// metrics should reflect at least total events
	metrics := eb.GetMetrics()
	if metrics.TotalEvents < int64(totalEvents) { // could be exactly equal
		t.Fatalf("expected metrics totalEvents >= %d got %d", totalEvents, metrics.TotalEvents)
	}
	if metrics.EventsPerTopic["alpha.topic"] < int64(totalEvents) {
		t.Fatalf("expected metrics eventsPerTopic >= %d got %d", totalEvents, metrics.EventsPerTopic["alpha.topic"])
	}

	// allow metricsCollector to tick at least once
	time.Sleep(120 * time.Millisecond)

	if err := eb.Stop(ctx); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}
