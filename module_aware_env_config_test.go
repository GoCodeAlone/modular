package modular

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModuleAwareEnvironmentVariableSearching tests the new module-aware environment variable search functionality
func TestModuleAwareEnvironmentVariableSearching(t *testing.T) {
	t.Run("reverseproxy_module_env_var_priority", func(t *testing.T) {
		type ReverseProxyConfig struct {
			DryRun         bool   `env:"MODTEST_DRY_RUN"`
			DefaultBackend string `env:"MODTEST_DEFAULT_BACKEND"`
			RequestTimeout int    `env:"MODTEST_REQUEST_TIMEOUT"`
		}

		// Clear all relevant environment variables (using unique test prefix)
		envVars := []string{
			"MODTEST_DRY_RUN", "REVERSEPROXY_MODTEST_DRY_RUN", "MODTEST_DRY_RUN_REVERSEPROXY",
			"MODTEST_DEFAULT_BACKEND", "REVERSEPROXY_MODTEST_DEFAULT_BACKEND", "MODTEST_DEFAULT_BACKEND_REVERSEPROXY",
			"MODTEST_REQUEST_TIMEOUT", "REVERSEPROXY_MODTEST_REQUEST_TIMEOUT", "MODTEST_REQUEST_TIMEOUT_REVERSEPROXY",
		}
		for _, env := range envVars {
			os.Unsetenv(env)
		}

		t.Run("module_prefix_takes_priority", func(t *testing.T) {
			// Set up all variants to test priority
			testEnvVars := map[string]string{
				"REVERSEPROXY_MODTEST_DRY_RUN": "true",  // Should win (highest priority)
				"MODTEST_DRY_RUN_REVERSEPROXY": "false", // Lower priority
				"MODTEST_DRY_RUN":              "false", // Lowest priority
			}

			for key, value := range testEnvVars {
				err := os.Setenv(key, value)
				require.NoError(t, err)
			}

			defer func() {
				for key := range testEnvVars {
					os.Unsetenv(key)
				}
			}()

			// Create a simple application to test module config
			app := createTestApplication(t)

			// Register a mock module with config
			mockModule := &mockModuleAwareConfigModule{
				name:   "reverseproxy",
				config: &ReverseProxyConfig{},
			}
			app.RegisterModule(mockModule)

			// Initialize the application to trigger config loading
			err := app.Init()
			require.NoError(t, err)

			// Verify that the module prefix took priority (DryRun should be true)
			config := mockModule.config.(*ReverseProxyConfig)
			assert.True(t, config.DryRun)
		})

		t.Run("module_suffix_fallback", func(t *testing.T) {
			// Clear all environment variables first
			for _, env := range envVars {
				os.Unsetenv(env)
			}

			// Set up suffix and base variants only (no prefix)
			testEnvVars := map[string]string{
				"MODTEST_DRY_RUN_REVERSEPROXY": "true",  // Should win (higher priority than base)
				"MODTEST_DRY_RUN":              "false", // Lower priority
			}

			for key, value := range testEnvVars {
				err := os.Setenv(key, value)
				require.NoError(t, err)
			}

			defer func() {
				for key := range testEnvVars {
					os.Unsetenv(key)
				}
			}()

			// Create a simple application to test module config
			app := createTestApplication(t)

			// Register a mock module with config
			mockModule := &mockModuleAwareConfigModule{
				name:   "reverseproxy",
				config: &ReverseProxyConfig{},
			}
			app.RegisterModule(mockModule)

			// Initialize the application to trigger config loading
			err := app.Init()
			require.NoError(t, err)

			// Verify that the module suffix took priority (DryRun should be true)
			config := mockModule.config.(*ReverseProxyConfig)
			assert.True(t, config.DryRun)
		})

		t.Run("base_env_var_fallback", func(t *testing.T) {
			// Clear all environment variables first
			for _, env := range envVars {
				os.Unsetenv(env)
			}

			// Set up only base variant
			testEnvVars := map[string]string{
				"MODTEST_DRY_RUN": "true", // Should be used as last resort
			}

			for key, value := range testEnvVars {
				err := os.Setenv(key, value)
				require.NoError(t, err)
			}

			defer func() {
				for key := range testEnvVars {
					os.Unsetenv(key)
				}
			}()

			// Create a simple application to test module config
			app := createTestApplication(t)

			// Register a mock module with config
			mockModule := &mockModuleAwareConfigModule{
				name:   "reverseproxy",
				config: &ReverseProxyConfig{},
			}
			app.RegisterModule(mockModule)

			// Initialize the application to trigger config loading
			err := app.Init()
			require.NoError(t, err)

			// Verify that the base env var was used (DryRun should be true)
			config := mockModule.config.(*ReverseProxyConfig)
			assert.True(t, config.DryRun)
		})

		t.Run("multiple_fields_with_mixed_env_vars", func(t *testing.T) {
			// Clear all environment variables first
			for _, env := range envVars {
				os.Unsetenv(env)
			}

			// Set up mixed variants to test all fields
			testEnvVars := map[string]string{
				"REVERSEPROXY_MODTEST_DRY_RUN":         "true",                // Prefix for first field
				"MODTEST_DEFAULT_BACKEND_REVERSEPROXY": "backend.example.com", // Suffix for second field
				"MODTEST_REQUEST_TIMEOUT":              "5000",                // Base for third field
			}

			for key, value := range testEnvVars {
				err := os.Setenv(key, value)
				require.NoError(t, err)
			}

			defer func() {
				for key := range testEnvVars {
					os.Unsetenv(key)
				}
			}()

			// Create a simple application to test module config
			app := createTestApplication(t)

			// Register a mock module with config
			mockModule := &mockModuleAwareConfigModule{
				name:   "reverseproxy",
				config: &ReverseProxyConfig{},
			}
			app.RegisterModule(mockModule)

			// Initialize the application to trigger config loading
			err := app.Init()
			require.NoError(t, err)

			// Verify that each field got the correct value from its respective env var
			config := mockModule.config.(*ReverseProxyConfig)
			assert.True(t, config.DryRun)                                 // From REVERSEPROXY_DRY_RUN
			assert.Equal(t, "backend.example.com", config.DefaultBackend) // From DEFAULT_BACKEND_REVERSEPROXY
			assert.Equal(t, 5000, config.RequestTimeout)                  // From REQUEST_TIMEOUT
		})
	})

	t.Run("httpserver_module_env_var_priority", func(t *testing.T) {
		type HTTPServerConfig struct {
			Host string `env:"MODTEST_HOST"`
			Port int    `env:"MODTEST_PORT"`
		}

		// Clear all relevant environment variables (using unique test prefix)
		envVars := []string{
			"MODTEST_HOST", "HTTPSERVER_MODTEST_HOST", "MODTEST_HOST_HTTPSERVER",
			"MODTEST_PORT", "HTTPSERVER_MODTEST_PORT", "MODTEST_PORT_HTTPSERVER",
		}
		for _, env := range envVars {
			os.Unsetenv(env)
		}

		t.Run("module_prefix_for_httpserver", func(t *testing.T) {
			// Set up environment variables
			testEnvVars := map[string]string{
				"HTTPSERVER_MODTEST_HOST": "api.example.com", // Should win (highest priority)
				"MODTEST_HOST_HTTPSERVER": "alt.example.com", // Lower priority
				"MODTEST_HOST":            "localhost",       // Lowest priority
				"HTTPSERVER_MODTEST_PORT": "9090",            // Should win (highest priority)
				"MODTEST_PORT":            "8080",            // Lowest priority
			}

			for key, value := range testEnvVars {
				err := os.Setenv(key, value)
				require.NoError(t, err)
			}

			defer func() {
				for key := range testEnvVars {
					os.Unsetenv(key)
				}
			}()

			// Create a simple application to test module config
			app := createTestApplication(t)

			// Register a mock module with config
			mockModule := &mockModuleAwareConfigModule{
				name:   "httpserver",
				config: &HTTPServerConfig{},
			}
			app.RegisterModule(mockModule)

			// Initialize the application to trigger config loading
			err := app.Init()
			require.NoError(t, err)

			// Verify that the module prefix took priority
			httpConfig := mockModule.config.(*HTTPServerConfig)
			assert.Equal(t, "api.example.com", httpConfig.Host)
			assert.Equal(t, 9090, httpConfig.Port)
		})
	})

	t.Run("backward_compatibility", func(t *testing.T) {
		type SimpleConfig struct {
			Value string `env:"MODTEST_SIMPLE_VALUE"`
		}

		// Clear environment variables (using unique test prefix)
		envVars := []string{"MODTEST_SIMPLE_VALUE", "TESTMODULE_MODTEST_SIMPLE_VALUE", "MODTEST_SIMPLE_VALUE_TESTMODULE"}
		for _, env := range envVars {
			os.Unsetenv(env)
		}

		// Set up only the base environment variable (old behavior)
		err := os.Setenv("MODTEST_SIMPLE_VALUE", "original_behavior")
		require.NoError(t, err)
		defer os.Unsetenv("MODTEST_SIMPLE_VALUE")

		// Create application with a module that doesn't use module-aware config
		app := createTestApplication(t)

		// Register a mock module
		mockModule := &mockModuleAwareConfigModule{
			name:   "testmodule",
			config: &SimpleConfig{},
		}
		app.RegisterModule(mockModule)

		// Initialize the application
		err = app.Init()
		require.NoError(t, err)

		// Verify that backward compatibility is maintained
		simpleConfig := mockModule.config.(*SimpleConfig)
		assert.Equal(t, "original_behavior", simpleConfig.Value)
	})
}

// mockModuleAwareConfigModule is a mock module for testing module-aware configuration
type mockModuleAwareConfigModule struct {
	name   string
	config interface{}
}

func (m *mockModuleAwareConfigModule) Name() string {
	return m.name
}

func (m *mockModuleAwareConfigModule) RegisterConfig(app Application) error {
	app.RegisterConfigSection(m.Name(), NewStdConfigProvider(m.config))
	return nil
}

func (m *mockModuleAwareConfigModule) Init(app Application) error {
	// Get the config section to populate our local config reference
	cfg, err := app.GetConfigSection(m.Name())
	if err != nil {
		return fmt.Errorf("failed to get config section for module %s: %w", m.Name(), err)
	}
	m.config = cfg.GetConfig()
	return nil
}

// createTestApplication creates a basic application for testing
func createTestApplication(t *testing.T) *StdApplication {
	logger := &simpleTestLogger{}
	app := NewStdApplication(nil, logger)
	return app.(*StdApplication)
}

// simpleTestLogger is a simple logger implementation for tests
type simpleTestLogger struct {
	messages []string
}

func (l *simpleTestLogger) Debug(msg string, args ...any) {
	l.messages = append(l.messages, msg)
}

func (l *simpleTestLogger) Info(msg string, args ...any) {
	l.messages = append(l.messages, msg)
}

func (l *simpleTestLogger) Warn(msg string, args ...any) {
	l.messages = append(l.messages, msg)
}

func (l *simpleTestLogger) Error(msg string, args ...any) {
	l.messages = append(l.messages, msg)
}
