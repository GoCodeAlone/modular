package logmasker

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/CrisisTextLine/modular"
)

// MockLogger implements modular.Logger for testing.
type MockLogger struct {
	InfoCalls  []LogCall
	ErrorCalls []LogCall
	WarnCalls  []LogCall
	DebugCalls []LogCall
}

type LogCall struct {
	Message string
	Args    []any
}

func (m *MockLogger) Info(msg string, args ...any) {
	m.InfoCalls = append(m.InfoCalls, LogCall{Message: msg, Args: args})
}

func (m *MockLogger) Error(msg string, args ...any) {
	m.ErrorCalls = append(m.ErrorCalls, LogCall{Message: msg, Args: args})
}

func (m *MockLogger) Warn(msg string, args ...any) {
	m.WarnCalls = append(m.WarnCalls, LogCall{Message: msg, Args: args})
}

func (m *MockLogger) Debug(msg string, args ...any) {
	m.DebugCalls = append(m.DebugCalls, LogCall{Message: msg, Args: args})
}

// MockApplication implements modular.Application for testing.
type MockApplication struct {
	configs        map[string]modular.ConfigProvider
	services       map[string]any
	logger         modular.Logger
	configProvider modular.ConfigProvider
}

func NewMockApplication(logger modular.Logger) *MockApplication {
	return &MockApplication{
		configs:  make(map[string]modular.ConfigProvider),
		services: make(map[string]any),
		logger:   logger,
	}
}

func (m *MockApplication) ConfigProvider() modular.ConfigProvider { return m.configProvider }
func (m *MockApplication) SvcRegistry() modular.ServiceRegistry   { return nil }
func (m *MockApplication) Logger() modular.Logger                 { return m.logger }

func (m *MockApplication) RegisterConfigSection(section string, cp modular.ConfigProvider) {
	m.configs[section] = cp
}

func (m *MockApplication) GetConfigSection(section string) (modular.ConfigProvider, error) {
	cp, exists := m.configs[section]
	if !exists {
		return nil, fmt.Errorf("%w: %s", modular.ErrConfigSectionNotFound, section)
	}
	return cp, nil
}

func (m *MockApplication) RegisterService(name string, service any) error {
	m.services[name] = service
	return nil
}

func (m *MockApplication) GetService(name string, target any) error {
	service, exists := m.services[name]
	if !exists {
		return fmt.Errorf("%w: %s", modular.ErrServiceNotFound, name)
	}

	// Simple type assignment - in real implementation this would be more sophisticated
	switch t := target.(type) {
	case *modular.Logger:
		if logger, ok := service.(modular.Logger); ok {
			*t = logger
		} else {
			return fmt.Errorf("%w: %s", modular.ErrServiceNotFound, name)
		}
	default:
		return fmt.Errorf("%w: %s", modular.ErrServiceNotFound, name)
	}

	return nil
}

func (m *MockApplication) RegisterModule(module modular.Module)              {}
func (m *MockApplication) ConfigSections() map[string]modular.ConfigProvider { return m.configs }
func (m *MockApplication) IsVerboseConfig() bool                             { return false }
func (m *MockApplication) SetVerboseConfig(bool)                             {}
func (m *MockApplication) SetLogger(modular.Logger)                          {}
func (m *MockApplication) Init() error                                       { return nil }
func (m *MockApplication) Start() error                                      { return nil }
func (m *MockApplication) Stop() error                                       { return nil }
func (m *MockApplication) Run() error                                        { return nil }

// Newly added methods to satisfy expanded modular.Application interface
func (m *MockApplication) GetServicesByModule(moduleName string) []string { return []string{} }
func (m *MockApplication) GetServiceEntry(serviceName string) (*modular.ServiceRegistryEntry, bool) {
	return nil, false
}
func (m *MockApplication) GetServicesByInterface(interfaceType reflect.Type) []*modular.ServiceRegistryEntry {
	return []*modular.ServiceRegistryEntry{}
}

// TestMaskableValue implements the MaskableValue interface for testing.
type TestMaskableValue struct {
	Value           string
	ShouldMaskValue bool
	MaskedValue     any
	Strategy        MaskStrategy
}

func (t *TestMaskableValue) ShouldMask() bool {
	return t.ShouldMaskValue
}

func (t *TestMaskableValue) GetMaskedValue() any {
	return t.MaskedValue
}

func (t *TestMaskableValue) GetMaskStrategy() MaskStrategy {
	return t.Strategy
}

func TestLogMaskerModule_Name(t *testing.T) {
	module := NewModule()
	if module.Name() != ModuleName {
		t.Errorf("Expected module name %s, got %s", ModuleName, module.Name())
	}
}

func TestLogMaskerModule_RegisterConfig(t *testing.T) {
	module := NewModule()
	mockLogger := &MockLogger{}
	app := NewMockApplication(mockLogger)

	err := module.RegisterConfig(app)
	if err != nil {
		t.Fatalf("RegisterConfig failed: %v", err)
	}

	// Verify config was registered
	if len(app.configs) != 1 {
		t.Errorf("Expected 1 config section, got %d", len(app.configs))
	}

	if _, exists := app.configs[ModuleName]; !exists {
		t.Error("Expected config section to be registered")
	}
}

func TestLogMaskerModule_Init(t *testing.T) {
	module := NewModule()
	mockLogger := &MockLogger{}
	app := NewMockApplication(mockLogger)

	// Register config and logger service
	err := module.RegisterConfig(app)
	if err != nil {
		t.Fatalf("RegisterConfig failed: %v", err)
	}

	app.RegisterService("logger", mockLogger)

	err = module.Init(app)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify module is properly initialized
	if module.config == nil {
		t.Error("Expected config to be set after initialization")
	}

	if module.originalLogger == nil {
		t.Error("Expected original logger to be set after initialization")
	}
}

func TestLogMaskerModule_ProvidesServices(t *testing.T) {
	module := NewModule()
	services := module.ProvidesServices()

	if len(services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(services))
	}

	if services[0].Name != ServiceName {
		t.Errorf("Expected service name %s, got %s", ServiceName, services[0].Name)
	}
}

func TestMaskingLogger_FieldBasedMasking(t *testing.T) {
	module := NewModule()
	mockLogger := &MockLogger{}
	app := NewMockApplication(mockLogger)

	// Setup
	module.RegisterConfig(app)
	app.RegisterService("logger", mockLogger)
	module.Init(app)

	masker := &MaskingLogger{
		BaseLoggerDecorator: modular.NewBaseLoggerDecorator(mockLogger),
		module:              module,
	}

	// Test password masking
	masker.Info("User login", "email", "user@example.com", "password", "secret123")

	if len(mockLogger.InfoCalls) != 1 {
		t.Fatalf("Expected 1 info call, got %d", len(mockLogger.InfoCalls))
	}

	args := mockLogger.InfoCalls[0].Args
	if len(args) != 4 {
		t.Fatalf("Expected 4 args, got %d", len(args))
	}

	// Check that password is redacted
	if args[3] != "[REDACTED]" {
		t.Errorf("Expected password to be redacted, got %v", args[3])
	}

	// Check that email is partially masked (default config shows first 2, last 2)
	emailValue := args[1].(string)
	if !strings.Contains(emailValue, "*") || len(emailValue) != len("user@example.com") {
		t.Errorf("Expected email to be partially masked, got %v", emailValue)
	}
}

func TestMaskingLogger_PatternBasedMasking(t *testing.T) {
	module := NewModule()
	mockLogger := &MockLogger{}
	app := NewMockApplication(mockLogger)

	// Setup
	module.RegisterConfig(app)
	app.RegisterService("logger", mockLogger)
	module.Init(app)

	masker := &MaskingLogger{
		BaseLoggerDecorator: modular.NewBaseLoggerDecorator(mockLogger),
		module:              module,
	}

	// Test credit card number masking
	masker.Info("Payment processed", "card", "4111-1111-1111-1111", "amount", "100")

	if len(mockLogger.InfoCalls) != 1 {
		t.Fatalf("Expected 1 info call, got %d", len(mockLogger.InfoCalls))
	}

	args := mockLogger.InfoCalls[0].Args
	cardValue := args[1]

	// Credit card should be redacted due to pattern matching
	if cardValue != "[REDACTED]" {
		t.Errorf("Expected credit card to be redacted, got %v", cardValue)
	}

	// Amount should not be masked
	if args[3] != "100" {
		t.Errorf("Expected amount to not be masked, got %v", args[3])
	}
}

func TestMaskingLogger_MaskableValueInterface(t *testing.T) {
	module := NewModule()
	mockLogger := &MockLogger{}
	app := NewMockApplication(mockLogger)

	// Setup
	module.RegisterConfig(app)
	app.RegisterService("logger", mockLogger)
	module.Init(app)

	masker := &MaskingLogger{
		BaseLoggerDecorator: modular.NewBaseLoggerDecorator(mockLogger),
		module:              module,
	}

	// Test with a value that should be masked
	maskableValue := &TestMaskableValue{
		Value:           "sensitive-data",
		ShouldMaskValue: true,
		MaskedValue:     "***MASKED***",
		Strategy:        MaskStrategyRedact,
	}

	// Test with a value that should not be masked
	nonMaskableValue := &TestMaskableValue{
		Value:           "public-data",
		ShouldMaskValue: false,
		MaskedValue:     "should not see this",
		Strategy:        MaskStrategyNone,
	}

	masker.Info("Testing maskable values",
		"sensitive", maskableValue,
		"public", nonMaskableValue)

	if len(mockLogger.InfoCalls) != 1 {
		t.Fatalf("Expected 1 info call, got %d", len(mockLogger.InfoCalls))
	}

	args := mockLogger.InfoCalls[0].Args
	if len(args) != 4 {
		t.Fatalf("Expected 4 args, got %d", len(args))
	}

	// Check that sensitive value was masked
	if args[1] != "***MASKED***" {
		t.Errorf("Expected sensitive value to be masked, got %v", args[1])
	}

	// Check that public value was not masked
	if args[3] != nonMaskableValue {
		t.Errorf("Expected public value to not be masked, got %v", args[3])
	}
}

func TestMaskingLogger_DisabledMasking(t *testing.T) {
	module := NewModule()
	mockLogger := &MockLogger{}
	app := NewMockApplication(mockLogger)

	// Setup with masking disabled
	module.RegisterConfig(app)

	// Override config to disable masking
	config := &LogMaskerConfig{Enabled: false}
	app.configs[ModuleName] = modular.NewStdConfigProvider(config)

	app.RegisterService("logger", mockLogger)
	module.Init(app)

	masker := &MaskingLogger{
		BaseLoggerDecorator: modular.NewBaseLoggerDecorator(mockLogger),
		module:              module,
	}

	// Test with sensitive data - should not be masked
	masker.Info("User login", "password", "secret123", "token", "abc-def-123")

	if len(mockLogger.InfoCalls) != 1 {
		t.Fatalf("Expected 1 info call, got %d", len(mockLogger.InfoCalls))
	}

	args := mockLogger.InfoCalls[0].Args

	// Values should not be masked when disabled
	if args[1] != "secret123" {
		t.Errorf("Expected password to not be masked when disabled, got %v", args[1])
	}

	if args[3] != "abc-def-123" {
		t.Errorf("Expected token to not be masked when disabled, got %v", args[3])
	}
}

func TestMaskingStrategies(t *testing.T) {
	module := NewModule()
	mockLogger := &MockLogger{}
	app := NewMockApplication(mockLogger)

	// Setup
	module.RegisterConfig(app)
	app.RegisterService("logger", mockLogger)
	module.Init(app)

	masker := &MaskingLogger{
		BaseLoggerDecorator: modular.NewBaseLoggerDecorator(mockLogger),
		module:              module,
	}

	tests := []struct {
		strategy MaskStrategy
		value    any
		expected func(any) bool // Function to check if result is as expected
	}{
		{
			strategy: MaskStrategyRedact,
			value:    "sensitive",
			expected: func(result any) bool { return result == "[REDACTED]" },
		},
		{
			strategy: MaskStrategyPartial,
			value:    "longstring",
			expected: func(result any) bool {
				str, ok := result.(string)
				return ok && strings.Contains(str, "*") && len(str) == len("longstring")
			},
		},
		{
			strategy: MaskStrategyHash,
			value:    "data",
			expected: func(result any) bool {
				str, ok := result.(string)
				return ok && strings.HasPrefix(str, "[HASH:")
			},
		},
		{
			strategy: MaskStrategyNone,
			value:    "data",
			expected: func(result any) bool { return result == "data" },
		},
	}

	for _, test := range tests {
		t.Run(string(test.strategy), func(t *testing.T) {
			result := masker.applyMaskStrategy(test.value, test.strategy, nil)
			if !test.expected(result) {
				t.Errorf("Strategy %s failed: expected valid result, got %v", test.strategy, result)
			}
		})
	}
}

func TestPartialMasking(t *testing.T) {
	module := NewModule()
	mockLogger := &MockLogger{} // Add mockLogger for the decorator
	masker := &MaskingLogger{
		BaseLoggerDecorator: modular.NewBaseLoggerDecorator(mockLogger),
		module:              module,
	}

	config := &PartialMaskConfig{
		ShowFirst: 2,
		ShowLast:  2,
		MaskChar:  "*",
		MinLength: 4,
	}

	tests := []struct {
		input    string
		expected string
		name     string
	}{
		{
			input:    "short",
			expected: "sh*rt",
			name:     "normal case",
		},
		{
			input:    "ab",
			expected: "ab", // Too short, not masked
			name:     "too short",
		},
		{
			input:    "abcd",
			expected: "abcd", // Exactly min length, but showFirst+showLast >= length
			name:     "exactly min length",
		},
		{
			input:    "abcde",
			expected: "ab*de",
			name:     "just above min length",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := masker.partialMask(test.input, config)
			if result != test.expected {
				t.Errorf("Expected %s, got %s", test.expected, result)
			}
		})
	}
}

func TestAllLogLevels(t *testing.T) {
	module := NewModule()
	mockLogger := &MockLogger{}
	app := NewMockApplication(mockLogger)

	// Setup
	module.RegisterConfig(app)
	app.RegisterService("logger", mockLogger)
	module.Init(app)

	masker := &MaskingLogger{
		BaseLoggerDecorator: modular.NewBaseLoggerDecorator(mockLogger),
		module:              module,
	}

	// Test all log levels
	masker.Info("Info message", "password", "secret")
	masker.Error("Error message", "password", "secret")
	masker.Warn("Warn message", "password", "secret")
	masker.Debug("Debug message", "password", "secret")

	// Verify all calls were made with masking
	if len(mockLogger.InfoCalls) != 1 || mockLogger.InfoCalls[0].Args[1] != "[REDACTED]" {
		t.Error("Info call was not properly masked")
	}

	if len(mockLogger.ErrorCalls) != 1 || mockLogger.ErrorCalls[0].Args[1] != "[REDACTED]" {
		t.Error("Error call was not properly masked")
	}

	if len(mockLogger.WarnCalls) != 1 || mockLogger.WarnCalls[0].Args[1] != "[REDACTED]" {
		t.Error("Warn call was not properly masked")
	}

	if len(mockLogger.DebugCalls) != 1 || mockLogger.DebugCalls[0].Args[1] != "[REDACTED]" {
		t.Error("Debug call was not properly masked")
	}
}
