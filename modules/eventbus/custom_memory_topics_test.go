package eventbus

import (
	"context"
	"testing"
	"time"
)

// TestCustomMemoryTopicsAndCounts exercises Topics() and SubscriberCount() behaviors.
func TestCustomMemoryTopicsAndCounts(t *testing.T) {
	ctx := context.Background()
	ebRaw, err := NewCustomMemoryEventBus(map[string]interface{}{})
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	eb := ebRaw.(*CustomMemoryEventBus)
	if err := eb.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// initial: no topics
	if len(eb.Topics()) != 0 {
		t.Fatalf("expected 0 topics initially")
	}

	// subscribe to specific topics
	subA, _ := eb.Subscribe(ctx, "topic.a", func(context.Context, Event) error { return nil })
	subB, _ := eb.Subscribe(ctx, "topic.b", func(context.Context, Event) error { return nil })
	// wildcard subscription
	subAll, _ := eb.Subscribe(ctx, "topic.*", func(context.Context, Event) error { return nil })
	_ = subAll

	topics := eb.Topics()
	if len(topics) != 3 {
		t.Fatalf("expected 3 topics got %d: %v", len(topics), topics)
	}
	if eb.SubscriberCount("topic.a") != 1 {
		t.Fatalf("expected 1 subscriber topic.a")
	}
	if eb.SubscriberCount("topic.b") != 1 {
		t.Fatalf("expected 1 subscriber topic.b")
	}
	if eb.SubscriberCount("topic.*") != 1 {
		t.Fatalf("expected 1 subscriber wildcard topic.*")
	}

	// publish events to exercise matchesTopic logic indirectly
	if err := eb.Publish(ctx, Event{Topic: "topic.a"}); err != nil {
		t.Fatalf("publish a: %v", err)
	}
	if err := eb.Publish(ctx, Event{Topic: "topic.b"}); err != nil {
		t.Fatalf("publish b: %v", err)
	}
	if err := eb.Publish(ctx, Event{Topic: "topic.c"}); err != nil {
		t.Fatalf("publish c: %v", err)
	}

	time.Sleep(30 * time.Millisecond)

	// Unsubscribe one specific topic
	if err := eb.Unsubscribe(ctx, subA); err != nil {
		t.Fatalf("unsubscribe a: %v", err)
	}
	// keep subB active to ensure selective removal works
	if subB.Topic() != "topic.b" {
		t.Fatalf("unexpected topic for subB")
	}
	if eb.SubscriberCount("topic.a") != 0 {
		t.Fatalf("expected 0 subs for topic.a after unsubscribe")
	}

	// topics should now be 2 or 3 depending on immediate cleanup; after unsubscribe if map empty it is removed
	remaining := eb.Topics()
	// ensure topic.a removed
	for _, tname := range remaining {
		if tname == "topic.a" {
			t.Fatalf("topic.a should have been removed")
		}
	}

	_ = eb.Stop(ctx)
}
