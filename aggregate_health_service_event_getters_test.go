package modular

import (
    "testing"
    "time"
)

// TestHealthStatusChangedEventGetters ensures the simple getter methods are covered.
func TestHealthStatusChangedEventGetters(t *testing.T) {
    ts := time.Now()
    ev := &HealthStatusChangedEvent{Timestamp: ts}

    if got := ev.GetEventType(); got != "health.aggregate.updated" {
        t.Fatalf("expected event type health.aggregate.updated, got %s", got)
    }
    if got := ev.GetEventSource(); got != "modular.core.health.aggregator" {
        t.Fatalf("expected event source modular.core.health.aggregator, got %s", got)
    }
    if got := ev.GetTimestamp(); !got.Equal(ts) {
        t.Fatalf("expected timestamp %v, got %v", ts, got)
    }
}
