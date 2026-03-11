package modular

import (
	"context"
	"testing"
)

type metricsTestModule struct {
	name string
}

func (m *metricsTestModule) Name() string               { return m.name }
func (m *metricsTestModule) Init(app Application) error { return nil }
func (m *metricsTestModule) CollectMetrics(ctx context.Context) ModuleMetrics {
	return ModuleMetrics{
		Name:   m.name,
		Values: map[string]float64{"requests_total": 100, "error_rate": 0.02},
	}
}

type nonMetricsModule struct {
	name string
}

func (m *nonMetricsModule) Name() string               { return m.name }
func (m *nonMetricsModule) Init(app Application) error { return nil }

func TestCollectAllMetrics(t *testing.T) {
	modA := &metricsTestModule{name: "api"}
	modB := &nonMetricsModule{name: "no-metrics"}

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithModules(modA, modB),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	stdApp := app.(*StdApplication)
	metrics := stdApp.CollectAllMetrics(context.Background())

	if len(metrics) != 1 {
		t.Fatalf("expected 1 metrics result, got %d", len(metrics))
	}
	if metrics[0].Name != "api" {
		t.Errorf("expected api, got %s", metrics[0].Name)
	}
	if metrics[0].Values["requests_total"] != 100 {
		t.Errorf("expected requests_total=100, got %v", metrics[0].Values["requests_total"])
	}
}

func TestCollectAllMetrics_NoProviders(t *testing.T) {
	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithModules(&nonMetricsModule{name: "plain"}),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	stdApp := app.(*StdApplication)
	metrics := stdApp.CollectAllMetrics(context.Background())
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics, got %d", len(metrics))
	}
}
