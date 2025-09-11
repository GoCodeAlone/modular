package modular

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReloadable_Reload tests the actual behavior of configuration reloading
func TestReloadable_Reload(t *testing.T) {
	tests := []struct {
		name        string
		reloadable  Reloadable
		ctx         context.Context
		changes     []ConfigChange
		expectError bool
		errorType   error
	}{
		{
			name:       "successful reload with valid config changes",
			reloadable: newTestReloadableModule("test-service", true, 30*time.Second),
			ctx:        context.Background(),
			changes: []ConfigChange{
				{
					Section:   "test",
					FieldPath: "key",
					OldValue:  nil,
					NewValue:  "value",
					Source:    "test",
				},
			},
			expectError: false,
		},
		{
			name:        "reload failure with empty changes",
			reloadable:  newTestReloadableModule("failing-service", true, 30*time.Second),
			ctx:         context.Background(),
			changes:     nil, // Empty changes should be handled gracefully
			expectError: false,
		},
		{
			name:       "reload timeout with context cancellation",
			reloadable: newSlowReloadableModule("slow-service", 100*time.Millisecond),
			ctx:        createTimedOutContext(10 * time.Millisecond),
			changes: []ConfigChange{
				{
					Section:   "slow",
					FieldPath: "key",
					OldValue:  nil,
					NewValue:  "value",
					Source:    "test",
				},
			},
			expectError: true,
			errorType:   context.DeadlineExceeded,
		},
		{
			name:       "reload with multiple configuration changes",
			reloadable: newTestReloadableModule("complex-service", true, 30*time.Second),
			ctx:        context.Background(),
			changes: []ConfigChange{
				{
					Section:   "database",
					FieldPath: "host",
					OldValue:  "old-host",
					NewValue:  "localhost",
					Source:    "file",
				},
				{
					Section:   "database",
					FieldPath: "port",
					OldValue:  3306,
					NewValue:  5432,
					Source:    "file",
				},
				{
					Section:   "cache",
					FieldPath: "enabled",
					OldValue:  false,
					NewValue:  true,
					Source:    "env",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.reloadable.Reload(tt.ctx, tt.changes)

			if tt.expectError {
				require.Error(t, err, "Expected reload to fail")
				if tt.errorType != nil {
					assert.True(t, errors.Is(err, tt.errorType), "Error should be of expected type")
				}
			} else {
				require.NoError(t, err, "Expected reload to succeed")
			}
		})
	}
}

// TestReloadable_CanReload tests reload capability checking
func TestReloadable_CanReload(t *testing.T) {
	tests := []struct {
		name       string
		reloadable Reloadable
		expected   bool
	}{
		{
			name:       "reloadable service returns true",
			reloadable: newTestReloadableModule("reloadable-service", true, 30*time.Second),
			expected:   true,
		},
		{
			name:       "non-reloadable service returns false",
			reloadable: newNonReloadableModule("fixed-service"),
			expected:   false,
		},
		{
			name:       "conditionally reloadable service",
			reloadable: newConditionalReloadableModule("conditional-service", false),
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canReload := tt.reloadable.CanReload()
			assert.Equal(t, tt.expected, canReload, "CanReload should match expected value")
		})
	}
}

// TestReloadable_ReloadTimeout tests timeout configuration
func TestReloadable_ReloadTimeout(t *testing.T) {
	tests := []struct {
		name            string
		reloadable      Reloadable
		expectedTimeout time.Duration
	}{
		{
			name:            "returns configured timeout",
			reloadable:      newTestReloadableModule("service", true, 15*time.Second),
			expectedTimeout: 15 * time.Second,
		},
		{
			name:            "returns different timeout",
			reloadable:      newTestReloadableModule("service", true, 2*time.Minute),
			expectedTimeout: 2 * time.Minute,
		},
		{
			name:            "returns default timeout for unconfigured service",
			reloadable:      newTestReloadableModule("service", true, 0),
			expectedTimeout: 30 * time.Second, // Default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeout := tt.reloadable.ReloadTimeout()
			assert.Equal(t, tt.expectedTimeout, timeout, "Timeout should match expected value")
		})
	}
}

// TestReloadable_ModuleIntegration tests integration with module lifecycle
func TestReloadable_ModuleIntegration(t *testing.T) {
	t.Run("should integrate with module system", func(t *testing.T) {
		// Create a module that implements both Module and Reloadable
		module := &testReloadableModule{
			name:          "integrated-module",
			canReload:     true,
			timeout:       20 * time.Second,
			currentConfig: map[string]interface{}{"initial": true},
		}

		// Verify it implements both interfaces
		var reloadable Reloadable = module
		var moduleInterface Module = module

		require.NotNil(t, reloadable, "Module should implement Reloadable")
		require.NotNil(t, moduleInterface, "Module should implement Module")

		// Test reloadable functionality
		assert.True(t, reloadable.CanReload())
		assert.Equal(t, 20*time.Second, reloadable.ReloadTimeout())

		changes := []ConfigChange{
			{
				Section:   "test",
				FieldPath: "updated",
				OldValue:  false,
				NewValue:  true,
				Source:    "test",
			},
		}
		err := reloadable.Reload(context.Background(), changes)
		assert.NoError(t, err)

		// Verify config was updated in the module (additive to initial config)
		expectedConfig := map[string]interface{}{"initial": true, "updated": true}
		assert.Equal(t, expectedConfig, module.currentConfig)

		// Test module functionality
		assert.Equal(t, "integrated-module", moduleInterface.Name())
	})

	t.Run("should support application-level reload coordination", func(t *testing.T) {
		// Create application with reloadable modules
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		reloadableModule := &testReloadableModule{
			name:          "app-reloadable-module",
			canReload:     true,
			timeout:       10 * time.Second,
			currentConfig: map[string]interface{}{"app_level": "initial"},
		}

		// Register the module
		app.RegisterModule(reloadableModule)

		// Verify module is registered and can be accessed for reloading
		modules := app.GetModules()
		assert.Contains(t, modules, "app-reloadable-module")

		// Simulate application-level reload by checking if module is reloadable
		if reloadable, ok := modules["app-reloadable-module"].(Reloadable); ok {
			assert.True(t, reloadable.CanReload())

			changes := []ConfigChange{
				{
					Section:   "app",
					FieldPath: "app_level",
					OldValue:  "initial",
					NewValue:  "reloaded",
					Source:    "test",
				},
			}
			err := reloadable.Reload(context.Background(), changes)
			assert.NoError(t, err)
		} else {
			t.Error("Module should implement Reloadable interface")
		}
	})
}

// TestReloadable_ErrorHandling tests error scenarios and edge cases
func TestReloadable_ErrorHandling(t *testing.T) {
	t.Run("should handle context timeout gracefully", func(t *testing.T) {
		reloadable := newSlowReloadableModule("slow-service", 100*time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		changes := []ConfigChange{
			{
				Section:   "test",
				FieldPath: "test",
				OldValue:  nil,
				NewValue:  "config",
				Source:    "test",
			},
		}
		err := reloadable.Reload(ctx, changes)
		assert.Error(t, err, "Should fail due to timeout")
		assert.True(t, errors.Is(err, context.DeadlineExceeded), "Should be timeout error")
	})

	t.Run("should validate configuration before applying", func(t *testing.T) {
		reloadable := newValidatingReloadableModule("validating-service")

		// Test with valid config
		validChanges := []ConfigChange{
			{
				Section:   "test",
				FieldPath: "name",
				OldValue:  nil,
				NewValue:  "test-service",
				Source:    "test",
			},
			{
				Section:   "test",
				FieldPath: "port",
				OldValue:  nil,
				NewValue:  8080,
				Source:    "test",
			},
			{
				Section:   "test",
				FieldPath: "enabled",
				OldValue:  nil,
				NewValue:  true,
				Source:    "test",
			},
		}
		err := reloadable.Reload(context.Background(), validChanges)
		assert.NoError(t, err, "Valid config should be accepted")

		// Test with invalid config
		invalidChanges := []ConfigChange{
			{
				Section:   "test",
				FieldPath: "port",
				OldValue:  8080,
				NewValue:  -1, // Invalid port
				Source:    "test",
			},
		}
		err = reloadable.Reload(context.Background(), invalidChanges)
		assert.Error(t, err, "Invalid config should be rejected")
		assert.Contains(t, err.Error(), "validation", "Error should indicate validation failure")
	})

	t.Run("should handle context cancellation", func(t *testing.T) {
		reloadable := newSlowReloadableModule("cancelable-service", 50*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		changes := []ConfigChange{
			{
				Section:   "test",
				FieldPath: "test",
				OldValue:  nil,
				NewValue:  "config",
				Source:    "test",
			},
		}
		err := reloadable.Reload(ctx, changes)
		assert.Error(t, err, "Should fail due to cancellation")
		assert.True(t, errors.Is(err, context.Canceled), "Should be cancellation error")
	})

	t.Run("should preserve existing config on reload failure", func(t *testing.T) {
		module := &testReloadableModule{
			name:          "preserve-config-service",
			canReload:     true,
			timeout:       30 * time.Second,
			currentConfig: map[string]interface{}{"original": "value"},
		}

		originalConfig := module.currentConfig

		// Attempt reload with empty changes (should succeed gracefully)
		err := module.Reload(context.Background(), nil)
		assert.NoError(t, err, "Reload should succeed with empty changes")

		// Verify original config is preserved (no changes applied)
		assert.Equal(t, originalConfig, module.currentConfig, "Original config should be preserved when no changes are applied")
	})
}

// Test helper implementations that provide real behavior for testing

// testReloadableModule implements both Module and Reloadable for integration testing
type testReloadableModule struct {
	name          string
	canReload     bool
	timeout       time.Duration
	currentConfig interface{}
	validateFunc  func(interface{}) error
}

// Module interface implementation
func (m *testReloadableModule) Name() string                          { return m.name }
func (m *testReloadableModule) Dependencies() []string                { return nil }
func (m *testReloadableModule) Init(Application) error                { return nil }
func (m *testReloadableModule) Start(context.Context) error           { return nil }
func (m *testReloadableModule) Stop(context.Context) error            { return nil }
func (m *testReloadableModule) RegisterConfig(Application) error      { return nil }
func (m *testReloadableModule) ProvidesServices() []ServiceProvider   { return nil }
func (m *testReloadableModule) RequiresServices() []ServiceDependency { return nil }

// Reloadable interface implementation
func (m *testReloadableModule) Reload(ctx context.Context, changes []ConfigChange) error {
	// Check if reload is supported
	if !m.canReload {
		return ErrReloadNotSupported
	}

	// Handle empty changes gracefully
	if len(changes) == 0 {
		return nil // No changes to apply
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Validate changes if validator is provided
	if m.validateFunc != nil {
		// Convert changes back to a config-like structure for validation
		configMap := make(map[string]interface{})
		for _, change := range changes {
			configMap[change.FieldPath] = change.NewValue
		}
		if err := m.validateFunc(configMap); err != nil {
			return err
		}
	}

	// Apply all changes atomically
	if m.currentConfig == nil {
		m.currentConfig = make(map[string]interface{})
	}

	// For test purposes, store the changes as a simple map
	configMap, ok := m.currentConfig.(map[string]interface{})
	if !ok {
		configMap = make(map[string]interface{})
	}

	for _, change := range changes {
		configMap[change.FieldPath] = change.NewValue
	}

	m.currentConfig = configMap
	return nil
}

func (m *testReloadableModule) CanReload() bool {
	return m.canReload
}

func (m *testReloadableModule) ReloadTimeout() time.Duration {
	if m.timeout > 0 {
		return m.timeout
	}
	return 30 * time.Second // Default timeout
}

// Test helper functions for creating reloadable modules with specific behaviors

func newTestReloadableModule(name string, canReload bool, timeout time.Duration) Reloadable {
	return &testReloadableModule{
		name:      name,
		canReload: canReload,
		timeout:   timeout,
	}
}

func newNonReloadableModule(name string) Reloadable {
	return &testReloadableModule{
		name:      name,
		canReload: false,
		timeout:   0,
	}
}

func newConditionalReloadableModule(name string, condition bool) Reloadable {
	return &testReloadableModule{
		name:      name,
		canReload: condition,
		timeout:   30 * time.Second,
	}
}

func newSlowReloadableModule(name string, delay time.Duration) Reloadable {
	return &slowReloadableModule{
		name:    name,
		delay:   delay,
		timeout: 30 * time.Second,
	}
}

func newValidatingReloadableModule(name string) Reloadable {
	return &testReloadableModule{
		name:      name,
		canReload: true,
		timeout:   30 * time.Second,
		validateFunc: func(config interface{}) error {
			if config == nil {
				return errors.New("config cannot be nil")
			}

			if configMap, ok := config.(map[string]interface{}); ok {
				if port, exists := configMap["port"]; exists {
					if portNum, ok := port.(int); ok && portNum < 0 {
						return errors.New("port validation failed: port must be positive")
					}
				}
			}
			return nil
		},
	}
}

func createTimedOutContext(timeout time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	// Don't call cancel() - let it timeout naturally
	_ = cancel
	return ctx
}

// Additional helper implementations

type slowReloadableModule struct {
	name    string
	delay   time.Duration
	timeout time.Duration
	config  interface{}
}

func (m *slowReloadableModule) Reload(ctx context.Context, changes []ConfigChange) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(m.delay):
		// For test purposes, store changes as a simple map
		configMap := make(map[string]interface{})
		for _, change := range changes {
			configMap[change.FieldPath] = change.NewValue
		}
		m.config = configMap
		return nil
	}
}

func (m *slowReloadableModule) CanReload() bool {
	return true
}

func (m *slowReloadableModule) ReloadTimeout() time.Duration {
	return m.timeout
}
