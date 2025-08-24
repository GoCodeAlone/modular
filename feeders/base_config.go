package feeders

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// BaseConfigFeeder supports layered configuration loading with base configs and environment-specific overrides
type BaseConfigFeeder struct {
	BaseDir      string // Directory containing base/ and environments/ subdirectories
	Environment  string // Environment name (e.g., "prod", "staging", "dev")
	verboseDebug bool
	logger       interface{ Debug(msg string, args ...any) }
	fieldTracker FieldTracker
}

// NewBaseConfigFeeder creates a new base configuration feeder
// baseDir should contain base/ and environments/ subdirectories
// environment specifies which environment overrides to apply (e.g., "prod", "staging", "dev")
func NewBaseConfigFeeder(baseDir, environment string) *BaseConfigFeeder {
	return &BaseConfigFeeder{
		BaseDir:      baseDir,
		Environment:  environment,
		verboseDebug: false,
		logger:       nil,
		fieldTracker: nil,
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (b *BaseConfigFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	b.verboseDebug = enabled
	b.logger = logger
	if enabled && logger != nil {
		b.logger.Debug("Verbose BaseConfig feeder debugging enabled", "baseDir", b.BaseDir, "environment", b.Environment)
	}
}

// SetFieldTracker sets the field tracker for recording field populations
func (b *BaseConfigFeeder) SetFieldTracker(tracker FieldTracker) {
	b.fieldTracker = tracker
}

// Feed loads and merges base configuration with environment-specific overrides
func (b *BaseConfigFeeder) Feed(structure interface{}) error {
	if b.verboseDebug && b.logger != nil {
		b.logger.Debug("BaseConfigFeeder: Starting feed process",
			"baseDir", b.BaseDir,
			"environment", b.Environment,
			"structureType", reflect.TypeOf(structure))
	}

	// Load base configuration first
	baseConfig, err := b.loadBaseConfig()
	if err != nil {
		if b.verboseDebug && b.logger != nil {
			b.logger.Debug("BaseConfigFeeder: Failed to load base config", "error", err)
		}
		return fmt.Errorf("failed to load base config: %w", err)
	}

	// Load environment overrides
	envConfig, err := b.loadEnvironmentConfig()
	if err != nil {
		if b.verboseDebug && b.logger != nil {
			b.logger.Debug("BaseConfigFeeder: Failed to load environment config", "error", err)
		}
		return fmt.Errorf("failed to load environment config: %w", err)
	}

	// Merge configurations (environment overrides base)
	mergedConfig := b.mergeConfigs(baseConfig, envConfig)

	// Apply merged configuration to the target structure
	err = b.applyConfigToStruct(mergedConfig, structure)
	if err != nil {
		if b.verboseDebug && b.logger != nil {
			b.logger.Debug("BaseConfigFeeder: Failed to apply config to struct", "error", err)
		}
		return fmt.Errorf("failed to apply merged config: %w", err)
	}

	if b.verboseDebug && b.logger != nil {
		b.logger.Debug("BaseConfigFeeder: Feed completed successfully")
	}

	return nil
}

// FeedKey loads and merges configurations for a specific key
func (b *BaseConfigFeeder) FeedKey(key string, target interface{}) error {
	if b.verboseDebug && b.logger != nil {
		b.logger.Debug("BaseConfigFeeder: Starting FeedKey process",
			"key", key,
			"targetType", reflect.TypeOf(target))
	}

	// Load base configuration for the specific key
	baseConfig, err := b.loadBaseConfigForKey(key)
	if err != nil {
		if b.verboseDebug && b.logger != nil {
			b.logger.Debug("BaseConfigFeeder: Failed to load base config for key", "key", key, "error", err)
		}
		return fmt.Errorf("failed to load base config for key %s: %w", key, err)
	}

	// Load environment overrides for the specific key
	envConfig, err := b.loadEnvironmentConfigForKey(key)
	if err != nil {
		if b.verboseDebug && b.logger != nil {
			b.logger.Debug("BaseConfigFeeder: Failed to load environment config for key", "key", key, "error", err)
		}
		return fmt.Errorf("failed to load environment config for key %s: %w", key, err)
	}

	// Merge configurations (environment overrides base)
	mergedConfig := b.mergeConfigs(baseConfig, envConfig)

	// Apply merged configuration to the target structure
	err = b.applyConfigToStruct(mergedConfig, target)
	if err != nil {
		if b.verboseDebug && b.logger != nil {
			b.logger.Debug("BaseConfigFeeder: Failed to apply config for key", "key", key, "error", err)
		}
		return fmt.Errorf("failed to apply merged config for key %s: %w", key, err)
	}

	if b.verboseDebug && b.logger != nil {
		b.logger.Debug("BaseConfigFeeder: FeedKey completed successfully", "key", key)
	}

	return nil
}

// loadBaseConfig loads the base configuration file
func (b *BaseConfigFeeder) loadBaseConfig() (map[string]interface{}, error) {
	baseConfigPath := b.findConfigFile(filepath.Join(b.BaseDir, "base"), "default")
	if baseConfigPath == "" {
		if b.verboseDebug && b.logger != nil {
			b.logger.Debug("BaseConfigFeeder: No base config file found", "baseDir", filepath.Join(b.BaseDir, "base"))
		}
		return make(map[string]interface{}), nil // Return empty config if no base file exists
	}

	return b.loadConfigFile(baseConfigPath)
}

// loadEnvironmentConfig loads the environment-specific overrides
func (b *BaseConfigFeeder) loadEnvironmentConfig() (map[string]interface{}, error) {
	envConfigPath := b.findConfigFile(filepath.Join(b.BaseDir, "environments", b.Environment), "overrides")
	if envConfigPath == "" {
		if b.verboseDebug && b.logger != nil {
			b.logger.Debug("BaseConfigFeeder: No environment config file found",
				"envDir", filepath.Join(b.BaseDir, "environments", b.Environment))
		}
		return make(map[string]interface{}), nil // Return empty config if no env file exists
	}

	return b.loadConfigFile(envConfigPath)
}

// loadBaseConfigForKey loads base config for a specific key (used for tenant configs)
func (b *BaseConfigFeeder) loadBaseConfigForKey(key string) (map[string]interface{}, error) {
	baseConfigPath := b.findConfigFile(filepath.Join(b.BaseDir, "base", "tenants"), key)
	if baseConfigPath == "" {
		if b.verboseDebug && b.logger != nil {
			b.logger.Debug("BaseConfigFeeder: No base tenant config found",
				"key", key,
				"baseDir", filepath.Join(b.BaseDir, "base", "tenants"))
		}
		return make(map[string]interface{}), nil
	}

	return b.loadConfigFile(baseConfigPath)
}

// loadEnvironmentConfigForKey loads environment config for a specific key (used for tenant configs)
func (b *BaseConfigFeeder) loadEnvironmentConfigForKey(key string) (map[string]interface{}, error) {
	envConfigPath := b.findConfigFile(filepath.Join(b.BaseDir, "environments", b.Environment, "tenants"), key)
	if envConfigPath == "" {
		if b.verboseDebug && b.logger != nil {
			b.logger.Debug("BaseConfigFeeder: No environment tenant config found",
				"key", key,
				"envDir", filepath.Join(b.BaseDir, "environments", b.Environment, "tenants"))
		}
		return make(map[string]interface{}), nil
	}

	return b.loadConfigFile(envConfigPath)
}

// findConfigFile searches for a config file with the given name and supported extensions.
// Extensions are tried in order: .yaml, .yml, .json, .toml - the first found file is returned.
// This order affects configuration precedence when multiple formats exist for the same config.
func (b *BaseConfigFeeder) findConfigFile(dir, name string) string {
	extensions := []string{".yaml", ".yml", ".json", ".toml"}

	for _, ext := range extensions {
		configPath := filepath.Join(dir, name+ext)
		if _, err := os.Stat(configPath); err == nil {
			if b.verboseDebug && b.logger != nil {
				b.logger.Debug("BaseConfigFeeder: Found config file", "path", configPath)
			}
			return configPath
		}
	}

	return ""
}

// loadConfigFile loads a configuration file into a map, automatically detecting the format
// based on the file extension (.yaml, .yml, .json, .toml)
func (b *BaseConfigFeeder) loadConfigFile(filePath string) (map[string]interface{}, error) {
	if b.verboseDebug && b.logger != nil {
		b.logger.Debug("BaseConfigFeeder: Loading config file", "path", filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var config map[string]interface{}
	ext := filepath.Ext(filePath)

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal YAML file %s: %w", filePath, err)
		}
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON file %s: %w", filePath, err)
		}
	case ".toml":
		if err := toml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal TOML file %s: %w", filePath, err)
		}
	default:
		// Default to YAML for backward compatibility
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config file %s (defaulted to YAML): %w", filePath, err)
		}
	}

	if b.verboseDebug && b.logger != nil {
		b.logger.Debug("BaseConfigFeeder: Successfully loaded config file", "path", filePath, "format", ext, "keys", len(config))
	}

	return config, nil
}

// mergeConfigs merges environment config over base config (deep merge)
func (b *BaseConfigFeeder) mergeConfigs(base, override map[string]interface{}) map[string]interface{} {
	if b.verboseDebug && b.logger != nil {
		b.logger.Debug("BaseConfigFeeder: Merging configurations",
			"baseKeys", len(base),
			"overrideKeys", len(override))
	}

	merged := make(map[string]interface{})

	// Copy all base config values
	for key, value := range base {
		merged[key] = value
	}

	// Apply overrides
	for key, overrideValue := range override {
		if baseValue, exists := base[key]; exists {
			// If both values are maps, merge them recursively
			if baseMap, baseIsMap := baseValue.(map[string]interface{}); baseIsMap {
				if overrideMap, overrideIsMap := overrideValue.(map[string]interface{}); overrideIsMap {
					merged[key] = b.mergeConfigs(baseMap, overrideMap)
					continue
				}
			}
		}
		// Otherwise, override completely replaces base value
		merged[key] = overrideValue
	}

	if b.verboseDebug && b.logger != nil {
		b.logger.Debug("BaseConfigFeeder: Configuration merge completed", "mergedKeys", len(merged))
	}

	return merged
}

// applyConfigToStruct applies the merged configuration to the target structure
func (b *BaseConfigFeeder) applyConfigToStruct(config map[string]interface{}, target interface{}) error {
	if b.verboseDebug && b.logger != nil {
		b.logger.Debug("BaseConfigFeeder: Applying config to struct",
			"targetType", reflect.TypeOf(target),
			"configKeys", len(config))
	}

	// Convert the merged config back to YAML and then unmarshal into target struct
	// This ensures proper type conversion and structure validation
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal merged config: %w", err)
	}

	if err := yaml.Unmarshal(yamlData, target); err != nil {
		return fmt.Errorf("failed to unmarshal config to target struct: %w", err)
	}

	if b.verboseDebug && b.logger != nil {
		b.logger.Debug("BaseConfigFeeder: Successfully applied config to struct")
	}

	return nil
}

// IsBaseConfigStructure checks if the given directory has the expected base config structure
func IsBaseConfigStructure(configDir string) bool {
	// Check for base/ directory
	baseDir := filepath.Join(configDir, "base")
	if stat, err := os.Stat(baseDir); err != nil || !stat.IsDir() {
		return false
	}

	// Check for environments/ directory
	envDir := filepath.Join(configDir, "environments")
	if stat, err := os.Stat(envDir); err != nil || !stat.IsDir() {
		return false
	}

	return true
}

// GetAvailableEnvironments returns the list of available environments in the config directory
// in alphabetical order for consistent, deterministic behavior
func GetAvailableEnvironments(configDir string) []string {
	envDir := filepath.Join(configDir, "environments")
	entries, err := os.ReadDir(envDir)
	if err != nil {
		return nil
	}

	var environments []string
	for _, entry := range entries {
		if entry.IsDir() {
			environments = append(environments, entry.Name())
		}
	}

	// Sort alphabetically for deterministic behavior
	sort.Strings(environments)
	return environments
}
