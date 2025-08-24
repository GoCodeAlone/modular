package eventlogger

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func TestEventLoggerModule_Init(t *testing.T) {
	// Create mock application
	app := &MockApplication{
		configSections: make(map[string]modular.ConfigProvider),
		logger:         &MockLogger{},
	}

	// Create module
	module := NewModule().(*EventLoggerModule)

	// Register config
	err := module.RegisterConfig(app)
	if err != nil {
		t.Fatalf("Failed to register config: %v", err)
	}

	// Initialize module
	err = module.Init(app)
	if err != nil {
		t.Fatalf("Failed to initialize module: %v", err)
	}

	// Check that module was initialized
	if module.config == nil {
		t.Error("Expected config to be set")
	}

	if module.logger == nil {
		t.Error("Expected logger to be set")
	}

	if len(module.outputs) == 0 {
		t.Error("Expected at least one output target")
	}
}

func TestEventLoggerModule_ObserverInterface(t *testing.T) {
	module := NewModule().(*EventLoggerModule)

	// Test ObserverID
	if module.ObserverID() != ModuleName {
		t.Errorf("Expected ObserverID to be %s, got %s", ModuleName, module.ObserverID())
	}

	// Test OnEvent without initialization (should fail)
	event := modular.NewCloudEvent(
		"test.event",
		"test",
		"test data",
		nil,
	)

	err := module.OnEvent(context.Background(), event)
	if !errors.Is(err, ErrLoggerNotStarted) {
		t.Errorf("Expected ErrLoggerNotStarted, got %v", err)
	}
}

func TestEventLoggerModule_ConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *EventLoggerConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &EventLoggerConfig{
				Enabled:       true,
				LogLevel:      "INFO",
				Format:        "json",
				FlushInterval: 5 * time.Second,
				OutputTargets: []OutputTargetConfig{
					{
						Type:   "console",
						Level:  "INFO",
						Format: "json",
						Console: &ConsoleTargetConfig{
							UseColor:   true,
							Timestamps: true,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid log level",
			config: &EventLoggerConfig{
				LogLevel: "INVALID",
				Format:   "json",
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			config: &EventLoggerConfig{
				LogLevel: "INFO",
				Format:   "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid flush interval",
			config: &EventLoggerConfig{
				LogLevel:      "INFO",
				Format:        "json",
				FlushInterval: -1 * time.Second, // Invalid negative duration
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOutputTargetConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  OutputTargetConfig
		wantErr bool
	}{
		{
			name: "valid console config",
			config: OutputTargetConfig{
				Type:   "console",
				Level:  "INFO",
				Format: "json",
				Console: &ConsoleTargetConfig{
					UseColor:   true,
					Timestamps: true,
				},
			},
			wantErr: false,
		},
		{
			name: "valid file config",
			config: OutputTargetConfig{
				Type:   "file",
				Level:  "DEBUG",
				Format: "json",
				File: &FileTargetConfig{
					Path:       "/tmp/test.log",
					MaxSize:    100,
					MaxBackups: 5,
					Compress:   true,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid type",
			config: OutputTargetConfig{
				Type:   "invalid",
				Level:  "INFO",
				Format: "json",
			},
			wantErr: true,
		},
		{
			name: "missing file config",
			config: OutputTargetConfig{
				Type:   "file",
				Level:  "INFO",
				Format: "json",
			},
			wantErr: true,
		},
		{
			name: "missing file path",
			config: OutputTargetConfig{
				Type:   "file",
				Level:  "INFO",
				Format: "json",
				File:   &FileTargetConfig{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("OutputTargetConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEventLoggerModule_EventProcessing(t *testing.T) {
	// Create mock application with test config
	app := &MockApplication{
		configSections: make(map[string]modular.ConfigProvider),
		logger:         &MockLogger{},
	}

	// Create module with test configuration
	module := NewModule().(*EventLoggerModule)

	// Set up test config manually for this test
	testConfig := &EventLoggerConfig{
		Enabled:       true,
		LogLevel:      "DEBUG",
		Format:        "json",
		BufferSize:    10,
		FlushInterval: 1 * time.Second,
		OutputTargets: []OutputTargetConfig{
			{
				Type:   "console",
				Level:  "DEBUG",
				Format: "json",
				Console: &ConsoleTargetConfig{
					UseColor:   false,
					Timestamps: true,
				},
			},
		},
	}

	module.config = testConfig
	module.logger = app.logger

	// Initialize output targets
	outputs := make([]OutputTarget, 0, len(testConfig.OutputTargets))
	for _, targetConfig := range testConfig.OutputTargets {
		output, err := NewOutputTarget(targetConfig, module.logger)
		if err != nil {
			t.Fatalf("Failed to create output target: %v", err)
		}
		outputs = append(outputs, output)
	}
	module.outputs = outputs

	// Initialize channels
	module.eventChan = make(chan cloudevents.Event, testConfig.BufferSize)
	module.stopChan = make(chan struct{})

	// Start the module
	ctx := context.Background()
	err := module.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start module: %v", err)
	}

	// Test event logging
	testEvent := modular.NewCloudEvent(
		"test.event",
		"test",
		"test data",
		nil,
	)

	err = module.OnEvent(ctx, testEvent)
	if err != nil {
		t.Errorf("OnEvent failed: %v", err)
	}

	// Wait a moment for processing
	time.Sleep(100 * time.Millisecond)

	// Stop the module
	err = module.Stop(ctx)
	if err != nil {
		t.Errorf("Failed to stop module: %v", err)
	}
}

func TestEventLoggerModule_EventFiltering(t *testing.T) {
	module := &EventLoggerModule{
		config: &EventLoggerConfig{
			LogLevel: "INFO",
			EventTypeFilters: []string{
				"module.registered",
				"service.registered",
			},
		},
	}

	tests := []struct {
		name     string
		event    cloudevents.Event
		expected bool
	}{
		{
			name:     "filtered event",
			event:    modular.NewCloudEvent("module.registered", "test", nil, nil),
			expected: true,
		},
		{
			name:     "unfiltered event",
			event:    modular.NewCloudEvent("unfiltered.event", "test", nil, nil),
			expected: false,
		},
		{
			name:     "error level event",
			event:    modular.NewCloudEvent("application.failed", "test", nil, nil),
			expected: false, // Filtered out by event type filter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := module.shouldLogEvent(tt.event)
			if result != tt.expected {
				t.Errorf("shouldLogEvent() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestEventLoggerModule_LogLevels(t *testing.T) {
	module := &EventLoggerModule{
		config: &EventLoggerConfig{
			LogLevel: "WARN",
		},
	}

	tests := []struct {
		name      string
		eventType string
		expected  bool
	}{
		{
			name:      "error event should log",
			eventType: modular.EventTypeApplicationFailed,
			expected:  true,
		},
		{
			name:      "info event should not log",
			eventType: modular.EventTypeModuleRegistered,
			expected:  false,
		},
		{
			name:      "debug event should not log",
			eventType: modular.EventTypeConfigLoaded,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := modular.NewCloudEvent(tt.eventType, "test", nil, nil)
			result := module.shouldLogEvent(event)
			if result != tt.expected {
				t.Errorf("shouldLogEvent() = %v, expected %v for event type %s", result, tt.expected, tt.eventType)
			}
		})
	}
}

// Mock types for testing
type MockApplication struct {
	configSections map[string]modular.ConfigProvider
	logger         modular.Logger
}

func (m *MockApplication) ConfigProvider() modular.ConfigProvider { return nil }
func (m *MockApplication) SvcRegistry() modular.ServiceRegistry   { return nil }
func (m *MockApplication) Logger() modular.Logger                 { return m.logger }
func (m *MockApplication) RegisterModule(module modular.Module)   {}
func (m *MockApplication) RegisterConfigSection(section string, cp modular.ConfigProvider) {
	m.configSections[section] = cp
}
func (m *MockApplication) GetConfigSection(section string) (modular.ConfigProvider, error) {
	if cp, exists := m.configSections[section]; exists {
		return cp, nil
	}
	return nil, modular.ErrConfigSectionNotFound
}
func (m *MockApplication) RegisterService(name string, service any) error { return nil }
func (m *MockApplication) ConfigSections() map[string]modular.ConfigProvider {
	return m.configSections
}
func (m *MockApplication) GetService(name string, target any) error { return nil }
func (m *MockApplication) IsVerboseConfig() bool                    { return false }
func (m *MockApplication) SetVerboseConfig(bool)                    {}
func (m *MockApplication) SetLogger(modular.Logger)                 {}
func (m *MockApplication) Init() error                              { return nil }
func (m *MockApplication) Start() error                             { return nil }
func (m *MockApplication) Stop() error                              { return nil }
func (m *MockApplication) Run() error                               { return nil }

type MockLogger struct {
	entries []MockLogEntry
}

type MockLogEntry struct {
	Level   string
	Message string
	Args    []interface{}
}

func (l *MockLogger) Info(msg string, args ...interface{}) {
	l.entries = append(l.entries, MockLogEntry{Level: "INFO", Message: msg, Args: args})
}

func (l *MockLogger) Error(msg string, args ...interface{}) {
	l.entries = append(l.entries, MockLogEntry{Level: "ERROR", Message: msg, Args: args})
}

func (l *MockLogger) Debug(msg string, args ...interface{}) {
	l.entries = append(l.entries, MockLogEntry{Level: "DEBUG", Message: msg, Args: args})
}

func (l *MockLogger) Warn(msg string, args ...interface{}) {
	l.entries = append(l.entries, MockLogEntry{Level: "WARN", Message: msg, Args: args})
}
