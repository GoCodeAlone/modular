package eventbus

import (
	"context"
	"testing"
)

// TestMultiEngineRouting verifies that routing rules send topics to expected engines.
func TestMultiEngineRouting(t *testing.T) {
	cfg := &EventBusConfig{
		Engines: []EngineConfig{
			{Name: "memA", Type: "memory", Config: map[string]interface{}{"workerCount": 1, "maxEventQueueSize": 100}},
			{Name: "memB", Type: "memory", Config: map[string]interface{}{"workerCount": 1, "maxEventQueueSize": 100}},
		},
		Routing: []RoutingRule{
			{Topics: []string{"alpha.*"}, Engine: "memA"},
			{Topics: []string{"beta.*"}, Engine: "memB"},
		},
	}
	if err := cfg.ValidateConfig(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	mod := &EventBusModule{name: ModuleName}
	mod.config = cfg
	router, err := NewEngineRouter(cfg)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	mod.router = router
	// start engines
	if err := mod.router.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	// ensure engine selection
	if got := mod.router.GetEngineForTopic("alpha.event"); got != "memA" {
		t.Fatalf("expected memA for alpha.event, got %s", got)
	}
	if got := mod.router.GetEngineForTopic("beta.event"); got != "memB" {
		t.Fatalf("expected memB for beta.event, got %s", got)
	}
	// unmatched goes to default (first engine memA)
	if got := mod.router.GetEngineForTopic("gamma.event"); got != "memA" {
		t.Fatalf("expected default memA for gamma.event, got %s", got)
	}
}
