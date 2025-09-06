package eventbus

import (
	"context"
	"testing"
)

// TestPublishBeforeStart ensures publish returns an error when bus not started.
func TestPublishBeforeStart(t *testing.T) {
	cfg := &EventBusConfig{Engine: "memory", MaxEventQueueSize: 10, DefaultEventBufferSize: 2, WorkerCount: 1}
	_ = cfg.ValidateConfig()
	mod := NewModule().(*EventBusModule)
	// mimic Init minimal pieces
	mod.config = cfg
	router, err := NewEngineRouter(cfg)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	mod.router = router
	// Intentionally do NOT call Start
	if err := mod.Publish(context.Background(), "test.topic", "data"); err == nil {
		// Underlying memory engine should not be started -> engine.Publish should error
		// We rely on ErrEventBusNotStarted bubbling
		// If implementation changes, adapt expectation.
		// For now, assert non-nil error.
		// Provide explicit failure message.
		// NOTE: MemoryEventBus Start sets isStarted; without Start, Publish returns ErrEventBusNotStarted.
		// So nil error here means regression.
		t.Fatalf("expected error publishing before Start")
	}
}
