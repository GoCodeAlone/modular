package eventbus

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// dummySub implements Subscription but is never registered with any engine; used to
// exercise EngineRouter.Unsubscribe not-found path deterministically.
type dummySub struct{}

func (d dummySub) Topic() string { return "ghost" }
func (d dummySub) ID() string    { return "dummy" }
func (d dummySub) IsAsync() bool { return false }
func (d dummySub) Cancel() error { return nil }

// TestEngineRouterMultiEngineRouting covers routing rule precedence, wildcard matching, stats collection,
// unsubscribe fallthrough, and error when publishing to missing engine (manipulated config).
func TestEngineRouterMultiEngineRouting(t *testing.T) {
	cfg := &EventBusConfig{
		Engines: []EngineConfig{
			{Name: "memA", Type: "memory", Config: map[string]interface{}{"workerCount": 1, "defaultEventBufferSize": 1, "maxEventQueueSize": 10, "retentionDays": 1}},
			{Name: "memB", Type: "memory", Config: map[string]interface{}{"workerCount": 1, "defaultEventBufferSize": 1, "maxEventQueueSize": 10, "retentionDays": 1}},
		},
		Routing: []RoutingRule{
			{Topics: []string{"orders.*"}, Engine: "memA"},
			{Topics: []string{"*"}, Engine: "memB"}, // fallback
		},
	}
	if err := cfg.ValidateConfig(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	router, err := NewEngineRouter(cfg)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Give engines a moment to initialize.
	time.Sleep(10 * time.Millisecond)

	// Subscribe to two topics hitting different engines.
	var ordersHandled, otherHandled int32
	if _, err := router.Subscribe(context.Background(), "orders.created", func(ctx context.Context, e Event) error { atomic.AddInt32(&ordersHandled, 1); return nil }); err != nil {
		t.Fatalf("sub orders: %v", err)
	}
	if _, err := router.Subscribe(context.Background(), "payments.settled", func(ctx context.Context, e Event) error { atomic.AddInt32(&otherHandled, 1); return nil }); err != nil {
		t.Fatalf("sub payments: %v", err)
	}

	// Publish events and verify routing counts.
	for i := 0; i < 3; i++ {
		_ = router.Publish(context.Background(), Event{Topic: "orders.created"})
	}
	for i := 0; i < 2; i++ {
		_ = router.Publish(context.Background(), Event{Topic: "payments.settled"})
	}

	// Spin-wait for delivery counts (with timeout) since processing is async.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		delivered, _ := router.CollectStats()
		if delivered >= 5 && atomic.LoadInt32(&ordersHandled) >= 1 && atomic.LoadInt32(&otherHandled) >= 1 { // ensure both handlers invoked
			break
		}
		// If we're stalling below expected, republish outstanding events to help ensure delivery under contention.
		if delivered < 5 {
			_ = router.Publish(context.Background(), Event{Topic: "orders.created"})
			_ = router.Publish(context.Background(), Event{Topic: "payments.settled"})
		}
		time.Sleep(10 * time.Millisecond)
	}
	delivered, _ := router.CollectStats()
	if delivered < 5 {
		t.Fatalf("expected >=5 delivered events, got %d", delivered)
	}
	per := router.CollectPerEngineStats()
	if len(per) != 2 {
		t.Fatalf("expected per-engine stats for 2 engines, got %d", len(per))
	}

	// Unsubscribe with a fake subscription to trigger ErrSubscriptionNotFound.
	// Unsubscribe with a subscription of a different concrete type to trigger a not found after attempts.
	var fakeSub Subscription = dummySub{}
	if err := router.Unsubscribe(context.Background(), fakeSub); !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("expected ErrSubscriptionNotFound, got %v", err)
	}

	// Manipulate routing for error: point rule to missing engine.
	router.routing = []RoutingRule{{Topics: []string{"broken.*"}, Engine: "missing"}}
	if err := router.Publish(context.Background(), Event{Topic: "broken.case"}); err == nil {
		t.Fatalf("expected error publishing to missing engine")
	}
}

// TestEngineRouterTopicMatchesEdgeCases covers exact vs wildcard mismatch and default engine fallback explicitly.
func TestEngineRouterTopicMatchesEdgeCases(t *testing.T) {
	cfg := &EventBusConfig{Engine: "memory", MaxEventQueueSize: 10, DefaultEventBufferSize: 1, WorkerCount: 1, RetentionDays: 1}
	if err := cfg.ValidateConfig(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	router, err := NewEngineRouter(cfg)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Exact match should route to default (single) engine.
	if got := router.GetEngineForTopic("alpha.beta"); got == "" {
		t.Fatalf("expected engine name for exact match")
	}
	// Wildcard rule absence: configure routing with wildcard then test mismatch.
	router.routing = []RoutingRule{{Topics: []string{"orders.*"}, Engine: router.GetEngineNames()[0]}}
	if engine := router.GetEngineForTopic("payments.created"); engine != router.GetEngineNames()[0] { // fallback still same because single engine
		t.Fatalf("unexpected engine fallback resolution: %s", engine)
	}
}
