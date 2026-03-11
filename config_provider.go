package modular

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
)

const mainConfigSection = "_main"

// LoadAppConfigFunc is the function type for loading application configuration.
// This function is responsible for loading configuration data into the application
// using the registered config feeders and config sections.
//
// The default implementation can be replaced for testing or custom configuration scenarios.
type LoadAppConfigFunc func(*StdApplication) error

// AppConfigLoader is the default implementation that can be replaced in tests.
// This variable allows the configuration loading strategy to be customized,
// which is particularly useful for testing scenarios where you want to
// control how configuration is loaded.
//
// Example of replacing for tests:
//
//	oldLoader := modular.AppConfigLoader
//	defer func() { modular.AppConfigLoader = oldLoader }()
//	modular.AppConfigLoader = func(app *StdApplication) error {
//	    // Custom test configuration loading
//	    return nil
//	}
var AppConfigLoader LoadAppConfigFunc = loadAppConfig

// ConfigProvider defines the interface for providing configuration objects.
// Configuration providers encapsulate configuration data and make it available
// to modules and the application framework.
//
// The framework supports multiple configuration sources (files, environment variables,
// command-line flags) and formats (JSON, YAML, TOML) through different providers.
type ConfigProvider interface {
	// GetConfig returns the configuration object.
	// The returned value should be a pointer to a struct that represents
	// the configuration schema. Modules typically type-assert this to
	// their expected configuration type.
	//
	// Example:
	//   cfg := provider.GetConfig().(*MyModuleConfig)
	GetConfig() any
}

// StdConfigProvider provides a standard implementation of ConfigProvider.
// It wraps a configuration struct and makes it available through the ConfigProvider interface.
//
// IMPORTANT THREAD SAFETY WARNING:
// StdConfigProvider returns the SAME reference on every GetConfig() call.
// This means:
//   - Multiple modules/goroutines will share the same configuration object
//   - Modifications by any consumer affect ALL other consumers
//   - NOT safe for concurrent modification
//   - NOT suitable for multi-tenant applications with per-tenant config isolation
//
// For safer alternatives, see:
//   - NewIsolatedConfigProvider: Returns deep copies (test isolation)
//   - NewImmutableConfigProvider: Thread-safe immutable config (production)
//   - NewCopyOnWriteConfigProvider: Copy-on-write for defensive mutations
//
// Best practices:
//   - Use StdConfigProvider only when you need shared mutable config
//   - Modules should NOT modify configs in-place
//   - Tests should use IsolatedConfigProvider to prevent pollution
type StdConfigProvider struct {
	cfg any
}

// GetConfig returns the configuration object.
// WARNING: The returned value is the exact object reference that was passed to NewStdConfigProvider.
// Any modifications to this object will affect all other consumers of this config provider.
func (s *StdConfigProvider) GetConfig() any {
	return s.cfg
}

// NewStdConfigProvider creates a new standard configuration provider.
// The cfg parameter should be a pointer to a struct that defines the
// configuration schema for your module.
//
// WARNING: This provider returns the SAME reference on every GetConfig() call.
// For test isolation or thread-safe scenarios, use NewIsolatedConfigProvider or
// NewImmutableConfigProvider instead.
//
// Example:
//
//	type MyConfig struct {
//	    Host string `json:"host" default:"localhost"`
//	    Port int    `json:"port" default:"8080"`
//	}
//
//	cfg := &MyConfig{}
//	provider := modular.NewStdConfigProvider(cfg)
func NewStdConfigProvider(cfg any) *StdConfigProvider {
	return &StdConfigProvider{cfg: cfg}
}

// IsolatedConfigProvider provides complete configuration isolation by returning
// a deep copy on every GetConfig() call. This ensures that each consumer receives
// its own independent copy of the configuration, preventing any possibility of
// shared state or mutation pollution.
//
// Use cases:
//   - Test isolation: Prevents config pollution between test runs
//   - Multi-tenant applications: Each tenant gets isolated config
//   - Defensive programming: Modules can modify their config without side effects
//
// Performance considerations:
//   - Deep copy on EVERY GetConfig() call (expensive)
//   - Best suited for scenarios where isolation is more important than performance
//   - For production workloads, consider ImmutableConfigProvider instead
//
// Example:
//
//	cfg := &MyConfig{Host: "localhost", Port: 8080}
//	provider := modular.NewIsolatedConfigProvider(cfg)
//	// Each call returns a completely independent copy
//	copy1 := provider.GetConfig().(*MyConfig)
//	copy2 := provider.GetConfig().(*MyConfig)
//	copy1.Port = 9090  // Does NOT affect copy2
type IsolatedConfigProvider struct {
	cfg any
}

// GetConfig returns a deep copy of the configuration object.
// Each call creates a new independent copy, ensuring complete isolation.
// Returns nil if deep copying fails to maintain isolation guarantees.
func (p *IsolatedConfigProvider) GetConfig() any {
	copied, err := DeepCopyConfig(p.cfg)
	if err != nil {
		// Return nil to prevent shared state pollution and maintain isolation guarantees
		return nil
	}
	return copied
}

// NewIsolatedConfigProvider creates a configuration provider that returns
// deep copies on every GetConfig() call, ensuring complete isolation between
// consumers.
//
// This is the recommended provider for test scenarios where config isolation
// is critical to prevent test pollution.
//
// Example:
//
//	cfg := &MyConfig{Host: "localhost"}
//	provider := modular.NewIsolatedConfigProvider(cfg)
func NewIsolatedConfigProvider(cfg any) *IsolatedConfigProvider {
	return &IsolatedConfigProvider{cfg: cfg}
}

// ImmutableConfigProvider provides thread-safe access to configuration using
// atomic operations. The configuration is stored in an atomic.Value, allowing
// concurrent reads without locks while supporting atomic updates.
//
// Use cases:
//   - Production applications with concurrent access
//   - High-performance read-heavy workloads
//   - Configuration hot-reloading with atomic swaps
//   - Multi-tenant applications with shared config
//
// Thread safety:
//   - GetConfig() is lock-free and safe for concurrent reads
//   - Multiple goroutines can read simultaneously without contention
//   - Updates via UpdateConfig() are atomic
//
// Performance:
//   - Excellent for read-heavy workloads (no locks, no copies)
//   - Near-zero overhead for reads
//   - Best choice for production concurrent scenarios
//
// Example:
//
//	cfg := &MyConfig{Host: "localhost", Port: 8080}
//	provider := modular.NewImmutableConfigProvider(cfg)
//	// Thread-safe reads from multiple goroutines
//	config := provider.GetConfig().(*MyConfig)
//	// Atomic update
//	newCfg := &MyConfig{Host: "example.com", Port: 443}
//	provider.UpdateConfig(newCfg)
type ImmutableConfigProvider struct {
	cfg atomic.Value
}

// GetConfig returns the current configuration object in a thread-safe manner.
// This operation is lock-free and safe for concurrent access from multiple goroutines.
func (p *ImmutableConfigProvider) GetConfig() any {
	return p.cfg.Load()
}

// UpdateConfig atomically replaces the configuration with a new value.
// This operation is thread-safe and all subsequent GetConfig() calls will
// return the new configuration.
//
// This is useful for configuration hot-reloading scenarios where you want to
// update the config without restarting the application.
func (p *ImmutableConfigProvider) UpdateConfig(cfg any) {
	p.cfg.Store(cfg)
}

// NewImmutableConfigProvider creates a thread-safe configuration provider
// using atomic operations. This is the recommended provider for production
// applications with concurrent access patterns.
//
// Example:
//
//	cfg := &MyConfig{Host: "localhost", Port: 8080}
//	provider := modular.NewImmutableConfigProvider(cfg)
func NewImmutableConfigProvider(cfg any) *ImmutableConfigProvider {
	provider := &ImmutableConfigProvider{}
	provider.cfg.Store(cfg)
	return provider
}

// CopyOnWriteConfigProvider provides a configuration provider with explicit
// copy-on-write semantics. It returns the original configuration for reads,
// but provides a GetMutableConfig() method that returns an isolated deep copy
// for scenarios where modifications are needed.
//
// Use cases:
//   - Modules that need to apply defensive modifications
//   - Scenarios where you want to modify config without affecting others
//   - When you need explicit control over when copies are made
//
// Thread safety:
//   - GetConfig() uses RLock for safe concurrent reads
//   - GetMutableConfig() uses Lock and creates isolated copies
//   - Safe for concurrent access with proper synchronization
//
// Performance:
//   - Good: Only copies when explicitly requested via GetMutableConfig()
//   - Read-heavy workloads perform well (RLock is cheap)
//   - Better than IsolatedConfigProvider (which copies on every read)
//
// Example:
//
//	cfg := &MyConfig{Host: "localhost", Port: 8080}
//	provider := modular.NewCopyOnWriteConfigProvider(cfg)
//
//	// Read-only access (no copy)
//	readCfg := provider.GetConfig().(*MyConfig)
//
//	// Need to modify? Get a mutable copy
//	mutableCfg, err := provider.GetMutableConfig()
//	if err == nil {
//	    cfg := mutableCfg.(*MyConfig)
//	    cfg.Port = 9090  // Safe to modify, won't affect others
//	}
type CopyOnWriteConfigProvider struct {
	cfg any
	mu  sync.RWMutex
}

// GetConfig returns the original configuration object for read-only access.
// This method uses RLock for safe concurrent reads without creating copies.
func (p *CopyOnWriteConfigProvider) GetConfig() any {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cfg
}

// GetMutableConfig returns a deep copy of the configuration for modification.
// The returned copy is completely isolated from the original and other consumers.
//
// This method should be used when you need to make modifications to the config
// without affecting other consumers. The copy is created using DeepCopyConfig.
//
// Returns an error if deep copying fails.
func (p *CopyOnWriteConfigProvider) GetMutableConfig() (any, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return DeepCopyConfig(p.cfg)
}

// UpdateOriginal atomically replaces the original configuration with a new value.
// This allows implementing config hot-reload scenarios where you want to update
// the base configuration that GetConfig() returns.
func (p *CopyOnWriteConfigProvider) UpdateOriginal(cfg any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = cfg
}

// NewCopyOnWriteConfigProvider creates a configuration provider with
// copy-on-write semantics. Use GetConfig() for read-only access and
// GetMutableConfig() when you need an isolated copy for modification.
//
// Example:
//
//	cfg := &MyConfig{Host: "localhost", Port: 8080}
//	provider := modular.NewCopyOnWriteConfigProvider(cfg)
func NewCopyOnWriteConfigProvider(cfg any) *CopyOnWriteConfigProvider {
	return &CopyOnWriteConfigProvider{cfg: cfg}
}

// Config represents a configuration builder that can combine multiple feeders and structures.
// It provides functionality for the modular framework to coordinate configuration loading.
//
// The Config builder allows you to:
//   - Add multiple configuration sources (files, environment, etc.)
//   - Combine configuration from different feeders
//   - Apply configuration to multiple struct targets
//   - Track which structs have been configured
//   - Enable verbose debugging for configuration processing
//   - Track field-level population details
type Config struct {
	// Feeders contains all the registered configuration feeders
	Feeders []Feeder
	// StructKeys maps struct identifiers to their configuration objects.
	// Used internally to track which configuration structures have been processed.
	StructKeys map[string]any
	// VerboseDebug enables detailed logging during configuration processing
	VerboseDebug bool
	// Logger is used for verbose debug logging
	Logger Logger
	// FieldTracker tracks which fields are populated by which feeders
	FieldTracker FieldTracker
}

// NewConfig creates a new configuration builder.
// The returned Config can be used to set up complex configuration scenarios
// involving multiple sources and target structures.
//
// Example:
//
//	cfg := modular.NewConfig()
//	cfg.AddFeeder(modular.ConfigFeeders[0]) // Add file feeder
//	cfg.AddStruct(&myConfig)                // Add target struct
//	err := cfg.Feed()                       // Load configuration
func NewConfig() *Config {
	return &Config{
		Feeders:      make([]Feeder, 0),
		StructKeys:   make(map[string]any),
		VerboseDebug: false,
		Logger:       nil,
		FieldTracker: NewDefaultFieldTracker(),
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (c *Config) SetVerboseDebug(enabled bool, logger Logger) *Config {
	c.VerboseDebug = enabled
	c.Logger = logger

	// Set logger on field tracker
	if c.FieldTracker != nil {
		c.FieldTracker.SetLogger(logger)
	}

	// Apply verbose debugging to any verbose-aware feeders
	for _, feeder := range c.Feeders {
		if verboseFeeder, ok := feeder.(VerboseAwareFeeder); ok {
			verboseFeeder.SetVerboseDebug(enabled, logger)
		}
	}

	return c
}

// AddFeeder adds a configuration feeder to support verbose debugging and field tracking
func (c *Config) AddFeeder(feeder Feeder) *Config {
	c.Feeders = append(c.Feeders, feeder)

	// If verbose debugging is enabled, apply it to this feeder
	if c.VerboseDebug && c.Logger != nil {
		if verboseFeeder, ok := feeder.(VerboseAwareFeeder); ok {
			verboseFeeder.SetVerboseDebug(true, c.Logger)
		}
	}
	// If field tracking is enabled, apply it to this feeder
	if c.FieldTracker != nil {
		// Check for main package FieldTrackingFeeder interface
		if trackingFeeder, ok := feeder.(FieldTrackingFeeder); ok {
			trackingFeeder.SetFieldTracker(c.FieldTracker)
		} else {
			// Check for feeders package interface compatibility
			// Use reflection to check if the feeder has a SetFieldTracker method
			feederValue := reflect.ValueOf(feeder)
			setFieldTrackerMethod := feederValue.MethodByName("SetFieldTracker")
			if setFieldTrackerMethod.IsValid() {
				// Create a bridge adapter and call SetFieldTracker
				bridge := NewFieldTrackerBridge(c.FieldTracker)
				args := []reflect.Value{reflect.ValueOf(bridge)}
				setFieldTrackerMethod.Call(args)
			}
		}
	}

	return c
}

// AddStructKey adds a structure with a key to the configuration
func (c *Config) AddStructKey(key string, target any) *Config {
	c.StructKeys[key] = target
	return c
}

// SetFieldTracker sets the field tracker for capturing field population details
func (c *Config) SetFieldTracker(tracker FieldTracker) *Config {
	c.FieldTracker = tracker
	if c.Logger != nil {
		c.FieldTracker.SetLogger(c.Logger)
	}

	// Apply field tracking to any tracking-aware feeders
	for _, feeder := range c.Feeders {
		if trackingFeeder, ok := feeder.(FieldTrackingFeeder); ok {
			trackingFeeder.SetFieldTracker(tracker)
		}
	}

	return c
}

// FeedWithModuleContext feeds a single configuration structure with module context information
// This allows module-aware feeders to customize their behavior based on the module name
func (c *Config) FeedWithModuleContext(target any, moduleName string) error {
	if c.VerboseDebug && c.Logger != nil {
		c.Logger.Debug("Starting module-aware config feed", "targetType", reflect.TypeOf(target), "moduleName", moduleName, "feedersCount", len(c.Feeders))
	}

	for i, f := range c.Feeders {
		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("Applying feeder with module context", "feederIndex", i, "feederType", fmt.Sprintf("%T", f), "moduleName", moduleName)
		}

		// Try module-aware feeder first if available
		if maf, ok := f.(ModuleAwareFeeder); ok {
			if c.VerboseDebug && c.Logger != nil {
				c.Logger.Debug("Using ModuleAwareFeeder", "feederType", fmt.Sprintf("%T", f), "moduleName", moduleName)
			}
			if err := maf.FeedWithModuleContext(target, moduleName); err != nil {
				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("ModuleAwareFeeder failed", "feederType", fmt.Sprintf("%T", f), "error", err)
				}
				return fmt.Errorf("config feeder error: %w: %w", ErrConfigFeederError, err)
			}
		} else {
			// Fall back to regular Feed method for non-module-aware feeders
			if c.VerboseDebug && c.Logger != nil {
				c.Logger.Debug("Using regular Feed method", "feederType", fmt.Sprintf("%T", f))
			}
			if err := f.Feed(target); err != nil {
				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("Regular Feed method failed", "feederType", fmt.Sprintf("%T", f), "error", err)
				}
				return fmt.Errorf("config feeder error: %w: %w", ErrConfigFeederError, err)
			}
		}

		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("Feeder applied successfully", "feederType", fmt.Sprintf("%T", f))
		}
	}

	// Apply defaults and validate config
	if c.VerboseDebug && c.Logger != nil {
		c.Logger.Debug("Validating config", "moduleName", moduleName)
	}

	if err := ValidateConfig(target); err != nil {
		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("Config validation failed", "moduleName", moduleName, "error", err)
		}
		return fmt.Errorf("config validation error for %s: %w", moduleName, err)
	}

	if c.VerboseDebug && c.Logger != nil {
		c.Logger.Debug("Config validation succeeded", "moduleName", moduleName)
	}

	// Call Setup if implemented
	if setupable, ok := target.(ConfigSetup); ok {
		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("Calling Setup for config", "moduleName", moduleName)
		}
		if err := setupable.Setup(); err != nil {
			if c.VerboseDebug && c.Logger != nil {
				c.Logger.Debug("Config setup failed", "moduleName", moduleName, "error", err)
			}
			return fmt.Errorf("%w for %s: %w", ErrConfigSetupError, moduleName, err)
		}
		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("Config setup succeeded", "moduleName", moduleName)
		}
	}

	return nil
}

// Feed with validation applies defaults and validates configs after feeding
func (c *Config) Feed() error {
	if c.VerboseDebug && c.Logger != nil {
		c.Logger.Debug("Starting config feed process", "structKeysCount", len(c.StructKeys), "feedersCount", len(c.Feeders))
	}

	// Sort feeders by priority (ascending order, so higher priority applies last)
	sortedFeeders := c.sortFeedersByPriority()

	// If we have struct keys, feed them directly with field tracking
	if len(c.StructKeys) > 0 {
		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("Using enhanced feeding process with field tracking")
		}

		// Feed each struct key with each feeder
		for key, target := range c.StructKeys {
			if c.VerboseDebug && c.Logger != nil {
				c.Logger.Debug("Processing struct key", "key", key, "targetType", reflect.TypeOf(target))
			}

			for i, f := range sortedFeeders {
				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("Applying feeder to struct", "key", key, "feederIndex", i, "feederType", fmt.Sprintf("%T", f))
				}

				// Try module-aware feeder first if this is a section config (not main config)
				if key != mainConfigSection {
					if maf, ok := f.(ModuleAwareFeeder); ok {
						if c.VerboseDebug && c.Logger != nil {
							c.Logger.Debug("Using ModuleAwareFeeder for section", "key", key, "feederType", fmt.Sprintf("%T", f))
						}
						if err := maf.FeedWithModuleContext(target, key); err != nil {
							if c.VerboseDebug && c.Logger != nil {
								c.Logger.Debug("ModuleAwareFeeder Feed method failed", "key", key, "feederType", fmt.Sprintf("%T", f), "error", err)
							}
							return fmt.Errorf("config feeder error: %w: %w", ErrConfigFeederError, err)
						}
					} else {
						// Fall back to regular Feed method for non-module-aware feeders
						if err := f.Feed(target); err != nil {
							if c.VerboseDebug && c.Logger != nil {
								c.Logger.Debug("Regular Feed method failed", "key", key, "feederType", fmt.Sprintf("%T", f), "error", err)
							}
							return fmt.Errorf("config feeder error: %w: %w", ErrConfigFeederError, err)
						}
					}
				} else {
					// Use regular Feed method for main config
					if err := f.Feed(target); err != nil {
						if c.VerboseDebug && c.Logger != nil {
							c.Logger.Debug("Feeder Feed method failed", "key", key, "feederType", fmt.Sprintf("%T", f), "error", err)
						}
						return fmt.Errorf("config feeder error: %w: %w", ErrConfigFeederError, err)
					}
				}

				// Also try ComplexFeeder if available (for instance-aware feeders)
				if cf, ok := f.(ComplexFeeder); ok {
					if c.VerboseDebug && c.Logger != nil {
						c.Logger.Debug("Applying ComplexFeeder FeedKey", "key", key, "feederType", fmt.Sprintf("%T", f))
					}

					if err := cf.FeedKey(key, target); err != nil {
						if c.VerboseDebug && c.Logger != nil {
							c.Logger.Debug("ComplexFeeder FeedKey failed", "key", key, "feederType", fmt.Sprintf("%T", f), "error", err)
						}
						return fmt.Errorf("config feeder error: %w: %w", ErrConfigFeederError, err)
					}
				}

				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("Feeder applied successfully", "key", key, "feederType", fmt.Sprintf("%T", f))
				}
			}

			// Apply defaults and validate config
			if c.VerboseDebug && c.Logger != nil {
				c.Logger.Debug("Validating config for struct key", "key", key)
			}

			if err := ValidateConfig(target); err != nil {
				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("Config validation failed", "key", key, "error", err)
				}
				return fmt.Errorf("config validation error for %s: %w", key, err)
			}

			if c.VerboseDebug && c.Logger != nil {
				c.Logger.Debug("Config validation succeeded", "key", key)
			}

			// Call Setup if implemented
			if setupable, ok := target.(ConfigSetup); ok {
				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("Calling Setup for config", "key", key)
				}
				if err := setupable.Setup(); err != nil {
					if c.VerboseDebug && c.Logger != nil {
						c.Logger.Debug("Config setup failed", "key", key, "error", err)
					}
					return fmt.Errorf("%w for %s: %w", ErrConfigSetupError, key, err)
				}
				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("Config setup succeeded", "key", key)
				}
			}
		}
	} else {
		// No struct keys configured - this means no explicit structures were added
		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("No struct keys configured - skipping feed process")
		}
	}

	if c.VerboseDebug && c.Logger != nil {
		c.Logger.Debug("Config feed process completed successfully")
	}

	return nil
}

// sortFeedersByPriority sorts feeders by priority in ascending order.
// Higher priority values are applied later, allowing them to override lower priority feeders.
// Feeders without priority (not implementing PrioritizedFeeder) default to priority 0.
// When priorities are equal, original order is preserved (stable sort).
func (c *Config) sortFeedersByPriority() []Feeder {
	// Create a copy of feeders to avoid modifying the original slice
	sortedFeeders := make([]Feeder, len(c.Feeders))
	copy(sortedFeeders, c.Feeders)

	// Sort by priority (ascending, so highest priority applies last)
	sort.SliceStable(sortedFeeders, func(i, j int) bool {
		priI := 0
		if pf, ok := sortedFeeders[i].(PrioritizedFeeder); ok {
			priI = pf.Priority()
		}

		priJ := 0
		if pf, ok := sortedFeeders[j].(PrioritizedFeeder); ok {
			priJ = pf.Priority()
		}

		return priI < priJ
	})

	if c.VerboseDebug && c.Logger != nil {
		c.Logger.Debug("Feeders sorted by priority")
		for i, f := range sortedFeeders {
			pri := 0
			if pf, ok := f.(PrioritizedFeeder); ok {
				pri = pf.Priority()
			}
			c.Logger.Debug("Feeder order", "index", i, "type", fmt.Sprintf("%T", f), "priority", pri)
		}
	}

	return sortedFeeders
}

// ConfigSetup is an interface that configs can implement
// to perform additional setup after being populated by feeders
type ConfigSetup interface {
	Setup() error
}

func loadAppConfig(app *StdApplication) error {
	// Guard against nil application
	if app == nil {
		return ErrApplicationNil
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Starting configuration loading process")
	}

	// Auto-detect base config structure if not explicitly configured
	if !IsBaseConfigEnabled() {
		if DetectBaseConfigStructure() {
			if app.IsVerboseConfig() {
				app.logger.Debug("Auto-detected base configuration structure",
					"configDir", BaseConfigSettings.ConfigDir,
					"environment", BaseConfigSettings.Environment)
			}
		}
	}

	// Prepare config feeders - include base config feeder if enabled.
	// Priority / order:
	//   1. Base config feeder (if enabled)
	//   2. Per-app feeders (if explicitly provided via SetConfigFeeders)
	//   3. Global ConfigFeeders fallback (if no per-app feeders provided)
	var effectiveFeeders []Feeder

	// Start capacity estimation (base + either per-app or global)
	baseCount := 0
	if IsBaseConfigEnabled() && GetBaseConfigFeeder() != nil {
		baseCount = 1
	}
	if app.configFeeders != nil {
		effectiveFeeders = make([]Feeder, 0, baseCount+len(app.configFeeders))
	} else {
		effectiveFeeders = make([]Feeder, 0, baseCount+len(ConfigFeeders))
	}

	// Add base config feeder first if enabled (so it gets processed first)
	if IsBaseConfigEnabled() {
		if baseFeeder := GetBaseConfigFeeder(); baseFeeder != nil {
			effectiveFeeders = append(effectiveFeeders, baseFeeder)
			if app.IsVerboseConfig() {
				app.logger.Debug("Added base config feeder",
					"configDir", BaseConfigSettings.ConfigDir,
					"environment", BaseConfigSettings.Environment)
			}
		}
	}

	// Append per-app feeders if provided; else fall back to global
	if app.configFeeders != nil {
		effectiveFeeders = append(effectiveFeeders, app.configFeeders...)
	} else {
		effectiveFeeders = append(effectiveFeeders, ConfigFeeders...)
	}

	// Skip if no feeders are defined
	if len(effectiveFeeders) == 0 {
		app.logger.Info("No config feeders defined, skipping config loading")
		return nil
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Configuration feeders available", "count", len(effectiveFeeders))
		for i, feeder := range effectiveFeeders {
			app.logger.Debug("Config feeder registered", "index", i, "type", fmt.Sprintf("%T", feeder))
		}
	}

	// Build the configuration
	cfgBuilder := NewConfig()
	if app.IsVerboseConfig() {
		cfgBuilder.SetVerboseDebug(true, app.logger)
	}
	for _, feeder := range effectiveFeeders {
		cfgBuilder.AddFeeder(feeder)
		if app.IsVerboseConfig() {
			app.logger.Debug("Added config feeder to builder", "type", fmt.Sprintf("%T", feeder))
		}
	}

	// Process configs
	tempConfigs, hasConfigs := processConfigs(app, cfgBuilder)

	// If no valid configs found, return early
	if !hasConfigs {
		app.logger.Info("No valid configs found, skipping config loading")
		return nil
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Configuration structures prepared for feeding", "count", len(tempConfigs))
	}

	// Feed all configs at once
	if err := cfgBuilder.Feed(); err != nil {
		if app.IsVerboseConfig() {
			app.logger.Debug("Configuration feeding failed", "error", err)
		}
		return err
	}

	// Apply instance-aware feeding for supported configurations AFTER regular feeding
	if err := applyInstanceAwareFeeding(app, tempConfigs); err != nil {
		if app.IsVerboseConfig() {
			app.logger.Debug("Instance-aware feeding failed", "error", err)
		}
		return err
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Configuration feeding completed successfully")
	}

	// Apply updated configs
	applyConfigUpdates(app, tempConfigs)

	if app.IsVerboseConfig() {
		app.logger.Debug("Configuration loading process completed")
	}

	return nil
}

// processConfigs handles the collection and preparation of configs
func processConfigs(app *StdApplication, cfgBuilder *Config) (map[string]configInfo, bool) {
	tempConfigs := make(map[string]configInfo)
	hasConfigs := false

	if app.IsVerboseConfig() {
		app.logger.Debug("Processing configuration sections")
	}

	// Process main app config if provided
	if processedMain := processMainConfig(app, cfgBuilder, tempConfigs); processedMain {
		hasConfigs = true
	}

	// Process registered sections
	if processedSections := processSectionConfigs(app, cfgBuilder, tempConfigs); processedSections {
		hasConfigs = true
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Configuration processing completed", "totalConfigs", len(tempConfigs), "hasValidConfigs", hasConfigs)
	}

	return tempConfigs, hasConfigs
}

// processMainConfig handles the main application config
func processMainConfig(app *StdApplication, cfgBuilder *Config, tempConfigs map[string]configInfo) bool {
	if app.cfgProvider == nil {
		if app.IsVerboseConfig() {
			app.logger.Debug("Main config provider is nil, skipping main config")
		}
		return false
	}

	mainCfg := app.cfgProvider.GetConfig()
	if mainCfg == nil {
		app.logger.Warn("Main config is nil, skipping main config loading")
		return false
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Processing main configuration", "configType", reflect.TypeOf(mainCfg), "section", mainConfigSection)
	}

	tempMainCfg, mainCfgInfo, err := createTempConfig(mainCfg)
	if err != nil {
		app.logger.Warn("Failed to create temp config, skipping main config", "error", err)
		return false
	}

	cfgBuilder.AddStructKey(mainConfigSection, tempMainCfg)
	tempConfigs[mainConfigSection] = mainCfgInfo
	app.logger.Debug("Added main config for loading", "type", reflect.TypeOf(mainCfg))

	if app.IsVerboseConfig() {
		app.logger.Debug("Main configuration prepared for feeding", "section", mainConfigSection)
	}

	return true
}

// processSectionConfigs handles the section configs
func processSectionConfigs(app *StdApplication, cfgBuilder *Config, tempConfigs map[string]configInfo) bool {
	hasValidSections := false

	if app.IsVerboseConfig() {
		app.logger.Debug("Processing configuration sections", "totalSections", len(app.cfgSections))
	}

	for sectionKey, provider := range app.cfgSections {
		if app.IsVerboseConfig() {
			app.logger.Debug("Processing configuration section", "section", sectionKey, "providerType", fmt.Sprintf("%T", provider))
		}

		if provider == nil {
			app.logger.Warn("Skipping nil config provider", "section", sectionKey)
			continue
		}

		sectionCfg := provider.GetConfig()
		if sectionCfg == nil {
			app.logger.Warn("Skipping section with nil config", "section", sectionKey)
			continue
		}

		if app.IsVerboseConfig() {
			app.logger.Debug("Section config retrieved", "section", sectionKey, "configType", reflect.TypeOf(sectionCfg))
		}

		tempSectionCfg, sectionInfo, err := createTempConfig(sectionCfg)
		if err != nil {
			app.logger.Warn("Failed to create temp config for section, skipping",
				"section", sectionKey, "error", err)
			continue
		}

		cfgBuilder.AddStructKey(sectionKey, tempSectionCfg)
		tempConfigs[sectionKey] = sectionInfo
		hasValidSections = true

		app.logger.Debug("Added section config for loading",
			"section", sectionKey, "type", reflect.TypeOf(sectionCfg))

		if app.IsVerboseConfig() {
			app.logger.Debug("Section configuration prepared for feeding", "section", sectionKey)
		}
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Section configuration processing completed", "validSections", hasValidSections)
	}

	return hasValidSections
}

// applyConfigUpdates applies updates to all configs
func applyConfigUpdates(app *StdApplication, tempConfigs map[string]configInfo) {
	// Update main config if it exists
	if mainInfo, exists := tempConfigs[mainConfigSection]; exists {
		updateConfig(app, mainInfo)
		app.logger.Debug("Updated main config")
	}

	// Update section configs
	for sectionKey, info := range tempConfigs {
		if sectionKey == mainConfigSection {
			continue
		}
		updateSectionConfig(app, sectionKey, info)
		app.logger.Debug("Updated section config", "section", sectionKey)
	}
}

// applyInstanceAwareFeeding applies instance-aware feeding to configurations that support it
func applyInstanceAwareFeeding(app *StdApplication, tempConfigs map[string]configInfo) error {
	if app.IsVerboseConfig() {
		app.logger.Debug("Starting instance-aware feeding process")
	}

	// Check each section for instance-aware config support
	for sectionKey := range tempConfigs {
		if sectionKey == mainConfigSection {
			continue // Skip main config section for now
		}

		// Get the original provider to check if it's instance-aware
		provider, exists := app.cfgSections[sectionKey]
		if !exists {
			continue
		}

		// Check if the provider is instance-aware
		iaProvider, isInstanceAware := provider.(*InstanceAwareConfigProvider)
		if !isInstanceAware {
			if app.IsVerboseConfig() {
				app.logger.Debug("Section provider is not instance-aware, skipping", "section", sectionKey)
			}
			continue
		}

		if app.IsVerboseConfig() {
			app.logger.Debug("Processing instance-aware section", "section", sectionKey)
		}

		// Get the config from the temporary config that was just fed with YAML/ENV data
		configInfo := tempConfigs[sectionKey]
		var tempConfig any
		if configInfo.isPtr {
			tempConfig = configInfo.tempVal.Interface()
		} else {
			tempConfig = configInfo.tempVal.Elem().Interface()
		}

		// Check if it supports instance configurations
		instanceSupport, supportsInstances := tempConfig.(InstanceAwareConfigSupport)
		if !supportsInstances {
			if app.IsVerboseConfig() {
				app.logger.Debug("Config does not support instances, skipping", "section", sectionKey)
			}
			continue
		}

		// Get the instance configurations
		instances := instanceSupport.GetInstanceConfigs()
		if len(instances) == 0 {
			if app.IsVerboseConfig() {
				app.logger.Debug("No instances found for section", "section", sectionKey)
			}
			continue
		}

		if app.IsVerboseConfig() {
			app.logger.Debug("Found instances for section", "section", sectionKey, "instanceCount", len(instances))
		}

		// Get the prefix function
		prefixFunc := iaProvider.GetInstancePrefixFunc()
		if prefixFunc == nil {
			app.logger.Warn("Instance-aware provider missing prefix function", "section", sectionKey)
			continue
		}

		// Create instance-aware feeder
		instanceFeeder := NewInstanceAwareEnvFeeder(prefixFunc)

		// Apply verbose debug if enabled
		if app.IsVerboseConfig() {
			if verboseFeeder, ok := instanceFeeder.(VerboseAwareFeeder); ok {
				verboseFeeder.SetVerboseDebug(true, app.logger)
			}
		}

		// Feed each instance
		for instanceKey, instanceConfig := range instances {
			if app.IsVerboseConfig() {
				app.logger.Debug("Feeding instance configuration", "section", sectionKey, "instance", instanceKey)
			}

			if err := instanceFeeder.FeedKey(instanceKey, instanceConfig); err != nil {
				app.logger.Warn("Failed to feed instance configuration",
					"section", sectionKey, "instance", instanceKey, "error", err)
				continue
			}

			if app.IsVerboseConfig() {
				app.logger.Debug("Successfully fed instance configuration", "section", sectionKey, "instance", instanceKey)
			}
		}
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Instance-aware feeding process completed")
	}

	return nil
}

// Helper types and functions
type configInfo struct {
	originalVal reflect.Value
	tempVal     reflect.Value
	isPtr       bool
}

// createTempConfig creates a temporary config for feeding values
func createTempConfig(cfg any) (any, configInfo, error) {
	if cfg == nil {
		return nil, configInfo{}, ErrConfigNil
	}

	cfgValue := reflect.ValueOf(cfg)
	isPtr := cfgValue.Kind() == reflect.Pointer

	var targetType reflect.Type
	var sourceValue reflect.Value
	if isPtr {
		if cfgValue.IsNil() {
			return nil, configInfo{}, ErrConfigNilPointer
		}
		targetType = cfgValue.Elem().Type()
		sourceValue = cfgValue.Elem()
	} else {
		targetType = cfgValue.Type()
		sourceValue = cfgValue
	}

	tempCfgValue := reflect.New(targetType)

	// Copy existing values from the original config to the temp config
	// This preserves any values that were already set (e.g., by tests)
	// NOTE: This is a SHALLOW copy - maps and slices are shared with the original
	tempCfgValue.Elem().Set(sourceValue)

	return tempCfgValue.Interface(), configInfo{
		originalVal: cfgValue,
		tempVal:     tempCfgValue,
		isPtr:       isPtr,
	}, nil
}

// DeepCopyConfig creates a deep copy of a configuration object, ensuring complete
// isolation between the original and the copy. This is useful for:
// - Test isolation: preventing config pollution between tests
// - Multi-tenant applications: ensuring tenant configs don't share state
// - Module initialization: giving each module its own config instance
//
// The function recursively copies all maps, slices, and nested structures.
// Returns an error if the config is nil or if the copy operation fails.
func DeepCopyConfig(cfg any) (any, error) {
	copied, _, err := createTempConfigDeep(cfg)
	return copied, err
}

// createTempConfigDeep creates a temporary deep copy of the configuration for processing.
// Unlike createTempConfig, this performs a DEEP copy where maps, slices, and nested
// structures are fully duplicated, ensuring complete isolation between the original
// and temporary configuration.
//
// This is useful when you need to ensure that modifications to the temporary config
// during processing will not affect the original configuration.
func createTempConfigDeep(cfg any) (any, configInfo, error) {
	if cfg == nil {
		return nil, configInfo{}, ErrConfigNil
	}

	cfgValue := reflect.ValueOf(cfg)
	isPtr := cfgValue.Kind() == reflect.Pointer

	var targetType reflect.Type
	var sourceValue reflect.Value
	if isPtr {
		if cfgValue.IsNil() {
			return nil, configInfo{}, ErrConfigNilPointer
		}
		targetType = cfgValue.Elem().Type()
		sourceValue = cfgValue.Elem()
	} else {
		targetType = cfgValue.Type()
		sourceValue = cfgValue
	}

	tempCfgValue := reflect.New(targetType)

	// Perform deep copy to ensure complete isolation
	deepCopyValue(tempCfgValue.Elem(), sourceValue)

	return tempCfgValue.Interface(), configInfo{
		originalVal: cfgValue,
		tempVal:     tempCfgValue,
		isPtr:       isPtr,
	}, nil
}

// deepCopyValue performs a deep copy of values including maps, slices, and nested structures.
// It recursively copies the source value into the destination value, ensuring that
// mutable types (maps, slices, pointers) are properly duplicated rather than shared.
//
// This is useful for creating isolated configuration copies where modifications to
// the copy should not affect the original.
func deepCopyValue(dst, src reflect.Value) {
	// Handle invalid or nil values
	if !src.IsValid() {
		return
	}

	switch src.Kind() {
	case reflect.Pointer:
		if src.IsNil() {
			return
		}
		// Allocate new pointer and recursively copy the pointed-to value
		dst.Set(reflect.New(src.Elem().Type()))
		deepCopyValue(dst.Elem(), src.Elem())

	case reflect.Interface:
		if src.IsNil() {
			return
		}
		// Copy the concrete value inside the interface
		concrete := src.Elem()
		newVal := reflect.New(concrete.Type()).Elem()
		deepCopyValue(newVal, concrete)
		dst.Set(newVal)

	case reflect.Map:
		if src.IsNil() {
			return
		}
		// Create a new map and copy all key-value pairs
		dst.Set(reflect.MakeMap(src.Type()))
		for _, key := range src.MapKeys() {
			srcVal := src.MapIndex(key)
			dstVal := reflect.New(srcVal.Type()).Elem()
			deepCopyValue(dstVal, srcVal)
			dst.SetMapIndex(key, dstVal)
		}

	case reflect.Slice:
		if src.IsNil() {
			return
		}
		// Create a new slice and copy all elements
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Cap()))
		for i := 0; i < src.Len(); i++ {
			deepCopyValue(dst.Index(i), src.Index(i))
		}

	case reflect.Array:
		// Copy all array elements
		for i := 0; i < src.Len(); i++ {
			deepCopyValue(dst.Index(i), src.Index(i))
		}

	case reflect.Struct:
		// Copy all struct fields
		for i := 0; i < src.NumField(); i++ {
			srcField := src.Field(i)
			dstField := dst.Field(i)
			// Only copy exported fields (CanSet returns true for exported fields)
			if dstField.CanSet() {
				deepCopyValue(dstField, srcField)
			}
		}

	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		// Channels, functions, and unsafe pointers are copied by reference (cannot deep copy)
		dst.Set(src)

	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String:
		// For basic types, direct assignment works
		dst.Set(src)

	case reflect.Invalid:
		// Invalid values - do nothing
		return
	}
}

func updateConfig(app *StdApplication, info configInfo) {
	if info.isPtr {
		info.originalVal.Elem().Set(info.tempVal.Elem())
	} else {
		app.logger.Debug("Creating new provider with updated config (original was non-pointer)")
		// For non-pointer configs, we need to update the provider reference
		app.cfgProvider = NewStdConfigProvider(info.tempVal.Elem().Interface())
	}
}

func updateSectionConfig(app *StdApplication, sectionKey string, info configInfo) {
	if info.isPtr {
		info.originalVal.Elem().Set(info.tempVal.Elem())
	} else {
		app.logger.Debug("Creating new provider for section", "section", sectionKey)
		app.cfgSections[sectionKey] = NewStdConfigProvider(info.tempVal.Elem().Interface())
	}
}
