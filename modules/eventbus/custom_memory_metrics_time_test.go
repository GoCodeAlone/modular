package eventbus

import (
	"context"
	"testing"
	"time"
)

// TestCustomMemoryMetricsAverageTime ensures AverageProcessingTime becomes >0 after processing varied durations.
func TestCustomMemoryMetricsAverageTime(t *testing.T) {
	ctx := context.Background()
	ebRaw, err := NewCustomMemoryEventBus(map[string]interface{}{"metricsInterval": "50ms"})
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	eb := ebRaw.(*CustomMemoryEventBus)
	if err := eb.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// handler with alternating small and larger sleeps
	var i int
	_, err = eb.Subscribe(ctx, "timed.topic", func(context.Context, Event) error {
		if i%2 == 0 {
			time.Sleep(5 * time.Millisecond)
		} else {
			time.Sleep(15 * time.Millisecond)
		}
		i++
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	for n := 0; n < 6; n++ {
		if err := eb.Publish(ctx, Event{Topic: "timed.topic"}); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	// wait for processing and at least one metrics collector tick
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if eb.GetMetrics().AverageProcessingTime > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if eb.GetMetrics().AverageProcessingTime <= 0 {
		t.Fatalf("expected average processing time > 0, got %v", eb.GetMetrics().AverageProcessingTime)
	}
	_ = eb.Stop(ctx)
}
