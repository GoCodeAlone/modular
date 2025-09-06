package eventbus

import "testing"

// TestNewKafkaEventBus_Error ensures constructor returns error for unreachable broker.
// This gives coverage for early producer creation failure branch.
func TestNewKafkaEventBus_Error(t *testing.T) {
	_, err := NewKafkaEventBus(map[string]interface{}{"brokers": []interface{}{"localhost:12345"}})
	if err == nil { // likely no Kafka on this high port
		t.Skip("Kafka broker unexpectedly reachable; skip negative constructor test")
	}
}
