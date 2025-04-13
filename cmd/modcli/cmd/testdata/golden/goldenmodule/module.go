package goldenmodule

import (
	"context"
	"github.com/GoCodeAlone/modular"
)

// GoldenModuleModule implements the Modular module interface
type GoldenModuleModule struct {
	config        *GoldenModuleConfig
	tenantConfigs map[modular.TenantID]*GoldenModuleConfig
}

// NewGoldenModuleModule creates a new instance of the GoldenModule module
func NewGoldenModuleModule() modular.Module {
	return &GoldenModuleModule{
		tenantConfigs: make(map[modular.TenantID]*GoldenModuleConfig),
	}
}

// Name returns the unique identifier for this module
func (m *GoldenModuleModule) Name() string {
	return "goldenmodule"
}

// RegisterConfig registers configuration requirements
func (m *GoldenModuleModule) RegisterConfig(app modular.Application) error {
	m.config = &GoldenModuleConfig{
		// Default values can be set here
	}

	app.RegisterConfigSection("goldenmodule", modular.NewStdConfigProvider(m.config))
	return nil
}

// Init initializes the module
func (m *GoldenModuleModule) Init(app modular.Application) error {
	// Initialize module resources

	return nil
}

// Dependencies returns names of other modules this module depends on
func (m *GoldenModuleModule) Dependencies() []string {
	return []string{
		// Add dependencies here
	}
}

// ProvidesServices returns a list of services provided by this module
func (m *GoldenModuleModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		// Example:
		// {
		//     Name:        "serviceName",
		//     Description: "Description of the service",
		//     Instance:    serviceInstance,
		// },
	}
}

// RequiresServices returns a list of services required by this module
func (m *GoldenModuleModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		// Example:
		// {
		//     Name:     "requiredService",
		//     Required: true, // Whether this service is optional or required
		// },
	}
}

// Start is called when the application is starting
func (m *GoldenModuleModule) Start(ctx context.Context) error {
	// Startup logic goes here

	return nil
}

// Stop is called when the application is shutting down
func (m *GoldenModuleModule) Stop(ctx context.Context) error {
	// Shutdown/cleanup logic goes here

	return nil
}

// OnTenantRegistered is called when a new tenant is registered
func (m *GoldenModuleModule) OnTenantRegistered(tenantID modular.TenantID) {
	// Initialize tenant-specific resources
}

// OnTenantRemoved is called when a tenant is removed
func (m *GoldenModuleModule) OnTenantRemoved(tenantID modular.TenantID) {
	// Clean up tenant-specific resources
	delete(m.tenantConfigs, tenantID)
}
