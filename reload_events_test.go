package modular

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigReloadStartedEvent(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_config_reload_started_event_type",
			testFunc: func(t *testing.T) {
				// Test that ConfigReloadStartedEvent type exists
				var event ConfigReloadStartedEvent
				assert.NotNil(t, event, "ConfigReloadStartedEvent type should be defined")
			},
		},
		{
			name: "should_have_required_event_fields",
			testFunc: func(t *testing.T) {
				// Test that ConfigReloadStartedEvent has required fields
				event := ConfigReloadStartedEvent{
					ReloadID:    "reload-123",
					Timestamp:   time.Now(),
					TriggerType: ReloadTriggerManual,
					ConfigDiff:  &ConfigDiff{},
				}
				assert.Equal(t, "reload-123", event.ReloadID, "Event should have ReloadID field")
				assert.NotNil(t, event.Timestamp, "Event should have Timestamp field")
				assert.Equal(t, ReloadTriggerManual, event.TriggerType, "Event should have TriggerType field")
				assert.NotNil(t, event.ConfigDiff, "Event should have ConfigDiff field")
			},
		},
		{
			name: "should_implement_observer_event_interface",
			testFunc: func(t *testing.T) {
				// Test that ConfigReloadStartedEvent implements ObserverEvent interface
				event := ConfigReloadStartedEvent{
					ReloadID:  "reload-123",
					Timestamp: time.Now(),
				}
				var observerEvent ObserverEvent = &event
				assert.NotNil(t, observerEvent, "ConfigReloadStartedEvent should implement ObserverEvent")
			},
		},
		{
			name: "should_provide_event_type_method",
			testFunc: func(t *testing.T) {
				// Test that event provides correct type
				event := ConfigReloadStartedEvent{}
				eventType := event.EventType()
				assert.Equal(t, "config.reload.started", eventType, "Event should return correct type")
			},
		},
		{
			name: "should_provide_event_source_method",
			testFunc: func(t *testing.T) {
				// Test that event provides correct source
				event := ConfigReloadStartedEvent{}
				source := event.EventSource()
				assert.Equal(t, "modular.core", source, "Event should return correct source")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestConfigReloadCompletedEvent(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_config_reload_completed_event_type",
			testFunc: func(t *testing.T) {
				// Test that ConfigReloadCompletedEvent type exists
				var event ConfigReloadCompletedEvent
				assert.NotNil(t, event, "ConfigReloadCompletedEvent type should be defined")
			},
		},
		{
			name: "should_have_required_event_fields",
			testFunc: func(t *testing.T) {
				// Test that ConfigReloadCompletedEvent has required fields
				event := ConfigReloadCompletedEvent{
					ReloadID:     "reload-123",
					Timestamp:    time.Now(),
					Success:      true,
					Duration:     50 * time.Millisecond,
					AffectedModules: []string{"database", "httpserver"},
				}
				assert.Equal(t, "reload-123", event.ReloadID, "Event should have ReloadID field")
				assert.NotNil(t, event.Timestamp, "Event should have Timestamp field")
				assert.True(t, event.Success, "Event should have Success field")
				assert.Equal(t, 50*time.Millisecond, event.Duration, "Event should have Duration field")
				assert.Len(t, event.AffectedModules, 2, "Event should have AffectedModules field")
			},
		},
		{
			name: "should_handle_failed_reload_events",
			testFunc: func(t *testing.T) {
				// Test that ConfigReloadCompletedEvent can represent failed reloads
				event := ConfigReloadCompletedEvent{
					ReloadID:  "reload-456",
					Timestamp: time.Now(),
					Success:   false,
					Error:     "validation failed: invalid port number",
					Duration:  25 * time.Millisecond,
				}
				assert.False(t, event.Success, "Event should support failed reloads")
				assert.Contains(t, event.Error, "validation failed", "Event should include error message")
			},
		},
		{
			name: "should_implement_observer_event_interface",
			testFunc: func(t *testing.T) {
				// Test that ConfigReloadCompletedEvent implements ObserverEvent interface
				event := ConfigReloadCompletedEvent{
					ReloadID:  "reload-123",
					Timestamp: time.Now(),
					Success:   true,
				}
				var observerEvent ObserverEvent = &event
				assert.NotNil(t, observerEvent, "ConfigReloadCompletedEvent should implement ObserverEvent")
			},
		},
		{
			name: "should_provide_event_type_method",
			testFunc: func(t *testing.T) {
				// Test that event provides correct type
				event := ConfigReloadCompletedEvent{}
				eventType := event.EventType()
				assert.Equal(t, "config.reload.completed", eventType, "Event should return correct type")
			},
		},
		{
			name: "should_provide_event_source_method",
			testFunc: func(t *testing.T) {
				// Test that event provides correct source
				event := ConfigReloadCompletedEvent{}
				source := event.EventSource()
				assert.Equal(t, "modular.core", source, "Event should return correct source")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestReloadTriggerType(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_reload_trigger_constants",
			testFunc: func(t *testing.T) {
				// Test that ReloadTrigger constants are defined
				assert.Equal(t, "manual", ReloadTriggerManual.String(), "ReloadTriggerManual should be 'manual'")
				assert.Equal(t, "file_change", ReloadTriggerFileChange.String(), "ReloadTriggerFileChange should be 'file_change'")
				assert.Equal(t, "api_request", ReloadTriggerAPIRequest.String(), "ReloadTriggerAPIRequest should be 'api_request'")
				assert.Equal(t, "scheduled", ReloadTriggerScheduled.String(), "ReloadTriggerScheduled should be 'scheduled'")
			},
		},
		{
			name: "should_support_string_conversion",
			testFunc: func(t *testing.T) {
				// Test that ReloadTrigger can be converted to string
				trigger := ReloadTriggerManual
				str := trigger.String()
				assert.Equal(t, "manual", str, "ReloadTrigger should convert to string")
			},
		},
		{
			name: "should_parse_from_string",
			testFunc: func(t *testing.T) {
				// Test that ReloadTrigger can be parsed from string
				trigger, err := ParseReloadTrigger("manual")
				assert.NoError(t, err, "Should parse valid trigger")
				assert.Equal(t, ReloadTriggerManual, trigger, "Should parse manual correctly")

				_, err = ParseReloadTrigger("invalid")
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

func TestReloadEventEmission(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T)
	}{
		{
			name:        "should_emit_reload_started_event_when_reload_begins",
			description: "System should emit ConfigReloadStartedEvent when a configuration reload begins",
			testFunc: func(t *testing.T) {
				// Create a mock event observer
				observer := &mockEventObserver{}
				
				// Create reload orchestrator (mock)
				orchestrator := &mockReloadOrchestrator{
					observer: observer,
				}

				// Trigger a reload
				reloadID := "test-reload-001"
				configDiff := &ConfigDiff{
					Changed: map[string]ConfigFieldChange{
						"database.host": {
							FieldPath: "database.host",
							OldValue:  "localhost",
							NewValue:  "db.example.com",
						},
					},
				}

				err := orchestrator.StartReload(context.Background(), reloadID, configDiff, ReloadTriggerManual)
				assert.NoError(t, err, "StartReload should succeed")

				// Verify that ConfigReloadStartedEvent was emitted
				require.Len(t, observer.events, 1, "Should emit exactly one event")
				event, ok := observer.events[0].(*ConfigReloadStartedEvent)
				require.True(t, ok, "Event should be ConfigReloadStartedEvent")
				assert.Equal(t, reloadID, event.ReloadID, "Event should have correct reload ID")
				assert.Equal(t, ReloadTriggerManual, event.TriggerType, "Event should have correct trigger type")
				assert.NotNil(t, event.ConfigDiff, "Event should include config diff")
			},
		},
		{
			name:        "should_emit_reload_completed_event_when_reload_finishes",
			description: "System should emit ConfigReloadCompletedEvent when a configuration reload completes",
			testFunc: func(t *testing.T) {
				// Create a mock event observer
				observer := &mockEventObserver{}
				
				// Create reload orchestrator (mock)
				orchestrator := &mockReloadOrchestrator{
					observer: observer,
				}

				// Complete a reload
				reloadID := "test-reload-002"
				affectedModules := []string{"database", "httpserver"}
				duration := 75 * time.Millisecond

				err := orchestrator.CompleteReload(context.Background(), reloadID, true, duration, affectedModules, "")
				assert.NoError(t, err, "CompleteReload should succeed")

				// Verify that ConfigReloadCompletedEvent was emitted
				require.Len(t, observer.events, 1, "Should emit exactly one event")
				event, ok := observer.events[0].(*ConfigReloadCompletedEvent)
				require.True(t, ok, "Event should be ConfigReloadCompletedEvent")
				assert.Equal(t, reloadID, event.ReloadID, "Event should have correct reload ID")
				assert.True(t, event.Success, "Event should indicate success")
				assert.Equal(t, duration, event.Duration, "Event should have correct duration")
				assert.Equal(t, affectedModules, event.AffectedModules, "Event should list affected modules")
			},
		},
		{
			name:        "should_emit_reload_completed_event_with_error_on_failure",
			description: "System should emit ConfigReloadCompletedEvent with error details when reload fails",
			testFunc: func(t *testing.T) {
				// Create a mock event observer
				observer := &mockEventObserver{}
				
				// Create reload orchestrator (mock)
				orchestrator := &mockReloadOrchestrator{
					observer: observer,
				}

				// Complete a failed reload
				reloadID := "test-reload-003"
				errorMsg := "database: connection timeout during reload"
				duration := 30 * time.Millisecond

				err := orchestrator.CompleteReload(context.Background(), reloadID, false, duration, nil, errorMsg)
				assert.NoError(t, err, "CompleteReload should succeed even for failed reload")

				// Verify that ConfigReloadCompletedEvent was emitted with error
				require.Len(t, observer.events, 1, "Should emit exactly one event")
				event, ok := observer.events[0].(*ConfigReloadCompletedEvent)
				require.True(t, ok, "Event should be ConfigReloadCompletedEvent")
				assert.Equal(t, reloadID, event.ReloadID, "Event should have correct reload ID")
				assert.False(t, event.Success, "Event should indicate failure")
				assert.Equal(t, errorMsg, event.Error, "Event should include error message")
				assert.Equal(t, duration, event.Duration, "Event should have correct duration")
			},
		},
		{
			name:        "should_include_structured_logging_fields",
			description: "Reload events should include structured logging fields for observability",
			testFunc: func(t *testing.T) {
				startedEvent := ConfigReloadStartedEvent{
					ReloadID:    "reload-456",
					TriggerType: ReloadTriggerFileChange,
					Timestamp:   time.Now(),
				}

				fields := startedEvent.StructuredFields()
				assert.Contains(t, fields, "module", "Should include module field")
				assert.Contains(t, fields, "phase", "Should include phase field")
				assert.Contains(t, fields, "event", "Should include event field")
				assert.Contains(t, fields, "reload_id", "Should include reload_id field")
				assert.Contains(t, fields, "trigger_type", "Should include trigger_type field")

				completedEvent := ConfigReloadCompletedEvent{
					ReloadID: "reload-789",
					Success:  true,
					Duration: 100 * time.Millisecond,
				}

				completedFields := completedEvent.StructuredFields()
				assert.Contains(t, completedFields, "success", "Should include success field")
				assert.Contains(t, completedFields, "duration_ms", "Should include duration field")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestReloadEventCorrelation(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_correlate_started_and_completed_events_by_reload_id",
			testFunc: func(t *testing.T) {
				reloadID := "correlation-test-001"

				startedEvent := ConfigReloadStartedEvent{
					ReloadID:    reloadID,
					TriggerType: ReloadTriggerAPIRequest,
					Timestamp:   time.Now(),
				}

				completedEvent := ConfigReloadCompletedEvent{
					ReloadID:  reloadID,
					Success:   true,
					Duration:  120 * time.Millisecond,
					Timestamp: time.Now().Add(120 * time.Millisecond),
				}

				// Events should have matching correlation ID
				assert.Equal(t, startedEvent.ReloadID, completedEvent.ReloadID, "Events should have matching reload ID")
				assert.True(t, completedEvent.Timestamp.After(startedEvent.Timestamp), "Completed event should be after started event")
			},
		},
		{
			name: "should_support_event_filtering_by_reload_id",
			testFunc: func(t *testing.T) {
				// Test event filtering capabilities
				events := []ObserverEvent{
					&ConfigReloadStartedEvent{ReloadID: "reload-001"},
					&ConfigReloadCompletedEvent{ReloadID: "reload-001"},
					&ConfigReloadStartedEvent{ReloadID: "reload-002"},
					&ConfigReloadCompletedEvent{ReloadID: "reload-002"},
				}

				// Filter events for specific reload
				filteredEvents := FilterEventsByReloadID(events, "reload-001")
				assert.Len(t, filteredEvents, 2, "Should filter events by reload ID")

				// Verify both started and completed events are present
				hasStarted := false
				hasCompleted := false
				for _, event := range filteredEvents {
					switch e := event.(type) {
					case *ConfigReloadStartedEvent:
						if e.ReloadID == "reload-001" {
							hasStarted = true
						}
					case *ConfigReloadCompletedEvent:
						if e.ReloadID == "reload-001" {
							hasCompleted = true
						}
					}
				}
				assert.True(t, hasStarted, "Should include started event")
				assert.True(t, hasCompleted, "Should include completed event")
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
type mockEventObserver struct {
	events []ObserverEvent
}

func (m *mockEventObserver) OnEvent(ctx context.Context, event ObserverEvent) error {
	m.events = append(m.events, event)
	return nil
}

type mockReloadOrchestrator struct {
	observer *mockEventObserver
}

func (m *mockReloadOrchestrator) StartReload(ctx context.Context, reloadID string, diff *ConfigDiff, trigger ReloadTrigger) error {
	event := &ConfigReloadStartedEvent{
		ReloadID:    reloadID,
		TriggerType: trigger,
		ConfigDiff:  diff,
		Timestamp:   time.Now(),
	}
	return m.observer.OnEvent(ctx, event)
}

func (m *mockReloadOrchestrator) CompleteReload(ctx context.Context, reloadID string, success bool, duration time.Duration, affectedModules []string, errorMsg string) error {
	event := &ConfigReloadCompletedEvent{
		ReloadID:        reloadID,
		Success:         success,
		Duration:        duration,
		AffectedModules: affectedModules,
		Error:           errorMsg,
		Timestamp:       time.Now(),
	}
	return m.observer.OnEvent(ctx, event)
}