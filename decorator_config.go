package modular

import (
	"errors"
)

// instanceAwareConfigDecorator implements instance-aware configuration decoration
type instanceAwareConfigDecorator struct{}

// DecorateConfig applies instance-aware configuration decoration
func (d *instanceAwareConfigDecorator) DecorateConfig(base ConfigProvider) ConfigProvider {
	return &instanceAwareConfigProvider{
		base: base,
	}
}

// Name returns the decorator name for debugging
func (d *instanceAwareConfigDecorator) Name() string {
	return "InstanceAware"
}

// instanceAwareConfigProvider wraps a config provider to add instance awareness
type instanceAwareConfigProvider struct {
	base ConfigProvider
}

// GetConfig returns the base configuration
func (p *instanceAwareConfigProvider) GetConfig() interface{} {
	return p.base.GetConfig()
}

// tenantAwareConfigDecorator implements tenant-aware configuration decoration
type tenantAwareConfigDecorator struct {
	loader TenantLoader
}

// DecorateConfig applies tenant-aware configuration decoration
func (d *tenantAwareConfigDecorator) DecorateConfig(base ConfigProvider) ConfigProvider {
	return &tenantAwareConfigProvider{
		base:   base,
		loader: d.loader,
	}
}

// Name returns the decorator name for debugging
func (d *tenantAwareConfigDecorator) Name() string {
	return "TenantAware"
}

// tenantAwareConfigProvider wraps a config provider to add tenant awareness
type tenantAwareConfigProvider struct {
	base   ConfigProvider
	loader TenantLoader
}

// GetConfig returns the base configuration
func (p *tenantAwareConfigProvider) GetConfig() interface{} {
	return p.base.GetConfig()
}

// Predefined error for missing tenant loader
var errNoTenantLoaderConfigured = errors.New("no tenant loader configured")

// GetTenantConfig retrieves configuration for a specific tenant
func (p *tenantAwareConfigProvider) GetTenantConfig(tenantID TenantID) (interface{}, error) {
	if p.loader == nil {
		return nil, errNoTenantLoaderConfigured
	}

	// This is a simplified implementation - in a real scenario,
	// you'd load tenant-specific configuration from the tenant loader
	return p.base.GetConfig(), nil
}
