package modular

import (
	"context"
	"testing"
	"time"
)

type drainableModule struct {
	name       string
	preStopSeq *[]string
	stopSeq    *[]string
}

func (m *drainableModule) Name() string                    { return m.name }
func (m *drainableModule) Init(app Application) error      { return nil }
func (m *drainableModule) Start(ctx context.Context) error { return nil }
func (m *drainableModule) PreStop(ctx context.Context) error {
	*m.preStopSeq = append(*m.preStopSeq, m.name)
	return nil
}
func (m *drainableModule) Stop(ctx context.Context) error {
	*m.stopSeq = append(*m.stopSeq, m.name)
	return nil
}

func TestDrainable_PreStopCalledBeforeStop(t *testing.T) {
	preStops := make([]string, 0)
	stops := make([]string, 0)

	mod := &drainableModule{name: "drainer", preStopSeq: &preStops, stopSeq: &stops}

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithModules(mod),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := app.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := app.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if len(preStops) != 1 || preStops[0] != "drainer" {
		t.Errorf("expected PreStop called for drainer, got %v", preStops)
	}
	if len(stops) != 1 || stops[0] != "drainer" {
		t.Errorf("expected Stop called for drainer, got %v", stops)
	}
}

func TestDrainable_WithDrainTimeout(t *testing.T) {
	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithDrainTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}

	stdApp := app.(*StdApplication)
	if stdApp.drainTimeout != 5*time.Second {
		t.Errorf("expected drain timeout 5s, got %v", stdApp.drainTimeout)
	}
}
