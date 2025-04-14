package goldenmodule

import (
	"context" 
	"github.com/GoCodeAlone/modular" 
	"log/slog" 
	"fmt" 
	"encoding/json" 
)


// Config holds the configuration for the GoldenModule module
type Config struct {
	// Add configuration fields here
	// ExampleField string `mapstructure:"example_field"`
}

// ProvideDefaults sets default values for the configuration
func (c *Config) ProvideDefaults() {
	// Set default values here
	// c.ExampleField = "default_value"
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Add validation logic here
	// if c.ExampleField == "" {
	//     return fmt.Errorf("example_field cannot be empty")
	// }
	return nil
}

// GetConfig implements the modular.ConfigProvider interface
func (c *Config) GetConfig() interface{} {
	return c
}


// GoldenModuleModule represents the GoldenModule module
type GoldenModuleModule struct {
	name string
	config *Config
	tenantConfigs map[modular.TenantID]*Config
	// Add other dependencies or state fields here
}

// NewGoldenModuleModule creates a new instance of the GoldenModule module
func NewGoldenModuleModule() modular.Module {
	return &GoldenModuleModule{
		name: "goldenmodule",
		tenantConfigs: make(map[modular.TenantID]*Config),
	}
}

// Name returns the name of the module
func (m *GoldenModuleModule) Name() string {
	return m.name
}


// RegisterConfig registers the module's configuration structure
func (m *GoldenModuleModule) RegisterConfig(app modular.Application) error {
	m.config = &Config{} // Initialize with defaults or empty struct
	app.RegisterConfigSection(m.Name(), m.config)
	
	// Load initial config values if needed (e.g., from app's main provider)
	// Note: Config values will be populated later by feeders during app.Init()
	slog.Debug("Registered config section", "module", m.Name())
	return nil
}


// Init initializes the module
func (m *GoldenModuleModule) Init(app modular.Application) error {
	slog.Info("Initializing GoldenModule module")
	
	// Example: Resolve service dependencies
	// var myService MyServiceType
	// if err := app.GetService("myServiceName", &myService); err != nil {
	//     return fmt.Errorf("failed to get service 'myServiceName': %w", err)
	// }
	// m.myService = myService
	
	// Add module initialization logic here
	return nil
}


// Start performs startup logic for the module
func (m *GoldenModuleModule) Start(ctx context.Context) error {
	slog.Info("Starting GoldenModule module")
	// Add module startup logic here
	return nil
}



// Stop performs shutdown logic for the module
func (m *GoldenModuleModule) Stop(ctx context.Context) error {
	slog.Info("Stopping GoldenModule module")
	// Add module shutdown logic here
	return nil
}



// Dependencies returns the names of modules this module depends on
func (m *GoldenModuleModule) Dependencies() []string {
	// return []string{"otherModule"} // Add dependencies here
	return nil
}



// ProvidesServices declares services provided by this module
func (m *GoldenModuleModule) ProvidesServices() []modular.ServiceProvider {
	// return []modular.ServiceProvider{
	//     {Name: "myService", Instance: myServiceImpl},
	// }
	return nil
}



// RequiresServices declares services required by this module
func (m *GoldenModuleModule) RequiresServices() []modular.ServiceDependency {
	// return []modular.ServiceDependency{
	//     {Name: "requiredService", Optional: false},
	// }
	return nil
}



// OnTenantRegistered is called when a new tenant is registered
func (m *GoldenModuleModule) OnTenantRegistered(tenantID modular.TenantID) {
	slog.Info("Tenant registered in GoldenModule module", "tenantID", tenantID)
	// Perform actions when a tenant is added, e.g., initialize tenant-specific resources
}

// OnTenantRemoved is called when a tenant is removed
func (m *GoldenModuleModule) OnTenantRemoved(tenantID modular.TenantID) {
	slog.Info("Tenant removed from GoldenModule module", "tenantID", tenantID)
	// Perform cleanup for the removed tenant
	delete(m.tenantConfigs, tenantID)
}

// LoadTenantConfig loads the configuration for a specific tenant
func (m *GoldenModuleModule) LoadTenantConfig(tenantService modular.TenantService, tenantID modular.TenantID) error {
	configProvider, err := tenantService.GetTenantConfig(tenantID, m.Name())
	if err != nil {
		// Handle cases where config might be optional for a tenant
		slog.Warn("No specific config found for tenant, using defaults/base.", "module", m.Name(), "tenantID", tenantID)
		// If config is required, return error:
		// return fmt.Errorf("failed to get config for tenant %s in module %s: %w", tenantID, m.Name(), err)
		m.tenantConfigs[tenantID] = m.config // Use base config as fallback
		return nil
	}

	tenantCfg := &Config{} // Create a new config struct for the tenant
	
	// Get the raw config data and unmarshal it
	configData, err := json.Marshal(configProvider.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to marshal config data for tenant %s in module %s: %w", tenantID, m.Name(), err)
	}
	
	if err := json.Unmarshal(configData, tenantCfg); err != nil {
		return fmt.Errorf("failed to unmarshal config for tenant %s in module %s: %w", tenantID, m.Name(), err)
	}

	m.tenantConfigs[tenantID] = tenantCfg
	slog.Debug("Loaded config for tenant", "module", m.Name(), "tenantID", tenantID)
	return nil
}

// GetTenantConfig retrieves the loaded configuration for a specific tenant
// Returns the base config if no specific tenant config is found.
func (m *GoldenModuleModule) GetTenantConfig(tenantID modular.TenantID) *Config {
	if cfg, ok := m.tenantConfigs[tenantID]; ok {
		return cfg
	}
	// Fallback to base config if tenant-specific config doesn't exist
	return m.config
}

