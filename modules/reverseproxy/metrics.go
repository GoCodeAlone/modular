// Package reverseproxy provides metrics collection for the reverse proxy module.
package reverseproxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// MetricsCollector collects and provides metrics for the reverse proxy.
type MetricsCollector struct {
	mu                 sync.RWMutex
	requestCounts      map[string]int
	requestLatency     map[string]time.Duration
	errorCounts        map[string]int
	statusCodeCounts   map[string]map[int]int
	circuitStatus      map[string]string // circuit state (closed/open/half-open)
	latencyPercentiles map[string]map[string]time.Duration
	latencySamples     map[string][]time.Duration
	metadata           map[string]map[string]map[string]int // backend -> key -> value -> count
	startTime          time.Time
}

// NewMetricsCollector creates a new MetricsCollector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		requestCounts:      make(map[string]int),
		requestLatency:     make(map[string]time.Duration),
		errorCounts:        make(map[string]int),
		statusCodeCounts:   make(map[string]map[int]int),
		circuitStatus:      make(map[string]string),
		latencyPercentiles: make(map[string]map[string]time.Duration),
		latencySamples:     make(map[string][]time.Duration),
		metadata:           make(map[string]map[string]map[string]int),
		startTime:          time.Now(),
	}
}

// RecordRequest records a request to a backend.
func (m *MetricsCollector) RecordRequest(backend string, start time.Time, statusCode int, err error, metadata ...map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record request count
	m.requestCounts[backend]++

	// Record latency
	latency := time.Since(start)
	m.requestLatency[backend] = latency

	// Store latency sample for percentile calculation
	if _, exists := m.latencySamples[backend]; !exists {
		m.latencySamples[backend] = make([]time.Duration, 0, 100)
	}
	if len(m.latencySamples[backend]) >= 100 {
		// Simple sliding window implementation - remove oldest sample
		m.latencySamples[backend] = m.latencySamples[backend][1:]
	}
	m.latencySamples[backend] = append(m.latencySamples[backend], latency)

	// Calculate percentiles when we have enough samples
	if len(m.latencySamples[backend]) >= 10 {
		m.updateLatencyPercentiles(backend)
	}

	// Record errors
	if err != nil {
		m.errorCounts[backend]++
	}

	// Record status codes
	if statusCode > 0 {
		if _, exists := m.statusCodeCounts[backend]; !exists {
			m.statusCodeCounts[backend] = make(map[int]int)
		}
		m.statusCodeCounts[backend][statusCode]++
	}

	// Record additional metadata if provided
	if len(metadata) > 0 && metadata[0] != nil {
		if _, exists := m.metadata[backend]; !exists {
			m.metadata[backend] = make(map[string]map[string]int)
		}

		for key, value := range metadata[0] {
			if _, exists := m.metadata[backend][key]; !exists {
				m.metadata[backend][key] = make(map[string]int)
			}
			m.metadata[backend][key][value]++
		}
	}
}

// SetCircuitBreakerStatus sets the status of a circuit breaker.
func (m *MetricsCollector) SetCircuitBreakerStatus(backend string, isOpen bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status := "closed"
	if isOpen {
		status = "open"
	}

	m.circuitStatus[backend] = status
}

// SetCircuitBreakerStateString sets the status of a circuit breaker with an explicit state string.
func (m *MetricsCollector) SetCircuitBreakerStateString(backend string, state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.circuitStatus[backend] = state
}

// updateLatencyPercentiles calculates the latency percentiles for a backend.
func (m *MetricsCollector) updateLatencyPercentiles(backend string) {
	samples := m.latencySamples[backend]
	if len(samples) < 10 {
		return
	}

	// Simple bubble sort for small sample sets
	sortedSamples := make([]time.Duration, len(samples))
	copy(sortedSamples, samples)
	for i := 0; i < len(sortedSamples); i++ {
		for j := i + 1; j < len(sortedSamples); j++ {
			if sortedSamples[i] > sortedSamples[j] {
				sortedSamples[i], sortedSamples[j] = sortedSamples[j], sortedSamples[i]
			}
		}
	}

	// Calculate percentiles
	if _, exists := m.latencyPercentiles[backend]; !exists {
		m.latencyPercentiles[backend] = make(map[string]time.Duration)
	}

	percentiles := []struct {
		name  string
		point float64
	}{
		{"p50", 0.5},
		{"p90", 0.9},
		{"p95", 0.95},
		{"p99", 0.99},
	}

	for _, p := range percentiles {
		idx := int(float64(len(sortedSamples)-1) * p.point)
		m.latencyPercentiles[backend][p.name] = sortedSamples[idx]
	}
}

// GetMetrics returns the collected metrics.
func (m *MetricsCollector) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := map[string]interface{}{
		"uptime_seconds": time.Since(m.startTime).Seconds(),
		"backends":       make(map[string]interface{}),
	}

	backendMetrics := metrics["backends"].(map[string]interface{})

	// Collect metrics for each backend
	for backend := range m.requestCounts {
		backendMetrics[backend] = map[string]interface{}{
			"request_count": m.requestCounts[backend],
			"error_count":   m.errorCounts[backend],
			"error_rate":    float64(m.errorCounts[backend]) / float64(m.requestCounts[backend]),
			"latency_ms":    m.requestLatency[backend].Milliseconds(),
			"status_codes":  m.statusCodeCounts[backend],
		}

		// Add circuit breaker status if available
		if _, exists := m.circuitStatus[backend]; exists {
			backendMetrics[backend].(map[string]interface{})["circuit_status"] = m.circuitStatus[backend]
		}

		// Add percentiles if available
		if percentiles, exists := m.latencyPercentiles[backend]; exists {
			percentilesMap := make(map[string]int64)
			for name, duration := range percentiles {
				percentilesMap[name] = duration.Milliseconds()
			}
			backendMetrics[backend].(map[string]interface{})["latency_percentiles_ms"] = percentilesMap
		}

		// Add metadata if available
		if metadata, exists := m.metadata[backend]; exists {
			backendMetrics[backend].(map[string]interface{})["metadata"] = metadata
		}
	}

	return metrics
}

// MetricsHandler returns an HTTP handler for metrics endpoint.
func (m *MetricsCollector) MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := m.GetMetrics()

		w.Header().Set("Content-Type", "application/json")

		// Pretty-print metrics in development environments
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")

		if err := encoder.Encode(metrics); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error": "Failed to encode metrics"}`)
			return
		}
	}
}
