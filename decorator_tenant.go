package modular

import (
	"fmt"
)

// TenantAwareDecorator wraps an application to add tenant resolution capabilities.
// It injects tenant resolution before Start() and provides tenant-aware functionality.
type TenantAwareDecorator struct {
	*BaseApplicationDecorator
	tenantLoader TenantLoader
}

// NewTenantAwareDecorator creates a new tenant-aware decorator
func NewTenantAwareDecorator(inner Application, loader TenantLoader) *TenantAwareDecorator {
	return &TenantAwareDecorator{
		BaseApplicationDecorator: NewBaseApplicationDecorator(inner),
		tenantLoader:             loader,
	}
}

// Start overrides the base Start method to inject tenant resolution
func (d *TenantAwareDecorator) Start() error {
	// Perform tenant resolution before starting the application
	if err := d.resolveTenants(); err != nil {
		return err
	}

	// Call the base Start method
	return d.BaseApplicationDecorator.Start()
}

// resolveTenants performs tenant resolution and setup
func (d *TenantAwareDecorator) resolveTenants() error {
	if d.tenantLoader == nil {
		d.Logger().Debug("No tenant loader provided, skipping tenant resolution")
		return nil
	}

	// Load tenants using the tenant loader
	tenants, err := d.tenantLoader.LoadTenants()
	if err != nil {
		return fmt.Errorf("failed to load tenants: %w", err)
	}

	// Register tenant service if available
	for _, tenant := range tenants {
		d.Logger().Debug("Resolved tenant", "tenantID", tenant.ID, "name", tenant.Name)
	}

	return nil
}

// GetTenantService implements TenantApplication interface
func (d *TenantAwareDecorator) GetTenantService() (TenantService, error) {
	return d.BaseApplicationDecorator.GetTenantService()
}

// WithTenant implements TenantApplication interface
func (d *TenantAwareDecorator) WithTenant(tenantID TenantID) (*TenantContext, error) {
	return d.BaseApplicationDecorator.WithTenant(tenantID)
}

// GetTenantConfig implements TenantApplication interface
func (d *TenantAwareDecorator) GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error) {
	return d.BaseApplicationDecorator.GetTenantConfig(tenantID, section)
}
