package modular

import (
	"fmt"
	"sync"
)

// StandardTenantService provides a basic implementation of the TenantService interface
type StandardTenantService struct {
	tenantConfigs      map[TenantID]*TenantConfigProvider
	mutex              sync.RWMutex
	logger             Logger
	tenantAwareModules []TenantAwareModule
}

// NewStandardTenantService creates a new tenant service
func NewStandardTenantService(logger Logger) *StandardTenantService {
	return &StandardTenantService{
		tenantConfigs:      make(map[TenantID]*TenantConfigProvider),
		logger:             logger,
		tenantAwareModules: make([]TenantAwareModule, 0),
	}
}

// GetTenantConfig retrieves tenant-specific configuration
func (ts *StandardTenantService) GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error) {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	tenantCfg, exists := ts.tenantConfigs[tenantID]
	if !exists {
		ts.logger.Debug("Tenant not found", "tenantID", tenantID)
		return nil, fmt.Errorf("tenant %s not found", tenantID)
	}

	provider, err := tenantCfg.GetTenantConfig(tenantID, section)
	if err != nil {
		ts.logger.Debug("Tenant config section not found", "tenantID", tenantID, "section", section, "error", err)
		return nil, err
	}

	if provider == nil {
		ts.logger.Warn("Tenant config provider is nil", "tenantID", tenantID, "section", section)
		return nil, fmt.Errorf("tenant %s has nil config provider for section %s", tenantID, section)
	}

	cfg := provider.GetConfig()
	if cfg == nil {
		ts.logger.Warn("Tenant config is nil", "tenantID", tenantID, "section", section)
		return nil, fmt.Errorf("tenant %s has nil config for section %s", tenantID, section)
	}

	return provider, nil
}

// GetTenants returns all registered tenant IDs
func (ts *StandardTenantService) GetTenants() []TenantID {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	tenants := make([]TenantID, 0, len(ts.tenantConfigs))
	for tenantID := range ts.tenantConfigs {
		tenants = append(tenants, tenantID)
	}
	return tenants
}

// RegisterTenant registers a new tenant with optional initial configs
func (ts *StandardTenantService) RegisterTenant(tenantID TenantID, configs map[string]ConfigProvider) error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	// Check if tenant already exists and update existing configs instead of returning an error
	if existingConfig, exists := ts.tenantConfigs[tenantID]; exists {
		ts.logger.Info("Tenant already registered, merging configurations", "tenantID", tenantID)

		// Add or update configs for existing tenant
		if configs != nil && len(configs) > 0 {
			for section, provider := range configs {
				if provider == nil || provider.GetConfig() == nil {
					ts.logger.Warn("Skipping nil config provider or config", "tenantID", tenantID, "section", section)
					continue
				}
				ts.logger.Info(fmt.Sprintf("Updating config for tenant %s", tenantID), "section", section)
				existingConfig.SetTenantConfig(tenantID, section, provider)
			}
		}
		return nil
	}

	// Create new tenant configuration
	tenantCfg := NewTenantConfigProvider(nil)
	ts.tenantConfigs[tenantID] = tenantCfg

	// Add initial configs if provided
	if configs != nil && len(configs) > 0 {
		for section, provider := range configs {
			if provider == nil || provider.GetConfig() == nil {
				ts.logger.Warn("Skipping nil config provider or config", "tenantID", tenantID, "section", section)
				continue
			}
			ts.logger.Info(fmt.Sprintf("Registering tenant %s", tenantID), "section", section)
			tenantCfg.SetTenantConfig(tenantID, section, provider)
		}
	} else {
		// Make sure the tenant has an empty configs map initialized
		tenantCfg.initializeConfigsForTenant(tenantID)
	}

	ts.logger.Info("Registered tenant", "tenantID", tenantID)

	// Notify tenant-aware modules
	for _, module := range ts.tenantAwareModules {
		module.OnTenantRegistered(tenantID)
	}

	return nil
}

// RemoveTenant removes a tenant and its configurations
func (ts *StandardTenantService) RemoveTenant(tenantID TenantID) error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if _, exists := ts.tenantConfigs[tenantID]; !exists {
		return fmt.Errorf("tenant %s not found", tenantID)
	}

	delete(ts.tenantConfigs, tenantID)
	ts.logger.Info("Removed tenant", "tenantID", tenantID)

	// Notify tenant-aware modules
	for _, module := range ts.tenantAwareModules {
		module.OnTenantRemoved(tenantID)
	}

	return nil
}

// RegisterTenantAwareModule registers a module to receive tenant events
func (ts *StandardTenantService) RegisterTenantAwareModule(module TenantAwareModule) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	ts.tenantAwareModules = append(ts.tenantAwareModules, module)

	// Notify about existing tenants
	for tenantID := range ts.tenantConfigs {
		module.OnTenantRegistered(tenantID)
	}
}

// RegisterTenantConfigSection registers a configuration section for a specific tenant
func (ts *StandardTenantService) RegisterTenantConfigSection(tenantID TenantID, section string, provider ConfigProvider) error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	tenantCfg, exists := ts.tenantConfigs[tenantID]
	if !exists {
		// Create tenant if it doesn't exist
		tenantCfg = NewTenantConfigProvider(nil)
		ts.tenantConfigs[tenantID] = tenantCfg
		ts.logger.Info("Created new tenant during config section registration", "tenantID", tenantID)

		// Notify modules of the new tenant
		for _, module := range ts.tenantAwareModules {
			module.OnTenantRegistered(tenantID)
		}
	}

	if provider == nil || provider.GetConfig() == nil {
		return fmt.Errorf("cannot register nil config provider or config for tenant %s section %s", tenantID, section)
	}

	tenantCfg.SetTenantConfig(tenantID, section, provider)
	ts.logger.Info("Registered tenant config section", "tenantID", tenantID, "section", section)
	return nil
}

// logTenantConfigStatus logs information about the configuration status for a tenant
func (ts *StandardTenantService) logTenantConfigStatus(tenantID TenantID) {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	tenantCfg, exists := ts.tenantConfigs[tenantID]
	if !exists {
		ts.logger.Warn("Attempting to log status for unregistered tenant", "tenantID", tenantID)
		return
	}

	if tenantCfg == nil {
		ts.logger.Warn("Tenant has nil config provider", "tenantID", tenantID)
		return
	}

	// Count sections and log them
	sectionCount := 0
	var sections []string

	for section := range tenantCfg.tenantConfigs[tenantID] {
		sectionCount++
		sections = append(sections, section)
	}

	ts.logger.Info("Tenant configuration status",
		"tenantID", tenantID,
		"sectionCount", sectionCount,
		"sections", sections)
}
