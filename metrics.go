package modular

import "context"

// ModuleMetrics holds metrics collected from a single module.
type ModuleMetrics struct {
	Name   string
	Values map[string]float64
}

// MetricsProvider is an optional interface for modules that expose operational metrics.
type MetricsProvider interface {
	CollectMetrics(ctx context.Context) ModuleMetrics
}
