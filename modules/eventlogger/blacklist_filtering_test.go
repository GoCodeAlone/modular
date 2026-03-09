package eventlogger

import (
	"context"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockOutputForTesting is a simple output target for testing
type mockOutputForTesting struct {
	events []*LogEntry
}

func (m *mockOutputForTesting) Start(ctx context.Context) error { return nil }
func (m *mockOutputForTesting) Stop(ctx context.Context) error  { return nil }
func (m *mockOutputForTesting) Flush() error                    { return nil }
func (m *mockOutputForTesting) WriteEvent(entry *LogEntry) error {
	m.events = append(m.events, entry)
	return nil
}

// TestEventLoggerModule_Blacklist tests blacklist filtering functionality
func TestEventLoggerModule_Blacklist(t *testing.T) {
	tests := []struct {
		name             string
		blacklist        []string
		eventType        string
		shouldBeFiltered bool
	}{
		{
			name:             "blacklisted event should be filtered",
			blacklist:        []string{"com.modular.eventlogger.event.received"},
			eventType:        "com.modular.eventlogger.event.received",
			shouldBeFiltered: true,
		},
		{
			name:             "non-blacklisted event should pass",
			blacklist:        []string{"com.modular.eventlogger.event.received"},
			eventType:        "user.created",
			shouldBeFiltered: false,
		},
		{
			name:             "empty blacklist allows all",
			blacklist:        []string{},
			eventType:        "any.event",
			shouldBeFiltered: false,
		},
		{
			name:             "multiple blacklist entries",
			blacklist:        []string{"event.one", "event.two", "event.three"},
			eventType:        "event.two",
			shouldBeFiltered: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &EventLoggerConfig{
				Enabled:            true,
				LogLevel:           "INFO",
				Format:             "structured",
				BufferSize:         10,
				FlushInterval:      5 * time.Second,
				EventTypeBlacklist: tt.blacklist,
				OutputTargets: []OutputTargetConfig{
					{
						Type:   "console",
						Level:  "INFO",
						Format: "structured",
						Console: &ConsoleTargetConfig{
							UseColor:   false,
							Timestamps: true,
						},
					},
				},
			}

			// Create mock output to track what gets logged
			mockOutput := &mockOutputForTesting{events: make([]*LogEntry, 0)}

			module := &EventLoggerModule{
				name:    ModuleName,
				config:  config,
				logger:  &testLogger{},
				outputs: []OutputTarget{mockOutput},
			}

			// Create a test event
			event := modular.NewCloudEvent(tt.eventType, "test-source", "test-data", nil)

			// Test shouldLogEvent
			shouldLog := module.shouldLogEvent(event)

			if tt.shouldBeFiltered {
				assert.False(t, shouldLog, "Event should be filtered by blacklist")
			} else {
				assert.True(t, shouldLog, "Event should not be filtered by blacklist")
			}
		})
	}
}

// TestEventLoggerModule_ExcludeOwnEvents tests the excludeOwnEvents flag
func TestEventLoggerModule_ExcludeOwnEvents(t *testing.T) {
	tests := []struct {
		name             string
		excludeOwnEvents bool
		eventType        string
		eventSource      string
		shouldBeFiltered bool
	}{
		{
			name:             "exclude own events enabled - eventlogger event filtered",
			excludeOwnEvents: true,
			eventType:        "com.modular.eventlogger.event.received",
			eventSource:      "eventlogger-module",
			shouldBeFiltered: true,
		},
		{
			name:             "exclude own events enabled - external event passes",
			excludeOwnEvents: true,
			eventType:        "user.created",
			eventSource:      "user-service",
			shouldBeFiltered: false,
		},
		{
			name:             "exclude own events disabled - eventlogger event passes",
			excludeOwnEvents: false,
			eventType:        "com.modular.eventlogger.event.received",
			eventSource:      "eventlogger-module",
			shouldBeFiltered: false,
		},
		{
			name:             "exclude own events enabled - eventlogger prefix filtered",
			excludeOwnEvents: true,
			eventType:        "com.modular.eventlogger.output.success",
			eventSource:      "other-source",
			shouldBeFiltered: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &EventLoggerConfig{
				Enabled:          true,
				LogLevel:         "INFO",
				Format:           "structured",
				BufferSize:       10,
				FlushInterval:    5 * time.Second,
				ExcludeOwnEvents: tt.excludeOwnEvents,
				OutputTargets: []OutputTargetConfig{
					{
						Type:   "console",
						Level:  "INFO",
						Format: "structured",
						Console: &ConsoleTargetConfig{
							UseColor:   false,
							Timestamps: true,
						},
					},
				},
			}

			mockOutput := &mockOutputForTesting{events: make([]*LogEntry, 0)}

			module := &EventLoggerModule{
				name:    ModuleName,
				config:  config,
				logger:  &testLogger{},
				outputs: []OutputTarget{mockOutput},
			}

			// Create a test event
			event := cloudevents.NewEvent()
			event.SetType(tt.eventType)
			event.SetSource(tt.eventSource)
			event.SetData(cloudevents.ApplicationJSON, "test-data")

			// Test shouldLogEvent
			shouldLog := module.shouldLogEvent(event)

			if tt.shouldBeFiltered {
				assert.False(t, shouldLog, "Event should be filtered by excludeOwnEvents")
			} else {
				assert.True(t, shouldLog, "Event should not be filtered by excludeOwnEvents")
			}
		})
	}
}

// TestEventLoggerModule_WhitelistAndBlacklist tests interaction between whitelist and blacklist
func TestEventLoggerModule_WhitelistAndBlacklist(t *testing.T) {
	tests := []struct {
		name             string
		whitelist        []string
		blacklist        []string
		eventType        string
		shouldBeFiltered bool
		reason           string
	}{
		{
			name:             "whitelist passes, blacklist blocks",
			whitelist:        []string{"user.created", "user.updated"},
			blacklist:        []string{"user.created"},
			eventType:        "user.created",
			shouldBeFiltered: true,
			reason:           "blacklist takes precedence over whitelist",
		},
		{
			name:             "whitelist passes, not in blacklist",
			whitelist:        []string{"user.created", "user.updated"},
			blacklist:        []string{"user.deleted"},
			eventType:        "user.created",
			shouldBeFiltered: false,
			reason:           "event in whitelist and not in blacklist",
		},
		{
			name:             "not in whitelist",
			whitelist:        []string{"user.created", "user.updated"},
			blacklist:        []string{"user.deleted"},
			eventType:        "order.created",
			shouldBeFiltered: true,
			reason:           "event not in whitelist",
		},
		{
			name:             "both empty allows all",
			whitelist:        []string{},
			blacklist:        []string{},
			eventType:        "any.event",
			shouldBeFiltered: false,
			reason:           "no filters configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &EventLoggerConfig{
				Enabled:            true,
				LogLevel:           "INFO",
				Format:             "structured",
				BufferSize:         10,
				FlushInterval:      5 * time.Second,
				EventTypeFilters:   tt.whitelist,
				EventTypeBlacklist: tt.blacklist,
				OutputTargets: []OutputTargetConfig{
					{
						Type:   "console",
						Level:  "INFO",
						Format: "structured",
						Console: &ConsoleTargetConfig{
							UseColor:   false,
							Timestamps: true,
						},
					},
				},
			}

			mockOutput := &mockOutputForTesting{events: make([]*LogEntry, 0)}

			module := &EventLoggerModule{
				name:    ModuleName,
				config:  config,
				logger:  &testLogger{},
				outputs: []OutputTarget{mockOutput},
			}

			event := modular.NewCloudEvent(tt.eventType, "test-source", "test-data", nil)

			shouldLog := module.shouldLogEvent(event)

			if tt.shouldBeFiltered {
				assert.False(t, shouldLog, "Event should be filtered: %s", tt.reason)
			} else {
				assert.True(t, shouldLog, "Event should not be filtered: %s", tt.reason)
			}
		})
	}
}

// TestEventLoggerModule_EventAmplificationPrevention tests the real-world scenario from the issue
func TestEventLoggerModule_EventAmplificationPrevention(t *testing.T) {
	config := &EventLoggerConfig{
		Enabled:       true,
		LogLevel:      "INFO",
		Format:        "structured",
		BufferSize:    100,
		FlushInterval: 5 * time.Second,
		// Use blacklist to exclude EventLogger's own operational events
		EventTypeBlacklist: []string{
			"com.modular.eventlogger.event.received",
			"com.modular.eventlogger.event.processed",
			"com.modular.eventlogger.output.success",
			"com.modular.eventlogger.buffer.full",
			"com.modular.eventlogger.event.dropped",
		},
		OutputTargets: []OutputTargetConfig{
			{
				Type:   "console",
				Level:  "INFO",
				Format: "structured",
				Console: &ConsoleTargetConfig{
					UseColor:   false,
					Timestamps: true,
				},
			},
		},
	}

	mockOutput := &mockOutputForTesting{events: make([]*LogEntry, 0)}

	module := &EventLoggerModule{
		name:    ModuleName,
		config:  config,
		logger:  &testLogger{},
		outputs: []OutputTarget{mockOutput},
	}

	// Test business event - should be logged
	businessEvent := modular.NewCloudEvent("order.created", "order-service", "order-data", nil)
	assert.True(t, module.shouldLogEvent(businessEvent), "Business events should be logged")

	// Test EventLogger operational events - should be filtered
	operationalEvents := []string{
		"com.modular.eventlogger.event.received",
		"com.modular.eventlogger.event.processed",
		"com.modular.eventlogger.output.success",
		"com.modular.eventlogger.buffer.full",
		"com.modular.eventlogger.event.dropped",
	}

	for _, eventType := range operationalEvents {
		event := modular.NewCloudEvent(eventType, "eventlogger-module", "operational-data", nil)
		assert.False(t, module.shouldLogEvent(event), "Operational event %s should be filtered", eventType)
	}
}

// TestEventLoggerModule_ExcludeOwnEventsIntegration tests excludeOwnEvents flag with integration
func TestEventLoggerModule_ExcludeOwnEventsIntegration(t *testing.T) {
	config := &EventLoggerConfig{
		Enabled:          true,
		LogLevel:         "INFO",
		Format:           "structured",
		BufferSize:       100,
		FlushInterval:    5 * time.Second,
		ExcludeOwnEvents: true, // Automatically exclude EventLogger's own events
		OutputTargets: []OutputTargetConfig{
			{
				Type:   "console",
				Level:  "INFO",
				Format: "structured",
				Console: &ConsoleTargetConfig{
					UseColor:   false,
					Timestamps: true,
				},
			},
		},
	}

	mockOutput := &mockOutputForTesting{events: make([]*LogEntry, 0)}

	module := &EventLoggerModule{
		name:    ModuleName,
		config:  config,
		logger:  &testLogger{},
		outputs: []OutputTarget{mockOutput},
	}

	// Test business event - should be logged
	businessEvent := modular.NewCloudEvent("user.registered", "auth-service", "user-data", nil)
	assert.True(t, module.shouldLogEvent(businessEvent), "Business events should be logged")

	// Test EventLogger operational events - should be filtered by excludeOwnEvents
	operationalEvent := modular.NewCloudEvent("com.modular.eventlogger.started", "eventlogger-module", "started", nil)
	assert.False(t, module.shouldLogEvent(operationalEvent), "EventLogger operational events should be filtered")

	// Test other module events - should be logged
	otherModuleEvent := modular.NewCloudEvent("com.modular.database.connected", "database-module", "connected", nil)
	assert.True(t, module.shouldLogEvent(otherModuleEvent), "Other module events should be logged")
}

// TestEventLoggerModule_FullIntegrationWithBlacklist tests end-to-end with actual application
func TestEventLoggerModule_FullIntegrationWithBlacklist(t *testing.T) {
	// Test that module can be initialized with blacklist configuration
	config := &EventLoggerConfig{
		Enabled:    true,
		LogLevel:   "INFO",
		Format:     "structured",
		BufferSize: 10,
		EventTypeBlacklist: []string{
			"com.modular.eventlogger.event.received",
			"com.modular.eventlogger.output.success",
		},
		FlushInterval: 5 * time.Second,
		OutputTargets: []OutputTargetConfig{
			{
				Type:   "console",
				Level:  "INFO",
				Format: "structured",
				Console: &ConsoleTargetConfig{
					UseColor:   false,
					Timestamps: true,
				},
			},
		},
	}

	// Validate configuration
	err := config.Validate()
	require.NoError(t, err, "Blacklist configuration should be valid")

	// Test that events are filtered correctly
	mockOutput := &mockOutputForTesting{events: make([]*LogEntry, 0)}
	module := &EventLoggerModule{
		name:    ModuleName,
		config:  config,
		logger:  &testLogger{},
		outputs: []OutputTarget{mockOutput},
	}

	// Business event should pass
	businessEvent := modular.NewCloudEvent("order.created", "order-service", "order-data", nil)
	assert.True(t, module.shouldLogEvent(businessEvent), "Business events should be logged")

	// Blacklisted event should be filtered
	blacklistedEvent := modular.NewCloudEvent("com.modular.eventlogger.event.received", "eventlogger-module", "operational-data", nil)
	assert.False(t, module.shouldLogEvent(blacklistedEvent), "Blacklisted event should be filtered")
}

// TestEventLoggerModule_ExcludeOwnEventsFullIntegration tests excludeOwnEvents with configuration
func TestEventLoggerModule_ExcludeOwnEventsFullIntegration(t *testing.T) {
	// Test that module can be initialized with excludeOwnEvents configuration
	config := &EventLoggerConfig{
		Enabled:          true,
		LogLevel:         "INFO",
		Format:           "structured",
		BufferSize:       10,
		ExcludeOwnEvents: true,
		FlushInterval:    5 * time.Second,
		OutputTargets: []OutputTargetConfig{
			{
				Type:   "console",
				Level:  "INFO",
				Format: "structured",
				Console: &ConsoleTargetConfig{
					UseColor:   false,
					Timestamps: true,
				},
			},
		},
	}

	// Validate configuration
	err := config.Validate()
	require.NoError(t, err, "ExcludeOwnEvents configuration should be valid")

	// Test that events are filtered correctly
	mockOutput := &mockOutputForTesting{events: make([]*LogEntry, 0)}
	module := &EventLoggerModule{
		name:    ModuleName,
		config:  config,
		logger:  &testLogger{},
		outputs: []OutputTarget{mockOutput},
	}

	// Business event should pass
	businessEvent := modular.NewCloudEvent("user.registered", "auth-service", "user-data", nil)
	assert.True(t, module.shouldLogEvent(businessEvent), "Business events should be logged")

	// EventLogger's own event should be filtered
	ownEvent := modular.NewCloudEvent("com.modular.eventlogger.started", "eventlogger-module", "started", nil)
	assert.False(t, module.shouldLogEvent(ownEvent), "EventLogger's own events should be filtered")

	// Other module events should pass
	otherModuleEvent := modular.NewCloudEvent("com.modular.database.connected", "database-module", "connected", nil)
	assert.True(t, module.shouldLogEvent(otherModuleEvent), "Other module events should be logged")
}
