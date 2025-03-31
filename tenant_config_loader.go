package modular

import (
	"regexp"
)

// TenantConfigLoader is an interface for loading tenant configurations
type TenantConfigLoader interface {
	// LoadTenantConfigurations loads configurations for all tenants
	LoadTenantConfigurations(app Application, tenantService TenantService) error
}

// FileBasedTenantConfigLoader implements TenantConfigLoader for file-based tenant configurations
type FileBasedTenantConfigLoader struct {
	configParams TenantConfigParams
}

// NewFileBasedTenantConfigLoader creates a new file-based tenant config loader
func NewFileBasedTenantConfigLoader(params TenantConfigParams) *FileBasedTenantConfigLoader {
	return &FileBasedTenantConfigLoader{
		configParams: params,
	}
}

// LoadTenantConfigurations loads tenant configurations from files
func (l *FileBasedTenantConfigLoader) LoadTenantConfigurations(app Application, tenantService TenantService) error {
	app.Logger().Info("Loading tenant configurations from files",
		"directory", l.configParams.ConfigDir,
		"pattern", l.configParams.ConfigNameRegex.String())

	// Ensure LoadTenantConfigs properly handles existing tenants
	if err := LoadTenantConfigs(app, tenantService, l.configParams); err != nil {
		app.Logger().Error("Failed to load tenant configurations", "error", err)
		return err
	}

	// Get the current tenants after loading
	tenants := tenantService.GetTenants()

	// If no tenants were loaded, log a warning but don't consider it an error
	if len(tenants) == 0 {
		app.Logger().Warn("No tenant configurations were loaded",
			"directory", l.configParams.ConfigDir,
			"pattern", l.configParams.ConfigNameRegex.String())
	} else {
		app.Logger().Info("Successfully loaded tenant configurations",
			"tenantCount", len(tenants))

		// Validate that each tenant has the expected config sections
		for _, tenantID := range tenants {
			if service, ok := tenantService.(*StandardTenantService); ok {
				service.logTenantConfigStatus(tenantID)
			}
		}
	}

	return nil
}

// DefaultTenantConfigLoader creates a loader with default configuration
func DefaultTenantConfigLoader(configDir string) *FileBasedTenantConfigLoader {
	return NewFileBasedTenantConfigLoader(TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile("^\\w+\\.(json|yaml|yml|toml)$"),
		ConfigDir:       configDir,
		ConfigFeeders:   []Feeder{},
	})
}
