package eventbus

import (
	"context"
	"testing"
)

// TestCustomMemoryStartStopIdempotent covers Start/Stop early return branches.
func TestCustomMemoryStartStopIdempotent(t *testing.T) {
	ctx := context.Background()
	ebRaw, err := NewCustomMemoryEventBus(map[string]interface{}{})
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	eb := ebRaw.(*CustomMemoryEventBus)

	// Stop before Start should be no-op
	if err := eb.Stop(ctx); err != nil {
		t.Fatalf("stop before start: %v", err)
	}

	// First start
	if err := eb.Start(ctx); err != nil {
		t.Fatalf("start1: %v", err)
	}
	// Second start (idempotent)
	if err := eb.Start(ctx); err != nil {
		t.Fatalf("start2: %v", err)
	}

	// First stop
	if err := eb.Stop(ctx); err != nil {
		t.Fatalf("stop1: %v", err)
	}
	// Second stop (idempotent)
	if err := eb.Stop(ctx); err != nil {
		t.Fatalf("stop2: %v", err)
	}
}
