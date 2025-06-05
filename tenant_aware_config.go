package modular

import (
	"context"
)

// TenantAwareConfig provides configuration that's aware of tenant context
type TenantAwareConfig struct {
	defaultConfig ConfigProvider
	tenantService TenantService
	configSection string
}

// NewTenantAwareConfig creates a new tenant-aware configuration provider
func NewTenantAwareConfig(
	defaultConfig ConfigProvider,
	tenantService TenantService,
	configSection string,
) *TenantAwareConfig {
	return &TenantAwareConfig{
		defaultConfig: defaultConfig,
		tenantService: tenantService,
		configSection: configSection,
	}
}

// GetConfig retrieves the default configuration when no tenant is specified
func (tac *TenantAwareConfig) GetConfig() any {
	if tac.defaultConfig == nil {
		return nil
	}
	return tac.defaultConfig.GetConfig()
}

// GetConfigWithContext retrieves tenant-specific configuration based on context
func (tac *TenantAwareConfig) GetConfigWithContext(ctx context.Context) any {
	tenantID, found := GetTenantIDFromContext(ctx)
	if !found {
		// Fall back to default config when no tenant in context
		return tac.GetConfig()
	}

	if tac.tenantService == nil {
		// No tenant service available, return default config
		return tac.GetConfig()
	}

	// Try to get tenant-specific config
	cfg, err := tac.tenantService.GetTenantConfig(tenantID, tac.configSection)
	if err != nil {
		// Fall back to default if tenant config not found
		return tac.GetConfig()
	}

	return cfg.GetConfig()
}

// TenantAwareRegistry provides common service discovery methods that are tenant-aware
type TenantAwareRegistry interface {
	// GetServiceForTenant returns a service instance for a specific tenant
	GetServiceForTenant(name string, tenantID TenantID, target any) error
}
