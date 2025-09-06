package eventbus

import (
	"context"
	"errors"
	"testing"
)

// TestKafkaGuardClauses covers early-return guard paths without needing a real Kafka cluster.
func TestKafkaGuardClauses(t *testing.T) {
	k := &KafkaEventBus{} // zero value (not started, nil producer/consumer)

	// Publish before start
	if err := k.Publish(context.Background(), Event{Topic: "t"}); !errors.Is(err, ErrEventBusNotStarted) {
		t.Fatalf("expected ErrEventBusNotStarted publishing, got %v", err)
	}
	if _, err := k.Subscribe(context.Background(), "t", func(ctx context.Context, e Event) error { return nil }); !errors.Is(err, ErrEventBusNotStarted) {
		t.Fatalf("expected ErrEventBusNotStarted subscribing, got %v", err)
	}
	if err := k.Unsubscribe(context.Background(), &kafkaSubscription{}); !errors.Is(err, ErrEventBusNotStarted) {
		t.Fatalf("expected ErrEventBusNotStarted unsubscribing, got %v", err)
	}

	// Start (safe even with nil producer/consumer) then exercise simple methods.
	if err := k.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !k.isStarted {
		t.Fatalf("expected isStarted true after Start")
	}

	// Kafka subscription simple methods & cancel idempotency.
	sub := &kafkaSubscription{topic: "t", id: "id", done: make(chan struct{}), handler: func(ctx context.Context, e Event) error { return errors.New("boom") }, bus: k}
	if sub.Topic() != "t" || sub.ID() != "id" || sub.IsAsync() {
		t.Fatalf("unexpected subscription getters")
	}
	if err := sub.Cancel(); err != nil {
		t.Fatalf("cancel1: %v", err)
	}
	if err := sub.Cancel(); err != nil {
		t.Fatalf("cancel2 idempotent: %v", err)
	}

	// Consumer group handler trivial methods & topic matching.
	h := &KafkaConsumerGroupHandler{}
	if err := h.Setup(nil); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := h.Cleanup(nil); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if !h.topicMatches("orders.created", "orders.*") {
		t.Fatalf("expected wildcard match")
	}
	if h.topicMatches("orders.created", "payments.*") {
		t.Fatalf("did not expect match")
	}

	// Process event (synchronous path) including error logging branch.
	k.processEvent(sub, Event{Topic: "t"})
}
