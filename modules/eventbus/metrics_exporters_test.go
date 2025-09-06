package eventbus

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestPrometheusCollectorBasic ensures metrics reflect published vs dropped events.
func TestPrometheusCollectorBasic(t *testing.T) {
	// Use existing mock app helper (defined in module_test.go) to init module
	app := newMockApp()
	eb := NewModule().(*EventBusModule)
	if err := eb.RegisterConfig(app); err != nil {
		t.Fatalf("register config: %v", err)
	}
	if err := eb.Init(app); err != nil {
		t.Fatalf("init module: %v", err)
	}
	ctx := context.Background()
	if err := eb.Start(ctx); err != nil {
		t.Fatalf("start module: %v", err)
	}
	t.Cleanup(func() { _ = eb.Stop(ctx) })

	// Subscribe
	sub, err := eb.Subscribe(ctx, "metric.test", func(ctx context.Context, e Event) error { return nil })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer func() { _ = eb.Unsubscribe(ctx, sub) }()

	// Publish some events
	for i := 0; i < 5; i++ {
		if err := eb.Publish(ctx, "metric.test", i); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	time.Sleep(50 * time.Millisecond) // allow delivery

	collector := NewPrometheusCollector(eb, "modular_eventbus_test")
	reg := prometheus.NewRegistry()
	reg.MustRegister(collector)

	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatalf("expected metrics gathered")
	}

	// Scan for aggregate delivered metric >= published count
	var found bool
	for _, m := range metrics {
		if m.GetName() == "modular_eventbus_test_delivered_total" {
			for _, mm := range m.GetMetric() {
				engineLabel := ""
				for _, l := range mm.GetLabel() {
					if l.GetName() == "engine" {
						engineLabel = l.GetValue()
					}
				}
				if engineLabel == "_all" {
					if mm.GetCounter().GetValue() < 5 {
						t.Fatalf("expected delivered >=5 got %v", mm.GetCounter().GetValue())
					}
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("did not find aggregate delivered metric")
	}

	// Optional: ensure testutil package is actually linked (avoid linter complaining unused import in future edits)
	_ = testutil.CollectAndCount
}
