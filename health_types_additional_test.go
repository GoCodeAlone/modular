package modular

import (
	"testing"
	"time"
)

// TestHealthStatus_StringAndIsHealthy covers string mapping and IsHealthy helper.
func TestHealthStatus_StringAndIsHealthy(t *testing.T) {
	cases := []struct {
		status   HealthStatus
		expected string
		healthy  bool
	}{
		{HealthStatusHealthy, "healthy", true},
		{HealthStatusDegraded, "degraded", false},
		{HealthStatusUnhealthy, "unhealthy", false},
		{HealthStatusUnknown, "unknown", false},
		{HealthStatus(999), "unknown", false}, // default path
	}
	for i, c := range cases {
		if got := c.status.String(); got != c.expected {
			t.Fatalf("case %d expected %s got %s", i, c.expected, got)
		}
		if c.status.IsHealthy() != c.healthy {
			t.Fatalf("case %d healthy mismatch", i)
		}
	}
}

// TestAggregateHealthSnapshot_Getters covers IsHealthy/IsReady and GetUnhealthyComponents.
func TestAggregateHealthSnapshot_Getters(t *testing.T) {
	snap := &AggregateHealthSnapshot{
		OverallStatus:   HealthStatusHealthy,
		ReadinessStatus: HealthStatusDegraded, // degraded still counts as ready
		Components: map[string]HealthResult{
			"db":      {Status: HealthStatusHealthy},
			"cache":   {Status: HealthStatusDegraded},
			"queue":   {Status: HealthStatusUnhealthy},
			"metrics": {Status: HealthStatusHealthy},
		},
		Summary: HealthSummary{HealthyCount: 2, TotalCount: 4, DegradedCount: 1, UnhealthyCount: 1},
	}
	if !snap.IsHealthy() {
		t.Fatalf("expected IsHealthy true")
	}
	if !snap.IsReady() {
		t.Fatalf("expected IsReady true (degraded readiness)")
	}
	unhealthy := snap.GetUnhealthyComponents()
	// Should include degraded and unhealthy components (anything not healthy)
	if len(unhealthy) != 2 {
		t.Fatalf("expected 2 unhealthy components, got %d: %v", len(unhealthy), unhealthy)
	}
}

// TestHealthTrigger_StringAndParse ensures enumeration mapping and parse errors.
func TestHealthTrigger_StringAndParse(t *testing.T) {
	triggers := []struct {
		trig HealthTrigger
		str  string
	}{
		{HealthTriggerThreshold, "threshold"},
		{HealthTriggerScheduled, "scheduled"},
		{HealthTriggerOnDemand, "on_demand"},
		{HealthTriggerStartup, "startup"},
		{HealthTriggerPostReload, "post_reload"},
		{HealthTrigger(42), "unknown"},
	}
	for i, tt := range triggers {
		if got := tt.trig.String(); got != tt.str {
			t.Fatalf("case %d expected %s got %s", i, tt.str, got)
		}
	}
	// Parse happy paths
	roundTrips := []string{"threshold", "scheduled", "on_demand", "startup", "post_reload"}
	for _, s := range roundTrips {
		trig, err := ParseHealthTrigger(s)
		if err != nil {
			t.Fatalf("unexpected error parsing %s: %v", s, err)
		}
		if trig.String() != s {
			t.Fatalf("round trip mismatch for %s", s)
		}
	}
	if _, err := ParseHealthTrigger("nope"); err == nil {
		t.Fatalf("expected error for invalid trigger")
	}
}

// TestHealthEvaluatedEvent_StructuredFields covers event field population including metrics and status change.
func TestHealthEvaluatedEvent_StructuredFields(t *testing.T) {
	metrics := &HealthEvaluationMetrics{ComponentsEvaluated: 5, FailedEvaluations: 1, AverageResponseTimeMs: 12.5}
	event := &HealthEvaluatedEvent{
		EvaluationID:   "abc123",
		Timestamp:      time.Now(),
		Snapshot:       AggregateHealthSnapshot{OverallStatus: HealthStatusDegraded, Summary: HealthSummary{HealthyCount: 3, TotalCount: 5, DegradedCount: 1, UnhealthyCount: 1}},
		Duration:       25 * time.Millisecond,
		TriggerType:    HealthTriggerOnDemand,
		StatusChanged:  true,
		PreviousStatus: HealthStatusHealthy,
		Metrics:        metrics,
	}
	fields := event.StructuredFields()
	// Basic assertions
	expectedKeys := []string{"module", "phase", "event", "evaluation_id", "duration_ms", "trigger_type", "overall_status", "healthy_count", "total_count", "status_changed", "previous_status", "degraded_count", "unhealthy_count", "components_evaluated", "failed_evaluations", "average_response_time_ms"}
	for _, k := range expectedKeys {
		if _, ok := fields[k]; !ok {
			t.Fatalf("missing key %s in structured fields", k)
		}
	}
	if fields["overall_status"] != "degraded" {
		t.Fatalf("unexpected overall_status: %v", fields["overall_status"])
	}
	if fields["previous_status"] != "healthy" {
		t.Fatalf("unexpected previous_status: %v", fields["previous_status"])
	}
}

// TestHealthEvaluationMetrics_Methods covers metrics helper functions edge cases.
func TestHealthEvaluationMetrics_Methods(t *testing.T) {
	m := &HealthEvaluationMetrics{ComponentsEvaluated: 4, ComponentsSkipped: 1, ComponentsTimedOut: 1}
	if eff := m.CalculateEfficiency(); eff <= 0 || eff >= 1 {
		t.Fatalf("unexpected efficiency %f", eff)
	}
	if m.HasPerformanceBottleneck() {
		t.Fatalf("no bottleneck expected with zero timing")
	}
	if bp := m.BottleneckPercentage(); bp != 0 {
		t.Fatalf("expected 0 bottleneck percentage, got %f", bp)
	}
	m.TotalEvaluationTime = 100 * time.Millisecond
	m.SlowestComponentTime = 60 * time.Millisecond
	if !m.HasPerformanceBottleneck() {
		t.Fatalf("expected bottleneck detection")
	}
	if bp := m.BottleneckPercentage(); bp < 59 || bp > 61 {
		t.Fatalf("expected ~60%%, got %f", bp)
	}
}

// TestHealthEventFilters exercises filter helper functions.
func TestHealthEventFilters(t *testing.T) {
	events := []ObserverEvent{
		&HealthEvaluatedEvent{EvaluationID: "1", Snapshot: AggregateHealthSnapshot{OverallStatus: HealthStatusHealthy}, TriggerType: HealthTriggerStartup, StatusChanged: false},
		&HealthEvaluatedEvent{EvaluationID: "2", Snapshot: AggregateHealthSnapshot{OverallStatus: HealthStatusUnhealthy}, TriggerType: HealthTriggerOnDemand, StatusChanged: true, PreviousStatus: HealthStatusHealthy},
		&HealthEvaluatedEvent{EvaluationID: "3", Snapshot: AggregateHealthSnapshot{OverallStatus: HealthStatusDegraded}, TriggerType: HealthTriggerOnDemand, StatusChanged: false},
	}
	changed := FilterHealthEventsByStatusChange(events, true)
	if len(changed) != 1 {
		t.Fatalf("expected 1 changed event, got %d", len(changed))
	}
	onDemand := FilterHealthEventsByTrigger(events, HealthTriggerOnDemand)
	if len(onDemand) != 2 {
		t.Fatalf("expected 2 on_demand events, got %d", len(onDemand))
	}
	unhealthy := FilterHealthEventsByStatus(events, HealthStatusUnhealthy)
	if len(unhealthy) != 1 {
		t.Fatalf("expected 1 unhealthy event, got %d", len(unhealthy))
	}
}
