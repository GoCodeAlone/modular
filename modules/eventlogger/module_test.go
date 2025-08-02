package eventlogger

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
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
				FlushInterval: "5s",
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
				FlushInterval: "invalid",
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
		FlushInterval: "1s",
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

// Additional test cases to improve coverage
func TestEventLoggerModule_Dependencies(t *testing.T) {
	module := NewModule().(*EventLoggerModule)
	deps := module.Dependencies()
	if len(deps) != 0 {
		t.Errorf("Expected 0 dependencies, got %d", len(deps))
	}
}

func TestEventLoggerModule_ProvidesServices(t *testing.T) {
	module := NewModule().(*EventLoggerModule)
	services := module.ProvidesServices()
	if len(services) != 1 {
		t.Errorf("Expected 1 provided service, got %d", len(services))
	}
}

func TestEventLoggerModule_RequiresServices(t *testing.T) {
	module := NewModule().(*EventLoggerModule)
	services := module.RequiresServices()
	if len(services) != 0 {
		t.Errorf("Expected 0 required services, got %d", len(services))
	}
}

func TestEventLoggerModule_Constructor(t *testing.T) {
	module := NewModule().(*EventLoggerModule)
	constructor := module.Constructor()
	if constructor == nil {
		t.Error("Expected non-nil constructor")
	}
}

func TestEventLoggerModule_RegisterObservers(t *testing.T) {
	// Test RegisterObservers functionality
	module := NewModule().(*EventLoggerModule)
	module.config = &EventLoggerConfig{Enabled: true}
	module.logger = &MockLogger{}

	// Create a mock observable application
	mockApp := &MockObservableApplication{
		observers: make(map[string][]modular.Observer),
	}

	// Register observers
	err := module.RegisterObservers(mockApp)
	if err != nil {
		t.Errorf("RegisterObservers failed: %v", err)
	}

	// Check that the observer was registered
	if len(mockApp.observers[module.ObserverID()]) != 1 {
		t.Error("Expected observer to be registered")
	}
}

func TestEventLoggerModule_EmitEvent(t *testing.T) {
	module := NewModule().(*EventLoggerModule)

	// Test EmitEvent (should always return error)
	event := modular.NewCloudEvent("test.event", "test", nil, nil)
	err := module.EmitEvent(context.Background(), event)
	if !errors.Is(err, ErrLoggerDoesNotEmitEvents) {
		t.Errorf("Expected ErrLoggerDoesNotEmitEvents, got %v", err)
	}
}

func TestOutputTargetError_Methods(t *testing.T) {
	originalErr := ErrFileNotOpen // Use existing static error
	err := NewOutputTargetError(1, originalErr)

	// Test Error method
	errorStr := err.Error()
	if !contains(errorStr, "output target 1") {
		t.Errorf("Error string should contain 'output target 1': %s", errorStr)
	}

	// Test Unwrap method
	unwrapped := err.Unwrap()
	if !errors.Is(unwrapped, originalErr) {
		t.Errorf("Unwrap should return original error, got %v", unwrapped)
	}
}

func TestConsoleOutput_FormatText(t *testing.T) {
	output := &ConsoleTarget{
		config: OutputTargetConfig{
			Format: "text",
			Console: &ConsoleTargetConfig{
				UseColor:   false,
				Timestamps: true,
			},
		},
	}

	// Create a LogEntry (this is what formatText expects)
	logEntry := &LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Type:      "test.event",
		Source:    "test",
		Data:      "test data",
		Metadata:  make(map[string]interface{}),
	}

	formatted, err := output.formatText(logEntry)
	if err != nil {
		t.Errorf("formatText failed: %v", err)
	}

	if len(formatted) == 0 {
		t.Error("Expected non-empty formatted text")
	}
}

func TestConsoleOutput_FormatStructured(t *testing.T) {
	output := &ConsoleTarget{
		config: OutputTargetConfig{
			Format: "structured",
			Console: &ConsoleTargetConfig{
				UseColor:   false,
				Timestamps: true,
			},
		},
	}

	// Create a LogEntry
	logEntry := &LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Type:      "test.event",
		Source:    "test",
		Data:      "test data",
		Metadata:  make(map[string]interface{}),
	}

	formatted, err := output.formatStructured(logEntry)
	if err != nil {
		t.Errorf("formatStructured failed: %v", err)
	}

	if len(formatted) == 0 {
		t.Error("Expected non-empty formatted structured output")
	}
}

func TestConsoleOutput_ColorizeLevel(t *testing.T) {
	output := &ConsoleTarget{
		config: OutputTargetConfig{
			Console: &ConsoleTargetConfig{
				UseColor: true,
			},
		},
	}

	tests := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	for _, level := range tests {
		colorized := output.colorizeLevel(level)
		if len(colorized) <= len(level) {
			t.Errorf("Expected colorized level to be longer than original: %s -> %s", level, colorized)
		}
	}
}

func TestFileTarget_Creation(t *testing.T) {
	config := OutputTargetConfig{
		Type: "file",
		File: &FileTargetConfig{
			Path:       "/tmp/test-eventlogger.log",
			MaxSize:    10,
			MaxBackups: 3,
			Compress:   true,
		},
	}

	target, err := NewFileTarget(config, &MockLogger{})
	if err != nil {
		t.Fatalf("Failed to create file target: %v", err)
	}

	if target == nil {
		t.Error("Expected non-nil file target")
	}

	// Test start/stop
	ctx := context.Background()
	err = target.Start(ctx)
	if err != nil {
		t.Errorf("Failed to start file target: %v", err)
	}

	err = target.Stop(ctx)
	if err != nil {
		t.Errorf("Failed to stop file target: %v", err)
	}
}

func TestFileTarget_Operations(t *testing.T) {
	config := OutputTargetConfig{
		Type: "file",
		File: &FileTargetConfig{
			Path:       "/tmp/test-eventlogger-ops.log",
			MaxSize:    10,
			MaxBackups: 3,
		},
	}

	target, err := NewFileTarget(config, &MockLogger{})
	if err != nil {
		t.Fatalf("Failed to create file target: %v", err)
	}

	ctx := context.Background()
	err = target.Start(ctx)
	if err != nil {
		t.Errorf("Failed to start file target: %v", err)
	}

	// Write an event
	logEntry := &LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Type:      "test.event",
		Source:    "test",
		Data:      "test data",
		Metadata:  make(map[string]interface{}),
	}
	err = target.WriteEvent(logEntry)
	if err != nil {
		t.Errorf("Failed to write event: %v", err)
	}

	// Test flush
	err = target.Flush()
	if err != nil {
		t.Errorf("Failed to flush: %v", err)
	}

	err = target.Stop(ctx)
	if err != nil {
		t.Errorf("Failed to stop file target: %v", err)
	}
}

func TestSyslogTarget_Creation(t *testing.T) {
	config := OutputTargetConfig{
		Type: "syslog",
		Syslog: &SyslogTargetConfig{
			Network:  "udp",
			Address:  "localhost:514",
			Tag:      "eventlogger",
			Facility: "local0",
		},
	}

	target, err := NewSyslogTarget(config, &MockLogger{})
	// Note: This may fail in test environment without syslog, which is expected
	if err != nil {
		t.Logf("Syslog target creation failed (expected in test environment): %v", err)
		return
	}

	if target != nil {
		_ = target.Stop(context.Background()) // Clean up if created
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Mock Observable Application for testing
type MockObservableApplication struct {
	observers map[string][]modular.Observer
}

func (m *MockObservableApplication) RegisterObserver(observer modular.Observer, eventTypes ...string) error {
	id := observer.ObserverID()
	if m.observers == nil {
		m.observers = make(map[string][]modular.Observer)
	}
	m.observers[id] = append(m.observers[id], observer)
	return nil
}

func (m *MockObservableApplication) UnregisterObserver(observer modular.Observer) error {
	id := observer.ObserverID()
	if m.observers != nil {
		delete(m.observers, id)
	}
	return nil
}

func (m *MockObservableApplication) GetObservers() []modular.ObserverInfo {
	var infos []modular.ObserverInfo
	for id, observers := range m.observers {
		if len(observers) > 0 {
			infos = append(infos, modular.ObserverInfo{
				ID:           id,
				EventTypes:   []string{}, // All events
				RegisteredAt: time.Now(),
			})
		}
	}
	return infos
}

func (m *MockObservableApplication) NotifyObservers(ctx context.Context, event cloudevents.Event) error {
	// Implementation not needed for these tests
	return nil
}
