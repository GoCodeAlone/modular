//go:build failing_test

package modular

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplicationOptions tests application option configuration behavior
func TestApplicationOptions_DynamicReload(t *testing.T) {
	tests := []struct {
		name           string
		options        []ApplicationOption
		expectedReload bool
		expectedConfig *DynamicReloadConfig
	}{
		{
			name:           "no options results in default configuration",
			options:        []ApplicationOption{},
			expectedReload: false,
			expectedConfig: nil,
		},
		{
			name: "with dynamic reload option enables reload",
			options: []ApplicationOption{
				WithDynamicReload(DynamicReloadConfig{
					Enabled:       true,
					ReloadTimeout: 30 * time.Second,
				}),
			},
			expectedReload: true,
			expectedConfig: &DynamicReloadConfig{
				Enabled:       true,
				ReloadTimeout: 30 * time.Second,
			},
		},
		{
			name: "with disabled dynamic reload option",
			options: []ApplicationOption{
				WithDynamicReload(DynamicReloadConfig{
					Enabled:       false,
					ReloadTimeout: 10 * time.Second,
				}),
			},
			expectedReload: false,
			expectedConfig: &DynamicReloadConfig{
				Enabled:       false,
				ReloadTimeout: 10 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create application builder and apply options
			builder := NewTestApplicationBuilder()
			for _, option := range tt.options {
				builder.AddOption(option)
			}

			// Build application configuration
			config := builder.GetApplicationConfig()

			// Verify dynamic reload configuration
			if tt.expectedReload {
				require.NotNil(t, config.DynamicReload, "Dynamic reload config should be set")
				assert.Equal(t, tt.expectedConfig.Enabled, config.DynamicReload.Enabled)
				assert.Equal(t, tt.expectedConfig.ReloadTimeout, config.DynamicReload.ReloadTimeout)
			} else {
				if config.DynamicReload != nil {
					assert.False(t, config.DynamicReload.Enabled, "Dynamic reload should be disabled")
				}
			}
		})
	}
}

// TestApplicationOptions_HealthAggregation tests health aggregation option behavior  
func TestApplicationOptions_HealthAggregation(t *testing.T) {
	tests := []struct {
		name           string
		options        []ApplicationOption
		expectedHealth bool
		expectedConfig *HealthAggregatorConfig
	}{
		{
			name:           "no options results in default configuration",
			options:        []ApplicationOption{},
			expectedHealth: false,
			expectedConfig: nil,
		},
		{
			name: "with health aggregation option enables health checks",
			options: []ApplicationOption{
				WithHealthAggregator(HealthAggregatorConfig{
					Enabled:       true,
					CheckInterval: 10 * time.Second,
					CheckTimeout:  5 * time.Second,
				}),
			},
			expectedHealth: true,
			expectedConfig: &HealthAggregatorConfig{
				Enabled:       true,
				CheckInterval: 10 * time.Second,
				CheckTimeout:  5 * time.Second,
			},
		},
		{
			name: "with disabled health aggregation option",
			options: []ApplicationOption{
				WithHealthAggregator(HealthAggregatorConfig{
					Enabled:       false,
					CheckInterval: 15 * time.Second,
					CheckTimeout:  3 * time.Second,
				}),
			},
			expectedHealth: false,
			expectedConfig: &HealthAggregatorConfig{
				Enabled:       false,
				CheckInterval: 15 * time.Second,
				CheckTimeout:  3 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create application builder and apply options
			builder := NewTestApplicationBuilder()
			for _, option := range tt.options {
				builder.AddOption(option)
			}

			// Build application configuration
			config := builder.GetApplicationConfig()

			// Verify health aggregation configuration
			if tt.expectedHealth {
				require.NotNil(t, config.HealthAggregator, "Health aggregator config should be set")
				assert.Equal(t, tt.expectedConfig.Enabled, config.HealthAggregator.Enabled)
				assert.Equal(t, tt.expectedConfig.CheckInterval, config.HealthAggregator.CheckInterval)
				assert.Equal(t, tt.expectedConfig.CheckTimeout, config.HealthAggregator.CheckTimeout)
			} else {
				if config.HealthAggregator != nil {
					assert.False(t, config.HealthAggregator.Enabled, "Health aggregation should be disabled")
				}
			}
		})
	}
}

// TestApplicationOptions_CombinedOptions tests combining multiple options
func TestApplicationOptions_CombinedOptions(t *testing.T) {
	t.Run("should support multiple options together", func(t *testing.T) {
		reloadConfig := DynamicReloadConfig{
			Enabled:       true,
			ReloadTimeout: 45 * time.Second,
		}

		healthConfig := HealthAggregatorConfig{
			Enabled:       true,
			CheckInterval: 20 * time.Second,
			CheckTimeout:  10 * time.Second,
		}

		options := []ApplicationOption{
			WithDynamicReload(reloadConfig),
			WithHealthAggregator(healthConfig),
		}

		builder := NewTestApplicationBuilder()
		for _, option := range options {
			builder.AddOption(option)
		}

		config := builder.GetApplicationConfig()

		// Verify both options are configured
		require.NotNil(t, config.DynamicReload, "Dynamic reload should be configured")
		assert.True(t, config.DynamicReload.Enabled)
		assert.Equal(t, 45*time.Second, config.DynamicReload.ReloadTimeout)

		require.NotNil(t, config.HealthAggregator, "Health aggregator should be configured")
		assert.True(t, config.HealthAggregator.Enabled)
		assert.Equal(t, 20*time.Second, config.HealthAggregator.CheckInterval)
		assert.Equal(t, 10*time.Second, config.HealthAggregator.CheckTimeout)
	})
}

// TestApplicationOptions_OptionOverriding tests option override behavior
func TestApplicationOptions_OptionOverriding(t *testing.T) {
	t.Run("should allow later options to override earlier ones", func(t *testing.T) {
		firstReloadConfig := DynamicReloadConfig{
			Enabled:       true,
			ReloadTimeout: 30 * time.Second,
		}

		secondReloadConfig := DynamicReloadConfig{
			Enabled:       false,
			ReloadTimeout: 60 * time.Second,
		}

		options := []ApplicationOption{
			WithDynamicReload(firstReloadConfig),
			WithDynamicReload(secondReloadConfig), // Should override the first
		}

		builder := NewTestApplicationBuilder()
		for _, option := range options {
			builder.AddOption(option)
		}

		config := builder.GetApplicationConfig()

		// Should use the second configuration
		require.NotNil(t, config.DynamicReload)
		assert.False(t, config.DynamicReload.Enabled, "Should use second config's Enabled value")
		assert.Equal(t, 60*time.Second, config.DynamicReload.ReloadTimeout, "Should use second config's timeout")
	})
}

// Test helper implementations

// ApplicationConfig represents application configuration
type ApplicationConfig struct {
	DynamicReload     *DynamicReloadConfig     `json:"dynamic_reload,omitempty"`
	HealthAggregator  *HealthAggregatorConfig  `json:"health_aggregator,omitempty"`
}

// DynamicReloadConfig configures dynamic reload behavior
type DynamicReloadConfig struct {
	Enabled       bool          `json:"enabled"`
	ReloadTimeout time.Duration `json:"reload_timeout"`
}

// HealthAggregatorConfig configures health aggregation
type HealthAggregatorConfig struct {
	Enabled       bool          `json:"enabled"`
	CheckInterval time.Duration `json:"check_interval"`
	CheckTimeout  time.Duration `json:"check_timeout"`
}

// ApplicationOption represents a configuration option for the application
type ApplicationOption func(*ApplicationConfig)

// WithDynamicReload configures dynamic reload functionality
func WithDynamicReload(config DynamicReloadConfig) ApplicationOption {
	return func(appConfig *ApplicationConfig) {
		appConfig.DynamicReload = &config
	}
}

// WithHealthAggregator configures health aggregation functionality
func WithHealthAggregator(config HealthAggregatorConfig) ApplicationOption {
	return func(appConfig *ApplicationConfig) {
		appConfig.HealthAggregator = &config
	}
}

// TestApplicationBuilder helps build applications with options for testing
type TestApplicationBuilder struct {
	config *ApplicationConfig
}

func NewTestApplicationBuilder() *TestApplicationBuilder {
	return &TestApplicationBuilder{
		config: &ApplicationConfig{},
	}
}

func (b *TestApplicationBuilder) AddOption(option ApplicationOption) {
	option(b.config)
}

func (b *TestApplicationBuilder) GetApplicationConfig() *ApplicationConfig {
	return b.config
}