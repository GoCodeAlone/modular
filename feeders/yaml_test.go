package feeders

import (
	"errors"
	"fmt"
	"os"
	"strings"
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

func TestYamlFeeder_Feed_StringConversions(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
intFromString: "42"
floatFromString: "3.14"
boolFromString: "true"
boolFromOne: "1"
boolFromFalse: "false"
boolFromZero: "0"
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type Config struct {
		IntFromString   int     `yaml:"intFromString"`
		FloatFromString float64 `yaml:"floatFromString"`
		BoolFromString  bool    `yaml:"boolFromString"`
		BoolFromOne     bool    `yaml:"boolFromOne"`
		BoolFromFalse   bool    `yaml:"boolFromFalse"`
		BoolFromZero    bool    `yaml:"boolFromZero"`
	}

	// Test with field tracking enabled to use custom parsing
	tracker := NewDefaultFieldTracker()
	var config Config
	feeder := NewYamlFeeder(tempFile.Name())
	feeder.SetFieldTracker(tracker)
	err = feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if config.IntFromString != 42 {
		t.Errorf("Expected IntFromString to be 42, got %d", config.IntFromString)
	}
	if config.FloatFromString != 3.14 {
		t.Errorf("Expected FloatFromString to be 3.14, got %f", config.FloatFromString)
	}
	if !config.BoolFromString {
		t.Errorf("Expected BoolFromString to be true, got false")
	}
	if !config.BoolFromOne {
		t.Errorf("Expected BoolFromOne to be true, got false")
	}
	if config.BoolFromFalse {
		t.Errorf("Expected BoolFromFalse to be false, got true")
	}
	if config.BoolFromZero {
		t.Errorf("Expected BoolFromZero to be false, got true")
	}
}

func TestYamlFeeder_Feed_MapFields(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
connections:
  primary:
    host: "localhost"
    port: 5432
    database: "mydb"
  secondary:
    host: "backup.host"
    port: 5433
    database: "backupdb"
stringMap:
  key1: "value1"
  key2: "value2"
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type DBConnection struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Database string `yaml:"database"`
	}

	type Config struct {
		Connections map[string]DBConnection `yaml:"connections"`
		StringMap   map[string]string       `yaml:"stringMap"`
	}

	var config Config
	feeder := NewYamlFeeder(tempFile.Name())
	err = feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(config.Connections) != 2 {
		t.Errorf("Expected 2 connections, got %d", len(config.Connections))
	}
	if config.Connections["primary"].Host != "localhost" {
		t.Errorf("Expected primary host to be 'localhost', got '%s'", config.Connections["primary"].Host)
	}
	if config.Connections["primary"].Port != 5432 {
		t.Errorf("Expected primary port to be 5432, got %d", config.Connections["primary"].Port)
	}
	if config.Connections["secondary"].Database != "backupdb" {
		t.Errorf("Expected secondary database to be 'backupdb', got '%s'", config.Connections["secondary"].Database)
	}

	if len(config.StringMap) != 2 {
		t.Errorf("Expected 2 string map entries, got %d", len(config.StringMap))
	}
	if config.StringMap["key1"] != "value1" {
		t.Errorf("Expected key1 to be 'value1', got '%s'", config.StringMap["key1"])
	}
}

func TestYamlFeeder_Feed_FieldTracking(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
app:
  name: TestApp
  version: "1.0"
database:
  host: localhost
  port: 5432
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type Config struct {
		App struct {
			Name    string `yaml:"name"`
			Version string `yaml:"version"`
		} `yaml:"app"`
		Database struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		} `yaml:"database"`
		NotFound string `yaml:"notfound"`
	}

	tracker := NewDefaultFieldTracker()
	var config Config
	feeder := NewYamlFeeder(tempFile.Name())
	feeder.SetFieldTracker(tracker)
	err = feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	populations := tracker.GetFieldPopulations()
	if len(populations) == 0 {
		t.Error("Expected field populations to be recorded")
	}

	// Check that we have records for found fields
	foundFields := make(map[string]bool)
	for _, pop := range populations {
		if pop.FoundKey != "" {
			foundFields[pop.FieldPath] = true
		}
	}

	expectedFields := []string{"App.Name", "App.Version", "Database.Host", "Database.Port"}
	for _, field := range expectedFields {
		if !foundFields[field] {
			t.Errorf("Expected field %s to be found and recorded", field)
		}
	}
}

func TestYamlFeeder_Feed_VerboseDebug(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
app:
  name: TestApp
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type Config struct {
		App struct {
			Name string `yaml:"name"`
		} `yaml:"app"`
	}

	logger := &mockLogger{}
	var config Config
	feeder := NewYamlFeeder(tempFile.Name())
	feeder.SetVerboseDebug(true, logger)
	err = feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	messages := logger.getMessages()
	if len(messages) == 0 {
		t.Error("Expected debug messages to be logged")
	}

	// Check for specific debug messages
	found := false
	for _, msg := range messages {
		if strings.Contains(msg, "Starting feed process") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find 'Starting feed process' in debug messages")
	}
}

func TestYamlFeeder_FeedKey(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
app:
  name: TestApp
  version: "1.0"
database:
  host: localhost
  port: 5432
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type AppConfig struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	}

	var appConfig AppConfig
	feeder := NewYamlFeeder(tempFile.Name())
	err = feeder.FeedKey("app", &appConfig)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if appConfig.Name != "TestApp" {
		t.Errorf("Expected Name to be 'TestApp', got '%s'", appConfig.Name)
	}
	if appConfig.Version != "1.0" {
		t.Errorf("Expected Version to be '1.0', got '%s'", appConfig.Version)
	}
}

func TestYamlFeeder_FeedKey_NotFound(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
app:
  name: TestApp
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type NotFoundConfig struct {
		Value string `yaml:"value"`
	}

	var config NotFoundConfig
	feeder := NewYamlFeeder(tempFile.Name())
	err = feeder.FeedKey("notfound", &config)
	if err != nil {
		t.Fatalf("Expected no error for missing key, got %v", err)
	}

	if config.Value != "" {
		t.Errorf("Expected empty value for missing key, got '%s'", config.Value)
	}
}

func TestYamlFeeder_Feed_FileNotFound(t *testing.T) {
	feeder := NewYamlFeeder("/nonexistent/file.yaml")
	var config struct{}
	err := feeder.Feed(&config)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestYamlFeeder_Feed_InvalidYaml(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	invalidYaml := `
app:
  name: TestApp
  version: "1.0"
  invalid: [unclosed array
`
	if _, err := tempFile.Write([]byte(invalidYaml)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type Config struct {
		App struct {
			Name string `yaml:"name"`
		} `yaml:"app"`
	}

	var config Config
	feeder := NewYamlFeeder(tempFile.Name())
	err = feeder.Feed(&config)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestYamlFeeder_Feed_BoolConversionError(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
boolField: "invalid"
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type Config struct {
		BoolField bool `yaml:"boolField"`
	}

	tracker := NewDefaultFieldTracker()
	var config Config
	feeder := NewYamlFeeder(tempFile.Name())
	feeder.SetFieldTracker(tracker)
	err = feeder.Feed(&config)
	if err == nil {
		t.Error("Expected error for invalid bool conversion")
	}
	if !errors.Is(err, ErrYamlBoolConversion) {
		t.Errorf("Expected ErrYamlBoolConversion, got %v", err)
	}
}

func TestYamlFeeder_Feed_IntConversionError(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
intField: "not_a_number"
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type Config struct {
		IntField int `yaml:"intField"`
	}

	tracker := NewDefaultFieldTracker()
	var config Config
	feeder := NewYamlFeeder(tempFile.Name())
	feeder.SetFieldTracker(tracker)
	err = feeder.Feed(&config)
	if err == nil {
		t.Error("Expected error for invalid int conversion")
	}
}

func TestYamlFeeder_Feed_NoFieldTracker(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
app:
  name: TestApp
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type Config struct {
		App struct {
			Name string `yaml:"name"`
		} `yaml:"app"`
	}

	var config Config
	feeder := NewYamlFeeder(tempFile.Name())
	// Don't set field tracker - should use original behavior
	err = feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if config.App.Name != "TestApp" {
		t.Errorf("Expected Name to be 'TestApp', got '%s'", config.App.Name)
	}
}

func TestYamlFeeder_Feed_NonStructPointer(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
- item1
- item2
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	tracker := NewDefaultFieldTracker()
	var config []string
	feeder := NewYamlFeeder(tempFile.Name())
	feeder.SetFieldTracker(tracker)
	err = feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(config) != 2 {
		t.Errorf("Expected 2 items, got %d", len(config))
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
	if feeder.logger != nil {
		t.Error("Expected logger to be nil by default")
	}
	if feeder.fieldTracker != nil {
		t.Error("Expected fieldTracker to be nil by default")
	}
}

func TestYamlFeeder_SetVerboseDebug(t *testing.T) {
	feeder := NewYamlFeeder("/test/path.yaml")
	logger := &mockLogger{}

	feeder.SetVerboseDebug(true, logger)

	if !feeder.verboseDebug {
		t.Error("Expected verboseDebug to be true")
	}
	if feeder.logger != logger {
		t.Error("Expected logger to be set")
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

	if feeder.fieldTracker != tracker {
		t.Error("Expected fieldTracker to be set")
	}
}
