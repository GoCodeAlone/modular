package eventbus

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/modular"
)

// TestModuleStatsBeforeInit ensures Stats/PerEngineStats fast-paths when router is nil.
func TestModuleStatsBeforeInit(t *testing.T) {
	m := &EventBusModule{}
	d, r := m.Stats()
	if d != 0 || r != 0 {
		t.Fatalf("expected zero stats prior to init, got delivered=%d dropped=%d", d, r)
	}
	per := m.PerEngineStats()
	if len(per) != 0 {
		t.Fatalf("expected empty per-engine stats prior to init, got %v", per)
	}
}

// TestModuleEmitEventNoSubject covers EmitEvent error branch when no subject registered.
func TestModuleEmitEventNoSubject(t *testing.T) {
	m := &EventBusModule{logger: noopLogger{}}
	ev := modular.NewCloudEvent("com.modular.test.event", "test-source", map[string]interface{}{"k": "v"}, nil)
	if err := m.EmitEvent(context.Background(), ev); err == nil {
		t.Fatalf("expected ErrNoSubjectForEventEmission when emitting without subject")
	}
}

// TestModuleStartStopIdempotency exercises Start/Stop idempotent branches directly.
func TestModuleStartStopIdempotency(t *testing.T) {
	cfg := &EventBusConfig{Engine: "memory", WorkerCount: 1, DefaultEventBufferSize: 1, MaxEventQueueSize: 10, RetentionDays: 1}
	if err := cfg.ValidateConfig(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	router, err := NewEngineRouter(cfg)
	if err != nil {
		t.Fatalf("router: %v", err)
	}

	m := &EventBusModule{config: cfg, router: router, logger: noopLogger{}}

	// First start
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("first start: %v", err)
	}
	// Second start should be idempotent (no error)
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("second start (idempotent) unexpected error: %v", err)
	}

	// First stop
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("first stop: %v", err)
	}
	// Second stop should be idempotent (no error)
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("second stop (idempotent) unexpected error: %v", err)
	}
}

// TestModulePublishBeforeStart validates error path when publishing before engines started.
func TestModulePublishBeforeStart(t *testing.T) {
	cfg := &EventBusConfig{Engine: "memory", WorkerCount: 1, DefaultEventBufferSize: 1, MaxEventQueueSize: 10, RetentionDays: 1}
	if err := cfg.ValidateConfig(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	router, err := NewEngineRouter(cfg)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	m := &EventBusModule{config: cfg, router: router, logger: noopLogger{}}
	// Publish before Start -> underlying memory engine not started -> ErrEventBusNotStarted wrapped.
	if err := m.Publish(context.Background(), "pre.start.topic", "payload"); err == nil {
		t.Fatalf("expected error publishing before start")
	}
}
