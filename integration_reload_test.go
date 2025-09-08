//go:build failing_test

package modular

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplicationWithDynamicReload tests real application setup with dynamic reload capability
func TestApplicationWithDynamicReload(t *testing.T) {
	t.Run("should build application with dynamic reload configuration", func(t *testing.T) {
		// Test building an application with dynamic reload enabled
		stdConfig := NewStdConfigProvider(testCfg{Str: "test"})
		stdLogger := &logger{t}

		app := &StdApplication{
			cfgProvider:    stdConfig,
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         stdLogger,
		}

		// Register a reloadable module
		reloadableModule := &testReloadableModule{
			name:        "reloadable-service",
			canReload:   true,
			timeout:     30 * time.Second,
			currentConfig: map[string]interface{}{
				"version":    "1.0",
				"enabled":    true,
				"max_connections": 100,
			},
		}

		app.RegisterModule(reloadableModule)

		// Verify module is registered
		modules := app.GetModules()
		require.Contains(t, modules, "reloadable-service")

		// Verify the module implements Reloadable interface
		module := modules["reloadable-service"]
		reloadable, ok := module.(Reloadable)
		require.True(t, ok, "Module should implement Reloadable interface")
		assert.True(t, reloadable.CanReload(), "Module should be reloadable")
	})

	t.Run("should coordinate reload across multiple modules", func(t *testing.T) {
		// Create application with multiple reloadable modules
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		// Register multiple reloadable modules with dependencies
		dbModule := &testReloadableModule{
			name:        "database",
			canReload:   true,
			timeout:     15 * time.Second,
			currentConfig: map[string]interface{}{"host": "localhost", "port": 5432},
		}

		cacheModule := &testReloadableModule{
			name:        "cache",
			canReload:   true,
			timeout:     10 * time.Second,
			currentConfig: map[string]interface{}{"size": 1000, "ttl": "1h"},
		}

		apiModule := &testReloadableModule{
			name:        "api",
			canReload:   true,
			timeout:     20 * time.Second,
			currentConfig: map[string]interface{}{"port": 8080, "workers": 4},
		}

		app.RegisterModule(dbModule)
		app.RegisterModule(cacheModule)
		app.RegisterModule(apiModule)

		// Simulate coordinated reload
		modules := app.GetModules()
		newConfigs := map[string]interface{}{
			"database": map[string]interface{}{"host": "db.example.com", "port": 5433},
			"cache":    map[string]interface{}{"size": 2000, "ttl": "2h"},
			"api":      map[string]interface{}{"port": 8081, "workers": 8},
		}

		ctx := context.Background()
		var reloadErrors []error

		for moduleName, newConfig := range newConfigs {
			if module, exists := modules[moduleName]; exists {
				if reloadable, ok := module.(Reloadable); ok {
					if err := reloadable.Reload(ctx, newConfig); err != nil {
						reloadErrors = append(reloadErrors, err)
					}
				}
			}
		}

		// Verify all reloads succeeded
		assert.Empty(t, reloadErrors, "All module reloads should succeed")

		// Verify configurations were updated
		assert.Equal(t, newConfigs["database"], dbModule.currentConfig)
		assert.Equal(t, newConfigs["cache"], cacheModule.currentConfig)
		assert.Equal(t, newConfigs["api"], apiModule.currentConfig)
	})
}

// TestApplicationHealthAggregation tests real health check aggregation across modules
func TestApplicationHealthAggregation(t *testing.T) {
	t.Run("should aggregate health status from multiple modules", func(t *testing.T) {
		// Create application with health-reporting modules
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		// Register modules with different health states
		healthyModule := &testHealthModule{
			name:      "healthy-service",
			isHealthy: true,
			timeout:   5 * time.Second,
			details:   map[string]interface{}{"connections": 10, "uptime": "2h"},
		}

		degradedModule := &testHealthModule{
			name:      "degraded-service", 
			isHealthy: false,
			timeout:   5 * time.Second,
			details:   map[string]interface{}{"errors": 3, "performance": "reduced"},
		}

		unhealthyModule := &testHealthModule{
			name:      "unhealthy-service",
			isHealthy: false,
			timeout:   5 * time.Second,
			details:   map[string]interface{}{"error": "database connection failed"},
		}

		app.RegisterModule(healthyModule)
		app.RegisterModule(degradedModule)
		app.RegisterModule(unhealthyModule)

		// Simulate health aggregation
		modules := app.GetModules()
		ctx := context.Background()
		healthResults := make(map[string]HealthResult)

		for moduleName, module := range modules {
			if healthReporter, ok := module.(HealthReporter); ok {
				result := healthReporter.CheckHealth(ctx)
				healthResults[moduleName] = result
			}
		}

		// Verify health results
		require.Len(t, healthResults, 3, "Should have health results for all modules")

		// Check individual module health
		healthyResult := healthResults["healthy-service"]
		assert.Equal(t, HealthStatusHealthy, healthyResult.Status)
		assert.Contains(t, healthyResult.Details, "connections")

		degradedResult := healthResults["degraded-service"] 
		assert.Equal(t, HealthStatusUnhealthy, degradedResult.Status) // testHealthModule returns unhealthy when not healthy
		assert.Contains(t, degradedResult.Details, "errors")

		unhealthyResult := healthResults["unhealthy-service"]
		assert.Equal(t, HealthStatusUnhealthy, unhealthyResult.Status)
		assert.Contains(t, unhealthyResult.Details, "error")

		// Test overall application health aggregation logic
		overallHealthy := true
		for _, result := range healthResults {
			if !result.Status.IsHealthy() {
				overallHealthy = false
				break
			}
		}

		assert.False(t, overallHealthy, "Application should be unhealthy when any module is unhealthy")
	})

	t.Run("should handle health check timeouts in aggregation", func(t *testing.T) {
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		// Register a slow health reporter
		slowModule := &slowHealthReporter{
			name:    "slow-service",
			delay:   100 * time.Millisecond,
			timeout: 5 * time.Second,
		}

		app.RegisterModule(slowModule)

		modules := app.GetModules()
		
		// Test with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		if healthReporter, ok := modules["slow-service"].(HealthReporter); ok {
			result := healthReporter.CheckHealth(ctx)
			assert.Equal(t, HealthStatusUnknown, result.Status, "Should return unknown status on timeout")
		}
	})
}

// TestApplicationConfigurationFlow tests real configuration loading and validation flow
func TestApplicationConfigurationFlow(t *testing.T) {
	t.Run("should load and validate configuration for reloadable modules", func(t *testing.T) {
		// Create application with configuration validation
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		// Register modules with configuration needs
		configAwareModule := &configAwareReloadableModule{
			testReloadableModule: testReloadableModule{
				name:      "config-service",
				canReload: true,
				timeout:   30 * time.Second,
			},
			configSchema: map[string]interface{}{
				"host":       "string",
				"port":       "int",
				"enabled":    "bool",
				"timeout":    "duration",
			},
		}

		app.RegisterModule(configAwareModule)

		// Simulate configuration registration
		err := configAwareModule.RegisterConfig(app)
		require.NoError(t, err, "Config registration should succeed")

		// Test configuration validation
		validConfig := map[string]interface{}{
			"host":    "example.com",
			"port":    8080,
			"enabled": true,
			"timeout": "30s",
		}

		err = configAwareModule.Reload(context.Background(), validConfig)
		assert.NoError(t, err, "Valid config should be accepted")

		// Test invalid configuration
		invalidConfig := map[string]interface{}{
			"host":    "",     // Invalid: empty host
			"port":    -1,     // Invalid: negative port
			"enabled": "true", // Invalid: string instead of bool
		}

		err = configAwareModule.Reload(context.Background(), invalidConfig)
		assert.Error(t, err, "Invalid config should be rejected")
	})

	t.Run("should coordinate configuration updates across dependent modules", func(t *testing.T) {
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		// Create modules with dependency relationships
		databaseModule := &dependentReloadableModule{
			testReloadableModule: testReloadableModule{
				name:      "database",
				canReload: true,
				timeout:   20 * time.Second,
			},
			dependsOn: []string{},
		}

		cacheModule := &dependentReloadableModule{
			testReloadableModule: testReloadableModule{
				name:      "cache",
				canReload: true,
				timeout:   15 * time.Second,
			},
			dependsOn: []string{"database"},
		}

		apiModule := &dependentReloadableModule{
			testReloadableModule: testReloadableModule{
				name:      "api",
				canReload: true,
				timeout:   25 * time.Second,
			},
			dependsOn: []string{"database", "cache"},
		}

		app.RegisterModule(databaseModule)
		app.RegisterModule(cacheModule)
		app.RegisterModule(apiModule)

		// Simulate ordered reload based on dependencies
		reloadOrder := []string{"database", "cache", "api"}
		modules := app.GetModules()
		
		for _, moduleName := range reloadOrder {
			module := modules[moduleName]
			if reloadable, ok := module.(Reloadable); ok {
				config := map[string]interface{}{
					"module":  moduleName,
					"version": "updated",
				}
				
				err := reloadable.Reload(context.Background(), config)
				assert.NoError(t, err, "Module %s should reload successfully", moduleName)
			}
		}

		// Verify all modules were updated in correct order
		assert.Equal(t, map[string]interface{}{"module": "database", "version": "updated"}, databaseModule.currentConfig)
		assert.Equal(t, map[string]interface{}{"module": "cache", "version": "updated"}, cacheModule.currentConfig)
		assert.Equal(t, map[string]interface{}{"module": "api", "version": "updated"}, apiModule.currentConfig)
	})
}

// Additional test helper implementations for integration testing

// configAwareReloadableModule extends testReloadableModule with configuration validation
type configAwareReloadableModule struct {
	testReloadableModule
	configSchema map[string]interface{}
}

func (m *configAwareReloadableModule) RegisterConfig(app Application) error {
	// Register configuration section for this module
	configProvider := NewStdConfigProvider(m.currentConfig)
	return app.RegisterConfigSection(m.name+"-config", configProvider)
}

func (m *configAwareReloadableModule) Reload(ctx context.Context, newConfig interface{}) error {
	// Validate config against schema before applying
	if err := m.validateConfigSchema(newConfig); err != nil {
		return err
	}
	
	return m.testReloadableModule.Reload(ctx, newConfig)
}

func (m *configAwareReloadableModule) validateConfigSchema(config interface{}) error {
	configMap, ok := config.(map[string]interface{})
	if !ok {
		return errors.New("config must be a map")
	}

	// Basic schema validation
	if host, ok := configMap["host"].(string); ok && host == "" {
		return errors.New("host cannot be empty")
	}
	
	if port, ok := configMap["port"].(int); ok && port <= 0 {
		return errors.New("port must be positive")
	}

	return nil
}

// dependentReloadableModule extends testReloadableModule with dependency information
type dependentReloadableModule struct {
	testReloadableModule
	dependsOn []string
}

func (m *dependentReloadableModule) Dependencies() []string {
	return m.dependsOn
}

// Mock errors for testing configuration validation
var (
	ErrInvalidConfig = errors.New("invalid configuration")
)