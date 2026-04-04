package feeders

import (
	"fmt"
	"os"
	"testing"
)

// Mock logger for testing verbose debug functionality
type mockLogger struct {
	messages []string
}

func (m *mockLogger) Debug(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	m.messages = append(m.messages, formatted)
}

func (m *mockLogger) getMessages() []string {
	return m.messages
}

func TestYamlFeeder_Feed_BasicStructure(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
app:
  name: TestApp
  version: "1.0"
  debug: true
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type Config struct {
		App struct {
			Name    string `yaml:"name"`
			Version string `yaml:"version"`
			Debug   bool   `yaml:"debug"`
		} `yaml:"app"`
	}

	var config Config
	feeder := NewYamlFeeder(tempFile.Name())
	err = feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if config.App.Name != "TestApp" {
		t.Errorf("Expected Name to be 'TestApp', got '%s'", config.App.Name)
	}
	if config.App.Version != "1.0" {
		t.Errorf("Expected Version to be '1.0', got '%s'", config.App.Version)
	}
	if !config.App.Debug {
		t.Errorf("Expected Debug to be true, got false")
	}
}

func TestYamlFeeder_Feed_PrimitiveTypes(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
stringField: "hello"
intField: 42
int64Field: 9223372036854775807
uintField: 123
floatField: 3.14
boolField: true
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type Config struct {
		StringField string  `yaml:"stringField"`
		IntField    int     `yaml:"intField"`
		Int64Field  int64   `yaml:"int64Field"`
		UintField   uint    `yaml:"uintField"`
		FloatField  float64 `yaml:"floatField"`
		BoolField   bool    `yaml:"boolField"`
	}

	var config Config
	feeder := NewYamlFeeder(tempFile.Name())
	err = feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if config.StringField != "hello" {
		t.Errorf("Expected StringField to be 'hello', got '%s'", config.StringField)
	}
	if config.IntField != 42 {
		t.Errorf("Expected IntField to be 42, got %d", config.IntField)
	}
	if config.Int64Field != 9223372036854775807 {
		t.Errorf("Expected Int64Field to be 9223372036854775807, got %d", config.Int64Field)
	}
	if config.UintField != 123 {
		t.Errorf("Expected UintField to be 123, got %d", config.UintField)
	}
	if config.FloatField != 3.14 {
		t.Errorf("Expected FloatField to be 3.14, got %f", config.FloatField)
	}
	if !config.BoolField {
		t.Errorf("Expected BoolField to be true, got false")
	}
}

func TestYamlFeeder_NewYamlFeeder(t *testing.T) {
	filePath := "/test/path.yaml"
	feeder := NewYamlFeeder(filePath)

	if feeder == nil {
		t.Fatal("Expected feeder to be created, got nil")
	}
	if feeder.Path != filePath {
		t.Errorf("Expected path to be '%s', got '%s'", filePath, feeder.Path)
	}
	if feeder.verboseDebug {
		t.Error("Expected verboseDebug to be false by default")
	}
	if feeder.debugFn != nil {
		t.Error("Expected debugFn to be nil by default")
	}
	if feeder.ft.Has() {
		t.Error("Expected field tracker holder to be empty by default")
	}
}

func TestYamlFeeder_SetVerboseDebug(t *testing.T) {
	feeder := NewYamlFeeder("/test/path.yaml")
	logger := &mockLogger{}

	feeder.SetVerboseDebug(true, logger)

	if !feeder.verboseDebug {
		t.Error("Expected verboseDebug to be true")
	}
	if feeder.debugFn == nil {
		t.Error("Expected debugFn to be set")
	}

	// Check that debug message was logged
	messages := logger.getMessages()
	if len(messages) == 0 {
		t.Error("Expected debug message to be logged")
	}
}

func TestYamlFeeder_SetFieldTracker(t *testing.T) {
	feeder := NewYamlFeeder("/test/path.yaml")
	tracker := NewDefaultFieldTracker()

	feeder.SetFieldTracker(tracker)

	if !feeder.ft.Has() {
		t.Error("Expected field tracker to be set")
	}
}
