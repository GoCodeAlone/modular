package modular

import (
	"context"
	"testing"
	"time"
)

// dummyHealthProvider minimal implementation for aggregation tests.
type dummyHealthProvider struct{ id string }

func (d *dummyHealthProvider) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	return []HealthReport{{Module: d.id, Status: HealthStatusHealthy, CheckedAt: time.Now(), ObservedSince: time.Now()}}, nil
}

// TestBasicHealthAggregatorCollect ensures Collect returns a healthy aggregate snapshot.
func TestBasicHealthAggregatorCollect(t *testing.T) {
	agg := &BasicHealthAggregator{}
	// collecting with no providers still returns healthy defaults per implementation
	h, err := agg.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect error: %v", err)
	}
	if h.Health != HealthStatusHealthy || h.Readiness != HealthStatusHealthy {
		t.Fatalf("unexpected statuses: %+v", h)
	}
	if len(h.Reports) != 0 {
		t.Fatalf("expected no reports, got %d", len(h.Reports))
	}
}

// TestBasicHealthAggregatorRegisterProvider validates provider registration mutates internal slice.
func TestBasicHealthAggregatorRegisterProvider(t *testing.T) {
	agg := &BasicHealthAggregator{}
	p1 := &dummyHealthProvider{id: "mod1"}
	p2 := &dummyHealthProvider{id: "mod2"}
	agg.RegisterProvider(p1)
	agg.RegisterProvider(p2)
	if len(agg.providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(agg.providers))
	}
	// Invoke providers to assert ordering indirectly
	r1, _ := agg.providers[0].HealthCheck(context.Background())
	r2, _ := agg.providers[1].HealthCheck(context.Background())
	if r1[0].Module != "mod1" || r2[0].Module != "mod2" {
		t.Fatalf("unexpected provider order: %v %v", r1[0].Module, r2[0].Module)
	}
}
