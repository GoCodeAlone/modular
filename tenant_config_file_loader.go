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
func LoadTenantConfigs(app Application, tenantService TenantService, params TenantConfigParams) error {
	// Check if directory exists, and throw a error if it doesn't
	if _, err := os.Stat(params.ConfigDir); os.IsNotExist(err) {
		app.Logger().Error("Tenant config directory does not exist", "directory", params.ConfigDir)
		return fmt.Errorf("tenant config directory does not exist: %w", err)
	}

	files, err := os.ReadDir(params.ConfigDir)
	if err != nil {
		return fmt.Errorf("failed to read tenant config directory: %w", err)
	}

	if len(files) == 0 {
		app.Logger().Warn("No files found in tenant config directory", "directory", params.ConfigDir)
		return nil
	}

	loadedTenants := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !params.ConfigNameRegex.MatchString(file.Name()) {
			app.Logger().Debug("Skipping file that doesn't match regex pattern",
				"file", file.Name(), "pattern", params.ConfigNameRegex.String())
			continue
		}

		// Strip the extension to get the tenant ID
		ext := filepath.Ext(file.Name())
		name := strings.TrimSuffix(file.Name(), ext)

		tenantID := TenantID(name)
		configPath := filepath.Join(params.ConfigDir, file.Name())

		app.Logger().Debug("Loading tenant config file", "tenantID", tenantID, "file", configPath)

		var feederSlice []Feeder
		switch strings.ToLower(ext) { // Ensure case-insensitive extension matching
		case ".json":
			feederSlice = append(feederSlice, feeders.NewJsonFeeder(configPath))
		case ".yaml", ".yml":
			feederSlice = append(feederSlice, feeders.NewYamlFeeder(configPath))
		case ".toml":
			feederSlice = append(feederSlice, feeders.NewTomlFeeder(configPath))
		default:
			app.Logger().Warn("Unsupported config file extension", "file", file.Name(), "extension", ext)
			continue // Skip but don't fail
		}

		// Add any additional feeders from params
		feederSlice = append(feederSlice, params.ConfigFeeders...)

		tenantCfgs, err := loadTenantConfig(app, feederSlice)
		if err != nil {
			app.Logger().Error("Failed to load tenant config", "tenantID", tenantID, "error", err)
			continue // Skip this tenant but continue with others
		}

		// Register the tenant even with empty configs
		if err = tenantService.RegisterTenant(tenantID, tenantCfgs); err != nil {
			return fmt.Errorf("failed to register tenant %s: %w", tenantID, err)
		}
		loadedTenants++
	}

	app.Logger().Info("Tenant configuration loading complete", "loadedTenants", loadedTenants)
	return nil
}

func loadTenantConfig(app Application, configFeeders []Feeder) (map[string]ConfigProvider, error) {
	// Guard against nil application
	if app == nil {
		return nil, ErrApplicationNil
	}

	// Skip if no configFeeders are defined
	if len(configFeeders) == 0 {
		app.Logger().Info("No config feeders defined, skipping tenant config loading")
		return nil, nil
	}

	app.Logger().Debug("Loading tenant config", "configFeedersCount", len(configFeeders))

	// Build the configuration
	cfgBuilder := NewConfig()
	for _, feeder := range configFeeders {
		cfgBuilder.AddFeeder(feeder)
	}

	// Process registered sections
	sectionInfos, hasValidSections := prepareSectionConfigs(app, cfgBuilder)
	if !hasValidSections {
		app.Logger().Warn("No valid sections found for tenant config")
		return make(map[string]ConfigProvider), nil
	}

	// Feed the config
	if err := cfgBuilder.Feed(); err != nil {
		return nil, fmt.Errorf("failed to feed configuration: %w", err)
	}

	// Process fed configurations
	tenantCfgSections := processFedConfigurations(app, sectionInfos)

	// Log the loaded configurations for debugging only - don't print actual values in production
	logLoadedSections(app, tenantCfgSections)

	return tenantCfgSections, nil
}

// prepareSectionConfigs creates temporary configurations for all registered sections
func prepareSectionConfigs(app Application, cfgBuilder *Config) (map[string]configInfo, bool) {
	sectionInfos := make(map[string]configInfo)
	hasValidSections := false

	for sectionKey, provider := range app.ConfigSections() {
		if provider == nil {
			app.Logger().Warn("Skipping nil config provider", "section", sectionKey)
			continue
		}

		sectionCfg := provider.GetConfig()
		if sectionCfg == nil {
			app.Logger().Warn("Skipping section with nil config", "section", sectionKey)
			continue
		}

		tempSectionCfg, sectionInfo, err := createTempConfig(sectionCfg)
		if err != nil {
			app.Logger().Warn("Failed to create temp config", "section", sectionKey, "error", err)
			continue
		}

		sectionInfos[sectionKey] = sectionInfo
		cfgBuilder.AddStructKey(sectionKey, tempSectionCfg)
		hasValidSections = true
	}

	return sectionInfos, hasValidSections
}

// processFedConfigurations handles the configuration after it's been fed with values
func processFedConfigurations(app Application, sectionInfos map[string]configInfo) map[string]ConfigProvider {
	tenantCfgSections := make(map[string]ConfigProvider)

	for sectionKey, info := range sectionInfos {
		if !info.tempVal.Elem().IsValid() {
			app.Logger().Warn("Tenant section config is invalid after feeding", "section", sectionKey)
			continue
		}

		provider, err := createSectionProvider(app, sectionKey, info)
		if err != nil {
			app.Logger().Warn("Failed to create section provider", "section", sectionKey, "error", err)
			continue
		}

		if provider != nil {
			tenantCfgSections[sectionKey] = provider
			app.Logger().Debug("Added tenant config section", "section", sectionKey,
				"configType", fmt.Sprintf("%T", provider.GetConfig()))
		}
	}

	return tenantCfgSections
}

// createSectionProvider creates a provider for a section with the fed configuration
func createSectionProvider(app Application, sectionKey string, info configInfo) (ConfigProvider, error) {
	// Get the actual instance of the config
	configValue := info.tempVal.Elem().Interface()
	if configValue == nil {
		return nil, ErrTenantSectionConfigNil
	}

	// Create a deep clone of the original section config type
	originalSectionCfgProvider, err := app.GetConfigSection(sectionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get original section config: %w", err)
	}

	originalSectionCfg := originalSectionCfgProvider.GetConfig()
	configClone, err := cloneConfigWithValues(originalSectionCfg, configValue)
	if err != nil {
		return nil, fmt.Errorf("failed to clone config with values: %w", err)
	}

	// Create a new provider with the properly typed config
	provider := NewStdConfigProvider(configClone)
	if provider == nil || provider.GetConfig() == nil {
		return nil, ErrCreatedNilProvider
	}

	return provider, nil
}

// logLoadedSections logs information about the loaded tenant config sections
func logLoadedSections(app Application, tenantCfgSections map[string]ConfigProvider) {
	if len(tenantCfgSections) > 0 {
		app.Logger().Debug("Loaded tenant config sections", "sectionCount", len(tenantCfgSections),
			"sections", getSectionNames(tenantCfgSections))
	} else {
		app.Logger().Warn("No tenant config sections were loaded. Check file format and section names.")
	}
}

// Helper function to extract section names for logging
func getSectionNames(sections map[string]ConfigProvider) []string {
	names := make([]string, 0, len(sections))
	for name := range sections {
		names = append(names, name)
	}
	return names
}

// cloneConfigWithValues creates a new instance of the originalConfig type
// and copies values from loadedConfig into it
func cloneConfigWithValues(originalConfig, loadedConfig interface{}) (interface{}, error) {
	if originalConfig == nil || loadedConfig == nil {
		return nil, ErrOriginalOrLoadedNil
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
		return ErrDestinationNotPointer
	}

	// Dereference pointers to get the underlying values
	if dstVal.Kind() == reflect.Ptr {
		dstVal = dstVal.Elem()
	}

	if srcVal.Kind() == reflect.Ptr {
		srcVal = srcVal.Elem()
	}

	// Handle different source types
	//nolint:exhaustive
	switch srcVal.Kind() {
	case reflect.Map:
		return copyMapToStruct(dstVal, srcVal)
	case reflect.Struct:
		return copyStructToStruct(dstVal, srcVal)
	default:
		return fmt.Errorf("%w: %v", ErrUnsupportedSourceType, srcVal.Kind())
	}
}

// copyMapToStruct copies values from a map to a struct
func copyMapToStruct(dstVal, srcVal reflect.Value) error {
	if dstVal.Kind() != reflect.Struct {
		return ErrCannotCopyMapToStruct
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

		if err := setFieldValue(dstField, srcValue); err != nil {
			// Just log and continue if a specific field can't be set
			continue
		}
	}
	return nil
}

// copyStructToStruct copies values from one struct to another
func copyStructToStruct(dstVal, srcVal reflect.Value) error {
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

// setFieldValue attempts to set a field value, handling various type conversions
func setFieldValue(dstField, srcValue reflect.Value) error {
	// Direct assignment if types are compatible
	if srcValue.Type().AssignableTo(dstField.Type()) {
		dstField.Set(srcValue)
		return nil
	}

	// Try to handle interface{} values by using their concrete type
	if srcValue.Kind() == reflect.Interface {
		return setInterfaceFieldValue(dstField, srcValue)
	}

	return ErrIncompatibleFieldTypes
}

// setInterfaceFieldValue handles setting a field from an interface{} value
func setInterfaceFieldValue(dstField, srcValue reflect.Value) error {
	concreteValue := srcValue.Elem()

	// Direct assignment of concrete value if possible
	if concreteValue.Type().AssignableTo(dstField.Type()) {
		dstField.Set(concreteValue)
		return nil
	}

	// Special handling for map types
	if dstField.Kind() == reflect.Map && concreteValue.Kind() == reflect.Map {
		return copyMapValues(dstField, concreteValue)
	}

	return ErrIncompatibleInterfaceValue
}

// copyMapValues copies values from one map to another
func copyMapValues(dstMap, srcMap reflect.Value) error {
	if dstMap.IsNil() {
		dstMap.Set(reflect.MakeMap(dstMap.Type()))
	}

	// Copy map entries if key types are compatible
	for _, mapKey := range srcMap.MapKeys() {
		mapValue := srcMap.MapIndex(mapKey)
		dstMap.SetMapIndex(mapKey, mapValue)
	}

	return nil
}
