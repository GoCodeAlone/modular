package modular

import (
	"fmt"
	"github.com/GoCodeAlone/modular/feeders"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

type TenantConfigParams struct {
	// ConfigNameRegex is a regex pattern for the config file names (e.g. "^tenant[0-9]+\\.json$").
	ConfigNameRegex *regexp.Regexp
	// ConfigDir is the directory where tenant config files are located.
	ConfigDir string
	// ConfigFeeders are the feeders to use for loading tenant configs.
	ConfigFeeders []Feeder
}

// LoadTenantConfigs scans the given directory for config files. Each file should be named with the tenant ID (e.g. "tenant123.json").
// For each file, it unmarshals the configuration and registers it with the provided TenantService for the given section.
// The configNameRegex is a regex pattern for the config file names (e.g. "^tenant[0-9]+\\.json$").
func LoadTenantConfigs(app *Application, tenantService TenantService, params TenantConfigParams) error {
	// Check if directory exists, and throw a error if it doesn't
	if _, err := os.Stat(params.ConfigDir); os.IsNotExist(err) {
		app.logger.Error("Tenant config directory does not exist", "directory", params.ConfigDir)
		return fmt.Errorf("tenant config directory does not exist: %w", err)
	}

	files, err := os.ReadDir(params.ConfigDir)
	if err != nil {
		return fmt.Errorf("failed to read tenant config directory: %w", err)
	}

	if len(files) == 0 {
		app.logger.Info("No files found in tenant config directory", "directory", params.ConfigDir)
		return nil
	}

	loadedTenants := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !params.ConfigNameRegex.MatchString(file.Name()) {
			app.logger.Debug("Skipping file that doesn't match regex pattern",
				"file", file.Name(), "pattern", params.ConfigNameRegex.String())
			continue
		}

		// Strip the extension to get the tenant ID
		ext := filepath.Ext(file.Name())
		name := strings.TrimSuffix(file.Name(), ext)

		tenantID := TenantID(name)
		configPath := filepath.Join(params.ConfigDir, file.Name())

		app.logger.Debug("Loading tenant config file", "tenantID", tenantID, "file", configPath)

		var feederSlice []Feeder
		switch strings.ToLower(ext) { // Ensure case-insensitive extension matching
		case ".json":
			feederSlice = append(feederSlice, feeders.NewJsonFeeder(configPath))
		case ".yaml", ".yml":
			feederSlice = append(feederSlice, feeders.NewYamlFeeder(configPath))
		case ".toml":
			feederSlice = append(feederSlice, feeders.NewTomlFeeder(configPath))
		default:
			app.logger.Warn("Unsupported config file extension", "file", file.Name(), "extension", ext)
			continue // Skip but don't fail
		}

		// Add any additional feeders from params
		for _, feeder := range params.ConfigFeeders {
			feederSlice = append(feederSlice, feeder)
		}

		tenantCfgs, err := loadTenantConfig(app, feederSlice)
		if err != nil {
			app.logger.Error("Failed to load tenant config", "tenantID", tenantID, "error", err)
			continue // Skip this tenant but continue with others
		}

		// Only register the tenant if we have valid configs to register
		if tenantCfgs != nil && len(tenantCfgs) > 0 {
			// Register the tenant with the loaded configs (will merge if tenant already exists)
			if err = tenantService.RegisterTenant(tenantID, tenantCfgs); err != nil {
				return fmt.Errorf("failed to register tenant %s: %w", tenantID, err)
			}
			loadedTenants++
		} else {
			app.logger.Warn("No valid configs loaded for tenant", "tenantID", tenantID)
			// Still register the tenant but without configs
			if err = tenantService.RegisterTenant(tenantID, nil); err != nil {
				return fmt.Errorf("failed to register tenant %s: %w", tenantID, err)
			}
			loadedTenants++
		}
	}

	app.logger.Info("Tenant configuration loading complete", "loadedTenants", loadedTenants)
	return nil
}

func loadTenantConfig(app *Application, configFeeders []Feeder) (map[string]ConfigProvider, error) {
	// Guard against nil application
	if app == nil {
		return nil, fmt.Errorf("application is nil")
	}

	// Skip if no configFeeders are defined
	if len(configFeeders) == 0 {
		app.logger.Info("No config feeders defined, skipping tenant config loading")
		return nil, nil
	}

	app.logger.Debug("Loading tenant config", "configFeeders", configFeeders)

	// Build the configuration
	cfgBuilder := NewConfig()
	for _, feeder := range configFeeders {
		cfgBuilder.AddFeeder(feeder)
	}

	// Process registered sections
	sectionInfos := make(map[string]configInfo)
	hasValidSections := false
	tenantCfgSections := make(map[string]ConfigProvider)

	// Create temporary structs for all registered sections
	for sectionKey, provider := range app.cfgSections {
		if provider == nil {
			app.logger.Warn("Skipping nil config provider", "section", sectionKey)
			continue
		}

		sectionCfg := provider.GetConfig()
		if sectionCfg == nil {
			app.logger.Warn("Skipping section with nil config", "section", sectionKey)
			continue
		}

		tempSectionCfg, sectionInfo, err := createTempConfig(sectionCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create temp config for section %s: %w", sectionKey, err)
		}

		sectionInfos[sectionKey] = sectionInfo
		cfgBuilder.AddStructKey(sectionKey, tempSectionCfg)
		hasValidSections = true
	}

	// Feed the config for sections if any exist
	if hasValidSections {
		if err := cfgBuilder.Feed(); err != nil {
			return nil, fmt.Errorf("failed to feed configuration: %w", err)
		}

		// Update all section configs
		for sectionKey, info := range sectionInfos {
			if !info.tempVal.Elem().IsValid() {
				app.logger.Warn("Tenant section config is invalid after feeding", "section", sectionKey)
				continue
			}

			// Get the actual instance of the config
			configValue := info.tempVal.Elem().Interface()
			if configValue == nil {
				app.logger.Warn("Tenant section config is nil after feeding", "section", sectionKey)
				continue
			}

			// Create a deep clone of the original section config type
			// This ensures we have the correct type that the modules expect
			originalSectionCfg := app.cfgSections[sectionKey].GetConfig()
			configClone, err := cloneConfigWithValues(originalSectionCfg, configValue)
			if err != nil {
				app.logger.Warn("Failed to clone config with values", "section", sectionKey, "error", err)
				continue
			}

			// Create a new provider with the properly typed config
			provider := NewStdConfigProvider(configClone)
			if provider == nil || provider.GetConfig() == nil {
				app.logger.Warn("Created nil provider for tenant section", "section", sectionKey)
				continue
			}

			tenantCfgSections[sectionKey] = provider
			app.logger.Debug("Added tenant config section",
				"section", sectionKey,
				"configType", fmt.Sprintf("%T", configClone))
		}
	}

	// Log the loaded configurations for debugging
	if len(tenantCfgSections) > 0 {
		for section, provider := range tenantCfgSections {
			app.logger.Debug("Tenant config section loaded",
				"section", section,
				"configType", fmt.Sprintf("%T", provider.GetConfig()),
				"config", fmt.Sprintf("%+v", provider.GetConfig()))
		}
	} else {
		app.logger.Warn("No tenant config sections were loaded. Check file format and section names.")
	}

	app.logger.Info("Loaded tenant config", "sectionCount", len(tenantCfgSections))

	return tenantCfgSections, nil
}

// cloneConfigWithValues creates a new instance of the originalConfig type
// and copies values from loadedConfig into it
func cloneConfigWithValues(originalConfig, loadedConfig interface{}) (interface{}, error) {
	if originalConfig == nil || loadedConfig == nil {
		return nil, fmt.Errorf("original or loaded config is nil")
	}

	origType := reflect.TypeOf(originalConfig)
	if origType.Kind() == reflect.Ptr {
		origType = origType.Elem()
	}

	// Create new instance of the original type
	newInstance := reflect.New(origType).Interface()

	// Copy loaded values to the new instance
	if err := copyStructFields(newInstance, loadedConfig); err != nil {
		return nil, err
	}

	return newInstance, nil
}

// copyStructFields copies field values from src to dst
func copyStructFields(dst, src interface{}) error {
	dstVal := reflect.ValueOf(dst)
	srcVal := reflect.ValueOf(src)

	// Ensure we're working with pointers
	if dstVal.Kind() != reflect.Ptr {
		return fmt.Errorf("destination must be a pointer")
	}

	// Dereference pointers to get the underlying values
	if dstVal.Kind() == reflect.Ptr {
		dstVal = dstVal.Elem()
	}

	if srcVal.Kind() == reflect.Ptr {
		srcVal = srcVal.Elem()
	}

	// Handle different kinds of src/dst
	if srcVal.Kind() == reflect.Map {
		// If source is a map, copy key/value pairs
		if dstVal.Kind() != reflect.Struct {
			return fmt.Errorf("cannot copy from map to non-struct")
		}

		for _, key := range srcVal.MapKeys() {
			if key.Kind() != reflect.String {
				continue // Skip non-string keys
			}

			fieldName := key.String()
			dstField := dstVal.FieldByName(fieldName)
			if !dstField.IsValid() || !dstField.CanSet() {
				continue // Skip fields that can't be set
			}

			srcValue := srcVal.MapIndex(key)
			if !srcValue.IsValid() {
				continue
			}

			// Convert and set if types are compatible
			if srcValue.Type().AssignableTo(dstField.Type()) {
				dstField.Set(srcValue)
			} else if srcValue.Kind() == reflect.Interface {
				// Try to handle interface{} values by using their concrete type
				concreteValue := srcValue.Elem()
				if concreteValue.Type().AssignableTo(dstField.Type()) {
					dstField.Set(concreteValue)
				} else if dstField.Kind() == reflect.Map && concreteValue.Kind() == reflect.Map {
					// Special handling for map types
					if dstField.IsNil() {
						dstField.Set(reflect.MakeMap(dstField.Type()))
					}

					// Copy map entries if key types are compatible
					for _, mapKey := range concreteValue.MapKeys() {
						mapValue := concreteValue.MapIndex(mapKey)
						dstField.SetMapIndex(mapKey, mapValue)
					}
				}
			}
		}
		return nil
	}

	// If source is a struct, copy matching fields
	if srcVal.Kind() == reflect.Struct {
		for i := 0; i < dstVal.NumField(); i++ {
			dstField := dstVal.Field(i)
			if !dstField.CanSet() {
				continue
			}

			fieldName := dstVal.Type().Field(i).Name
			srcField := srcVal.FieldByName(fieldName)
			if !srcField.IsValid() {
				continue
			}

			// Copy if types are compatible
			if srcField.Type().AssignableTo(dstField.Type()) {
				dstField.Set(srcField)
			}
		}
		return nil
	}

	return fmt.Errorf("unsupported source type: %v", srcVal.Kind())
}
