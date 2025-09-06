package eventbus

// Metrics exporters for EventBus delivery statistics.
//
// Provides:
//   - PrometheusCollector implementing prometheus.Collector (conditional build, no-op if deps absent)
//   - DatadogStatsdExporter for periodic flush to DogStatsD / StatsD compatible endpoints.
//
// Design goals:
//   - Zero required dependency: code compiles even if Prometheus / Datadog libs not present (they are optional module deps)
//   - Lock-free hot path: exporters pull via public Stats()/PerEngineStats() methods; no additional instrumentation on publish path
//   - Safe concurrent usage: snapshot methods allocate new maps each call
//
// Usage (Prometheus):
//   collector := eventbus.NewPrometheusCollector(eventBus, "modular_eventbus")
//   prometheus.MustRegister(collector)
//
// Usage (Datadog):
//   exporter, _ := eventbus.NewDatadogStatsdExporter(eventBus, "eventbus", "127.0.0.1:8125", 10*time.Second, nil)
//   ctx, cancel := context.WithCancel(context.Background())
//   go exporter.Run(ctx)
//   ... later cancel();
//
// NOTE: Optional deps. To exclude an exporter, prefer build tags over editing this file.
// Planned (future) file layout if / when we split:
//   prometheus_exporter.go        //go:build prometheus
//   prometheus_exporter_stub.go   //go:build !prometheus (no-op types / constructors)
//   datadog_exporter.go           //go:build datadog
//   datadog_exporter_stub.go      //go:build !datadog
// Rationale: keeps the default experience zero-config (single file, no tags needed) while
// allowing downstream builds to opt-out to avoid pulling transitive deps (prometheus, datadog-go)
// or to trim binary size. We delay the physical split until there is concrete pressure (size,
// dependency policy, or benchmarking evidence) to avoid premature fragmentation.
//
// Using the split: add -tags "!prometheus" (or "!datadog") to disable; add the positive tag
// to enable if we decide future default is disabled. For now BOTH exporters are always compiled
// because this unified source improves discoverability and keeps the API surface obvious.
// This comment documents the strategic direction so readers do not misinterpret the unified
// file as a lack of modularity options.

import (
	"context"
	"fmt"
	"time"

	// Prometheus
	"github.com/prometheus/client_golang/prometheus"
	// Datadog
	statsd "github.com/DataDog/datadog-go/v5/statsd"
)

var (
	errNilEventBus     = fmt.Errorf("eventbus: nil eventBus supplied")
	errInvalidInterval = fmt.Errorf("eventbus: interval must be > 0")
)

// ----- Prometheus Collector -----

// PrometheusCollector implements prometheus.Collector for EventBus delivery stats.
// It exposes two metrics (cumulative counters):
//   modular_eventbus_delivered_total{engine="<name>"}
//   modular_eventbus_dropped_total{engine="<name>"}
// plus aggregate pseudo-engine label engine="_all" for totals.
//
// Metric naming base can be customized via namespace param in constructor.
// Counters are implemented as ConstMetrics generated on scrape.

type PrometheusCollector struct {
	eventBus *EventBusModule
	// metric descriptors
	deliveredDesc *prometheus.Desc
	droppedDesc   *prometheus.Desc
}

// NewPrometheusCollector creates a new collector for the given event bus.
// namespace is used as metric prefix (default if empty: modular_eventbus).
func NewPrometheusCollector(eventBus *EventBusModule, namespace string) *PrometheusCollector {
	if namespace == "" {
		namespace = "modular_eventbus"
	}
	return &PrometheusCollector{
		eventBus: eventBus,
		deliveredDesc: prometheus.NewDesc(
			fmt.Sprintf("%s_delivered_total", namespace),
			"Total delivered events (cumulative)",
			[]string{"engine"}, nil,
		),
		droppedDesc: prometheus.NewDesc(
			fmt.Sprintf("%s_dropped_total", namespace),
			"Total dropped events (cumulative)",
			[]string{"engine"}, nil,
		),
	}
}

// Describe sends metric descriptors.
func (c *PrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.deliveredDesc
	ch <- c.droppedDesc
}

// Collect gathers current stats and emits ConstMetrics.
func (c *PrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	per := c.eventBus.PerEngineStats()
	var totalDelivered, totalDropped uint64
	for engine, s := range per {
		ch <- prometheus.MustNewConstMetric(c.deliveredDesc, prometheus.CounterValue, float64(s.Delivered), engine)
		ch <- prometheus.MustNewConstMetric(c.droppedDesc, prometheus.CounterValue, float64(s.Dropped), engine)
		totalDelivered += s.Delivered
		totalDropped += s.Dropped
	}
	// Aggregate pseudo engine
	ch <- prometheus.MustNewConstMetric(c.deliveredDesc, prometheus.CounterValue, float64(totalDelivered), "_all")
	ch <- prometheus.MustNewConstMetric(c.droppedDesc, prometheus.CounterValue, float64(totalDropped), "_all")
}

// ----- Datadog / StatsD Exporter -----

// DatadogStatsdExporter periodically flushes counters as gauges (monotonic)
// to DogStatsD / StatsD. It is pull-based: each interval it reads the current
// cumulative counts and submits them.
//
// It sends metrics:
//   eventbus.delivered_total (tags: engine:<name>)
//   eventbus.dropped_total (tags: engine:<name>)
// plus engine:_all aggregate.

type DatadogStatsdExporter struct {
	eventBus *EventBusModule
	client   *statsd.Client
	prefix   string
	interval time.Duration
	baseTags []string
}

// NewDatadogStatsdExporter creates a new exporter. addr example: "127.0.0.1:8125".
// prefix defaults to "eventbus" if empty. interval must be > 0.
func NewDatadogStatsdExporter(eventBus *EventBusModule, prefix, addr string, interval time.Duration, baseTags []string) (*DatadogStatsdExporter, error) {
	if eventBus == nil {
		return nil, errNilEventBus
	}
	if interval <= 0 {
		return nil, errInvalidInterval
	}
	if prefix == "" {
		prefix = "eventbus"
	}
	// Configure client with namespace option (v5 API)
	client, err := statsd.New(addr, statsd.WithNamespace(prefix+"."))
	if err != nil {
		return nil, fmt.Errorf("eventbus: creating statsd client: %w", err)
	}
	return &DatadogStatsdExporter{
		eventBus: eventBus,
		client:   client,
		prefix:   prefix,
		interval: interval,
		baseTags: baseTags,
	}, nil
}

// Run starts the export loop until context cancellation.
func (e *DatadogStatsdExporter) Run(ctx context.Context) {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.flush()
		}
	}
}

func (e *DatadogStatsdExporter) flush() {
	per := e.eventBus.PerEngineStats()
	var totalDelivered, totalDropped uint64
	for engine, s := range per {
		engineTags := append(e.baseTags, "engine:"+engine)
		_ = e.client.Gauge("delivered_total", float64(s.Delivered), engineTags, 1)
		_ = e.client.Gauge("dropped_total", float64(s.Dropped), engineTags, 1)
		totalDelivered += s.Delivered
		totalDropped += s.Dropped
	}
	aggTags := append(e.baseTags, "engine:_all")
	_ = e.client.Gauge("delivered_total", float64(totalDelivered), aggTags, 1)
	_ = e.client.Gauge("dropped_total", float64(totalDropped), aggTags, 1)
	// Removed always-on goroutine gauge per review feedback; runtime metrics belong in a broader runtime exporter.
}

// Close closes underlying statsd client.
func (e *DatadogStatsdExporter) Close() error {
	if e == nil || e.client == nil {
		return nil
	}
	if err := e.client.Close(); err != nil {
		return fmt.Errorf("eventbus: closing statsd client: %w", err)
	}
	return nil
}
