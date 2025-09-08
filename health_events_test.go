//go:build failing_test

package modular

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthEvaluatedEvent(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_health_evaluated_event_type",
			testFunc: func(t *testing.T) {
				// Test that HealthEvaluatedEvent type exists
				var event HealthEvaluatedEvent
				assert.NotNil(t, event, "HealthEvaluatedEvent type should be defined")
			},
		},
		{
			name: "should_have_required_event_fields",
			testFunc: func(t *testing.T) {
				// Test that HealthEvaluatedEvent has required fields
				snapshot := AggregateHealthSnapshot{
					OverallStatus: HealthStatusHealthy,
					Components: map[string]HealthResult{
						"database": {Status: HealthStatusHealthy, Message: "Connected"},
					},
					Summary:   HealthSummary{HealthyCount: 1, TotalCount: 1},
					Timestamp: time.Now(),
				}

				event := HealthEvaluatedEvent{
					EvaluationID: "health-eval-123",
					Timestamp:    time.Now(),
					Snapshot:     snapshot,
					Duration:     25 * time.Millisecond,
					TriggerType:  HealthTriggerScheduled,
				}

				assert.Equal(t, "health-eval-123", event.EvaluationID, "Event should have EvaluationID field")
				assert.NotNil(t, event.Timestamp, "Event should have Timestamp field")
				assert.Equal(t, snapshot, event.Snapshot, "Event should have Snapshot field")
				assert.Equal(t, 25*time.Millisecond, event.Duration, "Event should have Duration field")
				assert.Equal(t, HealthTriggerScheduled, event.TriggerType, "Event should have TriggerType field")
			},
		},
		{
			name: "should_implement_observer_event_interface",
			testFunc: func(t *testing.T) {
				// Test that HealthEvaluatedEvent implements ObserverEvent interface
				event := HealthEvaluatedEvent{
					EvaluationID: "health-eval-123",
					Timestamp:    time.Now(),
				}
				var observerEvent ObserverEvent = &event
				assert.NotNil(t, observerEvent, "HealthEvaluatedEvent should implement ObserverEvent")
			},
		},
		{
			name: "should_provide_event_type_method",
			testFunc: func(t *testing.T) {
				// Test that event provides correct type
				event := HealthEvaluatedEvent{}
				eventType := event.EventType()
				assert.Equal(t, "health.evaluated", eventType, "Event should return correct type")
			},
		},
		{
			name: "should_provide_event_source_method",
			testFunc: func(t *testing.T) {
				// Test that event provides correct source
				event := HealthEvaluatedEvent{}
				source := event.EventSource()
				assert.Equal(t, "modular.core.health", source, "Event should return correct source")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestHealthTriggerType(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_health_trigger_constants",
			testFunc: func(t *testing.T) {
				// Test that HealthTrigger constants are defined
				assert.Equal(t, "scheduled", string(HealthTriggerScheduled), "HealthTriggerScheduled should be 'scheduled'")
				assert.Equal(t, "on_demand", string(HealthTriggerOnDemand), "HealthTriggerOnDemand should be 'on_demand'")
				assert.Equal(t, "threshold", string(HealthTriggerThreshold), "HealthTriggerThreshold should be 'threshold'")
				assert.Equal(t, "startup", string(HealthTriggerStartup), "HealthTriggerStartup should be 'startup'")
				assert.Equal(t, "post_reload", string(HealthTriggerPostReload), "HealthTriggerPostReload should be 'post_reload'")
			},
		},
		{
			name: "should_support_string_conversion",
			testFunc: func(t *testing.T) {
				// Test that HealthTrigger can be converted to string
				trigger := HealthTriggerScheduled
				str := trigger.String()
				assert.Equal(t, "scheduled", str, "HealthTrigger should convert to string")
			},
		},
		{
			name: "should_parse_from_string",
			testFunc: func(t *testing.T) {
				// Test that HealthTrigger can be parsed from string
				trigger, err := ParseHealthTrigger("scheduled")
				assert.NoError(t, err, "Should parse valid trigger")
				assert.Equal(t, HealthTriggerScheduled, trigger, "Should parse scheduled correctly")

				trigger, err = ParseHealthTrigger("on_demand")
				assert.NoError(t, err, "Should parse on_demand correctly")
				assert.Equal(t, HealthTriggerOnDemand, trigger)

				_, err = ParseHealthTrigger("invalid")
				assert.Error(t, err, "Should return error for invalid trigger")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestHealthEvaluatedEventEmission(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T)
	}{
		{
			name:        "should_emit_health_evaluated_event_after_health_check",
			description: "System should emit HealthEvaluatedEvent after completing a health evaluation",
			testFunc: func(t *testing.T) {
				// Create a mock event observer
				observer := &mockHealthEventObserver{}
				
				// Create health aggregation service (mock)
				healthService := &mockAggregateHealthService{
					observer: observer,
				}

				// Perform health evaluation
				evaluationID := "health-eval-001"
				ctx := context.Background()

				snapshot, err := healthService.EvaluateHealth(ctx, evaluationID, HealthTriggerScheduled)
				assert.NoError(t, err, "EvaluateHealth should succeed")
				assert.NotNil(t, snapshot, "Should return health snapshot")

				// Verify that HealthEvaluatedEvent was emitted
				require.Len(t, observer.events, 1, "Should emit exactly one event")
				event, ok := observer.events[0].(*HealthEvaluatedEvent)
				require.True(t, ok, "Event should be HealthEvaluatedEvent")
				assert.Equal(t, evaluationID, event.EvaluationID, "Event should have correct evaluation ID")
				assert.Equal(t, HealthTriggerScheduled, event.TriggerType, "Event should have correct trigger type")
				assert.NotNil(t, event.Snapshot, "Event should include health snapshot")
			},
		},
		{
			name:        "should_emit_health_evaluated_event_on_status_change",
			description: "System should emit HealthEvaluatedEvent when overall health status changes",
			testFunc: func(t *testing.T) {
				// Create a mock event observer
				observer := &mockHealthEventObserver{}
				
				// Create health aggregation service (mock)
				healthService := &mockAggregateHealthService{
					observer: observer,
					previousStatus: HealthStatusHealthy,
				}

				// Perform health evaluation that results in status change
				ctx := context.Background()
				
				// Simulate status change from healthy to degraded
				snapshot, err := healthService.EvaluateHealthWithStatusChange(ctx, "health-eval-002", HealthTriggerThreshold, HealthStatusDegraded)
				assert.NoError(t, err, "EvaluateHealth should succeed")
				assert.Equal(t, HealthStatusDegraded, snapshot.OverallStatus, "Status should change to degraded")

				// Verify that HealthEvaluatedEvent was emitted
				require.Len(t, observer.events, 1, "Should emit exactly one event")
				event, ok := observer.events[0].(*HealthEvaluatedEvent)
				require.True(t, ok, "Event should be HealthEvaluatedEvent")
				assert.Equal(t, HealthTriggerThreshold, event.TriggerType, "Event should indicate threshold trigger")
				assert.True(t, event.StatusChanged, "Event should indicate status changed")
				assert.Equal(t, HealthStatusHealthy, event.PreviousStatus, "Event should include previous status")
				assert.Equal(t, HealthStatusDegraded, event.Snapshot.OverallStatus, "Event should include new status")
			},
		},
		{
			name:        "should_emit_health_evaluated_event_with_performance_metrics",
			description: "HealthEvaluatedEvent should include performance metrics about the health evaluation",
			testFunc: func(t *testing.T) {
				// Create a mock event observer
				observer := &mockHealthEventObserver{}
				
				// Create health aggregation service (mock)
				healthService := &mockAggregateHealthService{
					observer: observer,
					simulatedDuration: 150 * time.Millisecond,
				}

				// Perform health evaluation
				ctx := context.Background()
				
				start := time.Now()
				_, err := healthService.EvaluateHealth(ctx, "health-eval-003", HealthTriggerOnDemand)
				duration := time.Since(start)
				assert.NoError(t, err, "EvaluateHealth should succeed")

				// Verify that event includes performance metrics
				require.Len(t, observer.events, 1, "Should emit exactly one event")
				event, ok := observer.events[0].(*HealthEvaluatedEvent)
				require.True(t, ok, "Event should be HealthEvaluatedEvent")
				
				assert.Greater(t, event.Duration, time.Duration(0), "Event should include duration")
				assert.GreaterOrEqual(t, event.Duration, 100*time.Millisecond, "Duration should reflect actual execution time")
				assert.NotNil(t, event.Metrics, "Event should include metrics")
				assert.Greater(t, event.Metrics.ComponentsEvaluated, 0, "Should report components evaluated")
			},
		},
		{
			name:        "should_include_structured_logging_fields",
			description: "HealthEvaluatedEvent should include structured logging fields for observability",
			testFunc: func(t *testing.T) {
				event := HealthEvaluatedEvent{
					EvaluationID: "health-eval-456",
					TriggerType:  HealthTriggerPostReload,
					Duration:     75 * time.Millisecond,
					Snapshot: AggregateHealthSnapshot{
						OverallStatus: HealthStatusDegraded,
						Summary: HealthSummary{
							HealthyCount:   2,
							DegradedCount:  1,
							UnhealthyCount: 0,
							TotalCount:     3,
						},
					},
					StatusChanged:  true,
					PreviousStatus: HealthStatusHealthy,
				}

				fields := event.StructuredFields()
				assert.Contains(t, fields, "module", "Should include module field")
				assert.Contains(t, fields, "phase", "Should include phase field")
				assert.Contains(t, fields, "event", "Should include event field")
				assert.Contains(t, fields, "evaluation_id", "Should include evaluation_id field")
				assert.Contains(t, fields, "trigger_type", "Should include trigger_type field")
				assert.Contains(t, fields, "overall_status", "Should include overall_status field")
				assert.Contains(t, fields, "duration_ms", "Should include duration field")
				assert.Contains(t, fields, "status_changed", "Should include status_changed field")
				assert.Contains(t, fields, "healthy_count", "Should include healthy_count field")
				assert.Contains(t, fields, "total_count", "Should include total_count field")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestHealthEventFiltering(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_filter_events_by_status_change",
			testFunc: func(t *testing.T) {
				events := []ObserverEvent{
					&HealthEvaluatedEvent{EvaluationID: "eval-001", StatusChanged: false},
					&HealthEvaluatedEvent{EvaluationID: "eval-002", StatusChanged: true},
					&HealthEvaluatedEvent{EvaluationID: "eval-003", StatusChanged: false},
					&HealthEvaluatedEvent{EvaluationID: "eval-004", StatusChanged: true},
				}

				statusChangeEvents := FilterHealthEventsByStatusChange(events, true)
				assert.Len(t, statusChangeEvents, 2, "Should filter events by status change")

				for _, event := range statusChangeEvents {
					healthEvent := event.(*HealthEvaluatedEvent)
					assert.True(t, healthEvent.StatusChanged, "All filtered events should have status changes")
				}
			},
		},
		{
			name: "should_filter_events_by_trigger_type",
			testFunc: func(t *testing.T) {
				events := []ObserverEvent{
					&HealthEvaluatedEvent{EvaluationID: "eval-001", TriggerType: HealthTriggerScheduled},
					&HealthEvaluatedEvent{EvaluationID: "eval-002", TriggerType: HealthTriggerOnDemand},
					&HealthEvaluatedEvent{EvaluationID: "eval-003", TriggerType: HealthTriggerScheduled},
				}

				scheduledEvents := FilterHealthEventsByTrigger(events, HealthTriggerScheduled)
				assert.Len(t, scheduledEvents, 2, "Should filter events by trigger type")

				for _, event := range scheduledEvents {
					healthEvent := event.(*HealthEvaluatedEvent)
					assert.Equal(t, HealthTriggerScheduled, healthEvent.TriggerType, "All filtered events should have correct trigger")
				}
			},
		},
		{
			name: "should_filter_events_by_overall_status",
			testFunc: func(t *testing.T) {
				events := []ObserverEvent{
					&HealthEvaluatedEvent{
						EvaluationID: "eval-001",
						Snapshot:     AggregateHealthSnapshot{OverallStatus: HealthStatusHealthy},
					},
					&HealthEvaluatedEvent{
						EvaluationID: "eval-002",
						Snapshot:     AggregateHealthSnapshot{OverallStatus: HealthStatusDegraded},
					},
					&HealthEvaluatedEvent{
						EvaluationID: "eval-003",
						Snapshot:     AggregateHealthSnapshot{OverallStatus: HealthStatusUnhealthy},
					},
				}

				unhealthyEvents := FilterHealthEventsByStatus(events, HealthStatusUnhealthy)
				assert.Len(t, unhealthyEvents, 1, "Should filter events by overall status")

				healthEvent := unhealthyEvents[0].(*HealthEvaluatedEvent)
				assert.Equal(t, HealthStatusUnhealthy, healthEvent.Snapshot.OverallStatus, "Filtered event should have correct status")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestHealthEventMetrics(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_health_evaluation_metrics",
			testFunc: func(t *testing.T) {
				// Test that HealthEvaluationMetrics type exists
				metrics := HealthEvaluationMetrics{
					ComponentsEvaluated:  5,
					ComponentsSkipped:    1,
					ComponentsTimedOut:   0,
					TotalEvaluationTime:  150 * time.Millisecond,
					SlowestComponentName: "database",
					SlowestComponentTime: 75 * time.Millisecond,
				}

				assert.Equal(t, 5, metrics.ComponentsEvaluated, "Should track components evaluated")
				assert.Equal(t, 1, metrics.ComponentsSkipped, "Should track components skipped")
				assert.Equal(t, 0, metrics.ComponentsTimedOut, "Should track components timed out")
				assert.Equal(t, 150*time.Millisecond, metrics.TotalEvaluationTime, "Should track total time")
				assert.Equal(t, "database", metrics.SlowestComponentName, "Should identify slowest component")
				assert.Equal(t, 75*time.Millisecond, metrics.SlowestComponentTime, "Should track slowest component time")
			},
		},
		{
			name: "should_calculate_health_evaluation_efficiency",
			testFunc: func(t *testing.T) {
				metrics := HealthEvaluationMetrics{
					ComponentsEvaluated: 8,
					ComponentsSkipped:   2,
					ComponentsTimedOut:  1,
				}

				efficiency := metrics.CalculateEfficiency()
				expectedEfficiency := float64(8) / float64(8+2+1) // 8/11 ≈ 0.727
				assert.InDelta(t, expectedEfficiency, efficiency, 0.01, "Should calculate efficiency correctly")
			},
		},
		{
			name: "should_identify_performance_bottlenecks",
			testFunc: func(t *testing.T) {
				metrics := HealthEvaluationMetrics{
					ComponentsEvaluated:  3,
					TotalEvaluationTime:  120 * time.Millisecond,
					SlowestComponentTime: 80 * time.Millisecond,
					SlowestComponentName: "external_api",
				}

				hasBottleneck := metrics.HasPerformanceBottleneck()
				assert.True(t, hasBottleneck, "Should identify performance bottleneck when one component takes >50% of total time")

				bottleneckPercentage := metrics.BottleneckPercentage()
				expectedPercentage := float64(80) / float64(120) * 100 // ~66.7%
				assert.InDelta(t, expectedPercentage, bottleneckPercentage, 0.1, "Should calculate bottleneck percentage correctly")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

// Mock implementations for testing
type mockHealthEventObserver struct {
	events []ObserverEvent
}

func (m *mockHealthEventObserver) OnEvent(ctx context.Context, event ObserverEvent) error {
	m.events = append(m.events, event)
	return nil
}

type mockAggregateHealthService struct {
	observer          *mockHealthEventObserver
	previousStatus    HealthStatus
	simulatedDuration time.Duration
}

func (m *mockAggregateHealthService) EvaluateHealth(ctx context.Context, evaluationID string, trigger HealthTrigger) (AggregateHealthSnapshot, error) {
	// Simulate health evaluation duration
	if m.simulatedDuration > 0 {
		time.Sleep(m.simulatedDuration)
	}

	snapshot := AggregateHealthSnapshot{
		OverallStatus: HealthStatusHealthy,
		Components: map[string]HealthResult{
			"database": {Status: HealthStatusHealthy, Message: "Connected"},
			"cache":    {Status: HealthStatusHealthy, Message: "Available"},
		},
		Summary: HealthSummary{
			HealthyCount: 2,
			TotalCount:   2,
		},
		Timestamp: time.Now(),
	}

	event := &HealthEvaluatedEvent{
		EvaluationID: evaluationID,
		TriggerType:  trigger,
		Snapshot:     snapshot,
		Duration:     m.simulatedDuration,
		Timestamp:    time.Now(),
		Metrics: &HealthEvaluationMetrics{
			ComponentsEvaluated: 2,
			TotalEvaluationTime: m.simulatedDuration,
		},
	}

	m.observer.OnEvent(ctx, event)
	return snapshot, nil
}

func (m *mockAggregateHealthService) EvaluateHealthWithStatusChange(ctx context.Context, evaluationID string, trigger HealthTrigger, newStatus HealthStatus) (AggregateHealthSnapshot, error) {
	snapshot := AggregateHealthSnapshot{
		OverallStatus: newStatus,
		Components: map[string]HealthResult{
			"database": {Status: HealthStatusHealthy, Message: "Connected"},
			"api":      {Status: HealthStatusDegraded, Message: "High latency"},
		},
		Summary: HealthSummary{
			HealthyCount:  1,
			DegradedCount: 1,
			TotalCount:    2,
		},
		Timestamp: time.Now(),
	}

	event := &HealthEvaluatedEvent{
		EvaluationID:   evaluationID,
		TriggerType:    trigger,
		Snapshot:       snapshot,
		Duration:       50 * time.Millisecond,
		StatusChanged:  true,
		PreviousStatus: m.previousStatus,
		Timestamp:      time.Now(),
	}

	m.observer.OnEvent(ctx, event)
	return snapshot, nil
}