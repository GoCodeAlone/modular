package feeders

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DurationTestConfig represents a configuration with time.Duration fields
type DurationTestConfig struct {
	RequestTimeout time.Duration  `env:"REQUEST_TIMEOUT" yaml:"request_timeout" json:"request_timeout" toml:"request_timeout"`
	CacheTTL       time.Duration  `env:"CACHE_TTL" yaml:"cache_ttl" json:"cache_ttl" toml:"cache_ttl"`
	PointerTimeout *time.Duration `env:"POINTER_TIMEOUT" yaml:"pointer_timeout" json:"pointer_timeout" toml:"pointer_timeout"`
}

func TestEnvFeeder_TimeDuration(t *testing.T) {
	tests := []struct {
		name           string
		requestTimeout string
		cacheTTL       string
		pointerTimeout string
		expectTimeout  time.Duration
		expectTTL      time.Duration
		expectPointer  *time.Duration
		shouldError    bool
	}{
		{
			name:           "valid durations",
			requestTimeout: "30s",
			cacheTTL:       "5m",
			pointerTimeout: "1h",
			expectTimeout:  30 * time.Second,
			expectTTL:      5 * time.Minute,
			expectPointer:  func() *time.Duration { d := 1 * time.Hour; return &d }(),
		},
		{
			name:           "complex durations",
			requestTimeout: "2h30m45s",
			cacheTTL:       "15m30s",
			pointerTimeout: "500ms",
			expectTimeout:  2*time.Hour + 30*time.Minute + 45*time.Second,
			expectTTL:      15*time.Minute + 30*time.Second,
			expectPointer:  func() *time.Duration { d := 500 * time.Millisecond; return &d }(),
		},
		{
			name:          "invalid duration format",
			requestTimeout: "invalid",
			shouldError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment
			os.Unsetenv("REQUEST_TIMEOUT")
			os.Unsetenv("CACHE_TTL")
			os.Unsetenv("POINTER_TIMEOUT")

			// Set environment variables
			if tt.requestTimeout != "" {
				os.Setenv("REQUEST_TIMEOUT", tt.requestTimeout)
			}
			if tt.cacheTTL != "" {
				os.Setenv("CACHE_TTL", tt.cacheTTL)
			}
			if tt.pointerTimeout != "" {
				os.Setenv("POINTER_TIMEOUT", tt.pointerTimeout)
			}

			config := &DurationTestConfig{}
			feeder := NewEnvFeeder()
			err := feeder.Feed(config)

			if tt.shouldError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectTimeout, config.RequestTimeout)
			assert.Equal(t, tt.expectTTL, config.CacheTTL)
			if tt.expectPointer != nil {
				require.NotNil(t, config.PointerTimeout)
				assert.Equal(t, *tt.expectPointer, *config.PointerTimeout)
			}
		})
	}
}

func TestEnvFeeder_TimeDuration_VerboseDebug(t *testing.T) {
	os.Setenv("REQUEST_TIMEOUT", "30s")
	defer os.Unsetenv("REQUEST_TIMEOUT")

	config := &DurationTestConfig{}
	feeder := NewEnvFeeder()
	
	// Create a simple logger for testing
	logger := &testLogger{messages: make([]string, 0)}
	feeder.SetVerboseDebug(true, logger)

	err := feeder.Feed(config)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, config.RequestTimeout)
	
	// Check that debug logging occurred
	assert.Greater(t, len(logger.messages), 0)
}

func TestYamlFeeder_TimeDuration(t *testing.T) {
	// Create test YAML file
	yamlContent := `request_timeout: 45s
cache_ttl: 10m
pointer_timeout: 2h`
	
	yamlFile := "/tmp/test_duration.yaml"
	err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	require.NoError(t, err)
	defer os.Remove(yamlFile)

	config := &DurationTestConfig{}
	feeder := NewYamlFeeder(yamlFile)
	err = feeder.Feed(config)

	require.NoError(t, err)
	assert.Equal(t, 45*time.Second, config.RequestTimeout)
	assert.Equal(t, 10*time.Minute, config.CacheTTL)
	require.NotNil(t, config.PointerTimeout)
	assert.Equal(t, 2*time.Hour, *config.PointerTimeout)
}

func TestYamlFeeder_TimeDuration_InvalidFormat(t *testing.T) {
	yamlContent := `request_timeout: invalid_duration`
	
	yamlFile := "/tmp/test_invalid_duration.yaml"
	err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	require.NoError(t, err)
	defer os.Remove(yamlFile)

	config := &DurationTestConfig{}
	feeder := NewYamlFeeder(yamlFile)
	err = feeder.Feed(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot convert string 'invalid_duration' to time.Duration")
}

func TestJSONFeeder_TimeDuration(t *testing.T) {
	jsonContent := `{"request_timeout": "1h", "cache_ttl": "15m", "pointer_timeout": "3h30m"}`
	
	jsonFile := "/tmp/test_duration.json"
	err := os.WriteFile(jsonFile, []byte(jsonContent), 0644)
	require.NoError(t, err)
	defer os.Remove(jsonFile)

	config := &DurationTestConfig{}
	feeder := NewJSONFeeder(jsonFile)
	err = feeder.Feed(config)

	require.NoError(t, err)
	assert.Equal(t, 1*time.Hour, config.RequestTimeout)
	assert.Equal(t, 15*time.Minute, config.CacheTTL)
	require.NotNil(t, config.PointerTimeout)
	assert.Equal(t, 3*time.Hour+30*time.Minute, *config.PointerTimeout)
}

func TestJSONFeeder_TimeDuration_InvalidFormat(t *testing.T) {
	jsonContent := `{"request_timeout": "bad_duration"}`
	
	jsonFile := "/tmp/test_invalid_duration.json"
	err := os.WriteFile(jsonFile, []byte(jsonContent), 0644)
	require.NoError(t, err)
	defer os.Remove(jsonFile)

	config := &DurationTestConfig{}
	feeder := NewJSONFeeder(jsonFile)
	err = feeder.Feed(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot convert string 'bad_duration' to time.Duration")
}

func TestTomlFeeder_TimeDuration(t *testing.T) {
	tomlContent := `request_timeout = "2h"
cache_ttl = "30m"
pointer_timeout = "45m"`
	
	tomlFile := "/tmp/test_duration.toml"
	err := os.WriteFile(tomlFile, []byte(tomlContent), 0644)
	require.NoError(t, err)
	defer os.Remove(tomlFile)

	config := &DurationTestConfig{}
	feeder := NewTomlFeeder(tomlFile)
	err = feeder.Feed(config)

	require.NoError(t, err)
	assert.Equal(t, 2*time.Hour, config.RequestTimeout)
	assert.Equal(t, 30*time.Minute, config.CacheTTL)
	require.NotNil(t, config.PointerTimeout)
	assert.Equal(t, 45*time.Minute, *config.PointerTimeout)
}

func TestTomlFeeder_TimeDuration_InvalidFormat(t *testing.T) {
	tomlContent := `request_timeout = "invalid"`
	
	tomlFile := "/tmp/test_invalid_duration.toml"
	err := os.WriteFile(tomlFile, []byte(tomlContent), 0644)
	require.NoError(t, err)
	defer os.Remove(tomlFile)

	config := &DurationTestConfig{}
	feeder := NewTomlFeeder(tomlFile)
	err = feeder.Feed(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot convert string 'invalid' to time.Duration")
}

func TestAllFeeders_TimeDuration_VerboseDebug(t *testing.T) {
	// Test that verbose debug logging works for all feeders with time.Duration
	logger := &testLogger{messages: make([]string, 0)}

	// Test EnvFeeder
	os.Setenv("REQUEST_TIMEOUT", "10s")
	defer os.Unsetenv("REQUEST_TIMEOUT")

	config1 := &DurationTestConfig{}
	envFeeder := NewEnvFeeder()
	envFeeder.SetVerboseDebug(true, logger)
	err := envFeeder.Feed(config1)
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, config1.RequestTimeout)

	// Test YamlFeeder
	yamlContent := `request_timeout: 20s`
	yamlFile := "/tmp/test_verbose_debug.yaml"
	err = os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	require.NoError(t, err)
	defer os.Remove(yamlFile)

	config2 := &DurationTestConfig{}
	yamlFeeder := NewYamlFeeder(yamlFile)
	yamlFeeder.SetVerboseDebug(true, logger)
	err = yamlFeeder.Feed(config2)
	require.NoError(t, err)
	assert.Equal(t, 20*time.Second, config2.RequestTimeout)

	// Test JSONFeeder
	jsonContent := `{"request_timeout": "30s"}`
	jsonFile := "/tmp/test_verbose_debug.json"
	err = os.WriteFile(jsonFile, []byte(jsonContent), 0644)
	require.NoError(t, err)
	defer os.Remove(jsonFile)

	config3 := &DurationTestConfig{}
	jsonFeeder := NewJSONFeeder(jsonFile)
	jsonFeeder.SetVerboseDebug(true, logger)
	err = jsonFeeder.Feed(config3)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, config3.RequestTimeout)

	// Test TomlFeeder
	tomlContent := `request_timeout = "40s"`
	tomlFile := "/tmp/test_verbose_debug.toml"
	err = os.WriteFile(tomlFile, []byte(tomlContent), 0644)
	require.NoError(t, err)
	defer os.Remove(tomlFile)

	config4 := &DurationTestConfig{}
	tomlFeeder := NewTomlFeeder(tomlFile)
	tomlFeeder.SetVerboseDebug(true, logger)
	err = tomlFeeder.Feed(config4)
	require.NoError(t, err)
	assert.Equal(t, 40*time.Second, config4.RequestTimeout)

	// Check that debug logging occurred
	assert.Greater(t, len(logger.messages), 0)
}

// testLogger is a simple logger implementation for testing
type testLogger struct {
	messages []string
}

func (l *testLogger) Debug(msg string, args ...any) {
	l.messages = append(l.messages, msg)
}