package reverseproxy

import (
	"context"
	"log/slog"

	"github.com/GoCodeAlone/modular"
)

// Compile-time assertions for v2 interfaces
var _ modular.MetricsProvider = (*ReverseProxyModule)(nil)
var _ modular.Drainable = (*ReverseProxyModule)(nil)

// CollectMetrics implements modular.MetricsProvider.
// It aggregates metrics from the internal MetricsCollector into the standard ModuleMetrics format.
func (m *ReverseProxyModule) CollectMetrics(_ context.Context) modular.ModuleMetrics {
	values := make(map[string]float64)

	// Always report backend count
	m.backendProxiesMutex.RLock()
	values["backend_count"] = float64(len(m.backendProxies))
	m.backendProxiesMutex.RUnlock()

	// If metrics are enabled and the collector exists, aggregate request/error totals
	if m.enableMetrics && m.metrics != nil {
		m.metrics.mu.RLock()
		var totalRequests, totalErrors int
		for _, count := range m.metrics.requestCounts {
			totalRequests += count
		}
		for _, count := range m.metrics.errorCounts {
			totalErrors += count
		}
		m.metrics.mu.RUnlock()

		values["total_requests"] = float64(totalRequests)
		values["total_errors"] = float64(totalErrors)
	}

	return modular.ModuleMetrics{
		Name:   m.Name(),
		Values: values,
	}
}

// PreStop implements modular.Drainable.
// It stops the health checker during the drain phase, before the full Stop() is called.
func (m *ReverseProxyModule) PreStop(ctx context.Context) error {
	if m.app != nil && m.app.Logger() != nil {
		m.app.Logger().Info("PreStop: draining reverseproxy module")
	} else {
		slog.InfoContext(ctx, "PreStop: draining reverseproxy module")
	}

	// Stop health checker early so backends aren't flapped during drain
	if m.healthChecker != nil {
		m.healthChecker.Stop(ctx)
	}

	return nil
}
