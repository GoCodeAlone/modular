package modular

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestObservableDecoratorLifecycleEvents_New(t *testing.T) { // renamed to avoid collisions
	cfg := &minimalConfig{Value: "ok"}
	cp := NewStdConfigProvider(cfg)
	logger := &noopLogger{}
	inner := NewStdApplication(cp, logger)
	var mu sync.Mutex
	received := map[string]int{}

	obsFn := func(ctx context.Context, e CloudEvent) error {
		mu.Lock()
		received[e.Type()]++
		mu.Unlock()
		return nil
	}

	o := NewObservableDecorator(inner, obsFn)
	if err := o.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := o.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := o.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Events are emitted via goroutines; allow a short grace period for delivery.
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	// We expect at least before/after events for init, start, stop
	wantTypes := []string{
		"com.modular.application.before.init", "com.modular.application.after.init",
		"com.modular.application.before.start", "com.modular.application.after.start",
		"com.modular.application.before.stop", "com.modular.application.after.stop",
	}
	for _, et := range wantTypes {
		if received[et] == 0 {
			t.Fatalf("expected event %s emitted", et)
		}
	}
	mu.Unlock()
}
