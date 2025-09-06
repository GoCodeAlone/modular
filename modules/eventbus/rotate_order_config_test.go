package eventbus

import (
	"testing"
)

// TestRotateSubscriberOrderDefault verifies that the validation logic no longer forces
// RotateSubscriberOrder=true when the user leaves it unset/false.
func TestRotateSubscriberOrderDefault(t *testing.T) {
	cfg := &EventBusConfig{ // single-engine legacy mode; leave RotateSubscriberOrder false
		Engine:                 "memory",
		MaxEventQueueSize:      10,
		DefaultEventBufferSize: 1,
		WorkerCount:            1,
	}
	if err := cfg.ValidateConfig(); err != nil {
		// Should not fail validation
		t.Fatalf("ValidateConfig error: %v", err)
	}
	if cfg.RotateSubscriberOrder {
		t.Fatalf("expected RotateSubscriberOrder to remain false by default, got true")
	}
}

// TestRotateSubscriberOrderExplicitTrue ensures an explicitly enabled value remains true.
func TestRotateSubscriberOrderExplicitTrue(t *testing.T) {
	cfg := &EventBusConfig{ // explicit enable
		Engine:                 "memory",
		MaxEventQueueSize:      10,
		DefaultEventBufferSize: 1,
		WorkerCount:            1,
		RotateSubscriberOrder:  true,
	}
	if err := cfg.ValidateConfig(); err != nil {
		t.Fatalf("ValidateConfig error: %v", err)
	}
	if !cfg.RotateSubscriberOrder {
		t.Fatalf("expected RotateSubscriberOrder to stay true when explicitly set")
	}
}
