package eventbus

import (
	"context"
	"testing"
	"time"
)

// Test basic publish/subscribe lifecycle using memory engine ensuring message receipt and stats increments.
func TestEventBusPublishSubscribeBasic(t *testing.T) {
	m := NewModule().(*EventBusModule)
	app := newMockApp()
	// Register default config section as RegisterConfig would
	if err := m.RegisterConfig(app); err != nil {
		t.Fatalf("register config: %v", err)
	}
	if err := m.Init(app); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer m.Stop(context.Background())

	received := make(chan struct{}, 1)
	_, err := m.Subscribe(context.Background(), "test.topic", func(ctx context.Context, e Event) error {
		if e.Topic != "test.topic" {
			t.Errorf("unexpected topic %s", e.Topic)
		}
		received <- struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := m.Publish(context.Background(), "test.topic", "payload"); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event delivery")
	}

	del, _ := m.Stats()
	if del == 0 {
		t.Fatalf("expected delivered stats > 0")
	}
}

// Test unsubscribe removes subscription and no further deliveries occur.
func TestEventBusUnsubscribe(t *testing.T) {
	m := NewModule().(*EventBusModule)
	app := newMockApp()
	if err := m.RegisterConfig(app); err != nil {
		t.Fatalf("register config: %v", err)
	}
	if err := m.Init(app); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer m.Stop(context.Background())

	count := 0
	sub, err := m.Subscribe(context.Background(), "once.topic", func(ctx context.Context, e Event) error { count++; return nil })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := m.Publish(context.Background(), "once.topic", 1); err != nil {
		t.Fatalf("publish1: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if count != 1 {
		t.Fatalf("expected 1 delivery got %d", count)
	}

	if err := m.Unsubscribe(context.Background(), sub); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	if err := m.Publish(context.Background(), "once.topic", 2); err != nil {
		t.Fatalf("publish2: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if count != 1 {
		t.Fatalf("expected no additional deliveries after unsubscribe")
	}
}

// Test async subscription processes events without blocking publisher.
func TestEventBusAsyncSubscription(t *testing.T) {
	m := NewModule().(*EventBusModule)
	app := newMockApp()
	if err := m.RegisterConfig(app); err != nil {
		t.Fatalf("register config: %v", err)
	}
	if err := m.Init(app); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer m.Stop(context.Background())

	received := make(chan struct{}, 1)
	_, err := m.SubscribeAsync(context.Background(), "async.topic", func(ctx context.Context, e Event) error { received <- struct{}{}; return nil })
	if err != nil {
		t.Fatalf("subscribe async: %v", err)
	}

	start := time.Now()
	if err := m.Publish(context.Background(), "async.topic", 123); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// We expect Publish to return quickly (well under 100ms) even if handler not yet executed.
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("publish blocked unexpectedly long")
	}

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for async delivery")
	}
}

// Removed local mockApp (reuse the one defined in module_test.go)
