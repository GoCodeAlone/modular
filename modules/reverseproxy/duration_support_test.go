package reverseproxy

import (
	"os"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular/feeders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReverseProxyConfig_TimeDurationSupport(t *testing.T) {
	t.Run("EnvFeeder", func(t *testing.T) {
		// Clean up environment
		os.Unsetenv("REQUEST_TIMEOUT")
		os.Unsetenv("CACHE_TTL")

		// Set environment variables
		os.Setenv("REQUEST_TIMEOUT", "30s")
		os.Setenv("CACHE_TTL", "5m")
		defer func() {
			os.Unsetenv("REQUEST_TIMEOUT")
			os.Unsetenv("CACHE_TTL")
		}()

		config := &ReverseProxyConfig{}
		feeder := feeders.NewEnvFeeder()
		
		// Test with verbose debug enabled (reproducing the original issue scenario)
		logger := &testDebugLogger{}
		feeder.SetVerboseDebug(true, logger)
		
		err := feeder.Feed(config)
		require.NoError(t, err)
		assert.Equal(t, 30*time.Second, config.RequestTimeout)
		assert.Equal(t, 5*time.Minute, config.CacheTTL)
		
		// Verify debug logging occurred
		assert.Greater(t, len(logger.messages), 0)
	})

	t.Run("YamlFeeder", func(t *testing.T) {
		yamlContent := `request_timeout: 45s
cache_ttl: 10m
backend_services:
  service1: "http://localhost:8080"
routes:
  "/api": "service1"
default_backend: "service1"
cache_enabled: true
metrics_enabled: true
metrics_path: "/metrics"`
		
		yamlFile := "/tmp/reverseproxy_test.yaml"
		err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
		require.NoError(t, err)
		defer os.Remove(yamlFile)
		
		config := &ReverseProxyConfig{}
		feeder := feeders.NewYamlFeeder(yamlFile)
		
		// Test with verbose debug enabled
		logger := &testDebugLogger{}
		feeder.SetVerboseDebug(true, logger)
		
		err = feeder.Feed(config)
		require.NoError(t, err)
		assert.Equal(t, 45*time.Second, config.RequestTimeout)
		assert.Equal(t, 10*time.Minute, config.CacheTTL)
		assert.True(t, config.CacheEnabled)
		assert.True(t, config.MetricsEnabled)
		assert.Equal(t, "/metrics", config.MetricsPath)
	})

	t.Run("JSONFeeder", func(t *testing.T) {
		jsonContent := `{
  "request_timeout": "1h",
  "cache_ttl": "15m",
  "backend_services": {
    "service1": "http://localhost:8080"
  },
  "routes": {
    "/api": "service1"
  },
  "default_backend": "service1",
  "cache_enabled": true,
  "metrics_enabled": true,
  "metrics_path": "/metrics"
}`
		
		jsonFile := "/tmp/reverseproxy_test.json"
		err := os.WriteFile(jsonFile, []byte(jsonContent), 0644)
		require.NoError(t, err)
		defer os.Remove(jsonFile)
		
		config := &ReverseProxyConfig{}
		feeder := feeders.NewJSONFeeder(jsonFile)
		
		// Test with verbose debug enabled
		logger := &testDebugLogger{}
		feeder.SetVerboseDebug(true, logger)
		
		err = feeder.Feed(config)
		require.NoError(t, err)
		assert.Equal(t, 1*time.Hour, config.RequestTimeout)
		assert.Equal(t, 15*time.Minute, config.CacheTTL)
		assert.True(t, config.CacheEnabled)
	})

	t.Run("TomlFeeder", func(t *testing.T) {
		tomlContent := `request_timeout = "2h"
cache_ttl = "30m"
cache_enabled = true
metrics_enabled = true
metrics_path = "/metrics"
default_backend = "service1"

[backend_services]
service1 = "http://localhost:8080"

[routes]
"/api" = "service1"`
		
		tomlFile := "/tmp/reverseproxy_test.toml"
		err := os.WriteFile(tomlFile, []byte(tomlContent), 0644)
		require.NoError(t, err)
		defer os.Remove(tomlFile)
		
		config := &ReverseProxyConfig{}
		feeder := feeders.NewTomlFeeder(tomlFile)
		
		// Test with verbose debug enabled
		logger := &testDebugLogger{}
		feeder.SetVerboseDebug(true, logger)
		
		err = feeder.Feed(config)
		require.NoError(t, err)
		assert.Equal(t, 2*time.Hour, config.RequestTimeout)
		assert.Equal(t, 30*time.Minute, config.CacheTTL)
		assert.True(t, config.CacheEnabled)
	})
}

func TestReverseProxyConfig_TimeDurationInvalidFormat(t *testing.T) {
	t.Run("EnvFeeder_InvalidDuration", func(t *testing.T) {
		os.Setenv("REQUEST_TIMEOUT", "invalid_duration")
		defer os.Unsetenv("REQUEST_TIMEOUT")

		config := &ReverseProxyConfig{}
		feeder := feeders.NewEnvFeeder()
		err := feeder.Feed(config)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot convert value to type time.Duration")
	})

	t.Run("YamlFeeder_InvalidDuration", func(t *testing.T) {
		yamlContent := `request_timeout: invalid_duration`
		
		yamlFile := "/tmp/invalid_reverseproxy_test.yaml"
		err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
		require.NoError(t, err)
		defer os.Remove(yamlFile)
		
		config := &ReverseProxyConfig{}
		feeder := feeders.NewYamlFeeder(yamlFile)
		err = feeder.Feed(config)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot convert string 'invalid_duration' to time.Duration")
	})
}

// testDebugLogger captures debug messages for verification
type testDebugLogger struct {
	messages []string
}

func (l *testDebugLogger) Debug(msg string, args ...any) {
	l.messages = append(l.messages, msg)
}