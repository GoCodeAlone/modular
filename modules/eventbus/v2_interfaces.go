package eventbus

import (
	"context"

	"github.com/GoCodeAlone/modular"
)

// Compile-time interface assertions.
var (
	_ modular.MetricsProvider = (*EventBusModule)(nil)
	_ modular.Drainable       = (*EventBusModule)(nil)
)

// CollectMetrics implements modular.MetricsProvider.
// It exposes delivery statistics and topology counts for external monitoring.
func (m *EventBusModule) CollectMetrics(_ context.Context) modular.ModuleMetrics {
	delivered, dropped := m.Stats()

	topics := m.Topics()
	var subscriberTotal int
	for _, t := range topics {
		subscriberTotal += m.SubscriberCount(t)
	}

	return modular.ModuleMetrics{
		Name: m.Name(),
		Values: map[string]float64{
			"delivered_count":  float64(delivered),
			"dropped_count":    float64(dropped),
			"topic_count":      float64(len(topics)),
			"subscriber_count": float64(subscriberTotal),
		},
	}
}

// PreStop implements modular.Drainable.
// It logs the drain intent. Actual resource cleanup is handled by Stop().
func (m *EventBusModule) PreStop(_ context.Context) error {
	if m.logger != nil {
		m.logger.Info("EventBus drain phase starting — no new publishes will be accepted after Stop")
	}
	return nil
}
