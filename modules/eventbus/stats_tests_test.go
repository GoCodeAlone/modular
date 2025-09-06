package eventbus

import (
	"context"
	"testing"
	"time"
)

// TestStatsAndPerEngineStats ensures stats accumulate per engine.
func TestStatsAndPerEngineStats(t *testing.T) {
	cfg := &EventBusConfig{Engines: []EngineConfig{{Name: "e1", Type: "memory", Config: map[string]interface{}{"workerCount": 1}}, {Name: "e2", Type: "memory", Config: map[string]interface{}{"workerCount": 1}}}, Routing: []RoutingRule{{Topics: []string{"a.*"}, Engine: "e1"}, {Topics: []string{"b.*"}, Engine: "e2"}}}
	if err := cfg.ValidateConfig(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	mod := NewModule().(*EventBusModule)
	mod.config = cfg
	router, err := NewEngineRouter(cfg)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	mod.router = router
	mod.logger = noopLogger{}
	router.SetModuleReference(mod)
	if err := mod.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer mod.Stop(context.Background())
	ctx := context.Background()
	_, _ = mod.Subscribe(ctx, "a.one", func(ctx context.Context, e Event) error { return nil })
	_, _ = mod.Subscribe(ctx, "b.two", func(ctx context.Context, e Event) error { return nil })
	_ = mod.Publish(ctx, "a.one", 1)
	_ = mod.Publish(ctx, "b.two", 2)
	_ = mod.Publish(ctx, "a.one", 3)
	// wait up to 200ms for synchronous delivery counters to update
	deadline := time.Now().Add(200 * time.Millisecond)
	var del uint64
	for time.Now().Before(deadline) {
		if d, _ := mod.Stats(); d >= 3 {
			del = d
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if del < 3 {
		t.Fatalf("expected delivered >=3 got %d", del)
	}
	per := mod.PerEngineStats()
	if len(per) != 2 {
		t.Fatalf("expected stats for 2 engines, got %d", len(per))
	}
	if per["e1"].Delivered == 0 || per["e2"].Delivered == 0 {
		t.Fatalf("expected delivered counts on both engines: %#v", per)
	}
}
